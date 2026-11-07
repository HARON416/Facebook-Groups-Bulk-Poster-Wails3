package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Submitted means the Publish click was dispatched, not that Facebook has
// approved the listing. Unknown means dispatch may have reached Chrome.
type MarketplacePublishResult struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

func marketplacePublishTarget(media MarketplaceTestMedia) (string, error) {
	count := 0
	for _, present := range []bool{media.Fields != nil, media.VehicleFields != nil, media.HomeFields != nil} {
		if present {
			count++
		}
	}
	if count != 1 {
		return "", errors.New("Choose one listing type to publish")
	}
	if media.HomeFields != nil {
		return marketplaceTestHomeURL, validateMarketplaceHomeFields(media.HomeFields)
	}
	if media.VehicleFields != nil {
		return marketplaceTestVehicleURL, validateMarketplaceVehicleFields(media.VehicleFields)
	}
	return marketplaceTestItemURL, validateMarketplaceItemFields(media.Fields)
}

func (m *ProfileService) PublishMarketplaceListingMedia(ctx context.Context, profileID string, media MarketplaceTestMedia) MarketplacePublishResult {
	result := MarketplacePublishResult{}
	failed := func(err error) MarketplacePublishResult {
		return MarketplacePublishResult{State: "failed", Message: err.Error()}
	}
	target, err := marketplacePublishTarget(media)
	if err != nil {
		return failed(err)
	}
	s, err := m.session(profileID)
	if err != nil {
		return failed(err)
	}
	err = s.prepareMarketplaceListingMedia(ctx, target, media, func(active context.Context, page *rod.Page) error {
		var submitErr error
		result, submitErr = submitMarketplacePublish(active, page)
		return submitErr
	})
	if err != nil {
		return failed(err)
	}
	return result
}

func submitMarketplacePublish(ctx context.Context, page *rod.Page) (MarketplacePublishResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	page = page.Context(ctx)
	button, err := page.ElementByJS(rod.Eval(marketplacePublishControl))
	if err != nil {
		return MarketplacePublishResult{}, fmt.Errorf("Facebook's Publish button is not ready: %w", err)
	}
	if err := button.Hover(); err != nil {
		return MarketplacePublishResult{}, fmt.Errorf("Could not reach Facebook's Publish button: %w", err)
	}
	if err := button.WaitEnabled(); err != nil {
		return MarketplacePublishResult{}, err
	}
	current, err := button.Eval(`function(){return this===(` + marketplacePublishControl + `)()}`)
	if err != nil || !current.Value.Bool() {
		return MarketplacePublishResult{}, errors.New("Facebook's Publish button changed before clicking. Nothing was submitted")
	}
	if err := ctx.Err(); err != nil {
		return MarketplacePublishResult{}, err
	}
	// Dispatch exactly one click. A lost response after dispatch is ambiguous;
	// report that as an outcome so the UI does not offer an automatic retry.
	point := page.Mouse.Position()
	buttons := 1
	unknown := MarketplacePublishResult{State: "unknown"}
	pressErr := (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMousePressed, Button: proto.InputMouseButtonLeft, Buttons: &buttons, ClickCount: 1, X: point.X, Y: point.Y}).Call(page)
	// Release even after a lost press response: Chrome may have received it.
	release, stop := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer stop()
	buttons = 0
	if err := (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMouseReleased, Button: proto.InputMouseButtonLeft, Buttons: &buttons, ClickCount: 1, X: point.X, Y: point.Y}).Call(page.Context(release)); err != nil {
		return unknown, nil
	}
	if pressErr != nil {
		return unknown, nil
	}
	return MarketplacePublishResult{State: "submitted"}, nil
}
