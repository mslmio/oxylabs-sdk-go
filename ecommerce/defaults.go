package ecommerce

import (
	"github.com/mslmio/oxylabs-sdk-go/oxylabs"
)

// SetDefaultSortBy sets the sort by parameter in the ctx if it is not set.
func SetDefaultSortBy(ctx ContextOption) {
	if ctx["sort_by"] == "" {
		ctx["sort_by"] = "r"
	}
}

// SetDefaultDomain sets the domain parameter if it is not set.
func SetDefaultDomain(domain *oxylabs.Domain) {
	if *domain == "" {
		*domain = oxylabs.DOMAIN_COM
	}
}

// SetDefaultStartPage sets the start_page parameter if it is not set.
func SetDefaultStartPage(startPage *int) {
	if *startPage == 0 {
		*startPage = 1
	}
}

// SetDefaultPages sets the pages parameter if it is not set.
func SetDefaultPages(pages *int) {
	if *pages == 0 {
		*pages = 1
	}
}

// SetDefaultLimit sets the limit parameter if it is not set.
func SetDefaultLimit(limit *int) {
	if *limit == 0 {
		*limit = 48
	}
}

// SetDefaultUserAgent sets the user_agent_type parameter if it is not set.
func SetDefaultUserAgent(userAgent *oxylabs.UserAgent) {
	if *userAgent == "" {
		*userAgent = oxylabs.UA_DESKTOP
	}
}