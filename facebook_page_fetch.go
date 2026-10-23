package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type FacebookPageFetchStatus struct {
	RunID   uint64             `json:"runID"`
	State   string             `json:"state"`
	Message string             `json:"message"`
	Target  int                `json:"target"`
	Posts   []FacebookPagePost `json:"posts"`
}
type facebookPageLoad struct {
	mu     sync.Mutex
	status FacebookPageFetchStatus
	cancel context.CancelFunc
	done   chan struct{}
}

func (load *facebookPageLoad) set(state, message string, posts []FacebookPagePost) {
	load.mu.Lock()
	defer load.mu.Unlock()
	load.status.State, load.status.Message = state, message
	if posts != nil {
		load.status.Posts = append([]FacebookPagePost{}, posts...)
	}
}
func (load *facebookPageLoad) snapshot() FacebookPageFetchStatus {
	load.mu.Lock()
	defer load.mu.Unlock()
	status := load.status
	status.Posts = append([]FacebookPagePost{}, status.Posts...)
	for i := range status.Posts {
		status.Posts[i].Images = append([]string{}, status.Posts[i].Images...)
	}
	return status
}

func (s *GroupsService) StartFacebookPageFetch(ctx context.Context, rawURL string, count int) (FacebookPageFetchStatus, error) {
	target, err := facebookPageURL(rawURL)
	if err != nil {
		return FacebookPageFetchStatus{}, err
	}
	return s.startFacebookPageFetch(ctx, target, count, isFacebookPostURL)
}

func (s *GroupsService) startFacebookPageFetch(ctx context.Context, target string, count int, match func(*url.URL) bool) (FacebookPageFetchStatus, error) {
	if count < 1 || int64(count) > 9007199254740991 {
		return FacebookPageFetchStatus{}, errors.New("Enter a positive whole number of posts to fetch")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return FacebookPageFetchStatus{}, errors.New("Postika is shutting down")
	}
	s.postMu.Lock()
	active := s.postingActiveLocked()
	s.postMu.Unlock()
	if active {
		return FacebookPageFetchStatus{}, errors.New("Wait for posting to finish before fetching Facebook posts")
	}
	if s.groupsLoad != nil {
		s.groupsLoad.mu.Lock()
		state := s.groupsLoad.status.State
		s.groupsLoad.mu.Unlock()
		if state == "waiting" || state == "loading" {
			return FacebookPageFetchStatus{}, errors.New("Finish loading groups before fetching Facebook posts")
		}
	}
	s.stopFacebookPageFetch()
	if err := s.ensureBrowserPage(ctx); err != nil {
		return FacebookPageFetchStatus{}, err
	}
	job, cancel := s.operationContext(context.Background())
	s.facebookPageRunID++
	load := &facebookPageLoad{cancel: cancel, done: make(chan struct{}), status: FacebookPageFetchStatus{RunID: s.facebookPageRunID, State: "opening", Message: "Opening Facebook…", Target: count, Posts: []FacebookPagePost{}}}
	s.facebookPageLoad = load
	// Arm before navigation so the first query cannot race the listener.
	page := s.page
	location := &timelineLocation{provided: target, current: target}
	capture := startTimelineCapture(job, page, location, match)
	go func() {
		defer close(load.done)
		defer cancel()
		defer capture.stop()
		posts, err := s.fetchFacebookPage(job, page, load, capture, location, target, count)
		if job.Err() != nil {
			load.set("cancelled", "Fetching stopped.", posts)
			return
		}
		if err != nil {
			load.set("error", err.Error(), posts)
			return
		}
		message := fmt.Sprintf("Fetched %d posts.", len(posts))
		if len(posts) < count {
			message = fmt.Sprintf("Fetched %d posts. The page has no more posts available.", len(posts))
		}
		load.set("complete", message, posts)
	}()
	return load.snapshot(), nil
}

func (s *GroupsService) GetFacebookPageFetchStatus() FacebookPageFetchStatus {
	s.mu.Lock()
	load := s.facebookPageLoad
	s.mu.Unlock()
	if load == nil {
		return FacebookPageFetchStatus{State: "idle", Posts: []FacebookPagePost{}}
	}
	return load.snapshot()
}

func (s *GroupsService) CancelFacebookPageFetch(runID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.facebookPageLoad != nil && s.facebookPageLoad.status.RunID == runID {
		s.stopFacebookPageFetch()
	}
}

// Caller holds s.mu. Cancellation stops navigation, capture, and Go HTTP replay.
func (s *GroupsService) stopFacebookPageFetch() {
	if s.facebookPageLoad != nil {
		state := s.facebookPageLoad.snapshot().State
		if state != "complete" && state != "error" && state != "cancelled" {
			s.facebookPageLoad.cancel()
			s.facebookPageLoad.set("cancelled", "Fetching stopped.", nil)
		}
	}
}

func (s *GroupsService) fetchFacebookPage(ctx context.Context, browserPage *rod.Page, load *facebookPageLoad, capture *timelineCapture, location *timelineLocation, target string, count int) ([]FacebookPagePost, error) {
	page := browserPage.Context(ctx)
	if err := page.Navigate(target); err != nil {
		return nil, errors.New("Could not open the Facebook page. Fetch again")
	}
	if _, err := page.Activate(); err != nil {
		return nil, errors.New("Could not activate the Facebook tab. Fetch again")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	wasBlocked := false
	scrolls := 0
	load.set("checking", "Checking Facebook login. Finish signing in or any verification prompts in Chrome; Postika will continue automatically.", nil)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-capture.results:
			if result.err != nil {
				return nil, result.err
			}
			load.set("capturing", "Timeline query captured. Opening a post photo to capture the full-size photo query…", nil)
			photoQuery, err := captureFacebookPhotoQuery(ctx, page, result.request, func(message string) { load.set("capturing", message, nil) })
			if err != nil {
				return nil, err
			}
			result.request.PhotoQuery = photoQuery
			cookies, err := (proto.NetworkGetCookies{Urls: []string{result.request.URL}}).Call(page)
			if err != nil || browserUser(cookies.Cookies) != result.request.User {
				return nil, errors.New("Could not preserve the current Facebook session. Fetch again")
			}
			// Snapshot before closing, as in the existing joined-groups flow.
			s.mu.Lock()
			if s.facebookPageLoad != load || ctx.Err() != nil {
				s.mu.Unlock()
				return nil, context.Canceled
			}
			s.closeBrowserConnection()
			s.mu.Unlock()
			load.set("loading", "Fresh timeline request captured. Fetching posts and full-size photos…", nil)
			return replayFacebookTimeline(ctx, result.request, cookies.Cookies, count, func(posts []FacebookPagePost) {
				load.set("loading", fmt.Sprintf("Fetched %d of %d posts…", len(posts), count), posts)
			})
		case <-ticker.C:
			if _, err := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(ctx)); err != nil {
				return nil, errors.New("The Facebook window was closed. Fetch again to reopen it")
			}
			login, err := page.Eval(facebookLoginSelectorScript)
			if err != nil {
				continue
			} // Login/navigation replaces the JS context.
			switch login.Value.Str() {
			case "logged_out", "login_required":
				wasBlocked = true
				scrolls = 0
				load.set(login.Value.Str(), "Finish signing in or any verification prompts in Chrome. Postika will detect your login and continue to the Facebook page automatically.", nil)
				continue
			case "logged_in":
				info, err := page.Info()
				if err != nil {
					continue
				}
				if wasBlocked || !location.matches(info.URL) {
					wasBlocked = false
					scrolls = 0
					load.set("opening", "Login detected. Opening the Facebook page…", nil)
					if err := page.Navigate(target); err != nil {
						return nil, errors.New("Could not continue to the Facebook page after login. Fetch again")
					}
					continue
				}
				load.set("capturing", "Scrolling the Facebook page to capture a fresh timeline request…", nil)
				if _, err := page.Eval(`() => { const main = document.querySelector('[role="main"]'); const pane = main && [main, ...main.querySelectorAll('div')].find(e => e.clientHeight > 200 && e.scrollHeight > e.clientHeight && ['auto','scroll'].includes(getComputedStyle(e).overflowY)); if (pane) pane.scrollBy(0, Math.max(600, pane.clientHeight)); window.scrollBy(0, Math.max(600, innerHeight)); }`); err == nil {
					scrolls++
				}
				if scrolls >= 60 {
					return nil, errors.New("No timeline pagination request appeared. Check that this Facebook page has visible posts, then fetch again")
				}
			default:
				load.set("checking", "Waiting for Facebook to finish loading and confirm your login…", nil)
			}
		}
	}
}
