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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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
				snapshot, _ := target.Context(context.Background()).Timeout(time.Second).Eval(`() => ({text:document.body.innerText.slice(0,2000),inputs:[...document.querySelectorAll('input')].map(i=>({type:i.type,disabled:i.disabled})),errors:[...document.querySelectorAll('.error')].map(e=>e.textContent)})`)
				t.Fatalf("UI timed out: %s (%v), snapshot: %v", predicate, err, snapshot)
			case <-time.After(25 * time.Millisecond):
				if time.Now().After(deadline) {
					snapshot, _ := target.Context(context.Background()).Timeout(time.Second).Eval(`() => ({text:document.body.innerText.slice(0,2000),inputs:[...document.querySelectorAll('input')].map(i=>({type:i.type,disabled:i.disabled}))})`)
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
		awaitPage(target, `() => document.querySelector('dialog')?.open && document.querySelectorAll('#theme-options [role=option]').length===189`)
		target.MustElement(fmt.Sprintf(`[data-theme-id="%s"]`, id)).MustClick()
		awaitPage(target, `() => !document.querySelector('dialog').open`)
	}

	eval := func(script string) {
		t.Helper()
		if _, err := page.Eval(script); err != nil {
			t.Fatal(err)
		}
	}
	selectAccount := func(id string) {
		t.Helper()
		eval(`() => document.querySelector('#account-profile').click()`)
		await(`() => document.querySelector('[role=listbox]') !== null`)
		eval(fmt.Sprintf(`() => document.querySelector('[role=option][data-profile-id="%s"]').click()`, id))
		await(fmt.Sprintf(`() => document.querySelector('#account-profile').dataset.profileId===%q && !document.querySelector('#account-profile').disabled`, id))
	}
	await(`() => document.querySelector('#post-body[contenteditable=true]') !== null`)
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
	eval(`() => [...document.querySelectorAll('.app-menu button')].find(b=>b.textContent==='Settings').click()`)
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
	await(`() => document.querySelector('.eyebrow')?.textContent==='LANGKAH 1 DARI 3' && document.querySelector('.editor-footer')?.textContent.includes('karakter') && document.querySelector('.editor-footer')?.textContent.includes('Mendukung pintasan Markdown') && document.body.innerText.includes('Tulis postingan Anda dan tambahkan gambar') && document.body.innerText.includes('Mulai dengan pesan Anda.') && document.querySelector('[aria-label="Tebal"]') !== null && document.querySelector('#image-formats')?.textContent.includes('Pilih beberapa gambar')`)
	selectAccount("default")
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .next').click()`)
	await(`() => document.querySelector('#group-search')?.placeholder==='Cari grup…' && document.querySelector('.selection-count')?.textContent.includes('dipilih')`)
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .next').click()`)
	await(`() => document.querySelector('#step-heading')?.textContent==='Preferensi & kirim' && document.querySelector('.account-context') === null && document.body.innerText.includes('Jeda antarpostingan') && document.body.innerText.includes('Kirim sekarang') && document.querySelector('[aria-label="Minimum (detik)"]') !== null`)
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
	settingsPage, err := browser.Page(proto.TargetCreateTarget{URL: server.URL + "/?view=settings&profile=" + business.ID + "&account=Business"})
	if err != nil {
		t.Fatal(err)
	}
	awaitPage(settingsPage, `() => document.querySelector('h1')?.textContent==='Mipangilio' && document.documentElement.lang==='sw'`)
	settingsPage.MustElement(`input[type=password]`).MustInput("unsaved-test-key")
	awaitPage(settingsPage, `() => document.documentElement.dataset.theme==='dark'`)
	chooseTheme(settingsPage, "light")
	await(`() => document.documentElement.dataset.theme==='light' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	awaitPage(settingsPage, `() => document.documentElement.dataset.theme==='light' && document.querySelector('input[type=password]').value==='unsaved-test-key'`)
	chooseTheme(page, "dark")
	awaitPage(settingsPage, `() => document.documentElement.dataset.theme==='dark' && document.querySelector('input[type=password]').value==='unsaved-test-key'`)
	settingsPage.MustEval(`() => {window.dispatchEvent(new Event('postika:window-hidden'));localStorage.setItem('postika:theme','light');window.dispatchEvent(new Event('postika:window-shown'));}`)
	awaitPage(settingsPage, `() => document.documentElement.dataset.theme==='light' && document.querySelector('input[type=password]').value==='unsaved-test-key'`)
	await(`() => document.documentElement.dataset.theme==='light'`)
	chooseTheme(page, "dark")
	awaitPage(settingsPage, `() => document.documentElement.dataset.theme==='dark'`)
	// Auxiliary titlebars share the controls and localize the default account.
	for _, view := range []struct{ query, title, screenshot string }{{"settings", "Mipangilio", "postika-settings-titlebar.png"}, {"posting-logs", "Kumbukumbu za uchapishaji", "postika-logs-titlebar.png"}} {
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
		if view.query == "settings" {
			if err := aux.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 600, Height: 480, DeviceScaleFactor: 1}); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => !document.querySelector('input[type=checkbox]').disabled`)
			assertWindowScrollContained(t, aux, ".auxiliary-content", ".auxiliary-title-bar")
			aux.MustEval(`() => document.querySelector('input[type=checkbox]').click()`)
			awaitPage(aux, `() => document.querySelector('textarea')!==null`)
			assertTextareaScrollContained(t, aux)
			aux.MustEval(`() => {document.querySelector('input[type=checkbox]').click();document.querySelector('.auxiliary-content').scrollTop=0;}`)
			if err := aux.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: 760, Height: 760, DeviceScaleFactor: 1}); err != nil {
				t.Fatal(err)
			}
		}
		if view.query == "posting-logs" {
			chooseTheme(aux, "light")
			await(`() => document.documentElement.dataset.theme==='light'`)
			awaitPage(settingsPage, `() => document.documentElement.dataset.theme==='light' && document.querySelector('input[type=password]').value==='unsaved-test-key'`)
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
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===189 && document.activeElement.matches('dialog input')`)
			aux.MustElement("dialog input").MustInput("no-theme-matches-this")
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===0 && document.querySelector('.no-results')!==null`)
			aux.MustEval(`() => document.querySelector('dialog input').focus()`)
			if err := aux.Keyboard.Type(input.Escape); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => !document.querySelector('dialog').open && document.activeElement.matches('[data-theme-picker]') && !document.documentElement.dataset.palette`)
			aux.MustElement("[data-theme-picker]").MustClick()
			aux.MustElement("dialog input").MustInput("CATPPUCCIN")
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===1`)
			if err := aux.Keyboard.Type(input.Enter); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => !document.querySelector('dialog').open && document.documentElement.dataset.palette==='palette:catppuccin' && getComputedStyle(document.documentElement).backgroundColor==='rgb(30, 30, 46)' && localStorage.getItem('postika:theme')==='palette:catppuccin'`)
			await(`() => document.documentElement.dataset.palette==='palette:catppuccin' && document.querySelector('#post-body')?.textContent==='Business draft'`)
			awaitPage(settingsPage, `() => document.documentElement.dataset.palette==='palette:catppuccin' && document.querySelector('input[type=password]').value==='unsaved-test-key'`)
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
			aux.MustElement("dialog input").MustInput("Postika Dark")
			awaitPage(aux, `() => document.querySelectorAll('#theme-options [role=option]').length===1`)
			if err := aux.Keyboard.Type(input.Enter); err != nil {
				t.Fatal(err)
			}
			awaitPage(aux, `() => !document.documentElement.dataset.palette && document.documentElement.dataset.theme==='dark' && getComputedStyle(document.documentElement).backgroundColor==='rgb(21, 28, 25)'`)
			await(`() => !document.documentElement.dataset.palette && document.documentElement.dataset.theme==='dark'`)
			awaitPage(settingsPage, `() => !document.documentElement.dataset.palette && document.documentElement.dataset.theme==='dark'`)
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
	// A user account with the same literal name is user data, not a UI label.
	customNamePage, err := browser.Page(proto.TargetCreateTarget{URL: server.URL + "/?view=settings&profile=" + business.ID + "&account=Default%20profile"})
	if err != nil {
		t.Fatal(err)
	}
	awaitPage(customNamePage, `() => document.querySelector('.window-account')?.textContent==='Default profile' && document.documentElement.lang==='sw'`)
	// A blocked storage write still allows theme changes for the current session.
	customNamePage.MustEval(`() => {Storage.prototype.setItem=function(){throw new DOMException('Storage blocked for test','QuotaExceededError')};}`)
	chooseTheme(customNamePage, "light")
	customNamePage.MustEval(`() => {window.dispatchEvent(new Event('focus'));window.dispatchEvent(new Event('postika:window-shown'));}`)
	awaitPage(customNamePage, `() => document.documentElement.dataset.theme==='light'`)
	await(`() => document.documentElement.dataset.theme==='dark'`)
	if err := customNamePage.Close(); err != nil {
		t.Fatal(err)
	}
	changeLanguage("id")
	awaitPage(settingsPage, `() => document.querySelector('h1')?.textContent==='Pengaturan' && document.querySelector('input[type=password]').placeholder==='Masukkan kunci API Gemini Anda' && document.querySelector('input[type=password]').value==='unsaved-test-key' && document.body.innerText.includes('Jeda antarpostingan') && document.body.innerText.includes('Kunci tersimpan tidak pernah ditampilkan.')`)
	changeLanguage("ar")
	await(`() => document.documentElement.dir==='rtl' && document.querySelector('#step-heading')?.textContent==='إنشاء منشور' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	awaitPage(settingsPage, `() => document.querySelector('h1')?.textContent==='الإعدادات' && document.querySelector('input[type=password]').value==='unsaved-test-key'`)
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='ar' && document.documentElement.dir==='rtl' && document.documentElement.dataset.theme==='dark' && document.querySelector('#post-body')?.textContent==='Business draft'`)
	changeLanguage("ja")
	await(`() => document.documentElement.dir==='ltr' && document.querySelector('#step-heading')?.textContent==='投稿を作成'`)
	// Exercise every bundled catalog; locale loading must work without external services.
	for _, code := range []string{"id", "cs", "da", "de", "en", "es", "fr", "hr", "it", "sw", "hu", "nl", "pl", "pt-BR", "ro", "sk", "sv", "vi", "tr", "el", "bg", "ru", "uk", "ar", "mr", "hi", "bn", "te", "th", "ko", "zh-CN", "ja"} {
		changeLanguage(code)
		await(`() => document.querySelector('#post-body')?.textContent==='Business draft' && document.querySelector('#step-heading')?.textContent.length>0`)
		if code != "en" {
			await(`() => document.querySelector('[data-theme-picker]')?.getAttribute('aria-label') !== 'Color themes'`)
			eval(`() => document.querySelector('#account-profile').click()`)
			await(`() => document.querySelector('#account-options [data-profile-id="default"] .account-name')?.textContent !== 'Default profile' && document.querySelector('#account-options [data-profile-id="default"] .account-name')?.textContent.length > 0 && document.querySelector('#account-profile .account-name')?.textContent==='Business'`)
			eval(`() => document.querySelector('#account-profile').click()`)
			await(`() => !document.querySelector('.account-workspace:not([hidden])').innerText.includes('Write your post and add images') && !document.querySelector('.account-workspace:not([hidden])').innerText.includes('Load groups and choose where to post') && !document.querySelector('.account-workspace:not([hidden])').innerText.includes('Review, set preferences, then post now or schedule') && !document.querySelector('.editor-footer')?.textContent.includes('characters') && !document.querySelector('#image-formats')?.textContent.includes('Select multiple images')`)
			awaitPage(settingsPage, fmt.Sprintf(`() => document.documentElement.lang===%q && document.querySelector('input[type=password]').value==='unsaved-test-key' && document.querySelector('input[type=password]').placeholder!=='Enter your Gemini API key' && !document.body.innerText.includes('Choose how long to wait before posting to the next group.')`, code))
			awaitPage(settingsPage, `() => document.querySelector('.window-label')?.textContent===document.querySelector('h1')?.textContent && document.querySelector('.window-account')?.textContent==='Business' && document.title===document.querySelector('h1').textContent+' · Business · Postika'`)
		}
	}
	// Unsupported saved preferences recover to source English on restart.
	eval(`() => {localStorage.setItem('postika:language','unsupported-locale');localStorage.setItem('postika:theme','unsupported-theme')}`)
	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}
	await(`() => document.documentElement.lang==='en' && document.documentElement.dataset.theme==='light' && document.querySelector('#step-heading')?.textContent==='Create post'`)
	if err := settingsPage.Close(); err != nil {
		t.Fatal(err)
	}
	// The add-account flow creates a fresh workspace and requests its own login.
	eval(`() => [...document.querySelectorAll('.app-menu button')].find(b=>b.textContent==='Add account').click()`)
	page.MustElement("#account-name").MustInput("Third account")
	page.MustElement(".add-account button").MustClick()
	await(`() => document.querySelector('#account-profile .account-name')?.textContent==='Third account' && document.querySelector('#step-heading')?.textContent==='Select groups' && !document.querySelector('.groups-list')`)
	changeLanguage("id")
	eval(`() => document.querySelector('.account-workspace:not([hidden]) .previous').click()`)
	await(`() => document.querySelector('.placeholder')?.textContent==='Apa yang ingin Anda bagikan?' && document.querySelector('.editor-footer')?.textContent.includes('0 karakter') && document.querySelector('#browse-instruction')?.textContent.includes('telusuri gambar')`)
	changeLanguage("en")
	closed.Store(false)
	eval(`() => document.querySelector('[aria-label="Close window"]').click()`)
	await(`() => !document.querySelector('[aria-label="Close window"]').disabled`)
	if !closed.Load() {
		t.Fatal("close did not reach native window API")
	}
	if len(m.GetProfiles().Profiles) != 3 {
		t.Fatal("add account did not persist profile")
	}
}
