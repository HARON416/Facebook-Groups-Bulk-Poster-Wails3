package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testPostingContext(s *GroupsService) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	s.postMu.Lock()
	s.posting = PostingStatus{State: "posting"}
	s.postingCancel = cancel
	s.signalPostingLocked()
	s.postMu.Unlock()
	return context.WithValue(ctx, postingCheckpointKey{}, s), cancel
}

func TestAcceptedPostLogsAreInformational(t *testing.T) {
	for _, state := range []string{"pending", "posted"} {
		t.Run(state, func(t *testing.T) {
			service := &GroupsService{}
			service.recordOutcome(PostOutcome{Group: Group{ID: "group", Name: "Example group"}, State: state, Message: "Accepted"})
			logs := service.GetPostingLogs(1, 10)
			if logs.Total != 1 || logs.Entries[0].Level != "info" || len(logs.Status.Results) != 1 || logs.Status.Results[0].State != state {
				t.Fatalf("accepted post was reported as an error: %#v", logs)
			}
			if state == "pending" && !strings.Contains(logs.Entries[0].Message, "Your post is pending") {
				t.Fatal("pending status is missing from the log")
			}
		})
	}
}
func awaitPostingState(t *testing.T, s *GroupsService, state string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s.GetPostingStatus().State == state {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected %s, got %#v", state, s.GetPostingStatus())
}
func TestPostingPauseResumeAndStop(t *testing.T) {
	s := &GroupsService{}
	ctx, cancel := testPostingContext(s)
	defer cancel()
	s.PausePosting()
	done := make(chan error, 1)
	go func() { done <- postingCheckpoint(ctx) }()
	awaitPostingState(t, s, "paused")
	if status := s.GetPostingStatus(); !status.Active || !status.PauseRequested {
		t.Fatal("paused run must remain active")
	}
	select {
	case <-done:
		t.Fatal("pause allowed new work")
	default:
	}
	s.ResumePosting()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("resume did not release work")
	}
	s.PausePosting()
	go func() { done <- postingCheckpoint(ctx) }()
	awaitPostingState(t, s, "paused")
	s.CancelPosting()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("stop did not cancel paused work", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop did not release paused work")
	}
	logs := s.GetPostingLogs(1, 100)
	if len(logs.Entries) != 6 {
		t.Fatalf("expected pause/resume/stop history, got %#v", logs.Entries)
	}
}
func TestPostingOutcomeKeepsSeveritySeparateFromStatus(t *testing.T) {
	for _, test := range []struct{ state, level string }{{"posted", "info"}, {"pending", "info"}, {"skipped", "warning"}, {"failed", "error"}, {"unknown", "error"}} {
		t.Run(test.state, func(t *testing.T) {
			s := &GroupsService{postingRunID: 1}
			s.recordOutcome(PostOutcome{State: test.state, diagnostic: "API confirmation: HTTP 200", response: `{"data":{}}`})
			logs := s.GetPostingLogs(1, 50).Entries
			if len(logs) != 2 || logs[0].Level != "debug" || logs[0].Outcome != "" || logs[1].Level != test.level || logs[1].Outcome != test.state || !logs[1].HasResponse {
				t.Fatalf("severity or outcome incorrect: %+v", logs)
			}
			if response, err := s.postingResponse(1, logs[1].ID); err != nil || !strings.Contains(response, `"data"`) {
				t.Fatal("outcome response is unavailable", err)
			}
		})
	}
}

func TestPostingLogsPaginationAndSnapshot(t *testing.T) {
	s := &GroupsService{}
	s.postMu.Lock()
	for i := 1; i <= 205; i++ {
		s.appendPostingLogLocked("info", fmt.Sprintf("Activity %d", i))
	}
	s.postMu.Unlock()
	if page := s.GetPostingLogs(1, 0); page.PageSize != 50 || len(page.Entries) != 50 {
		t.Fatal("default page must contain 50 logs", page)
	}
	page := s.GetPostingLogs(2147483647, 10)
	if page.Page != 21 || page.Total != 205 || len(page.Entries) != 5 || page.Entries[0].ID != 201 {
		t.Fatal("latest page incorrect", page)
	}
	page.Entries[0].Message = "modified display copy"
	if s.GetPostingLogs(21, 10).Entries[0].Message != "Activity 201" {
		t.Fatal("log history can be changed by reader")
	}
	if page = s.GetPostingLogs(-4, 10000); page.Page != 1 || page.PageSize != 100 || len(page.Entries) != 100 {
		t.Fatal("page bounds incorrect")
	}
	s.setPostStatus("Idle display state")
	if s.GetPostingLogs(1, 10).Total != 205 {
		t.Fatal("idle polling added log noise")
	}
}
func TestPauseAllowsInflightRequestAndBlocksNext(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration for local HTTP fixtures")
	}
	s := &GroupsService{}
	ctx, cancel := testPostingContext(s)
	defer cancel()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		number := requests.Add(1)
		started <- struct{}{}
		if number == 1 {
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(w, `{"data":{"story_create":{"post_id":"fixture-post"}}}`)
	}))
	defer server.Close()
	defer close(release)
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(server.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
	replay := &postReplay{client: &http.Client{Jar: jar}, user: "fixture-user"}
	first := make(chan error, 1)
	go func() { _, _, err := replay.send(ctx, server.URL, http.Header{}, []byte("first")); first <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("first request did not start")
	}
	s.PausePosting()
	select {
	case err := <-first:
		t.Fatal("pause interrupted in-flight request", err)
	default:
	}
	// Release the in-flight handler without closing the deferred cleanup channel.
	release <- struct{}{}
	select {
	case err := <-first:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight result did not complete")
	}
	second := make(chan error, 1)
	go func() { _, _, err := replay.send(ctx, server.URL, http.Header{}, []byte("second")); second <- err }()
	awaitPostingState(t, s, "paused")
	if requests.Load() != 1 {
		t.Fatal("second request started while paused")
	}
	s.ResumePosting()
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second request did not resume")
	}
	if requests.Load() != 2 {
		t.Fatal("unexpected request count")
	}
	if strings.Contains(s.GetPostingLogs(1, 100).Entries[0].Message, "fixture-user") {
		t.Fatal("session value leaked into logs")
	}
}
