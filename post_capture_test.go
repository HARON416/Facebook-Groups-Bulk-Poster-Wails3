package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestPostCaptureBrowserIntegration(t *testing.T) {
	runPostCaptureBrowserIntegration(t, "pending")
}

func TestPostCaptureRestrictionBeforeResponse(t *testing.T) {
	runPostCaptureBrowserIntegration(t, "failed")
}

func TestPostCapturePublishedWithApprovalNotice(t *testing.T) {
	runPostCaptureBrowserIntegration(t, "posted")
}

func TestPostCaptureMutationNameInHeaderOnly(t *testing.T) {
	runPostCaptureBrowserIntegration(t, "header_only")
}

func TestFacebookPostCaptureEndpoints(t *testing.T) {
	for _, raw := range []string{"https://web.facebook.com/api/graphql/", "https://www.facebook.com/api/graphql", "https://facebook.com/api/graphql/"} {
		u, _ := url.Parse(raw)
		if !isFacebookPostURL(u) {
			t.Fatalf("missed post endpoint %s", raw)
		}
	}
	for _, raw := range []string{"http://web.facebook.com/api/graphql/", "https://facebook.com.example/api/graphql/", "https://web.facebook.com:8443/api/graphql/", "https://web.facebook.com/other"} {
		u, _ := url.Parse(raw)
		if isFacebookPostURL(u) {
			t.Fatalf("accepted unrelated endpoint %s", raw)
		}
	}
}

func runPostCaptureBrowserIntegration(t *testing.T, state string) {
	formBody := postFixtureForm()
	if state == "header_only" {
		form, _ := url.ParseQuery(formBody)
		form.Del("fb_api_req_friendly_name")
		formBody = form.Encode()
		state = "posted"
	}
	restricted := state == "failed"
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/graphql/" {
			posts++
			r.ParseForm()
			if r.Form.Get("doc_id") != "captured-document" {
				t.Error("wrong operation")
			}
			if restricted {
				// Render the generic alert while the response is still in flight.
				// The API restriction must win even if the UI text is translated.
				time.Sleep(900 * time.Millisecond)
				fmt.Fprint(w, `{"data":{"story_create":null},"errors":[{"message":"A server error field_exception occured. Check server logs for details.","code":1390008,"api_error_code":368,"summary":"You Can't Use This Feature Right Now","description":{"__html":"We limit how often you can post. <a href='https://example.com'>Learn More</a>"},"allow_user_retry":false}],"extensions":{"is_final":true}}`)
				return
			}
			if state == "posted" {
				fmt.Fprint(w, `{"data":{"story_create":{"story_id":null,"post_id":null,"publishing_flow":"FALLBACK","story":{"id":"browser-post","url":"https://web.facebook.com/groups/old-group/permalink/browser-post/","if_viewer_can_learn_more_about_pending_post":null,"to":{"if_viewer_can_see_pending_post_in_mall":null}}}}}`)
			} else {
				fmt.Fprint(w, `{"data":{"story_create":{"story_id":null,"post_id":null,"publishing_flow":"FALLBACK","story":{"id":"browser-post","url":"https://web.facebook.com/groups/old-group/permalink/browser-post/","if_viewer_can_learn_more_about_pending_post":{"__typename":"Story","id":"browser-post"},"to":{"if_viewer_can_see_pending_post_in_mall":{"if_viewer_can_see_pending_post_in_mall":{"comet_pending_post_cards":{}}}}}}}}`)
			}
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "c_user", Value: "fixture-user", HttpOnly: true, Path: "/"})
		w.Header().Set("Content-Type", "text/html")
		// An approval live region is visible before the delayed request begins.
		// A success alert also remains during modal dismissal after confirmation.
		fmt.Fprintf(w, `<div role="dialog" aria-modal="true"><form method="post" onsubmit="return false"><div role="textbox" contenteditable="true" data-lexical-editor="true">First<br>Second</div><input type="submit" hidden><button type="button" id="submit" style="width:100%%" onclick='setTimeout(()=>fetch("/api/graphql/",{method:"POST",headers:{"content-type":"application/x-www-form-urlencoded","X-Fb-Friendly-Name":"ComposerStoryCreateMutation","x-fixture-header":"fresh"},body:%q}).then(()=>{if(!%t)setTimeout(()=>this.closest("[role=dialog]").remove(),650)}),650);if(%t)setTimeout(()=>document.querySelector("#error").hidden=false,800)'><span>Publier</span><div role="none" data-visualcompletion="ignore"></div></button><div role="alert">Your post is pending</div><div id="error" role="alert" hidden>Une erreur est survenue.</div></form></div>`, formBody, restricted, restricted)
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true, targetURL: server.URL}
	t.Cleanup(func() { service.ServiceShutdown(); time.Sleep(500 * time.Millisecond) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	page := service.page.Context(ctx)
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}
	capture := startPostCaptureMatching(ctx, page, "old-group", func(u *url.URL) bool { return u.String() == server.URL+"/api/graphql/" })
	defer capture.stop()
	button, err := waitPostSubmit(ctx, page, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	var phases []string
	captured, err := capture.wait(ctx, page, func(message string) { phases = append(phases, message) })
	if err != nil {
		t.Fatal(err)
	}
	if captured.Body != formBody || captured.Headers.Get("X-Fixture-Header") != "fresh" || cookieUser(captured.Headers.Get("Cookie")) != "fixture-user" {
		t.Fatal("capture lost body, headers or wire cookies")
	}
	if len(phases) == 0 {
		t.Fatal("capture did not expose progress")
	}
	result := parsePostResponse(captured.Response, captured.Status)
	if restricted {
		if result.State != "failed" || result.Code != "1390008" || !result.stop || result.Message != "Facebook has temporarily limited posting. Try again later." || posts != 1 {
			t.Fatalf("restriction was not read from the API: %#v posts=%d", result, posts)
		}
		return
	}
	if err := waitPostedComposerClosed(ctx, page); err != nil {
		t.Fatal(err)
	}
	if result.State != state || result.PostID != "browser-post" || posts != 1 {
		t.Fatalf("wrong first-post confirmation: %#v posts=%d", result, posts)
	}
}
