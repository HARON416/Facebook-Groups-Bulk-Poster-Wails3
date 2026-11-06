package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplacePublishReadyBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<body><button onclick="window.publishClicks++">Publish</button><main><h1>次の画面</h1><div><div role="img"><div><div></div></div><div><div></div></div></div></div><div id="actions"><button>戻る</button><div id="publish" role="button" tabindex="0" aria-label="公開" aria-disabled="true" onclick="window.publishClicks++"><div role="status" aria-label="読み込み中" data-visualcompletion="loading-state">…</div><span>公開</span></div></div></main><script>window.publishClicks=0</script></body>`))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	eval := func(js string) {
		t.Helper()
		if _, err := page.Context(ctx).Eval(js); err != nil {
			t.Fatal(err)
		}
	}
	expectWaiting := func() {
		t.Helper()
		short, stop := context.WithTimeout(ctx, 120*time.Millisecond)
		defer stop()
		if err := waitMarketplacePublishReady(short, page); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Disabled/loading/ambiguous Publish accepted: %v", err)
		}
	}
	expectReady := func() {
		t.Helper()
		short, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		if err := waitMarketplacePublishReady(short, page); err != nil {
			t.Fatal(err)
		}
	}
	// Presence on step two does not mean the action is ready.
	if err := waitMarketplaceItemSecondStep(ctx, page); err != nil {
		t.Fatal(err)
	}
	expectWaiting()
	eval(`()=>document.getElementById('publish').removeAttribute('aria-disabled')`)
	expectWaiting() // The loading spinner is still visible.
	t.Run("reacquires-enabled-replacement-without-clicking", func(t *testing.T) {
		done := make(chan error, 1)
		go func() { done <- waitMarketplacePublishReady(ctx, page) }()
		eval(`()=>{const old=document.getElementById('publish');const ready=old.cloneNode(true);ready.querySelector('[role=status]').remove();ready.className='new-generated-classes';old.replaceWith(ready)}`)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Did not recognize enabled Publish replacement")
		}
	})
	for _, state := range []struct{ name, setup, cleanup string }{
		{"explicit-false", `button.setAttribute('aria-disabled','false')`, `button.removeAttribute('aria-disabled')`},
		{"disabled", `button.setAttribute('aria-disabled','true')`, `button.removeAttribute('aria-disabled')`},
		{"native-disabled", `button.setAttribute('disabled','')`, `button.removeAttribute('disabled')`},
		{"disabled-ancestor", `button.parentElement.setAttribute('aria-disabled','true')`, `button.parentElement.removeAttribute('aria-disabled')`},
		{"inert-ancestor", `button.parentElement.setAttribute('inert','')`, `button.parentElement.removeAttribute('inert')`},
		{"busy", `button.setAttribute('aria-busy','true')`, `button.removeAttribute('aria-busy')`},
		{"hidden", `button.hidden=true`, `button.hidden=false`},
		{"ambiguous-footer", `button.after(button.cloneNode(true))`, `button.nextElementSibling.remove()`},
		{"hidden-stale-spinner", `button.insertAdjacentHTML('beforeend','<div role="status" data-visualcompletion="loading-state" hidden>…</div>')`, `button.querySelector('[role=status]').remove()`},
	} {
		t.Run(state.name, func(t *testing.T) {
			eval(`()=>{const button=document.getElementById('publish');` + state.setup + `}`)
			if state.name == "explicit-false" || state.name == "hidden-stale-spinner" {
				expectReady()
			} else {
				expectWaiting()
			}
			eval(`()=>{const button=document.getElementById('publish');` + state.cleanup + `}`)
		})
	}
	stopped, stop := context.WithCancel(ctx)
	stop()
	if err := waitMarketplacePublishReady(stopped, page); !errors.Is(err, context.Canceled) {
		t.Fatalf("Cancellation ignored: %v", err)
	}
	result, err := page.Context(ctx).Eval(`()=>window.publishClicks`)
	if err != nil || result.Value.Int() != 0 {
		t.Fatalf("Readiness check clicked Publish: %v, %v", result, err)
	}
}
