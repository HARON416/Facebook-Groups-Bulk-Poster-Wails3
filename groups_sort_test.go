package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestGroupsSortMenuIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome/Chromium is required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`<body></body>`)) }))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, targetURL: server.URL + "/groups/joins/", headless: true}
	t.Cleanup(func() { _ = service.ServiceShutdown(); time.Sleep(500 * time.Millisecond) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	control := `<div role="button" tabindex="0" aria-haspopup="menu" aria-expanded="false" onclick="window.clicks++; this.setAttribute('aria-expanded','true')"><span>Trier</span><div role="none" data-visualcompletion="ignore"></div></div>`
	cases := []struct {
		name, class, html string
		want, wantError   bool
		clicks            int
	}{
		{"waiting for control", "UIPage_LoggedIn", `<main role="main"></main>`, false, false, 0},
		{"logged out", "UIPage_LoggedOut", control, false, false, 0},
		{"verification", "UIPage_LoggedIn", `<form action="/checkpoint/"></form>` + control, false, false, 0},
		{"ambiguous controls", "UIPage_LoggedIn", control + control, false, true, 0},
		{"translated control", "UIPage_LoggedIn", `<main role="main">` + control + `</main>`, true, false, 1},
		{"hidden decoy", "UIPage_LoggedIn", `<div hidden>` + control + `</div>` + control, true, false, 1},
		{"scoped to main", "UIPage_LoggedIn", control + `<main role="main">` + control + `</main>`, true, false, 1},
		{"already open", "UIPage_LoggedIn", `<div role="button" aria-haspopup="menu" aria-expanded="true"><span>Ordenar</span><div role="none" data-visualcompletion="ignore"></div></div>`, true, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service.sortMenuOpened = false
			_, err := service.page.Context(ctx).Eval(`(className, html) => { document.body.className=className; document.body.innerHTML=html; window.clicks=0; }`, tc.class, tc.html)
			if err != nil {
				t.Fatal(err)
			}
			opened, err := service.OpenGroupsSortMenu(ctx)
			if opened != tc.want || (err != nil) != tc.wantError {
				t.Fatalf("opened=%v error=%v", opened, err)
			}
			count, err := service.page.Context(ctx).Eval(`() => window.clicks`)
			if err != nil || count.Value.Int() != tc.clicks {
				t.Fatalf("clicks=%v error=%v", count, err)
			}
			if tc.want {
				_, _ = service.page.Context(ctx).Eval(`() => document.querySelectorAll('[aria-expanded]').forEach(el => el.setAttribute('aria-expanded','false'))`)
				opened, err = service.OpenGroupsSortMenu(ctx)
				count, _ = service.page.Context(ctx).Eval(`() => window.clicks`)
				if !opened || err != nil || count.Value.Int() != tc.clicks {
					t.Fatal("poll reopened the menu")
				}
			}
		})
	}

	checked := `<div role="menuitemradio" aria-checked="true" onclick="window.chosen='checked'">Actuel</div>`
	first := `<div role="menuitemradio" aria-checked="false" onclick="window.chosen='first'; window.optionClicks++; this.setAttribute('aria-checked','true')">Autre</div>`
	second := `<div role="menuitemradio" aria-checked="false" onclick="window.chosen='second'; window.optionClicks++">التالي</div>`
	for _, tc := range []struct {
		name, html, want string
		wantError        bool
	}{
		{"skip selected first", `<div role="menu">` + checked + first + second + `</div>`, "first", false},
		{"selected later", `<div role="menu">` + first + checked + second + `</div>`, "first", false},
		{"skip hidden and disabled", `<div role="menu">` + checked + `<div hidden>` + first + `</div><div role="menuitemradio" aria-checked="false" aria-disabled="true" onclick="window.chosen='disabled'">Disabled</div>` + second + `</div>`, "second", false},
		{"wait for menu", "", "", false},
		{"no unchecked option", `<div role="menu">` + checked + `</div>`, "", false},
		{"ambiguous menus", `<div role="menu">` + checked + first + `</div><div role="menu">` + checked + second + `</div>`, "", true},
		{"login form blocks selection", `<form id="login_form"></form><div role="menu">` + checked + first + `</div>`, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service.sortMenuOpened = true
			service.sortOptionSelected = false
			if _, err := service.page.Context(ctx).Eval(`html => {document.body.className='UIPage_LoggedIn'; document.body.innerHTML=html; window.chosen=''; window.optionClicks=0}`, tc.html); err != nil {
				t.Fatal(err)
			}
			done, err := service.SelectUnselectedGroupsSortOption(ctx)
			if done != (tc.want != "") || (err != nil) != tc.wantError {
				t.Fatalf("selected=%v error=%v", done, err)
			}
			chosen, err := service.page.Context(ctx).Eval(`() => window.chosen`)
			if err != nil || chosen.Value.Str() != tc.want {
				t.Fatalf("chosen=%v want=%q error=%v", chosen, tc.want, err)
			}
			if done {
				if _, err := service.SelectUnselectedGroupsSortOption(ctx); err != nil {
					t.Fatal(err)
				}
				clicks, _ := service.page.Context(ctx).Eval(`() => window.optionClicks`)
				if clicks.Value.Int() != 1 {
					t.Fatal("poll selected another option")
				}
			}
		})
	}
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	if service.sortMenuOpened || service.sortOptionSelected {
		t.Fatal("refresh did not reset menu state")
	}
	if err := service.page.Context(ctx).Navigate(server.URL + "/other"); err != nil {
		t.Fatal(err)
	}
	_, _ = service.page.Context(ctx).Eval(`html => { document.body.className='UIPage_LoggedIn'; document.body.innerHTML=html; }`, control)
	if opened, err := service.OpenGroupsSortMenu(ctx); opened || err != nil {
		t.Fatalf("wrong page: opened=%v error=%v", opened, err)
	}
}
