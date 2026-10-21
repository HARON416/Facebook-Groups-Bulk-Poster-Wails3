package main

import (
	"context"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestPostGroupValidation(t *testing.T) {
	for _, address := range []string{"https://example.com/groups/one/", "http://web.facebook.com/groups/one/", "https://web.facebook.com/groups/joins/", "https://web.facebook.com/groups/", "https://user@web.facebook.com/groups/one/"} {
		if _, err := facebookGroupURL(Group{URL: address}); err == nil {
			t.Fatalf("accepted %s", address)
		}
	}
	if _, err := facebookGroupURL(Group{URL: "https://web.facebook.com/groups/my-group/"}); err != nil {
		t.Fatal(err)
	}
	service := &GroupsService{groupsLoad: &groupsLoad{}}
	service.groupsLoad.set("complete", "", []Group{{ID: "one", URL: "https://web.facebook.com/groups/one/"}})
	for _, ids := range [][]string{nil, {"unknown"}} {
		if _, err := service.OpenPostComposer(context.Background(), ids, "Draft", nil); err == nil {
			t.Fatalf("accepted selection %v", ids)
		}
	}
	if service.browser != nil {
		t.Fatal("invalid selection launched a browser")
	}
}

func TestPostComposerBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome is required")
	}
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true, groupsLoad: &groupsLoad{cancel: func() {}}}
	t.Cleanup(func() { _ = service.ServiceShutdown(); time.Sleep(500 * time.Millisecond) })
	groups := []Group{{ID: "one", Name: "First", URL: "https://web.facebook.com/groups/one/"}, {ID: "two", Name: "Second", URL: "https://web.facebook.com/groups/two/"}}
	service.groupsLoad.set("complete", "", groups)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	service.mu.Lock()
	err := service.ensureBrowserPage(ctx)
	service.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	originalPage, originalBrowser := service.page, service.browser
	// This fixture fulfills every request without a network connection, so it
	// cannot emit wire ExtraInfo. Real wire capture has its own integration test.
	service.photoUploadWatcher = func(context.Context, *rod.Page) *photoUploadCapture { return nil }
	// All browser requests are fulfilled locally; this never contacts Facebook.
	var documents atomic.Int32
	var loggedOut atomic.Bool
	var missingFirst atomic.Bool
	var missingAll atomic.Bool
	router := service.page.HijackRequests()
	err = router.Add("*", "", func(h *rod.Hijack) {
		if h.Request.Type() != "Document" {
			if isFacebookPhotoUploadURL(h.Request.URL().String()) {
				h.Response.SetHeader("Access-Control-Allow-Origin", "https://web.facebook.com", "Access-Control-Allow-Credentials", "true", "Access-Control-Allow-Methods", "POST, OPTIONS", "Access-Control-Allow-Headers", "x-upload-test", "Content-Type", "application/json").SetBody("{}")
			} else {
				h.Response.SetBody("")
			}
			return
		}
		documents.Add(1)
		if loggedOut.Load() {
			h.Response.SetHeader("Content-Type", "text/html").SetBody("<body class='UIPage_LoggedOut'><form id='login_form'></form>")
			return
		}
		if missingFirst.Swap(false) || missingAll.Load() {
			h.Response.SetHeader("Content-Type", "text/html").SetBody("<body class='UIPage_LoggedIn'><main>No composer</main></body>")
			return
		}
		h.Response.SetHeader("Content-Type", "text/html").SetBody(`<body class="UIPage_LoggedIn">
 <script>document.cookie='c_user=fixture-owner;domain=.facebook.com;path=/';window.clicked=0;function uploadImages(files){for(const file of files){const form=new FormData();form.append('source','8');form.append('profile_id','fixture-owner');form.append('upload_id',crypto.randomUUID());form.append('farr',file);fetch('https://upload.facebook.com/ajax/react_composer/attachments/photo/upload?__user=fixture-owner&av=fixture-owner&fb_dtsg=fresh-fixture-token',{method:'POST',credentials:'include',headers:{'x-upload-test':'fresh'},body:form})}};setTimeout(()=>document.querySelector('#composer').hidden=false,300)</script>
 <div style="display:none" class="x1i10hfl x1ejq31n x18oe1m7 x1sy0etr xstzfhl" role="button" tabindex="0"><div class="xi81zsa x1lkfr7t xkjl1po x1mzt3pk xh8yej3 x13faqbe"><span class="x1lliihq x6ikm8r x10wlt62 x1n2onr6">Hidden</span></div><div role="none" data-visualcompletion="ignore"></div></div>
 <div hidden id="composer" class="x1i10hfl x1ejq31n x18oe1m7 x1sy0etr xstzfhl" role="button" tabindex="0" onclick="setInterval(()=>document.querySelector('#dialog').dataset.tracking=String(Date.now()),100);window.clicked++;document.body.dataset.open='yes';document.querySelector('#dialog').hidden=false;window.settled=false;setTimeout(()=>{const dialog=document.querySelector('#dialog');dialog.replaceWith(dialog.cloneNode(true))},200);[100,300,700].forEach((delay,i)=>setTimeout(()=>{document.querySelector('#dialog').appendChild(document.createElement('span'));if(i===2)window.settled=true},delay))"><div class="xi81zsa x1lkfr7t xkjl1po x1mzt3pk xh8yej3 x13faqbe"><span class="x1lliihq x6ikm8r x10wlt62 x1n2onr6">Écrivez quelque chose…</span></div><div role="none" data-visualcompletion="ignore"></div></div>
 <div id="dialog" role="dialog" aria-modal="true" hidden>
 <form method="post" onsubmit="return false">
 <input class="x1s85apg" type="file" accept="image/*" multiple onchange="uploadImages(this.files);window.insertedEarly=!window.settled;window.files=Array.from(this.files).map(f=>f.name);window.sequence=['images'];document.querySelector('#editor').hidden=false;setTimeout(()=>{const editor=document.querySelector('#editor');const replacement=editor.cloneNode(true);replacement.innerHTML='';editor.replaceWith(replacement)},100)">
 <div id="editor" role="textbox" contenteditable="true" data-lexical-editor="true" oninput="window.sequence.push('text')"></div>
 <input type="submit" hidden>
 <div id="submit-post" role="button" tabindex="0" aria-disabled="true" class="changed-facebook-classes" onclick="window.submitted=true"><span>Publier</span><div role="none" data-visualcompletion="ignore"></div></div>
 </form>
 </div>
 </body>`)
	})
	if err != nil {
		t.Fatal(err)
	}
	go router.Run()
	defer router.Stop()
	selectedIDs := []string{"one", "two", "one"}
	prepared, err := service.OpenPostComposer(ctx, selectedIDs, "Hello 🌍\nSecond line", []PostUpload{{Name: "first.png", Data: fixturePNG}, {Name: "second.png", Data: fixturePNG}})
	if err != nil {
		t.Fatal(err)
	}

	shuffled := prepared.Groups
	if len(shuffled) != 2 || shuffled[0].ID == shuffled[1].ID || !reflect.DeepEqual(selectedIDs, []string{"one", "two", "one"}) {
		t.Fatalf("selection: %+v", shuffled)
	}
	info, err := service.page.Context(ctx).Info()
	if err != nil || info.URL != prepared.OpenedGroup.URL {
		t.Fatalf("opened wrong group: %+v %v", info, err)
	}
	clicked, err := service.page.Context(ctx).Eval("() => window.clicked")
	if err != nil || clicked.Value.Int() != 1 || documents.Load() != 1 {
		t.Fatalf("composer clicks: %v %v, navigations: %d", clicked, err, documents.Load())
	}
	if service.page != originalPage || service.browser != originalBrowser {
		t.Fatal("did not reuse browser profile and tab")
	}
	result, err := service.page.Context(ctx).Eval(`() => ({text:document.querySelector('#editor').innerText,files:window.files,sequence:window.sequence,submitted:!!window.submitted,insertedEarly:!!window.insertedEarly})`)
	if err != nil || result.Value.Get("text").Str() != "Hello 🌍\nSecond line" || result.Value.Get("files").Arr()[0].Str() != "first.png" || result.Value.Get("files").Arr()[1].Str() != "second.png" || result.Value.Get("sequence").Arr()[0].Str() != "images" || result.Value.Get("submitted").Bool() || result.Value.Get("insertedEarly").Bool() {
		t.Fatalf("draft: %v %v", result, err)
	}
	// Replacing a contenteditable draft must not append or duplicate text.
	if err := fillGroupComposer(ctx, service.page.Context(ctx), "Replacement", nil, &service.postImageDir); err != nil {
		t.Fatal(err)
	}
	replaced, err := service.page.Context(ctx).Eval(`() => document.querySelector('#editor').innerText`)
	if err != nil || replaced.Value.Str() != "Replacement" {
		t.Fatalf("replacement: %v %v", replaced, err)
	}
	// HTML must be delivered through the editor's paste handler, never a direct
	// DOM assignment. Reject an editor that ignores that handler before submit.
	richCtx := context.WithValue(ctx, richPostBodyKey{}, richPostBody{"Rich draft", "<p><strong>Rich</strong> draft</p>"})
	if err := fillGroupComposer(richCtx, service.page.Context(ctx), "Rich draft", nil, &service.postImageDir); err == nil {
		t.Fatal("editor that ignored formatted paste was accepted")
	}
	_, err = service.page.Context(ctx).Eval(`() => document.querySelector('#editor').addEventListener('paste', event => {
		event.preventDefault(); window.pastedHTML=event.clipboardData.getData('text/html');
		const range=getSelection().getRangeAt(0);range.deleteContents();range.insertNode(range.createContextualFragment(window.pastedHTML));
	})`)
	if err != nil {
		t.Fatal(err)
	}
	if err := fillGroupComposer(richCtx, service.page.Context(ctx), "Rich draft", nil, &service.postImageDir); err != nil {
		t.Fatal(err)
	}
	richResult, err := service.page.Context(ctx).Eval(`() => window.pastedHTML === '<p><strong>Rich</strong> draft</p>' && document.querySelector('#editor strong').textContent === 'Rich' && !window.submitted`)
	if err != nil || !richResult.Value.Bool() {
		t.Fatalf("formatted paste: %v %v", richResult, err)
	}
	// Exercise the production submit lookup and native click after preparation.
	// Enabled actions may omit aria-disabled entirely, with changed classes and
	// translated text. Keep the dialog busy and mutating after enabling: those
	// updates must not delay an available action. Still wait while disabled.
	_, err = service.page.Context(ctx).Eval(`() => {
		const dialog = document.querySelector('#dialog');
		dialog.setAttribute('aria-busy', 'true');
		const progress = document.createElement('div');
		progress.setAttribute('role', 'progressbar');
		progress.textContent = 'Updating preview';
		dialog.appendChild(progress);
		setInterval(() => progress.textContent = String(performance.now()), 30);
		document.querySelector('#submit-post').addEventListener('click', () => window.submitClickedAt = performance.now());
		setTimeout(() => {
			window.submitReadyAt = performance.now();
			document.querySelector('#submit-post').removeAttribute('aria-disabled');
		}, 400);
	}`)
	if err != nil {
		t.Fatal(err)
	}
	button, err := waitPostSubmit(ctx, service.page.Context(ctx), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	submitted, err := service.page.Context(ctx).Eval(`() => window.submitted && !document.querySelector('#submit-post').hasAttribute('aria-disabled')`)
	if err != nil || !submitted.Value.Bool() {
		t.Fatalf("enabled composer action was not clicked: %v %v", submitted, err)
	}
	latency, err := service.page.Context(ctx).Eval(`() => window.submitClickedAt - window.submitReadyAt`)
	if err != nil || latency.Value.Num() < 0 || latency.Value.Num() > 300 {
		t.Fatalf("submit action was not clicked promptly after enabling: %v ms, %v", latency, err)
	}
	missingFirst.Store(true)
	before := documents.Load()
	prepared, err = service.OpenPostComposer(ctx, []string{"one", "two"}, "Skipped first group", nil)
	if err != nil || len(prepared.Skipped) != 1 || prepared.OpenedGroup.ID == prepared.Skipped[0].ID || documents.Load() != before+2 {
		t.Fatalf("skip to next group: %+v %v", prepared, err)
	}
	missingAll.Store(true)
	_, err = service.OpenPostComposer(ctx, []string{"one"}, "Draft", nil)
	if err == nil || !strings.Contains(err.Error(), "None of the 1 selected groups") {
		t.Fatalf("exhausted groups: %v", err)
	}
	missingAll.Store(false)
	loggedOut.Store(true)
	if _, err := service.OpenPostComposer(ctx, []string{"one"}, "Draft", nil); err == nil || !strings.Contains(err.Error(), "Log in") {
		t.Fatalf("logout: %v", err)
	}
}

// A real PNG signature and body, decoded and sniffed by the backend.
const fixturePNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a9S8AAAAASUVORK5CYII="
