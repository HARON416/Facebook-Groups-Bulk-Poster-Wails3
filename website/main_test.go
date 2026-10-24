package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicPagesAndAssets(t *testing.T) {
	release, err := loadRelease()
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(release)
	for _, path := range []string{"/", "/downloads", "/partials/release", "/static/css/global.css", "/static/css/site.css", "/static/js/main.js", "/static/js/theme-init.js", "/static/css/themes.css", "/static/vendor/htmx.min.js", "/static/screenshots/logs.svg"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
			if response.Code != 200 {
				t.Fatalf("status %d", response.Code)
			}
			if response.Body.Len() == 0 {
				t.Fatal("empty response")
			}
			if path == "/downloads" {
				if strings.Contains(response.Body.String(), "class=\"button button-download\"") {
					t.Fatal("unconfigured release exposes external download links")
				}
				if !strings.Contains(response.Body.String(), "Coming soon") {
					t.Fatal("missing unreleased state")
				}
			}
		})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/missing", nil))
	if response.Code != 404 {
		t.Fatalf("unknown page status %d", response.Code)
	}
}
