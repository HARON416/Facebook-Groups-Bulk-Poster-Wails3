package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestFacebookPostingImageDownloadsAndCache(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString(fixturePNG)
	requests := 0
	client := &http.Client{Transport: diagnosticTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Header.Get("Cookie") != "" {
			t.Fatal("forwarded session cookies to image CDN")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})}
	uri := "https://scontent.example.fbcdn.net/full.png?signature=keep%2Bexact"
	drafts, err := downloadFacebookPostingDrafts(context.Background(), client, []FacebookPagePost{{Text: "First", Images: []string{uri}}, {Text: "Second"}, {Text: "Third", Images: []string{uri}}}, nil)
	if err != nil || len(drafts) != 3 || requests != 1 || drafts[0].images[0].Data != fixturePNG || len(drafts[1].images) != 0 || !reflect.DeepEqual(drafts[0].images, drafts[2].images) {
		t.Fatalf("drafts=%+v requests=%d err=%v", drafts, requests, err)
	}
	for _, uri := range []string{"http://scontent.example.fbcdn.net/image.jpg", "https://example.test/image.jpg", "https://fbcdn.net.evil.test/image.jpg", "https://user:pass@scontent.example.fbcdn.net/image.jpg"} {
		if _, err := downloadFacebookPostingDrafts(context.Background(), client, []FacebookPagePost{{Text: "Draft", Images: []string{uri}}}, nil); err == nil {
			t.Fatalf("accepted invalid image URL %s", uri)
		}
	}
	client.Transport = diagnosticTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<html>expired</html>"))}, nil
	})
	if _, err := downloadFacebookPostingDrafts(context.Background(), client, []FacebookPagePost{{Text: "Draft", Images: []string{uri}}}, nil); err == nil {
		t.Fatal("accepted an expired image's HTML response")
	}
}

func TestFacebookQueueScheduleCanBeStoppedBeforeDownloads(t *testing.T) {
	s := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	done := make(chan error, 1)
	go func() {
		result, err := s.ScheduleFacebookPostsWithOptions(context.Background(), []string{"group"}, []FacebookPagePost{{Text: "First", Images: []string{"https://scontent.example.fbcdn.net/full.jpg"}}, {Text: "Second"}}, defaultSettings().PostingOptions, time.Now().Add(time.Hour).Format(time.RFC3339))
		if err == nil && result.State != "stopped" {
			t.Error("schedule did not stop")
		}
		done <- err
	}()
	awaitPostingState(t, s, "scheduled")
	if _, err := s.PublishFacebookPostsWithOptions(context.Background(), []string{"other"}, []FacebookPagePost{{Text: "Other"}}, defaultSettings().PostingOptions); err == nil {
		t.Fatal("allowed another run during schedule")
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
		t.Fatal("schedule opened browser before its due time")
	}
}

func TestFacebookQueueLedgerUsesEveryCard(t *testing.T) {
	drafts := []postingDraft{{body: "First"}, {body: "Second"}}
	fingerprint := postingDraftFingerprint(drafts)
	changed := []postingDraft{{body: "First"}, {body: "Edited second"}}
	if fingerprint == postingDraftFingerprint(changed) || fingerprint == postingDraftFingerprint([]postingDraft{drafts[1], drafts[0]}) {
		t.Fatal("queue edits/order ignored by ledger")
	}
	s := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json"), groupsLoad: &groupsLoad{user: "fixture-user"}, postLedger: map[string]PostOutcome{"fixture-user:" + fingerprint + ":group": {Group: Group{ID: "group"}, State: "posted"}}}
	result, err := s.PublishFacebookPostsWithOptions(context.Background(), []string{"group", "group"}, []FacebookPagePost{{Text: "First"}, {Text: "Second"}}, defaultSettings().PostingOptions)
	if err != nil || result.State != "complete" || len(result.Results) != 1 || s.browser != nil {
		t.Fatalf("duplicate queue was not suppressed: %+v %v", result, err)
	}
}

func TestRotatingQueueReplaysDifferentCaptionsAndImages(t *testing.T) {
	captured := capturedPost{URL: "https://web.facebook.com/api/graphql/", Body: postFixtureForm(), User: "fixture-user", Headers: http.Header{"Cookie": {"c_user=fixture-user"}, "Content-Type": {"application/x-www-form-urlencoded"}}}
	replay, err := newPostReplay([]*proto.NetworkCookie{{Name: "c_user", Value: "fixture-user", Domain: ".facebook.com", Path: "/", Secure: true}}, capturedPhotoUpload{}, captured)
	if err != nil {
		t.Fatal(err)
	}
	replay.allowRewrite = true
	var got []string
	replay.client.Transport = diagnosticTransport(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(data))
		var vars map[string]any
		_ = json.Unmarshal([]byte(form.Get("variables")), &vars)
		input := record(vars["input"])
		group := stringValue(record(input["audience"])["to_id"])
		caption := stringValue(record(input["message"])["text"])
		attachments := input["attachments"].([]any)
		got = append(got, fmt.Sprintf("%s:%s:%d", group, caption, len(attachments)))
		body := fmt.Sprintf(`{"data":{"story_create":{"post_id":"created-%s","story":{"to":{"id":%q,"if_viewer_can_see_pending_post_in_mall":null},"if_viewer_can_learn_more_about_pending_post":null}}}}`, group, group)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	drafts := []postingDraft{{body: "First edited card"}, {body: "Second card", images: []PostUpload{{Name: "photo.png", Data: fixturePNG}}}}
	for index, group := range []string{"one", "two", "three", "four"} {
		draft, _ := rotatingPostingDraft(drafts, index)
		photos := []string{}
		if len(draft.images) > 0 {
			photos = []string{"fresh-uploaded-photo"}
		}
		result := replay.create(context.Background(), draft.body, photos, group)
		if result.State != "posted" {
			t.Fatalf("rotation stopped: %+v", result)
		}
	}
	want := []string{"one:First edited card:0", "two:Second card:1", "three:First edited card:0", "four:Second card:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rotation: got %v want %v", got, want)
	}
	// Single-draft posting must retain its strict captured-caption validation.
	replay.allowRewrite = false
	if result := replay.create(context.Background(), "Different hidden draft", nil, "five"); result.State != "failed" {
		t.Fatal("allowed an unrelated manual caption")
	}
}
