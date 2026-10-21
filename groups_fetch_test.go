package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func groupResponse(ids []string, cursor string, more bool) string {
	edges := []map[string]any{}
	for _, id := range ids {
		edges = append(edges, map[string]any{"node": map[string]any{"__typename": "Group", "id": id, "name": "Group " + id, "url": "https://example.test/groups/" + id, "profile_picture": map[string]any{"uri": "https://example.test/picture/" + id}}})
	}
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"viewer": map[string]any{"all_joined_groups": map[string]any{"tab_groups_list": map[string]any{"edges": edges, "page_info": map[string]any{"end_cursor": cursor, "has_next_page": more}}}}}})
	return string(body)
}

func TestParseGroupsResponse(t *testing.T) {
	body := groupResponse([]string{"one", "two"}, "next", true)
	for _, prefix := range []string{"", "for (;;);"} {
		groups, next, more, err := parseGroupsResponse([]byte(prefix + body + "\n" + `{"extensions":{"is_final":true}}`))
		if err != nil || len(groups) != 2 || groups[1].Picture != "https://example.test/picture/two" || next != "next" || !more {
			t.Fatalf("groups=%v next=%q more=%v error=%v", groups, next, more, err)
		}
	}
	for _, body := range []string{`<html>login</html>`, `{"data":null}`, `{"error":1357004}`, `{"errors":[{"message":"private token must not be exposed"}]}`, `{"data":{"viewer":{"all_joined_groups":{"tab_groups_list":{"edges":[],"page_info":{}}}}}}`} {
		_, _, _, err := parseGroupsResponse([]byte(body))
		if err == nil || strings.Contains(err.Error(), "private token") {
			t.Fatalf("invalid response: error=%v", err)
		}
	}
}

func TestGroupsCaptureAndReplayIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	var mu sync.Mutex
	var requests []url.Values
	var rawBodies []string
	var wireHeaders []http.Header
	var requestCookies []string
	var queryDoc = "doc-one"
	var queryToken = "token-one"
	var profile = "profile-one"
	var repeatCursor bool
	var pauseReplay bool
	var service *GroupsService
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/groups/joins/" {
			http.SetCookie(w, &http.Cookie{Name: "c_user", Value: profile, Path: "/", HttpOnly: true})
			http.SetCookie(w, &http.Cookie{Name: "xs", Value: "session-" + profile, Path: "/", HttpOnly: true})
			fmt.Fprintf(w, `<body class="UIPage_LoggedIn"><main role="main"><div role="button" aria-haspopup="menu" aria-expanded="false" onclick="this.setAttribute('aria-expanded','true'); document.querySelector('[role=menu]').hidden=false"><span>Ordenar</span><div role="none" data-visualcompletion="ignore"></div></div></main><div role="menu" hidden><div role="menuitemradio" aria-checked="true">Current</div><div role="menuitemradio" aria-checked="false" onclick="load(); document.querySelector('[role=menu]').hidden=true">Other</div></div><script>
   function load() {
    document.cookie='ui_ready=0;path=/';
    const fields=new URLSearchParams({__user:%q,av:%q,doc_id:%q,fb_dtsg:%q,lsd:%q,fb_api_req_friendly_name:%q,variables:JSON.stringify({count:2,cursor:null,ordering:['captured_order'],scale:1}),extra:'preserve-me'});
    const keys=['__aaid','__a','__req','__hs','dpr','__ccg','__rev','__s','__hsi','__dyn','__csr','__hsdp','__hblp','__sjsp','__comet_req','jazoest','__spin_r','__spin_b','__spin_t','__crn','fb_api_caller_class','server_timestamps'];
    for(const key of keys) fields.set(key,'fresh_'+key);
    fields.set('__dyn','DYN'.repeat(25000));
    fields.set('__csr','reserved+&=雪');
    fields.append('repeated','first'); fields.append('repeated','second');
    fetch('/api/graphql/',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded','x-fb-lsd':%q,'x-local-source':'browser','priority':'u=4, i','future-header':'opaque-from-browser'},body:fields}).then(r=>r.json()).then(()=>{
     let count=0;
     const timer=setInterval(()=>{
      document.querySelector('main').append(document.createTextNode('sorted-update-'+(++count)));
      if(count===4){clearInterval(timer);document.cookie='ui_ready=1;path=/';}
     },200);
    });
   }
   </script></body>`, profile, profile, queryDoc, queryToken, queryToken, joinedGroupsQuery, queryToken)
			return
		}
		if r.URL.Path != "/api/graphql/" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		_ = r.ParseForm()
		rawBodies = append(rawBodies, string(body))
		wireHeaders = append(wireHeaders, r.Header.Clone())
		var variables struct {
			Cursor *string `json:"cursor"`
		}
		_ = json.Unmarshal([]byte(r.Form.Get("variables")), &variables)
		requests = append(requests, r.Form)
		if len(requests)%3 == 0 {
			ready, _ := r.Cookie("ui_ready")
			if ready == nil || ready.Value != "1" {
				t.Error("session captured before sorted DOM finished updating")
			}
		}
		if len(requests)%3 == 2 {
			service.mu.Lock()
			closed := service.browser == nil && service.page == nil
			service.mu.Unlock()
			if !closed {
				t.Error("Go replay began before Chrome and its page were closed")
			}
		}
		cookie, _ := r.Cookie("xs")
		cookieValue := ""
		if cookie != nil {
			cookieValue = cookie.Value
		}
		requestCookies = append(requestCookies, cookieValue)
		if r.Header.Get("x-fb-lsd") != queryToken || r.Form.Get("doc_id") != queryDoc || r.Form.Get("fb_dtsg") != queryToken {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if pauseReplay && len(requests) == 11 {
			http.SetCookie(w, &http.Cookie{Name: "c_user", Value: "switched-profile", Path: "/", HttpOnly: true})
		}
		if variables.Cursor == nil {
			_, _ = w.Write([]byte(groupResponse([]string{"one", "two"}, "next", true)))
		} else if repeatCursor {
			_, _ = w.Write([]byte(groupResponse([]string{"two"}, "next", true)))
		} else {
			_, _ = w.Write([]byte(groupResponse([]string{"two", "three"}, "", false)))
		}
	}))
	defer server.Close()
	service = &GroupsService{profileDir: t.TempDir(), chromePath: chrome, targetURL: server.URL + "/groups/joins/", headless: true}
	t.Cleanup(func() { _ = service.ServiceShutdown(); time.Sleep(500 * time.Millisecond) })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	run := func(want string) GroupsLoadStatus {
		t.Helper()
		if err := service.OpenJoinedGroups(ctx); err != nil {
			t.Fatal(err)
		}
		if err := service.page.Context(ctx).WaitLoad(); err != nil {
			t.Fatal(err)
		}
		opened, err := service.OpenGroupsSortMenu(ctx)
		if err != nil || !opened {
			t.Fatalf("open=%v error=%v", opened, err)
		}
		selected, err := service.SelectUnselectedGroupsSortOption(ctx)
		if err != nil || !selected {
			t.Fatalf("selected=%v error=%v", selected, err)
		}
		for {
			status := service.GetGroupsLoadStatus()
			if status.State == "complete" || status.State == "error" {
				if status.State != want {
					t.Fatalf("status=%+v", status)
				}
				return status
			}
			select {
			case <-ctx.Done():
				t.Fatal("loading timed out")
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	first := run("complete")
	service.mu.Lock()
	closed := service.browser == nil && service.page == nil
	service.mu.Unlock()
	if !closed {
		t.Fatal("Chrome remained open after session capture")
	}
	if state, err := service.GetLoginState(ctx); err != nil || state != "session_captured" {
		t.Fatalf("captured state: %s %v", state, err)
	}
	if len(first.Groups) != 3 {
		t.Fatalf("dedup failed: %+v", first)
	}
	restored := &GroupsService{profileDir: service.profileDir}
	if saved := restored.GetGroupsLoadStatus(); saved.State != "complete" || !reflect.DeepEqual(saved.Groups, first.Groups) {
		t.Fatalf("completed capture was not saved: %+v", saved)
	}
	verify := func(start int, user, doc, token string) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(requests) != start+3 {
			t.Fatalf("requests=%d want=%d", len(requests), start+3)
		}
		if rawBodies[start] != rawBodies[start+1] {
			t.Fatal("initial Go replay changed POST bytes")
		}
		for key, values := range wireHeaders[start] {
			if key == "Accept-Encoding" || key == "Connection" || key == "Content-Length" {
				continue
			}
			if !reflect.DeepEqual(values, wireHeaders[start+1].Values(key)) {
				t.Fatalf("initial replay changed header %s", key)
			}
		}
		if wireHeaders[start+1].Get("Priority") == "" || wireHeaders[start+1].Get("Future-Header") == "" {
			t.Fatal("wire headers were dropped")
		}
		for i, form := range requests[start:] {
			if form.Get("__user") != user || form.Get("av") != user || form.Get("doc_id") != doc || form.Get("fb_dtsg") != token || form.Get("extra") != "preserve-me" || requestCookies[start+i] != "session-"+user {
				t.Fatal("fresh profile values were not preserved")
			}
			original := requests[start]
			for key, values := range original {
				if key != "variables" && !reflect.DeepEqual(values, form[key]) {
					t.Fatalf("replay changed field %s", key)
				}
			}
			var variables struct {
				Count    int      `json:"count"`
				Cursor   *string  `json:"cursor"`
				Ordering []string `json:"ordering"`
			}
			_ = json.Unmarshal([]byte(form.Get("variables")), &variables)
			if variables.Count != 2 || len(variables.Ordering) != 1 || variables.Ordering[0] != "captured_order" {
				t.Fatal("captured variables changed")
			}
			if i < 2 && variables.Cursor != nil {
				t.Fatal("first replay did not start at initial cursor")
			}
			if i == 2 && (variables.Cursor == nil || *variables.Cursor != "next") {
				t.Fatal("pagination did not use returned cursor")
			}
		}
	}
	verify(0, "profile-one", "doc-one", "token-one")
	// Change every session field and document ID. The next load must recapture.
	mu.Lock()
	profile = "profile-two"
	queryDoc = "doc-two"
	queryToken = "token-two"
	mu.Unlock()
	second := run("complete")
	if len(second.Groups) != 3 {
		t.Fatalf("second profile: %+v", second)
	}
	verify(3, "profile-two", "doc-two", "token-two")
	mu.Lock()
	repeatCursor = true
	mu.Unlock()
	last := run("error")
	if len(last.Groups) != 2 || !strings.Contains(last.Message, "repeated") {
		t.Fatalf("cursor guard: %+v", last)
	}
	mu.Lock()
	repeatCursor = false
	pauseReplay = true
	mu.Unlock()
	switched := run("error")
	if len(switched.Groups) != 0 || !strings.Contains(switched.Message, "changed") {
		t.Fatalf("profile switch exposed old groups: %+v", switched)
	}
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	service.startGroupsCapture()
	oldLoad := service.groupsLoad
	service.mu.Unlock()
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-oldLoad.done:
	case <-time.After(3 * time.Second):
		t.Fatal("old capture was not cancelled")
	}
	if status := service.GetGroupsLoadStatus(); status.State != "idle" || len(status.Groups) != 0 {
		t.Fatalf("stale load: %+v", status)
	}
}
