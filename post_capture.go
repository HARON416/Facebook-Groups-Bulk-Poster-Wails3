package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const createPostMutation = "ComposerStoryCreateMutation"

type capturedPost struct {
	URL      string
	Headers  http.Header
	Body     string
	User     string
	Response []byte
	Status   int
}
type postCaptureResult struct {
	request *capturedPost
	err     error
}
type postCapture struct {
	results        chan postCaptureResult
	cancel         context.CancelFunc
	done           chan struct{}
	requestStarted atomic.Bool
	progress       chan string
	startErr       error
}

func startPostCapture(ctx context.Context, page *rod.Page, groupID string) *postCapture {
	return startPostCaptureMatching(ctx, page, groupID, func(u *url.URL) bool {
		return isFacebookPostURL(u)
	})
}

func isFacebookPostURL(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && u.Port() == "" &&
		(u.Hostname() == "web.facebook.com" || u.Hostname() == "www.facebook.com" || u.Hostname() == "facebook.com") &&
		(u.Path == "/api/graphql/" || u.Path == "/api/graphql")
}

func startPostCaptureMatching(ctx context.Context, page *rod.Page, groupID string, match func(*url.URL) bool) *postCapture {
	ctx, cancel := context.WithCancel(ctx)
	page = page.Context(ctx)
	capture := &postCapture{results: make(chan postCaptureResult, 1), cancel: cancel, done: make(chan struct{}), progress: make(chan string, 8)}
	progress := func(message string) {
		select {
		case capture.progress <- message:
		default:
		}
	}
	// Explicitly arm Network before clicking. EachEvent's implicit enable ignores
	// protocol errors; a failed listener must not leave a submitted post waiting.
	buffer := 16 * 1024 * 1024
	if err := (proto.NetworkEnable{MaxResourceBufferSize: &buffer, MaxTotalBufferSize: &buffer, MaxPostDataSize: &buffer}).Call(page); err != nil {
		capture.startErr = errors.New("Could not enable create-post capture in Chrome")
		capture.results <- postCaptureResult{err: capture.startErr}
		close(capture.done)
		return capture
	}
	var candidate *capturedPost
	var id proto.NetworkRequestID
	extras := map[proto.NetworkRequestID]proto.NetworkHeaders{}
	finished := false
	deliver := func(err error) bool {
		select {
		case capture.results <- postCaptureResult{candidate, err}:
		default:
		}
		return true
	}
	complete := func() bool {
		if candidate == nil || !finished || extras[id] == nil {
			return false
		}
		progress("Reading Facebook's create-post confirmation…")
		candidate.Headers = captureUploadHeaders(candidate.Headers, extras[id])
		if cookieUser(candidate.Headers.Get("Cookie")) != candidate.User {
			return deliver(errors.New("The captured post cookies do not match the logged-in profile"))
		}
		response, err := (proto.NetworkGetResponseBody{RequestID: id}).Call(page)
		if err != nil {
			return deliver(errors.New("Could not read Facebook's post confirmation"))
		}
		candidate.Response = []byte(response.Body)
		if response.Base64Encoded {
			candidate.Response, err = base64.StdEncoding.DecodeString(response.Body)
		}
		if err != nil || len(candidate.Response) > 16*1024*1024 {
			return deliver(errors.New("Could not read Facebook's post confirmation"))
		}
		return deliver(nil)
	}
	wait := page.EachEvent(func(event *proto.NetworkRequestWillBeSent) bool {
		r := event.Request
		if candidate != nil || r == nil || r.Method != http.MethodPost {
			return false
		}
		u, err := url.Parse(r.URL)
		if err != nil || !match(u) {
			return false
		}
		headers := captureUploadHeaders(nil, r.Headers)
		headerMatch := headers.Get("X-Fb-Friendly-Name") == createPostMutation
		if headerMatch {
			capture.requestStarted.Store(true)
			progress("Create-post request detected. Capturing fresh form values…")
		}
		data, err := (proto.NetworkGetRequestPostData{RequestID: event.RequestID}).Call(page)
		if err != nil {
			if headerMatch {
				return deliver(errors.New("Could not capture the complete create-post request"))
			}
			return false
		}
		form, err := url.ParseQuery(data.PostData)
		if err != nil {
			if headerMatch {
				return deliver(errors.New("Could not parse the captured create-post form"))
			}
			return false
		}
		if !headerMatch && form.Get("fb_api_req_friendly_name") != createPostMutation {
			return false
		}
		capture.requestStarted.Store(true)
		var variables map[string]json.RawMessage
		var input struct {
			Actor    string `json:"actor_id"`
			Audience struct {
				To string `json:"to_id"`
			} `json:"audience"`
		}
		if json.Unmarshal([]byte(form.Get("variables")), &variables) != nil || json.Unmarshal(variables["input"], &input) != nil || input.Audience.To != groupID {
			return deliver(errors.New("The captured post target does not match the selected group"))
		}
		user := form.Get("__user")
		if user == "" || input.Actor != user || form.Get("av") != "" && form.Get("av") != user {
			return deliver(errors.New("The captured post belongs to a different Facebook profile"))
		}
		id = event.RequestID
		candidate = &capturedPost{URL: r.URL, Headers: captureUploadHeaders(nil, r.Headers), Body: data.PostData, User: user}
		progress("Fresh create-post form captured. Waiting for Facebook's response…")
		return complete()
	}, func(event *proto.NetworkRequestWillBeSentExtraInfo) bool {
		if len(extras) >= 256 {
			for k := range extras {
				if k != id {
					delete(extras, k)
					break
				}
			}
		}
		extras[event.RequestID] = event.Headers
		return complete()
	}, func(event *proto.NetworkResponseReceived) {
		if candidate != nil && event.RequestID == id && event.Response != nil {
			candidate.Status = event.Response.Status
			progress("Create-post response received. Waiting for its complete body…")
		}
	}, func(event *proto.NetworkLoadingFinished) bool {
		if candidate == nil || event.RequestID != id {
			return false
		}
		finished = true
		if extras[id] == nil {
			progress("Create-post response complete. Waiting for captured session headers…")
		}
		return complete()
	}, func(event *proto.NetworkLoadingFailed) bool {
		if candidate != nil && event.RequestID == id {
			if extras[id] != nil {
				candidate.Headers = captureUploadHeaders(candidate.Headers, extras[id])
			}
			return deliver(errors.New("The create-post connection failed. Check Facebook before retrying"))
		}
		return false
	})
	go func() { defer close(capture.done); wait() }()
	return capture
}
func (capture *postCapture) stop() { capture.cancel(); <-capture.done }

// The extension uses composer disappearance as its UI completion signal. Here
// it complements explicit API confirmation rather than being sufficient alone.
func waitPostedComposerClosed(ctx context.Context, page *rod.Page) error {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := page.Context(ctx).Eval(`() => {
		 const dialogs=[...document.querySelectorAll('[role="dialog"][aria-modal="true"]')].filter(e=>e.getClientRects().length && e.querySelector('[contenteditable="true"][data-lexical-editor="true"]'));
		 if (!dialogs.length) return 'closed';
		 return 'waiting';
		}`)
		if err != nil {
			return errors.New("The first post was confirmed, but Chrome's completion state could not be checked. Posting stopped")
		}
		if result.Value.Str() == "closed" {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("Posting stopped after the first post was confirmed")
		case <-ticker.C:
		}
	}
}

// API confirmation determines acceptance and pending approval. Live regions can
// also contain success/approval notices; their existence never proves failure.
// Only a recognized restriction is a fallback before request capture begins.
func (capture *postCapture) wait(ctx context.Context, page *rod.Page, progress ...func(string)) (*capturedPost, error) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case result := <-capture.results:
			return result.request, result.err
		case message := <-capture.progress:
			if len(progress) > 0 {
				progress[0](message)
			}
		case <-capture.done:
			select {
			case result := <-capture.results:
				return result.request, result.err
			default:
				return nil, errors.New("Create-post capture ended before confirmation. Check Facebook before retrying")
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			result, err := page.Context(ctx).Eval(`() => [...document.querySelectorAll('[role="dialog"] [role="alert"], [role="dialog"] [aria-live="assertive"]')].filter(e=>e.getClientRects().length).map(e=>e.textContent).join(' ')`)
			if err != nil {
				return nil, errors.New("The browser connection was lost. Check Facebook before retrying")
			}
			text := result.Value.Str()
			if text != "" {
				// An alert can render before LoadingFinished delivers the body.
				// Wait for the in-flight mutation's response and its error codes.
				if capture.requestStarted.Load() {
					continue
				}
				// Do not expose arbitrary Facebook DOM text, which may contain private data.
				payload, _ := json.Marshal(map[string]any{"errorSummary": text})
				outcome := parsePostResponse(payload, 200)
				if outcome.State == "failed" && outcome.stop && outcome.Message != "Facebook rejected this post." {
					return nil, errors.New(outcome.Message)
				}
			}
		}
	}
}
