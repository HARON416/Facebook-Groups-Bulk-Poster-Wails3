package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

// Local fixtures reproduce the reported popup, including public timeline queries
// underneath it, and expiry after Chrome has handed the session to Go.
func TestFacebookPageSessionExpiryRecoveryIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	for _, scenario := range []struct {
		name, phase, nextUser string
		cancel                bool
	}{
		{"timeline replay", "timeline", "123", false},
		{"photo replay", "photo", "123", false},
		{"photo capture", "capture", "123", false},
		{"different account", "timeline", "456", false},
		{"cancel login wait", "timeline", "", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var expired, didExpire atomic.Bool
			var guests, logins, photoCalls atomic.Int32
			expire := func(w http.ResponseWriter) {
				expired.Store(true)
				didExpire.Store(true)
				http.SetCookie(w, &http.Cookie{Name: "c_user", Path: "/", MaxAge: -1})
			}
			popup := func(w http.ResponseWriter) {
				// Authenticated-looking backdrop must not override the popup's form.
				fmt.Fprintf(w, `<body>%s<div role="dialog" aria-modal="true"><form id="login_popup_cta_form" action="/login/device-based/regular/login/"><input name="email"><input name="pass" type="password"><button id="signin" type="button" onclick="location.href='/login/complete'">Ingia</button></form></div><script>fetch('/api/graphql/',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams({__user:'0',doc_id:'guest-doc',fb_api_req_friendly_name:%q,variables:JSON.stringify({id:'456',feedLocation:'TIMELINE',cursor:null})})});</script>`, facebookAccountNavigationFixture, facebookTimelineQuery)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				switch r.URL.Path {
				case "/login/complete":
					user := r.URL.Query().Get("user")
					if user == "" {
						user = "123"
					}
					http.SetCookie(w, &http.Cookie{Name: "c_user", Value: user, Path: "/", HttpOnly: true})
					logins.Add(1)
					expired.Store(false)
					fmt.Fprint(w, `<body>`, facebookAccountNavigationFixture, `</body>`)
				case "/expire":
					expire(w)
					popup(w)
				case "/swiftlineMotors":
					cookie, _ := r.Cookie("c_user")
					if cookie == nil || expired.Load() {
						popup(w)
						return
					}
					expirePhoto := scenario.phase == "capture" && !didExpire.Load()
					fmt.Fprintf(w, `<body>%s<main role="main"><a href="/photo/?fbid=100&set=pcb.1" onclick="event.preventDefault();openPhoto()"><img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw=="></a></main><script>
      const user=%q;function send(name,variables){return fetch('/api/graphql/',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams({__user:user,av:user,doc_id:%q,fb_api_req_friendly_name:name,variables:JSON.stringify(variables)})})}
      function openPhoto(){if(%t){location.href='/expire';return}send(%q,{nodeID:'100',mediasetToken:'pcb.1',scale:1})}
      send(%q,{id:'456',feedLocation:'TIMELINE',cursor:'browser',count:2});</script>`, facebookAccountNavigationFixture, cookie.Value, fmt.Sprintf("fresh-%d", logins.Load()), expirePhoto, facebookPhotoQuery, facebookTimelineQuery)
				case "/api/graphql/":
					w.Header().Set("Content-Type", "application/json")
					if err := r.ParseForm(); err != nil {
						t.Error(err)
						return
					}
					if r.Form.Get("__user") == "0" {
						guests.Add(1)
						fmt.Fprint(w, photoTimelineResponse([]string{"1"}, "end", false))
						return
					}
					if r.Form.Get("doc_id") != fmt.Sprintf("fresh-%d", logins.Load()) {
						t.Error("import reused request values from before login")
					}
					if r.Form.Get("fb_api_req_friendly_name") == facebookPhotoQuery {
						calls := photoCalls.Add(1)
						if scenario.phase == "photo" && calls == 2 && !didExpire.Load() {
							expire(w)
						}
						fmt.Fprint(w, `{"data":{"currMedia":{"__typename":"Photo","id":"100","image":{"uri":"https://cdn.example.test/full.jpg","width":1600,"height":1200}}},"extensions":{"is_final":true}}`)
						return
					}
					if scenario.phase == "timeline" && strings.Contains(r.Form.Get("variables"), `"cursor":null`) && !didExpire.Load() {
						expire(w)
					}
					fmt.Fprint(w, photoTimelineResponse([]string{"1"}, "end", false))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
			defer func() { _ = s.ServiceShutdown(); time.Sleep(500 * time.Millisecond) }()
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			start, err := s.startFacebookPageFetch(ctx, server.URL+"/swiftlineMotors", 1, func(u *url.URL) bool { return u.String() == server.URL+"/api/graphql/" })
			if err != nil {
				t.Fatal(err)
			}
			waitFor := func(ready func(FacebookPageFetchStatus) bool) FacebookPageFetchStatus {
				t.Helper()
				for {
					status := s.GetFacebookPageFetchStatus()
					if ready(status) {
						return status
					}
					if status.State == "error" || status.State == "complete" || status.State == "cancelled" {
						t.Fatalf("import ended before expected login state: %+v", status)
					}
					select {
					case <-ctx.Done():
						t.Fatalf("import did not settle: %+v", status)
					case <-time.After(25 * time.Millisecond):
					}
				}
			}
			status := waitFor(func(status FacebookPageFetchStatus) bool { return status.State == "logged_out" })
			if !strings.Contains(status.Message, "signing in") || guests.Load() == 0 {
				t.Fatalf("popup did not show login guidance or exercise public query: %+v", status)
			}
			s.mu.Lock()
			page := s.page
			s.mu.Unlock()
			page.Context(ctx).MustElement("#signin").MustClick()
			status = waitFor(func(status FacebookPageFetchStatus) bool { return didExpire.Load() && status.State == "logged_out" })
			if status.RunID != start.RunID || !strings.Contains(status.Message, "automatically") {
				t.Fatalf("expiry lost the active import or login message: %+v", status)
			}
			if scenario.cancel {
				s.CancelFacebookPageFetch(start.RunID)
				s.mu.Lock()
				load := s.facebookPageLoad
				s.mu.Unlock()
				select {
				case <-load.done:
				case <-ctx.Done():
					t.Fatal("cancel left login wait running")
				}
				if s.GetFacebookPageFetchStatus().State != "cancelled" {
					t.Fatal("cancelled import restarted")
				}
				return
			}
			s.mu.Lock()
			page = s.page
			s.mu.Unlock()
			if err := page.Context(ctx).Navigate(server.URL + "/login/complete?user=" + scenario.nextUser); err != nil {
				t.Fatal(err)
			}
			status = waitFor(func(status FacebookPageFetchStatus) bool {
				return status.State == "complete" || status.State == "error"
			})
			if scenario.nextUser != "123" {
				if status.State != "error" || !strings.Contains(status.Message, "does not match") {
					t.Fatalf("import accepted a different account: %+v", status)
				}
				return
			}
			if status.State != "complete" || status.RunID != start.RunID || len(status.Posts) != 1 || status.Posts[0].ID != "1" || len(status.Posts[0].Images) != 1 || status.Posts[0].Images[0] != "https://cdn.example.test/full.jpg" || logins.Load() != 2 {
				t.Fatalf("import did not resume automatically with fresh photo data: %+v, logins=%d", status, logins.Load())
			}
		})
	}
}
