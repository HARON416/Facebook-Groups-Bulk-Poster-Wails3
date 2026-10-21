package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Private session data: never serialized into bindings, curl files or logs.
type capturedPhotoUpload struct {
	URL            string
	Headers        http.Header
	PostData       string // CDP omits binary file contents; this is NOT a replayable body.
	Fields         url.Values
	Files          []photoUploadFile
	User           string
	Status         int
	Response       []byte
	wire, finished bool
}
type photoUploadFile struct{ Field, Filename, ContentType string }
type photoUploadCapture struct {
	mu       sync.Mutex
	requests map[proto.NetworkRequestID]*capturedPhotoUpload
	extras   map[proto.NetworkRequestID]proto.NetworkHeaders
	changed  chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
	err      error
}

func isFacebookPhotoUploadURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "upload.facebook.com" && u.Path == "/ajax/react_composer/attachments/photo/upload"
}
func captureUploadHeaders(base http.Header, extra proto.NetworkHeaders) http.Header {
	headers := base.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	for key, value := range extra {
		if !strings.HasPrefix(key, ":") {
			headers.Set(key, value.Str())
		}
	}
	return headers
}
func uploadMultipart(postData, contentType string) (url.Values, []photoUploadFile, error) {
	media, parameters, err := mime.ParseMediaType(contentType)
	if err != nil || media != "multipart/form-data" || parameters["boundary"] == "" {
		return nil, nil, errors.New("The image upload did not contain multipart form data")
	}
	fields := make(url.Values)
	files := []photoUploadFile{}
	reader := multipart.NewReader(strings.NewReader(postData), parameters["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return fields, files, nil
		}
		if err != nil {
			return nil, nil, errors.New("Could not capture the image upload form fields")
		}
		if filename := part.FileName(); filename != "" {
			files = append(files, photoUploadFile{Field: part.FormName(), Filename: filename, ContentType: part.Header.Get("Content-Type")})
			_, err = io.Copy(io.Discard, part)
		} else {
			var value []byte
			value, err = io.ReadAll(io.LimitReader(part, 1024*1024+1))
			if len(value) > 1024*1024 {
				err = errors.New("upload field too large")
			}
			if err == nil {
				fields.Add(part.FormName(), string(value))
			}
		}
		part.Close()
		if err != nil {
			return nil, nil, errors.New("Could not capture the image upload form fields")
		}
	}
}

func startPhotoUploadCapture(ctx context.Context, page *rod.Page, match func(string) bool) *photoUploadCapture {
	ctx, cancel := context.WithCancel(ctx)
	page = page.Context(ctx)
	capture := &photoUploadCapture{
		requests: map[proto.NetworkRequestID]*capturedPhotoUpload{},
		extras:   map[proto.NetworkRequestID]proto.NetworkHeaders{},
		changed:  make(chan struct{}, 1), cancel: cancel, done: make(chan struct{}),
	}
	signal := func() {
		select {
		case capture.changed <- struct{}{}:
		default:
		}
	}
	fail := func(err error) {
		capture.mu.Lock()
		if capture.err == nil {
			capture.err = err
		}
		capture.mu.Unlock()
		signal()
	}
	wait := page.EachEvent(func(event *proto.NetworkRequestWillBeSent) {
		request := event.Request
		if request == nil || !match(request.URL) || request.Method != http.MethodPost {
			return
		}
		headers := captureUploadHeaders(nil, request.Headers)
		if headers.Get("Access-Control-Request-Method") != "" {
			return
		}
		u, err := url.Parse(request.URL)
		if err != nil {
			fail(errors.New("Could not capture the upload endpoint"))
			return
		}
		user := u.Query().Get("__user")
		if user == "" || (u.Query().Get("av") != "" && u.Query().Get("av") != user) {
			fail(errors.New("The image upload does not match its active Facebook profile"))
			return
		}
		body := request.PostData
		full, err := (proto.NetworkGetRequestPostData{RequestID: event.RequestID}).Call(page)
		if err != nil {
			fail(errors.New("Could not capture the image upload form. Retry Post"))
			return
		}
		body = full.PostData
		fields, files, err := uploadMultipart(body, headers.Get("Content-Type"))
		if err != nil {
			fail(err)
			return
		}
		if owner := fields.Get("profile_id"); owner != "" && owner != user {
			fail(errors.New("The image upload form belongs to a different Facebook profile"))
			return
		}
		entry := &capturedPhotoUpload{URL: request.URL, Headers: headers, PostData: body, Fields: fields, Files: files, User: user}
		capture.mu.Lock()
		if extra := capture.extras[event.RequestID]; extra != nil {
			entry.Headers = captureUploadHeaders(entry.Headers, extra)
			entry.wire = true
			delete(capture.extras, event.RequestID)
		}
		capture.requests[event.RequestID] = entry
		capture.mu.Unlock()
		signal()
	}, func(event *proto.NetworkRequestWillBeSentExtraInfo) {
		capture.mu.Lock()
		if entry := capture.requests[event.RequestID]; entry != nil {
			entry.Headers = captureUploadHeaders(entry.Headers, event.Headers)
			entry.wire = true
		} else {
			if len(capture.extras) >= 256 {
				for id := range capture.extras {
					delete(capture.extras, id)
					break
				}
			}
			capture.extras[event.RequestID] = event.Headers
		}
		capture.mu.Unlock()
		signal()
	}, func(event *proto.NetworkResponseReceived) {
		capture.mu.Lock()
		if entry := capture.requests[event.RequestID]; entry != nil && event.Response != nil {
			entry.Status = event.Response.Status
		}
		capture.mu.Unlock()
	}, func(event *proto.NetworkLoadingFinished) {
		capture.mu.Lock()
		entry := capture.requests[event.RequestID]
		capture.mu.Unlock()
		if entry == nil {
			return
		}
		// Keep the response privately for later photo-ID parsing, not in the UI.
		response, err := (proto.NetworkGetResponseBody{RequestID: event.RequestID}).Call(page)
		if err != nil {
			fail(errors.New("Could not capture the completed image upload response"))
			return
		}
		data := []byte(response.Body)
		if response.Base64Encoded {
			data, err = base64.StdEncoding.DecodeString(response.Body)
		}
		if err != nil || len(data) > 16*1024*1024 {
			fail(errors.New("Could not read the completed image upload response"))
			return
		}
		capture.mu.Lock()
		entry.Response = data
		entry.finished = true
		capture.mu.Unlock()
		signal()
	}, func(event *proto.NetworkLoadingFailed) {
		capture.mu.Lock()
		entry := capture.requests[event.RequestID]
		capture.mu.Unlock()
		if entry != nil {
			fail(errors.New("An image upload failed in Chrome. Check the composer and retry Post"))
		}
	})
	go func() { defer close(capture.done); wait() }()
	return capture
}
func (capture *photoUploadCapture) stop() { capture.cancel(); <-capture.done }
func (capture *photoUploadCapture) wait(ctx context.Context, expected int, progress func(string)) (int, error) {
	for {
		capture.mu.Lock()
		err := capture.err
		complete := 0
		for _, entry := range capture.requests {
			if !entry.wire || !entry.finished {
				continue
			}
			if cookieUser(entry.Headers.Get("Cookie")) != entry.User {
				err = errors.New("The captured image upload cookies do not match the Facebook profile")
				break
			}
			if entry.Status < 200 || entry.Status >= 300 {
				err = fmt.Errorf("Chrome image upload returned HTTP %d", entry.Status)
				break
			}
			complete++
		}
		capture.mu.Unlock()
		if err != nil {
			return complete, err
		}
		if complete >= expected {
			return complete, nil
		}
		progress(fmt.Sprintf("Capturing image uploads (%d of %d)…", complete, expected))
		select {
		case <-ctx.Done():
			return complete, ctx.Err()
		case <-capture.changed:
		}
	}
}
