package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Match the supplied control's structure without translated text or generated classes.
const groupsSortSelector = `div[role="button"][aria-haspopup="menu"][aria-expanded]:has(> span):has(> div[role="none"][data-visualcompletion="ignore"])`

// OpenGroupsSortMenu retries while the page loads and opens the menu once per
// Load groups navigation. It leaves choosing a sort option to the next step.
func (s *GroupsService) OpenGroupsSortMenu(ctx context.Context) (bool, error) {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.browser == nil || s.page == nil {
		return false, nil
	}
	if s.sortMenuOpened {
		return true, nil
	}
	page := s.page.Context(ctx)
	info, err := page.Info()
	if err != nil {
		return false, nil // The tab may be navigating or closed.
	}
	current, err := url.Parse(info.URL)
	if err != nil {
		return false, nil
	}
	target := joinedGroupsURL
	if s.targetURL != "" {
		target = s.targetURL
	}
	expected, err := url.Parse(target)
	if err != nil || current.Host != expected.Host || strings.TrimRight(current.Path, "/") != strings.TrimRight(expected.Path, "/") {
		return false, nil
	}
	login, err := page.Eval(facebookLoginSelectorScript)
	if err != nil || login.Value.Str() == "logged_out" || login.Value.Str() == "login_required" {
		return false, nil
	}
	selector := groupsSortSelector
	if hasMain, _, err := page.Has(`[role="main"]`); err != nil {
		return false, nil
	} else if hasMain {
		selector = `[role="main"] ` + selector
	}
	elements, err := page.Elements(selector)
	if err != nil {
		return false, nil
	}
	var visible []*rod.Element
	for _, element := range elements {
		if shown, err := element.Visible(); err == nil && shown {
			visible = append(visible, element)
		}
	}
	if len(visible) == 0 {
		return false, nil
	}
	if len(visible) != 1 {
		return false, fmt.Errorf("Found multiple matching menus on the groups page. Please open the Sort menu in Chrome")
	}
	expanded, err := visible[0].Attribute("aria-expanded")
	if err != nil {
		return false, nil
	}
	if expanded == nil || *expanded != "true" {
		if err := visible[0].Click(proto.InputMouseButtonLeft, 1); err != nil {
			return false, nil
		}
		expanded, err = visible[0].Attribute("aria-expanded")
		if err != nil || expanded == nil || *expanded != "true" {
			return false, nil
		}
	}
	s.sortMenuOpened = true
	return true, nil
}

// SelectUnselectedGroupsSortOption chooses the first visible, enabled radio
// option whose explicit checked state is false. Complete once per navigation.
func (s *GroupsService) SelectUnselectedGroupsSortOption(ctx context.Context) (bool, error) {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.page == nil || s.browser == nil || !s.sortMenuOpened {
		return false, nil
	}
	if s.sortOptionSelected {
		return true, nil
	}
	page := s.page.Context(ctx)
	info, err := page.Info()
	if err != nil {
		return false, nil
	}
	target := joinedGroupsURL
	if s.targetURL != "" {
		target = s.targetURL
	}
	current, currentErr := url.Parse(info.URL)
	expected, targetErr := url.Parse(target)
	if currentErr != nil || targetErr != nil || current.Host != expected.Host || strings.TrimRight(current.Path, "/") != strings.TrimRight(expected.Path, "/") {
		return false, nil
	}
	login, err := page.Eval(facebookLoginSelectorScript)
	if err != nil || login.Value.Str() == "logged_out" || login.Value.Str() == "login_required" {
		return false, nil
	}
	// Facebook renders the menu outside main. A checked radio item distinguishes
	// the sort menu from ordinary action menus without using its localized label.
	menus, err := page.Elements(`[role="menu"]:has([role="menuitemradio"][aria-checked="true"]):has([role="menuitemradio"][aria-checked="false"])`)
	if err != nil {
		return false, nil
	}
	var visible []*rod.Element
	for _, menu := range menus {
		if shown, err := menu.Visible(); err == nil && shown {
			visible = append(visible, menu)
		}
	}
	if len(visible) == 0 {
		return false, nil // Retry when the opened menu finishes rendering.
	}
	if len(visible) != 1 {
		return false, fmt.Errorf("Found multiple matching radio menus. Please choose a sort option in Chrome")
	}
	options, err := visible[0].Elements(`[role="menuitemradio"][aria-checked="false"]:not([aria-disabled="true"]):not([disabled])`)
	if err != nil {
		return false, nil
	}
	for _, option := range options {
		if shown, err := option.Visible(); err != nil || !shown {
			continue
		}
		s.startGroupsCapture()
		if err := option.Click(proto.InputMouseButtonLeft, 1); err != nil {
			s.stopGroupsCapture()
			return false, nil
		}
		// Selection may navigate or unmount the menu, so do not inspect it again.
		s.sortOptionSelected = true
		return true, nil
	}
	return false, nil
}
