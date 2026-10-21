package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReplayConnectionRetries(t *testing.T) {
	for _, phase := range []string{"post", "upload"} {
		for _, mode := range []string{"recover", "exhausted", "cancel"} {
			t.Run(phase+"/"+mode, func(t *testing.T) {
				endpoint, _ := url.Parse("https://web.facebook.com/api/graphql/")
				jar, _ := cookiejar.New(nil)
				jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				attempts, retryLogs := 0, 0
				var firstBody string
				replay := &postReplay{
					user:   "fixture-user",
					post:   capturedPost{URL: endpoint.String(), Body: postFixtureForm(), Headers: http.Header{}},
					upload: capturedPhotoUpload{URL: endpoint.String(), Headers: http.Header{}, Files: []photoUploadFile{{Field: "photo"}}},
					progress: func(message string) {
						retryLogs++
						if !strings.Contains(message, "Retrying request") {
							t.Errorf("missing retry progress: %s", message)
						}
						if mode == "cancel" {
							cancel()
						}
					},
				}
				replay.client = &http.Client{Jar: jar, Transport: diagnosticTransport(func(request *http.Request) (*http.Response, error) {
					attempts++
					body, _ := io.ReadAll(request.Body)
					if attempts == 1 {
						firstBody = string(body)
					}
					if attempts <= 4 && string(body) != firstBody {
						t.Error("retry changed the request or mutation ID")
					}
					if mode != "recover" && attempts <= 4 || mode == "recover" && attempts <= 3 {
						return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("i/o timeout")}
					}
					response := `{"data":{"story_create":{"post_id":"created","story":{"to":{"id":"target","if_viewer_can_see_pending_post_in_mall":null},"if_viewer_can_learn_more_about_pending_post":null}}}}`
					if phase == "upload" {
						response = `{"__ar":1,"payload":{"photoID":"photo"}}`
					}
					return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response)), Request: request}, nil
				})}
				perform := func() PostOutcome {
					if phase == "post" {
						return replay.create(ctx, "First\nSecond", []string{"photo"}, "target")
					}
					_, outcome := replay.uploadImage(ctx, PostUpload{Name: "image.png", Data: fixturePNG})
					return outcome
				}
				outcome := perform()
				if mode == "cancel" {
					if attempts != 1 || retryLogs != 1 || !outcome.stop {
						t.Fatal("cancellation did not stop retries")
					}
					return
				}
				if attempts != 4 || retryLogs != 3 {
					t.Fatalf("attempts=%d retry logs=%d", attempts, retryLogs)
				}
				if mode == "exhausted" {
					if outcome.stop || outcome.State != "failed" || !strings.Contains(outcome.Message, "Continuing") || !strings.Contains(outcome.diagnostic, "3 retries") {
						t.Fatalf("exhausted retries stopped the queue: %#v", outcome)
					}
					outcome = perform() // A later group/request can proceed.
				}
				if outcome.stop || phase == "post" && outcome.State != "posted" || phase == "upload" && outcome.State != "" {
					t.Fatalf("recovery failed: %#v", outcome)
				}
			})
		}
	}
}

func TestReplayUnconfirmedRetriesContinue(t *testing.T) {
	for _, failure := range []string{"write", "response", "interrupted", "restriction", "session"} {
		t.Run(failure, func(t *testing.T) {
			endpoint, _ := url.Parse("https://web.facebook.com/api/graphql/")
			jar, _ := cookiejar.New(nil)
			jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
			attempts, retries := 0, 0
			var original []byte
			replay := &postReplay{user: "fixture-user", post: capturedPost{URL: endpoint.String(), Body: postFixtureForm(), Headers: http.Header{}}, progress: func(string) { retries++ }}
			replay.client = &http.Client{Jar: jar, Transport: diagnosticTransport(func(request *http.Request) (*http.Response, error) {
				attempts++
				body, _ := io.ReadAll(request.Body)
				if attempts == 1 {
					original = body
				} else if attempts <= 4 && string(body) != string(original) {
					t.Fatal("retry changed mutation bytes")
				}
				response := &http.Response{StatusCode: 200, Header: http.Header{}, Request: request}
				if attempts <= 4 {
					switch failure {
					case "write":
						return nil, &net.OpError{Op: "write", Net: "tcp", Err: errors.New("connection reset")}
					case "interrupted":
						response.Body = &interruptedResponse{}
					case "restriction":
						response.StatusCode = 429
						response.Body = io.NopCloser(strings.NewReader(`{}`))
					case "session":
						response.StatusCode = 401
						response.Body = io.NopCloser(strings.NewReader(`{}`))
					default:
						response.Body = io.NopCloser(strings.NewReader(`{"data":{"story_create":null}}`))
					}
				} else {
					response.Body = io.NopCloser(strings.NewReader(`{"data":{"story_create":{"post_id":"confirmed","story":{"to":{"id":"next","if_viewer_can_see_pending_post_in_mall":null},"if_viewer_can_learn_more_about_pending_post":null}}}}`))
				}
				return response, nil
			})}
			outcome := replay.create(context.Background(), "First\nSecond", []string{"photo"}, "target")
			if failure == "restriction" || failure == "session" {
				if attempts != 1 || retries != 0 || !outcome.stop {
					t.Fatalf("explicit stop was retried: %#v attempts=%d", outcome, attempts)
				}
				return
			}
			if attempts != 4 || retries != 3 || outcome.stop || outcome.State != "unknown" {
				t.Fatalf("unconfirmed did not exhaust and continue: %#v attempts=%d retries=%d", outcome, attempts, retries)
			}
			outcome = replay.create(context.Background(), "First\nSecond", []string{"photo"}, "next")
			if outcome.State != "posted" || outcome.stop || attempts != 5 {
				t.Fatalf("next group failed: %#v", outcome)
			}
		})
	}
}

func TestCapturedBrowserPostGetsOnlyThreeAdditionalAttempts(t *testing.T) {
	endpoint, _ := url.Parse("https://web.facebook.com/api/graphql/")
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
	attempts := 0
	replay := &postReplay{user: "fixture-user", client: &http.Client{Jar: jar, Transport: diagnosticTransport(func(request *http.Request) (*http.Response, error) {
		attempts++
		body, _ := io.ReadAll(request.Body)
		if string(body) != postFixtureForm() {
			t.Fatal("captured browser request changed")
		}
		return nil, errors.New("connection reset")
	})}}
	_, _, err := replay.sendAttempts(context.Background(), endpoint.String(), http.Header{}, []byte(postFixtureForm()), 1, nil)
	var exhausted *replayRetriesExhausted
	if attempts != 3 || !errors.As(err, &exhausted) || !exhausted.uncertain {
		t.Fatalf("browser retry budget incorrect: attempts=%d error=%v", attempts, err)
	}
}

func TestReplayOutagesDoNotConsumeRetries(t *testing.T) {
	s := &GroupsService{connectivityEnabled: true, connectivity: ConnectivityStatus{Known: true, Online: true}}
	var online atomic.Bool
	online.Store(true)
	s.connectivityProbe = func(context.Context) bool { return online.Load() }
	ctx, cancel := testPostingContext(s)
	defer cancel()
	endpoint, _ := url.Parse("https://web.facebook.com/api/graphql/")
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
	failures := make(chan struct{}, 1)
	attempts := 0
	firstBody := ""
	replay := &postReplay{user: "fixture-user"}
	replay.client = &http.Client{Jar: jar, Transport: diagnosticTransport(func(request *http.Request) (*http.Response, error) {
		attempts++
		body, _ := io.ReadAll(request.Body)
		if attempts == 1 {
			firstBody = string(body)
		}
		if string(body) != firstBody {
			t.Error("reconnection changed the request bytes")
		}
		if attempts <= 5 {
			online.Store(false)
			failures <- struct{}{}
			return nil, &net.OpError{Op: "write", Net: "tcp", Err: errors.New("connection lost")}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("confirmed")), Header: http.Header{}, Request: request}, nil
	})}
	done := make(chan error, 1)
	go func() {
		data, _, err := replay.send(ctx, endpoint.String(), http.Header{}, []byte("unchanged mutation"))
		if err == nil && string(data) != "confirmed" {
			err = errors.New("missing confirmation")
		}
		done <- err
	}()
	for i := 0; i < 5; i++ {
		select {
		case <-failures:
		case <-time.After(2 * time.Second):
			t.Fatal("missing failed request")
		}
		awaitPostingState(t, s, "paused")
		online.Store(true)
		s.checkConnectivity(ctx)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not recover")
	}
	if attempts != 6 {
		t.Fatalf("expected 6 attempts, got %d", attempts)
	}
}
