package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestPostResponseConfirmation(t *testing.T) {
	tests := []struct {
		name, body  string
		status      int
		state, code string
		stop        bool
	}{
		{"published", `{"data":{"story_create":{"post_id":"post-1","story":{"url":"https://web.facebook.com/groups/g/posts/post-1/"}}}}`, 200, "posted", "", false},
		{"pending link", `for (;;);{"data":{"story_create":{"story":{"id":"post-2","url":"https://web.facebook.com/groups/g/pending_posts/post-2/"}}}}`, 200, "pending", "", false},
		{"pending flag", `{"data":{"story_create":{"story":{"if_viewer_can_learn_more_about_pending_post":{},"to":{"if_viewer_can_see_pending_post_in_mall":null}}}}}`, 200, "pending", "", false},
		{"pending fallback permalink", `{"data":{"story_create":{"story_id":null,"post_id":null,"publishing_flow":"FALLBACK","story":{"id":"opaque-story-id","url":"https://web.facebook.com/groups/g/permalink/123/","if_viewer_can_learn_more_about_pending_post":{"__typename":"Story","id":"opaque-story-id"},"to":{"if_viewer_can_see_pending_post_in_mall":{"if_viewer_can_see_pending_post_in_mall":{"comet_pending_post_cards":{}}}}}}}}`, 200, "pending", "", false},
		{"published fallback permalink", `{"data":{"story_create":{"story_id":null,"post_id":null,"publishing_flow":"FALLBACK","story":{"id":"opaque-story-id","url":"https://web.facebook.com/groups/g/permalink/123/","if_viewer_can_learn_more_about_pending_post":null,"to":{"if_viewer_can_see_pending_post_in_mall":null}}}}}`, 200, "posted", "", false},
		{"caption is not a restriction", `{"data":{"story_create":{"post_id":"post-1","story":{"message":"Facebook has temporarily blocked posting; we limit how often..."}}}}`, 200, "posted", "", false},
		{"group limit localized", `{"errors":[{"message":"Limite atteint","code":1357006}]}`, 200, "failed", "1357006", false},
		{"group limit message", `{"errors":[{"message":"You're at the limit for pending content in this group."}]}`, 200, "failed", "", false},
		{"account limit localized", `{"errors":[{"message":"Limite atteinte","code":1390008}]}`, 200, "failed", "1390008", true},
		{"api account limit", `{"errors":[{"extensions":{"api_error_code":368}}]}`, 200, "failed", "368", true},
		{"session expired", `{"error":190}`, 200, "failed", "190", true},
		{"unknown rejection", `{"error":999,"errorSummary":"Private server detail"}`, 200, "failed", "999", true},
		{"empty", ``, 200, "unknown", "", true},
		{"html", `<html>Login</html>`, 200, "unknown", "", true},
		{"unrelated story", `{"data":{"feed":{"post_id":"unrelated","url":"https://web.facebook.com/posts/other/"}}}`, 200, "unknown", "", true},
		{"stream", `{"data":{}}` + "\n" + `{"data":{"story_create":{"post_id":"post-3"}},"is_final":true}`, 200, "posted", "", false},
		{"conflicting", `{"data":{"story_create":{"post_id":"post-4"}},"errors":[{"code":1390008}]}`, 200, "unknown", "", true},
		{"http limit", ``, 429, "failed", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := parsePostResponse([]byte(test.body), test.status)
			if result.State != test.state || result.Code != test.code || result.stop != test.stop {
				t.Fatalf("got %#v", result)
			}
			if strings.Contains(result.Message, "Private server detail") {
				t.Fatal("raw response leaked into display data")
			}
		})
	}
	if id, _ := parseUploadedPhoto([]byte(`for (;;);{"__ar":1,"payload":{"photoID":"fresh-photo"}}`), 200); id != "fresh-photo" {
		t.Fatal("upload confirmation not parsed")
	}
	if id, _ := parseUploadedPhoto([]byte(`{"payload":{"photoID":"unconfirmed"}}`), 200); id != "" {
		t.Fatal("unconfirmed upload accepted")
	}
}

func TestPostRestrictionScope(t *testing.T) {
	// Preserve the supplied error shapes without private trace IDs or links.
	tests := []struct {
		name, body, code, message string
		stop                      bool
	}{
		{
			name: "account posting temporarily limited",
			body: `{"data":{"story_create":null},"errors":[{"message":"A server error field_exception occured. Check server logs for details.","severity":"CRITICAL","code":1390008,"api_error_code":368,"summary":"You can't use this feature at the moment","description":{"__html":"We limit how often you can post, comment or do other things in a given amount of time. You can try again later."},"description_html":null,"description_raw":{"__html":"We limit how often you can post."},"is_silent":false,"is_transient":false,"is_not_critical":true,"requires_reauth":false,"allow_user_retry":false,"path":["story_create"]}],"extensions":{"is_final":true}}`,
			code: "1390008", message: "Facebook has temporarily limited posting. Try again later.", stop: true,
		},
		{
			name: "group rejection continues queue",
			body: `{"data":{"story_create":null},"errors":[{"message":"A server error field_exception occured. Check server logs for details.","severity":"CRITICAL","code":1357006,"api_error_code":200,"summary":"Something went wrong","description":"We're having trouble completing your request. Please try again.","description_html":null,"description_raw":"We're having trouble completing your request. Please try again.","is_silent":false,"is_transient":false,"is_not_critical":true,"requires_reauth":false,"allow_user_retry":false,"path":["story_create"]}],"extensions":{"is_final":true}}`,
			code: "1357006", message: "Facebook could not post to this group. Continuing with the remaining groups.",
		},
		{
			name: "account API restriction takes precedence over group code",
			body: `{"data":{"story_create":null},"errors":[{"code":1357006,"api_error_code":368}]}`,
			code: "1357006", message: "Facebook has temporarily limited posting. Try again later.", stop: true,
		},
		{
			name: "account restriction takes precedence over separate group error",
			body: `{"data":{"story_create":null},"errors":[{"code":1357006,"api_error_code":200},{"code":1390008,"api_error_code":368}]}`,
			code: "1390008", message: "Facebook has temporarily limited posting. Try again later.", stop: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome := parsePostResponse([]byte(test.body), http.StatusOK)
			if outcome.State != "failed" || outcome.Code != test.code || outcome.Message != test.message || outcome.stop != test.stop {
				t.Fatalf("unexpected restriction outcome: %#v", outcome)
			}
		})
	}
}

func postFixtureForm() string {
	variables := `{"input":{"actor_id":"fixture-user","audience":{"to_id":"old-group","unknown":"keep"},"message":{"text":"First\nSecond","ranges":[]},"composed_text":{"blocks":["First\nSecond"],"opaque":"keep"},"attachments":[{"photo":{"id":"old-photo"}}],"logging":{"composer_session_id":"old-session"},"client_mutation_id":"old-mutation"},"groupID":"old-group","large":7694986411746282026,"relay_flag":true}`
	return url.Values{"__user": {"fixture-user"}, "av": {"fixture-user"}, "fb_api_req_friendly_name": {createPostMutation}, "doc_id": {"captured-document"}, "fb_dtsg": {"captured-token"}, "__req": {"z"}, "unknown": {"preserve"}, "variables": {variables}}.Encode()
}
func TestReplayPreservesBrowserSerialization(t *testing.T) {
	form, err := buildReplayPost(postFixtureForm(), "First\nSecond", "new-group", []string{"photo-a", "photo-b"})
	if err != nil {
		t.Fatal(err)
	}
	var variables map[string]json.RawMessage
	if err = json.Unmarshal([]byte(form.Get("variables")), &variables); err != nil {
		t.Fatal(err)
	}
	if string(variables["large"]) != "7694986411746282026" || string(variables["relay_flag"]) != "true" {
		t.Fatal("unknown variables lost precision")
	}
	var input map[string]any
	json.Unmarshal(variables["input"], &input)
	if record(input["audience"])["to_id"] != "new-group" || record(input["audience"])["unknown"] != "keep" || record(input["composed_text"])["opaque"] != "keep" {
		t.Fatal("captured fields changed unexpectedly")
	}
	if record(input["logging"])["composer_session_id"] == "old-session" || input["client_mutation_id"] == "old-mutation" {
		t.Fatal("mutation IDs reused")
	}
	if form.Get("doc_id") != "captured-document" || form.Get("fb_dtsg") != "captured-token" || form.Get("unknown") != "preserve" {
		t.Fatal("fresh form values lost")
	}
	if _, err = buildReplayPost(postFixtureForm(), "different draft", "new-group", nil); err == nil {
		t.Fatal("different body accepted")
	}
}

func TestReplayUploadsAndCreatesWithoutRetry(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration for local HTTP fixtures")
	}
	var uploaded []string
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner, _ := r.Cookie("c_user")
		if owner == nil || owner.Value != "fixture-user" {
			t.Error("wrong owner")
		}
		if r.URL.Path == "/upload" {
			if r.URL.Query().Get("fb_dtsg") != "fresh-upload-token" || r.Header.Get("X-Captured") != "fresh-header" {
				t.Error("fresh values lost")
			}
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			if r.FormValue("profile_id") != "fixture-user" || r.FormValue("source") != "captured-source" {
				t.Error("captured multipart fields lost")
			}
			file, header, err := r.FormFile("captured-file-field")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			data, _ := io.ReadAll(file)
			if len(data) == 0 || header.Header.Get("Content-Type") != "image/png" {
				t.Error("image bytes lost")
			}
			uploaded = append(uploaded, header.Filename)
			http.SetCookie(w, &http.Cookie{Name: "xs", Value: "updated-session", Path: "/"})
			fmt.Fprintf(w, `for (;;);{"__ar":1,"payload":{"photoID":"photo-%d"}}`, len(uploaded))
			return
		}
		posts++
		session, _ := r.Cookie("xs")
		if session == nil || session.Value != "updated-session" {
			t.Error("server session update was not applied")
		}
		r.ParseForm()
		var variables map[string]any
		json.Unmarshal([]byte(r.Form.Get("variables")), &variables)
		input := record(variables["input"])
		if record(input["audience"])["to_id"] != "new-group" {
			t.Error("wrong target group")
		}
		attachments := input["attachments"].([]any)
		if len(attachments) != 2 || record(record(attachments[0])["photo"])["id"] != "photo-1" || record(record(attachments[1])["photo"])["id"] != "photo-2" {
			t.Error("photo order changed")
		}
		fmt.Fprint(w, `{"data":{"story_create":{"post_id":"created-post","story":{"to":{"id":"new-group","if_viewer_can_see_pending_post_in_mall":null},"if_viewer_can_learn_more_about_pending_post":null}}}}`)
	}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	endpoint, _ := url.Parse(server.URL)
	jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
	replay := &postReplay{client: &http.Client{Jar: jar}, user: "fixture-user", upload: capturedPhotoUpload{URL: server.URL + "/upload?__user=fixture-user&fb_dtsg=fresh-upload-token", Headers: http.Header{"X-Captured": {"fresh-header"}}, Fields: url.Values{"profile_id": {"fixture-user"}, "source": {"captured-source"}}, Files: []photoUploadFile{{Field: "captured-file-field"}}}, post: capturedPost{URL: server.URL + "/create", Body: postFixtureForm(), Headers: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}}}
	ids := []string{}
	for _, name := range []string{"first.png", "second.png"} {
		id, outcome := replay.uploadImage(context.Background(), PostUpload{Name: name, Data: fixturePNG})
		if id == "" {
			t.Fatalf("upload: %#v", outcome)
		}
		ids = append(ids, id)
	}
	result := replay.create(context.Background(), "First\nSecond", ids, "new-group")
	if result.State != "posted" || posts != 1 || !reflect.DeepEqual(uploaded, []string{"first.png", "second.png"}) {
		t.Fatalf("unexpected replay: %#v uploads=%v posts=%d", result, uploaded, posts)
	}
	jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "different-profile", Path: "/"}})
	result = replay.create(context.Background(), "First\nSecond", ids, "new-group")
	if result.State != "unknown" || posts != 1 {
		t.Fatal("changed profile was replayed")
	}
}

func TestPublishLedgerPreventsDuplicateAndUncertainRetry(t *testing.T) {
	images := []PostUpload{{Name: "test.png", Data: fixturePNG}}
	key := "fixture-user:" + draftFingerprint("Draft", images) + ":group"
	for _, state := range []string{"posted", "pending", "unknown"} {
		t.Run(state, func(t *testing.T) {
			s := &GroupsService{groupsLoad: &groupsLoad{user: "fixture-user"}, postLedger: map[string]PostOutcome{key: {Group: Group{ID: "group"}, State: state}}}
			result, err := s.PublishPost(context.Background(), []string{"group", "group"}, "Draft", images)
			if err != nil || len(result.Results) != 1 || s.browser != nil {
				t.Fatal("duplicate attempt opened browser", err)
			}
			if result.State != "complete" {
				t.Fatal("previous submission stopped the queue")
			}
		})
	}
}

func TestPriorUnconfirmedDoesNotBlockRemainingGroups(t *testing.T) {
	images := []PostUpload{{Name: "test.png", Data: fixturePNG}}
	key := "fixture-user:" + draftFingerprint("Draft", images) + ":old"
	s := &GroupsService{settingsPath: t.TempDir() + "/settings.json", groupsLoad: &groupsLoad{user: "fixture-user"}, postLedger: map[string]PostOutcome{key: {Group: Group{ID: "old"}, State: "unknown", stop: true}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Stop at the checkpoint immediately after filtering prior groups.
	result, err := s.PublishPost(ctx, []string{"old", "next"}, "Draft", images)
	if err != nil || result.Message != "Posting stopped." || len(result.Results) != 1 || result.Remaining != 1 {
		t.Fatalf("prior unconfirmed prevented next-group checkpoint: %#v %v", result, err)
	}
	if result.Results[0].stop || !strings.Contains(result.Results[0].Message, "continuing") {
		t.Fatalf("prior outcome still blocks queue: %#v", result.Results[0])
	}
}

func TestReplayPreservesFormattedPost(t *testing.T) {
	form, _ := url.ParseQuery(postFixtureForm())
	var variables map[string]any
	if err := json.Unmarshal([]byte(form.Get("variables")), &variables); err != nil {
		t.Fatal(err)
	}
	input := record(variables["input"])
	input["composed_text"] = map[string]any{"blocks": []string{"First", "Second"}, "block_types": []string{"header-one", "unordered-list-item"}, "inline_styles": []any{map[string]any{"offset": 0, "length": 5, "style": "BOLD"}}}
	input["message"] = map[string]any{"text": "First\nSecond", "ranges": []any{map[string]any{"offset": 0, "length": 5, "type": "bold"}}}
	encoded, _ := json.Marshal(variables)
	form.Set("variables", string(encoded))
	replay, err := buildReplayPost(form.Encode(), "First\nSecond", "new-group", []string{"fresh-photo"})
	if err != nil {
		t.Fatal(err)
	}
	var returned map[string]any
	if err := json.Unmarshal([]byte(replay.Get("variables")), &returned); err != nil {
		t.Fatal(err)
	}
	actual := record(returned["input"])
	for _, key := range []string{"message", "composed_text"} {
		want, _ := json.Marshal(input[key])
		got, _ := json.Marshal(actual[key])
		if string(want) != string(got) {
			t.Fatalf("%s formatting changed: %s", key, got)
		}
	}
}
