package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplacePageReadyBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	resourceRequested := make(chan struct{}, 1)
	releaseResource := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseResource) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow-image" {
			resourceRequested <- struct{}{}
			select {
			case <-releaseResource:
			case <-r.Context().Done():
			}
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<body><main><span id="status">Loading</span><input></main>
<script>window.changes=setInterval(()=>document.querySelector('#status').textContent=String(++window.count),50);window.count=0;</script>
<img src="/slow-image"></body>`))
	}))
	defer server.Close()
	defer release()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-resourceRequested:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	done := make(chan error, 1)
	go func() { done <- waitMarketplacePageReady(ctx, page) }()
	select {
	case err := <-done:
		t.Fatalf("Returned before the page loaded: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	release()
	if err := page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatalf("Returned while the DOM was changing: %v", err)
	case <-time.After(1200 * time.Millisecond):
	}
	if _, err := page.Context(ctx).Eval(`() => clearInterval(window.changes)`); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	t.Run("deadline", func(t *testing.T) {
		short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
		defer stop()
		if err := waitMarketplacePageReady(short, page); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Expected deadline error, got %v", err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		stopped, stop := context.WithCancel(ctx)
		stop()
		if err := waitMarketplacePageReady(stopped, page); !errors.Is(err, context.Canceled) {
			t.Fatalf("Expected cancellation, got %v", err)
		}
	})
}
