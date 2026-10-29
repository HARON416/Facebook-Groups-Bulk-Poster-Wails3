package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

type MarketplaceItemFields struct {
	Title               string                  `json:"title"`
	Price               string                  `json:"price"`
	Category            string                  `json:"category"`
	Condition           string                  `json:"condition"`
	Description         string                  `json:"description"`
	Availability        string                  `json:"availability"`
	Tags                []string                `json:"tags"`
	SKU                 string                  `json:"sku"`
	Location            *MarketplaceLocationHit `json:"location"`
	PublicMeetup        bool                    `json:"publicMeetup"`
	DoorPickup          bool                    `json:"doorPickup"`
	DoorDropoff         bool                    `json:"doorDropoff"`
	PromoteAfterPublish bool                    `json:"promoteAfterPublish"`
	HideFromFriends     bool                    `json:"hideFromFriends"`
}

// These are the owner-supplied Facebook labels, independent of the app locale.
var marketplaceItemCategories = map[string]string{
	"tools": "Tools", "furniture": "Furniture", "household": "Household", "garden": "Garden", "appliances": "Appliances",
	"video-games": "Video Games", "books-movies-music": "Books, Movies & Music", "bags-luggage": "Bags & Luggage",
	"womens-clothing-shoes": "Women's clothing & shoes", "mens-clothing-shoes": "Men's clothing & shoes", "jewelry-accessories": "Jewelry & Accessories",
	"health-beauty": "Health & beauty", "pet-supplies": "Pet Supplies", "baby-kids": "Baby & kids", "toys-games": "Toys & Games",
	"electronics-computers": "Electronics & computers", "mobile-phones": "Mobile phones", "bicycles": "Bicycles", "arts-crafts": "Arts & Crafts",
	"sports-outdoors": "Sports & Outdoors", "auto-parts": "Auto parts", "musical-instruments": "Musical Instruments",
	"antiques-collectibles": "Antiques & Collectibles", "garage-sale": "Garage Sale", "miscellaneous": "Miscellaneous",
}
var marketplaceItemConditions = map[string]string{"new": "New", "used-like-new": "Used - Like New", "used-good": "Used - Good", "used-fair": "Used - Fair"}
var marketplaceItemPrice = regexp.MustCompile(`^[0-9]+$`)
var marketplaceDisplayedPrice = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?$`)

func validateMarketplaceItemFields(fields *MarketplaceItemFields) error {
	if fields == nil || strings.TrimSpace(fields.Title) == "" || !marketplaceItemPrice.MatchString(fields.Price) || marketplaceItemCategories[fields.Category] == "" || marketplaceItemConditions[fields.Condition] == "" {
		return errors.New("Enter a title, valid price, category, and condition")
	}
	if fields.Availability != "" && fields.Availability != "single" && fields.Availability != "in-stock" {
		return errors.New("Choose a valid availability")
	}
	if len(fields.Tags) > 20 {
		return errors.New("You may only add a maximum of 20 tags")
	}
	if fields.Location != nil && (fields.Location.PageID == "" || strings.TrimSpace(fields.Location.Label) == "") {
		return errors.New("Please enter a valid location")
	}
	return nil
}

// Resolve the supplied field labels through native labels / aria-labelledby,
// never copied React IDs or generated class names. Only the listing form is used.
const marketplaceNamedControl = `(name, selector) => {
 const norm = value => (value || '').replace(/\s+/g,' ').trim().toLocaleLowerCase();
 const matches = [...document.querySelectorAll(selector)].filter(el => {
   if (!el.closest('main,[role="main"],[role="form"]') || !el.getClientRects().length || el.closest('[hidden],[aria-hidden="true"]')) return false;
   if (norm(el.getAttribute('aria-label')) === norm(name)) return true;
   if ((el.getAttribute('aria-labelledby') || '').split(/\s+/).some(id => norm(document.getElementById(id)?.textContent) === norm(name))) return true;
   const labels = [...(el.labels || [])]; const ancestor = el.closest('label'); if (ancestor) labels.push(ancestor);
   if (labels.some(label => norm(label.textContent) === norm(name) || [...label.querySelectorAll('span')].some(s => norm(s.textContent) === norm(name)))) return true;
   return (el.matches('[role="checkbox"],[role="switch"],[role="button"]') && [...el.querySelectorAll('span')].some(s => norm(s.textContent) === norm(name)));
 });
 if(matches.length>1) throw new Error('Multiple matching controls: '+name);
 return matches[0] || null;
}`

func marketplaceField(ctx context.Context, page *rod.Page, name, selector string) (*rod.Element, error) {
	control, err := page.Context(ctx).ElementByJS(rod.Eval(marketplaceNamedControl, name, selector))
	if err != nil {
		return nil, fmt.Errorf("Could not find Facebook field %s: %w", name, err)
	}
	return control, nil
}

func marketplaceText(ctx context.Context, page *rod.Page, name, value string) error {
	field, err := marketplaceField(ctx, page, name, `input:not([type="file"]),textarea`)
	if err != nil {
		return err
	}
	if err = field.SelectAllText(); err != nil {
		return err
	}
	if err = field.Input(value); err != nil {
		return fmt.Errorf("Could not fill %s: %w", name, err)
	}
	// React can replace/reformat the control after input. Read the live field,
	// allowing currency/group separators without accepting a different amount.
	verify, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		live, findErr := marketplaceField(verify, page, name, `input:not([type="file"]),textarea`)
		if findErr == nil {
			actual, readErr := live.Property("value")
			if readErr == nil && (actual.Str() == value || name == "Price" && marketplacePriceEqual(actual.Str(), value)) {
				return nil
			}
		}
		select {
		case <-verify.Done():
			return fmt.Errorf("Facebook did not retain %s", name)
		case <-ticker.C:
		}
	}
}

func marketplacePriceEqual(display, requested string) bool {
	wanted, ok := new(big.Rat).SetString(requested)
	if !ok || !marketplaceItemPrice.MatchString(requested) {
		return false
	}
	display = strings.TrimSpace(display)
	if strings.EqualFold(display, "Free") {
		return wanted.Sign() == 0
	}
	first, last := -1, -1
	for i, r := range display {
		if r >= '0' && r <= '9' {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return false
	}
	for _, affix := range []string{display[:first], display[last+1:]} {
		for _, r := range affix {
			if !unicode.IsSpace(r) && !unicode.IsLetter(r) && !unicode.Is(unicode.Sc, r) {
				return false
			}
		}
	}
	var number strings.Builder
	for _, r := range display[first : last+1] {
		if r >= '0' && r <= '9' || r == '.' || r == ',' {
			number.WriteRune(r)
		} else if !unicode.IsSpace(r) {
			return false
		}
	}
	text := number.String()
	integer, fraction := text, ""
	if i := strings.LastIndexAny(text, ".,"); i >= 0 && len(text)-i-1 <= 2 {
		integer, fraction = text[:i], text[i+1:]
		if strings.ContainsRune(integer, rune(text[i])) {
			return false
		}
	}
	if strings.ContainsAny(integer, ".,") {
		if strings.Contains(integer, ".") && strings.Contains(integer, ",") {
			return false
		}
		groups := strings.FieldsFunc(integer, func(r rune) bool { return r == '.' || r == ',' })
		if len(groups) < 2 || len(groups[0]) < 1 || len(groups[0]) > 3 {
			return false
		}
		for _, group := range groups[1:] {
			if len(group) != 3 {
				return false
			}
		}
		// FieldsFunc drops empty groups; reject consecutive separators explicitly.
		if strings.Contains(integer, "..") || strings.Contains(integer, ",,") || strings.HasSuffix(integer, ".") || strings.HasSuffix(integer, ",") {
			return false
		}
		integer = strings.Join(groups, "")
	}
	canonical := integer
	if fraction != "" {
		canonical += "." + fraction
	}
	if !marketplaceDisplayedPrice.MatchString(canonical) {
		return false
	}
	actual, ok := new(big.Rat).SetString(canonical)
	return ok && actual.Cmp(wanted) == 0
}

const marketplaceLocationPopup = `() => {
 const visible = el => el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"]');
 const control = (` + marketplaceNamedControl + `)('Location','input[role="combobox"]');
 if(!control) return null;
 const ids = ((control.getAttribute('aria-controls')||'')+' '+(control.getAttribute('aria-owns')||'')).trim().split(/\s+/).filter(Boolean);
 if(ids.length) return ids.map(id=>document.getElementById(id)).find(el=>el && visible(el)) || null;
 const active = document.getElementById(control.getAttribute('aria-activedescendant')||'')?.closest('[role="listbox"]');
 if(active && visible(active)) return active;
 const roots = [...document.querySelectorAll('[role="listbox"]')].filter(visible);
 let menus = roots.filter(el=>el.tagName==='UL');
 if(!menus.length) menus=roots.filter(el=>!el.getAttribute('aria-label') || /suggested search/i.test(el.getAttribute('aria-label')));
 if(menus.length>1) throw new Error('Multiple open location suggestion lists');
 return menus[0] || null;
}`

const marketplaceOption = `(label, subtitle) => {
 const norm = value => (value || '').replace(/\s+/g,' ').trim().toLocaleLowerCase();
 const menu = (` + marketplaceLocationPopup + `)();
 if(!menu) return null;
 // Facebook nests a role=option div inside each role=option li. Keep one
 // top-level row, rather than treating both elements as separate results.
 const options = [...menu.querySelectorAll('[role="option"]')].filter(el => {
   const parent = el.parentElement?.closest('[role="option"]');
   return (!parent || !menu.contains(parent)) && el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"]');
 });
 const primary = el => el.querySelector('span[dir="auto"]')?.textContent || el.querySelector('span')?.textContent || el.getAttribute('aria-label') || el.textContent;
 let matches = options.filter(el=>norm(primary(el))===norm(label));
 if(subtitle) matches = matches.filter(el=>norm(el.textContent).includes(norm(subtitle)));
 if(matches.length>1) throw new Error('Multiple matching options: '+label);
 return matches[0] || null;
}`

const marketplaceLocationSelected = `(label) => {
 const control = (` + marketplaceNamedControl + `)('Location','input[role="combobox"]');
 const norm = value => (value || '').replace(/\s+/g,' ').trim().toLocaleLowerCase();
 return !!control && norm(control.value)===norm(label) && control.getAttribute('aria-expanded')!=='true' && !(` + marketplaceLocationPopup + `)();
}`

const marketplaceDropdownOption = `(name, label) => {
 const norm = value => (value || '').replace(/\s+/g,' ').trim().toLocaleLowerCase();
 const visible = el => el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"]');
 const control = (` + marketplaceNamedControl + `)(name, '[role="combobox"]');
 if(!control) return null;
 const linkedIDs = ((control.getAttribute('aria-controls')||'')+' '+(control.getAttribute('aria-owns')||'')).trim().split(/\s+/).filter(Boolean);
 const linked = linkedIDs.map(id=>document.getElementById(id)).filter(el=>el && visible(el));
 if(linkedIDs.length && !linked.length) return null;
 let menus = linked;
 if(!menus.length) {
   const all = [...document.querySelectorAll('[role="listbox"],[role="dialog"][aria-label="Dropdown menu"]')].filter(visible);
   const named = all.filter(el => norm(el.getAttribute('aria-label'))===norm(name) || (el.getAttribute('aria-labelledby')||'').split(/\s+/).some(id=>norm(document.getElementById(id)?.textContent)===norm(name)));
   if(named.length) menus = named;
   else if(name==='Category') menus = all.filter(el=>el.getAttribute('role')==='dialog');
   else menus = all.filter(el=>el.getAttribute('role')==='listbox' && (!el.getAttribute('aria-label') || norm(el.getAttribute('aria-label'))==='select an option'));
 }
 menus = menus.filter(root=>!menus.some(other=>other!==root && root.contains(other)));
 if(menus.length>1) throw new Error('Multiple open dropdown menus: '+name);
 if(!menus.length) return null;
 const menu = menus[0];
 const selector = name==='Category' && menu.getAttribute('role')==='dialog' ? '[role="button"],[role="option"]' : '[role="option"]';
 const matches = [...menu.querySelectorAll(selector)].filter(visible).filter(el => norm(el.getAttribute('aria-label'))===norm(label) || norm(el.textContent)===norm(label) || [...el.querySelectorAll('span')].some(s=>norm(s.textContent)===norm(label)));
 if(matches.length>1) throw new Error('Multiple matching options: '+label);
 return matches[0] || null;
}`

const marketplaceDropdownSelected = `(name,label) => {
 const control = (` + marketplaceNamedControl + `)(name,'[role="combobox"]');
 if(!control) return false;
 const norm = value => (value || '').replace(/\s+/g,' ').trim().toLocaleLowerCase();
 return norm(control.value)===norm(label) || [...control.querySelectorAll('span')].some(span=>norm(span.textContent)===norm(label));
}`

func marketplaceChoose(ctx context.Context, page *rod.Page, name, label string) error {
	if err := marketplaceOpenDropdown(ctx, page, name); err != nil {
		return err
	}
	option, err := page.Context(ctx).ElementByJS(rod.Eval(marketplaceDropdownOption, name, label))
	if err != nil {
		return fmt.Errorf("Could not find Facebook option %s: %w", label, err)
	}
	if err = option.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	if err = page.Context(ctx).Wait(rod.Eval(marketplaceDropdownSelected, name, label)); err != nil {
		return fmt.Errorf("Facebook did not retain %s selection: %w", name, err)
	}
	return nil
}

// Click the live Facebook label first and verify that React opened it. A
// dispatched click is not sufficient while the composer is still hydrating.
func marketplaceOpenDropdown(ctx context.Context, page *rod.Page, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	var nextClick time.Time
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("Could not open Facebook %s dropdown: %w", name, ctx.Err())
		}
		if _, err := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(ctx)); err != nil {
			return errors.New("Facebook was closed before the dropdown opened")
		}
		attempt, stop := context.WithTimeout(ctx, 2*time.Second)
		field, err := marketplaceField(attempt, page, name, `label[role="combobox"][aria-labelledby]`)
		if err == nil {
			state, stateErr := field.Eval(`function(){return {open:this.getAttribute('aria-expanded')==='true',disabled:this.getAttribute('aria-disabled')==='true'||this.hasAttribute('disabled')}}`)
			if stateErr == nil && state.Value.Get("open").Bool() {
				stop()
				return nil
			}
			if stateErr == nil && !state.Value.Get("disabled").Bool() && !time.Now().Before(nextClick) {
				// Reacquire the label on each retry; React may replace the ignored
				// control. Never re-click an expanded field and close its menu.
				_ = field.Click(proto.InputMouseButtonLeft, 1)
				nextClick = time.Now().Add(time.Second)
			}
		}
		stop()
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}

func marketplaceCheck(ctx context.Context, page *rod.Page, name string) error {
	field, err := marketplaceField(ctx, page, name, `[role="checkbox"],[role="switch"],input[type="checkbox"]`)
	if err != nil {
		return err
	}
	checked, err := field.Eval(`function(){return this.checked===true || this.getAttribute('aria-checked')==='true'}`)
	if err != nil {
		return err
	}
	if checked.Value.Bool() {
		return nil
	}
	if err = field.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	return field.Wait(rod.Eval(`function(){return this.checked===true || this.getAttribute('aria-checked')==='true'}`))
}

func marketplaceMoreDetails(ctx context.Context, page *rod.Page) error {
	control, err := marketplaceField(ctx, page, "More details", `[role="button"][aria-expanded],summary`)
	if err != nil {
		return err
	}
	state, err := control.Eval(`function(){return this.getAttribute('aria-expanded')==='true' || this.closest('details')?.open===true}`)
	if err != nil {
		return err
	}
	if state.Value.Bool() {
		return nil
	}
	if err = control.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	return control.Wait(rod.Eval(`function(){return this.getAttribute('aria-expanded')==='true' || this.closest('details')?.open===true}`))
}

func fillMarketplaceItemFields(ctx context.Context, page *rod.Page, fields MarketplaceItemFields) error {
	if err := validateMarketplaceItemFields(&fields); err != nil {
		return err
	}
	// Login waiting has no deadline. Filling a known form has a bounded, cancellable
	// deadline so missing/changed Facebook controls return a useful error.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := marketplaceText(ctx, page, "Title", fields.Title); err != nil {
		return err
	}
	if err := marketplaceText(ctx, page, "Price", fields.Price); err != nil {
		return err
	}
	if err := marketplaceChoose(ctx, page, "Category", marketplaceItemCategories[fields.Category]); err != nil {
		return err
	}
	if err := marketplaceChoose(ctx, page, "Condition", marketplaceItemConditions[fields.Condition]); err != nil {
		return err
	}
	if strings.TrimSpace(fields.Description) != "" {
		if err := marketplaceText(ctx, page, "Description", fields.Description); err != nil {
			return err
		}
	}
	more := fields.Availability != "" || len(fields.Tags) > 0 || strings.TrimSpace(fields.SKU) != "" || fields.Location != nil || fields.PublicMeetup || fields.DoorPickup || fields.DoorDropoff
	if more {
		if err := marketplaceMoreDetails(ctx, page); err != nil {
			return err
		}
	}
	if fields.Availability != "" {
		label := "List as Single Item"
		if fields.Availability == "in-stock" {
			label = "List as In Stock"
		}
		if err := marketplaceChoose(ctx, page, "Availability", label); err != nil {
			return err
		}
	}
	for _, tag := range fields.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		field, err := marketplaceField(ctx, page, "Product tags", `textarea,input:not([type="file"])`)
		if err != nil {
			return err
		}
		if err = field.Input(tag); err != nil {
			return err
		}
		if err = field.Type(input.Enter); err != nil {
			return err
		}
		if err = field.Wait(rod.Eval(`function(tag){return this.value==='' && [...(this.closest('label')?.querySelectorAll('[role="list"]')||[])].some(list=>list.textContent.includes(tag))}`, tag)); err != nil {
			return fmt.Errorf("Could not add product tag: %w", err)
		}
	}
	if strings.TrimSpace(fields.SKU) != "" {
		if err := marketplaceText(ctx, page, "SKU", fields.SKU); err != nil {
			return err
		}
	}
	if fields.Location != nil {
		if err := marketplaceText(ctx, page, "Location", fields.Location.Label); err != nil {
			return err
		}
		option, err := page.Context(ctx).ElementByJS(rod.Eval(marketplaceOption, fields.Location.Label, fields.Location.Subtitle))
		if err != nil {
			return fmt.Errorf("Could not match the selected location in Facebook: %w", err)
		}
		if err = option.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return err
		}
		if err = page.Context(ctx).Wait(rod.Eval(marketplaceLocationSelected, fields.Location.Label)); err != nil {
			return fmt.Errorf("Facebook did not confirm the selected location: %w", err)
		}
	}
	for _, toggle := range []struct {
		name    string
		enabled bool
	}{
		{"Public meetup", fields.PublicMeetup}, {"Door pickup", fields.DoorPickup}, {"Door dropoff", fields.DoorDropoff}, {"Promote listing after publish", fields.PromoteAfterPublish}, {"Hide from friends", fields.HideFromFriends},
	} {
		if toggle.enabled {
			if err := marketplaceCheck(ctx, page, toggle.name); err != nil {
				return err
			}
		}
	}
	return nil
}
