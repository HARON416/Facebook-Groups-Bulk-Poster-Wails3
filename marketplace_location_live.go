package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Observe the form's own typeahead response. Saved captions can differ between
// the city search and the home address search; the Facebook page ID is shared.
// This listener only reads search traffic and never sends or submits a listing.
func marketplaceLiveLocations(ctx context.Context, page *rod.Page, query string) (<-chan []MarketplaceLocationHit, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	page = page.Context(ctx)
	updates := make(chan []MarketplaceLocationHit, 1)
	info, err := page.Info()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	if err := (proto.NetworkEnable{}).Call(page); err != nil {
		cancel()
		return nil, nil, err
	}
	requests := map[proto.NetworkRequestID]string{}
	wait := page.EachEvent(func(event *proto.NetworkRequestWillBeSent) {
		request := event.Request
		if request == nil || request.Method != "POST" {
			return
		}
		endpoint, err := url.Parse(request.URL)
		if err != nil || !marketplaceGraphQL(endpoint, info.URL) {
			return
		}
		body, err := (proto.NetworkGetRequestPostData{RequestID: event.RequestID}).Call(page)
		if err != nil {
			return
		}
		form, err := url.ParseQuery(body.PostData)
		if err != nil || form.Get("fb_api_req_friendly_name") != marketplaceAddressQuery {
			return
		}
		var variables struct {
			Params struct {
				Query      string `json:"query"`
				SearchType string `json:"search_type"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(form.Get("variables")), &variables) != nil || strings.TrimSpace(variables.Params.Query) != strings.TrimSpace(query) {
			return
		}
		requests[event.RequestID] = variables.Params.SearchType
	}, func(event *proto.NetworkLoadingFinished) {
		kind, tracked := requests[event.RequestID]
		if !tracked {
			return
		}
		delete(requests, event.RequestID)
		response, err := (proto.NetworkGetResponseBody{RequestID: event.RequestID}).Call(page)
		if err != nil {
			return
		}
		body := []byte(response.Body)
		if response.Base64Encoded {
			body, err = base64.StdEncoding.DecodeString(response.Body)
		}
		if err != nil {
			return
		}
		hits, err := parseMarketplaceLocations(body, kind == "STREET_PLACE_TYPEAHEAD")
		if err != nil {
			return
		}
		select {
		case <-updates:
		default:
		}
		updates <- hits
	}, func(event *proto.NetworkLoadingFailed) {
		delete(requests, event.RequestID)
	})
	done := make(chan struct{})
	go func() { defer close(done); wait() }()
	return updates, func() { cancel(); <-done }, nil
}
