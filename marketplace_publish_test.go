package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplacePublishTarget(t *testing.T) {
	item := MarketplaceItemFields{Title: "Desk lamp", Price: "20", Category: "tools", Condition: "new"}
	vehicle := validMarketplaceVehicle()
	sale, rent, room := validMarketplaceHome(), validMarketplaceHome(), validMarketplaceHome()
	rent.HomeOffer = "rent"
	room.HomeOffer, room.PrivateRoom, room.Occupants, room.BathroomType = "rent", true, "2", "shared"
	for _, tc := range []struct {
		name  string
		media MarketplaceTestMedia
		url   string
	}{
		{"item", MarketplaceTestMedia{Fields: &item}, marketplaceTestItemURL},
		{"vehicle", MarketplaceTestMedia{VehicleFields: &vehicle}, marketplaceTestVehicleURL},
		{"sale", MarketplaceTestMedia{HomeFields: &sale}, marketplaceTestHomeURL},
		{"rent", MarketplaceTestMedia{HomeFields: &rent}, marketplaceTestHomeURL},
		{"room", MarketplaceTestMedia{HomeFields: &room}, marketplaceTestHomeURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			url, err := marketplacePublishTarget(tc.media)
			if err != nil || url != tc.url {
				t.Fatalf("Wrong publish destination: %s, %v", url, err)
			}
		})
	}
	for _, media := range []MarketplaceTestMedia{
		{}, {Fields: &item, HomeFields: &sale}, {Fields: &MarketplaceItemFields{}},
		{VehicleFields: &MarketplaceVehicleFields{}}, {HomeFields: &MarketplaceHomeFields{}},
	} {
		// Invalid input must fail before accessing an account or opening Chrome.
		result := (&ProfileService{}).PublishMarketplaceListingMedia(context.Background(), "", media)
		if result.State != "failed" || result.Message == "" {
			t.Fatalf("Invalid publish accepted: %+v", result)
		}
	}
}

func TestMarketplacePublishBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(marketplaceItemFieldsFixture))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fields := MarketplaceItemFields{Title: "Publish fixture", Price: "20", Category: "tools", Condition: "new"}
	media := MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}
	err := service.prepareMarketplaceListingMedia(ctx, server.URL, media, func(ctx context.Context, page *rod.Page) error {
		// Facebook may replace Publish immediately after a click. Never click the replacement.
		_, err := page.Context(ctx).Eval(`()=>{window.publishClicks=0;const button=document.querySelector('#fixture-actions button:last-child');button.onclick=()=>{publishClicks++;button.replaceWith(button.cloneNode(true))};const decoy=document.createElement('button');decoy.textContent='Publish';decoy.onclick=()=>publishClicks+=100;document.body.prepend(decoy)}`)
		if err != nil {
			return err
		}
		result, err := submitMarketplacePublish(ctx, page)
		if err == nil && result.State != "submitted" {
			t.Fatalf("Unexpected outcome: %+v", result)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	page := service.marketplaceTestPage.Context(ctx)
	result, err := page.Eval(`()=>({clicks:publishClicks,next:values.nextClicks,photos:values.photos})`)
	if err != nil || result.Value.Get("clicks").Int() != 1 || result.Value.Get("next").Int() != 1 || result.Value.Get("photos").Int() != 1 {
		t.Fatalf("Did not prepare and publish exactly once: %v, %v", result, err)
	}
	for _, setup := range []string{
		`document.querySelector('#fixture-actions button:last-child').setAttribute('aria-disabled','true')`,
		`document.querySelector('#fixture-actions button:last-child').removeAttribute('aria-disabled');document.querySelector('#fixture-actions').append(document.createElement('button'))`,
	} {
		if _, err := page.Eval(`()=>{` + setup + `}`); err != nil {
			t.Fatal(err)
		}
		short, stop := context.WithTimeout(ctx, 150*time.Millisecond)
		_, err := submitMarketplacePublish(short, page)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Unready/ambiguous Publish accepted: %v", err)
		}
	}
	stopped, stop := context.WithCancel(ctx)
	stop()
	if _, err := submitMarketplacePublish(stopped, page); !errors.Is(err, context.Canceled) {
		t.Fatalf("Cancellation ignored: %v", err)
	}
	result, err = page.Eval(`()=>publishClicks`)
	if err != nil || result.Value.Int() != 1 {
		t.Fatalf("Unexpected extra publish: %v, %v", result, err)
	}
}
