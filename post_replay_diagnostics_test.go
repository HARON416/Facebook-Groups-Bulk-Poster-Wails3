package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

type diagnosticTransport func(*http.Request) (*http.Response, error)

func (transport diagnosticTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type interruptedResponse struct{ read bool }

func (body *interruptedResponse) Read(data []byte) (int, error) {
	if body.read {
		return 0, io.ErrUnexpectedEOF
	}
	body.read = true
	return copy(data, `{"debug":"partial confirmation"`), nil
}

func (*interruptedResponse) Close() error { return nil }

func TestReplayFailureRetainsDebugDetails(t *testing.T) {
	for _, phase := range []string{"post", "upload"} {
		for _, failure := range []string{"transport", "interrupted", "profile", "oversized"} {
			t.Run(phase+"/"+failure, func(t *testing.T) {
				endpoint, _ := url.Parse("https://web.facebook.com/api/graphql/?fb_dtsg=private-token")
				jar, _ := cookiejar.New(nil)
				jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
				transport := diagnosticTransport(func(request *http.Request) (*http.Response, error) {
					if failure == "transport" {
						return nil, errors.New("connection reset by peer")
					}
					response := &http.Response{StatusCode: 200, Header: http.Header{}, Request: request}
					switch failure {
					case "interrupted":
						response.Body = &interruptedResponse{}
					case "profile":
						response.Header.Set("Set-Cookie", "c_user=changed-profile; Path=/")
						response.Body = io.NopCloser(strings.NewReader(`{"debug":"session changed"}`))
					case "oversized":
						response.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", 16*1024*1024+1)))
					}
					return response, nil
				})
				replay := &postReplay{
					client: &http.Client{Jar: jar, Transport: transport}, user: "fixture-user",
					post:   capturedPost{URL: endpoint.String(), Body: postFixtureForm(), Headers: http.Header{}},
					upload: capturedPhotoUpload{URL: endpoint.String(), Headers: http.Header{}, Files: []photoUploadFile{{Field: "photo"}}},
				}
				var outcome PostOutcome
				if phase == "post" {
					outcome = replay.create(context.Background(), "First\nSecond", []string{"photo"}, "target")
				} else {
					_, outcome = replay.uploadImage(context.Background(), PostUpload{Name: "image.png", Data: fixturePNG})
				}
				if outcome.stop != (failure == "profile") || phase == "post" && outcome.State != "unknown" || phase == "upload" && outcome.State != "failed" {
					t.Fatalf("unexpected failure outcome: %s, stop=%v", outcome.State, outcome.stop)
				}
				service := &GroupsService{postingRunID: 1}
				service.recordOutcome(outcome)
				logs := service.GetPostingLogs(1, 50)
				entry := logs.Entries[len(logs.Entries)-1]
				if entry.Diagnostic == "" || strings.Contains(entry.Diagnostic, "private-token") {
					t.Fatalf("missing diagnostic or exposed request URL: %s", entry.Diagnostic)
				}
				if failure == "transport" {
					if entry.HasResponse || !strings.Contains(entry.Diagnostic, "no HTTP response") || !strings.Contains(entry.Diagnostic, "connection reset by peer") {
						t.Fatalf("transport failure lost: %s", entry.Diagnostic)
					}
					return
				}
				copied, err := service.postingResponse(1, entry.ID)
				if !entry.HasResponse || err != nil || copied == "" || !strings.Contains(entry.Diagnostic, "HTTP 200") {
					t.Fatalf("response not available: %s, %v", entry.Diagnostic, err)
				}
				switch failure {
				case "interrupted":
					if copied != `{"debug":"partial confirmation"` || !strings.Contains(entry.Diagnostic, "unexpected EOF") {
						t.Fatalf("partial response lost: %s, %s", copied, entry.Diagnostic)
					}
				case "profile":
					if !strings.Contains(copied, "session changed") || !strings.Contains(entry.Diagnostic, "profile changed or expired") {
						t.Fatal("session failure details lost")
					}
				case "oversized":
					if len(copied) != 16*1024*1024 || !strings.Contains(entry.Diagnostic, "truncated") {
						t.Fatal("oversized response not bounded and marked truncated")
					}
				}
			})
		}
	}
}
