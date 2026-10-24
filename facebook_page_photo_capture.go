package main

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Arm CDP before clicking; Facebook's client-side navigation can issue the
// viewer query immediately. Select photo links, excluding avatars and videos.
func captureFacebookPhotoQuery(ctx context.Context, page *rod.Page, timeline timelineRequest, progress func(string)) (*timelineRequest, error) {
	page = page.Context(ctx)
	endpoint, err := url.Parse(timeline.URL)
	if err != nil {
		return nil, errors.New("Could not prepare photo capture")
	}
	var target string
	for attempt := 0; attempt < 10; attempt++ {
		if facebookLoginBlocked(page) {
			return nil, errFacebookSessionExpired
		}
		result, err := page.Eval(`() => {
  const main = document.querySelector('[role="main"]') || document;
  for (const a of main.querySelectorAll('a[href]')) {
   if (!a.querySelector('img')) continue;
   const u = new URL(a.href, location.href);
   if (u.origin !== location.origin || !['/photo/','/photo.php'].includes(u.pathname) || !/^\d+$/.test(u.searchParams.get('fbid') || '')) continue;
   if (!/^pcb\.\d+$/.test(u.searchParams.get('set') || '') && !a.closest('[role=\"article\"]')) continue;
   return u.href;
  }
  return '';
 }`)
		if err == nil {
			target = result.Value.Str()
		}
		if target != "" {
			break
		}
		if attempt < 9 {
			// The captured timeline response may finish before React renders its cards.
			// Continue looking further down the feed if the visible posts have no photos.
			_, _ = page.Eval(`() => window.scrollBy(0, Math.max(400, innerHeight / 2))`)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	// A text-only timeline does not need a viewer template.
	if target == "" {
		return nil, nil
	}
	photoURL, err := url.Parse(target)
	if err != nil {
		return nil, errors.New("Could not identify the selected Facebook photo")
	}
	photoID := photoURL.Query().Get("fbid")
	captureCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	capture := startFacebookQueryCapture(captureCtx, page, nil, func(u *url.URL) bool {
		return u.Scheme == endpoint.Scheme && u.Host == endpoint.Host && u.Path == endpoint.Path
	}, photoID, timeline.User, photoURL.Query().Get("set"))
	defer capture.stop()
	_, err = page.Context(captureCtx).Eval(`(target) => {
  const main = document.querySelector('[role="main"]') || document;
  const link = [...main.querySelectorAll('a[href]')].find(a => a.href === target && a.querySelector('img'));
  if (!link) throw new Error('Photo link disappeared');
  link.click();
 }`, target)
	if err != nil {
		return nil, errors.New("Could not open the post photo in Chrome. Fetch again")
	}
	// Facebook may satisfy the first click from Relay's in-memory cache.
	// Reload once with the listener already armed to request fresh viewer data.
	reloadTimer := time.NewTimer(4 * time.Second)
	defer reloadTimer.Stop()
	loginTicker := time.NewTicker(500 * time.Millisecond)
	defer loginTicker.Stop()
	reloaded := false
	for {
		select {
		case <-loginTicker.C:
			if facebookLoginBlocked(page.Context(ctx)) {
				return nil, errFacebookSessionExpired
			}
		case <-reloadTimer.C:
			if facebookLoginBlocked(page.Context(ctx)) {
				return nil, errFacebookSessionExpired
			}
			select {
			case result := <-capture.results:
				if result.err != nil {
					return nil, result.err
				}
				return &result.request, nil
			default:
			}
			if reloaded {
				if progress != nil {
					progress("No separate photo query appeared after reload. Opening the next photo to capture its viewer query…")
				}
				// Prefer Facebook's own gallery control so its Relay router
				// makes a client-side request instead of another HTML load.
				clicked, clickErr := page.Context(captureCtx).Eval(`() => {
                 const next = document.querySelector('[role="button"][aria-label="Next photo"],button[aria-label="Next photo"],a[aria-label="Next photo"]');
                 if (!next) return false;
                 next.click(); return true;
                }`)
				if clickErr != nil || !clicked.Value.Bool() {
					// Facebook supports gallery keyboard navigation regardless
					// of the language used for the visible button label.
					_, _ = page.Context(captureCtx).Eval(`() => { if (document.activeElement instanceof HTMLElement) document.activeElement.blur(); }`)
					_ = page.Context(captureCtx).Keyboard.Type(input.ArrowRight)
				}
				continue
			}
			reloaded = true
			if progress != nil {
				progress("No photo query appeared after opening the image. Reloading the photo page to capture fresh values…")
			}
			info, infoErr := page.Context(captureCtx).Info()
			var current *url.URL
			var parseErr error
			if infoErr == nil {
				current, parseErr = url.Parse(info.URL)
			}
			if infoErr == nil && parseErr == nil && current.Scheme == photoURL.Scheme && current.Host == photoURL.Host && current.Path == photoURL.Path && current.Query().Get("fbid") == photoID {
				err = (proto.PageReload{IgnoreCache: true}).Call(page.Context(captureCtx))
			} else {
				err = page.Context(captureCtx).Navigate(target)
			}
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, errors.New("Could not reload the Facebook photo page. Fetch again")
			}
			reloadTimer.Reset(4 * time.Second)
		case <-captureCtx.Done():
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if facebookLoginBlocked(page.Context(ctx)) {
				return nil, errFacebookSessionExpired
			}
			return nil, errors.New("No fresh photo-viewer query appeared after reloading the photo page. Check Chrome and fetch again")
		case result := <-capture.results:
			if facebookLoginBlocked(page.Context(ctx)) {
				return nil, errFacebookSessionExpired
			}
			if result.err != nil {
				return nil, result.err
			}
			return &result.request, nil
		}
	}
}
