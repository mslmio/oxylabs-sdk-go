package serp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mslmio/oxylabs-sdk-go/oxylabs"
)

// ScrapeGoogleSearch scrapes google with google_search as source with async polling runtime.
func (c *SerpClientAsync) ScrapeGoogleSearch(
	query string,
	opts ...*GoogleSearchOpts,
) (chan *Response, error) {
	responseChan := make(chan *Response)
	errChan := make(chan error)

	// Prepare options.
	opt := &GoogleSearchOpts{}
	if len(opts) > 0 && opts[len(opts)-1] != nil {
		opt = opts[len(opts)-1]
	}

	// Initialize the context map and apply each provided context modifier function.
	context := make(ContextOption)
	for _, modifier := range opt.Context {
		modifier(context)
	}

	// Check if limit_per_page context parameter is used together with limit, start_page or pages parameters.
	if (opt.Limit != 0 || opt.StartPage != 0 || opt.Pages != 0) && context["limit_per_page"] != nil {
		return nil, fmt.Errorf("limit, start_page and pages parameters cannot be used together with limit_per_page context parameter")
	}

	// Set defaults.
	SetDefaultDomain(&opt.Domain)
	SetDefaultStartPage(&opt.StartPage)
	SetDefaultLimit(&opt.Limit)
	SetDefaultPages(&opt.Pages)
	SetDefaultUserAgent(&opt.UserAgent)

	// Check validity of parameters.
	err := opt.checkParameterValidity(context)
	if err != nil {
		return nil, err
	}

	// Prepare payload.
	var payload map[string]interface{}

	// If user sends limit_per_page context parameter, use it instead of limit, start_page and pages parameters.
	if context["limit_per_page"] != nil {
		payload = map[string]interface{}{
			"source":          "google_search",
			"domain":          opt.Domain,
			"query":           query,
			"geo_location":    opt.Geolocation,
			"user_agent_type": opt.UserAgent,
			"parse":           opt.Parse,
			"render":          opt.Render,
			"context": []map[string]interface{}{
				{
					"key":   "results_language",
					"value": context["results_language"],
				},
				{
					"key":   "filter",
					"value": context["filter"],
				},
				{
					"key":   "limit_per_page",
					"value": context["limit_per_page"],
				},
				{
					"key":   "nfpr",
					"value": context["nfpr"],
				},
				{
					"key":   "safe_search",
					"value": context["safe_search"],
				},
				{
					"key":   "fpstate",
					"value": context["fpstate"],
				},
				{
					"key":   "tbm",
					"value": context["tbm"],
				},
				{
					"key":   "tbs",
					"value": context["tbs"],
				},
			},
		}
	} else {
		payload = map[string]interface{}{
			"source":          "google_search",
			"domain":          opt.Domain,
			"query":           query,
			"start_page":      opt.StartPage,
			"pages":           opt.Pages,
			"limit":           opt.Limit,
			"geo_location":    opt.Geolocation,
			"user_agent_type": opt.UserAgent,
			"parse":           opt.Parse,
			"render":          opt.Render,
			"context": []map[string]interface{}{
				{
					"key":   "results_language",
					"value": context["results_language"],
				},
				{
					"key":   "filter",
					"value": context["filter"],
				},
				{
					"key":   "nfpr",
					"value": context["nfpr"],
				},
				{
					"key":   "safe_search",
					"value": context["safe_search"],
				},
				{
					"key":   "fpstate",
					"value": context["fpstate"],
				},
				{
					"key":   "tbm",
					"value": context["tbm"],
				},
				{
					"key":   "tbs",
					"value": context["tbs"],
				},
			},
		}
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshalling payload: %v", err)
	}

	request, _ := http.NewRequest(
		"POST",
		c.BaseUrl,
		bytes.NewBuffer(jsonPayload),
	)
	request.Header.Add("Content-type", "application/json")
	request.SetBasicAuth(c.ApiCredentials.Username, c.ApiCredentials.Password)
	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	response.Body.Close()

	// Unmarshal into job.
	job := &Job{}
	json.Unmarshal(responseBody, &job)

	go func() {
		startNow := time.Now()

		for {
			request, _ = http.NewRequest(
				"GET",
				fmt.Sprintf("https://data.oxylabs.io/v1/queries/%s", job.ID),
				nil,
			)
			request.Header.Add("Content-type", "application/json")
			request.SetBasicAuth(c.ApiCredentials.Username, c.ApiCredentials.Password)
			response, err = c.HttpClient.Do(request)
			if err != nil {
				errChan <- err
				close(responseChan)
				return
			}

			responseBody, err = io.ReadAll(response.Body)
			if err != nil {
				err = fmt.Errorf("error reading response body: %v", err)
				errChan <- err
				close(responseChan)
				return
			}
			response.Body.Close()

			json.Unmarshal(responseBody, &job)

			if job.Status == "done" {
				JobId := job.ID
				request, _ = http.NewRequest(
					"GET",
					fmt.Sprintf("https://data.oxylabs.io/v1/queries/%s/results", JobId),
					nil,
				)
				request.Header.Add("Content-type", "application/json")
				request.SetBasicAuth(c.ApiCredentials.Username, c.ApiCredentials.Password)
				response, err = c.HttpClient.Do(request)
				if err != nil {
					errChan <- err
					close(responseChan)
					return
				}

				// Read the response body into a buffer.
				responseBody, err := io.ReadAll(response.Body)
				if err != nil {
					err = fmt.Errorf("error reading response body: %v", err)
					errChan <- err
					close(responseChan)
					return
				}
				response.Body.Close()

				// Send back error message.
				if response.StatusCode != 200 {
					err = fmt.Errorf("error with status code %s: %s", response.Status, responseBody)
					errChan <- err
					close(responseChan)
					return
				}

				// Unmarshal the JSON object.
				resp := &Response{}
				if err := resp.UnmarshalJSON(responseBody); err != nil {
					err = fmt.Errorf("failed to parse JSON object: %v", err)
					errChan <- err
					close(responseChan)
					return
				}
				resp.StatusCode = response.StatusCode
				resp.Status = response.Status
				close(errChan)
				responseChan <- resp
			} else if job.Status == "faulted" {
				err = fmt.Errorf("There was an error processing your query")
				errChan <- err
				close(responseChan)
				return
			}

			if time.Since(startNow) > oxylabs.DefaultTimeout {
				err = fmt.Errorf("timeout exceeded: %v", oxylabs.DefaultTimeout)
				errChan <- err
				close(responseChan)
				return
			}

			time.Sleep(oxylabs.DefaultWaitTime)
		}
	}()

	err = <-errChan
	if err != nil {
		return nil, err
	}

	return responseChan, nil
}

// ScrapeGoogleUrl scrapes google with google as source with async polling runtime.
func (c *SerpClientAsync) ScrapeGoogleUrl(
	url string,
	opts ...*GoogleUrlOpts,
) (chan *Response, error) {
	responseChan := make(chan *Response)
	errChan := make(chan error)

	// Check validity of url.
	err := oxylabs.ValidateURL(url, "google")
	if err != nil {
		return nil, err
	}

	// Prepare options.
	opt := &GoogleUrlOpts{}
	if len(opts) > 0 && opts[len(opts)-1] != nil {
		opt = opts[len(opts)-1]
	}

	// Set defaults.
	SetDefaultUserAgent(&opt.UserAgent)

	// Check validity of parameters.
	err = opt.checkParameterValidity()
	if err != nil {
		return nil, err
	}

	// Prepare payload.
	payload := map[string]interface{}{
		"source":          "google",
		"url":             url,
		"user_agent_type": opt.UserAgent,
		"render":          opt.Render,
		"callback_url":    opt.CallbackUrl,
		"geo_location":    opt.GeoLocation,
		"parse":           opt.Parse,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshalling payload: %v", err)
	}

	request, _ := http.NewRequest(
		"POST",
		c.BaseUrl,
		bytes.NewBuffer(jsonPayload),
	)
	request.Header.Add("Content-type", "application/json")
	request.SetBasicAuth(c.ApiCredentials.Username, c.ApiCredentials.Password)
	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	response.Body.Close()

	// Unmarshal into job.
	job := &Job{}
	json.Unmarshal(responseBody, &job)

	go func() {
		startNow := time.Now()

		for {
			request, _ = http.NewRequest(
				"GET",
				fmt.Sprintf("https://data.oxylabs.io/v1/queries/%s", job.ID),
				nil,
			)
			request.Header.Add("Content-type", "application/json")
			request.SetBasicAuth(c.ApiCredentials.Username, c.ApiCredentials.Password)
			response, err = c.HttpClient.Do(request)
			if err != nil {
				errChan <- err
				close(responseChan)
				return
			}

			responseBody, err = io.ReadAll(response.Body)
			if err != nil {
				err = fmt.Errorf("error reading response body: %v", err)
				errChan <- err
				close(responseChan)
				return
			}
			response.Body.Close()

			json.Unmarshal(responseBody, &job)

			if job.Status == "done" {
				JobId := job.ID
				request, _ = http.NewRequest(
					"GET",
					fmt.Sprintf("https://data.oxylabs.io/v1/queries/%s/results", JobId),
					nil,
				)
				request.Header.Add("Content-type", "application/json")
				request.SetBasicAuth(c.ApiCredentials.Username, c.ApiCredentials.Password)
				response, err = c.HttpClient.Do(request)
				if err != nil {
					errChan <- err
					close(responseChan)
					return
				}

				// Read the response body into a buffer.
				responseBody, err := io.ReadAll(response.Body)
				if err != nil {
					err = fmt.Errorf("error reading response body: %v", err)
					errChan <- err
					close(responseChan)
					return
				}
				response.Body.Close()

				// Send back error message.
				if response.StatusCode != 200 {
					err = fmt.Errorf("error with status code %s: %s", response.Status, responseBody)
					errChan <- err
					close(responseChan)
					return
				}

				// Unmarshal the JSON object.
				resp := &Response{}
				if err := resp.UnmarshalJSON(responseBody); err != nil {
					err = fmt.Errorf("failed to parse JSON object: %v", err)
					errChan <- err
					close(responseChan)
					return
				}
				resp.StatusCode = response.StatusCode
				resp.Status = response.Status
				close(errChan)
				responseChan <- resp
			} else if job.Status == "faulted" {
				err = fmt.Errorf("There was an error processing your query")
				errChan <- err
				close(responseChan)
				return
			}

			if time.Since(startNow) > oxylabs.DefaultTimeout {
				err = fmt.Errorf("timeout exceeded: %v", oxylabs.DefaultTimeout)
				errChan <- err
				close(responseChan)
				return
			}

			time.Sleep(oxylabs.DefaultWaitTime)
		}
	}()

	err = <-errChan
	if err != nil {
		return nil, err
	}

	return responseChan, nil
}