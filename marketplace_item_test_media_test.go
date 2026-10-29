package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceTestMediaValidation(t *testing.T) {
	for _, media := range []MarketplaceTestMedia{
		{}, {Photos: make([]PostUpload, 11)},
		{Video: &PostUpload{Name: "fake.mp4", Data: base64.StdEncoding.EncodeToString([]byte("not a video"))}},
	} {
		if dir, _, _, err := stageMarketplaceTestMedia(media); err == nil {
			os.RemoveAll(dir)
			t.Fatal("Invalid media accepted")
		}
	}
	dir, photos, video, err := stageMarketplaceTestMedia(MarketplaceTestMedia{Photos: []PostUpload{{Name: "../../image.png", Data: fixturePNG}}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if len(photos) != 1 || video != "" || !strings.HasPrefix(photos[0], dir+string(os.PathSeparator)) || filepath.Base(photos[0]) != "image.png" {
		t.Fatal("Incorrect staged media paths")
	}
}

func TestMarketplaceItemMediaBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/login" {
			w.Write([]byte(`<body class="UIPage_LoggedOut"><form id="login_form"><input name="email"><input type="password" name="pass"></form>`))
			return
		}
		w.Write([]byte(`<body class="UIPage_LoggedIn"><input type="file" accept="image/*" id="decoy"><main><div role="form">
		<input style="display:none" type="file" multiple accept="image/*,image/heif,image/heic" onchange="window.photos=Array.from(this.files).map(f=>f.name);let old=document.querySelector('#movie');old.replaceWith(old.cloneNode(true));">
		<input style="display:none" id="movie" type="file" accept="video/*" onchange="window.video=Array.from(this.files).map(f=>f.name)">
		<button onclick="window.published=true">Publish</button></div></main><script>window.photos=[];window.video=[];window.published=false</script>`))
	}))
	defer server.Close()
	s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = s.shutdown(true) })
	data, err := os.ReadFile("frontend/testdata/vehicle-short.webm")
	if err != nil {
		t.Fatal(err)
	}
	video := &PostUpload{Name: "clip.webm", Data: base64.StdEncoding.EncodeToString(data)}
	photo := PostUpload{Name: "one.png", Data: fixturePNG}
	var previousDir string
	var previousPage string
	for _, item := range []struct {
		name                   string
		media                  MarketplaceTestMedia
		photoCount, videoCount int
	}{
		{"photos", MarketplaceTestMedia{Photos: []PostUpload{photo}}, 1, 0},
		{"video", MarketplaceTestMedia{Video: video}, 0, 1},
		{"both", MarketplaceTestMedia{Photos: []PostUpload{photo}, Video: video}, 1, 1},
	} {
		t.Run(item.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.testMarketplaceItemMedia(ctx, server.URL, item.media); err != nil {
				t.Fatal(err)
			}
			page := s.marketplaceTestPage.Context(ctx)
			result, err := page.Eval(`() => ({photos:window.photos.length,video:window.video.length,published:window.published,decoy:document.querySelector('#decoy').files.length})`)
			if err != nil {
				t.Fatal(err)
			}
			if result.Value.Get("photos").Int() != item.photoCount || result.Value.Get("video").Int() != item.videoCount || result.Value.Get("published").Bool() || result.Value.Get("decoy").Int() != 0 {
				t.Fatalf("Incorrect uploaded media: %v", result.Value)
			}
			if previousPage != "" && string(page.TargetID) != previousPage {
				t.Fatal("Test created an extra tab")
			}
			previousPage = string(page.TargetID)
			if previousDir != "" {
				if _, err := os.Stat(previousDir); !os.IsNotExist(err) {
					t.Fatal("Previous staged files retained")
				}
			}
			previousDir = s.marketplaceTestDir
			if _, err := os.Stat(previousDir); err != nil {
				t.Fatal("Staged files removed before Chrome finishes reading")
			}
		})
	}
	t.Run("cancel-login", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := s.testMarketplaceItemMedia(ctx, server.URL+"/login", MarketplaceTestMedia{Photos: []PostUpload{photo}}); err == nil {
			t.Fatal("Login wait ignored cancellation")
		}
		if s.marketplaceTesting {
			t.Fatal("Test remains busy after cancellation")
		}
	})
}
