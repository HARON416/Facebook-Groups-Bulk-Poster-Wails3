package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestMarketplaceLocationClosedService(t *testing.T) {
	s := &GroupsService{closed: true}
	if err := s.openMarketplaceLocation(context.Background(), marketplaceItemURL); err == nil {
		t.Fatal("closed service opened a browser")
	}
	if marketplaceItemURL != "https://web.facebook.com/marketplace/create/item" {
		t.Fatal("incorrect Marketplace destination")
	}
}

func TestMarketplaceLocationBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/login/") {
			http.SetCookie(w, &http.Cookie{Name: "fixture-account", Value: strings.TrimPrefix(r.URL.Path, "/login/"), Path: "/", HttpOnly: true})
		}
		_, _ = w.Write([]byte("<title>Account fixture</title><body>Marketplace fixture</body>"))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var browserIDs []string
	for _, account := range []string{"first", "second"} {
		s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
		t.Cleanup(func() { _ = s.ServiceShutdown(); time.Sleep(200 * time.Millisecond) })
		if err := s.ensureBrowserPage(ctx); err != nil {
			t.Fatal(err)
		}
		original := s.page
		if err := original.Context(ctx).Navigate(server.URL + "/login/" + account); err != nil {
			t.Fatal(err)
		}
		if err := original.Context(ctx).WaitLoad(); err != nil {
			t.Fatal(err)
		}
		target := server.URL + "/marketplace/create/item"
		if err := s.openMarketplaceLocation(ctx, target); err != nil {
			t.Fatal(err)
		}
		tab := s.marketplaceLocationPage
		if err := tab.Context(ctx).WaitLoad(); err != nil {
			t.Fatal(err)
		}
		info, err := original.Context(ctx).Info()
		if err != nil || info.URL != server.URL+"/login/"+account || s.page != original {
			t.Fatal("opening Marketplace navigated the existing account tab")
		}
		cookies, err := tab.Context(ctx).Cookies([]string{target})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, cookie := range cookies {
			if cookie.Name == "fixture-account" {
				found = cookie.Value == account
			}
		}
		if !found {
			t.Fatal("Marketplace did not use the selected account's cookies")
		}
		browserIDs = append(browserIDs, string(tab.TargetID))
		if err := s.openMarketplaceLocation(ctx, target); err != nil {
			t.Fatal(err)
		}
		if s.marketplaceLocationPage != tab {
			t.Fatal("opening Marketplace again created an extra tab")
		}
		if err := tab.Close(); err != nil {
			t.Fatal(err)
		}
		if err := s.openMarketplaceLocation(ctx, target); err != nil {
			t.Fatal(err)
		}
		if s.marketplaceLocationPage.TargetID == tab.TargetID {
			t.Fatal("closed Marketplace tab was not replaced")
		}
	}
	if browserIDs[0] == browserIDs[1] {
		t.Fatal("accounts shared a Marketplace tab")
	}
}

func TestMarketplaceLoginRecoveryIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	var itemVisits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/signed-in" {
			http.SetCookie(w, &http.Cookie{Name: "marketplace-auth", Value: "yes", Path: "/", HttpOnly: true})
			_, _ = w.Write([]byte(`<body class="UIPage_LoggedIn">Home</body>`))
			return
		}
		if r.URL.Path == "/marketplace/create/item" {
			itemVisits.Add(1)
		}
		if cookie, _ := r.Cookie("marketplace-auth"); cookie != nil && cookie.Value == "yes" {
			_, _ = w.Write([]byte(marketplaceDetailsPageFixture))
			return
		}
		_, _ = w.Write([]byte(`<body class="UIPage_LoggedIn"><div data-pagelet="CometSettingsDropdownTrigger"></div><div role="dialog" aria-modal="true"><form id="login_popup_cta_form" action="/login/device-based/regular/login/"><input name="email"><input name="pass" type="password"><button type="button" id="sign-in" onclick="location.href='/signed-in'">Ingia</button></form></div></body>`))
	}))
	defer server.Close()
	s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = s.ServiceShutdown(); time.Sleep(200 * time.Millisecond) })
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err := s.ensureBrowserPage(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.page.Context(ctx).Eval(`() => document.body.className='UIPage_LoggedIn'`); err != nil {
		t.Fatal(err)
	}
	target := server.URL + "/marketplace/create/item"
	caller, endCall := context.WithCancel(ctx)
	if err := s.openMarketplaceLocation(caller, target); err != nil {
		t.Fatal(err)
	}
	endCall() // Cancelling the opening binding must not cancel the background wait.
	waitStatus := func(want string) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if state := s.marketplaceLocationLoad.snapshot().State; state == want {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatalf("Marketplace status=%+v; want %s", s.marketplaceLocationLoad.snapshot(), want)
	}
	waitStatus("waiting_login")
	tab := s.marketplaceLocationPage
	// Removing the popup, with no positive login evidence, must not resume.
	if _, err := tab.Context(ctx).Eval(`() => {document.body.className='';document.body.innerHTML='<p>Loading</p>';}`); err != nil {
		t.Fatal(err)
	}
	time.Sleep(650 * time.Millisecond)
	if s.marketplaceLocationLoad.snapshot().State != "waiting_login" {
		t.Fatal("popup disappearance was treated as login")
	}
	if _, err := tab.Context(ctx).Eval(`() => {document.body.className='UIPage_LoggedIn';document.body.innerHTML='<form action="/checkpoint/"></form>';}`); err != nil {
		t.Fatal(err)
	}
	time.Sleep(650 * time.Millisecond)
	if s.marketplaceLocationLoad.snapshot().State != "waiting_login" {
		t.Fatal("verification was treated as login")
	}
	if err := tab.Context(ctx).Navigate(server.URL + "/signed-in"); err != nil {
		t.Fatal(err)
	}
	waitStatus("ready")
	info, err := tab.Context(ctx).Info()
	if err != nil || info.URL != target || itemVisits.Load() < 2 {
		t.Fatal("login did not resume the Marketplace item page automatically")
	}
	if s.marketplaceLocationPage != tab || s.page == tab {
		t.Fatal("login recovery changed the account's original tab")
	}
	// A fresh login wait is cancellable and keeps the browser available.
	if _, err := tab.Context(ctx).Eval(`() => {document.body.innerHTML='<form id="login_popup_cta_form"></form>';}`); err != nil {
		t.Fatal(err)
	}
	// Navigate to a guest fixture by clearing only its fixture cookie.
	if err := (proto.NetworkDeleteCookies{Name: "marketplace-auth", URL: server.URL}).Call(tab.Context(ctx)); err != nil {
		t.Fatal(err)
	}
	if err := s.openMarketplaceLocation(ctx, target); err != nil {
		t.Fatal(err)
	}
	waitStatus("waiting_login")
	load := s.marketplaceLocationLoad
	load.cancel()
	waitStatus("cancelled")
	if _, err := tab.Context(ctx).Info(); err != nil {
		t.Fatal("cancelling the login wait closed Chrome")
	}
	if err := s.openMarketplaceLocation(ctx, target); err != nil {
		t.Fatal(err)
	}
	waitStatus("waiting_login")
	if err := tab.Close(); err != nil {
		t.Fatal(err)
	}
	waitStatus("browser_closed")
	if err := s.openMarketplaceLocation(ctx, target); err != nil {
		t.Fatal(err)
	}
	waitStatus("waiting_login")
	load = s.marketplaceLocationLoad
	if err := s.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-load.done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown left login watcher running")
	}
}

func TestMarketplaceDirectOpenIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(marketplaceDetailsPageFixture))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = s.ServiceShutdown(); time.Sleep(200 * time.Millisecond) })
	target := server.URL + "/marketplace/create/item"
	for i := 0; i < 2; i++ {
		if err := s.openMarketplaceLocation(ctx, target); err != nil {
			t.Fatal(err)
		}
		if s.page != nil {
			t.Fatal("Marketplace opening created an unused groups tab")
		}
		if err := s.marketplaceLocationPage.Context(ctx).WaitLoad(); err != nil {
			t.Fatal(err)
		}
		pages, err := s.browser.Context(ctx).Pages()
		if err != nil || len(pages) != 1 {
			t.Fatalf("wanted only the Marketplace tab, got %d pages: %v", len(pages), err)
		}
		info, err := pages[0].Context(ctx).Info()
		if err != nil || info.URL != target {
			t.Fatalf("browser opened %v instead of Marketplace: %v", info, err)
		}
	}
	// Loading groups later still gets its separate tab without replacing Marketplace.
	marketplace := s.marketplaceLocationPage
	if err := s.ensureBrowserPage(ctx); err != nil {
		t.Fatal(err)
	}
	if s.page == marketplace || s.marketplaceLocationPage != marketplace {
		t.Fatal("groups page replaced Marketplace")
	}
}
