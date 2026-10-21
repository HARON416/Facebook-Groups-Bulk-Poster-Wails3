package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestPostConfirmationRequiresTargetAndPublication(t *testing.T) {
	tests := []struct{ name, story, state string }{
		{"explicit published", `"post_id":"123","url":"https://web.facebook.com/groups/target/permalink/123/","if_viewer_can_learn_more_about_pending_post":null,"to":{"id":"target","if_viewer_can_see_pending_post_in_mall":null}`, "posted"},
		{"layout published", `"post_id":"123","to":{"id":"target"},"comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "posted"},
		{"participation pending", `"post_id":"123","to":{"id":"target"},"comet_sections":{"layout":{"__typename":"CometStoryPendingParticipationPostLayoutStrategy"},"outer_footer":{"__typename":"CometFeedStoryPendingParticipationPostOuterFooterStrategy"}}`, "pending"},
		{"approval pending", `"post_id":"123","to":{"id":"target"},"if_viewer_can_learn_more_about_pending_post":{}`, "pending"},
		{"id alone", `"post_id":"123"`, "unknown"},
		{"target but missing approval state", `"post_id":"123","to":{"id":"target"}`, "unknown"},
		{"permalink does not prove publication", `"post_id":"123","url":"https://web.facebook.com/groups/target/posts/123/"`, "unknown"},
		{"original group echoed", `"post_id":"123","to":{"id":"original"},"comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "unknown"},
		{"contradictory permalink", `"post_id":"123","to":{"id":"target"},"url":"https://web.facebook.com/groups/original/posts/123/","comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "unknown"},
		{"verified custom group address", `"post_id":"123","to":{"id":"target","url":"https://web.facebook.com/groups/custom.address/"},"url":"https://web.facebook.com/groups/custom.address/posts/123/","comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "posted"},
		{"verified custom address pending", `"post_id":"123","to":{"id":"target","url":"https://web.facebook.com/groups/custom.address/"},"url":"https://web.facebook.com/groups/custom.address/posts/123/","if_viewer_can_learn_more_about_pending_post":{}`, "pending"},
		{"unverified custom group address", `"post_id":"123","to":{"id":"target"},"url":"https://web.facebook.com/groups/custom.address/posts/123/","comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "unknown"},
		{"different numeric group even with matching to ID", `"post_id":"123","to":{"id":"target","url":"https://web.facebook.com/groups/999/"},"url":"https://web.facebook.com/groups/999/posts/123/","comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "unknown"},
		{"alias for wrong group", `"post_id":"123","to":{"id":"original","url":"https://web.facebook.com/groups/custom.address/"},"url":"https://web.facebook.com/groups/custom.address/posts/123/","comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "unknown"},
		{"different alias", `"post_id":"123","to":{"id":"target","url":"https://web.facebook.com/groups/custom.address/"},"url":"https://web.facebook.com/groups/other.address/posts/123/","comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}`, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"data":{"story_create":{"story":{` + tt.story + `}}}}`)
			outcome := parsePostResponseForGroup(body, 200, "target")
			if outcome.State != tt.state || outcome.stop != (tt.state == "unknown") {
				t.Fatalf("incorrect confirmation: %#v", outcome)
			}
			if !strings.Contains(outcome.diagnostic, "requested group target") {
				t.Fatal("missing safe API diagnostic")
			}
		})
	}
}

func TestPublishedCarsUnder500KResponse(t *testing.T) {
	// Structural excerpt of the reported response; omit personal and tracking data.
	body := []byte(`{"data":{"story_create":{
		"post_id":null,"story_id":null,"publishing_flow":"FALLBACK",
		"group_feed_story_edge":{"node":{"post_id":"2385367835608923",
			"to":{"id":"441195636692829","url":"https://web.facebook.com/groups/carsunder500ke/"},
			"permalink_url":"https://web.facebook.com/groups/carsunder500ke/posts/2385367835608923/",
			"comet_sections":{"layout":{"__typename":"CometStoryDefaultLayoutStrategy"}}}},
		"story":{"legacy_story_hideable_id":"2385367835608923",
			"url":"https://web.facebook.com/groups/carsunder500ke/permalink/2385367835608923/",
			"if_viewer_can_learn_more_about_pending_post":null,
			"to":{"id":"441195636692829","url":"https://web.facebook.com/groups/carsunder500ke/","if_viewer_can_see_pending_post_in_mall":null}}
	}}}`)
	outcome := parsePostResponseForGroup(body, 200, "441195636692829")
	if outcome.State != "posted" || outcome.stop || outcome.PostID != "2385367835608923" || outcome.URL != "https://web.facebook.com/groups/carsunder500ke/permalink/2385367835608923/" {
		t.Fatalf("published custom-address post rejected: %+v", outcome)
	}
	if wrong := parsePostResponseForGroup(body, 200, "999"); wrong.State != "unknown" || !wrong.stop {
		t.Fatalf("wrong requested group accepted: %+v", wrong)
	}
}

func TestReplayReplacesNestedGroupReferences(t *testing.T) {
	form, _ := url.ParseQuery(postFixtureForm())
	var variables map[string]any
	json.Unmarshal([]byte(form.Get("variables")), &variables)
	variables["context"] = map[string]any{"nested": []any{"old-group", map[string]any{"target": "old-group", "unrelated": "other-group"}}}
	encoded, _ := json.Marshal(variables)
	form.Set("variables", string(encoded))
	replayed, err := buildReplayPost(form.Encode(), "First\nSecond", "new-group", []string{"fresh-photo"})
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(replayed.Get("variables")), &variables)
	nested := record(variables["context"])["nested"].([]any)
	if nested[0] != "new-group" || record(nested[1])["target"] != "new-group" || record(nested[1])["unrelated"] != "other-group" {
		t.Fatal("nested group references were not replaced correctly")
	}
}

func TestConfirmationDiagnosticsExcludePrivateResponseContent(t *testing.T) {
	body := []byte(`{"data":{"story_create":{"post_id":"123","story":{"to":{"id":"target"},"message":{"text":"private caption"},"encrypted_tracking":"private-token","url":"https://web.facebook.com/groups/target/posts/123/?token=private-token"}}},"extensions":{"secret":"private-token"}}`)
	message := postResponseDiagnostic(body, 200, "target")
	if strings.Contains(message, "private") || strings.Contains(message, "token=") {
		t.Fatal("private response content leaked into logs")
	}
}
