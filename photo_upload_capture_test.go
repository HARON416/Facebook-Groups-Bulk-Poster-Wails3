package main

import (
	"context"
	"fmt"
	"github.com/go-rod/rod/lib/launcher"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPhotoUploadCaptureIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	var preflights, posts atomic.Int32
	var origin string
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "x-upload-test")
		if r.Method == "OPTIONS" {
			preflights.Add(1)
			w.WriteHeader(204)
			return
		}
		posts.Add(1)
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Error(err)
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"payload":{"photo_id":"fixture-photo"}}`)
	}))
	defer uploadServer.Close()
	endpoint := uploadServer.URL + "/ajax/react_composer/attachments/photo/upload"
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "c_user", Value: "fresh-user", Path: "/", HttpOnly: true})
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<input type="file" multiple onchange="for(const file of this.files){const f=new FormData();f.append('source','8');f.append('profile_id','fresh-user');f.append('upload_id',crypto.randomUUID());f.append('farr',file);fetch('%s?__user=fresh-user&av=fresh-user&fb_dtsg=fresh-token',{method:'POST',credentials:'include',headers:{'x-upload-test':'fresh-header'},body:f})}">`, endpoint)
	}))
	defer fixture.Close()
	origin = fixture.URL
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true, targetURL: fixture.URL}
	t.Cleanup(func() { service.ServiceShutdown(); time.Sleep(500 * time.Millisecond) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := service.OpenJoinedGroups(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	capture := startPhotoUploadCapture(ctx, service.page, func(raw string) bool { return strings.HasPrefix(raw, endpoint+"?") })
	defer capture.stop()
	dir, paths, err := stagePostImages([]PostUpload{{Name: "first.png", Data: fixturePNG}, {Name: "second.png", Data: fixturePNG}})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	input, err := service.page.Context(ctx).Element("input[type=file]")
	if err != nil {
		t.Fatal(err)
	}
	if err := input.SetFiles(paths); err != nil {
		t.Fatal(err)
	}
	count, err := capture.wait(ctx, 2, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || posts.Load() != 2 || preflights.Load() == 0 {
		t.Fatalf("uploads=%d posts=%d preflights=%d", count, posts.Load(), preflights.Load())
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(capture.requests) != 2 {
		t.Fatal("preflight was captured as an image upload")
	}
	for _, upload := range capture.requests {
		if upload.User != "fresh-user" || upload.Fields.Get("profile_id") != "fresh-user" || upload.Fields.Get("source") != "8" || upload.Fields.Get("upload_id") == "" || upload.Headers.Get("X-Upload-Test") != "fresh-header" || cookieUser(upload.Headers.Get("Cookie")) != "fresh-user" || len(upload.Files) != 1 || len(upload.Response) == 0 {
			t.Fatalf("incomplete upload capture: fields=%v files=%v", upload.Fields, upload.Files)
		}
	}
}
func TestPhotoUploadEndpointAndMultipart(t *testing.T) {
	if !isFacebookPhotoUploadURL("https://upload.facebook.com/ajax/react_composer/attachments/photo/upload?__user=fresh") {
		t.Fatal("expected upload URL")
	}
	for _, raw := range []string{"http://upload.facebook.com/ajax/react_composer/attachments/photo/upload", "https://example.com/ajax/react_composer/attachments/photo/upload", "https://upload.facebook.com/other"} {
		if isFacebookPhotoUploadURL(raw) {
			t.Fatalf("matched unrelated URL %s", raw)
		}
	}
	if _, _, err := uploadMultipart("invalid", "text/plain"); err == nil {
		t.Fatal("accepted invalid multipart capture")
	}
}
