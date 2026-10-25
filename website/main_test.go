package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFeedRedirects(t *testing.T) {
	handler := newHandler(Release{})
	for _, feed := range []string{"stable.json", "stable-deb.json", "stable-rpm.json", "stable-arch.json", "stable-binary.json"} {
		for _, method := range []string{"GET", "HEAD"} {
			t.Run(feed+method, func(t *testing.T) {
				query := "?version=0.1.1&platform=linux&arch=amd64&channel=stable"
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(method, "/updates/postika/"+feed+query, nil))
				if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "https://postika-desktop-application.b-cdn.net/updates/"+feed+query {
					t.Fatalf("unexpected redirect: %d %s", response.Code, response.Header().Get("Location"))
				}
				if response.Header().Get("Cache-Control") != "no-cache" {
					t.Fatal("stable feed redirect must revalidate")
				}
			})
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/updates/postika/unknown.json", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown feed status %d", response.Code)
	}
}

func TestPublicPagesAndAssets(t *testing.T) {
	t.Setenv("RELEASE_FILE", "")
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
				body := response.Body.String()
				if strings.Count(body, `class="button button-download"`) != 8 {
					t.Fatal("default release must show all eight download buttons")
				}
				for _, download := range release.Downloads {
					if !strings.Contains(body, `href="`+download.URL+`"`) {
						t.Fatalf("missing default download: %s", download.Name)
					}
				}
				for _, placeholder := range []string{"Coming soon", "being prepared", "still being verified", "Release notes will", `class="release-history"`} {
					if strings.Contains(body, placeholder) {
						t.Fatalf("unexpected release placeholder: %s", placeholder)
					}
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

func TestDownloadGroups(t *testing.T) {
	// Release metadata may interleave platforms. Keep each format with its OS.
	release := Release{Downloads: []Download{
		{OS: "linux", Name: "AppImage", URL: "https://example.com/postika.AppImage"},
		{OS: "windows", Name: "Windows installer", URL: "https://example.com/postika.exe"},
		{OS: "linux", Name: "Debian / Ubuntu"},
	}}
	response := httptest.NewRecorder()
	newHandler(release).ServeHTTP(response, httptest.NewRequest("GET", "/downloads", nil))
	body := response.Body.String()
	if strings.Count(body, `id="platform-linux"`) != 1 || strings.Contains(body, `id="platform-macos"`) {
		t.Fatal("expected one Linux group and no empty macOS group")
	}
	for _, name := range []string{"AppImage", "Windows installer", "Debian / Ubuntu"} {
		if strings.Count(body, "<h3>"+name+"</h3>") != 1 {
			t.Fatalf("format %q missing or repeated", name)
		}
	}
	if !strings.Contains(body, `href="https://example.com/postika.exe" aria-label="Download Windows installer"`) || !strings.Contains(body, "Coming soon") {
		t.Fatal("expected configured download links and an unavailable format")
	}
}

func TestReleaseOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.json")
	if err := os.WriteFile(path, []byte(`{"version":"0.2.0","downloads":[{"os":"windows","name":"Windows installer","url":"https://example.com/new.exe"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELEASE_FILE", path)
	release, err := loadRelease()
	if err != nil || release.Version != "0.2.0" || len(release.Downloads) != 1 {
		t.Fatalf("override failed: %+v, %v", release, err)
	}
	if err := os.WriteFile(path, []byte(`{"downloads":[{"url":"http://example.com/unsafe.exe"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRelease(); err == nil {
		t.Fatal("HTTP override URL accepted")
	}
	t.Setenv("RELEASE_FILE", path+".missing")
	if _, err := loadRelease(); err == nil {
		t.Fatal("missing explicit override silently ignored")
	}
}
