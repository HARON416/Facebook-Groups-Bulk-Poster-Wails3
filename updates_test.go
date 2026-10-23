package main

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"
)

type testUpdateEngine struct {
	check    func(context.Context) (*updater.Release, error)
	download func(context.Context) error
	path     string
}

func (e testUpdateEngine) Check(ctx context.Context) (*updater.Release, error) { return e.check(ctx) }
func (e testUpdateEngine) DownloadAndInstall(ctx context.Context) error {
	if e.download == nil {
		return errors.New("unexpected download")
	}
	return e.download(ctx)
}
func (e testUpdateEngine) DownloadedPath() string        { return e.path }
func (e testUpdateEngine) Restart(context.Context) error { panic("unexpected restart") }

func TestUpdatePolling(t *testing.T) {
	if updateInterval != time.Minute {
		t.Fatalf("poll interval = %s", updateInterval)
	}
	checks := make(chan struct{}, 10)
	s := &UpdateService{startupDelay: time.Millisecond, checkInterval: time.Millisecond,
		engine: testUpdateEngine{check: func(context.Context) (*updater.Release, error) {
			select {
			case checks <- struct{}{}:
			default:
			}
			return nil, nil
		}},
		status: UpdateStatus{State: "idle", CurrentVersion: "0.1.0"},
	}
	if err := s.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-checks:
		case <-time.After(time.Second):
			t.Fatal("startup/periodic check missing")
		}
	}
	if err := s.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	if s.GetUpdateStatus().State != "up-to-date" {
		t.Fatal(s.GetUpdateStatus())
	}
}
func TestUpdateAutomaticDownload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "postika.exe")
	if err := os.WriteFile(path, []byte("MZ test update"), 0700); err != nil {
		t.Fatal(err)
	}
	started, finish := make(chan struct{}), make(chan struct{})
	s := &UpdateService{ctx: context.Background(), target: updateTarget{Kind: "windows"}, engine: testUpdateEngine{
		path: path, check: func(context.Context) (*updater.Release, error) { return &updater.Release{Version: "0.2.0"}, nil },
		download: func(context.Context) error { close(started); <-finish; return nil },
	}}
	if _, err := s.CheckForUpdates(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-started
	if s.GetUpdateStatus().State != "downloading" {
		t.Fatal(s.GetUpdateStatus())
	}
	if _, err := s.CheckForUpdates(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.DownloadUpdate(); err == nil {
		t.Fatal("duplicate download allowed")
	}
	close(finish)
	s.op.Lock()
	s.op.Unlock()
	if s.GetUpdateStatus().State != "ready" {
		t.Fatal(s.GetUpdateStatus())
	}
	if _, err := s.CheckForUpdates(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.GetUpdateStatus().State != "ready" {
		t.Fatal("poll discarded staged update")
	}
}
func TestUpdateCheckFailureKeepsRelease(t *testing.T) {
	s := &UpdateService{status: UpdateStatus{State: "available", Version: "0.2.0"}, engine: testUpdateEngine{
		check: func(context.Context) (*updater.Release, error) { return nil, errors.New("offline") },
	}}
	status, err := s.CheckForUpdates(context.Background())
	if err != nil || status.State != "available" || status.Version != "0.2.0" || status.Error == "" {
		t.Fatalf("%+v, %v", status, err)
	}
}
func TestUpdateDownloadCancellation(t *testing.T) {
	started := make(chan struct{})
	s := &UpdateService{startupDelay: time.Millisecond, checkInterval: time.Minute, engine: testUpdateEngine{
		check:    func(context.Context) (*updater.Release, error) { return &updater.Release{Version: "0.2.0"}, nil },
		download: func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() },
	}}
	_ = s.ServiceStartup(context.Background(), application.ServiceOptions{})
	<-started
	done := make(chan struct{})
	go func() { _ = s.ServiceShutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel download")
	}
	if s.GetUpdateStatus().State != "available" {
		t.Fatal("failed download offered restart")
	}
}
func TestUpdateRestartGateAndRecovery(t *testing.T) {
	profiles := &ProfileService{}
	var applies, quits atomic.Int32
	s := &UpdateService{profiles: profiles, target: updateTarget{Kind: "deb"}, status: UpdateStatus{State: "ready", Version: "0.2.0"},
		engine: testUpdateEngine{}, quit: func() { quits.Add(1) },
		apply: func(context.Context, string, string, *updater.Verification) error {
			applies.Add(1)
			return errors.New("authorization cancelled")
		},
	}
	profiles.updateGate.RLock()
	if err := s.RestartToUpdate(context.Background()); err == nil {
		t.Fatal("restart permitted during queue")
	}
	profiles.updateGate.RUnlock()
	if applies.Load() != 0 {
		t.Fatal("installer started during queue")
	}
	if err := s.RestartToUpdate(context.Background()); err == nil {
		t.Fatal("installation failure ignored")
	}
	if quits.Load() != 0 || s.GetUpdateStatus().State != "ready" {
		t.Fatal("failed install closed app or lost staged update")
	}
	if !profiles.updateGate.TryRLock() {
		t.Fatal("failed installation left posting blocked")
	}
	profiles.updateGate.RUnlock()
	s.apply = func(context.Context, string, string, *updater.Verification) error { return nil }
	if err := s.RestartToUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if quits.Load() != 1 {
		t.Fatal("successful helper handoff did not quit")
	}
	if profiles.updateGate.TryRLock() {
		profiles.updateGate.RUnlock()
		t.Fatal("new posting allowed during restart")
	}
	profiles.updateGate.Unlock()
}

func TestUpdateNativeShutdownDoesNotDeadlock(t *testing.T) {
	s := &UpdateService{profiles: &ProfileService{}, target: updateTarget{Kind: "windows"}, status: UpdateStatus{State: "ready"}, engine: testUpdateEngine{}}
	s.apply = func(context.Context, string, string, *updater.Verification) error { return s.ServiceShutdown() }
	done := make(chan error, 1)
	go func() { done <- s.RestartToUpdate(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("synchronous native shutdown deadlocked the update service")
	}
	s.profiles.updateGate.Unlock()
}

func TestUpdatePayloadFilenames(t *testing.T) {
	for _, test := range []struct {
		kind, name string
		valid      bool
	}{
		{"windows", "postika-0.2.0-windows-amd64.exe", true},
		{"windows", "../../postika.exe", false},
		{"windows", `..\postika.exe`, false},
		{"windows", "postika-installer.exe", false},
		{"macos", "postika-darwin-arm64.app.zip", true},
		{"macos", "postika.dmg", false},
		{"deb", "postika-linux-amd64.deb", true},
		{"rpm", "postika-linux-amd64.rpm", true},
		{"arch", "postika-linux-amd64.pkg.tar.zst", true},
		{"arch", "postika-linux-amd64.pkg.tar.xz", true},
		{"rpm", "postika.deb", false},
		{"arch", "postika.tar.gz", false},
		{"appimage", "postika-linux-amd64.AppImage", true},
		{"linux-binary", "postika-0.2.0-linux-amd64", true},
		{"linux-binary", "postika.deb", false},
	} {
		if err := validateUpdateFilename(test.kind, test.name); (err == nil) != test.valid {
			t.Fatalf("%s/%s: %v", test.kind, test.name, err)
		}
	}
}

type testUpdateProvider struct {
	release *updater.Release
	payload []byte
}

func (p testUpdateProvider) Name() string { return "test" }
func (p testUpdateProvider) Check(context.Context, updater.CheckRequest) (*updater.Release, error) {
	return p.release, nil
}
func (p testUpdateProvider) Download(_ context.Context, _ *updater.Release, dst io.Writer, progress func(int64, int64)) error {
	_, err := dst.Write(p.payload)
	progress(int64(len(p.payload)), int64(len(p.payload)))
	return err
}

type testUpdateHost struct{}

func (testUpdateHost) Emit(string, ...any) bool         { return true }
func (testUpdateHost) OnEvent(string, func(any)) func() { return func() {} }
func (testUpdateHost) OpenWindow(updater.WindowOptions) updater.WindowHandle {
	panic("unexpected window")
}
func (testUpdateHost) Quit() { panic("unexpected restart") }

func TestUpdateAuthenticatesPayload(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("MZ signed application")
	digest := sha512.Sum512(payload)
	signature, err := private.Sign(rand.Reader, digest[:], &ed25519.Options{Hash: crypto.SHA512})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "unsigned", "digest-only", "wrong-key", "tampered", "http", "installer"} {
		t.Run(mode, func(t *testing.T) {
			v := &updater.Verification{DigestAlgo: "sha512", Digest: digest[:], SignatureAlgo: "ed25519ph", Signature: signature}
			data, key := payload, public
			artifactURL, filename := "https://postika.softwrhq.com/downloads/postika.exe", "postika.exe"
			switch mode {
			case "unsigned":
				v = nil
			case "digest-only":
				v.Signature = nil
			case "wrong-key":
				key, _, _ = ed25519.GenerateKey(rand.Reader)
			case "tampered":
				data = []byte("MZ malicious payload")
			case "http":
				artifactURL = "http://example.com/file"
			case "installer":
				filename = "postika-installer.exe"
			}
			provider := verifiedUpdateProvider{kind: "windows", Provider: testUpdateProvider{release: &updater.Release{Version: "0.2.0", Artifact: updater.Artifact{Filename: filename}, Verification: v, Metadata: map[string]any{"endpoint.artifact.url": artifactURL}}, payload: data}}
			engine := updater.New(testUpdateHost{})
			if err := engine.Init(updater.Config{CurrentVersion: "0.1.0", PublicKey: key, Providers: []updater.Provider{provider}, Window: updater.WindowNone}); err != nil {
				t.Fatal(err)
			}
			_, checkErr := engine.Check(context.Background())
			if mode == "unsigned" || mode == "digest-only" || mode == "http" || mode == "installer" {
				if checkErr == nil {
					t.Fatal("untrusted release accepted")
				}
				return
			}
			if checkErr != nil {
				t.Fatal(checkErr)
			}
			installErr := engine.DownloadAndInstall(context.Background())
			if mode != "valid" {
				if installErr == nil {
					t.Fatal("untrusted bytes staged")
				}
				return
			}
			if installErr != nil {
				t.Fatal(installErr)
			}
			defer os.RemoveAll(filepath.Dir(engine.DownloadedPath()))
			if err := validateStagedUpdate("windows", engine.DownloadedPath()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestUpdateFeedAllPackageTargets(t *testing.T) {
	digest := make([]byte, sha512.Size)
	signature := make([]byte, ed25519.SignatureSize)
	targets := []struct{ platform, arch, kind, filename string }{
		{"darwin", "arm64", "macos", "postika-darwin-arm64.app.zip"}, {"darwin", "amd64", "macos", "postika-darwin-amd64.app.zip"},
		{"linux", "amd64", "appimage", "postika-linux-amd64.AppImage"}, {"linux", "amd64", "deb", "postika-linux-amd64.deb"},
		{"windows", "amd64", "windows", "postika-windows-amd64.exe"},
		{"linux", "amd64", "rpm", "postika-linux-amd64.rpm"},
		{"linux", "amd64", "arch", "postika-linux-amd64.pkg.tar.zst"},
	}
	for _, target := range targets {
		t.Run(target.kind+"/"+target.arch, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/stable.json" {
					t.Error("checker downloaded payload")
					w.WriteHeader(500)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"schemaVersion": 1, "version": "0.2.0", "channel": "stable", "artifacts": []any{
					map[string]any{"platform": target.platform, "arch": target.arch, "url": "https://postika.softwrhq.com/" + target.filename,
						"digestAlgo": "sha512", "digest": digest, "signatureAlgo": "ed25519ph", "signature": signature},
				}})
			}))
			defer server.Close()
			provider, err := endpoint.New(endpoint.Config{URL: server.URL + "/stable.json", Channel: "stable", HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			release, err := (verifiedUpdateProvider{Provider: provider, kind: target.kind}).Check(context.Background(), updater.CheckRequest{CurrentVersion: "0.1.0", Platform: target.platform, Arch: target.arch})
			if err != nil || release == nil || release.Artifact.Filename != target.filename {
				t.Fatalf("%+v, %v", release, err)
			}
		})
	}
}
