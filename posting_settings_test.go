package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSettingsPersistWithoutReturningKey(t *testing.T) {
	s := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	settings, err := s.GetSettings()
	if err != nil || settings.MinimumInterval != 60 || settings.KeyConfigured {
		t.Fatalf("defaults: %#v %v", settings, err)
	}
	settings.Instructions = "Keep the original phone numbers"
	settings.Rewrite = true
	if err = s.SaveSettings(settings, "fixture-secret", false); err != nil {
		t.Fatal(err)
	}
	next := &GroupsService{settingsPath: s.settingsPath}
	saved, err := next.GetSettings()
	if err != nil || !saved.KeyConfigured || saved.Instructions != settings.Instructions {
		t.Fatalf("saved: %#v %v", saved, err)
	}
	data, _ := json.Marshal(saved)
	if strings.Contains(string(data), "fixture-secret") {
		t.Fatal("API key exposed to UI")
	}
	info, err := os.Stat(s.settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	// Windows ACLs are not represented by Go's Unix permission bits.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("settings file must be private", err)
	}
	saved.MaximumInterval = 180
	if err = next.SaveSettings(saved, "", false); err != nil {
		t.Fatal(err)
	}
	kept, _ := next.GetSettings()
	if !kept.KeyConfigured {
		t.Fatal("blank key deleted saved key")
	}
	if err = next.SaveSettings(saved, "", true); err != nil {
		t.Fatal(err)
	}
	removed, _ := next.GetSettings()
	if removed.KeyConfigured {
		t.Fatal("key was not removed")
	}
	saved.MinimumInterval = 30
	if next.SaveSettings(saved, "", false) == nil {
		t.Fatal("short interval accepted")
	}
}
func TestGeminiRewriteRequestAndFailures(t *testing.T) {
	for _, scenario := range []string{"success", "empty", "blocked", "truncated", "error"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("x-goog-api-key") != "fixture-key" || r.URL.RawQuery != "" {
					t.Error("key must be sent only in header")
				}
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				encoded, _ := json.Marshal(body)
				if !strings.Contains(string(encoded), "Keep it short") || !strings.Contains(string(encoded), "0718448461") {
					t.Error("missing original or instructions")
				}
				switch scenario {
				case "success":
					fmt.Fprint(w, `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"internal thought","thought":true},{"text":"Rewritten 0718448461"}]}}]}`)
				case "empty":
					fmt.Fprint(w, `{"candidates":[]}`)
				case "blocked":
					fmt.Fprint(w, `{"candidates":[{"finishReason":"SAFETY","content":{"parts":[]}}]}`)
				case "truncated":
					fmt.Fprint(w, `{"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"text":"Partial"}]}}]}`)
				case "error":
					w.WriteHeader(429)
					fmt.Fprint(w, `{"error":{"message":"fixture-key private server detail"}}`)
				}
			}))
			defer server.Close()
			body, err := rewriteGemini(context.Background(), server.Client(), server.URL, "fixture-key", "Original 0718448461", "Keep it short")
			if scenario == "success" {
				if err != nil || body != "Rewritten 0718448461" {
					t.Fatal(body, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "fixture-key") {
				t.Fatal("unsafe failure", body, err)
			}
		})
	}
}
func TestRewriteFailureFallsBackForOnlyOnePost(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("x-goog-api-key") != "saved-key" {
			t.Error("Gemini key changed after fallback")
		}
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"Rewritten next post"}]},"finishReason":"STOP"}]}`)
	}))
	defer server.Close()
	s := &GroupsService{}
	settings := storedSettings{APIKey: "saved-key"}
	options := PostingOptions{Rewrite: true}
	first, err := s.rewriteForPost(context.Background(), "Original", options, &settings, server.Client(), server.URL)
	if err != nil || first != "Original" || settings.APIKey != "saved-key" {
		t.Fatalf("bad fallback: %q %v", first, err)
	}
	next, err := s.rewriteForPost(context.Background(), "Original", options, &settings, server.Client(), server.URL)
	if err != nil || next != "Rewritten next post" || calls != 2 {
		t.Fatalf("next rewrite not attempted: %q %v calls=%d", next, err, calls)
	}
	page := s.GetPostingLogs(1, 50)
	if len(page.Entries) != 1 || page.Entries[0].Level != "warning" || !strings.Contains(page.Entries[0].Message, "this post only") {
		t.Fatal("fallback warning not logged")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.rewriteForPost(ctx, "Original", options, &settings, server.Client(), server.URL); err == nil || calls != 2 {
		t.Fatal("canceled posting continued rewriting")
	}
}
func TestCountdownPausesAndStopsWithoutLogFlood(t *testing.T) {
	s := &GroupsService{}
	ctx, cancel := testPostingContext(s)
	defer cancel()
	s.postMu.Lock()
	s.posting.Total = 4
	s.postMu.Unlock()
	s.beginPostingGroup(Group{ID: "next", Name: "Next group"})
	s.PausePosting()
	done := make(chan error, 1)
	go func() { done <- s.waitPostingDelay(ctx, 400*time.Millisecond) }()
	awaitPostingState(t, s, "paused")
	time.Sleep(450 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("countdown continued while paused")
	default:
	}
	s.ResumePosting()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("countdown did not resume")
	}
	if s.GetPostingLogs(1, 50).Total > 6 {
		t.Fatal("countdown flooded logs")
	}
	go func() { done <- s.waitPostingDelay(ctx, time.Minute) }()
	s.CancelPosting()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("stop ignored")
		}
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel countdown")
	}
}
func TestRewrittenReplaySerialization(t *testing.T) {
	body := "Rewritten\n\n0718448461"
	form, err := buildReplayPostWithRewrite(postFixtureForm(), body, "next", []string{"photo"}, true)
	if err != nil {
		t.Fatal(err)
	}
	var variables map[string]any
	json.Unmarshal([]byte(form.Get("variables")), &variables)
	input := record(variables["input"])
	message := record(input["message"])
	composed := record(input["composed_text"])
	if message["text"] != body || composed["blocks"].([]any)[0] != body || len(message["ranges"].([]any)) != 0 {
		t.Fatal("rewrite lost line breaks or kept stale ranges")
	}
	if variables["large"].(float64) == 0 || input["actor_id"] != "fixture-user" {
		t.Fatal("unrelated template fields changed")
	}
}

func TestRewriteDoesNotBypassOriginalDraftLedger(t *testing.T) {
	images := []PostUpload{{Name: "test.png", Data: fixturePNG}}
	key := "fixture-user:" + draftFingerprint("Original", images) + ":group"
	s := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json"), groupsLoad: &groupsLoad{user: "fixture-user"}, postLedger: map[string]PostOutcome{key: {Group: Group{ID: "group"}, State: "pending"}}}
	settings := defaultSettings()
	if err := s.SaveSettings(settings, "fixture-key", false); err != nil {
		t.Fatal(err)
	}
	options := settings.PostingOptions
	options.Rewrite = true
	result, err := s.PublishPostWithOptions(context.Background(), []string{"group"}, "Original", images, options)
	if err != nil || result.State != "complete" || len(result.Results) != 1 || s.browser != nil {
		t.Fatalf("rewrite bypassed duplicate ledger: %#v %v", result, err)
	}
}
