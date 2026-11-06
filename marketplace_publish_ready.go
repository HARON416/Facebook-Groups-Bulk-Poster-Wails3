package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// Shared by listing types using the supplied second-step footer. This is a
// read-only check: resolving an enabled Publish control never dispatches input.
const marketplacePublishControl = `() => {
 const navigation=(` + marketplaceStructuralNavigation + `)();
 if(!navigation || navigation.step!==2)return null;
 const button=navigation.buttons[1];
 if(button.disabled || button.hasAttribute('disabled') || button.matches(':disabled') || button.closest('[aria-disabled="true"],[inert],[aria-busy="true"]'))return null;
 const visible=el=>el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"]') && getComputedStyle(el).visibility!=='hidden';
 if([...button.querySelectorAll('[data-visualcompletion="loading-state"],[aria-busy="true"]')].some(visible))return null;
 return button;
}`

const marketplacePublishReady = `() => Boolean((` + marketplacePublishControl + `)())`

func waitMarketplacePublishReady(ctx context.Context, page *rod.Page) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := page.Context(ctx).Wait(rod.Eval(marketplacePublishReady)); err != nil {
		return fmt.Errorf("Facebook's Publish button is not ready. Check validation and media processing in Chrome. Nothing was published: %w", err)
	}
	return nil
}
