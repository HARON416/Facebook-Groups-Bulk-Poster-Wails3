package main

import (
	"context"
	"errors"
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
