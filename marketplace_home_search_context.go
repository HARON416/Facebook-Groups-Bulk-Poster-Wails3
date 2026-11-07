package main

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Reproduce only the saved, non-authentication address-search params in the
// form's own request. Chrome keeps its current cookies, tokens, document ID,
// headers and response handling. Other GraphQL operations are passed through.
func marketplaceHomeSearchContext(ctx context.Context, page *rod.Page, location *MarketplaceLocationHit, query string) (<-chan error, func(), error) {
	if location.SearchType != "STREET_PLACE_TYPEAHEAD" || location.SearchParams == "" {
		return nil, func() {}, nil
	}
	// Validate before enabling interception or typing in Facebook.
	probe := "variables=" + url.QueryEscape(`{"params":{}}`)
	if _, err := marketplaceLocationBody(probe, query, location.SearchParams); err != nil {
		return nil, nil, err
	}
	info, err := page.Context(ctx).Info()
	if err != nil {
		return nil, nil, err
	}
	job, cancel := context.WithCancel(ctx)
	active := page.Context(job)
	failures := make(chan error, 1)
	fail := func(err error) {
		if err != nil && job.Err() == nil {
			select {
			case failures <- err:
			default:
			}
		}
	}
	wait := active.EachEvent(func(event *proto.FetchRequestPaused) {
		continuation := proto.FetchContinueRequest{RequestID: event.RequestID}
		request := event.Request
		if request != nil && request.Method == "POST" {
			endpoint, err := url.Parse(request.URL)
			if err == nil && marketplaceGraphQL(endpoint, info.URL) {
				form, parseErr := url.ParseQuery(request.PostData)
				var variables struct {
					Params struct {
						Query      string `json:"query"`
						SearchType string `json:"search_type"`
					} `json:"params"`
				}
				if parseErr == nil && form.Get("fb_api_req_friendly_name") == marketplaceAddressQuery && json.Unmarshal([]byte(form.Get("variables")), &variables) == nil && variables.Params.SearchType == "STREET_PLACE_TYPEAHEAD" && strings.TrimSpace(variables.Params.Query) == strings.TrimSpace(query) {
					body, bodyErr := marketplaceLocationBody(request.PostData, query, location.SearchParams)
					if bodyErr == nil {
						continuation.PostData = []byte(body)
					} else {
						fail(bodyErr)
					}
				}
			}
		}
		fail(continuation.Call(active))
	})
	if err := (proto.FetchEnable{Patterns: []*proto.FetchRequestPattern{{URLPattern: "*/api/graphql/*"}}}).Call(active); err != nil {
		cancel()
		return nil, nil, err
	}
	done := make(chan struct{})
	go func() { defer close(done); wait() }()
	return failures, func() {
		cancel()
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = (proto.FetchDisable{}).Call(page.Context(cleanup))
		<-done
	}, nil
}
