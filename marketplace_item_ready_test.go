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

func TestMarketplaceItemNextReadyBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<body class="UIPage_LoggedIn"><main><h1>Fixture</h1><div><div role="img"><div><div></div></div><div><div></div></div></div></div><div id="fixture-actions"><div id="next" role="button" aria-label="Next" aria-disabled="true" tabindex="0" onclick="window.nextClicks++">Next</div></div></main><button aria-label="Publish" onclick="window.publishClicks++">Publish</button><script>window.nextClicks=0;window.publishClicks=0</script>`))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	eval := func(js string) {
		t.Helper()
		if _, err := page.Context(ctx).Eval(js); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("waits-through-disabled-and-replacement", func(t *testing.T) {
		done := make(chan error, 1)
		go func() { done <- waitMarketplaceItemReady(ctx, page) }()
		select {
		case err := <-done:
			t.Fatalf("Disabled button accepted: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		eval(`() => {const previous=document.querySelector('#next');const fresh=previous.cloneNode(true);fresh.className='new-facebook-classes';fresh.removeAttribute('aria-disabled');previous.replaceWith(fresh)}`)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Did not recognize enabled replacement")
		}
	})
	t.Run("explicit-false-is-enabled", func(t *testing.T) {
		eval(`() => document.querySelector('#next').setAttribute('aria-disabled','false')`)
		if err := waitMarketplaceItemReady(ctx, page); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("clicks-once-after-disabled-replacement", func(t *testing.T) {
		eval(`() => {
          const button=document.querySelector('#next');
          button.setAttribute('aria-disabled','true');
          button.setAttribute('onclick', "window.nextClicks++; this.setAttribute('aria-label','Continue'); const next=document.createElement('button');next.setAttribute('aria-label','Next');next.textContent='Next';next.onclick=()=>window.nextClicks+=100;this.after(next)");
        }`)
		done := make(chan error, 1)
		go func() { done <- clickMarketplaceItemNext(ctx, page) }()
		select {
		case err := <-done:
			t.Fatalf("Clicked a disabled control: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		eval(`() => {const old=document.querySelector('#next');const fresh=old.cloneNode(true);fresh.removeAttribute('aria-disabled');fresh.className='new-ready-classes';old.replaceWith(fresh)}`)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Enabled Next was not clicked")
		}
		result, err := page.Context(ctx).Eval(`() => ({next:window.nextClicks,publish:window.publishClicks})`)
		if err != nil || result.Value.Get("next").Int() != 1 || result.Value.Get("publish").Int() != 0 {
			t.Fatalf("Expected exactly one Next click and no Publish click: %v, %v", result, err)
		}
		eval(`() => {document.querySelector('#fixture-actions').innerHTML='<button id="next" aria-label="Next" onclick="window.nextClicks++">Next</button>';window.nextClicks=0}`)
		if err := clickMarketplaceItemNext(ctx, page); err != nil {
			t.Fatal(err)
		}
		result, err = page.Context(ctx).Eval(`() => window.nextClicks===1 && window.publishClicks===0`)
		if err != nil || !result.Value.Bool() {
			t.Fatalf("Native Next button was not clicked exactly once: %v, %v", result, err)
		}
		eval(`() => {window.nextClicks=0}`)
	})
	for _, fixture := range []struct{ name, setup string }{
		{"aria-disabled", `button.setAttribute('aria-disabled','true')`},
		{"hidden", `button.style.display='none'`},
		{"invisible", `button.style.visibility='hidden'`},
		{"native-disabled", `button.setAttribute('disabled','')`},
		{"disabled-parent", `button.parentElement.setAttribute('aria-disabled','true')`},
		{"inert-parent", `button.parentElement.setAttribute('inert','')`},
		{"ambiguous", `button.parentElement.append(button.cloneNode(true))`},
		{"missing", `button.remove()`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			eval(`() => {document.querySelector('#fixture-actions').innerHTML='<div id="next" role="button" aria-label="Next" tabindex="0" onclick="window.nextClicks++">Next</div>';document.querySelector('#fixture-actions').removeAttribute('aria-disabled');document.querySelector('#fixture-actions').removeAttribute('inert');const button=document.querySelector('#next');` + fixture.setup + `}`)
			short, stop := context.WithTimeout(ctx, 150*time.Millisecond)
			defer stop()
			err := waitMarketplaceItemReady(short, page)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Expected a bounded wait, got %v", err)
			}
		})
	}
	t.Run("cancelled", func(t *testing.T) {
		stopped, stop := context.WithCancel(ctx)
		stop()
		if err := waitMarketplaceItemReady(stopped, page); !errors.Is(err, context.Canceled) {
			t.Fatalf("Cancellation ignored: %v", err)
		}
	})
	result, err := page.Context(ctx).Eval(`() => window.nextClicks===0 && window.publishClicks===0`)
	if err != nil || !result.Value.Bool() {
		t.Fatalf("Readiness check clicked Next or Publish: %v, %v", result, err)
	}
}

func TestMarketplaceItemSecondStepBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<body><header><h1>List in more places</h1><div role="img" aria-label="Progress, currently on step 2 of 2">Outside step</div><button aria-label="Publish" onclick="window.published++">Publish</button></header><div role="main"><h1 id="heading">Item for sale</h1><div><div id="progress" role="img" aria-label="Progress, currently on step 1 of 2"><div><div></div></div><div><div></div></div></div></div><div><button>Previous</button><div id="publish" role="button" aria-label="Publish" hidden onclick="window.published++">Publish</div></div><div id="groups"></div></div><script>window.published=0</script></body>`))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
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
		short, stop := context.WithTimeout(ctx, 150*time.Millisecond)
		defer stop()
		if err := waitMarketplaceItemSecondStep(short, page); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Unconfirmed step accepted: %v", err)
		}
	}
	t.Run("ignores-outside-markers", func(t *testing.T) { expectWaiting() })
	t.Run("needs-heading-and-progress-and-publish", func(t *testing.T) {
		eval(`() => {document.querySelector('#heading').textContent='List in more places'}`)
		expectWaiting()
		eval(`() => {document.querySelector('#progress').setAttribute('aria-label','Progress, currently on step 2 of 2')}`)
		expectWaiting()
	})
	t.Run("waits-for-delayed-render-without-url-change", func(t *testing.T) {
		done := make(chan error, 1)
		go func() { done <- waitMarketplaceItemSecondStep(ctx, page) }()
		select {
		case err := <-done:
			t.Fatalf("Hidden Publish accepted: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		eval(`() => {const old=document.querySelector('#publish');const ready=old.cloneNode(true);ready.hidden=false;old.replaceWith(ready)}`)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Did not recognize step 2")
		}
	})
	t.Run("empty-groups-and-hide-friends-warning-still-confirm-step", func(t *testing.T) {
		eval(`() => {document.querySelector('#groups').textContent="When you hide listings from friends, you can't crosspost to groups."}`)
		if err := waitMarketplaceItemSecondStep(ctx, page); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("hidden-stale-heading-does-not-confirm-step", func(t *testing.T) {
		eval(`() => {document.querySelector('#heading').hidden=true}`)
		expectWaiting()
		eval(`() => {document.querySelector('#heading').hidden=false}`)
	})
	t.Run("cancelled", func(t *testing.T) {
		stopped, stop := context.WithCancel(ctx)
		stop()
		if err := waitMarketplaceItemSecondStep(stopped, page); !errors.Is(err, context.Canceled) {
			t.Fatalf("Cancellation ignored: %v", err)
		}
	})
	result, err := page.Context(ctx).Eval(`() => window.published`)
	if err != nil || result.Value.Int() != 0 {
		t.Fatalf("Step detection clicked Publish: %v, %v", result, err)
	}
}
