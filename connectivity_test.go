package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnectivityPausesAndResumesQueue(t *testing.T) {
	s := &GroupsService{connectivityEnabled: true}
	var online atomic.Bool
	s.connectivityProbe = func(context.Context) bool { return online.Load() }
	ctx, cancel := testPostingContext(s)
	defer cancel()
	s.checkConnectivity(ctx)
	done := make(chan error, 1)
	go func() { done <- postingCheckpoint(ctx) }()
	awaitPostingState(t, s, "paused")
	if status := s.GetPostingStatus(); !status.Offline || !status.PauseRequested || !status.Active {
		t.Fatal(status)
	}
	select {
	case <-done:
		t.Fatal("offline queue advanced")
	default:
	}
	online.Store(true)
	s.checkConnectivity(ctx)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("queue did not resume")
	}
	if status := s.GetPostingStatus(); status.Offline || status.PauseRequested || status.State != "posting" {
		t.Fatal(status)
	}
	logs := s.GetPostingLogs(1, 100)
	count := logs.Total
	s.checkConnectivity(ctx)
	if s.GetPostingLogs(1, 100).Total != count {
		t.Fatal("unchanged connection created duplicate logs")
	}
}

func TestReconnectPreservesManualPause(t *testing.T) {
	s := &GroupsService{connectivityEnabled: true}
	var online atomic.Bool
	s.connectivityProbe = func(context.Context) bool { return online.Load() }
	ctx, cancel := testPostingContext(s)
	defer cancel()
	s.PausePosting()
	s.checkConnectivity(ctx)
	done := make(chan error, 1)
	go func() { done <- postingCheckpoint(ctx) }()
	awaitPostingState(t, s, "paused")
	online.Store(true)
	s.checkConnectivity(ctx)
	if !s.GetConnectivityStatus().ManualPause {
		t.Fatal("manual pause was lost")
	}
	select {
	case <-done:
		t.Fatal("reconnect overrode manual pause")
	default:
	}
	s.ResumePosting()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("manual resume did not release queue")
	}
}

func TestStopWhileOffline(t *testing.T) {
	s := &GroupsService{connectivityEnabled: true}
	s.connectivityProbe = func(context.Context) bool { return false }
	ctx, cancel := testPostingContext(s)
	defer cancel()
	s.checkConnectivity(ctx)
	done := make(chan error, 1)
	go func() { done <- postingCheckpoint(ctx) }()
	awaitPostingState(t, s, "paused")
	s.CancelPosting()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not release offline queue")
	}
}

func TestConnectivitySamplesDebounceLossAndRecoverImmediately(t *testing.T) {
	s := &GroupsService{connectivityEnabled: true, connectivity: ConnectivityStatus{Known: true, Online: true}}
	var online atomic.Bool
	s.connectivityProbe = func(context.Context) bool { return online.Load() }
	s.checkConnectivitySample(context.Background(), true)
	if !s.GetConnectivityStatus().Online {
		t.Fatal("one failed periodic probe reported an outage")
	}
	s.checkConnectivitySample(context.Background(), true)
	if s.GetConnectivityStatus().Online {
		t.Fatal("confirmed outage stayed online")
	}
	for range 3 {
		online.Store(true)
		s.checkConnectivitySample(context.Background(), true)
		if !s.GetConnectivityStatus().Online {
			t.Fatal("reconnection required multiple successes")
		}
		online.Store(false)
		s.checkConnectivitySample(context.Background(), true)
		if !s.GetConnectivityStatus().Online {
			t.Fatal("failure counter did not reset on reconnection")
		}
		s.checkConnectivitySample(context.Background(), true)
		if s.GetConnectivityStatus().Online {
			t.Fatal("second failure did not confirm disconnection")
		}
	}
}

func TestPostingFailureConfirmsOutageWithoutDebounce(t *testing.T) {
	s := &GroupsService{connectivityEnabled: true, connectivity: ConnectivityStatus{Known: true, Online: true}, connectivityProbe: func(context.Context) bool { return false }}
	s.checkConnectivity(context.Background())
	if s.GetConnectivityStatus().Online {
		t.Fatal("transport failure did not pause promptly")
	}
}

func TestConnectivityRefreshInvalidatesOldProbeAndCoalescesHints(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	s := &GroupsService{connectivityEnabled: true, connectivity: ConnectivityStatus{Known: true, Online: true}, connectivityWake: make(chan struct{}, 1)}
	s.connectivityProbe = func(context.Context) bool { close(started); <-release; return false }
	done := make(chan struct{})
	go func() { s.checkConnectivity(context.Background()); close(done) }()
	<-started
	for range 5 {
		s.RefreshConnectivity()
	}
	close(release)
	<-done
	if !s.GetConnectivityStatus().Online {
		t.Fatal("old route's probe overwrote the state after a network change")
	}
	if len(s.connectivityWake) != 1 {
		t.Fatal("network hints were not coalesced")
	}
	s.postMu.Lock()
	s.connectivityProbe = func(context.Context) bool { return true }
	s.postMu.Unlock()
	s.checkConnectivitySample(context.Background(), true)
	if !s.GetConnectivityStatus().Online {
		t.Fatal("fresh probe failed to restore connectivity")
	}
}

func TestConnectivityProbeUsesAnyReachableOriginAndCancelsStalledProbes(t *testing.T) {
	cancelled := make(chan struct{}, 1)
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			close(started)
			<-r.Context().Done()
			cancelled <- struct{}{}
		case "/ok":
			<-started
			w.WriteHeader(http.StatusNoContent)
		case "/redirect":
			http.Redirect(w, r, "/portal", http.StatusFound)
		case "/portal":
			t.Error("followed captive-portal redirect")
			fmt.Fprint(w, "portal")
		case "/blocked":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Start both requests before the fast endpoint is released, proving that a
	// slow first origin cannot delay another origin's success.
	if !probeInternetReachable(ctx, client, []connectivityEndpoint{{server.URL + "/slow", 204}, {server.URL + "/ok", 204}}) {
		t.Fatal("slow origin blocked a healthy origin")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("successful probe did not cancel the stalled origin")
	}
	if probeInternetReachable(ctx, client, []connectivityEndpoint{{server.URL + "/redirect", 204}}) {
		t.Fatal("captive portal counted as online")
	}
	if !probeInternetReachable(ctx, client, []connectivityEndpoint{{server.URL + "/blocked", 0}, {server.URL + "/redirect", 204}}) {
		t.Fatal("reachable application origin was mistaken for an internet outage")
	}
	if probeInternetReachable(ctx, client, []connectivityEndpoint{{server.URL + "/blocked", 204}}) {
		t.Fatal("invalid 204 response counted as success")
	}
}

func TestConnectivityProbeDeadlineAndCancelledState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if probeInternetReachable(ctx, server.Client(), []connectivityEndpoint{{server.URL, 204}}) {
		t.Fatal("timed out probe reported success")
	}
	s := &GroupsService{connectivityEnabled: true, connectivity: ConnectivityStatus{Known: true, Online: true}, connectivityProbe: func(ctx context.Context) bool { <-ctx.Done(); return false }}
	s.checkConnectivity(ctx)
	if !s.GetConnectivityStatus().Online {
		t.Fatal("cancelled probe changed known connectivity")
	}
}

func TestConnectivityStatusDoesNotClaimAnIdleQueueIsPaused(t *testing.T) {
	s := &GroupsService{connectivityEnabled: true, connectivity: ConnectivityStatus{Known: true, Online: false}}
	if s.GetConnectivityStatus().Active {
		t.Fatal("idle account claims an active queue")
	}
	_, cancel := testPostingContext(s)
	defer cancel()
	if !s.GetConnectivityStatus().Active {
		t.Fatal("active queue omitted from connectivity status")
	}
	s.postMu.Lock()
	s.posting.State = "scheduled"
	s.postMu.Unlock()
	if s.GetConnectivityStatus().Active {
		t.Fatal("future schedule claims an actively paused queue")
	}
}

func TestConnectivityMonitorRefreshWakesWithoutWaitingForTimer(t *testing.T) {
	var online atomic.Bool
	s := &GroupsService{connectivityProbe: func(context.Context) bool { return online.Load() }}
	s.startConnectivityMonitor()
	defer s.ServiceShutdown()
	awaitConnectivity := func(want bool) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for {
			status := s.GetConnectivityStatus()
			if status.Known && status.Online == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("monitor did not update to online=%t", want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	awaitConnectivity(false)
	online.Store(true)
	s.RefreshConnectivity()
	awaitConnectivity(true)
}
