package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPostingScheduleValidation(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	for _, value := range []string{"", "invalid", "2026-10-15T11:00:00Z", "2026-10-15T12:00:00Z"} {
		if _, err := parsePostingSchedule(value, now); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	at, err := parsePostingSchedule("2026-10-15T16:00:00+03:00", now)
	if err != nil || !at.Equal(now.Add(time.Hour)) {
		t.Fatalf("incorrect time zone conversion: %s, %v", at, err)
	}
}

func TestScheduledQueueCanBeCancelledAndExcludesOtherRuns(t *testing.T) {
	s := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	done := make(chan error, 1)
	go func() {
		result, err := s.ScheduleRichPostWithOptions(context.Background(), []string{"group"}, "draft", "", []PostUpload{{Name: "image.png", Data: fixturePNG}}, defaultSettings().PostingOptions, time.Now().Add(time.Hour).Format(time.RFC3339))
		if err == nil && result.State != "stopped" {
			t.Error("cancelled schedule did not stop")
		}
		done <- err
	}()
	awaitPostingState(t, s, "scheduled")
	if !s.GetPostingStatus().Active {
		t.Fatal("scheduled queue is not active")
	}
	s.PausePosting()
	if s.GetPostingStatus().State != "scheduled" {
		t.Fatal("pause changed the scheduled time")
	}
	_, err := s.PublishPostWithOptions(context.Background(), []string{"other"}, "draft", []PostUpload{{Data: fixturePNG}}, defaultSettings().PostingOptions)
	if err == nil {
		t.Fatal("allowed a second run during scheduled queue")
	}
	s.CancelPosting()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("schedule did not cancel")
	}
	if s.browser != nil {
		t.Fatal("schedule opened browser before due time")
	}
}

func TestScheduledQueueWaitsForConnectivityWhenDue(t *testing.T) {
	images := []PostUpload{{Name: "image.png", Data: fixturePNG}}
	s := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json"), connectivityEnabled: true}
	s.connectivityProbe = func(context.Context) bool { return false }
	// A completed draft lets this test exercise the real scheduling entry point
	// and timer without opening Facebook once the queue is released.
	s.postLedger = map[string]PostOutcome{":" + draftFingerprint("draft", images) + ":group": {Group: Group{ID: "group"}, State: "posted"}}
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		result, err := s.ScheduleRichPostWithOptions(ctx, []string{"group"}, "draft", "", images, defaultSettings().PostingOptions, time.Now().Add(100*time.Millisecond).Format(time.RFC3339Nano))
		if err == nil && result.State != "complete" {
			t.Errorf("schedule failed: %#v", result)
		}
		done <- err
	}()
	awaitPostingState(t, s, "scheduled")
	s.checkConnectivity(ctx)
	if s.GetPostingStatus().State != "scheduled" {
		t.Fatal("outage replaced scheduled state")
	}
	awaitPostingState(t, s, "paused")
	select {
	case <-done:
		t.Fatal("offline scheduled queue advanced")
	default:
	}
	s.postMu.Lock()
	s.connectivityProbe = func(context.Context) bool { return true }
	s.postMu.Unlock()
	s.checkConnectivity(ctx)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduled queue did not continue")
	}
}
