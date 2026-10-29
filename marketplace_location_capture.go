package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func sameMarketplaceItemPage(current, target string) bool {
	a, ea := url.Parse(current)
	b, eb := url.Parse(target)
	if ea != nil || eb != nil || a.User != nil || b.User != nil {
		return false
	}
	facebook := func(u *url.URL) bool {
		return u.Scheme == "https" && u.Port() == "" && (u.Hostname() == "web.facebook.com" || u.Hostname() == "www.facebook.com" || u.Hostname() == "facebook.com")
	}
	host := a.Scheme == b.Scheme && a.Host == b.Host
	if facebook(a) && facebook(b) {
		host = true
	}
	return host && strings.TrimRight(a.Path, "/") == strings.TrimRight(b.Path, "/") && (b.RawQuery == "" || a.RawQuery == b.RawQuery)
}
func marketplaceGraphQL(endpoint *url.URL, target string) bool {
	origin, err := url.Parse(target)
	if err == nil && origin.Scheme == "https" && (origin.Hostname() == "web.facebook.com" || origin.Hostname() == "www.facebook.com" || origin.Hostname() == "facebook.com") {
		return isFacebookPostURL(endpoint)
	}
	return err == nil && endpoint.User == nil && origin.Scheme == "http" && (origin.Hostname() == "127.0.0.1" || origin.Hostname() == "localhost") && endpoint.Scheme == origin.Scheme && endpoint.Host == origin.Host && endpoint.Path == "/api/graphql/"
}

var errMarketplaceLocationBrowserClosed = errors.New("The Facebook window was closed")

type marketplaceLocationCaptureResult struct {
	request marketplaceLocationRequest
	err     error
}
type marketplaceLocationCapture struct {
	results chan marketplaceLocationCaptureResult
	cancel  context.CancelFunc
	done    chan struct{}
}

func (c *marketplaceLocationCapture) stop() { c.cancel(); <-c.done }
func startMarketplaceLocationCapture(ctx context.Context, browserPage *rod.Page, target, query string) *marketplaceLocationCapture {
	ctx, cancel := context.WithCancel(ctx)
	page := browserPage.Context(ctx)
	capture := &marketplaceLocationCapture{make(chan marketplaceLocationCaptureResult, 1), cancel, make(chan struct{})}
	buffer := 8 * 1024 * 1024
	if err := (proto.NetworkEnable{MaxResourceBufferSize: &buffer, MaxTotalBufferSize: &buffer, MaxPostDataSize: &buffer}).Call(page); err != nil {
		capture.results <- marketplaceLocationCaptureResult{err: errMarketplaceLocationConnection}
		close(capture.done)
		return capture
	}
	var candidate *marketplaceLocationRequest
	var id proto.NetworkRequestID
	var finished bool
	var status int
	extras := map[proto.NetworkRequestID]proto.NetworkHeaders{}
	deliver := func(err error) bool {
		result := marketplaceLocationCaptureResult{err: err}
		if candidate != nil {
			result.request = *candidate
		}
		capture.results <- result
		return true
	}
	complete := func() bool {
		if candidate == nil || !finished || extras[id] == nil || status == 0 {
			return false
		}
		candidate.Headers = mergeReplayHeaders(candidate.Headers, extras[id])
		if status >= 500 || status == 408 || status == 429 {
			return deliver(errMarketplaceLocationTemporary)
		}
		if status != 200 || cookieUser(candidate.Headers.Get("Cookie")) != candidate.User {
			return deliver(errMarketplaceLocationConnection)
		}
		response, err := (proto.NetworkGetResponseBody{RequestID: id}).Call(page)
		if err != nil {
			return deliver(errMarketplaceLocationConnection)
		}
		body := []byte(response.Body)
		if response.Base64Encoded {
			body, err = base64.StdEncoding.DecodeString(response.Body)
		}
		if err != nil || len(body) > buffer {
			return deliver(errMarketplaceLocationConnection)
		}
		if _, err = parseMarketplaceLocations(body); err != nil {
			return deliver(err)
		}
		login, err := page.Eval(facebookLoginSelectorScript)
		if err != nil || login.Value.Str() != "logged_in" {
			return deliver(errFacebookLoginRequired)
		}
		return deliver(nil)
	}
	wait := page.EachEvent(func(event *proto.NetworkRequestWillBeSent) bool {
		r := event.Request
		if r == nil || r.Method != http.MethodPost || !sameMarketplaceItemPage(event.DocumentURL, target) {
			return false
		}
		endpoint, err := url.Parse(r.URL)
		if err != nil || !marketplaceGraphQL(endpoint, target) {
			return false
		}
		body, err := (proto.NetworkGetRequestPostData{RequestID: event.RequestID}).Call(page)
		if err != nil {
			return false
		}
		if len(body.PostData) > buffer {
			return false
		}
		form, err := url.ParseQuery(body.PostData)
		if err != nil || form.Get("fb_api_req_friendly_name") != marketplaceAddressQuery {
			return false
		}
		user := form.Get("__user")
		if user == "" || user == "0" {
			return false
		}
		if form.Get("av") != "" && form.Get("av") != user || form.Get("doc_id") == "" {
			return deliver(errMarketplaceLocationConnection)
		}
		var variables struct {
			Params *struct {
				Caller     string  `json:"caller"`
				SearchType string  `json:"search_type"`
				Query      *string `json:"query"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(form.Get("variables")), &variables) != nil || variables.Params == nil || variables.Params.Caller != "MARKETPLACE" || variables.Params.SearchType != "PLACE_TYPEAHEAD" || variables.Params.Query == nil {
			return deliver(errMarketplaceLocationConnection)
		}
		if *variables.Params.Query != query {
			return false
		}
		finished, status = false, 0
		id = event.RequestID
		candidate = &marketplaceLocationRequest{r.URL, body.PostData, user, mergeReplayHeaders(nil, r.Headers)}
		return complete()
	}, func(event *proto.NetworkRequestWillBeSentExtraInfo) bool {
		if len(extras) >= 128 {
			for key := range extras {
				delete(extras, key)
				break
			}
		}
		extras[event.RequestID] = event.Headers
		return complete()
	}, func(event *proto.NetworkResponseReceived) bool {
		if candidate != nil && event.RequestID == id && event.Response != nil {
			status = event.Response.Status
			return complete()
		}
		return false
	}, func(event *proto.NetworkLoadingFinished) bool {
		if candidate != nil && event.RequestID == id {
			finished = true
			return complete()
		}
		return false
	}, func(event *proto.NetworkLoadingFailed) bool {
		if candidate != nil && event.RequestID == id {
			return deliver(errMarketplaceLocationTemporary)
		}
		return false
	})
	go func() { defer close(capture.done); wait() }()
	return capture
}

const marketplaceLocationInputSelector = `input[role="combobox"][type="text"][aria-autocomplete="list"][autocomplete="off"]`

func focusMarketplaceLocation(ctx context.Context, browserPage *rod.Page, query string) (bool, error) {
	probe, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	page := browserPage.Context(probe)
	elements, err := page.Elements(marketplaceLocationInputSelector)
	if err != nil {
		return false, nil
	}
	defer func() {
		for _, el := range elements {
			_ = el.Context(ctx).Release()
		}
	}()
	var visible, named []*rod.Element
	for _, el := range elements {
		if shown, err := el.Visible(); err == nil && shown {
			info, err := el.Eval(`function(){return {hidden:!!this.closest('[hidden],[aria-hidden="true"],header,nav,[role="banner"]'),named:this.getAttribute('aria-label')==='Location'||!!this.closest('label')?.querySelector('svg')}}`)
			if err == nil && !info.Value.Get("hidden").Bool() {
				visible = append(visible, el)
				if info.Value.Get("named").Bool() {
					named = append(named, el)
				}
			}
		}
	}
	if len(named) > 0 {
		visible = named
	}
	if len(visible) == 0 {
		return false, nil
	}
	if len(visible) > 1 {
		return false, errors.New("Could not identify the Marketplace location field")
	}
	if facebookLoginBlocked(page) {
		return false, errFacebookLoginRequired
	}
	el := visible[0]
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return false, nil
	}
	if err := el.SelectAllText(); err != nil {
		return false, nil
	}
	if err := el.Type(input.Backspace); err != nil {
		return false, nil
	}
	if err := el.Input(query); err != nil {
		return false, nil
	}
	return true, nil
}
func captureMarketplaceLocation(ctx context.Context, page *rod.Page, target, query string) (*marketplaceLocationReplay, error) {
	job, stop := context.WithCancel(ctx)
	defer stop()
	capture := startMarketplaceLocationCapture(job, page, target, query)
	defer capture.stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	focused := false
	for {
		select {
		case result := <-capture.results:
			if result.err != nil {
				return nil, result.err
			}
			cookies, err := (proto.NetworkGetCookies{Urls: []string{result.request.URL}}).Call(page.Context(job))
			if err != nil {
				return nil, errMarketplaceLocationConnection
			}
			return newMarketplaceLocationReplay(result.request, cookies.Cookies)
		case <-job.Done():
			return nil, job.Err()
		case <-ticker.C:
			probe, stopProbe := context.WithTimeout(job, 2*time.Second)
			_, tabErr := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(probe))
			if tabErr != nil {
				stopProbe()
				if job.Err() != nil {
					continue
				}
				if errors.Is(tabErr, context.DeadlineExceeded) {
					continue
				}
				return nil, errMarketplaceLocationBrowserClosed
			}
			login, loginErr := page.Context(probe).Eval(facebookLoginSelectorScript)
			stopProbe()
			if loginErr == nil && (login.Value.Str() == "logged_out" || login.Value.Str() == "login_required") {
				return nil, errFacebookLoginRequired
			}
			if !focused {
				var err error
				focused, err = focusMarketplaceLocation(job, page, query)
				if err != nil {
					return nil, err
				}
			}
		}
	}
}

// Setup follows the user's cancellable operation, not the connection speed.
// Keep a verified capture while retrying a temporarily failed replay.
func prepareMarketplaceLocationReplay(ctx context.Context, page *rod.Page, target, query string) (*marketplaceLocationReplay, error) {
	var replay *marketplaceLocationReplay
	delay := time.Second
	for {
		var err error
		if replay == nil {
			replay, err = captureMarketplaceLocation(ctx, page, target, query)
		}
		if err == nil {
			err = verifyMarketplaceLocationReplay(ctx, page, replay, query)
		}
		if !errors.Is(err, errMarketplaceLocationTemporary) {
			return replay, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		delay = min(delay*2, 5*time.Second)
	}
}

// A slow HTTP verification must still stop when the user closes Facebook.
func verifyMarketplaceLocationReplay(ctx context.Context, page *rod.Page, replay *marketplaceLocationReplay, query string) error {
	job, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := replay.search(job, query); result <- err }()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			probe, stop := context.WithTimeout(job, 2*time.Second)
			_, err := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(probe))
			stop()
			if err != nil && !errors.Is(err, context.DeadlineExceeded) && job.Err() == nil {
				return errMarketplaceLocationBrowserClosed
			}
		}
	}
}
