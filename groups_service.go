package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const joinedGroupsURL = "https://web.facebook.com/groups/joins/?nav_source=tab&ordering=viewer_added"

// GroupsService owns one browser and a persistent profile dedicated to Postika.
type GroupsService struct {
	mu                  sync.Mutex
	postMu              sync.Mutex
	postStatus          string
	postRunMu           sync.Mutex
	posting             PostingStatus
	postingCancel       context.CancelFunc
	postingPaused       bool
	connectivity        ConnectivityStatus
	connectivityEnabled bool
	connectivityCheckMu sync.Mutex
	connectivityProbe   func(context.Context) bool
	postingChanged      chan struct{}
	postingLogs         []PostingLog
	postingRunID        uint64
	postingStarted      time.Time
	postingItemStarted  time.Time
	settingsMu          sync.Mutex
	settingsPath        string
	windowMu            sync.Mutex
	app                 *application.App
	postLedger          map[string]PostOutcome
	photoUploads        *photoUploadCapture
	photoUploadWatcher  func(context.Context, *rod.Page) *photoUploadCapture
	workMu              sync.Mutex
	workCtx             context.Context
	workCancel          context.CancelFunc
	browser             *rod.Browser
	page                *rod.Page
	launcher            *launcher.Launcher
	cancel              context.CancelFunc
	closed              bool
	profileDir          string
	profileID           string
	profileName         string
	postImageDir        string
	chromePath          string
	targetURL           string
	headless            bool
	sortMenuOpened      bool
	sortOptionSelected  bool
	groupsLoad          *groupsLoad
	groupsCachePath     string
	groupsCacheLoaded   bool
}

func (s *GroupsService) OpenJoinedGroups(ctx context.Context) error {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("Postika is shutting down")
	}
	s.stopGroupsCapture()

	if err := s.ensureBrowserPage(ctx); err != nil {
		return err
	}

	url := s.targetURL
	if url == "" {
		url = joinedGroupsURL
	}
	s.sortMenuOpened = false
	s.sortOptionSelected = false
	if err := s.page.Context(ctx).Navigate(url); err != nil {
		return fmt.Errorf("open Facebook groups: %w", err)
	}
	if _, err := s.page.Context(ctx).Activate(); err != nil {
		return fmt.Errorf("activate groups tab: %w", err)
	}
	return nil
}

// ensureBrowserPage is called with mu held and reuses the persistent profile.
func (s *GroupsService) ensureBrowserPage(ctx context.Context) error {

	if s.browser != nil {
		if _, err := (proto.BrowserGetVersion{}).Call(s.browser.Context(ctx)); err != nil {
			// The user may have closed Chrome since the previous click.
			s.closeBrowser()
		}
	}
	if s.browser == nil {
		if err := s.launchBrowser(ctx); err != nil {
			return err
		}
	}
	if s.page != nil {
		if _, err := (proto.TargetGetTargetInfo{TargetID: s.page.TargetID}).Call(s.browser.Context(ctx)); err != nil {
			s.page = nil
		}
	}
	if s.page == nil {
		// Rod keeps the creating browser's context in the page session, so use
		// the long-lived browser here rather than a request-context clone.
		page, err := s.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
		if err != nil {
			return fmt.Errorf("open browser tab: %w", err)
		}
		// The tab remains available after this binding call finishes.
		s.page = page
	}
	return nil
}

func (s *GroupsService) launchBrowser(ctx context.Context) error {
	chrome := s.chromePath
	if chrome == "" {
		var found bool
		chrome, found = launcher.LookPath()
		if !found {
			return errors.New("Chrome or Chromium was not found. Install a browser to open your Facebook groups")
		}
	}
	dir := s.profileDir
	if dir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("locate Postika browser profile: %w", err)
		}
		dir = filepath.Join(configDir, "postika", "browser-profile")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create Postika browser profile: %w", err)
	}
	l := launcher.New().Context(ctx).Bin(chrome).UserDataDir(dir).Headless(s.headless)
	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch Chrome: %w", err)
	}
	// The browser connection must outlive the request that opened it.
	browserCtx, cancel := context.WithCancel(context.Background())
	browser := rod.New().ControlURL(controlURL).Context(browserCtx).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		cancel()
		l.Kill()
		return fmt.Errorf("connect to Chrome: %w", err)
	}
	s.browser, s.launcher, s.cancel = browser, l, cancel
	return nil
}

// closeBrowser is called with the service mutex held. Never call launcher.Cleanup:
// it would delete the persistent profile, including the user's Facebook login.
func (s *GroupsService) closeBrowser() {
	s.stopGroupsCapture()
	s.closeBrowserConnection()
}

// Close Chrome without cancelling the independent Go groups job. Called with mu held.
func (s *GroupsService) closeBrowserConnection() {
	s.photoUploads = nil
	if s.browser != nil {
		err := s.browser.Context(context.Background()).Close()
		if err != nil && s.launcher != nil {
			s.launcher.Kill()
		}
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.browser, s.page, s.launcher, s.cancel = nil, nil, nil, nil
	if s.postImageDir != "" {
		_ = os.RemoveAll(s.postImageDir)
		s.postImageDir = ""
	}
}

func (s *GroupsService) ServiceShutdown() error {
	s.workMu.Lock()
	if s.workCancel != nil {
		s.workCancel()
	}
	s.workMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.closeBrowser()
	return nil
}

// Operations have cancellation, but no elapsed-time deadlines.
func (s *GroupsService) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	s.workMu.Lock()
	if s.workCtx == nil {
		s.workCtx, s.workCancel = context.WithCancel(context.Background())
	}
	lifetime := s.workCtx
	s.workMu.Unlock()
	operation, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(lifetime, cancel)
	return operation, func() { stop(); cancel() }
}
