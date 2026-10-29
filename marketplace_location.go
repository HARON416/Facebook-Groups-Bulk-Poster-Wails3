package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type MarketplaceLocationStatus struct {
	RunID    uint64 `json:"runID"`
	State    string `json:"state"`
	Message  string `json:"message"`
	Captured bool   `json:"captured"`
}

type marketplaceLocationLoad struct {
	mu     sync.Mutex
	status MarketplaceLocationStatus
	cancel context.CancelFunc
	done   chan struct{}
	seed   string
	replay *marketplaceLocationReplay
}

func (load *marketplaceLocationLoad) set(state, message string) {
	load.mu.Lock()
	defer load.mu.Unlock()
	load.status.State, load.status.Message = state, message
}
func (load *marketplaceLocationLoad) snapshot() MarketplaceLocationStatus {
	load.mu.Lock()
	defer load.mu.Unlock()
	status := load.status
	status.Captured = load.replay != nil
	return status
}

func (m *ProfileService) GetMarketplaceLocationStatus(profileID string) (MarketplaceLocationStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		return MarketplaceLocationStatus{}, err
	}
	s.mu.Lock()
	load := s.marketplaceLocationLoad
	s.mu.Unlock()
	if load == nil {
		return MarketplaceLocationStatus{State: "idle"}, nil
	}
	return load.snapshot(), nil
}
func (m *ProfileService) CancelMarketplaceLocation(profileID string, runID uint64) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if load := s.marketplaceLocationLoad; load != nil && load.snapshot().RunID == runID {
		load.cancel()
	}
	return nil
}

const marketplaceItemURL = "https://web.facebook.com/marketplace/create/item"

func (m *ProfileService) OpenMarketplaceLocation(ctx context.Context, profileID, query string) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 1024 {
		return errors.New("Enter a town or city")
	}
	return s.openMarketplaceLocation(ctx, marketplaceItemURL, query)
}

// Keep Marketplace in its own tab, using the account's existing browser cookies.
func (s *GroupsService) openMarketplaceLocation(ctx context.Context, target string, queries ...string) error {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	ctx, timeout := context.WithTimeout(ctx, 30*time.Second)
	defer timeout()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("Postika is shutting down")
	}
	if err := s.ensureBrowser(ctx); err != nil {
		return err
	}
	if s.marketplaceLocationPage != nil {
		if _, err := (proto.TargetGetTargetInfo{TargetID: s.marketplaceLocationPage.TargetID}).Call(s.browser.Context(ctx)); err != nil {
			s.marketplaceLocationPage = nil
		}
	}
	if previous := s.marketplaceLocationLoad; previous != nil {
		previous.cancel()
		<-previous.done
		previous.mu.Lock()
		oldReplay := previous.replay
		previous.mu.Unlock()
		if oldReplay != nil {
			oldReplay.stop()
		}
	}
	created := false
	if s.marketplaceLocationPage == nil {
		// Rod's Browser.Page first creates about:blank even when given a URL.
		// Create the destination target directly and attach on the lasting context.
		targetInfo, err := (proto.TargetCreateTarget{URL: target}).Call(s.browser.Context(ctx))
		if err != nil {
			return fmt.Errorf("open Marketplace tab: %w", err)
		}
		page, err := s.browser.PageFromTarget(targetInfo.TargetID)
		if err != nil {
			_, _ = (proto.TargetCloseTarget{TargetID: targetInfo.TargetID}).Call(s.browser.Context(ctx))
			return fmt.Errorf("attach Marketplace tab: %w", err)
		}
		s.marketplaceLocationPage = page
		created = true
	}
	page := s.marketplaceLocationPage.Context(ctx)
	if !created {
		if err := page.Navigate(target); err != nil {
			return fmt.Errorf("open Facebook Marketplace: %w", err)
		}
	}
	if !s.headless {
		if err := page.SetWindow(&proto.BrowserBounds{WindowState: proto.BrowserWindowStateMaximized}); err != nil {
			return fmt.Errorf("show Facebook Marketplace: %w", err)
		}
	}
	if _, err := page.Activate(); err != nil {
		return fmt.Errorf("activate Facebook Marketplace: %w", err)
	}
	job, stop := s.operationContext(context.Background())
	s.marketplaceLocationRunID++
	load := &marketplaceLocationLoad{cancel: stop, done: make(chan struct{}), status: MarketplaceLocationStatus{RunID: s.marketplaceLocationRunID, State: "checking", Message: facebookLoginWaitMessage}}
	if len(queries) > 0 {
		load.seed = strings.TrimSpace(queries[0])
	}
	s.marketplaceLocationLoad = load
	browserPage := s.marketplaceLocationPage
	go func() {
		defer close(load.done)
		defer stop()
		err := s.waitForMarketplaceLogin(job, browserPage, target, load)
		if job.Err() != nil {
			load.set("cancelled", "")
			return
		}
		if errors.Is(err, errMarketplaceDetails) {
			load.set("details_error", err.Error())
		} else if errors.Is(err, errMarketplaceLocationBrowserClosed) {
			load.set("browser_closed", "")
		} else if errors.Is(err, errMarketplaceLocationConnection) {
			load.set("connection_required", "")
		} else if err != nil {
			load.set("error", err.Error())
		}
	}()
	return nil
}

// The job outlives the opening binding call and has no login-wait deadline.
// An absent popup alone is not proof of login: use the same positive DOM
// detector and cancellable wait as group posting and Facebook page imports.
func (s *GroupsService) waitForMarketplaceLogin(ctx context.Context, browserPage *rod.Page, target string, load *marketplaceLocationLoad) error {
	page := browserPage.Context(ctx)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var details marketplaceDetailsAttempt
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		probe, stopProbe := context.WithTimeout(ctx, 2*time.Second)
		_, probeErr := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(probe))
		stopProbe()
		if probeErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(probeErr, context.DeadlineExceeded) {
				continue
			}
			load.set("browser_closed", "")
			return nil
		}
		probe, stopProbe = context.WithTimeout(ctx, 2*time.Second)
		login, err := page.Context(probe).Eval(facebookLoginSelectorScript)
		stopProbe()
		if err == nil {
			switch login.Value.Str() {
			case "logged_out", "login_required":
				details = marketplaceDetailsAttempt{}
				load.set("waiting_login", facebookLoginWaitMessage)
				// Bound each probe cycle, not the user's login wait. A context
				// replaced during navigation must not stall closure/cancellation.
				loginProbe, stopLoginProbe := context.WithTimeout(ctx, 2*time.Second)
				loginErr := waitForFacebookLogin(loginProbe, page.Context(loginProbe), nil)
				stopLoginProbe()
				if errors.Is(loginErr, context.DeadlineExceeded) && ctx.Err() == nil {
					continue
				}
				if err := loginErr; err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					if _, tabErr := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(ctx)); tabErr != nil {
						load.set("browser_closed", "")
						return nil
					}
					return err
				}
				load.set("checking", facebookLoginWaitMessage)
				if err := page.Navigate(target); err != nil {
					return fmt.Errorf("return to Marketplace after login: %w", err)
				}
				continue
			case "logged_in":
				info, err := page.Info()
				if err != nil {
					break
				}
				if !sameMarketplaceItemPage(info.URL, target) {
					details = marketplaceDetailsAttempt{}
					if err := page.Navigate(target); err != nil {
						return fmt.Errorf("return to Marketplace after login: %w", err)
					}
					break
				}
				load.set("expanding_details", "")
				expanded, err := expandMarketplaceDetails(ctx, page, &details)
				if err != nil {
					return err
				}
				if expanded {
					if load.seed != "" {
						load.set("capturing_location", "")
						replay, err := prepareMarketplaceLocationReplay(ctx, page, target, load.seed)
						if errors.Is(err, errFacebookLoginRequired) {
							details = marketplaceDetailsAttempt{}
							continue
						}
						if err != nil {
							return err
						}
						if err := s.closeMarketplaceAfterCapture(ctx, browserPage, load); err != nil {
							return err
						}
						load.mu.Lock()
						load.replay = replay
						load.mu.Unlock()
					}
					load.set("ready", "")
					return nil
				}

			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Called only after a successful capture and replay. Other account work keeps
// its browser; otherwise release the dedicated browser without cancelling the
// completing job or removing the persistent Facebook login data.
func (s *GroupsService) closeMarketplaceAfterCapture(ctx context.Context, page *rod.Page, load *marketplaceLocationLoad) error {
	// A new opening call may hold mu while cancelling this job and awaiting
	// done, so acquiring mu must remain responsive to that cancellation.
	for !s.mu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer s.mu.Unlock()
	if s.closed || ctx.Err() != nil || s.marketplaceLocationLoad != load {
		return context.Canceled
	}
	if s.browser == nil || s.marketplaceLocationPage == nil {
		return nil
	}
	if s.marketplaceLocationPage.TargetID != page.TargetID {
		return context.Canceled
	}
	pages, err := s.browser.Context(ctx).Pages()
	if err != nil {
		return errors.New("Could not close the Marketplace browser")
	}
	s.postMu.Lock()
	posting := s.postingActiveLocked()
	s.postMu.Unlock()
	if s.page == nil && len(pages) == 1 && !posting && s.postImageDir == "" {
		s.closeBrowserConnection(load)
		return nil
	}
	if _, err := (proto.TargetCloseTarget{TargetID: page.TargetID}).Call(s.browser.Context(ctx)); err != nil {
		return errors.New("Could not close the Marketplace tab")
	}
	s.marketplaceLocationPage = nil
	return nil
}
