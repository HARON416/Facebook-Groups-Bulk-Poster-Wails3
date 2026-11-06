package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-rod/rod"
)

const marketplaceTestHomeURL = "https://www.facebook.com/marketplace/create/rental"

type MarketplaceHomeFields struct {
	HomeOffer           string                  `json:"homeOffer"`
	PropertyType        string                  `json:"propertyType"`
	Bedrooms            string                  `json:"bedrooms"`
	Bathrooms           string                  `json:"bathrooms"`
	Price               string                  `json:"price"`
	Address             string                  `json:"address"`
	Location            *MarketplaceLocationHit `json:"location"`
	Description         string                  `json:"description"`
	SquareFeet          string                  `json:"squareFeet"`
	Laundry             string                  `json:"laundry"`
	Parking             string                  `json:"parking"`
	AirConditioning     string                  `json:"airConditioning"`
	Heating             string                  `json:"heating"`
	PromoteAfterPublish bool                    `json:"promoteAfterPublish"`
}

// The sale schema comes from the supplied form. Internal names are never
// compared with translated Facebook labels or generated ID prefixes.
const marketplaceHomeControl = `(name,selector)=>{
 const visible=el=>el.getClientRects().length&&!el.closest('[hidden],[aria-hidden="true"],[inert],header,nav')&&getComputedStyle(el).visibility!=='hidden';
 const roots=[...document.querySelectorAll('main,[role="main"]')].filter(visible);
 const composers=roots.filter(el=>el.querySelector('input[type="file"][accept*="image/"]'));
 const scopes=composers.length?composers:roots;
 if(scopes.length>1)throw new Error('Multiple Marketplace home composers');
 const root=scopes[0];if(!root)return null;
 const all=query=>[...root.querySelectorAll(query)].filter(visible);
 const before=(a,b)=>!!(a.compareDocumentPosition(b)&Node.DOCUMENT_POSITION_FOLLOWING);
 const names=['Home offer','Property type','Laundry','Parking','Air conditioning','Heating'];
 const combos=all('label[role="combobox"][aria-haspopup="listbox"]');
 let matches=[];
 if(name==='Home offer')matches=combos.slice(0,1);
 else if(names.includes(name)) {
  if(combos.length!==names.length)return null;
  matches=[combos[names.indexOf(name)]];
 } else if(name==='Price')matches=all('input[type="text"][autocomplete="off"]:not([role="combobox"])');
 else if(name==='Location')matches=all('input[role="combobox"][aria-autocomplete="list"]');
 else if(name==='Description')matches=all('textarea');
 else if(name==='Promote listing after publish')matches=all('input[type="checkbox"][role="switch"]');
 else if(name==='Bedrooms'||name==='Bathrooms') {
  const prices=all('input[type="text"][autocomplete="off"]:not([role="combobox"])');
  if(combos.length!==names.length||prices.length!==1)return null;
  const fields=all('input[type="text"]:not([role="combobox"])').filter(el=>before(combos[1],el)&&before(el,prices[0]));
  if(fields.length!==2)throw new Error('Unsupported home bedroom/bathroom layout');
  matches=[fields[name==='Bedrooms'?0:1]];
 } else if(name==='Square feet') {
  const descriptions=all('textarea');if(descriptions.length!==1)return null;
  matches=all('input[type="text"]:not([role="combobox"])').filter(el=>before(descriptions[0],el));
 }
 matches=matches.filter(el=>el.matches(selector));
 if(matches.length>1)throw new Error('Multiple matching controls: '+name);
 return matches[0]||null;
}`

var marketplaceHomeOptions = map[string][]string{
	"Home offer":       {"rent", "sale"},
	"Property type":    {"apartment", "house", "townhouse"},
	"Laundry":          {"in-unit", "in-building", "available", "none"},
	"Parking":          {"garage", "street", "off-street", "available", "none"},
	"Air conditioning": {"central", "available", "none"},
	"Heating":          {"central", "electric", "gas", "radiator", "available", "none"},
}

var marketplaceHomeBathrooms = regexp.MustCompile(`^[0-9]+(\.[05])?$`)
var marketplaceHomeArea = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func validateMarketplaceHomeFields(f *MarketplaceHomeFields) error {
	if f == nil || f.HomeOffer != "sale" {
		return errors.New("Only homes for sale are supported for testing")
	}
	if !marketplaceItemPrice.MatchString(f.Bedrooms) || !marketplaceHomeBathrooms.MatchString(f.Bathrooms) || !marketplaceItemPrice.MatchString(f.Price) || strings.TrimSpace(f.Description) == "" || strings.TrimSpace(f.Address) == "" || f.Location == nil || f.Location.PageID == "" || f.Location.Label != f.Address {
		return errors.New("Enter valid bedrooms, bathrooms, price, description, and a selected address")
	}
	if f.SquareFeet != "" && !marketplaceHomeArea.MatchString(f.SquareFeet) {
		return errors.New("Enter valid property square feet")
	}
	for _, field := range []struct {
		name, value string
		required    bool
	}{
		{"Property type", f.PropertyType, true}, {"Laundry", f.Laundry, false}, {"Parking", f.Parking, false}, {"Air conditioning", f.AirConditioning, false}, {"Heating", f.Heating, false},
	} {
		if field.value == "" && !field.required {
			continue
		}
		found := false
		for _, value := range marketplaceHomeOptions[field.name] {
			if value == field.value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Choose a valid home %s", field.name)
		}
	}
	return nil
}

func chooseMarketplaceHome(ctx context.Context, page *rod.Page, name, value string) error {
	values := marketplaceHomeOptions[name]
	for index, key := range values {
		if key == value {
			return marketplaceChooseIndex(ctx, page, name, index, len(values), marketplaceHomeControl)
		}
	}
	return fmt.Errorf("Unsupported home %s value: %s", name, value)
}

func fillMarketplaceHomeFields(ctx context.Context, page *rod.Page, f MarketplaceHomeFields) error {
	if err := validateMarketplaceHomeFields(&f); err != nil {
		return err
	}
	// The runner chooses Sale before media upload. Inherit caller cancellation
	// without adding a form/dropdown time limit.
	if err := chooseMarketplaceHome(ctx, page, "Property type", f.PropertyType); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{{"Bedrooms", f.Bedrooms}, {"Bathrooms", f.Bathrooms}, {"Price", f.Price}, {"Description", f.Description}, {"Square feet", f.SquareFeet}} {
		if field.value != "" {
			if err := marketplaceTextWith(ctx, page, field.name, field.value, marketplaceHomeControl); err != nil {
				return err
			}
		}
	}
	if err := marketplaceSelectLocation(ctx, page, f.Location); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{{"Laundry", f.Laundry}, {"Parking", f.Parking}, {"Air conditioning", f.AirConditioning}, {"Heating", f.Heating}} {
		if field.value != "" {
			if err := chooseMarketplaceHome(ctx, page, field.name, field.value); err != nil {
				return err
			}
		}
	}
	if f.PromoteAfterPublish {
		return marketplaceCheckWith(ctx, page, "Promote listing after publish", marketplaceHomeControl)
	}
	return nil
}
