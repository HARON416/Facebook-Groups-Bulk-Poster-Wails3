package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

var uiArtifactDir = flag.String("ui-artifacts", "", "Optional directory for local UI screenshots")

func TestPostingLogsUI(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration; build frontend first")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	s := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	var copiedResponse atomic.Value
	var logRequests atomic.Int64
	job, cancelJob := testPostingContext(s)
	defer cancelJob()
	s.postMu.Lock()
	s.postingRunID = 1
	s.posting.Total = 300
	s.postingStarted = time.Now()
	s.postStatus = "Posting fixture groups…"
	for i := 1; i <= 151; i++ {
		s.appendPostingLogLocked("info", fmt.Sprintf("Activity %d", i))
	}
	s.postMu.Unlock()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for {
			if err := postingCheckpoint(job); err != nil {
				s.postMu.Lock()
				s.posting.State = "stopped"
				s.postingCancel = nil
				s.postStatus = "Posting stopped."
				s.appendPostingLogLocked("info", s.postStatus)
				s.postMu.Unlock()
				return
			}
			select {
			case <-job.Done():
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()
	defer func() { cancelJob(); <-workerDone }()
	files := http.FileServer(http.Dir("frontend/dist"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/create" {
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				return
			}
			var variables map[string]any
			if err := json.Unmarshal([]byte(r.Form.Get("variables")), &variables); err != nil {
				t.Error(err)
				return
			}
			group := stringValue(record(record(variables["input"])["audience"])["to_id"])
			w.Header().Set("Content-Type", "application/json")
			switch group {
			case "published-group":
				fmt.Fprint(w, `{"data":{"story_create":{"post_id":"replayed-post","story":{"to":{"id":"published-group","if_viewer_can_see_pending_post_in_mall":null},"if_viewer_can_learn_more_about_pending_post":null}}}}`)
			case "pending-group":
				fmt.Fprint(w, `{"data":{"story_create":{"story":{"id":"replayed-pending-post","to":{"id":"pending-group"},"if_viewer_can_learn_more_about_pending_post":{}}}}}`)
			case "failed-group":
				fmt.Fprint(w, `{"data":{"story_create":null},"errors":[{"code":1357006,"api_error_code":200}]}`)
			default:
				t.Errorf("unexpected replay group %q", group)
			}
			return
		}
		if r.URL.Path != "/wails/runtime" {
			files.ServeHTTP(w, r)
			return
		}
		var call struct {
			Args struct {
				MethodID   uint32            `json:"methodID"`
				MethodName string            `json:"methodName"`
				Args       []json.RawMessage `json:"args"`
			} `json:"args"`
		}
		json.NewDecoder(r.Body).Decode(&call)
		w.Header().Set("Content-Type", "application/json")
		var result any
		if call.Args.MethodID == 2582885906 {
			_ = json.NewEncoder(w).Encode(ProfileList{Selected: "default", Profiles: []ProfileActivity{{AccountProfile: AccountProfile{ID: "default", Name: "Default profile"}}}})
			return
		}
		if len(call.Args.Args) > 0 {
			call.Args.Args = call.Args.Args[1:]
		}
		if call.Args.MethodID == 1812541693 || call.Args.MethodName == "main.ProfileService.GetSettings" {
			result, _ = s.GetSettings()
			json.NewEncoder(w).Encode(result)
			return
		}
		if call.Args.MethodID == 1578609276 || call.Args.MethodName == "main.ProfileService.SaveSettings" {
			var settings AppSettings
			var key string
			var clearKey bool
			json.Unmarshal(call.Args.Args[0], &settings)
			json.Unmarshal(call.Args.Args[1], &key)
			json.Unmarshal(call.Args.Args[2], &clearKey)
			if err := s.SaveSettings(settings, key, clearKey); err != nil {
				t.Error(err)
				w.WriteHeader(500)
			}
			json.NewEncoder(w).Encode(nil)
			return
		}
		switch call.Args.MethodID {
		case 558896662:
			var runID uint64
			var logID int
			json.Unmarshal(call.Args.Args[0], &runID)
			json.Unmarshal(call.Args.Args[1], &logID)
			text, err := s.postingResponse(runID, logID)
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
			}
			copiedResponse.Store(text)
		case 3106071693:
			logRequests.Add(1)
			var page, size int
			json.Unmarshal(call.Args.Args[0], &page)
			json.Unmarshal(call.Args.Args[1], &size)
			result = s.GetPostingLogs(page, size)
		case 3053059640:
			s.PausePosting()
		case 2435194967:
			s.ResumePosting()
		case 186143472:
			s.CancelPosting()
		case 1478001064:
			result = s.GetPostingStatus()
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	launch := launcher.New().Bin(chrome).UserDataDir(t.TempDir()).Headless(true)
	control, err := launch.Launch()
	if err != nil {
		t.Fatal(err)
	}
	browser := rod.New().ControlURL(control).Context(ctx).NoDefaultDevice()
	if err = browser.Connect(); err != nil {
		launch.Kill()
		t.Fatal(err)
	}
	defer func() { browser.Context(context.Background()).Close(); time.Sleep(500 * time.Millisecond) }()
	page, err := browser.Page(proto.TargetCreateTarget{URL: server.URL + "/?view=posting-logs"})
	if err != nil {
		t.Fatal(err)
	}
	if err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1100, Height: 750, DeviceScaleFactor: 1}); err != nil {
		t.Fatal(err)
	}
	await := func(predicate string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			value, err := page.Eval(predicate)
			if err == nil && value.Value.Bool() {
				return
			}
			select {
			case <-ctx.Done():
				t.Fatalf("logs UI timed out: %s", predicate)
			case <-time.After(25 * time.Millisecond):
				if time.Now().After(deadline) {
					snapshot, _ := page.Eval(`() => ({viewport:[innerWidth,innerHeight],document:[document.documentElement.scrollWidth,document.documentElement.scrollHeight],layout:Object.fromEntries(['.auxiliary-window','.auxiliary-content','.logs-window','.logs-window header','.feed-heading','main','footer'].map(s=>{const e=document.querySelector(s),r=e?.getBoundingClientRect();return [s,r?{top:r.top,bottom:r.bottom,height:r.height,clientHeight:e.clientHeight}:null]}))})`)
					t.Fatalf("logs UI did not settle: %s, snapshot: %v", predicate, snapshot)
				}
			}
		}
	}
	eval := func(script string) {
		t.Helper()
		if _, err := page.Eval(script); err != nil {
			t.Fatal(err)
		}
	}
	await(`() => document.querySelector('.timeline')?.textContent.includes('Activity 151') && document.querySelector('[aria-current="page"]')?.textContent==='4'`)
	await(`async () => {await document.fonts.ready; return getComputedStyle(document.querySelector('.timeline')).fontFamily.includes('InterVariable') && getComputedStyle(document.querySelector('.controls button')).fontFamily.includes('InterVariable') && [...document.fonts].some(font => font.family === 'InterVariable' && font.status === 'loaded');}`)
	await(`() => document.querySelector('.level')?.textContent==='INFO'`)
	// Native auxiliary close hides the webview. No polling should survive it;
	// reopen resumes exactly one session and immediately catches up with logs.
	for _, lifecycle := range [][2]string{{"postika:window-hidden", "postika:window-shown"}, {"pagehide", "pageshow"}} {
		eval(fmt.Sprintf(`() => window.dispatchEvent(new Event(%q))`, lifecycle[0]))
		time.Sleep(100 * time.Millisecond) // Let any request already in flight finish.
		before := logRequests.Load()
		time.Sleep(1100 * time.Millisecond)
		if after := logRequests.Load(); after != before {
			t.Fatalf("polling continued after %s: %d -> %d requests", lifecycle[0], before, after)
		}
		eval(fmt.Sprintf(`() => window.dispatchEvent(new Event(%q))`, lifecycle[1]))
		deadline := time.Now().Add(3 * time.Second)
		for logRequests.Load() == before && time.Now().Before(deadline) {
			time.Sleep(25 * time.Millisecond)
		}
		if logRequests.Load() == before {
			t.Fatalf("polling did not resume after %s", lifecycle[1])
		}
	}
	eval(`() => document.querySelector('nav button[aria-label="Previous page"]').click()`)
	await(`() => document.querySelector('[aria-current="page"]')?.textContent==='3' && document.querySelectorAll('.timeline-item').length===50`)
	// Incoming logs must not jump away from a page the user is reading.
	s.postMu.Lock()
	for i := 152; i <= 204; i++ {
		s.appendPostingLogLocked("info", fmt.Sprintf("Activity %d", i))
	}
	s.postMu.Unlock()
	await(`() => document.querySelector('.pagination p')?.textContent.includes('204 logs') && document.querySelector('[aria-current="page"]')?.textContent==='3'`)
	eval(`() => document.querySelector('[aria-label="Page 5"]').click()`)
	await(`() => document.querySelector('.timeline')?.textContent.includes('Activity 204')`)
	// Exercise real local API replays and the same outcome logger used by the
	// posting queue, then confirm each result reaches the polling logs window.
	jar, _ := cookiejar.New(nil)
	endpoint, _ := url.Parse(server.URL)
	jar.SetCookies(endpoint, []*http.Cookie{{Name: "c_user", Value: "fixture-user", Path: "/"}})
	replay := &postReplay{client: &http.Client{Jar: jar}, user: "fixture-user", post: capturedPost{URL: server.URL + "/create", Body: postFixtureForm(), Headers: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}}}
	for _, test := range []struct{ id, name, state, visible string }{
		{"published-group", "Published replay group", "posted", "Posted successfully to Published replay group."},
		{"pending-group", "Pending replay group", "pending", "Pending replay group: Your post is pending"},
		{"failed-group", "Failed replay group", "failed", "Failed replay group: Facebook could not post to this group."},
	} {
		outcome := replay.create(job, "First\nSecond", nil, test.id)
		if outcome.State != test.state || outcome.stop {
			t.Fatalf("unexpected replay result: %#v", outcome)
		}
		outcome.Group = Group{ID: test.id, Name: test.name}
		s.recordOutcome(outcome)
		visible, _ := json.Marshal(test.visible)
		await(fmt.Sprintf(`() => document.querySelector('.timeline')?.textContent.includes(%s)`, visible))
	}
	await(`() => document.querySelector('.progress-label')?.textContent.includes('3/300 groups processed') && document.querySelector('.counts')?.textContent.includes('297')`)
	await(`() => [...document.querySelectorAll('.log-title')].some(e=>e.textContent==='Published to Published replay group.') && [...document.querySelectorAll('.log-title')].some(e=>e.textContent==='Pending replay group: Awaiting admin approval.')`)
	eval(`() => [...document.querySelectorAll('.log-card')].find(e=>e.querySelector('.log-title').textContent==='Facebook response received').querySelector('summary').click()`)
	await(`() => document.querySelector('.debug-details[open] .technical')?.textContent.includes('API confirmation: HTTP 200')`)
	time.Sleep(600 * time.Millisecond)
	await(`() => document.querySelector('.debug-details[open] .technical')?.textContent.includes('requested group published-group')`)
	// The response diagnostic must also be attached to the actual failure row,
	// so debugging it does not require finding a separate earlier debug entry.
	eval(`() => document.querySelector('.timeline-item.error .debug-details summary').click()`)
	await(`() => document.querySelector('.timeline-item.error .debug-details[open] .technical')?.textContent.includes('API confirmation: HTTP 200')`)
	await(`() => {const card=document.querySelector('.timeline-item.error .log-card'), badge=document.querySelector('.timeline-item.error .level'); return card && getComputedStyle(card).borderLeftColor===getComputedStyle(badge).color && badge.textContent==='FAILED';}`)
	await(`() => {const pending=document.querySelector('.timeline-item.warning .log-card'), published=document.querySelector('.timeline-item.published .log-card'), debug=document.querySelector('.timeline-item.debug .log-card'),cards=[pending,published,debug]; return pending?.querySelector('.level').textContent==='PENDING' && pending.querySelector('.severity').textContent==='INFO' && published?.querySelector('.level').textContent==='PUBLISHED' && debug?.querySelector('.level').textContent==='DEBUG' && cards.every(card=>getComputedStyle(card).borderLeftColor===getComputedStyle(card.querySelector('.level')).color) && new Set(cards.map(card=>getComputedStyle(card).borderLeftColor)).size===3;}`)
	eval(`() => [...document.querySelectorAll('.timeline-item')].find(r=>r.textContent.includes('Posted successfully to Published replay group.')).querySelector('[aria-label="Copy Facebook response"]').click()`)
	await(`() => document.querySelector('[role="status"].copy-feedback')?.textContent==='Facebook response copied.'`)
	text, ok := copiedResponse.Load().(string)
	if !ok || !strings.Contains(text, `"post_id": "replayed-post"`) || !strings.Contains(text, `"id": "published-group"`) {
		t.Fatalf("copy action returned the wrong response: %q", text)
	}
	s.beginPostingGroup(Group{ID: "next-group", Name: "Next replay group"})
	countdownDone := make(chan error, 1)
	go func() { countdownDone <- s.waitPostingDelay(job, 62*time.Second) }()
	defer func() { cancelJob(); <-countdownDone }()
	await(`() => document.querySelector('.current-group')?.textContent.includes('Group 4/300: Next replay group') && document.querySelector('.countdown')?.textContent.includes('Next post in 01:')`)
	eval(`() => [...document.querySelectorAll('.controls button')].find(b=>b.textContent==='Pause').click()`)
	await(`() => document.querySelector('.auxiliary-title-bar .run-state')?.textContent==='Paused' && document.querySelector('.controls button')?.textContent==='Resume' && !document.querySelector('.controls button').disabled`)
	await(`() => document.querySelector('.countdown')?.textContent.includes('Paused')`)
	if dir := *uiArtifactDir; dir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "postika-activity.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 760, Height: 480, DeviceScaleFactor: 1}); err != nil {
		t.Fatal(err)
	}
	await(`() => {const footer=document.querySelector('footer').getBoundingClientRect(); return footer.bottom<=innerHeight && document.querySelector('main').clientHeight>90 && document.documentElement.scrollWidth<=innerWidth && document.documentElement.scrollHeight<=innerHeight;}`)
	if dir := *uiArtifactDir; dir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "postika-activity-compact.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	assertWindowScrollContained(t, page, ".logs-window main", ".auxiliary-title-bar")
	if !s.GetPostingStatus().Active {
		t.Fatal("paused job became inactive")
	}
	eval(`() => [...document.querySelectorAll('.controls button')].find(b=>b.textContent==='Resume').click()`)
	await(`() => document.querySelector('.auxiliary-title-bar .run-state')?.textContent==='Posting' && document.querySelector('.controls button')?.textContent==='Pause'`)
	eval(`() => [...document.querySelectorAll('.controls button')].find(b=>b.textContent==='Stop').click()`)
	await(`() => document.querySelector('.auxiliary-title-bar .run-state')?.textContent==='Stopped' && [...document.querySelectorAll('.controls button')].every(b=>b.disabled)`)
	eval(`() => document.querySelector('main').scrollTop=10000`)
	await(`() => {const shell=document.querySelector('.auxiliary-window').getBoundingClientRect(),footer=document.querySelector('footer').getBoundingClientRect();return Math.abs(shell.height-innerHeight)<1 && footer.bottom<=innerHeight && footer.top>=0 && document.documentElement.scrollHeight<=innerHeight;}`)
	// Historical app messages must react to every supported language without
	// altering group names, codes, links, response bodies, or raw diagnostics.
	s.postMu.Lock()
	s.appendPostingLogLocked("info", "Posted successfully to Group: A & B 日本語. (Code 368) https://www.facebook.com/groups/123/posts/456/ · Item time 12s")
	s.appendPostingLogLocked("debug", "socket failure: fixture connection reset")
	s.postMu.Unlock()
	await(`() => [...document.querySelectorAll('.log-title')].some(e=>e.textContent.includes('Group: A & B 日本語'))`)
	eval(`() => {window.translatedLogCard=[...document.querySelectorAll('.log-card')].find(e=>e.querySelector('.log-title').textContent.includes('Group: A & B 日本語'));}`)
	for _, locale := range []string{"id", "cs", "da", "de", "es", "fr", "hr", "it", "sw", "hu", "nl", "pl", "pt-BR", "ro", "sk", "sv", "vi", "tr", "el", "bg", "ru", "uk", "ar", "mr", "hi", "bn", "te", "th", "ko", "zh-CN", "ja", "en"} {
		eval(fmt.Sprintf(`() => {localStorage.setItem('postika:language',%q);window.dispatchEvent(new StorageEvent('storage',{key:'postika:language'}));}`, locale))
		await(fmt.Sprintf(`() => document.documentElement.lang===%q && window.translatedLogCard.isConnected && window.translatedLogCard.querySelector('.log-title').textContent.includes('Group: A & B 日本語') && window.translatedLogCard.querySelector('.log-title').textContent.includes('368') && window.translatedLogCard.querySelector('.log-title').textContent.includes('https://www.facebook.com/groups/123/posts/456/') && window.translatedLogCard.querySelector('.debug-details p').textContent==='Posted successfully to Group: A & B 日本語. (Code 368) https://www.facebook.com/groups/123/posts/456/ · Item time 12s'`, locale))
		if locale != "en" {
			await(`() => !window.translatedLogCard.querySelector('.log-title').textContent.includes('Published to ') && document.querySelector('.action-message')?.textContent!=='Posting stopped.' && document.querySelector('.copy-feedback')?.textContent!=='Facebook response copied.' && [...document.querySelectorAll('.log-title')].every(e=>e.textContent!=='Facebook response received' && e.textContent!=='Technical details recorded. Open debug details to inspect the original message.')`)
		}
		if locale == "id" && *uiArtifactDir != "" {
			eval(`() => window.translatedLogCard.scrollIntoView({block:'center'})`)
			data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-logs-indonesian.png"), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = page.Navigate(server.URL + "/?view=settings"); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('input[type=password]') && !document.querySelector('input[type=password]').disabled`)
	eval(`() => {const key=document.querySelector('input[type=password]');key.value='fixture-ui-key';key.dispatchEvent(new Event('input',{bubbles:true}));const max=document.querySelectorAll('input[type=number]')[1];max.value='180';max.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('input[type=checkbox]').click();}`)
	await(`() => document.querySelector('textarea') !== null`)
	eval(`() => {const input=document.querySelector('textarea');input.value='Keep it concise';input.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('button[type=submit]').click();}`)
	await(`() => document.querySelector('.success')?.textContent.includes('Settings saved') && document.querySelector('input[type=password]').value==='' && document.querySelector('.hint')?.textContent.includes('A key is saved')`)
	settings, err := s.GetSettings()
	if err != nil || !settings.KeyConfigured || settings.MaximumInterval != 180 || !settings.Rewrite || settings.Instructions != "Keep it concise" {
		t.Fatalf("UI settings not saved: %#v %v", settings, err)
	}
	if dir := *uiArtifactDir; dir != "" {
		data, err := page.Screenshot(true, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "postika-settings.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = page.Navigate(server.URL + "/?view=settings"); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('input[type=password]')?.placeholder.includes('Key saved') && document.querySelector('textarea')?.value==='Keep it concise' && document.querySelectorAll('input[type=number]')[1]?.value==='180'`)
	rewriteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer rewriteServer.Close()
	rewriteCtx, cancelRewrite := testPostingContext(s)
	defer cancelRewrite()
	stored := storedSettings{APIKey: "fixture-key"}
	body, rewriteErr := s.rewriteForPost(rewriteCtx, "Original body", PostingOptions{Rewrite: true}, &stored, rewriteServer.Client(), rewriteServer.URL)
	if rewriteErr != nil || body != "Original body" || stored.APIKey != "fixture-key" {
		t.Fatal("rewrite fallback changed future posts", rewriteErr)
	}
	if err = page.Navigate(server.URL + "/?view=posting-logs"); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('.timeline')?.textContent.includes('Using the original text for this post only. Gemini remains enabled for the next post.')`)
	await(`() => [...document.querySelectorAll('.timeline-item.warning .level')].some(e=>e.textContent==='WARN')`)
}
