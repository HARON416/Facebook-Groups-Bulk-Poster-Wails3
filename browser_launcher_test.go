package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
)

func TestUserModeBrowserLaunchConfiguration(t *testing.T) {
	for _, headless := range []bool{false, true} {
		l := newBrowserLauncher(context.Background(), "/chrome", "/postika/account", 43210, headless)
		if l.Get(flags.Bin) != "/chrome" || l.Get(flags.UserDataDir) != "/postika/account" || l.Get(flags.RemoteDebuggingPort) != "43210" {
			t.Fatal("browser executable, account profile or explicit port was lost")
		}
		if l.Has(flags.Headless) != headless {
			t.Fatal("incorrect browser visibility")
		}
		for _, flag := range []flags.Flag{flags.Leakless, flags.NoSandbox, "disable-notifications", "start-maximized"} {
			if !l.Has(flag) {
				t.Fatalf("missing requested launch option %s", flag)
			}
		}
		for _, flag := range []flags.Flag{"enable-automation", "auto-open-devtools-for-tabs", "disable-web-security", "ignore-certificate-errors", "disable-client-side-phishing-detection"} {
			if l.Has(flag) {
				t.Fatalf("unexpected launch option %s", flag)
			}
		}
	}
}

func TestUserModeBrowsersKeepAccountsIsolatedIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -args -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome/Chromium is required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>Account fixture</title><p>Ready</p>"))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	accounts := []*GroupsService{
		{profileDir: t.TempDir(), chromePath: chrome, targetURL: server.URL, headless: true},
		{profileDir: t.TempDir(), chromePath: chrome, targetURL: server.URL, headless: true},
	}
	for _, s := range accounts {
		t.Cleanup(func() {
			launch := s.launcher
			_ = s.ServiceShutdown()
			if launch != nil {
				done := make(chan struct{})
				go func() { launch.Cleanup(); close(done) }()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					t.Error("browser process cleanup did not complete")
				}
			}
		})
		if err := s.OpenJoinedGroups(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if accounts[0].launcher.Get(flags.RemoteDebuggingPort) == accounts[1].launcher.Get(flags.RemoteDebuggingPort) {
		t.Fatal("account browsers share a debugging port")
	}
	if _, err := accounts[0].page.Context(ctx).Eval(`() => {document.cookie = 'account=first; path=/'; localStorage.setItem('account','first');}`); err != nil {
		t.Fatal(err)
	}
	value, err := accounts[1].page.Context(ctx).Eval(`() => document.cookie + (localStorage.getItem('account') || '')`)
	if err != nil {
		t.Fatal(err)
	}
	if value.Value.Str() != "" {
		t.Fatalf("second account inherited data: %s", value.Value.Str())
	}
}
