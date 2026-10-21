package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

var browserIntegration = flag.Bool("browser-integration", false, "run local headless Chrome integration checks")

func TestOperationsHaveNoDeadlineAndCancelOnShutdown(t *testing.T) {
	service := &GroupsService{}
	ctx, cancel := service.operationContext(context.Background())
	defer cancel()
	if _, set := ctx.Deadline(); set {
		t.Fatal("operation unexpectedly has a deadline")
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the pending operation")
	}
}

func TestGroupsServiceShutdownRejectsLaunch(t *testing.T) {
	service := &GroupsService{}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	if err := service.OpenJoinedGroups(context.Background()); err == nil {
		t.Fatal("expected launch to be rejected after shutdown")
	}
}

func TestGroupsServiceLaunchFailure(t *testing.T) {
	service := &GroupsService{
		profileDir: t.TempDir(),
		chromePath: filepath.Join(t.TempDir(), "missing-chrome"),
	}
	err := service.OpenJoinedGroups(context.Background())
	if err == nil || !strings.Contains(err.Error(), "launch Chrome") {
		t.Fatalf("expected actionable launch error, got %v", err)
	}
	if service.browser != nil {
		t.Fatal("failed launch must not retain a browser")
	}
}

func TestGroupsBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with go test . -args -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome/Chromium is required for the browser integration check")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<title>Local groups fixture</title><p>Groups</p>"))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, targetURL: server.URL, headless: true}
	t.Cleanup(func() {
		_ = service.ServiceShutdown()
		time.Sleep(500 * time.Millisecond)
	})
	ctx, cancel := context.WithCancel(context.Background())
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	checkCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	if _, err := service.page.Context(checkCtx).Info(); err != nil {
		t.Fatalf("browser must outlive the request context: %v", err)
	}
	if err := service.page.Context(checkCtx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.page.Context(checkCtx).Eval(`() => { document.cookie = 'postika_fixture=retained; max-age=3600; path=/'; }`); err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	errs := make(chan error, 3)
	for range 3 {
		workers.Add(1)
		go func() { defer workers.Done(); errs <- service.OpenJoinedGroups(checkCtx) }()
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	pages, err := service.browser.Context(checkCtx).Pages()
	if err != nil || len(pages) != 1 {
		t.Fatalf("repeated calls should reuse one tab: pages=%d, error=%v", len(pages), err)
	}
	if err := service.page.Context(checkCtx).Close(); err != nil {
		t.Fatal(err)
	}
	if err := service.OpenJoinedGroups(checkCtx); err != nil {
		t.Fatalf("reopen a manually closed tab: %v", err)
	}
	service.mu.Lock()
	service.closeBrowser()
	service.mu.Unlock()
	// Chrome finishes flushing profile data shortly after acknowledging Close.
	time.Sleep(500 * time.Millisecond)
	if err := service.OpenJoinedGroups(checkCtx); err != nil {
		t.Fatalf("reopen browser with persistent profile: %v", err)
	}
	cookies, err := service.page.Context(checkCtx).Cookies([]string{server.URL})
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range cookies {
		if cookie.Name == "postika_fixture" && cookie.Value == "retained" {
			return
		}
	}
	t.Fatal("persistent profile did not retain the fixture cookie")
}
