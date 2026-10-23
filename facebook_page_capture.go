package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const facebookTimelineQuery = "ProfileCometTimelineFeedRefetchQuery"
const facebookPhotoQuery = "CometPhotoRootContentQuery"

type timelineRequest struct {
	URL        string
	Body       string
	Headers    http.Header
	User       string
	PhotoQuery *timelineRequest
	Sequence   uint64
}
type timelineCaptureResult struct {
	request timelineRequest
	err     error
}
type timelineCapture struct {
	results chan timelineCaptureResult
	cancel  context.CancelFunc
	done    chan struct{}
}

// Follow Facebook's document redirects (for example profile.php?id=... to a
// page username) without accepting a timeline from unrelated manual navigation.
type timelineLocation struct {
	mu       sync.Mutex
	provided string
	current  string
}

func (location *timelineLocation) matches(document string) bool {
	location.mu.Lock()
	defer location.mu.Unlock()
	return sameTimelinePage(document, location.current)
}

func (location *timelineLocation) redirect(previous, next string) {
	location.mu.Lock()
	defer location.mu.Unlock()
	if !sameTimelinePage(previous, location.current) && !sameTimelinePage(previous, location.provided) {
		return
	}
	if _, err := facebookPageURL(next); err != nil {
		return
	}
	location.current = next
}

func startTimelineCapture(ctx context.Context, page *rod.Page, location *timelineLocation, match func(*url.URL) bool) *timelineCapture {
	return startFacebookQueryCapture(ctx, page, location, match, "", "", "")
}

func startFacebookQueryCapture(ctx context.Context, page *rod.Page, location *timelineLocation, match func(*url.URL) bool, photoID, user, mediaset string) *timelineCapture {
	query := facebookTimelineQuery
	if photoID != "" {
		query = facebookPhotoQuery
	}
	ctx, cancel := context.WithCancel(ctx)
	page = page.Context(ctx)
	capture := &timelineCapture{results: make(chan timelineCaptureResult, 1), cancel: cancel, done: make(chan struct{})}
	buffer := 32 * 1024 * 1024
	if err := (proto.NetworkEnable{MaxResourceBufferSize: &buffer, MaxTotalBufferSize: &buffer, MaxPostDataSize: &buffer}).Call(page); err != nil {
		capture.results <- timelineCaptureResult{err: errors.New("Could not enable Facebook request capture in Chrome")}
		close(capture.done)
		return capture
	}
	var candidate *timelineRequest
	var candidatePhotoID string
	var id proto.NetworkRequestID
	var finished bool
	var status int
	extras := map[proto.NetworkRequestID]proto.NetworkHeaders{}
	deliver := func(err error) bool {
		result := timelineCaptureResult{err: err}
		if candidate != nil {
			result.request = *candidate
		}
		capture.results <- result
		return true
	}
	complete := func() bool {
		if candidate == nil || !finished || extras[id] == nil {
			return false
		}
		candidate.Headers = mergeReplayHeaders(candidate.Headers, extras[id])
		if cookieUser(candidate.Headers.Get("Cookie")) != candidate.User {
			return deliver(errors.New("The captured Facebook session does not match the logged-in account. Fetch again"))
		}
		if status != http.StatusOK {
			return deliver(errors.New("Facebook's captured request failed. Check Chrome and fetch again"))
		}
		response, err := (proto.NetworkGetResponseBody{RequestID: id}).Call(page)
		if err != nil {
			return deliver(errors.New("Could not read the browser's captured response"))
		}
		body := []byte(response.Body)
		if response.Base64Encoded {
			body, err = base64.StdEncoding.DecodeString(response.Body)
		}
		if err != nil || len(body) > buffer {
			return deliver(errors.New("Could not read the browser's captured response"))
		}
		if photoID != "" {
			if _, err := readFacebookPhotoResponse(strings.NewReader(string(body)), candidatePhotoID); err != nil {
				return deliver(errors.New("Could not read the browser photo-viewer response. Fetch again"))
			}
		} else if _, _, _, err := parseFacebookPageResponse(body); err != nil {
			return deliver(err)
		}
		return deliver(nil)
	}
	wait := page.EachEvent(func(event *proto.NetworkRequestWillBeSent) bool {
		r := event.Request
		if location != nil && r != nil && event.Type == proto.NetworkResourceTypeDocument && event.RedirectResponse != nil {
			location.redirect(event.RedirectResponse.URL, r.URL)
		}
		if candidate != nil || r == nil || r.Method != http.MethodPost {
			return false
		}
		u, err := url.Parse(r.URL)
		if err != nil || !match(u) {
			return false
		}
		// Do not capture a timeline from another tab/page the user navigated to.
		if photoID == "" && !location.matches(event.DocumentURL) {
			return false
		}
		data, err := (proto.NetworkGetRequestPostData{RequestID: event.RequestID}).Call(page)
		if err != nil {
			if mergeReplayHeaders(nil, r.Headers).Get("X-Fb-Friendly-Name") == query {
				return deliver(errors.New("Could not capture the complete Facebook request"))
			}
			return false
		}
		form, err := url.ParseQuery(data.PostData)
		if err != nil || form.Get("fb_api_req_friendly_name") != query {
			return false
		}
		var variables struct {
			ID       string `json:"id"`
			Location string `json:"feedLocation"`
			NodeID   string `json:"nodeID"`
			Mediaset string `json:"mediasetToken"`
		}
		if json.Unmarshal([]byte(form.Get("variables")), &variables) != nil || form.Get("doc_id") == "" || form.Get("__user") == "" {
			return deliver(errors.New("Facebook's captured request was incomplete. Fetch again"))
		}
		if photoID != "" {
			// Facebook often requests the next gallery photo while the clicked
			// photo is already prefetched. Any photo in this post's collection
			// supplies the same usable query template.
			if !decimalID(variables.NodeID) || mediaset != "" && variables.Mediaset != mediaset {
				return false
			}
			candidatePhotoID = variables.NodeID
			if form.Get("__user") != user {
				return deliver(errors.New("The Facebook session changed before photo capture. Fetch again"))
			}
		} else if variables.ID == "" || variables.Location != "TIMELINE" {
			return deliver(errors.New("Facebook's timeline request was incomplete. Fetch again"))
		}
		id = event.RequestID
		candidate = &timelineRequest{URL: r.URL, Body: data.PostData, Headers: mergeReplayHeaders(nil, r.Headers), User: form.Get("__user")}
		return complete()
	}, func(event *proto.NetworkRequestWillBeSentExtraInfo) bool {
		if candidate != nil && event.RequestID != id {
			return false
		}
		if len(extras) >= 256 {
			for key := range extras {
				delete(extras, key)
				break
			}
		}
		extras[event.RequestID] = event.Headers
		return complete()
	}, func(event *proto.NetworkResponseReceived) {
		if candidate != nil && event.RequestID == id && event.Response != nil {
			status = event.Response.Status
		}
	}, func(event *proto.NetworkLoadingFinished) bool {
		if candidate != nil && event.RequestID == id {
			finished = true
			return complete()
		}
		return false
	}, func(event *proto.NetworkLoadingFailed) bool {
		if candidate != nil && event.RequestID == id {
			return deliver(errors.New("The browser's captured request failed. Check Chrome and fetch again"))
		}
		return false
	})
	go func() { defer close(capture.done); wait() }()
	return capture
}

func (capture *timelineCapture) stop() { capture.cancel(); <-capture.done }

func sameTimelinePage(document, target string) bool {
	a, errA := url.Parse(document)
	b, errB := url.Parse(target)
	if errA != nil || errB != nil {
		return false
	}
	// Local HTTP fixture origins are also compared exactly in integration tests.
	sameHost := a.Host == b.Host
	if strings.HasSuffix(a.Hostname(), ".facebook.com") || a.Hostname() == "facebook.com" {
		x, ex := facebookPageURL(document)
		y, ey := facebookPageURL(target)
		if ex != nil || ey != nil {
			return false
		}
		a, _ = url.Parse(x)
		b, _ = url.Parse(y)
		sameHost = a.Host == b.Host
	}
	return sameHost && strings.TrimRight(a.Path, "/") == strings.TrimRight(b.Path, "/") && a.Query().Get("id") == b.Query().Get("id")
}
