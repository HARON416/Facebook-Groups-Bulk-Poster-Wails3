package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const marketplaceDetailsSelector = `div[role="button"][tabindex="0"][aria-expanded]:not([aria-haspopup]):has(span[id]):has(i[data-visualcompletion="css-img"]):has(> div[role="none"][data-visualcompletion="ignore"])`

var errMarketplaceDetails = errors.New("Could not expand More details in Facebook")

type marketplaceDetailsAttempt struct {
	deadline  time.Time
	nextClick time.Time
	node      proto.DOMBackendNodeID
}

func (state *marketplaceDetailsAttempt) pending(reason string) (bool, error) {
	if !time.Now().Before(state.deadline) {
		return false, fmt.Errorf("%w: %s", errMarketplaceDetails, reason)
	}
	return false, nil
}

// Poll independently of login waiting. A successful mouse dispatch does not
// prove React handled the click: retry a still-collapsed control after a grace
// period, and confirm its live expanded state immediately before each click.
func expandMarketplaceDetails(ctx context.Context, browserPage *rod.Page, state *marketplaceDetailsAttempt) (bool, error) {
	if state.deadline.IsZero() {
		state.deadline = time.Now().Add(30 * time.Second)
	}
	attempt, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	page := browserPage.Context(attempt)
	elements, err := page.Elements(marketplaceDetailsSelector)
	if err != nil {
		return state.pending("could not inspect the item composer")
	}
	defer func() {
		for _, el := range elements {
			_ = el.Context(ctx).Release()
		}
	}()
	type candidate struct {
		el                *rod.Element
		sidebar, composer bool
	}
	var candidates []candidate
	for _, el := range elements {
		shown, err := el.Visible()
		if err != nil || !shown {
			continue
		}
		info, err := el.Eval(`function(){return {
   hidden:!!this.closest('[hidden],[aria-hidden="true"],header,nav,[role="banner"],[role="navigation"]'),
   sidebar:!!this.closest('[role="complementary"]'),
   composer:!!this.closest('main,[role="main"]')
  }}`)
		if err == nil && !info.Value.Get("hidden").Bool() {
			candidates = append(candidates, candidate{el, info.Value.Get("sidebar").Bool(), info.Value.Get("composer").Bool()})
		}
	}
	// Prefer item-main controls, then complementary composer controls. An
	// unrelated complementary region must not hide the real control elsewhere.
	var composer, sidebar []candidate
	for _, candidate := range candidates {
		if candidate.composer {
			composer = append(composer, candidate)
		}
		if candidate.sidebar {
			sidebar = append(sidebar, candidate)
		}
	}
	if len(composer) > 0 {
		candidates = composer
	} else if len(sidebar) > 0 {
		candidates = sidebar
	}
	if len(candidates) == 0 {
		return state.pending("no visible More details control was found")
	}
	if len(candidates) > 1 {
		return false, fmt.Errorf("%w: multiple matching controls", errMarketplaceDetails)
	}
	el := candidates[0].el
	expanded, err := el.Attribute("aria-expanded")
	if err != nil {
		return state.pending("could not read the accordion state")
	}
	if expanded != nil && *expanded == "true" {
		return true, nil
	}
	if !time.Now().Before(state.deadline) {
		return state.pending("the control stayed collapsed")
	}
	disabled, err := el.Attribute("aria-disabled")
	if err != nil || disabled != nil && *disabled == "true" {
		return state.pending("the control is disabled")
	}
	node, err := el.Describe(0, false)
	if err != nil {
		return state.pending("the control is still rendering")
	}
	if node.BackendNodeID == state.node && time.Now().Before(state.nextClick) {
		return state.pending("waiting for expansion after clicking")
	}
	if facebookLoginBlocked(page) {
		return false, nil
	}
	// Use real mouse input, including scrolling and checking for overlays.
	if err := el.Hover(); err != nil {
		return state.pending("the control is covered or not yet clickable")
	}
	if err := el.WaitEnabled(); err != nil {
		return state.pending("the control is not yet enabled")
	}
	expanded, err = el.Attribute("aria-expanded")
	if err != nil {
		return state.pending("the control changed during preparation")
	}
	if expanded != nil && *expanded == "true" {
		return true, nil
	}
	if facebookLoginBlocked(page) {
		return false, nil
	}
	if err := page.Mouse.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return state.pending("mouse click failed")
	}
	state.node = node.BackendNodeID
	state.nextClick = time.Now().Add(3 * time.Second)
	expanded, err = el.Attribute("aria-expanded")
	return err == nil && expanded != nil && *expanded == "true", nil
}
