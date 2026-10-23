//go:build linux

package main

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateDebMetadata(t *testing.T) {
	for _, test := range []struct {
		input string
		valid bool
	}{
		{"Package: postika\nVersion: 0.2.0-1\nArchitecture: amd64", true},
		{"Package: postika\nVersion: 0.2.0\nArchitecture: amd64", true},
		{"Package: another-app\nVersion: 0.2.0-1\nArchitecture: amd64", false},
		{"Package: postika\nVersion: 0.1.0-1\nArchitecture: amd64", false},
		{"Package: postika\nVersion: 0.2.0-1\nArchitecture: arm64", false},
	} {
		if err := validateDebMetadata(test.input, "0.2.0", "amd64"); (err == nil) != test.valid {
			t.Fatalf("%q: %v", test.input, err)
		}
	}
	if !debOwnsExecutable("postika: /usr/local/bin/postika\n", "/usr/local/bin/postika") {
		t.Fatal("DEB install not recognized")
	}
	if debOwnsExecutable("other: /usr/local/bin/postika\n", "/usr/local/bin/postika") {
		t.Fatal("unrelated package accepted")
	}
}
func TestWailsAppImageStagesBesideOuterFile(t *testing.T) {
	outer := filepath.Join(t.TempDir(), "Postika.AppImage")
	if err := os.WriteFile(outer, []byte("old image"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPIMAGE", outer)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDIR", filepath.Dir(executable))
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte{'\x7f', 'E', 'L', 'F', 0, 0, 0, 0, 'A', 'I', 2, 0}
	digest := sha512.Sum512(payload)
	sig, err := private.Sign(rand.Reader, digest[:], &ed25519.Options{Hash: crypto.SHA512})
	if err != nil {
		t.Fatal(err)
	}
	provider := testUpdateProvider{payload: payload, release: &updater.Release{Version: "0.2.0", Artifact: updater.Artifact{Filename: "postika-linux-amd64.AppImage"}, Verification: &updater.Verification{DigestAlgo: "sha512", Digest: digest[:], SignatureAlgo: "ed25519ph", Signature: sig}}}
	engine := updater.New(testUpdateHost{})
	if err := engine.Init(updater.Config{CurrentVersion: "0.1.0", Providers: []updater.Provider{provider}, PublicKey: public, Window: updater.WindowNone}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.DownloadAndInstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := engine.DownloadedPath()
	defer os.RemoveAll(filepath.Dir(path))
	if filepath.Dir(filepath.Dir(path)) != filepath.Dir(outer) {
		t.Fatalf("staged in mount/temp directory: %s", path)
	}
	if err := validateStagedUpdate("appimage", path); err != nil {
		t.Fatal(err)
	}
}
func TestUpdatePackageStagesInUserCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("!<arch>\nsigned test package")
	digest := sha512.Sum512(payload)
	signature, err := private.Sign(rand.Reader, digest[:], &ed25519.Options{Hash: crypto.SHA512})
	if err != nil {
		t.Fatal(err)
	}
	provider := testUpdateProvider{payload: payload, release: &updater.Release{Version: "0.2.0", Artifact: updater.Artifact{Filename: "postika-linux-amd64.deb"}, Verification: &updater.Verification{DigestAlgo: "sha512", Digest: digest[:], SignatureAlgo: "ed25519ph", Signature: signature}}}
	engine := &packageUpdateEngine{publicKey: public, provider: provider, request: updater.CheckRequest{CurrentVersion: "0.1.0", Platform: "linux", Arch: "amd64"}}
	if _, err := engine.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := engine.DownloadAndInstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cache, engine.DownloadedPath())
	if err != nil || relative == ".." {
		t.Fatal("system package staged beside root-owned binary")
	}
	actual, err := os.ReadFile(engine.DownloadedPath())
	if err != nil || string(actual) != string(payload) {
		t.Fatal("package bytes changed")
	}
	// A tampered package cannot replace a verified cached payload.
	provider.payload = []byte("tampered package")
	engine.provider = provider
	if err := engine.DownloadAndInstall(context.Background()); err == nil {
		t.Fatal("tampered package accepted")
	}
	actual, _ = os.ReadFile(engine.DownloadedPath())
	if string(actual) != string(payload) {
		t.Fatal("failed download discarded verified payload")
	}
}
func TestUpdateRelaunchWaitsForParent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := waitUpdateParent(ctx, os.Getpid()); err == nil {
		t.Fatal("relaunch did not wait for running parent")
	}
	if err := waitUpdateParent(context.Background(), 999999999); err != nil {
		t.Fatal(err)
	}
}
func TestUpdateStagedPayloadTypes(t *testing.T) {
	for _, test := range []struct {
		kind  string
		data  []byte
		valid bool
	}{
		{"appimage", []byte{'\x7f', 'E', 'L', 'F', 0, 0, 0, 0, 'A', 'I', 2, 0}, true},
		{"appimage", []byte("\x7fELF not an AppImage"), false},
		{"deb", []byte("!<arch>\npackage"), true},
		{"rpm", []byte{0xed, 0xab, 0xee, 0xdb, 0, 0, 0, 0, 0, 0, 0, 0}, true},
		{"rpm", []byte("!<arch>\npackage"), false},
		{"arch", []byte{0x28, 0xb5, 0x2f, 0xfd, 0, 0, 0, 0, 0, 0, 0, 0}, true},
		{"arch", []byte("\x7fELF application"), false},
		{"deb", []byte("\x7fELF application"), false},
		{"windows", []byte("MZ test binary"), true},
		{"windows", []byte("not executable"), false},
		{"linux-binary", []byte("\x7fELF binary"), true},
	} {
		path := filepath.Join(t.TempDir(), "payload")
		_ = os.WriteFile(path, test.data, 0700)
		if err := validateStagedUpdate(test.kind, path); (err == nil) != test.valid {
			t.Fatalf("%s: %v", test.kind, err)
		}
	}
	bundle := filepath.Join(t.TempDir(), "postika.app")
	_ = os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0700)
	if err := validateStagedUpdate("macos", bundle); err == nil {
		t.Fatal("incomplete app bundle accepted")
	}
	_ = os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte("plist"), 0600)
	_ = os.WriteFile(filepath.Join(bundle, "Contents", "MacOS", "postika"), []byte("executable"), 0700)
	if err := validateStagedUpdate("macos", bundle); err != nil {
		t.Fatal(err)
	}
}
