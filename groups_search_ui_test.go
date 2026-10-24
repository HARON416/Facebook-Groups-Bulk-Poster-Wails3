package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestGroupSearchAndBulkSelectionUI(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration; build frontend first")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	groups := []Group{{ID: "one", Name: "Kahawa West"}, {ID: "two", Name: "Kasarani residents"}, {ID: "three", Name: "Ruiru community"}}
	service := &GroupsService{settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	if err := service.saveGroups("fixture-user", groups); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	files := http.FileServer(http.Dir("frontend/dist"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewDecoder(r.Body).Decode(&call)
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
			result, _ = service.GetSettings()
			json.NewEncoder(w).Encode(result)
			return
		}
		if call.Args.MethodID == 2067197279 || call.Args.MethodName == "main.ProfileService.PublishRichPostWithOptions" {
			call.Args.MethodID = 2519456857
		}
		switch call.Args.MethodID {
		case 2512580126:
			result = "logged_in"
		case 2114412655, 928874507:
			result = true
		case 4280265894:
			result = service.GetGroupsLoadStatus()
		case 2519456857:
			var selected []string
			if len(call.Args.Args) != 5 || json.Unmarshal(call.Args.Args[0], &selected) != nil || len(selected) != 1 || selected[0] != "one" {
				w.WriteHeader(500)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unexpected selected groups"})
				return
			}
			var body, bodyHTML string
			_ = json.Unmarshal(call.Args.Args[2], &bodyHTML)
			if !strings.Contains(bodyHTML, "<strong>Draft</strong>") {
				t.Errorf("formatted body missing: %s", bodyHTML)
			}
			var uploads []PostUpload
			_ = json.Unmarshal(call.Args.Args[1], &body)
			_ = json.Unmarshal(call.Args.Args[3], &uploads)
			var options PostingOptions
			if json.Unmarshal(call.Args.Args[4], &options) != nil || options.MinimumInterval != 90 || options.MaximumInterval != 120 || options.Rewrite || options.Instructions != "" {
				t.Error("posting settings not passed to backend")
			}
			if body != "Draft" || len(uploads) != 1 || uploads[0].Name != "test.png" || uploads[0].Data != fixturePNG {
				w.WriteHeader(500)
				return
			}
			job, cancelJob := context.WithCancel(r.Context())
			service.postMu.Lock()
			service.postingCancel = cancelJob
			service.posting = PostingStatus{State: "paused"}
			service.postMu.Unlock()
			select {
			case <-time.After(400 * time.Millisecond):
				result = &PostingStatus{State: "complete", Message: "Finished: 1 published, 0 awaiting approval, 0 failed or skipped.", Results: []PostOutcome{{Group: groups[0], State: "posted", Message: "Post published.", PostID: "fixture-post"}}}
			case <-job.Done():
				result = &PostingStatus{State: "stopped", Message: "Posting stopped.", Results: []PostOutcome{}}
			}
			cancelJob()
			service.postMu.Lock()
			service.postingCancel = nil
			service.posting.State = result.(*PostingStatus).State
			service.postMu.Unlock()
		case 1478001064:
			result = service.GetPostingStatus()
		case 186143472:
			service.CancelPosting()
		case 1930204456:
			var query string
			_ = json.Unmarshal(call.Args.Args[0], &query)
			ids, err := service.SearchGroups(r.Context(), query)
			if err != nil {
				w.WriteHeader(500)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
				return
			}
			result = ids
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
	defer func() { _ = browser.Close(); time.Sleep(500 * time.Millisecond) }()
	page, err := browser.Page(proto.TargetCreateTarget{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	await := func(predicate string) {
		t.Helper()
		for {
			result, err := page.Eval(predicate)
			if err == nil && result.Value.Bool() {
				return
			}
			select {
			case <-ctx.Done():
				t.Fatalf("UI timed out: %s", predicate)
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	eval := func(script string) {
		t.Helper()
		if _, err := page.Eval(script); err != nil {
			t.Fatal(err)
		}
	}
	selectPostText := func() {
		eval(`() => { const input=document.querySelector('#post-body');input.focus();const range=document.createRange();range.selectNodeContents(input);const selection=getSelection();selection.removeAllRanges();selection.addRange(range); }`)
	}
	await(`() => document.querySelector('#post-body[contenteditable=true]') !== null`)
	selectPostText()
	page.MustElement("#post-body").MustInput("Draft")
	await(`() => document.querySelector('button.next').disabled`)
	eval(`() => {
  const bytes=Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a9S8AAAAASUVORK5CYII='),c=>c.charCodeAt(0));
  const transfer=new DataTransfer();transfer.items.add(new File([bytes],'test.png',{type:'image/png',lastModified:12345}));
  const input=document.querySelector('.file-input');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
 }`)
	await(`() => !document.querySelector('button.next').disabled && !document.querySelector('.image-selector')`)
	// Use the Wails native hover helpers and dispatch the same payload produced
	// by Go's native-drop reader. Browse and drop must share duplicate handling.
	eval(`() => {const zone=document.querySelector('#post-images-default[data-file-drop-target]');const tile=zone.querySelector('.image-drag');tile.scrollIntoView({block:'center',behavior:'instant'});const bounds=tile.getBoundingClientRect();window._wails.handleDragEnter();window._wails.handleDragOver(bounds.left+bounds.width/2,bounds.top+bounds.height/2);}`)
	await(`() => document.querySelector('#post-images-default').classList.contains('file-drop-target-active') && getComputedStyle(document.querySelector('#post-images-default')).outlineStyle==='dashed'`)
	eval(`() => window._wails.handleDragLeave()`)
	await(`() => !document.querySelector('#post-images-default').classList.contains('file-drop-target-active')`)
	dropDir := t.TempDir()
	imageBytes, _ := base64.StdEncoding.DecodeString(fixturePNG)
	firstImage, nextImage, invalidImage := filepath.Join(dropDir, "test.png"), filepath.Join(dropDir, "dropped.png"), filepath.Join(dropDir, "invalid.jpg")
	for _, path := range []string{firstImage, nextImage} {
		if err := os.WriteFile(path, imageBytes, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(invalidImage, []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	dropped := readDroppedImages([]string{firstImage, invalidImage, nextImage})
	dropped.Images[0].LastModified = 12345
	dropJSON, _ := json.Marshal(dropped)
	dispatchDrop := fmt.Sprintf(`() => window._wails.dispatchWailsEvent({name:'postika:images-dropped',data:%s})`, dropJSON)
	eval(dispatchDrop)
	await(`() => document.querySelectorAll('.previews .thumbnail').length===2 && document.querySelector('.file-message').textContent.includes('invalid.jpg') && document.querySelector('[aria-label="Remove dropped.png"]')!==null`)
	eval(dispatchDrop)
	await(`() => document.querySelectorAll('.previews .thumbnail').length===2`)
	// Native import must preserve the existing keyboard reorder controls.
	eval(`() => document.querySelector('.image-drag').dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',altKey:true,bubbles:true}))`)
	await(`() => document.querySelector('.image-drag img').alt==='dropped.png'`)
	eval(`() => document.querySelector('[aria-label="Remove dropped.png"]').click()`)
	await(`() => document.querySelectorAll('.previews .thumbnail').length===1 && document.querySelector('.image-drag img').alt==='test.png'`)
	selectPostText()
	page.MustElement("#post-body").MustInput("   ")
	await(`() => document.querySelector('button.next').disabled`)
	selectPostText()
	page.MustElement("#post-body").MustInput("Draft")
	await(`() => !document.querySelector('button.next').disabled`)
	// Formatting must update the editor state, preview, and submitted HTML.
	selectPostText()
	page.MustElement(`[aria-label="Bold"]`).MustClick()
	await(`() => document.querySelector('#post-body .post-bold')?.textContent==='Draft'`)
	// Soft line breaks (from paste or Shift+Enter) must become separate items,
	// retaining each line's inline formatting when switching list types.
	selectPostText()
	eval(`() => { const clipboard=new DataTransfer();clipboard.setData('text/html','<p><strong>First</strong><br><em>Second</em><br>Third</p>');document.querySelector('#post-body').dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:clipboard})); }`)
	await(`() => document.querySelectorAll('#post-body br').length===2`)
	selectPostText()
	page.MustElement(`[aria-label="Bullet list"]`).MustClick()
	await(`() => [...document.querySelectorAll('#post-body ul > li')].map(li=>li.textContent).join('|')==='First|Second|Third' && document.querySelector('#post-body li:first-child .post-bold')?.textContent==='First' && document.querySelector('#post-body li:nth-child(2) .post-italic')?.textContent==='Second'`)
	page.MustElement(`[aria-label="Undo"]`).MustClick()
	await(`() => !document.querySelector('#post-body ul') && document.querySelectorAll('#post-body br').length===2`)
	selectPostText()
	page.MustElement(`[aria-label="Bullet list"]`).MustClick()
	await(`() => document.querySelectorAll('#post-body ul > li').length===3`)
	selectPostText()
	page.MustElement(`[aria-label="Numbered list"]`).MustClick()
	await(`() => [...document.querySelectorAll('#post-body ol > li')].map(li=>li.textContent).join('|')==='First|Second|Third' && document.querySelector('#post-body li:nth-child(2) .post-italic')?.textContent==='Second'`)
	page.MustElement(`[aria-label="Numbered list"]`).MustClick()
	await(`() => !document.querySelector('#post-body ol') && document.querySelectorAll('#post-body p').length===1 && document.querySelectorAll('#post-body br').length===2 && document.querySelector('.editor-footer span:last-child').textContent==='18 characters'`)
	selectPostText()
	eval(`() => { const clipboard=new DataTransfer();clipboard.setData('text/plain','First\nSecond\nThird');document.querySelector('#post-body').dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:clipboard})); }`)
	await(`() => document.querySelectorAll('#post-body br').length===2`)
	selectPostText()
	page.MustElement(`[aria-label="Numbered list"]`).MustClick()
	await(`() => [...document.querySelectorAll('#post-body ol > li')].map(li=>li.textContent).join('|')==='First|Second|Third'`)
	page.MustElement(`[aria-label="Numbered list"]`).MustClick()
	await(`() => !document.querySelector('#post-body ol') && document.querySelectorAll('#post-body p').length===1 && document.querySelectorAll('#post-body br').length===2 && document.querySelector('.editor-footer span:last-child').textContent==='18 characters'`)
	selectPostText()
	eval(`() => { const clipboard=new DataTransfer();clipboard.setData('text/html','<ul><li><strong>First</strong><br><em>Second</em><br>Third</li></ul>');document.querySelector('#post-body').dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:clipboard})); }`)
	await(`() => document.querySelectorAll('#post-body ul > li').length===1 && document.querySelectorAll('#post-body br').length===2`)
	selectPostText()
	page.MustElement(`[aria-label="Numbered list"]`).MustClick()
	await(`() => [...document.querySelectorAll('#post-body ol > li')].map(li=>li.textContent).join('|')==='First|Second|Third'`)
	page.MustElement(`[aria-label="Numbered list"]`).MustClick()
	await(`() => !document.querySelector('#post-body ol') && document.querySelectorAll('#post-body p').length===1 && document.querySelectorAll('#post-body br').length===2 && document.querySelector('.editor-footer span:last-child').textContent==='18 characters'`)
	// Restore soft breaks without merging intentional paragraph gaps or removing
	// intentional blank lines. Repeated toggles must not accumulate spacing.
	selectPostText()
	eval(`() => { const clipboard=new DataTransfer();clipboard.setData('text/html','<p><strong>First</strong><br><br><em>Second</em></p><p>Separate paragraph</p>');document.querySelector('#post-body').dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:clipboard})); }`)
	await(`() => document.querySelectorAll('#post-body p').length===2 && document.querySelectorAll('#post-body br').length===2`)
	eval(`() => { window.originalPostText=document.querySelector('#post-body').innerText;window.originalCharacterCount=document.querySelector('.editor-footer span:last-child').textContent; }`)
	for range 2 {
		selectPostText()
		page.MustElement(`[aria-label="Bullet list"]`).MustClick()
		await(`() => document.querySelectorAll('#post-body ul > li').length===4`)
		page.MustElement(`[aria-label="Bullet list"]`).MustClick()
		await(`() => !document.querySelector('#post-body ul') && document.querySelectorAll('#post-body p').length===2 && document.querySelectorAll('#post-body br').length===2 && document.querySelector('#post-body').innerText===window.originalPostText && document.querySelector('.editor-footer span:last-child').textContent===window.originalCharacterCount && document.querySelector('#post-body .post-bold')?.textContent==='First' && document.querySelector('#post-body .post-italic')?.textContent==='Second'`)
	}
	selectPostText()
	eval(`() => { const clipboard=new DataTransfer();clipboard.setData('text/html','<p><strong>Draft</strong></p>');document.querySelector('#post-body').dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:clipboard})); }`)
	await(`() => document.querySelector('#post-body').textContent==='Draft'`)
	for _, style := range []struct{ label, selector string }{
		{"Heading 1", "h1"}, {"Heading 2", "h2"}, {"Quote", "blockquote"}, {"Bullet list", "ul"}, {"Numbered list", "ol"},
	} {
		selectPostText()
		page.MustElement(`[aria-label="` + style.label + `"]`).MustClick()
		await(fmt.Sprintf(`() => document.querySelector('#post-body %s')?.textContent==='Draft'`, style.selector))
		page.MustElement(`[aria-label="` + style.label + `"]`).MustClick()
		await(`() => document.querySelector('#post-body p')?.textContent==='Draft'`)
	}
	selectPostText()
	page.MustElement(`[aria-label="Italic"]`).MustClick()
	await(`() => document.querySelector('#post-body .post-italic')?.textContent==='Draft'`)
	page.MustElement(`[aria-label="Italic"]`).MustClick()
	await(`() => document.querySelector('#post-body .post-italic')===null`)
	// Pasting must keep supported emphasis and remove links, underline, styles,
	// and embedded images; keyboard underline must also be ignored.
	selectPostText()
	eval(`() => { const input=document.querySelector('#post-body');const clipboard=new DataTransfer();clipboard.setData('text/html','<p><u><a href="https://example.com"><strong style="color:red">Draft</strong></a></u><img src="invalid" /></p>');input.dispatchEvent(new ClipboardEvent('paste',{bubbles:true,cancelable:true,clipboardData:clipboard})); }`)
	await(`() => document.querySelector('#post-body .post-bold')?.textContent==='Draft' && !document.querySelector('#post-body u, #post-body a, #post-body img, #post-body [style]')`)
	selectPostText()
	eval(`() => document.querySelector('#post-body').dispatchEvent(new KeyboardEvent('keydown',{key:'u',code:'KeyU',ctrlKey:true,bubbles:true,cancelable:true}))`)
	await(`() => !document.querySelector('#post-body u') && !document.querySelector('#post-body [style*=underline]')`)

	await(`() => !document.querySelector('#post-preview-tab, #post-editor-preview') && document.querySelector('#post-body .post-bold')?.textContent==='Draft'`)
	if dir := *uiArtifactDir; dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		page.MustSetViewport(1280, 900, 1, false)
		eval(`() => document.querySelector('[data-step-scroll]').scrollTop=0`)
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "postika-rich-editor.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	eval(`() => { const button=document.querySelector('button.next'); const rect=button.getBoundingClientRect(); window.confettiOrigin={x:rect.left+rect.width/2,y:rect.top+rect.height/2}; button.click(); }`)
	await(`() => document.querySelector('.load-groups') !== null`)
	await(`() => { const effect=document.querySelector('.click-confetti'); return effect !== null && effect.querySelector('svg') !== null && Math.abs(parseFloat(effect.style.left)-window.confettiOrigin.x)<1 && Math.abs(parseFloat(effect.style.top)-window.confettiOrigin.y)<1 && getComputedStyle(effect).pointerEvents==='none'; }`)
	await(`() => { const effect=document.querySelector('.click-confetti'); const viewBox=effect?.querySelector('svg')?.viewBox.baseVal; return effect !== null && viewBox && Math.abs(parseFloat(effect.style.width)/parseFloat(effect.style.height)-viewBox.width/viewBox.height)<.01 && effect.style.transform.includes('px'); }`)
	if dir := *uiArtifactDir; dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "postika-confetti.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	await(`() => document.querySelector('.click-confetti') === null`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	eval(`() => document.querySelector('button.previous').click()`)
	await(`() => document.querySelector('.composer') !== null && document.querySelector('.click-confetti') === null && document.querySelector('#post-body .post-bold')?.textContent==='Draft'`)
	eval(`() => document.querySelector('button.next').click()`)
	await(`() => document.querySelector('.load-groups') !== null && document.querySelector('.click-confetti') === null`)
	if err := (proto.EmulationSetEmulatedMedia{}).Call(page); err != nil {
		t.Fatal(err)
	}
	await(`() => document.querySelectorAll('.groups-list li').length === 3`)
	await(`() => document.querySelector('.group-status')?.textContent.includes('Restored 3 saved groups')`)
	await(`() => document.querySelector('button.next').disabled`)
	eval(`() => document.querySelector('.checkbox__trigger').click()`)
	await(`() => !document.querySelector('button.next').disabled`)
	eval(`() => {const input=document.querySelector('#group-search');input.value='kaha';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	await(`() => document.querySelectorAll('.groups-list li').length === 1 && document.querySelector('.groups-list').textContent.includes('Kahawa')`)
	await(`() => document.querySelector('.checkbox__trigger').checked`)
	await(`() => { const mark=document.querySelector('.search-match'); return mark && mark.textContent.length > 0 && getComputedStyle(mark).color !== getComputedStyle(mark.parentElement).color && getComputedStyle(mark).backgroundColor==='rgba(0, 0, 0, 0)' && document.querySelector('.checkbox__textwrapper').textContent === 'Kahawa West'; }`)
	eval(`() => {const input=document.querySelector('#group-search');input.value='kahwa';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	await(`() => !document.querySelector('.search-message') && document.querySelectorAll('.groups-list li').length === 1 && document.querySelector('.search-match')?.textContent === 'Kahawa'`)
	eval(`() => document.querySelector('.bulk-actions button').click()`)
	await(`() => document.querySelector('.selection-count').textContent.includes('3 selected')`)
	eval(`() => document.querySelector('.clear-search').click()`)
	await(`() => document.querySelectorAll('.groups-list li').length === 3 && Array.from(document.querySelectorAll('.checkbox__trigger')).every(input => input.checked)`)
	await(`() => document.activeElement.id === 'group-search'`)
	await(`() => document.querySelectorAll('.search-match').length === 0`)
	eval(`() => document.querySelectorAll('.bulk-actions button')[1].click()`)
	await(`() => Array.from(document.querySelectorAll('.checkbox__trigger')).every(input => !input.checked)`)
	await(`() => document.querySelector('button.next').disabled`)
	eval(`() => {const input=document.querySelector('#group-search');input.value='zzzzmissing';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	await(`() => document.querySelector('.search-message')?.textContent.includes('No groups found')`)
	eval(`() => document.querySelector('#group-search').dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}))`)
	await(`() => document.querySelectorAll('.groups-list li').length === 3`)
	eval(`() => document.querySelector('.checkbox__trigger').click()`)
	await(`() => !document.querySelector('button.next').disabled`)
	eval(`() => document.querySelector('button.next').click()`)
	await(`() => document.querySelector('#step-heading')?.textContent === 'Post or schedule' && !!document.querySelector('.run-settings')`)
	await(`() => !document.querySelector('button.post').disabled`)
	eval(`() => { const input=document.querySelector('input[aria-label="Minimum (seconds)"]'); input.value='90'; input.dispatchEvent(new Event('input',{bubbles:true})); }`)
	await(`() => !document.querySelector('.rewrite-card') && !document.querySelector('#rewrite-instructions') && !document.querySelector('.settings-actions') && ![...document.querySelectorAll('.app-menu button')].some(b=>b.textContent==='Settings')`)
	// Legacy drafts must not silently turn rewriting back on after an update.
	eval(`async () => {const db=await new Promise((resolve,reject)=>{const r=indexedDB.open('postika-account-drafts',2);r.onsuccess=()=>resolve(r.result);r.onerror=reject;});await new Promise((resolve,reject)=>{const tx=db.transaction('drafts','readwrite'),store=tx.objectStore('drafts'),r=store.get('default');r.onsuccess=()=>store.put({...r.result,minimumInterval:90,rewrite:true,instructions:'Legacy rewrite instructions'},'default');tx.oncomplete=resolve;tx.onerror=reject;});db.close();}`)
	page.MustReload().MustWaitLoad()
	await(`() => document.querySelector('#step-heading')?.textContent==='Post or schedule' && document.querySelector('input[aria-label="Minimum (seconds)"]')?.value==='90' && !document.querySelector('button.post').disabled`)
	if dir := *uiArtifactDir; dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		page.MustSetViewport(1200, 1000, 1, false)
		data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "postika-posting-preferences.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	await(`() => !document.querySelector('nav[aria-label="Post progress"] li:last-child').classList.contains('completed')`)
	eval(`() => document.querySelector('button.post').click()`)
	await(`() => document.querySelector('button.post').disabled && document.querySelector('button.post').textContent.includes('Posting')`)
	await(`() => !document.querySelector('nav[aria-label="Post progress"] li:last-child').classList.contains('completed')`)
	await(`() => document.querySelector('.group-status')?.textContent.includes('1 published') && !document.querySelector('button.post').disabled && !document.querySelector('.posting-results') && [...document.querySelectorAll('.step-actions button')].some(button => button.textContent.trim() === 'View logs')`)
	await(`() => document.querySelectorAll('nav[aria-label="Post progress"] li.completed').length===3 && document.querySelector('nav[aria-label="Post progress"] li:last-child .sr-only').textContent.includes('Completed') && !document.querySelector('nav[aria-label="Post progress"] [aria-current]')`)
	// Navigation retains the completed run; changing the selected groups resets it.
	eval(`() => document.querySelector('button.previous').click()`)
	await(`() => document.querySelector('.checkbox__trigger')?.checked && document.querySelectorAll('nav[aria-label="Post progress"] li.completed').length===3`)
	eval(`() => document.querySelector('.checkbox__trigger').click()`)
	await(`() => !document.querySelector('nav[aria-label="Post progress"] li:last-child').classList.contains('completed')`)
	eval(`() => document.querySelector('.checkbox__trigger').click()`)
	await(`() => !document.querySelector('button.next').disabled`)
	eval(`() => document.querySelector('button.next').click()`)
	await(`() => document.querySelector('button.post')!==null`)

	eval(`() => document.querySelector('button.post').click()`)
	await(`() => document.querySelector('button.post').disabled && !document.querySelector('nav[aria-label="Post progress"] li:last-child').classList.contains('completed')`)
	awaitPostingState(t, service, "paused")
	eval(`() => [...document.querySelectorAll('.step-actions button')].find(b=>b.textContent.trim()==='Stop').click()`)
	await(`() => !document.querySelector('button.post').disabled && document.querySelector('.group-status')?.textContent==='Posting stopped.' && !document.querySelector('nav[aria-label="Post progress"] li:last-child').classList.contains('completed')`)
	eval(`() => document.querySelector('button.previous').click()`)
	await(`() => document.querySelector('.checkbox__trigger')?.checked && !document.querySelector('button.next').disabled`)
	// Old settings URLs must no longer expose a settings window.
	page.MustNavigate(server.URL + "/?view=settings").MustWaitLoad()
	await(`() => document.querySelector('.top-bar') && !document.querySelector('input[type=password]')`)
}
