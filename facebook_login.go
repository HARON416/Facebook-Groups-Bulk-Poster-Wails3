package main

import (
	"context"
	"errors"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

var errFacebookLoginRequired = errors.New("Facebook login is required")
var errFacebookSessionExpired = errors.New("The active Facebook profile changed or expired")

const facebookLoginWaitMessage = "Checking Facebook login. Finish signing in or any verification prompts in Chrome; Postika will continue automatically."

// DOM-only detection: never inspect translated text, aria-labels, or credentials.
// An absent login form is not sufficient evidence of an authenticated session.
const facebookLoginSelectorScript = `() => {
  const has = selector => document.querySelector(selector) !== null;
  if (has('form[action*="/checkpoint"], #checkpointSubmitButton, input[name="approvals_code"], input[name="recovery_code"]')) {
    return 'login_required';
  }
  const loggedOut = has('body.UIPage_LoggedOut, #login_form, #login_popup_cta_form, [data-testid="royal_login_form"]') ||
    Array.from(document.querySelectorAll('form')).some(form =>
      form.querySelector('input[name="email"], input[data-testid="royal-email"]') &&
      form.querySelector('input[name="pass"][type="password"], input[data-testid="royal-pass"]')
    );
  if (loggedOut) return 'logged_out';
  if (has('body.UIPage_LoggedIn, [data-testid="blue_bar_profile_link"], [data-pagelet="CometSettingsDropdownTrigger"], form[action*="/logout"], a[href*="/logout.php"]')) {
    return 'logged_in';
  }
  // Modern Facebook's authenticated shell uses both notification and account
  // controls. Use their structural pagelets, never their localized aria-labels.
  if (has('[role="banner"] [data-pagelet="CometNotificationsDropdown"]') &&
      has('[role="banner"] [data-pagelet="CometProfileDropdown"]')) {
    return 'logged_in';
  }
  // Current Facebook omits those pagelets. Its account/navigation region has
  // an avatar dropdown alongside other interactive dropdowns. Require the
  // complete visible structure, not a feed avatar or a generic public header.
  const accountControls = [...document.querySelectorAll('[role="banner"] [role="navigation"] [role="button"][aria-haspopup="dialog"][aria-expanded][tabindex="0"]')]
    .filter(control => control.getClientRects().length && !control.closest('[hidden], [aria-hidden="true"]'));
  if (accountControls.length >= 3 && accountControls.some(control => control.querySelector('svg image, img'))) {
    return 'logged_in';
  }
  return 'checking';
}`

func facebookLoginBlocked(page *rod.Page) bool {
	login, err := page.Eval(facebookLoginSelectorScript)
	return err == nil && (login.Value.Str() == "logged_out" || login.Value.Str() == "login_required")
}

// Called after a login form or verification checkpoint is detected. Keep the
// browser available for the user and require positive evidence of login before
// resuming. Navigation can replace the JS context, so failed evaluations retry.
func waitForFacebookLogin(ctx context.Context, page *rod.Page, progress func(string)) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := postingCheckpoint(ctx); err != nil {
			return err
		}
		if _, err := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(ctx)); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.New("Posting stopped.")
		}
		login, err := page.Context(ctx).Eval(facebookLoginSelectorScript)
		if err == nil && login.Value.Str() == "logged_in" {
			return postingCheckpoint(ctx)
		}
		if progress != nil {
			progress(facebookLoginWaitMessage)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *GroupsService) GetLoginState(ctx context.Context) (string, error) {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.browser == nil && s.groupsLoad != nil {
		s.groupsLoad.mu.Lock()
		state := s.groupsLoad.status.State
		s.groupsLoad.mu.Unlock()
		if state == "loading" || state == "complete" || state == "error" {
			return "session_captured", nil
		}
	}
	if s.closed || s.browser == nil || s.page == nil {
		return "browser_closed", nil
	}
	if _, err := (proto.TargetGetTargetInfo{TargetID: s.page.TargetID}).Call(s.browser.Context(ctx)); err != nil {
		if ctx.Err() != nil {
			return "checking", nil
		}
		return "browser_closed", nil
	}
	result, err := s.page.Context(ctx).Eval(facebookLoginSelectorScript)
	if err != nil {
		// Navigation replaces the JavaScript context during login; retry on the
		// next poll rather than reporting a logout or an application error.
		return "checking", nil
	}
	return result.Value.Str(), nil
}
