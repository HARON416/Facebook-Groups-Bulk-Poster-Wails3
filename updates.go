package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha512"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"postika/internal/updateauth"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"
)

const updateEndpoint = "https://postika.softwrhq.com/updates/postika/stable.json"
const updateInterval = time.Minute

//go:embed build/windows/info.json
var updateBuildInfo []byte

var updatePublicKey = updateauth.PublicKey

type UpdateStatus struct {
	CurrentVersion string `json:"currentVersion"`
	Version        string `json:"version"`
	State          string `json:"state"`
	Error          string `json:"error"`
	CheckedAt      string `json:"checkedAt"`
	Package        string `json:"package"`
}

type updateEngine interface {
	Check(context.Context) (*updater.Release, error)
	DownloadAndInstall(context.Context) error
	DownloadedPath() string
	Restart(context.Context) error
}

type updateTarget struct{ Kind, Path string }

// UpdateService automatically checks and stages verified updates. Installing
// waits for an explicit restart request and the completion of all posting runs.
type UpdateService struct {
	verification                *updater.Verification
	mu                          sync.Mutex
	op                          sync.Mutex
	status                      UpdateStatus
	engine                      updateEngine
	profiles                    *ProfileService
	target                      updateTarget
	apply                       func(context.Context, string, string, *updater.Verification) error
	quit                        func()
	ctx                         context.Context
	cancel                      context.CancelFunc
	done                        chan struct{}
	startupDelay, checkInterval time.Duration
}

func newUpdateService(app *application.App, profiles *ProfileService) (*UpdateService, error) {
	var info struct {
		Info map[string]struct {
			ProductVersion string `json:"ProductVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(updateBuildInfo, &info); err != nil {
		return nil, err
	}
	version := info.Info["0000"].ProductVersion
	target, err := detectUpdateTarget()
	if err != nil {
		return nil, err
	}
	feed := updateEndpoint
	if isSystemPackage(target.Kind) {
		feed = "https://postika.softwrhq.com/updates/postika/stable-" + target.Kind + ".json"
	}
	if target.Kind == "linux-binary" {
		feed = "https://postika.softwrhq.com/updates/postika/stable-binary.json"
	}
	provider, err := endpoint.New(endpoint.Config{URL: feed, Channel: "stable", HTTPClient: &http.Client{
		Timeout: 30 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 || req.URL.Scheme != "https" {
				return errors.New("Update redirects must use HTTPS")
			}
			return nil
		},
	}})
	if err != nil {
		return nil, err
	}
	verified := verifiedUpdateProvider{Provider: provider, kind: target.Kind}
	var engine updateEngine
	if isSystemPackage(target.Kind) {
		engine = &packageUpdateEngine{publicKey: updatePublicKey, provider: verified, request: updater.CheckRequest{CurrentVersion: version, Platform: runtime.GOOS, Arch: runtime.GOARCH}}
	} else {
		if err := app.Updater.Init(updater.Config{CurrentVersion: version, PublicKey: updatePublicKey, Providers: []updater.Provider{verified}, Window: updater.WindowNone}); err != nil {
			return nil, err
		}
		engine = app.Updater
	}
	s := &UpdateService{engine: engine, profiles: profiles, target: target, quit: app.Quit,
		startupDelay: 10 * time.Second, checkInterval: updateInterval,
		status: UpdateStatus{CurrentVersion: version, State: "idle", Package: target.Kind},
	}
	s.apply = func(ctx context.Context, staged, version string, verification *updater.Verification) error {
		if isSystemPackage(target.Kind) {
			return applyPlatformUpdate(ctx, target, staged, version, verification)
		}
		return app.Updater.Restart(ctx)
	}
	return s, nil
}

// Wails permits unsigned releases by default; require the signature and digest
// emitted by the pinned release CLI and reject payload types for other installs.
type verifiedUpdateProvider struct {
	updater.Provider
	kind string
}

func (p verifiedUpdateProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	r, err := p.Provider.Check(ctx, req)
	if err != nil || r == nil {
		return r, err
	}
	v := r.Verification
	if v == nil || v.DigestAlgo != "sha512" || len(v.Digest) != sha512.Size || v.SignatureAlgo != "ed25519ph" || len(v.Signature) != ed25519.SignatureSize {
		return nil, errors.New("Update is missing its checksum or signature")
	}
	raw, _ := r.Metadata["endpoint.artifact.url"].(string)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, errors.New("Update downloads must use HTTPS")
	}
	if err := validateUpdateFilename(p.kind, r.Artifact.Filename); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *UpdateService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		timer := time.NewTimer(s.startupDelay)
		defer timer.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-timer.C:
				timer.Reset(s.checkInterval)
				_, _ = s.CheckForUpdates(s.ctx)
			}
		}
	}()
	return nil
}
func (s *UpdateService) ServiceShutdown() error {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}
	s.op.Lock()
	s.op.Unlock()
	return nil
}
func (s *UpdateService) GetUpdateStatus() UpdateStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *UpdateService) CheckForUpdates(ctx context.Context) (UpdateStatus, error) {
	if !s.op.TryLock() {
		return s.GetUpdateStatus(), nil
	}
	releaseLock := true
	defer func() {
		if releaseLock {
			s.op.Unlock()
		}
	}()
	previous := s.GetUpdateStatus()
	if previous.State == "ready" || previous.State == "installing" || previous.State == "restarting" {
		return previous, nil
	}
	s.mu.Lock()
	s.status.State = "checking"
	s.status.Error = ""
	s.mu.Unlock()
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	r, err := s.engine.Check(checkCtx)
	cancel()
	s.mu.Lock()
	s.status.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		s.status.State = "error"
		s.status.Error = "Could not check for updates. Please try again later."
		if previous.Version != "" {
			s.status.State = "available"
		}
	} else if r == nil {
		s.status.State = "up-to-date"
		s.status.Version = ""
	} else {
		s.verification = r.Verification
		s.status.Version = r.Version
		s.status.State = "downloading"
	}
	s.mu.Unlock()
	if err == nil && r != nil {
		releaseLock = false // The staging goroutine owns the operation lock.
		go s.stageUpdate()
	}
	return s.GetUpdateStatus(), nil
}

func (s *UpdateService) stageUpdate() {
	defer s.op.Unlock()
	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()
	err := s.engine.DownloadAndInstall(ctx)
	if err == nil {
		err = validateStagedUpdate(s.target.Kind, s.engine.DownloadedPath())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.status.State = "available"
		s.status.Error = "Could not download or verify the update. Please try again."
		return
	}
	s.status.State = "ready"
	s.status.Error = ""
}
func (s *UpdateService) DownloadUpdate() error {
	if !s.op.TryLock() {
		return errors.New("An update operation is already running")
	}
	if s.GetUpdateStatus().State != "available" {
		s.op.Unlock()
		return errors.New("No update is available to retry")
	}
	s.mu.Lock()
	s.status.State = "downloading"
	s.status.Error = ""
	s.mu.Unlock()
	go s.stageUpdate()
	return nil
}
func (s *UpdateService) RestartToUpdate(ctx context.Context) error {
	if !s.op.TryLock() {
		return errors.New("An update operation is already running")
	}
	locked := true
	defer func() {
		if locked {
			s.op.Unlock()
		}
	}()
	status := s.GetUpdateStatus()
	if status.State != "ready" {
		return errors.New("The update must finish downloading before restarting")
	}
	if !s.profiles.updateGate.TryLock() {
		return errors.New("Stop all posting queues, including scheduled or paused queues, before restarting to update")
	}
	s.mu.Lock()
	s.status.State = "installing"
	s.status.Error = ""
	s.mu.Unlock()
	// Wails Restart calls app.Quit synchronously, which runs ServiceShutdown.
	// State guards prevent competing operations while the mutex is released.
	// Holding it through that callback would deadlock native shutdown.
	if !isSystemPackage(s.target.Kind) {
		locked = false
		s.op.Unlock()
	}
	if err := s.apply(ctx, s.engine.DownloadedPath(), status.Version, s.verification); err != nil {
		s.profiles.updateGate.Unlock()
		s.mu.Lock()
		s.status.State = "ready"
		s.mu.Unlock()
		return fmt.Errorf("Could not install the update: %w", err)
	}
	s.mu.Lock()
	s.status.State = "restarting"
	s.mu.Unlock()
	if isSystemPackage(s.target.Kind) {
		locked = false
		s.op.Unlock()
		s.quit()
	}
	return nil
}

func validateUpdateFilename(kind, filename string) error {
	// Each distribution has its own payload; installers must never be swapped
	// into a running executable or .app bundle.
	if filename == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") {
		return errors.New("Update payload filename must not contain a path")
	}
	lower := strings.ToLower(filename)
	valid := false
	switch kind {
	case "windows":
		valid = strings.HasSuffix(lower, ".exe") && !strings.Contains(lower, "installer") && !strings.Contains(lower, "setup")
	case "macos":
		valid = strings.HasSuffix(lower, ".zip")
	case "deb":
		valid = strings.HasSuffix(lower, ".deb")
	case "rpm":
		valid = strings.HasSuffix(lower, ".rpm")
	case "arch":
		valid = strings.HasSuffix(lower, ".pkg.tar.zst") || strings.HasSuffix(lower, ".pkg.tar.xz") || strings.HasSuffix(lower, ".pkg.tar.gz")
	case "appimage":
		valid = strings.HasSuffix(lower, ".appimage")
	case "linux-binary":
		valid = !strings.HasSuffix(lower, ".rpm") && !strings.Contains(lower, ".pkg.tar.") && !strings.HasSuffix(lower, ".zip") && !strings.HasSuffix(lower, ".deb") && !strings.HasSuffix(lower, ".appimage") && !strings.HasSuffix(lower, ".exe")
	}
	if !valid {
		return fmt.Errorf("Invalid %s update payload: %s", kind, filename)
	}
	return nil
}

func validateStagedUpdate(kind, path string) error {
	if kind == "macos" {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() || !strings.HasSuffix(path, ".app") {
			return errors.New("macOS update must contain one complete .app bundle")
		}
		for _, relative := range []string{"Contents/Info.plist", "Contents/MacOS/postika"} {
			if _, err := os.Stat(filepath.Join(path, relative)); err != nil {
				return err
			}
		}
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 12)
	n, _ := io.ReadFull(file, header)
	valid := false
	switch kind {
	case "windows":
		valid = n >= 2 && string(header[:2]) == "MZ"
	case "appimage":
		valid = n >= 11 && string(header[:4]) == "\x7fELF" && string(header[8:10]) == "AI" && (header[10] == 1 || header[10] == 2)
	case "rpm":
		valid = n >= 4 && string(header[:4]) == "\xed\xab\xee\xdb"
	case "arch":
		valid = (n >= 4 && string(header[:4]) == "\x28\xb5\x2f\xfd") || (n >= 6 && string(header[:6]) == "\xfd7zXZ\x00") || (n >= 2 && header[0] == 0x1f && header[1] == 0x8b)
	case "deb":
		valid = n >= 8 && string(header[:8]) == "!<arch>\n"
	case "linux-binary":
		valid = n >= 4 && string(header[:4]) == "\x7fELF"
	}
	if !valid {
		return errors.New("Downloaded update is not the expected application/package type")
	}
	return nil
}
