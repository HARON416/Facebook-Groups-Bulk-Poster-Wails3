package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func timelineResponse(ids []string, next string, more bool) string {
	edges := []any{}
	for _, id := range ids {
		edges = append(edges, map[string]any{"node": map[string]any{"__typename": "Story", "id": "story-" + id, "post_id": id, "url": "https://web.facebook.com/swiftlineMotors/posts/" + id, "comet_sections": map[string]any{"content": map[string]any{"story": map[string]any{"message": map[string]any{"text": "Post " + id}, "attachments": []any{map[string]any{"media": map[string]any{"original_image": map[string]any{"uri": "https://cdn.example.test/" + id + ".jpg"}}}}}}}}})
	}
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"node": map[string]any{"timeline_list_feed_units": map[string]any{"edges": edges, "page_info": map[string]any{"end_cursor": next, "has_next_page": more}}}}})
	return string(body)
}

func TestFacebookPageURL(t *testing.T) {
	for _, raw := range []string{"https://web.facebook.com/swiftlineMotors", "https://WEB.FACEBOOK.COM/swiftlineMotors", "http://m.facebook.com/swiftlineMotors/?ref=tracking", "https://www.facebook.com/profile.php?id=123", "https://www.facebook.com/profile.php/?id=123", "https://facebook.com/pages/Some-Page/123"} {
		if _, err := facebookPageURL(raw); err != nil {
			t.Errorf("valid URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"https://facebook.com.evil.test/page", "https://evilfacebook.com/page", "https://user:pass@facebook.com/page", "https://facebook.com:444/page", "file:///tmp/page", "https://facebook.com/", "https://facebook.com/groups", "https://facebook.com/groups/123", "https://facebook.com/page/posts/123", "https://facebook.com/profile.php?id=wrong"} {
		if _, err := facebookPageURL(raw); err == nil {
			t.Errorf("accepted invalid URL %q", raw)
		}
	}
}

func TestTimelineLocationRedirects(t *testing.T) {
	location := &timelineLocation{provided: "https://web.facebook.com/profile.php?id=123", current: "https://web.facebook.com/profile.php?id=123"}
	location.redirect("https://www.facebook.com/profile.php?id=123", "https://www.facebook.com/swiftlineMotors/")
	if !location.matches("https://web.facebook.com/swiftlineMotors") || location.matches("https://web.facebook.com/anotherPage") {
		t.Fatal("canonical Facebook redirect was not followed")
	}
	location.redirect("https://web.facebook.com/unrelated", "https://web.facebook.com/anotherPage")
	if location.matches("https://web.facebook.com/anotherPage") {
		t.Fatal("unrelated redirect changed the target")
	}
	location.redirect("https://web.facebook.com/swiftlineMotors", "https://example.test/page")
	if location.matches("https://example.test/page") {
		t.Fatal("external redirect changed the target")
	}
}

func TestTimelineResponse(t *testing.T) {
	base := timelineResponse([]string{"1"}, "next", true)
	patch := `{"path":["node","timeline_list_feed_units","edges",0,"node","comet_sections","content","story","message"],"data":{"text":"Deferred text"}}`
	posts, next, more, err := parseFacebookPageResponse([]byte("for (;;);" + base + "\n" + patch + "\n" + `{"extensions":{"is_final":true}}`))
	if err != nil || len(posts) != 1 || posts[0].ID != "1" || posts[0].Text != "Deferred text" || len(posts[0].Images) != 1 || next != "next" || !more {
		t.Fatalf("posts=%+v next=%s more=%v err=%v", posts, next, more, err)
	}
	stream := `{"path":["node","timeline_list_feed_units","edges",1],"data":{"node":{"__typename":"Story","post_id":"2","message":{"text":"Streamed post"}}}}`
	posts, _, _, err = parseFacebookPageResponse([]byte(base + "\n" + stream))
	if err != nil || len(posts) != 2 || posts[1].ID != "2" {
		t.Fatalf("stream: %+v %v", posts, err)
	}
	for _, invalid := range []string{`<html>login</html>`, `{"data":null}`, `{"errors":[{"message":"private session text"}]}`, `{"data":{"node":{"comments":{"edges":[],"page_info":{"has_next_page":false}}}}}`, strings.Replace(base, `"has_next_page":true`, `"has_next_page":null`, 1), base + "\n" + `{"error":1357004}`} {
		if _, _, _, err := parseFacebookPageResponse([]byte(invalid)); err == nil || strings.Contains(err.Error(), "private session") {
			t.Fatalf("invalid response err=%v", err)
		}
	}
}

func TestTimelineBodyPreservesCapturedFields(t *testing.T) {
	raw := "__dyn=opaque%2fvalue&repeated=one&variables=" + url.QueryEscape(`{"id":"123","count":2,"cursor":"scroll-position","opaque_integer":18446744073709551615}`) + "&repeated=two&fb_dtsg=fresh%3atoken"
	body, err := timelineBody(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(body, "__dyn=opaque%2fvalue&repeated=one&variables=") || !strings.HasSuffix(body, "&repeated=two&fb_dtsg=fresh%3atoken") {
		t.Fatal("captured form fields were changed")
	}
	form, _ := url.ParseQuery(body)
	if !strings.Contains(form.Get("variables"), `"opaque_integer":18446744073709551615`) || !strings.Contains(form.Get("variables"), `"cursor":null`) {
		t.Fatal("opaque variables changed")
	}
	for _, invalid := range []string{"fb_dtsg=fresh", "variables=null", "variables=%zz", "variables=%7B%7D&variables=%7B%7D"} {
		if _, err := timelineBody(invalid, nil); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
}

func TestTimelineReplay(t *testing.T) {
	for _, scenario := range []string{"target", "end", "repeat", "session", "error", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, _ := io.ReadAll(r.Body)
				form, _ := url.ParseQuery(string(body))
				var variables map[string]json.RawMessage
				_ = json.Unmarshal([]byte(form.Get("variables")), &variables)
				if form.Get("doc_id") != "fresh-doc" || form.Get("fb_dtsg") != "fresh-token" || r.Header.Get("X-Fb-Lsd") != "fresh-lsd" || len(form["repeated"]) != 2 {
					t.Error("fresh captured fields were lost")
				}
				if calls == 1 && string(variables["cursor"]) != "null" {
					t.Error("did not start from beginning of timeline")
				}
				if calls > 1 {
					cookie, _ := r.Cookie("xs")
					if cookie == nil || cookie.Value != "rotated" {
						t.Error("updated cookie was not replayed")
					}
				}
				http.SetCookie(w, &http.Cookie{Name: "xs", Value: "rotated", Path: "/"})
				if scenario == "error" {
					fmt.Fprint(w, `{"errors":[{"message":"private token"}]}`)
					return
				}
				if scenario == "session" {
					http.SetCookie(w, &http.Cookie{Name: "c_user", Value: "different-user", Path: "/"})
				}
				if calls == 1 {
					fmt.Fprint(w, timelineResponse([]string{"1", "2"}, "next", scenario != "end"))
					return
				}
				if scenario == "repeat" {
					fmt.Fprint(w, timelineResponse([]string{"2"}, "next", true))
					return
				}
				fmt.Fprint(w, timelineResponse([]string{"2", "3", "4"}, "last", false))
			}))
			defer server.Close()
			request := timelineRequest{URL: server.URL + "/api/graphql/", User: "123", Headers: http.Header{"Cookie": {"c_user=123; xs=original"}, "Content-Type": {"application/x-www-form-urlencoded"}, "X-Fb-Lsd": {"fresh-lsd"}}, Body: "doc_id=fresh-doc&fb_dtsg=fresh-token&repeated=one&repeated=two&variables=" + url.QueryEscape(`{"id":"456","cursor":"browser-position","count":2}`)}
			cookies := []*proto.NetworkCookie{{Name: "c_user", Value: "123", Path: "/"}, {Name: "xs", Value: "original", Path: "/"}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancel" {
				cancel()
			}
			posts, err := replayFacebookTimeline(ctx, request, cookies, 3, nil)
			if scenario == "target" {
				if err != nil || len(posts) != 3 || posts[2].ID != "3" || calls != 2 {
					t.Fatalf("posts=%v calls=%d err=%v", posts, calls, err)
				}
			} else if scenario == "end" {
				if err != nil || len(posts) != 2 || calls != 1 {
					t.Fatalf("end: posts=%v calls=%d err=%v", posts, calls, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "private token") {
				t.Fatalf("expected sanitized failure, got %v", err)
			}
		})
	}
}

func TestFacebookPageLoginCaptureReplayIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	var mu sync.Mutex
	var cursors []string
	var docs []string
	var photoDocs []string
	var photoReloads int
	doc := "fresh-doc-one"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/complete" {
			http.SetCookie(w, &http.Cookie{Name: "c_user", Value: "123", Path: "/", HttpOnly: true, Expires: time.Now().Add(time.Hour)})
			http.SetCookie(w, &http.Cookie{Name: "xs", Value: "session", Path: "/", HttpOnly: true, Expires: time.Now().Add(time.Hour)})
			fmt.Fprint(w, `<body class="UIPage_LoggedIn">Signed in</body>`)
			return
		}
		if r.URL.Path == "/photo/" {
			mu.Lock()
			currentDoc := doc
			photoReloads++
			mu.Unlock()
			fmt.Fprintf(w, `<body class="UIPage_LoggedIn"><button aria-label="Next photo" onclick="openNext()">Next</button><script>function openNext() {
 const fields=new URLSearchParams({__user:'123',av:'123',doc_id:%q+'-photo',fb_dtsg:'fresh-photo-token',fb_api_req_friendly_name:'CometPhotoRootContentQuery',variables:JSON.stringify({nodeID:'401',mediasetToken:'pcb.4',scale:1,renderLocation:'comet_media_viewer'})});
 fetch('/api/graphql/',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded','x-fb-friendly-name':'CometPhotoRootContentQuery','x-fb-lsd':'photo-lsd'},body:fields});
 }</script></body>`, currentDoc)
			return
		}
		if r.URL.Path == "/swiftlineMotors" {
			if cookie, _ := r.Cookie("c_user"); cookie == nil {
				fmt.Fprint(w, `<body class="UIPage_LoggedOut"><form id="login_form"><button type="button" onclick="location.href='/login/complete'">Sign in</button></form></body>`)
				return
			}
			mu.Lock()
			currentDoc := doc
			mu.Unlock()
			fmt.Fprintf(w, `<body class="UIPage_LoggedIn" style="height:10000px"><main role="main">Timeline<a href="/photo/?fbid=400&set=pcb.4" onclick="event.preventDefault();openPhoto()"><img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw=="></a></main><script>
 function openPhoto() {
 // Simulate Relay's cached viewer: clicking changes the URL but emits no query.
 if (%q === 'fresh-doc-two') { history.pushState({},'', '/photo/?fbid=400&set=pcb.4'); return; }
 const fields=new URLSearchParams({__user:'123',av:'123',doc_id:%q+'-photo',fb_dtsg:'fresh-photo-token',fb_api_req_friendly_name:'CometPhotoRootContentQuery',variables:JSON.stringify({nodeID:'401',mediasetToken:'pcb.4',scale:1,renderLocation:'comet_media_viewer'})});
 fetch('/api/graphql/',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded','x-fb-friendly-name':'CometPhotoRootContentQuery','x-fb-lsd':'photo-lsd'},body:fields});
 }

 let sent=false; addEventListener('scroll',()=>{if(sent)return;sent=true;
 const fields=new URLSearchParams({__user:'123',av:'123',doc_id:%q,fb_dtsg:'fresh-token',fb_api_req_friendly_name:%q,variables:JSON.stringify({id:'456',feedLocation:'TIMELINE',cursor:'browser-position',count:2,unknown:{preserve:true}})});
 fields.set('__dyn','DYN'.repeat(25000));fields.append('repeated','one');fields.append('repeated','two');
 fetch('/api/graphql/',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded','x-fb-friendly-name':%q,'x-fb-lsd':'fresh-lsd','x-future':'preserved'},body:fields});});</script></body>`, currentDoc, currentDoc, currentDoc, facebookTimelineQuery, facebookTimelineQuery)
			return
		}
		if r.URL.Path != "/api/graphql/" {
			http.NotFound(w, r)
			return
		}
		data, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(data))
		if form.Get("fb_api_req_friendly_name") == facebookPhotoQuery {
			mu.Lock()
			photoDocs = append(photoDocs, form.Get("doc_id"))
			mu.Unlock()
			if form.Get("fb_dtsg") != "fresh-photo-token" || r.Header.Get("X-Fb-Lsd") != "photo-lsd" {
				t.Error("fresh photo session fields lost")
			}
			var vars map[string]any
			_ = json.Unmarshal([]byte(form.Get("variables")), &vars)
			id, _ := vars["nodeID"].(string)
			expectedCollection := "pcb." + strings.TrimSuffix(id, "00")
			if id == "401" {
				expectedCollection = "pcb.4"
			}
			if vars["mediasetToken"] != expectedCollection {
				t.Error("photo collection was not updated")
			}
			fmt.Fprintf(w, `{"data":{"currMedia":{"__typename":"Photo","id":%q,"image":{"uri":%q,"width":1600,"height":1200}}},"extensions":{"is_final":true}}`, id, "https://cdn.example.test/"+id+"-viewer.jpg")
			return
		}
		var variables struct {
			Cursor *string `json:"cursor"`
		}
		_ = json.Unmarshal([]byte(form.Get("variables")), &variables)
		cursor := "null"
		if variables.Cursor != nil {
			cursor = *variables.Cursor
		}
		mu.Lock()
		cursors = append(cursors, cursor)
		docs = append(docs, form.Get("doc_id"))
		mu.Unlock()
		if len(form.Get("__dyn")) != 75000 || r.Header.Get("X-Future") != "preserved" || r.Header.Get("X-Fb-Lsd") != "fresh-lsd" || form.Get("fb_dtsg") != "fresh-token" {
			t.Error("fresh wire values were not preserved")
		}
		w.Header().Set("Content-Type", "application/json")
		switch cursor {
		case "null":
			fmt.Fprint(w, photoTimelineResponse([]string{"1", "2"}, "next", true))
		case "next":
			fmt.Fprint(w, photoTimelineResponse([]string{"2", "3"}, "last", true))
		default:
			fmt.Fprint(w, photoTimelineResponse([]string{"4", "5"}, "end", false))
		}
	}))
	defer server.Close()
	// Browser.Close acknowledges the command before Chrome finishes flushing its
	// profile. Retry removal at teardown rather than racing t.TempDir's cleanup.
	profileDir, err := os.MkdirTemp("", "postika-photo-capture-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			err := os.RemoveAll(profileDir)
			if err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Errorf("remove browser fixture profile: %v", err)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	s := &GroupsService{profileDir: profileDir, chromePath: chrome, headless: true}
	defer s.ServiceShutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	match := func(u *url.URL) bool { return u.String() == server.URL+"/api/graphql/" }
	start, err := s.startFacebookPageFetch(ctx, server.URL+"/swiftlineMotors", 4, match)
	if err != nil {
		t.Fatal(err)
	}
	waitFor := func(state string) FacebookPageFetchStatus {
		t.Helper()
		for {
			status := s.GetFacebookPageFetchStatus()
			if status.State == state {
				return status
			}
			if status.State == "error" {
				t.Fatalf("fetch failed: %s", status.Message)
			}
			select {
			case <-ctx.Done():
				t.Fatalf("waiting for %s, got %+v", state, status)
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	waitFor("logged_out")
	s.mu.Lock()
	page := s.page
	s.mu.Unlock()
	page.Context(ctx).MustElement("button").MustClick()
	status := waitFor("complete")
	if status.RunID != start.RunID || len(status.Posts) != 4 || status.Posts[0].ID != "1" || status.Posts[3].ID != "4" {
		t.Fatalf("unexpected posts %+v", status)
	}
	if !reflect.DeepEqual(status.Posts[0].Images, []string{"https://cdn.example.test/100-viewer.jpg"}) {
		t.Fatalf("viewer images: %v", status.Posts[0].Images)
	}
	mu.Lock()
	gotCursors := append([]string{}, cursors...)
	mu.Unlock()
	if !reflect.DeepEqual(gotCursors, []string{"browser-position", "null", "next", "last"}) {
		t.Fatalf("cursors: %v", gotCursors)
	}
	s.mu.Lock()
	closed := s.browser == nil
	s.mu.Unlock()
	if !closed {
		t.Fatal("browser was not closed after snapshot")
	}
	// A second fetch must obtain a new document ID rather than reuse credentials.
	mu.Lock()
	doc = "fresh-doc-two"
	cursors = nil
	docs = nil
	photoDocs = nil
	mu.Unlock()
	if _, err := s.startFacebookPageFetch(ctx, server.URL+"/swiftlineMotors", 1, match); err != nil {
		t.Fatal(err)
	}
	status = waitFor("complete")
	if len(status.Posts) != 1 {
		t.Fatalf("second fetch: %+v", status)
	}
	mu.Lock()
	if photoReloads != 1 {
		t.Fatalf("expected one photo-page reload before gallery navigation, got %d", photoReloads)
	}
	if len(photoDocs) != 2 {
		t.Fatalf("expected browser capture and photo replay, got %v", photoDocs)
	}
	for _, value := range photoDocs {
		if value != "fresh-doc-two-photo" {
			t.Fatalf("stale photo query: %s", value)
		}
	}
	for _, value := range docs {
		if value != "fresh-doc-two" {
			t.Fatalf("stale document ID: %s", value)
		}
	}
	mu.Unlock()
	third, err := s.startFacebookPageFetch(ctx, server.URL+"/swiftlineMotors", 4, match)
	if err != nil {
		t.Fatal(err)
	}
	s.CancelFacebookPageFetch(start.RunID)
	if s.GetFacebookPageFetchStatus().State == "cancelled" {
		t.Fatal("an older run cancelled the new fetch")
	}
	s.CancelFacebookPageFetch(third.RunID)
	s.mu.Lock()
	load := s.facebookPageLoad
	s.mu.Unlock()
	select {
	case <-load.done:
	case <-ctx.Done():
		t.Fatal("cancelled capture did not exit")
	}
	if s.GetFacebookPageFetchStatus().State != "cancelled" {
		t.Fatal("cancelled fetch was overwritten")
	}
}
