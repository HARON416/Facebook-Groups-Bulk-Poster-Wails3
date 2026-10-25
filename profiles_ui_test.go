package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Exercise the production bindings and profile manager through the browser.
// Native window operations are recorded and Facebook navigation is stubbed;
// no real accounts or external requests are used.
func TestAccountWorkspacesUI(t *testing.T) {
	testAccountWorkspacesUI(t, false)
}

func TestFacebookPageFetchDialogUI(t *testing.T) {
	testAccountWorkspacesUI(t, true)
}

func testAccountWorkspacesUI(t *testing.T, pageFetchOnly bool) {
	t.Helper()
	if !*browserIntegration {
		t.Skip("enable with -browser-integration; build frontend first")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	m, err := newProfileService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	business, err := m.CreateProfile("Business")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ id, user, group string }{{"default", "personal-user", "Personal group"}, {business.ID, "business-user", "Business group"}} {
		s, _ := m.session(entry.id)
		if err := s.saveGroups(entry.user, []Group{{ID: entry.user, Name: entry.group}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.SelectProfile("default"); err != nil {
		t.Fatal(err)
	}
	methods := map[uint32]reflect.Value{}
	bound := reflect.ValueOf(m)
	for i := 0; i < bound.NumMethod(); i++ {
		name := bound.Type().Method(i).Name
		hash := fnv.New32a()
		_, _ = hash.Write([]byte("main.ProfileService." + name))
		methods[hash.Sum32()] = bound.Method(i)
	}
	var maximised, minimised, closed atomic.Bool
	var auxiliaryProfile atomic.Value
	var nativeTitles, windowActions sync.Map
	var copiedLogProfile atomic.Value
	var uploadedImages atomic.Value
	var pageFetches sync.Map
	var pageFetchStarts atomic.Int64
	var fakeGroupsOpened, fakeGroupsLoading atomic.Bool
	var postedFacebookCards, scheduledFacebookCards atomic.Value
	var stoppedFacebookPost atomic.Bool
	var stallConnectivity atomic.Bool
	connectivityStarted := make(chan struct{})
	connectivityRelease := make(chan struct{})
	var releaseConnectivity sync.Once
	files := http.FileServer(http.Dir("frontend/dist"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wails/runtime" {
			files.ServeHTTP(w, r)
			return
		}
		var call struct {
			Object int `json:"object"`
			Method int `json:"method"`
			Args   struct {
				MethodID uint32            `json:"methodID"`
				Args     []json.RawMessage `json:"args"`
				Title    string            `json:"title"`
			} `json:"args"`
		}
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if call.Object == 10 {
			_ = json.NewEncoder(w).Encode(nil)
			return
		}
		if pageFetchOnly && call.Args.MethodID == 4101159777 && stallConnectivity.CompareAndSwap(true, false) {
			close(connectivityStarted)
			select {
			case <-connectivityRelease:
			case <-r.Context().Done():
				return
			}
			_ = json.NewEncoder(w).Encode(ConnectivityStatus{Known: true, Online: false})
			return
		}

		if call.Args.MethodID == 3662562897 || call.Args.MethodID == 3704090331 || call.Args.MethodID == 714486655 {
			var id string
			_ = json.Unmarshal(call.Args.Args[0], &id)
			status := FacebookPageFetchStatus{State: "idle", Posts: []FacebookPagePost{}}
			if saved, ok := pageFetches.Load(id); ok {
				status = saved.(FacebookPageFetchStatus)
			}
			switch call.Args.MethodID {
			case 3662562897:
				var target string
				var count int
				_ = json.Unmarshal(call.Args.Args[1], &target)
				_ = json.Unmarshal(call.Args.Args[2], &count)
				if id != "default" || target != "https://web.facebook.com/swiftlineMotors" || count != 2 {
					t.Errorf("unexpected fetch account/options: %s %s %d", id, target, count)
				}
				status = FacebookPageFetchStatus{RunID: uint64(pageFetchStarts.Add(1)), State: "logged_out", Target: count, Message: "Finish signing in in Chrome. Postika will continue automatically.", Posts: []FacebookPagePost{}}
				pageFetches.Store(id, status)
			case 714486655:
				var runID uint64
				_ = json.Unmarshal(call.Args.Args[1], &runID)
				if runID != status.RunID {
					t.Error("cancelled the wrong page fetch")
				}
				status.State = "cancelled"
				pageFetches.Store(id, status)
				_ = json.NewEncoder(w).Encode(nil)
				return
			}
			_ = json.NewEncoder(w).Encode(status)
			return
		}
		if call.Object == 6 {
			windowActions.Store(fmt.Sprintf("%s#%d", r.Referer(), call.Method), true)
			var result any
			switch call.Method {
			case 14:
				result = maximised.Load()
			case 17:
				minimised.Store(true)
			case 39:
				maximised.Store(!maximised.Load())
			case 2:
				closed.Store(true)
			case 34:
				nativeTitles.Store(r.Referer(), call.Args.Title)
			default:
				t.Errorf("unexpected window action: %d", call.Method)
			}
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		if call.Args.MethodID == 710182265 || call.Args.MethodID == 1194434161 {
			var id string
			if len(call.Args.Args) != 1 || json.Unmarshal(call.Args.Args[0], &id) != nil {
				t.Error("missing account for auxiliary window")
			}
			auxiliaryProfile.Store(id)
			_ = json.NewEncoder(w).Encode(nil)
			return
		}
		if call.Args.MethodID == 558896662 {
			var id string
			_ = json.Unmarshal(call.Args.Args[0], &id)
			copiedLogProfile.Store(id)
			_ = json.NewEncoder(w).Encode(nil)
			return
		}
		if pageFetchOnly {
			switch call.Args.MethodID {
			case 3411990957:
				fakeGroupsOpened.Store(true)
				fakeGroupsLoading.Store(true)
				_ = json.NewEncoder(w).Encode(nil)
				return
			case 2512580126:
				_ = json.NewEncoder(w).Encode("session_captured")
				return
			case 4280265894:
				if fakeGroupsOpened.Load() {
					state := "complete"
					if fakeGroupsLoading.Load() {
						state = "loading"
					}
					_ = json.NewEncoder(w).Encode(GroupsLoadStatus{State: state, Message: "Loaded 2 groups…", Groups: []Group{{ID: "personal-user", Name: "Personal group"}, {ID: "second-group", Name: "Second group"}}})
					return
				}
			case 3098564704, 1560895530:
				var profileID string
				var selected []string
				var posts []FacebookPagePost
				_ = json.Unmarshal(call.Args.Args[0], &profileID)
				_ = json.Unmarshal(call.Args.Args[1], &selected)
				_ = json.Unmarshal(call.Args.Args[2], &posts)
				if profileID != "default" || len(selected) != 2 || len(posts) != 2 || posts[0].Text != "First fetched post" || len(posts[0].Images) != 2 || posts[1].Text != "Second fetched post" || len(posts[1].Images) != 0 {
					t.Error("fetched cards or selected groups were lost before posting")
				}
				if call.Args.MethodID == 3098564704 {
					postedFacebookCards.Store(posts)
				} else {
					var at string
					_ = json.Unmarshal(call.Args.Args[4], &at)
					if _, err := parsePostingSchedule(at, time.Now()); err != nil {
						t.Error(err)
					}
					scheduledFacebookCards.Store(posts)
				}
				if stoppedFacebookPost.Load() {
					_ = json.NewEncoder(w).Encode(PostingStatus{State: "stopped", Message: "Posting stopped."})
					return
				}
				_ = json.NewEncoder(w).Encode(PostingStatus{State: "complete", Message: "Posting finished."})
				return
			}
		}
		if call.Args.MethodID == 2067197279 {
			var uploads []PostUpload
			if err := json.Unmarshal(call.Args.Args[4], &uploads); err != nil {
				t.Error(err)
			}
			uploadedImages.Store(uploads)
			_ = json.NewEncoder(w).Encode(PostingStatus{State: "complete", Message: "Posting finished."})
			return
		}
		if call.Args.MethodID == 3411990957 {
			_ = json.NewEncoder(w).Encode(nil)
			return
		}
		method, ok := methods[call.Args.MethodID]
		if !ok {
			http.Error(w, "unknown method", 400)
			return
		}
		args := []reflect.Value{}
		position := 0
		for i := 0; i < method.Type().NumIn(); i++ {
			typ := method.Type().In(i)
			if typ == reflect.TypeOf((*context.Context)(nil)).Elem() {
				args = append(args, reflect.ValueOf(r.Context()))
				continue
			}
			value := reflect.New(typ)
			if position >= len(call.Args.Args) || json.Unmarshal(call.Args.Args[position], value.Interface()) != nil {
				http.Error(w, "invalid args", 400)
				return
			}
			args = append(args, value.Elem())
			position++
		}
		var result any
		for _, value := range method.Call(args) {
			if value.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				if !value.IsNil() {
					w.WriteHeader(500)
					_ = json.NewEncoder(w).Encode(map[string]string{"message": value.Interface().(error).Error()})
					return
				}
			} else {
				result = value.Interface()
			}
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	defer releaseConnectivity.Do(func() { close(connectivityRelease) })
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	launch := launcher.New().Bin(chrome).UserDataDir(t.TempDir()).Headless(true)
	control, err := launch.Launch()
	if err != nil {
		t.Fatal(err)
	}
	browser := rod.New().ControlURL(control).Context(ctx).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		launch.Kill()
		t.Fatal(err)
	}
	defer func() { _ = browser.Context(context.Background()).Close(); time.Sleep(500 * time.Millisecond) }()
	page, err := browser.Page(proto.TargetCreateTarget{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1200, Height: 800, DeviceScaleFactor: 1}); err != nil {
		t.Fatal(err)
	}
	awaitPage := func(target *rod.Page, predicate string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			result, err := target.Eval(predicate)
			if err == nil && result.Value.Bool() {
				return
			}
			select {
			case <-ctx.Done():
				snapshot, _ := target.Context(context.Background()).Timeout(time.Second).Eval(`() => ({text:document.body.innerText.slice(0,2000),featureScroll:document.querySelector('#features-content')?.scrollTop,active:document.activeElement?.outerHTML.slice(0,250),account:{disabled:document.querySelector('#account-profile')?.disabled,expanded:document.querySelector('#account-profile')?.getAttribute('aria-expanded'),menuDisabled:document.querySelector('.menu-controls')?.disabled},inputs:[...document.querySelectorAll('input')].map(i=>({type:i.type,disabled:i.disabled,validation:i.validationMessage})),errors:[...document.querySelectorAll('.error')].map(e=>e.textContent)})`)
				t.Fatalf("UI timed out: %s (%v), snapshot: %v", predicate, err, snapshot)
			case <-time.After(25 * time.Millisecond):
				if time.Now().After(deadline) {
					snapshot, _ := target.Context(context.Background()).Timeout(time.Second).Eval(`() => ({text:document.body.innerText.slice(0,2000),featureScroll:document.querySelector('#features-content')?.scrollTop,active:document.activeElement?.outerHTML.slice(0,250),account:{disabled:document.querySelector('#account-profile')?.disabled,expanded:document.querySelector('#account-profile')?.getAttribute('aria-expanded'),menuDisabled:document.querySelector('.menu-controls')?.disabled},inputs:[...document.querySelectorAll('input')].map(i=>({type:i.type,disabled:i.disabled,validation:i.validationMessage}))})`)
					t.Fatalf("UI did not settle: %s, snapshot: %v", predicate, snapshot)
				}
			}
		}
	}
	await := func(predicate string) { t.Helper(); awaitPage(page, predicate) }
	chooseTheme := func(target *rod.Page, id string) {
		t.Helper()
		target.MustActivate()
		target.MustElement("[data-theme-picker]").MustClick()
		awaitPage(target, `() => document.querySelector('dialog[aria-labelledby="theme-picker-heading"]')?.open && document.querySelectorAll('#theme-options [role=option]').length===189`)
		target.MustElement(fmt.Sprintf(`[data-theme-id="%s"]`, id)).MustClick()
		awaitPage(target, `() => !document.querySelector('dialog[aria-labelledby="theme-picker-heading"]').open`)
	}

	eval := func(script string) {
		t.Helper()
		if _, err := page.Eval(script); err != nil {
			t.Fatal(err)
		}
	}
	selectAccount := func(id string) {
		t.Helper()
		await(`() => !document.querySelector('#features-dialog')?.open && document.querySelector('[data-features-card]')?.getAttribute('aria-expanded')==='false'`)
		eval(`() => document.querySelector('#account-profile').click()`)
		await(`() => document.querySelector('[role=listbox]') !== null`)
		eval(fmt.Sprintf(`() => document.querySelector('[role=option][data-profile-id="%s"]').click()`, id))
		await(fmt.Sprintf(`() => document.querySelector('#account-profile').dataset.profileId===%q && !document.querySelector('#account-profile').disabled`, id))
	}
	await(`() => document.querySelector('#post-body[contenteditable=true]') !== null`)
	if pageFetchOnly {
		// An inactive account dialog must not process an unrelated submission.
		eval(`async () => {const form=document.querySelector('#add-account-form');if(form){const input=form.querySelector('input');input.value='Unintended account';input.dispatchEvent(new Event('input',{bubbles:true}));form.requestSubmit();}await new Promise(resolve=>setTimeout(resolve,400));}`)
		if list := m.GetProfiles(); len(list.Profiles) != 2 || list.Selected != "default" {
			t.Fatalf("inactive account form created or selected a profile: %+v", list)
		}
		await(`() => !document.querySelector('#add-account-form') && !document.querySelector('#account-name')`)
		// Page fetching validates inputs, keeps login/progress visible, renders
		// successful results outside the dialog, and retains the manual draft.
		page.MustElement("#post-body").MustInput("Keep this manual draft")
		page.MustElement(".facebook-page-option").MustClick()
		await(`() => document.querySelector('dialog[id^="facebook-page-dialog-"]')?.open`)
		page.MustElement(`input[type="url"]`).MustInput("https://example.test/page")
		eval(`() => { const f=document.querySelector('dialog[id^="facebook-page-dialog-"] form'); if(f.reportValidity()) throw new Error('External URL accepted'); }`)
		eval(`() => {localStorage.setItem('postika:language','he');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language'}));}`)
		await(`() => document.documentElement.lang==='he'`)
		eval(`() => {const u=document.querySelector('dialog[open] input[type=url]'),n=document.querySelector('dialog[open] input[type=number]');n.value='-1';n.dispatchEvent(new Event('input',{bubbles:true}));u.checkValidity();n.checkValidity();}`)
		await(`() => document.querySelector('dialog[open] input[type=url]').validationMessage.startsWith('יש להזין כתובת של דף פייסבוק') && document.querySelector('dialog[open] input[type=number]').validationMessage==='יש להזין מספר שלם וחיובי של פוסטים לייבוא'`)
		page.MustElement(`dialog[open] input[type=number]`).MustSelectAllText().MustInput("1")
		await(`() => document.querySelector('dialog[open] input[type=number]').value==='1' && document.querySelector('dialog[open] input[type=number]').validity.valid`)
		eval(`() => {const n=document.querySelector('dialog[open] input[type=number]');n.value='1';n.dispatchEvent(new Event('input',{bubbles:true}));localStorage.setItem('postika:language','en');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language'}));}`)
		await(`() => document.documentElement.lang==='en'`)
		eval(`() => { const e=document.querySelector('dialog[open] input[type="url"]'); e.value=''; e.dispatchEvent(new Event('input',{bubbles:true})); }`)
		page.MustElement(`dialog[open] input[type="url"]`).MustInput("https://web.facebook.com/swiftlineMotors")
		eval(`() => { const e=document.querySelector('dialog[open] input[type="number"]'); e.value='1'; e.dispatchEvent(new Event('input',{bubbles:true})); }`)
		page.MustElement(`dialog[open] input[type=number]`).MustFocus()
		if err := page.Keyboard.Type(input.ArrowUp); err != nil {
			t.Fatal(err)
		}
		await(`() => document.querySelector('dialog[open] input[type="number"]').value==='2'`)
		page.MustElement(`dialog[id^="facebook-page-dialog-"] button[type="submit"]`).MustClick()
		await(`() => document.querySelector('dialog[open] .fetch-status')?.innerText.includes('Finish signing in') && document.querySelector('dialog[open] button[type="submit"]').disabled && document.querySelector('dialog[open] input[type="url"]').matches(':disabled')`)
		pixel := "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"
		pageFetches.Store("default", FacebookPageFetchStatus{RunID: 1, State: "complete", Target: 2, Message: "Fetched 2 posts.", Posts: []FacebookPagePost{{ID: "first", Text: "First fetched post", Images: []string{pixel, pixel}}, {ID: "second", Text: "Second fetched post"}}})
		await(`() => !document.querySelector('dialog[id^="facebook-page-dialog-"]').open && document.querySelectorAll('.fetched-posts li').length===2 && document.querySelector('.fetched-posts').innerText.includes('First fetched post') && !document.querySelector('#post-body') && !document.querySelector('[data-file-drop-target]') && !document.querySelector('dialog .fetched-posts')`)
		await(`() => { const cards=[...document.querySelectorAll('.post-card.animate__slideInLeft')];return cards.length===2 && cards.every(card=>getComputedStyle(card).animationName==='slideInLeft') && getComputedStyle(cards[0]).animationDelay==='0s' && getComputedStyle(cards[1]).animationDelay==='0.08s'; }`)
		await(`() => !document.querySelector('.post-card.animate__slideInLeft')`)
		eval(`() => { const trigger=document.querySelector('.facebook-page-option'), cards=document.querySelector('.fetched-posts'); if(trigger.getBoundingClientRect().bottom >= cards.getBoundingClientRect().top) throw new Error('Cards should follow the fetch button'); }`)
		// New labels and backend progress update in retained UI on locale changes;
		// fetched post text and the supplied page URL stay untouched.
		eval(`() => {localStorage.setItem('postika:language','sw');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language',newValue:'sw'}));}`)
		await(`() => document.documentElement.lang==='sw' && document.querySelector('.facebook-page-option').textContent.includes('Au bofya hapa') && document.querySelector('.fetched-heading h2').textContent.includes('Machapisho yaliyoletwa') && document.querySelector('.post-card .card-action').textContent==='Hariri' && document.querySelector('.post-card').getAttribute('aria-label')==='Chapisho 1' && document.querySelector('.post-text').textContent==='First fetched post'`)
		pageFetches.Store("default", FacebookPageFetchStatus{RunID: 1, State: "loading", Target: 2, Message: "Fetched 1 of 2 posts…", Posts: []FacebookPagePost{{ID: "first", Text: "First fetched post"}}})
		page.MustElement(".facebook-page-option").MustClick()
		await(`() => document.querySelector('dialog[id^="facebook-page-dialog-"]')?.open && document.querySelector('dialog[open] .fetch-status > p')?.textContent==='Yameletwa machapisho 1 kati ya 2…' && document.querySelector('dialog[id^="facebook-page-dialog-"] h2').textContent==='Leta machapisho kutoka ukurasa wa Facebook' && document.querySelector('dialog[open] button[type="submit"]').textContent==='Inaleta…' && document.querySelector('dialog[open] input[type="url"]').value==='https://web.facebook.com/swiftlineMotors'`)
		pageFetches.Store("default", FacebookPageFetchStatus{RunID: 1, State: "capturing", Target: 2, Message: "No photo query appeared after opening the image. Reloading the photo page to capture fresh values…"})
		await(`() => document.querySelector('dialog[open] .fetch-status > p')?.textContent==='Inapakia upya ukurasa wa picha…'`)
		eval(`() => {localStorage.setItem('postika:language','ar');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language',newValue:'ar'}));}`)
		await(`() => document.documentElement.lang==='ar' && document.documentElement.dir==='rtl' && document.querySelector('dialog[open] .fetch-status > p')?.textContent==='جارٍ إعادة تحميل صفحة الصورة…' && document.querySelector('dialog[id^="facebook-page-dialog-"] h2').textContent==='جلب منشورات من صفحة فيسبوك' && document.querySelector('dialog[open] input[type="url"]').value==='https://web.facebook.com/swiftlineMotors' && document.querySelector('.post-text').textContent==='First fetched post'`)
		eval(`() => {localStorage.setItem('postika:language','he');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language',newValue:'he'}));}`)
		await(`() => document.documentElement.lang==='he' && document.documentElement.dir==='rtl' && document.querySelector('dialog[open] .fetch-status > p')?.textContent==='דף התמונה נטען מחדש…' && document.querySelector('dialog[id^="facebook-page-dialog-"] h2').textContent==='ייבוא פוסטים מדף פייסבוק' && document.querySelector('.post-card .card-action').textContent==='עריכה' && document.querySelector('.post-card').getAttribute('aria-label')==='פוסט 1' && document.querySelector('dialog[open] input[type="url"]').value==='https://web.facebook.com/swiftlineMotors' && getComputedStyle(document.querySelector('dialog[open] input[type="url"]')).direction==='ltr' && document.querySelector('.post-text').textContent==='First fetched post'`)
		eval(`() => {const d=document.querySelector('dialog[id^="facebook-page-dialog-"]'),r=d.getBoundingClientRect();if(r.left<0 || r.right>innerWidth || d.scrollWidth>d.clientWidth)throw new Error('Localized dialog overflows');}`)
		page.MustElement(`dialog[id^="facebook-page-dialog-"] .action-button--secondary`).MustClick()
		await(`() => !document.querySelector('dialog[id^="facebook-page-dialog-"]').open`)
		eval(`() => {localStorage.setItem('postika:language','en');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language',newValue:'en'}));}`)
		await(`() => document.documentElement.lang==='en' && document.documentElement.dir==='ltr' && document.querySelector('.post-card .card-action').textContent==='Edit' && document.querySelector('.fetched-posts').innerText.includes('Second fetched post')`)
		// Group-loading locks conflicting actions, while navigation and utilities remain usable.
		await(`() => !document.querySelector('button.next').disabled`)
		page.MustElement("button.next").MustClick()
		await(`() => document.querySelector('#step-heading')?.textContent==='Select groups'`)
		page.MustElement("button.load-groups").MustClick()
		await(`() => document.querySelector('.groups-list')?.getAttribute('aria-busy')==='true' && !document.querySelector('button.previous').disabled && document.querySelector('button.next').disabled && !document.querySelector('#account-profile').disabled && !document.querySelector('.menu-controls').disabled && [...document.querySelectorAll('.groups-toolbar button,.bulk-actions button')].every(b=>b.matches(':disabled')) && document.querySelector('#group-search').disabled && [...document.querySelectorAll('.groups-list input')].every(e=>e.disabled)`)
		eval(`() => {localStorage.setItem('postika:language','he');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language'}));}`)
		await(`() => document.documentElement.lang==='he' && document.querySelector('.group-status')?.textContent==='נטענו 2 קבוצות…' && !document.querySelector('.group-status details')`)
		eval(`() => {localStorage.setItem('postika:language','en');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language'}));}`)
		await(`() => document.documentElement.lang==='en'`)
		page.MustElement("[data-features-card]").MustClick()
		await(`() => document.querySelector('#features-dialog').open && !!document.querySelector('.usage-guide')`)
		page.MustElement("#features-dialog .close").MustClick()
		page.MustElement("button.previous").MustClick()
		await(`() => !!document.querySelector('.fetched-posts') && !document.querySelector('.click-confetti') && document.querySelector('.facebook-page-option').disabled && !document.querySelector('button.next').disabled`)
		page.MustElement("button.next").MustClick()
		await(`() => !!document.querySelector('.groups-toolbar') && document.querySelector('button.next').disabled`)
		selectAccount(business.ID)
		selectAccount("default")
		await(`() => !!document.querySelector('.groups-toolbar') && document.querySelector('button.next').disabled`)
		page.MustElement("button.previous").MustClick()
		fakeGroupsLoading.Store(false)
		await(`() => !document.querySelector('button.next').disabled`)
		page.MustElement("button.next").MustClick()
		await(`() => !document.querySelector('button.previous').disabled && !document.querySelector('.bulk-actions button').disabled`)
		page.MustElement(".bulk-actions button").MustClick()
		await(`() => document.querySelector('.selection-count').textContent.includes('2 selected') && !document.querySelector('button.next').disabled`)
		page.MustElement("button.next").MustClick()
		await(`() => document.querySelector('#step-heading')?.textContent==='Post or schedule' && [...document.querySelectorAll('.step-actions button')].filter(b=>['Post now','Schedule'].includes(b.textContent.trim())).length===2 && [...document.querySelectorAll('.step-actions button')].filter(b=>['Post now','Schedule'].includes(b.textContent.trim())).every(b=>!b.disabled)`)
		await(`() => {const review=document.querySelector('.queue-review');return review?.textContent.includes('Fetched posts') && review.textContent.includes('First fetched post') && review.textContent.includes('Second fetched post') && review.textContent.includes('2 selected');}`)
		eval(`() => [...document.querySelectorAll('.step-actions button')].find(b=>b.textContent.trim()==='Schedule').click()`)
		await(`() => !!document.querySelector('#schedule-form') && !document.querySelector('button[form="schedule-form"]').disabled`)
		page.MustElement(`button[form="schedule-form"]`).MustClick()
		await(`() => !document.querySelector('#schedule-form') && document.querySelector('.step-content').textContent.includes('Posting finished.')`)
		if _, ok := scheduledFacebookCards.Load().([]FacebookPagePost); !ok {
			t.Fatal("scheduled queue omitted fetched cards")
		}
		eval(`() => [...document.querySelectorAll('.step-actions button')].find(b=>b.textContent.trim()==='Post now').click()`)
		await(`() => !document.querySelector('.step-actions button.post').disabled && document.querySelector('.step-content').textContent.includes('Posting finished.')`)
		if _, ok := postedFacebookCards.Load().([]FacebookPagePost); !ok {
			t.Fatal("posting queue omitted fetched cards")
		}
		// A stopped result must stay translated with no English debug duplicate
		// while an existing status is switched through every offered language.
		stoppedFacebookPost.Store(true)
		page.MustElement("button.post").MustClick()
		await(`() => document.querySelector('.post-feedback')?.textContent==='Posting stopped.' && !document.querySelector('.post-feedback details')`)
		page.MustElement("#language-profile").MustClick()
		await(`() => document.querySelectorAll('#language-options [data-profile-id]').length===33`)
		localeCodes, err := page.Eval(`() => [...document.querySelectorAll('#language-options [data-profile-id]')].map(el=>el.dataset.profileId)`)
		if err != nil {
			t.Fatal(err)
		}
		eval(`() => document.querySelector('#language-profile').click()`)
		for _, code := range localeCodes.Value.Arr() {
			locale := code.Str()
			eval(fmt.Sprintf(`() => {localStorage.setItem('postika:language',%q);window.dispatchEvent(new StorageEvent('storage',{key:'postika:language'}));}`, locale))
			await(fmt.Sprintf(`() => document.documentElement.lang===%q && !document.querySelector('.post-feedback details') && document.querySelector('.post-feedback').textContent.trim().length>0 && (%q==='en' || !document.querySelector('.post-feedback').textContent.includes('Posting stopped.'))`, locale, locale))
		}
		eval(`() => {localStorage.setItem('postika:language','en');window.dispatchEvent(new StorageEvent('storage',{key:'postika:language'}));}`)
		await(`() => document.documentElement.lang==='en'`)
		stoppedFacebookPost.Store(false)
		page.MustElement("button.previous").MustClick()
		await(`() => document.querySelector('#step-heading')?.textContent==='Select groups'`)
		page.MustElement("button.previous").MustClick()
		await(`() => document.querySelectorAll('.fetched-posts li').length===2 && document.querySelector('.fetched-posts').innerText.includes('First fetched post') && document.querySelector('.fetched-posts').innerText.includes('Second fetched post') && !document.querySelector('button.next').disabled`)
		// Reopening a completed fetch lets the user fetch again; it does not
		// immediately close the dialog or reapply the old result.
		page.MustElement(".facebook-page-option").MustClick()
		await(`() => document.querySelector('dialog[id^="facebook-page-dialog-"]').open`)
		page.MustElement(`dialog[open] input[type="url"]`).MustInput("https://web.facebook.com/swiftlineMotors")
		eval(`() => { const e=document.querySelector('dialog[open] input[type="number"]');e.value='2';e.dispatchEvent(new Event('input',{bubbles:true})); }`)
		page.MustElement(`dialog[id^="facebook-page-dialog-"] button[type="submit"]`).MustClick()
		await(`() => document.querySelector('dialog[open] .fetch-status')?.innerText.includes('Finish signing in')`)
		pageFetches.Store("default", FacebookPageFetchStatus{RunID: 2, State: "error", Target: 2, Message: "Could not fetch the next page.", Posts: []FacebookPagePost{{ID: "partial", Text: "Partial result must not replace cards"}}})
		await(`() => document.querySelector('dialog[open] .fetch-status.error') && document.querySelector('dialog[id^="facebook-page-dialog-"]').open && document.querySelectorAll('.fetched-posts li').length===2 && !document.querySelector('.fetched-posts').innerText.includes('Partial result')`)
		page.MustElement(`dialog[id^="facebook-page-dialog-"] button[type="submit"]`).MustClick()
		await(`() => document.querySelector('dialog[open] .fetch-status')?.innerText.includes('Finish signing in')`)
		if err := page.Keyboard.Type(input.Escape); err != nil {
			t.Fatal(err)
		}
		await(`() => !document.querySelector('dialog[id^="facebook-page-dialog-"]').open && document.activeElement.matches('.facebook-page-option')`)
		if saved, _ := pageFetches.Load("default"); saved.(FacebookPageFetchStatus).State != "cancelled" || pageFetchStarts.Load() != 3 {
			t.Fatal("page fetch was not cancelled")
		}
		if list := m.GetProfiles(); len(list.Profiles) != 2 || list.Selected != "default" {
			t.Fatalf("page imports created or selected an account: %+v", list)
		}
		// Card edits commit on Save, Cancel discards pending changes, and
		// deletion commits directly without an Undo message.
		// A blank post anywhere in the import blocks the whole batch. Explain
		// which post needs text, and clear the blocker as edits repair the batch.
		for _, postNumber := range []int{2, 1} {
			page.MustElement(fmt.Sprintf(`[aria-label="Edit post %d"]`, postNumber)).MustClick()
			eval(`() => { const input=document.querySelector('.fetched-posts textarea'); input.value='  \n '; input.dispatchEvent(new Event('input',{bubbles:true})); document.querySelector('.fetched-posts button[type="submit"]').click(); }`)
			await(fmt.Sprintf(`() => document.querySelector('button.next').disabled && document.querySelector('button.next').getAttribute('aria-describedby')==='next-blocker' && document.querySelector('#next-blocker')?.textContent==='Add text to fetched post %d before posting'`, postNumber))
		}
		if *uiArtifactDir != "" {
			if err := os.MkdirAll(*uiArtifactDir, 0755); err != nil {
				t.Fatal(err)
			}
			data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-import-next-blocker.png"), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		for _, post := range []struct {
			number int
			text   string
		}{{1, "First fetched post"}, {2, "Second fetched post"}} {
			page.MustElement(fmt.Sprintf(`[aria-label="Edit post %d"]`, post.number)).MustClick()
			eval(fmt.Sprintf(`() => { const input=document.querySelector('.fetched-posts textarea'); input.value=%q; input.dispatchEvent(new Event('input',{bubbles:true})); document.querySelector('.fetched-posts button[type="submit"]').click(); }`, post.text))
			if post.number == 1 {
				await(`() => document.querySelector('button.next').disabled && document.querySelector('#next-blocker')?.textContent==='Add text to fetched post 2 before posting'`)
			}
		}
		await(`() => !document.querySelector('button.next').disabled && !document.querySelector('#next-blocker') && !document.querySelector('button.next').hasAttribute('aria-describedby')`)
		page.MustElement(`[aria-label="Edit post 1"]`).MustClick()
		await(`() => document.activeElement.matches('.fetched-posts textarea')`)
		eval(`() => { const input=document.querySelector('.fetched-posts textarea'); input.value='Edited fetched post'; input.dispatchEvent(new Event('input',{bubbles:true})); }`)
		page.MustElement(`[aria-label="Remove image 1 from post 1"]`).MustClick()
		page.MustElement(`.fetched-posts button[type="submit"]`).MustClick()
		await(`() => document.querySelector('.fetched-posts .post-text').textContent==='Edited fetched post' && document.querySelectorAll('.fetched-posts li:first-child img').length===1 && document.activeElement.matches('[aria-label="Edit post 1"]')`)
		page.MustElement(`[aria-label="Edit post 1"]`).MustClick()
		eval(`() => { const input=document.querySelector('.fetched-posts textarea'); input.value='Discard this edit'; input.dispatchEvent(new Event('input',{bubbles:true})); document.querySelector('.fetched-posts footer button[type="button"]').click(); }`)
		await(`() => !document.querySelector('.fetched-posts textarea') && document.querySelector('.fetched-posts .post-text').textContent==='Edited fetched post'`)
		page.MustElement(`[aria-label="Delete post 2"]`).MustClick()
		await(`() => { const card=document.querySelector('.post-card.animate__animated.animate__fadeOutLeft');return !!card && getComputedStyle(card).animationName==='fadeOutLeft' && card.querySelector('.delete').disabled && document.querySelectorAll('.fetched-posts li').length===2; }`)
		await(`() => document.querySelectorAll('.fetched-posts li').length===1 && !document.querySelector('.delete-feedback') && !document.querySelector('.facebook-source').textContent.includes('Post removed') && !document.querySelector('button.next').disabled`)

		await(`async () => { const db=await new Promise((resolve,reject)=>{const request=indexedDB.open('postika-account-drafts',2);request.onsuccess=()=>resolve(request.result);request.onerror=()=>reject(request.error)}); const draft=await new Promise(resolve=>{const request=db.transaction('drafts').objectStore('drafts').get('default');request.onsuccess=()=>resolve(request.result)});db.close();return draft?.fetchedPosts?.length===1 && draft.fetchedPosts[0].text==='Edited fetched post' && draft.fetchedPosts[0].images.length===1; }`)
		// Cards and mode are saved independently for the active account.
		selectAccount(business.ID)
		await(`() => document.querySelector('.account-workspace:not([hidden]) #post-body') && !document.querySelector('.account-workspace:not([hidden]) .fetched-posts')`)
		selectAccount("default")
		await(`() => document.querySelectorAll('.account-workspace:not([hidden]) .fetched-posts li').length===1 && document.querySelector('.account-workspace:not([hidden]) .post-text').textContent==='Edited fetched post'`)
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
			t.Fatal(err)
		}
		page.MustElement(`.account-workspace:not([hidden]) [aria-label="Delete post 1"]`).MustClick()
		await(`() => !document.querySelector('.account-workspace:not([hidden]) .fetched-section') && !!document.querySelector('.account-workspace:not([hidden]) #post-body') && !!document.querySelector('.account-workspace:not([hidden]) [data-file-drop-target]') && document.querySelector('.account-workspace:not([hidden]) [data-step-scroll]').scrollTop===0`)
		await(`() => document.querySelector('.account-workspace:not([hidden]) #post-body')?.innerText.includes('Keep this manual draft') && !document.querySelector('.delete-feedback') && document.querySelector('.account-workspace:not([hidden]) button.next').disabled`)

		// An old bridge response must not pin the toast offline after a new
		// network event and a later successful poll.
		service, _ := m.session("default")
		service.postMu.Lock()
		service.connectivityEnabled = true
		service.connectivity = ConnectivityStatus{Known: true, Online: false}
		service.connectivityWake = make(chan struct{}, 1)
		wake := service.connectivityWake
		service.postMu.Unlock()
		await(`() => !!document.querySelector('.connection-toast:not(.online)') && !document.querySelector('.connection-toast p')`)
		stallConnectivity.Store(true)
		select {
		case <-connectivityStarted:
		case <-time.After(3 * time.Second):
			t.Fatal("connectivity poll did not start")
		}
		eval(`() => window.dispatchEvent(new Event('online'))`)
		select {
		case <-wake:
		case <-time.After(time.Second):
			t.Fatal("network event did not request a fresh backend check")
		}
		eval(`() => {if(!document.querySelector('.connection-toast:not(.online)'))throw new Error('Browser online hint was trusted without verification')}`)
		service.postMu.Lock()
		service.connectivity.Online = true
		service.postMu.Unlock()
		await(`() => !!document.querySelector('.connection-toast.online') && !document.querySelector('.connection-toast p')`)
		releaseConnectivity.Do(func() { close(connectivityRelease) })
		eval(`async () => {await new Promise(r=>setTimeout(r,200));if(!document.querySelector('.connection-toast.online'))throw new Error('Stale poll restored the offline toast')}`)
		// A real active queue gets pause/recovery details; a manual pause is
		// retained when connectivity returns and changes without another outage.
		_, cancelPosting := testPostingContext(service)
		defer cancelPosting()
		service.postMu.Lock()
		service.connectivity.Online = false
		service.postMu.Unlock()
		await(`() => document.querySelector('.connection-toast:not(.online) p')?.textContent.includes('Your queue is paused')`)
		service.PausePosting()
		service.postMu.Lock()
		service.connectivity.Online = true
		service.postMu.Unlock()
		await(`() => document.querySelector('.connection-toast.online p')?.textContent==='Your queue is still manually paused.'`)
		service.ResumePosting()
		await(`() => document.querySelector('.connection-toast.online p')?.textContent==='Your queue will continue automatically.'`)

		return
	}
	// The feature card traps focus, scrolls independently, and returns focus on close.
	page.MustElement("[data-features-card]").MustClick()
	await(`() => document.querySelector('[data-features-card]').textContent.includes('How to use') && document.querySelector('#features-dialog').open && !!document.querySelector('.usage-guide')`)
	eval(`() => document.querySelector('.view-switch button:first-child').click()`)
	await(`() => {const d=document.querySelector('#features-dialog');return d.open && d.contains(document.activeElement) && d.querySelectorAll('section').length===6 && d.innerText.includes('33 languages · 189 themes')}`)
	if err := page.Keyboard.Type(input.Tab); err != nil {
		t.Fatal(err)
	}
	await(`() => document.activeElement.matches('.view-switch button[aria-pressed=true]')`)
	if err := page.Keyboard.Type(input.Tab, input.Enter); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('#features-heading').textContent==='Features & How to Use' && [...document.querySelectorAll('.usage-guide section > h2')].map(h=>h.textContent).join('|')==='Create post|Select groups|Post or schedule' && document.querySelector('.usage-guide .introduction').textContent.includes('default account') && document.querySelector('.additional-accounts').textContent.includes('optional') && document.querySelector('.usage-guide').textContent.includes('both are required') && document.activeElement.matches('[data-usage-guide][aria-pressed=true]')`)
	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-usage-guide.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := page.Keyboard.Type(input.Tab); err != nil {
		t.Fatal(err)
	}
	await(`() => document.activeElement.matches('#features-dialog .feature-content')`)
	if err := page.Keyboard.Type(input.End); err != nil {
		t.Fatal(err)
	}
	await(`() => {const c=document.querySelector('#features-dialog .feature-content');return c.scrollTop>0 && document.documentElement.scrollTop===0}`)
	page.MustElement(".view-switch button:first-child").MustClick()
	await(`() => document.querySelector('#features-heading').textContent==='Features & How to Use' && document.querySelector('#features-content').scrollTop===0 && document.querySelectorAll('#features-content section').length===6 && !document.querySelector('.usage-guide')`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	await(`() => !document.querySelector('#features-dialog').open && document.activeElement.matches('[data-features-card]') && document.querySelector('[data-features-card]').getAttribute('aria-expanded')==='false'`)
	page.MustElement("[data-features-card]").MustClick()
	await(`() => document.querySelector('#features-dialog .feature-content').scrollTop===0`)
	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-features.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 520, Height: 680, DeviceScaleFactor: 1}); err != nil {
		t.Fatal(err)
	}
	await(`() => {const d=document.querySelector('#features-dialog'),r=d.getBoundingClientRect(),c=d.querySelector('.feature-content');return r.left>=0 && r.right<=innerWidth && r.top>=0 && r.bottom<=innerHeight && c.scrollWidth<=c.clientWidth && c.scrollHeight>c.clientHeight}`)
	page.MustElement("[data-usage-guide]").MustClick()
	await(`() => {const c=document.querySelector('#features-content');return document.querySelector('.usage-guide') && c.scrollWidth<=c.clientWidth && c.scrollHeight>c.clientHeight}`)
	page.MustElement("#features-dialog .close").MustClick()
	await(`() => !document.querySelector('#features-dialog').open && document.activeElement.matches('[data-features-card]')`)
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 1200, Height: 800, DeviceScaleFactor: 1}); err != nil {
		t.Fatal(err)
	}
	page.MustElement("[data-features-card]").MustClick()
	eval(`() => document.querySelector('#features-dialog').dispatchEvent(new MouseEvent('click',{bubbles:true,clientX:0,clientY:0}))`)
	await(`() => !document.querySelector('#features-dialog').open && document.activeElement.matches('[data-features-card]')`)
	// Follow the OS until the user chooses a theme, then persist that choice.
	setSystemTheme := func(mode string) {
		t.Helper()
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-color-scheme", Value: mode}}}).Call(page); err != nil {
			t.Fatal(err)
		}
	}
	setSystemTheme("dark")
	await(`() => document.documentElement.dataset.theme==='dark' && localStorage.getItem('postika:theme')===null`)
	setSystemTheme("light")
	await(`() => document.documentElement.dataset.theme==='light' && document.querySelector('[data-theme-picker]') !== null && document.querySelector('[data-theme-toggle]') === null`)
	chooseTheme(page, "dark")
	await(`() => document.documentElement.dataset.theme==='dark' && localStorage.getItem('postika:theme')==='dark' && document.querySelector('[data-theme-picker]') !== null`)
	setSystemTheme("dark")
	setSystemTheme("light")
	await(`async () => {await new Promise(r=>setTimeout(r,50));return document.documentElement.dataset.theme==='dark'}`)
	// The editor remains readable, and the teal frame stays inside the viewport.
	await(`() => {const luminance=value=>{const c=value.match(/[\d.]+/g).slice(0,3).map(v=>{v=Number(v)/255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4});return c[0]*.2126+c[1]*.7152+c[2]*.0722},fg=luminance(getComputedStyle(document.querySelector('#post-body')).color),bg=luminance(getComputedStyle(document.querySelector('.editor')).backgroundColor),frame=getComputedStyle(document.querySelector('.workspace'));return (Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05)>=4.5 && frame.borderTopWidth==='2px' && frame.borderTopStyle==='solid' && document.documentElement.scrollWidth<=innerWidth && document.documentElement.scrollHeight<=innerHeight}`)
	assertWindowScrollContained(t, page, ".account-workspace:not([hidden]) .app-shell main", ".top-bar")
	// Custom titlebar controls call the current native window and keep restore state.
	await(`() => document.querySelector('[aria-label="Maximize window"]') !== null`)
	eval(`() => document.querySelector('[aria-label="Maximize window"]').click()`)
	await(`() => document.querySelector('[aria-label="Restore window"]') !== null`)
	if !maximised.Load() {
		t.Fatal("maximize did not reach native window API")
	}
	eval(`() => document.querySelector('.drag-area').dispatchEvent(new MouseEvent('dblclick',{bubbles:true}))`)
	await(`() => document.querySelector('[aria-label="Maximize window"]') !== null`)
	if maximised.Load() {
		t.Fatal("titlebar double click did not restore window")
	}
	eval(`() => document.querySelector('[aria-label="Minimize window"]').click()`)
	await(`() => !document.querySelector('[aria-label="Minimize window"]').disabled`)
	if !minimised.Load() {
		t.Fatal("minimize did not reach native window API")
	}
	await(`() => getComputedStyle(document.querySelector('.drag-area')).getPropertyValue('--wails-draggable').trim()==='drag' && getComputedStyle(document.querySelector('#account-profile')).getPropertyValue('--wails-draggable').trim()==='no-drag'`)
	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-header.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Keyboard navigation previews an account without switching until confirmed.
	eval(`() => document.querySelector('#account-profile').focus()`)
	if err := page.Keyboard.Type(input.Space); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('#account-profile').getAttribute('aria-expanded')==='true' && document.activeElement?.id==='account-search'`)
	if err := page.Keyboard.Type(input.ArrowDown); err != nil {
		t.Fatal(err)
	}
	await(fmt.Sprintf(`() => document.querySelector('#account-search').getAttribute('aria-activedescendant')==='account-option-%s' && document.querySelector('#account-profile').dataset.profileId==='default'`, business.ID))
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('[role=listbox]')===null && document.activeElement?.id==='account-profile'`)
	eval(`() => document.querySelector('#account-profile').click()`)
	await(`() => document.querySelector('[role=listbox]') !== null`)
	page.MustElement("#account-search").MustInput("business")
	await(`() => document.querySelectorAll('[role=option]').length===1 && document.querySelector('[role=option]').textContent.trim()==='Business'`)
	page.MustElement("#account-search").MustSelectAllText().MustInput("missing account")
	await(`() => document.querySelectorAll('[role=option]').length===0 && document.querySelector('.no-results')?.textContent.includes('No accounts found')`)
	eval(`() => document.querySelector('[aria-label="Clear account search"]').click()`)
	await(`() => document.querySelectorAll('[role=option]').length===2 && document.activeElement?.id==='account-search'`)
	if err := page.Keyboard.Type(input.Tab); err != nil {
		t.Fatal(err)
	}
	await(`() => document.activeElement?.matches('[data-add-account]') && document.querySelector('[role=listbox]')!==null`)
	if err := page.Keyboard.Type(input.Tab); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('[role=listbox]')===null && document.activeElement?.textContent==='Updates'`)
	if err := page.Keyboard.Type(input.Tab); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('[role=listbox]')===null && document.activeElement.matches('[data-features-card]')`)
	if err := page.Keyboard.Type(input.Tab); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('[role=listbox]')===null && document.activeElement?.textContent==='Add account'`)
	page.MustElement("#post-body").MustInput("Personal draft")
	// Use an actual disk-backed File, then delete its source before relaunch.
	originalImagePath := filepath.Join(t.TempDir(), "personal.png")
	imageData, err := base64.StdEncoding.DecodeString(fixturePNG)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(originalImagePath, imageData, 0600); err != nil {
		t.Fatal(err)
	}
	page.MustElement(".file-input").MustSetFiles(originalImagePath)
	await(`() => document.querySelectorAll('.previews .thumbnail').length===1 && !document.querySelector('button.next').disabled`)
	page.MustElement("button.next").MustClick()
	await(`() => document.querySelector('.groups-list')?.textContent.includes('Personal group')`)
	eval(`() => document.querySelector(".groups-list input").click()`)
	eval(`() => document.querySelector('#account-profile').focus()`)
	if err := page.Keyboard.Type(input.ArrowDown); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('[role=listbox]') !== null`)
	if err := page.Keyboard.Type(input.End, input.Enter); err != nil {
		t.Fatal(err)
	}
	await(fmt.Sprintf(`() => document.querySelector('#account-profile').dataset.profileId===%q && !document.querySelector('#account-profile').disabled && document.querySelector('[role=listbox]')===null`, business.ID))
	await(`() => document.querySelector('#post-body')?.textContent==='' && document.querySelectorAll('.previews .thumbnail').length===0`)
	await(`() => ![...document.querySelectorAll('.app-menu button')].some(b=>b.textContent==='Settings')`)
	await(`() => !document.querySelector('.profile-error')`)
	eval(`() => [...document.querySelectorAll('.app-menu button')].find(b=>b.textContent==='Posting logs').click()`)
	await(`() => !document.querySelector('.profile-error')`)
	page.MustElement("#post-body").MustInput("Business draft")
	// A native file import completing after a switch must go to its original draft.
	eval(fmt.Sprintf(`() => window._wails.dispatchWailsEvent({name:'postika:images-dropped',data:{target:'post-images',profileID:'default',images:[{name:'late.png',type:'image/png',data:%q,lastModified:12345}],errors:[]}})`, fixturePNG))
	await(`() => document.querySelectorAll('.previews .thumbnail').length===0`)
	selectAccount("default")
	await(`() => document.querySelector('.groups-list input')?.checked===true && document.querySelector('.groups-list').textContent.includes('Personal group')`)
	page.MustElement("button.previous").MustClick()
	await(`() => document.querySelector('#post-body')?.textContent==='Personal draft' && document.querySelectorAll('.previews .thumbnail').length===2`)
	// Reload the app after deleting the original to verify independent image bytes.
	eval(`async () => { const db=await new Promise((resolve,reject)=>{const r=indexedDB.open('postika-account-drafts',2);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error);});const tx=db.transaction('drafts','readonly');const draft=await new Promise((resolve,reject)=>{const r=tx.objectStore('drafts').get('default');r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error);});if(draft.schema!==2 || draft.images.length!==2 || draft.images.some(i=>!i.key || 'file' in i)) throw new Error('Draft image bytes not saved');for(const image of draft.images){const data=await new Promise(r=>{const q=db.transaction('images').objectStore('images').get(image.key);q.onsuccess=()=>r(q.result)});if(!(data instanceof ArrayBuffer)||!data.byteLength)throw new Error('Missing independent image bytes');} }`)
	if err := os.Remove(originalImagePath); err != nil {
		t.Fatal(err)
	}
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('#post-body')?.textContent==='Personal draft' && document.querySelectorAll('.previews .thumbnail').length===2`)
	page.MustElement("button.next").MustClick()
	await(`() => document.querySelector('.groups-list input')?.checked===true`)
	page.MustElement("button.next").MustClick()
	await(`() => document.querySelector('.step-actions .post') && !document.querySelector('.step-actions .post').disabled`)
	if id, _ := auxiliaryProfile.Load().(string); id != business.ID {
		t.Fatalf("header menus opened the wrong account: %q", id)
	}
	page.MustElement(".step-actions .post").MustClick()
	await(`() => document.querySelector('.post-feedback')?.textContent.includes('Posting finished.') || document.body.innerText.includes('Posting finished.')`)
	if uploads, ok := uploadedImages.Load().([]PostUpload); !ok || len(uploads) != 2 || uploads[0].Name != "personal.png" || uploads[0].Data != fixturePNG || uploads[1].Data != fixturePNG {
		t.Fatalf("restored attachments did not reach posting intact: %#v", uploads)
	}
	page.MustElement("button.previous").MustClick()
	page.MustElement("button.previous").MustClick()
	await(`() => document.querySelector('#post-body')?.textContent==='Personal draft'`)
	// Migrate one readable legacy File and retain an unreadable one in place.
	eval(`async () => {const db=await new Promise(r=>{const q=indexedDB.open('postika-account-drafts',2);q.onsuccess=()=>r(q.result)});let saved;while(!saved || saved.currentStep!==0){saved=await new Promise(r=>{const q=db.transaction('drafts').objectStore('drafts').get('default');q.onsuccess=()=>r(q.result)});await new Promise(r=>setTimeout(r,10));}delete saved.schema;saved.images=await Promise.all(saved.images.map(async(i,index)=>{const data=await new Promise(r=>{const q=db.transaction('images').objectStore('images').get(i.key);q.onsuccess=()=>r(q.result)});return {id:i.id,file:new File([data],index?'legacy-unavailable.png':i.name,{type:i.type,lastModified:i.lastModified})};}));saved.currentStep=2;await new Promise((resolve,reject)=>{const tx=db.transaction('drafts','readwrite');tx.objectStore('drafts').put(saved,'default');tx.oncomplete=resolve;tx.onerror=reject;});}`)
	removeHook, err := page.EvalOnNewDocument(`{const original=File.prototype.arrayBuffer;window.originalFileRead=original;File.prototype.arrayBuffer=function(){return this.name==='legacy-unavailable.png'?Promise.reject(new DOMException('Unavailable legacy backing file','NotReadableError')):original.call(this);};}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('#post-body')?.textContent==='Personal draft' && document.querySelectorAll('.previews .thumbnail').length===2 && document.querySelector('.unavailable-image')?.textContent.includes('legacy-unavailable.png') && document.querySelector('button.next')?.disabled===true && document.querySelector('.previews img')?.complete`)
	if err := removeHook(); err != nil {
		t.Fatal(err)
	}
	eval(`() => {File.prototype.arrayBuffer=window.originalFileRead;}`)
	// Re-selecting the missing filename repairs that slot without duplicating it.
	eval(fmt.Sprintf(`() => {const bytes=Uint8Array.from(atob(%q),c=>c.charCodeAt(0)),transfer=new DataTransfer();transfer.items.add(new File([bytes],'legacy-unavailable.png',{type:'image/png'}));const input=document.querySelector('.file-input');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));}`, fixturePNG))
	await(`() => !document.querySelector('.unavailable-image') && document.querySelectorAll('.previews .thumbnail').length===2 && !document.querySelector('button.next').disabled`)

	selectAccount(business.ID)
	await(`() => document.querySelector('#post-body')?.textContent==='Business draft' && document.querySelectorAll('.previews .thumbnail').length===0`)
	// Locale changes update existing workspaces and retained windows without remounts.
	changeLanguage := func(code string) {
		t.Helper()
		eval(`() => document.querySelector('#language-profile').click()`)
		await(`() => document.querySelector('#language-options [role=option]') !== null`)
		eval(fmt.Sprintf(`() => document.querySelector('#language-options [data-profile-id="%s"]').click()`, code))
		await(fmt.Sprintf(`() => document.documentElement.lang===%q && !document.querySelector('#language-profile').disabled`, code))
	}
	// Descriptions, editor help, counts, and all three stages must translate too.
	changeLanguage("id")
	page.MustElement("[data-features-card]").MustClick()
	eval(`() => document.querySelector('.view-switch button:first-child').click()`)
	await(`() => document.querySelector('#features-heading').textContent==='Fitur & Cara Menggunakan' && document.querySelector('#features-dialog').innerText.includes('33 bahasa · 189 tema') && getComputedStyle(document.querySelector('#features-dialog')).backgroundColor===getComputedStyle(document.querySelector('.top-bar')).backgroundColor`)
	page.MustElement("[data-usage-guide]").MustClick()
	await(`() => document.querySelector('#features-heading').textContent==='Fitur & Cara Menggunakan' && document.querySelector('.usage-guide').textContent.includes('keduanya wajib') && !document.querySelector('.usage-guide').textContent.includes('Write your message') && document.querySelector('#post-body').textContent==='Business draft'`)
	page.MustElement("#features-dialog .close").MustClick()
	await(`() => !document.querySelector('#features-dialog').open`)
	await(`() => document.querySelector('.eyebrow')?.textContent==='LANGKAH 1 DARI 3' && document.querySelector('.editor-footer')?.textContent.includes('karakter') && document.querySelector('.editor-footer')?.textContent.includes('Mendukung pintasan Markdown') && document.body.innerText.includes('Tulis postingan Anda dan tambahkan gambar') && document.body.innerText.includes('Mulai dengan pesan Anda.') && document.querySelector('[aria-label="Tebal"]') !== null && document.querySelector('#image-formats')?.textContent.includes('Pilih beberapa gambar')`)
	selectAccount("default")
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .next').click()`)
	await(`() => document.querySelector('#group-search')?.placeholder==='Cari grup…' && document.querySelector('.selection-count')?.textContent.includes('dipilih')`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .next').click()`)
	await(`() => document.querySelector('#step-heading')?.textContent==='Posting atau jadwalkan' && document.querySelector('.account-context') === null && document.body.innerText.includes('Jeda antarpostingan') && document.body.innerText.includes('Kirim sekarang') && document.querySelector('[aria-label="Minimum (detik)"]') !== null`)
	// Scheduling and selected calendar days use the same dark palette.
	eval(`() => [...document.querySelectorAll('.account-workspace:not([hidden]) .step-actions button')].find(b=>b.textContent==='Jadwalkan').click()`)
	await(`() => document.querySelector('.date-time-picker') && document.documentElement.dataset.theme==='dark' && getComputedStyle(document.querySelector('.date-time-picker')).backgroundColor===getComputedStyle(document.querySelector('.option-card')).backgroundColor && getComputedStyle(document.querySelector('.calendar .selected')).color!==getComputedStyle(document.querySelector('.calendar .selected')).backgroundColor`)
	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-schedule-dark.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	await(`() => document.querySelector('#post-body')?.textContent==='Personal draft'`)
	selectAccount(business.ID)
	changeLanguage("sw")
	await(`() => document.querySelector('#step-heading')?.textContent==='Tengeneza chapisho' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	// Auxiliary titlebars share the controls and localize the default account.
	for _, view := range []struct{ query, title, screenshot string }{{"posting-logs", "Kumbukumbu za uchapishaji", "postika-logs-titlebar.png"}} {
		auxURL := server.URL + "/?view=" + view.query + "&profile=default&account=Default%20profile"
		aux, err := browser.Page(proto.TargetCreateTarget{URL: auxURL})
		if err != nil {
			t.Fatal(err)
		}
		if err := aux.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 760, Height: 760, DeviceScaleFactor: 1}); err != nil {
			t.Fatal(err)
		}
		awaitPage(aux, `() => (document.querySelector('.window-account')?.textContent==='Wasifu chaguomsingi' || document.querySelector('.window-profile .account-name')?.textContent==='Wasifu chaguomsingi') && document.querySelectorAll('.window-controls button').length===3 && getComputedStyle(document.querySelector('.window-caption')).getPropertyValue('--wails-draggable').trim()==='drag' && !document.querySelector('.window-controls [title]') && document.title.includes('Wasifu chaguomsingi')`)
		awaitPage(aux, `() => document.documentElement.dataset.theme==='dark' && getComputedStyle(document.querySelector('.auxiliary-window')).borderTopWidth==='2px' && document.documentElement.scrollWidth<=innerWidth && document.documentElement.scrollHeight<=innerHeight`)
		if view.query == "posting-logs" {
			chooseTheme(aux, "light")
			await(`() => document.documentElement.dataset.theme==='light'`)
			chooseTheme(aux, "dark")
			await(`() => document.documentElement.dataset.theme==='dark'`)
		}
		aux.MustElement(".window-controls button:nth-child(1)").MustClick()
		awaitPage(aux, `() => !document.querySelector('.window-controls button').disabled`)
		aux.MustElement(".window-controls button:nth-child(2)").MustClick()
		awaitPage(aux, `() => document.querySelector('.window-controls .restore') !== null`)
		aux.MustEval(`() => document.querySelector('.window-caption').dispatchEvent(new MouseEvent('dblclick',{bubbles:true}))`)
		awaitPage(aux, `() => document.querySelector('.window-controls .restore') === null && !document.querySelector('.window-controls button').disabled`)
		if *uiArtifactDir != "" {
			data, err := aux.Screenshot(false, &proto.PageCaptureScreenshot{})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(*uiArtifactDir, view.screenshot), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		aux.MustElement(".window-controls button:nth-child(3)").MustClick()
		awaitPage(aux, `() => !document.querySelector('.window-controls button').disabled`)
		for _, method := range []int{17, 39, 2, 34} {
			if _, ok := windowActions.Load(fmt.Sprintf("%s#%d", auxURL, method)); !ok {
				t.Fatalf("auxiliary %s action %d did not reach its own window", view.query, method)
			}
		}
		if title, ok := nativeTitles.Load(auxURL); !ok || !strings.Contains(title.(string), "Wasifu chaguomsingi") || !strings.Contains(title.(string), view.title) {
			t.Fatalf("untranslated native %s title: %v", view.query, title)
		}
		if view.query == "posting-logs" {
			// Imported themes share the existing persistence and cross-window sync.
			aux.MustElement("[data-theme-picker]").MustClick()
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===189 && document.activeElement.matches('dialog[open] input')`)
			aux.MustElement("dialog[open] input").MustInput("no-theme-matches-this")
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===0 && document.querySelector('.no-results')!==null`)
			aux.MustEval(`() => document.querySelector('dialog[open] input').focus()`)
			if err := aux.Keyboard.Type(input.Escape); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => !document.querySelector('dialog').open && document.activeElement.matches('[data-theme-picker]') && !document.documentElement.dataset.palette`)
			aux.MustElement("[data-theme-picker]").MustClick()
			aux.MustElement("dialog[open] input").MustInput("CATPPUCCIN")
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===1`)
			if err := aux.Keyboard.Type(input.Enter); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => !document.querySelector('dialog').open && document.documentElement.dataset.palette==='palette:catppuccin' && getComputedStyle(document.documentElement).backgroundColor==='rgb(30, 30, 46)' && localStorage.getItem('postika:theme')==='palette:catppuccin'`)
			await(`() => document.documentElement.dataset.palette==='palette:catppuccin' && document.querySelector('#post-body')?.textContent==='Business draft'`)
			if err := aux.Reload(); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => document.documentElement.dataset.palette==='palette:catppuccin' && document.querySelector('[data-theme-picker]')!==null`)
			aux.MustElement("[data-theme-picker]").MustClick()
			awaitPage(aux, `() => document.querySelector('[data-theme-id="palette:catppuccin"]').getAttribute('aria-selected')==='true'`)
			if *uiArtifactDir != "" {
				data, err := aux.Screenshot(false, &proto.PageCaptureScreenshot{})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-theme-picker.png"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			aux.MustElement("dialog[open] input").MustInput("Postika Dark")
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===1`)
			if err := aux.Keyboard.Type(input.Enter); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => !document.documentElement.dataset.palette && document.documentElement.dataset.theme==='dark' && getComputedStyle(document.documentElement).backgroundColor==='rgb(21, 28, 25)'`)
			await(`() => !document.documentElement.dataset.palette && document.documentElement.dataset.theme==='dark'`)
			// Same run/entry IDs across accounts must never mix their data or actions.
			for _, id := range []string{"default", business.ID} {
				s, _ := m.session(id)
				s.postMu.Lock()
				s.postingRunID = 1
				s.posting = PostingStatus{State: "posting"}
				s.postingCancel = func() {}
				s.postingLogs = []PostingLog{{ID: 1, Level: "info", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Message: "Profile log: " + id, HasResponse: true}}
				s.postMu.Unlock()
			}
			awaitPage(aux, `() => document.querySelector('.log-title')?.textContent==='Profile log: default'`)
			aux.MustElement(".window-profile .account-trigger").MustClick()
			aux.MustElement(fmt.Sprintf(`[role=option][data-profile-id="%s"]`, business.ID)).MustClick()
			awaitPage(aux, fmt.Sprintf(`() => document.querySelector('.log-title')?.textContent==='Profile log: %s' && !document.body.innerText.includes('Profile log: default') && document.title.includes('Business')`, business.ID))
			if m.GetProfiles().Selected != business.ID {
				t.Fatal("switching the log viewer changed the main account")
			}
			aux.MustElement(".copy-response").MustClick()
			awaitPage(aux, `() => !document.querySelector('.copy-response').disabled`)
			if copiedLogProfile.Load() != business.ID {
				t.Fatal("copy response targeted another profile")
			}
			aux.MustElement(".controls button").MustClick()
			awaitPage(aux, `() => !document.querySelector('.controls button').disabled`)
			personal, _ := m.session("default")
			company, _ := m.session(business.ID)
			if personal.GetPostingStatus().PauseRequested || !company.GetPostingStatus().PauseRequested {
				t.Fatal("pause did not stay bound to the viewed profile")
			}
			// Simulate opening logs for the default profile in the retained viewer.
			aux.MustEval(`() => {const u=new URL(location.href);u.searchParams.set('profile','default');u.searchParams.set('account','Default profile');history.replaceState(null,'',u);window.dispatchEvent(new Event('postika:logs-profile'));}`)
			awaitPage(aux, `() => document.querySelector('.log-title')?.textContent==='Profile log: default' && document.querySelector('.window-profile .account-trigger').dataset.profileId==='default'`)
			for _, s := range []*GroupsService{personal, company} {
				s.postMu.Lock()
				s.postingCancel = nil
				s.posting.State = "complete"
				s.postingPaused = false
				s.postMu.Unlock()
			}
		}
		if err := aux.Close(); err != nil {
			t.Fatal(err)
		}
	}
	changeLanguage("id")
	changeLanguage("ar")
	page.MustActivate()
	page.MustElement("[data-features-card]").MustClick()
	eval(`() => document.querySelector('.view-switch button:first-child').click()`)
	await(`() => {const d=document.querySelector('#features-dialog');return d.open && document.querySelector('#features-heading').textContent==='الميزات وكيفية الاستخدام' && getComputedStyle(d).direction==='rtl' && d.innerText.includes('33 لغة · 189 سمة')}`)
	page.MustElement("[data-usage-guide]").MustClick()
	await(`() => document.querySelector('#features-heading').textContent==='الميزات وكيفية الاستخدام' && document.querySelector('.usage-guide').textContent.includes('كلاهما مطلوب') && getComputedStyle(document.querySelector('.usage-guide')).direction==='rtl'`)
	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-usage-guide-arabic.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	page.MustElement("#features-dialog .close").MustClick()
	await(`() => !document.querySelector('#features-dialog').open`)
	await(`() => document.documentElement.dir==='rtl' && document.querySelector('#step-heading')?.textContent==='إنشاء منشور' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='ar' && document.documentElement.dir==='rtl' && document.documentElement.dataset.theme==='dark' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	// Hebrew updates every retained window and dialog without changing drafts.
	changeLanguage("he")
	await(`() => document.documentElement.dir==='rtl' && document.querySelector('#step-heading')?.textContent==='יצירת פוסט' && document.querySelector('#post-body')?.textContent==='Business draft' && getComputedStyle(document.querySelector('#language-profile')).direction==='rtl'`)
	eval(`() => [...document.querySelectorAll('.app-menu button')].find(b=>b.textContent==='הוספת חשבון').click()`)
	page.MustElement("#account-name").MustInput("Business")
	page.MustElement(".add-account button[type=submit]").MustClick()
	await(`() => document.querySelector('.profile-error')?.textContent==='פרופיל חשבון בשם זה כבר קיים' && !document.querySelector('.profile-error details')`)
	page.MustElement(".add-account .action-button--secondary").MustClick()

	page.MustElement("[data-features-card]").MustClick()
	eval(`() => document.querySelector('.view-switch button:first-child').click()`)
	await(`() => {const d=document.querySelector('#features-dialog');return d.open && getComputedStyle(d).direction==='rtl' && document.querySelector('#features-heading').textContent==='תכונות ומדריך שימוש' && d.innerText.includes('33 שפות · 189 ערכות נושא')}`)
	page.MustElement("[data-usage-guide]").MustClick()
	await(`() => document.querySelector('.usage-guide').textContent.includes('שניהם נדרשים') && getComputedStyle(document.querySelector('.usage-guide')).direction==='rtl'`)
	page.MustElement("#features-dialog .close").MustClick()
	await(`() => !document.querySelector('#features-dialog').open && document.querySelector('[data-features-card]').getAttribute('aria-expanded')==='false'`)
	page.MustElement("[data-theme-picker]").MustClick()
	await(`() => {const d=document.querySelector('dialog:has(#theme-picker-heading)');return d.open && getComputedStyle(d).direction==='rtl' && document.querySelector('#theme-picker-heading').textContent==='ערכות צבעים'}`)
	eval(`() => document.querySelector('dialog:has(#theme-picker-heading)').close()`)
	selectAccount("default")
	await(`() => document.querySelector('.account-workspace:not([hidden]) #post-body')?.textContent==='Personal draft' && !document.querySelector('.account-workspace:not([hidden]) .next').disabled`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .next').click()`)
	await(`() => document.querySelector('#step-heading')?.textContent==='בחירת קבוצות' && document.querySelector('#group-search')?.placeholder==='חיפוש קבוצות…'`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .next').click()`)
	await(`() => document.querySelector('#step-heading')?.textContent==='פרסום או תזמון' && document.body.innerText.includes('מרווח בין פרסומים')`)
	eval(`() => [...document.querySelectorAll('.account-workspace:not([hidden]) .step-actions button')].find(b=>b.textContent==='תזמון').click()`)
	await(`() => document.querySelector('.date-time-picker') && document.querySelector('.month-navigation [aria-label="החודש הבא"]') && getComputedStyle(document.querySelector('.time-controls')).direction==='ltr'`)
	eval(`() => {const current=document.querySelector('.calendar .selected');window.hebrewSelectedDay=current.dataset.date;current.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowLeft',bubbles:true}));}`)
	await(`() => {const next=new Date(window.hebrewSelectedDay+'T12:00:00');next.setDate(next.getDate()+1);const day=next.getFullYear()+'-'+String(next.getMonth()+1).padStart(2,'0')+'-'+String(next.getDate()).padStart(2,'0');return document.querySelector('.calendar .selected').dataset.date===day;}`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	selectAccount(business.ID)
	hebrewLogs, err := browser.Page(proto.TargetCreateTarget{URL: server.URL + "/?view=posting-logs&profile=" + business.ID + "&account=Business"})
	if err != nil {
		t.Fatal(err)
	}
	awaitPage(hebrewLogs, `() => document.documentElement.lang==='he' && document.documentElement.dir==='rtl' && document.querySelector('h1')?.textContent==='יומני פרסום' && document.querySelector('[aria-label="התקדמות הפרסום"]') && document.querySelector('progress[aria-label="קבוצות שטופלו"]') && document.querySelector('.window-account')===null && getComputedStyle(document.querySelector('.timeline')).direction==='rtl'`)
	page.MustActivate()
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='he' && document.documentElement.dir==='rtl' && document.querySelector('#step-heading')?.textContent==='יצירת פוסט' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	changeLanguage("ja")
	await(`() => document.documentElement.dir==='ltr' && document.querySelector('#step-heading')?.textContent==='投稿を作成'`)
	awaitPage(hebrewLogs, `() => document.documentElement.lang==='ja' && document.documentElement.dir==='ltr' && document.querySelector('h1')?.textContent==='投稿ログ'`)
	if err := hebrewLogs.Close(); err != nil {
		t.Fatal(err)
	}
	// Exercise every bundled catalog; locale loading must work without external services.
	for _, code := range []string{"id", "cs", "da", "de", "en", "es", "fr", "hr", "it", "sw", "hu", "nl", "pl", "pt-BR", "ro", "sk", "sv", "vi", "tr", "el", "bg", "ru", "uk", "ar", "he", "mr", "hi", "bn", "te", "th", "ko", "zh-CN", "ja"} {
		changeLanguage(code)
		await(`() => document.querySelector('#post-body')?.textContent==='Business draft' && document.querySelector('#step-heading')?.textContent.length>0`)
		if code != "en" {
			await(`() => document.querySelector('[data-theme-picker]')?.getAttribute('aria-label') !== 'Color themes'`)
			eval(`() => document.querySelector('#account-profile').click()`)
			await(`() => document.querySelector('#account-options [data-profile-id="default"] .account-name')?.textContent !== 'Default profile' && document.querySelector('#account-options [data-profile-id="default"] .account-name')?.textContent.length > 0 && document.querySelector('#account-profile .account-name')?.textContent==='Business'`)
			eval(`() => document.querySelector('#account-profile').click()`)
			await(`() => !document.querySelector('.account-workspace:not([hidden])').innerText.includes('Write your post and add images') && !document.querySelector('.account-workspace:not([hidden])').innerText.includes('Load groups and choose where to post') && !document.querySelector('.account-workspace:not([hidden])').innerText.includes('Review, set preferences, then post now or schedule') && !document.querySelector('.editor-footer')?.textContent.includes('characters') && !document.querySelector('#image-formats')?.textContent.includes('Select multiple images')`)
		}
	}
	// Unsupported saved preferences recover to source English on restart.
	eval(`() => {localStorage.setItem('postika:language','unsupported-locale');localStorage.setItem('postika:theme','unsupported-theme')}`)
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='en' && document.documentElement.dataset.theme==='light' && document.querySelector('#step-heading')?.textContent==='Create post'`)
	// First launch follows the webview's ordered language preferences without
	// saving an automatic choice. An explicit choice must survive a restart.
	removePreferredLanguages, err := page.EvalOnNewDocument(`Object.defineProperty(navigator,'languages',{configurable:true,get:()=>['fi-FI','sw-KE','en-US']});Object.defineProperty(navigator,'language',{configurable:true,get:()=> 'fi-FI'});`)
	if err != nil {
		t.Fatal(err)
	}
	eval(`() => localStorage.removeItem('postika:language')`)
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='sw' && document.documentElement.dir==='ltr' && document.querySelector('#post-body')?.textContent==='Business draft' && document.querySelector('[data-features-card]')?.textContent.includes('Jinsi ya kutumia') && localStorage.getItem('postika:language')===null`)
	changeLanguage("en")
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='en' && document.querySelector('#step-heading')?.textContent==='Create post' && localStorage.getItem('postika:language')==='en'`)
	eval(`() => localStorage.setItem('postika:language','unsupported-locale')`)
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='sw' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	changeLanguage("en")
	if err := removePreferredLanguages(); err != nil {
		t.Fatal(err)
	}
	// The add-account flow creates a fresh workspace and requests its own login.
	eval(`() => [...document.querySelectorAll('.app-menu button')].find(b=>b.textContent==='Add account').click()`)
	await(`() => document.querySelector('#add-account-dialog')?.open && document.activeElement.id==='account-name' && getComputedStyle(document.querySelector('#add-account-dialog')).backgroundColor===getComputedStyle(document.querySelector('.top-bar')).backgroundColor && document.querySelector('#add-account-dialog .action-button--primary').disabled`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	await(`() => !document.querySelector('#add-account-dialog')?.open && document.activeElement.textContent==='Add account'`)
	eval(`() => [...document.querySelectorAll('.app-menu button')].find(b=>b.textContent==='Add account').click()`)
	page.MustElement("#account-name").MustInput("https://web.facebook.com/swiftlineMotors")
	page.MustElement(".add-account button[type=submit]").MustClick()
	await(`() => document.querySelector('#add-account-dialog')?.open && document.querySelector('#account-name').validationMessage==='Account names cannot be URLs'`)
	page.MustElement("#account-name").MustSelectAllText().MustInput("Third account")
	eval(`async () => {document.querySelector('#add-account-form').requestSubmit();await new Promise(resolve=>setTimeout(resolve,300));}`)
	if list := m.GetProfiles(); len(list.Profiles) != 2 {
		t.Fatal("account creation accepted a submission without its Create button")
	}

	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-add-account.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	page.MustElement("#account-name").MustFocus()
	if err := page.Keyboard.Type(input.Enter); err != nil {
		t.Fatal(err)
	}
	await(`() => !document.querySelector('#add-account-dialog')?.open && document.querySelector('#account-profile .account-name')?.textContent==='Third account' && document.querySelector('#step-heading')?.textContent==='Select groups' && !document.querySelector('.groups-list')`)
	changeLanguage("id")
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	await(`() => document.querySelector('.placeholder')?.textContent==='Apa yang ingin Anda bagikan?' && document.querySelector('.editor-footer')?.textContent.includes('0 karakter') && document.querySelector('#browse-instruction')?.textContent.includes('telusuri gambar')`)
	changeLanguage("en")
	// Rename from the hovered row without switching accounts or replacing its
	// draft, and keep the same operation available in the shared logs window.
	thirdID := m.GetProfiles().Selected
	page.MustElement("#post-body").MustInput("Draft kept through rename")
	page.MustElement("#account-profile").MustClick()
	page.MustElement(fmt.Sprintf(`#account-options [data-profile-id="%s"]`, business.ID)).MustHover()
	await(fmt.Sprintf(`() => getComputedStyle(document.querySelector('[data-rename-profile="%s"]')).opacity==='1'`, business.ID))
	page.MustElement(fmt.Sprintf(`[data-rename-profile="%s"]`, business.ID)).MustClick()
	await(`() => document.querySelector('#rename-profile-dialog')?.open && document.activeElement.id==='rename-profile-name' && document.querySelector('#rename-profile-name').value==='Business' && document.querySelector('#account-profile .account-name').textContent==='Third account'`)
	page.MustElement("#rename-profile-name").MustSelectAllText().MustInput("Business renamed")
	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-rename-profile.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := page.Keyboard.Type(input.Enter); err != nil {
		t.Fatal(err)
	}
	await(`() => !document.querySelector('#rename-profile-dialog') && document.activeElement.id==='account-profile' && document.querySelector('#account-profile .account-name').textContent==='Third account' && document.querySelector('#post-body').textContent==='Draft kept through rename'`)
	if list := m.GetProfiles(); list.Selected != thirdID || len(list.Profiles) != 3 || list.Profiles[1].ID != business.ID || list.Profiles[1].Name != "Business renamed" {
		t.Fatalf("rename changed identity or selection: %+v", list)
	}
	page.MustElement("#account-profile").MustClick()
	page.MustElement("#account-search").MustInput("Business renamed")
	await(`() => document.querySelectorAll('#account-options [role=option]').length===1`)
	if err := page.Keyboard.Type(input.F2); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('#rename-profile-name')?.value==='Business renamed'`)
	page.MustElement("#rename-profile-name").MustSelectAllText().MustInput("Third account")
	page.MustElement(`#rename-profile-dialog button[type=submit]`).MustClick()
	await(`() => document.querySelector('#rename-profile-dialog [role=alert]')?.textContent==='An account profile with this name already exists'`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	await(`() => !document.querySelector('#rename-profile-dialog') && document.activeElement.id==='account-profile'`)
	if m.GetProfiles().Profiles[1].Name != "Business renamed" {
		t.Fatal("cancelled duplicate rename changed profile")
	}
	renameLogs, err := browser.Page(proto.TargetCreateTarget{URL: server.URL + "/?view=posting-logs&profile=" + business.ID + "&account=Business%20renamed"})
	if err != nil {
		t.Fatal(err)
	}
	awaitPage(renameLogs, `() => document.querySelector('#account-profile .account-name')?.textContent==='Business renamed'`)
	renameLogs.MustElement("#account-profile").MustClick()
	renameLogs.MustElement(fmt.Sprintf(`#account-options [data-profile-id="%s"]`, business.ID)).MustHover()
	renameLogs.MustElement(fmt.Sprintf(`[data-rename-profile="%s"]`, business.ID)).MustClick()
	awaitPage(renameLogs, `() => document.querySelector('#rename-profile-name')?.value==='Business renamed'`)
	renameLogs.MustElement("#rename-profile-name").MustSelectAllText().MustInput("Business logs")
	renameLogs.MustElement(`#rename-profile-dialog button[type=submit]`).MustClick()
	awaitPage(renameLogs, `() => !document.querySelector('#rename-profile-dialog') && document.querySelector('#account-profile .account-name').textContent==='Business logs' && document.title.includes('Business logs') && new URLSearchParams(location.search).get('account')==='Business logs'`)
	if err := renameLogs.Close(); err != nil {
		t.Fatal(err)
	}
	page.MustElement("#account-profile").MustClick()
	await(fmt.Sprintf(`() => document.querySelector('#account-options [data-profile-id="%s"] .account-name')?.textContent==='Business logs'`, business.ID))
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	if list := m.GetProfiles(); list.Selected != thirdID || len(list.Profiles) != 3 {
		t.Fatal("log-window rename changed selected profile")
	}
	closed.Store(false)
	eval(`() => document.querySelector('[aria-label="Close window"]').click()`)
	await(`() => !document.querySelector('[aria-label="Close window"]').disabled`)
	if !closed.Load() {
		t.Fatal("close did not reach native window API")
	}
	if len(m.GetProfiles().Profiles) != 3 {
		t.Fatal("add account did not persist profile")
	}
	// The switcher adds accounts through the same modal, leaving the top-bar
	// entry available. Escape and cancelled deletion must leave every draft intact.
	page.MustElement("#account-profile").MustClick()
	page.MustElement("[data-add-account]").MustClick()
	await(`() => document.querySelector('#add-account-dialog')?.open && document.activeElement.id==='account-name'`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	await(`() => !document.querySelector('#add-account-dialog') && document.activeElement.id==='account-profile'`)
	page.MustElement("#account-profile").MustClick()
	page.MustElement(fmt.Sprintf(`#account-options [data-profile-id="%s"]`, business.ID)).MustHover()
	page.MustElement(fmt.Sprintf(`[data-delete-profile="%s"]`, business.ID)).MustClick()
	await(`() => document.querySelector('#delete-profile-dialog')?.open && document.querySelector('#delete-profile-dialog .profile-name').textContent==='Business logs' && document.activeElement.textContent==='Cancel'`)
	if *uiArtifactDir != "" {
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*uiArtifactDir, "postika-delete-profile.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	await(`() => !document.querySelector('#delete-profile-dialog') && document.querySelector('#post-body').textContent==='Draft kept through rename'`)
	if len(m.GetProfiles().Profiles) != 3 {
		t.Fatal("cancel deleted a profile")
	}
	// Delete from another window while the main workspace still has the profile
	// mounted. Its pending autosaves must not restore the removed draft/images.
	deleteLogs, err := browser.Page(proto.TargetCreateTarget{URL: server.URL + "/?view=posting-logs&profile=" + business.ID})
	if err != nil {
		t.Fatal(err)
	}
	awaitPage(deleteLogs, `() => document.querySelector('#account-profile .account-name')?.textContent==='Business logs'`)
	deleteLogs.MustElement("#account-profile").MustClick()
	deleteLogs.MustElement(fmt.Sprintf(`#account-options [data-profile-id="%s"]`, business.ID)).MustHover()
	deleteLogs.MustElement(fmt.Sprintf(`[data-delete-profile="%s"]`, business.ID)).MustClick()
	deleteLogs.MustElement("#delete-profile-dialog .delete-confirm").MustClick()
	awaitPage(deleteLogs, `() => !document.querySelector('#delete-profile-dialog') && document.querySelector('#account-profile .account-name')?.textContent==='Third account'`)
	await(fmt.Sprintf(`() => !document.querySelector('.account-workspace [data-profile-id="%s"]') && document.querySelector('#post-body').textContent==='Draft kept through rename'`, business.ID))
	await(fmt.Sprintf(`async () => {const db=await new Promise(r=>{const q=indexedDB.open('postika-account-drafts',2);q.onsuccess=()=>r(q.result)});const draft=await new Promise(r=>{const q=db.transaction('drafts').objectStore('drafts').get('%s');q.onsuccess=()=>r(q.result)});const keys=await new Promise(r=>{const q=db.transaction('images').objectStore('images').getAllKeys();q.onsuccess=()=>r(q.result)});db.close();return !draft && !keys.some(key=>key.startsWith('%s:'));}`, business.ID, business.ID))
	if len(m.GetProfiles().Profiles) != 2 || m.GetProfiles().Selected != thirdID {
		t.Fatal("deleting background profile changed main selection")
	}
	// Deleting the selected account selects the surviving account; deleting the
	// last account leaves an empty switcher with a working Add account action.
	if _, err := page.Activate(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{thirdID, "default"} {
		await(`() => !document.querySelector('#account-profile').disabled && !document.querySelector('dialog[open]')`)
		page.MustElement("#account-profile").MustClick()
		page.MustElement(fmt.Sprintf(`#account-options [data-profile-id="%s"]`, id)).MustHover()
		page.MustElement(fmt.Sprintf(`[data-delete-profile="%s"]`, id)).MustClick()
		page.MustElement("#delete-profile-dialog .delete-confirm").MustClick()
		await(`() => !document.querySelector('#delete-profile-dialog')`)
	}
	await(`() => !document.querySelector('.account-workspace') && document.querySelector('.loading')?.textContent==='Add an account to get started.'`)
	awaitPage(deleteLogs, `() => !document.querySelector('.logs-window') && document.body.textContent.includes('Add an account to get started.')`)
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelector('.loading')?.textContent==='Add an account to get started.' && !document.querySelector('#account-profile').disabled`)
	page.MustElement("#account-profile").MustClick()
	page.MustElement("[data-add-account]").MustClick()
	page.MustElement("#account-name").MustInput("Fresh account")
	page.MustElement("#add-account-dialog button[type=submit]").MustClick()
	await(`() => !document.querySelector('#add-account-dialog') && document.querySelector('#account-profile .account-name')?.textContent==='Fresh account' && [...document.querySelectorAll('.app-menu button')].some(button=>button.textContent==='Add account')`)
	if list := m.GetProfiles(); len(list.Profiles) != 1 || list.Selected == "default" {
		t.Fatal("fresh profile reused deleted login", list)
	}
	if err := deleteLogs.Close(); err != nil {
		t.Fatal(err)
	}

}
