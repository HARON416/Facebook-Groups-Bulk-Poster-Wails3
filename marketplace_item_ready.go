package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Reacquire Next on every check: Facebook may replace it as media uploads and
// required fields finish processing. Generated CSS classes are not stable.
const marketplaceItemNextControl = `() => {
 const navigation=(` + marketplaceStructuralNavigation + `)();
 if(!navigation || navigation.step!==1)return null;
 const button=navigation.buttons[0];
 return !button.disabled && !button.hasAttribute('disabled') && !button.closest('[aria-disabled="true"],[inert]')?button:null;
}`

const marketplaceItemNextReady = `() => Boolean((` + marketplaceItemNextControl + `)())`

func waitMarketplaceItemReady(ctx context.Context, page *rod.Page) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := page.Context(ctx).Wait(rod.Eval(marketplaceItemNextReady)); err != nil {
		return fmt.Errorf("Facebook's Next button is not ready. Check required fields and media processing: %w", err)
	}
	return nil
}

func clickMarketplaceItemNext(ctx context.Context, page *rod.Page) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	button, err := page.Context(ctx).ElementByJS(rod.Eval(marketplaceItemNextControl))
	if err != nil {
		return fmt.Errorf("Facebook's Next button is not ready. Check required fields and media processing: %w", err)
	}
	// Deliver one real mouse click, then stop. The following screen may have its
	// own Next button; automatically retrying a click could advance another step.
	if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("Could not click Facebook's Next button: %w", err)
	}
	return nil
}

// Facebook can change steps without changing the URL. Confirm the supplied
// second-step footer layout in the same visible main region, without depending on
// optional group rows or whether hiding the listing from friends is enabled.
const marketplaceItemSecondStepReady = `() => {
 const navigation=(` + marketplaceStructuralNavigation + `)();
 return !!navigation && navigation.step===2;
}`

func waitMarketplaceItemSecondStep(ctx context.Context, page *rod.Page) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := page.Context(ctx).Wait(rod.Eval(marketplaceItemSecondStepReady)); err != nil {
		return fmt.Errorf("Next was clicked, but Facebook did not confirm step 2. Check Chrome before trying again: %w", err)
	}
	return nil
}
