package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const joinedGroupsURL = "https://web.facebook.com/groups/joins/?nav_source=tab&ordering=viewer_added"

// GroupsService owns one browser and a persistent profile dedicated to Postika.
type GroupsService struct {
	updateGate               *sync.RWMutex
	mu                       sync.Mutex
	postMu                   sync.Mutex
	postStatus               string
	postRunMu                sync.Mutex
	posting                  PostingStatus
	postingCancel            context.CancelFunc
	postingPaused            bool
	connectivity             ConnectivityStatus
	connectivityEnabled      bool
	connectivityCheckMu      sync.Mutex
	connectivityProbe        func(context.Context) bool
	connectivityWake         chan struct{}
	connectivityFailures     int
	connectivityGeneration   uint64
	postingChanged           chan struct{}
	postingLogs              []PostingLog
	postingRunID             uint64
	postingStarted           time.Time
	postingItemStarted       time.Time
	settingsMu               sync.Mutex
	settingsPath             string
	settingsDeleted          bool
	windowMu                 sync.Mutex
	app                      *application.App
	postLedger               map[string]PostOutcome
	photoUploads             *photoUploadCapture
	photoUploadWatcher       func(context.Context, *rod.Page) *photoUploadCapture
	workMu                   sync.Mutex
	workCtx                  context.Context
	workCancel               context.CancelFunc
	browser                  *rod.Browser
	marketplaceLocationPage  *rod.Page
	marketplaceTestPage      *rod.Page
	marketplaceTestDir       string
	marketplaceTesting       bool
	marketplaceLocationLoad  *marketplaceLocationLoad
	marketplaceLocationLoads map[bool]*marketplaceLocationLoad
	marketplaceLocationRunID uint64
	page                     *rod.Page
	launcher                 *launcher.Launcher
	cancel                   context.CancelFunc
	closed                   bool
	profileDir               string
	profileID                string
	profileName              string
	profileNameMu            sync.RWMutex
	postImageDir             string
	chromePath               string
	targetURL                string
	headless                 bool
	sortMenuOpened           bool
	sortOptionSelected       bool
	groupsLoad               *groupsLoad
	groupsCachePath          string
	groupsCacheLoaded        bool
	facebookPageLoad         *facebookPageLoad
	facebookPageRunID        uint64
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
	s.stopFacebookPageFetch()

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
	page := s.page.Context(ctx)
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait for Facebook groups to load: %w", err)
	}
	// Facebook keeps changing the DOM in the background. Give the initial
	// content time to settle without waiting forever for a completely idle page.
	settleCtx, settleCancel := context.WithTimeout(ctx, 5*time.Second)
	err := page.Context(settleCtx).WaitDOMStable(300*time.Millisecond, 0)
	settleCancel()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("wait for Facebook groups content: %w", err)
	}
	if _, err := page.Activate(); err != nil {
		return fmt.Errorf("activate groups tab: %w", err)
	}
	return nil
}

// ensureBrowser is called with mu held. It connects the account browser without
// opening a tab, so callers can create their destination directly.
func (s *GroupsService) ensureBrowser(ctx context.Context) error {
	if s.browser != nil {
		if _, err := (proto.BrowserGetVersion{}).Call(s.browser.Context(ctx)); err != nil {
			s.closeBrowser()
		}
	}
	if s.browser == nil {
		return s.launchBrowser(ctx)
	}
	return nil
}

// ensureBrowserPage is called with mu held and reuses the persistent profile.
func (s *GroupsService) ensureBrowserPage(ctx context.Context) error {
	if err := s.ensureBrowser(ctx); err != nil {
		return err
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
		if !s.headless {
			if err := page.Context(ctx).SetWindow(&proto.BrowserBounds{WindowState: proto.BrowserWindowStateMaximized}); err != nil {
				return fmt.Errorf("maximize Chrome window: %w", err)
			}
		}
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
		configDir, err := defaultProfileRoot()
		if err != nil {
			return fmt.Errorf("locate Postika browser profile: %w", err)
		}
		dir = filepath.Join(configDir, "browser-profile")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create Postika browser profile: %w", err)
	}
	// NewUserMode defaults to a shared fixed port. Choose a free loopback port
	// for this launch so different accounts never attach to the same browser.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("allocate Chrome debugging port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		return fmt.Errorf("release Chrome debugging port: %w", err)
	}
	l := newBrowserLauncher(ctx, chrome, dir, port, s.headless)
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

// Use normal Chrome settings rather than Rod's automation preset. Each account
// still has its own persistent directory and independently managed process.
func newBrowserLauncher(ctx context.Context, chrome, dir string, port int, headless bool) *launcher.Launcher {
	// Rod v0.116.2's NewUserMode does not initialize its exit channel. Keep
	// New's process lifecycle (needed by profile deletion's Cleanup), but
	// replace its automation defaults with the complete user-mode preset.
	l := launcher.New()
	l.Flags = launcher.NewUserMode().Flags
	return l.Context(ctx).Bin(chrome).
		Leakless(true).
		NoSandbox(true).
		Headless(headless).
		Devtools(false).
		UserDataDir(dir).
		RemoteDebuggingPort(port).
		Set("disable-notifications").
		Set("start-maximized")
}

// closeBrowser is called with the service mutex held. Never call launcher.Cleanup:
// it would delete the persistent profile, including the user's Facebook login.
func (s *GroupsService) closeBrowser() {
	s.stopFacebookPageFetch()
	s.stopGroupsCapture()
	s.closeBrowserConnection()
}

// Close Chrome without cancelling the independent Go groups job. Called with mu held.
func (s *GroupsService) closeBrowserConnection(keep ...*marketplaceLocationLoad) {
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
	if s.marketplaceLocationLoad != nil && (len(keep) == 0 || s.marketplaceLocationLoad != keep[0]) {
		s.marketplaceLocationLoad.cancel()
	}
	s.marketplaceLocationPage = nil
	s.marketplaceTestPage = nil
	if s.marketplaceTestDir != "" {
		_ = os.RemoveAll(s.marketplaceTestDir)
		s.marketplaceTestDir = ""
	}
	s.browser, s.page, s.launcher, s.cancel = nil, nil, nil, nil
	if s.postImageDir != "" {
		_ = os.RemoveAll(s.postImageDir)
		s.postImageDir = ""
	}
}

func (s *GroupsService) ServiceShutdown() error { return s.shutdown(false) }

func (s *GroupsService) shutdown(removeBrowserData bool) error {
	s.workMu.Lock()
	if s.workCancel != nil {
		s.workCancel()
	}
	s.workMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, load := range s.marketplaceLocationLoads {
		if load.cancel != nil {
			load.cancel()
		}
		load.mu.Lock()
		if load.replay != nil {
			load.replay.stop()
		}
		load.mu.Unlock()
	}
	launch := s.launcher
	s.closeBrowser()
	if removeBrowserData && launch != nil {
		launch.Cleanup() // Wait for Chrome to exit before removing its login data.
	}
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
