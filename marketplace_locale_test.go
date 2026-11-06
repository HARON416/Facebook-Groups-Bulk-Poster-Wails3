package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

// Pseudolocalize every field/navigation label used by the former selectors.
// The fixture also changes generated option IDs and reverses their DOM order:
// the captured numeric option suffix, rather than the translated caption, wins.
func marketplaceLocalizedFixture(prefix, direction string) string {
	labels := []string{"List as Single Item", "List as In Stock", "Progress, currently on step 1 of 2", "List in more places", "Promote listing after publish", "Hide from friends", "Product tags", "More details", "Public meetup", "Door pickup", "Door dropoff", "Dropdown menu", "Select an option", "Used - Like New", "Used - Good", "Used - Fair", "Description", "Availability", "Condition", "Category", "Household", "Location", "Previous", "Publish", "Title", "Price", "Next", "Tools", "New", "SKU"}
	pairs := make([]string, 0, len(labels)*2)
	for i, label := range labels {
		pairs = append(pairs, label, prefix+strings.Repeat("字", i+1))
	}
	fixture := strings.NewReplacer(pairs...).Replace(marketplaceItemFieldsFixture)
	fixture = strings.Replace(fixture, "for(const label of choices[key]){", "for(const [index,label] of choices[key].entries()){", 1)
	fixture = strings.Replace(fixture, "option.tabIndex=0;", "option.tabIndex=0;if(key!=='category')option.id='ephemeral_'+Math.random().toString(36).slice(2)+'___'+index;", 1)
	fixture = strings.Replace(fixture, "menu.append(option)", "if(key==='category')menu.append(option);else menu.prepend(option)", 1)
	return `<html lang="und" dir="` + direction + `">` + fixture + `</html>`
}

func TestMarketplaceLocalizedBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	for _, locale := range []struct{ prefix, direction string }{{"É", "ltr"}, {"ع", "rtl"}, {"日", "ltr"}} {
		t.Run(locale.prefix, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(marketplaceLocalizedFixture(locale.prefix, locale.direction)))
			}))
			defer server.Close()
			s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
			t.Cleanup(func() { _ = s.shutdown(true) })
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fields := MarketplaceItemFields{Title: "Lampe مصباح", Price: "29999", Category: "household", Condition: "used-good", Description: "Description utilisateur", Availability: "in-stock", Tags: []string{"bureau", "منزل"}, SKU: "SKU-123", Location: &MarketplaceLocationHit{PageID: "selected", Label: "Vancouver, BC, Canada", Subtitle: "City"}, PublicMeetup: true, DoorPickup: true, DoorDropoff: true, PromoteAfterPublish: true, HideFromFriends: true}
			if err := s.testMarketplaceItemMedia(ctx, server.URL, MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}); err != nil {
				t.Fatal(err)
			}
			result, err := s.marketplaceTestPage.Context(ctx).Eval(`() => window.values`)
			if err != nil {
				t.Fatal(err)
			}
			v := result.Value
			if v.Get("title").Str() != fields.Title || v.Get("price").Str() != fields.Price || v.Get("description").Str() != fields.Description || v.Get("sku").Str() != fields.SKU || v.Get("location").Str() != "City" || len(v.Get("tags").Arr()) != 2 || !v.Get("pickup").Bool() || !v.Get("dropoff").Bool() || !v.Get("promote").Bool() || !v.Get("hide").Bool() || v.Get("nextClicks").Int() != 1 || v.Get("published").Bool() {
				t.Fatalf("Localized flow did not retain fields or stop at step two: %v", v)
			}
			for _, key := range []string{"category", "condition", "availability"} {
				if !strings.HasPrefix(v.Get(key).Str(), locale.prefix) {
					t.Fatalf("Selection %s was not localized: %v", key, v)
				}
			}
			// Check exact canonical slots despite reversed option DOM order.
			if v.Get("condition").Str() != locale.prefix+strings.Repeat("字", 15) || v.Get("availability").Str() != locale.prefix+strings.Repeat("字", 2) || v.Get("category").Str() != locale.prefix+strings.Repeat("字", 21) {
				t.Fatalf("Wrong canonical option selected: %v", v)
			}
			page := s.marketplaceTestPage.Context(ctx)
			if _, err := page.Eval(`() => {document.querySelector('[role=form]').hidden=false;dropdown('condition',document.querySelector('label[aria-required=true]'))}`); err != nil {
				t.Fatal(err)
			}
			if ready, err := page.Eval(marketplaceItemSecondStepReady); err != nil || ready.Value.Bool() {
				t.Fatalf("Visible required fields incorrectly accepted as step two: %v, %v", ready, err)
			}
			if _, err := page.Element(`[data-active-menu]`); err != nil {
				t.Fatal(err)
			}
			if _, err := page.Eval(`() => {document.querySelectorAll('[data-active-menu] [role=option]').forEach(el=>el.id='changed___0')}`); err != nil {
				t.Fatal(err)
			}
			if _, err := page.Eval(marketplaceDropdownOption, "Condition", 2, 4); err == nil {
				t.Fatal("Duplicate option indices were accepted")
			}
			if _, err := page.Eval(`() => document.querySelector('[data-active-menu] [role=option]').remove()`); err != nil {
				t.Fatal(err)
			}
			if _, err := page.Eval(marketplaceDropdownOption, "Condition", 2, 4); err == nil {
				t.Fatal("Changed option count was accepted")
			}
		})
	}
}
