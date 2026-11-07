package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceHomeDateDisplayBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html lang="en-US"><body><main><input type="file" accept="image/*"><input type="text" role="combobox" aria-autocomplete="none"></main></body></html>`)
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		locale, display string
		invalid, want   bool
	}{
		{"en-US", "11/30/2026", false, true},
		{"en-US", "Nov 30, 2026", false, true},
		{"en-US", "November 30, 2026", false, true},
		{"en-US", "2026-11-30", false, true},
		{"en-GB", "30 Nov 2026", false, true},
		{"en-GB", "30 November 2026", false, true},
		{"ar", "٣٠ نوفمبر ٢٠٢٦", false, true},
		{"en-US", "Nov 29, 2026", false, false},
		{"en-US", "Oct 30, 2026", false, false},
		{"en-US", "Nov 30, 2027", false, false},
		{"en-US", "30/11/2026", false, false},
		{"en-US", "Nov 30, 2026", true, false},
	} {
		t.Run(tc.locale+"/"+tc.display+fmt.Sprint(tc.invalid), func(t *testing.T) {
			_, err := page.Context(ctx).Eval(`(locale,value,invalid)=>{
 document.documentElement.lang=locale;
 const field=document.querySelector('input[role="combobox"]');field.value=value;field.setAttribute('aria-invalid',String(invalid));
}`, tc.locale, tc.display, tc.invalid)
			if err != nil {
				t.Fatal(err)
			}
			result, err := page.Context(ctx).Eval(marketplaceHomeDateConfirmed, "2026-11-30")
			if err != nil || result.Value.Bool() != tc.want {
				t.Fatalf("Date confirmation=%v, want %t, error=%v", result, tc.want, err)
			}
		})
	}
}
