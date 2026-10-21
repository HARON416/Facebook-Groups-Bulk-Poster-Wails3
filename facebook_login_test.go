package main

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestLoginStateWithoutBrowser(t *testing.T) {
	state, err := (&GroupsService{}).GetLoginState(context.Background())
	if err != nil || state != "browser_closed" {
		t.Fatalf("state=%q, error=%v", state, err)
	}
}

func TestFacebookLoginSelectorsIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with go test . -args -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome/Chromium is required")
	}
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, targetURL: "about:blank", headless: true}
	t.Cleanup(func() {
		_ = service.ServiceShutdown()
		time.Sleep(500 * time.Millisecond)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, class, html, want string
	}{
		{"pasted body marker", "UIPage_LoggedOut Locale_sw_KE", "<p>Ingia</p>", "logged_out"},
		{"pasted login form", "", `<form id="login_form"><input name="email"><input name="pass" type="password"></form>`, "logged_out"},
		{"pasted test id", "", `<div data-testid="royal_login_form">Connexion</div>`, "logged_out"},
		{"login fields in same form", "", `<form><input name="email"><input name="pass" type="password"></form>`, "logged_out"},
		{"signed in body", "UIPage_LoggedIn Locale_ar_AR", "<p>مرحبا</p>", "logged_in"},
		{"signed in account control", "", `<div data-pagelet="CometSettingsDropdownTrigger"></div>`, "logged_in"},
		{"signed in shell", "", `<header role="banner"><div data-pagelet="CometNotificationsDropdown"></div><div data-pagelet="CometProfileDropdown"></div></header>`, "logged_in"},
		{"logout control", "", `<form action="/logout.php"></form>`, "logged_in"},
		{"verification takes priority", "UIPage_LoggedIn", `<form action="/checkpoint/"></form>`, "login_required"},
		{"logged out takes priority", "UIPage_LoggedOut", `<div data-pagelet="CometSettingsDropdownTrigger"></div>`, "logged_out"},
		{"public page is not signed in", "", `<nav role="navigation"></nav><main role="main"><a href="/groups/123">Group</a></main>`, "checking"},
		{"login words are not evidence", "", "<p>Log in · Logged in · Sign out</p>", "checking"},
		{"unrelated password field", "", `<input type="password" name="pass">`, "checking"},
		{"blank loading page", "", "", "checking"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.page.Context(ctx).Eval(`(className, html) => {
                document.body.className = className;
                document.body.innerHTML = html;
            }`, tc.class, tc.html)
			if err != nil {
				t.Fatal(err)
			}
			state, err := service.GetLoginState(ctx)
			if err != nil || state != tc.want {
				t.Fatalf("state=%q, want=%q, error=%v", state, tc.want, err)
			}
		})
	}
	// A poll sees the login transition without a second Load groups click.
	if _, err := service.page.Context(ctx).Eval(`() => { document.body.className = 'UIPage_LoggedIn'; document.body.innerHTML = ''; }`); err != nil {
		t.Fatal(err)
	}
	state, err := service.GetLoginState(ctx)
	if err != nil || state != "logged_in" {
		t.Fatalf("login transition: state=%q, error=%v", state, err)
	}
	if err := service.page.Context(ctx).Close(); err != nil {
		t.Fatal(err)
	}
	state, err = service.GetLoginState(ctx)
	if err != nil || state != "browser_closed" {
		t.Fatalf("closed tab: state=%q, error=%v", state, err)
	}
}
