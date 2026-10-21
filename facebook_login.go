package main

import (
	"context"

	"github.com/go-rod/rod/lib/proto"
)

// DOM-only detection: never inspect translated text, aria-labels, or credentials.
// An absent login form is not sufficient evidence of an authenticated session.
const facebookLoginSelectorScript = `() => {
  const has = selector => document.querySelector(selector) !== null;
  if (has('form[action*="/checkpoint"], #checkpointSubmitButton, input[name="approvals_code"], input[name="recovery_code"]')) {
    return 'login_required';
  }
  const loggedOut = has('body.UIPage_LoggedOut, #login_form, [data-testid="royal_login_form"]') ||
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
  return 'checking';
}`

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
