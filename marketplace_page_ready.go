package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// Wait after login and any redirect, before interacting with any listing type.
// A loaded document can still be rendering Facebook's listing form.
func waitMarketplacePageReady(ctx context.Context, page *rod.Page) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	page = page.Context(ctx)
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait for Marketplace page to load: %w", err)
	}
	if err := page.WaitDOMStable(time.Second, 0); err != nil {
		return fmt.Errorf("wait for Marketplace page to settle before filling details: %w", err)
	}
	return nil
}
