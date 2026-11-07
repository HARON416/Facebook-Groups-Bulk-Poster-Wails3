package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

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
	PrivateRoom         bool                    `json:"privateRoom"`
	Occupants           string                  `json:"occupants"`
	BathroomType        string                  `json:"bathroomType"`
	RoomFurnished       bool                    `json:"roomFurnished"`
	DateAvailable       string                  `json:"dateAvailable"`
	CatFriendly         bool                    `json:"catFriendly"`
	DogFriendly         bool                    `json:"dogFriendly"`
	PromoteAfterPublish bool                    `json:"promoteAfterPublish"`
}

// Sale, whole-property and private-room schemas come from the supplied forms. Internal names are never
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
 const combos=all('label[role="combobox"][aria-haspopup="listbox"]');
 const privateRoom=combos.length===7;
 const names=privateRoom?['Home offer','Property type','Bathroom type','Laundry','Parking','Air conditioning','Heating']:['Home offer','Property type','Laundry','Parking','Air conditioning','Heating'];
 let matches=[];
 if(name==='Home offer')matches=combos.slice(0,1);
 else if(names.includes(name)) {
  if(combos.length!==names.length)return null;
  matches=[combos[names.indexOf(name)]];
 } else if(name==='Price')matches=all('input[type="text"][autocomplete="off"]:not([role="combobox"])');
 else if(name==='Location')matches=all('input[role="combobox"][aria-autocomplete="list"]');
 else if(name==='Description')matches=all('textarea');
 else if(name==='Date available')matches=all('input[role="combobox"][aria-autocomplete="none"]');
 else if(['Private room','Room furnished','Cat friendly','Dog friendly','Promote listing after publish'].includes(name)) {
  const switches=all('input[type="checkbox"][role="switch"]');
  const slots=switches.length===1?['Promote listing after publish']:switches.length===4?['Private room','Cat friendly','Dog friendly','Promote listing after publish']:switches.length===5?['Private room','Room furnished','Cat friendly','Dog friendly','Promote listing after publish']:[];
  const index=slots.indexOf(name);if(index>=0)matches=[switches[index]];
 }
 else if(name==='Bedrooms'||name==='Bathrooms'||name==='Occupants') {
  const prices=all('input[type="text"][autocomplete="off"]:not([role="combobox"])');
  if(combos.length!==names.length||prices.length!==1)return null;
  let fields;
  if(name==='Occupants') {
   if(!privateRoom)return null;
   fields=all('input[type="text"]:not([role="combobox"])').filter(el=>before(combos[1],el)&&before(el,prices[0]));
   if(fields.length!==1)throw new Error('Unsupported private-room occupant layout');
   matches=fields;
  } else {
  const locations=all('input[role="combobox"][aria-autocomplete="list"]');
  if(privateRoom && locations.length!==1)return null;
  fields=all('input[type="text"]:not([role="combobox"])').filter(el=>privateRoom?before(combos[2],el)&&before(el,locations[0]):before(combos[1],el)&&before(el,prices[0]));
  if(fields.length!==2)throw new Error('Unsupported home bedroom/bathroom layout');
  matches=[fields[name==='Bedrooms'?0:1]];
  }
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
	"Bathroom type":    {"private", "shared"},
	"Laundry":          {"in-unit", "in-building", "available", "none"},
	"Parking":          {"garage", "street", "off-street", "available", "none"},
	"Air conditioning": {"central", "available", "none"},
	"Heating":          {"central", "electric", "gas", "radiator", "available", "none"},
}

var marketplaceHomeBathrooms = regexp.MustCompile(`^[0-9]+(\.[05])?$`)
var marketplaceHomeArea = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func validateMarketplaceHomeFields(f *MarketplaceHomeFields) error {
	if f == nil || (f.HomeOffer != "sale" && f.HomeOffer != "rent") {
		return errors.New("Choose whether the home is for sale or rent")
	}
	if f.HomeOffer == "rent" {
		if f.PrivateRoom {
			if !marketplaceItemPrice.MatchString(f.Occupants) || (f.BathroomType != "private" && f.BathroomType != "shared") {
				return errors.New("Enter the number of occupants and choose a valid bathroom type")
			}
		}
		if f.DateAvailable != "" {
			if _, err := time.Parse("2006-01-02", f.DateAvailable); err != nil {
				return errors.New("Choose a valid date available")
			}
		}
	}
	if !marketplaceItemPrice.MatchString(f.Bedrooms) || !marketplaceHomeBathrooms.MatchString(f.Bathrooms) || !marketplaceItemPrice.MatchString(f.Price) || strings.TrimSpace(f.Description) == "" || strings.TrimSpace(f.Address) == "" || f.Location == nil || f.Location.Label != f.Address {
		return errors.New("Enter valid bedrooms, bathrooms, price, description, and a selected address")
	}
	if f.Location.PageID == "" && (f.Location.AddressKey == "" || f.Location.AddressKey != marketplaceGeocodedAddressKey(f.Location.Label, f.Location.Latitude, f.Location.Longitude)) {
		return errors.New("Select a valid house address from the search results")
	}
	if f.Location.SearchType != "STREET_PLACE_TYPEAHEAD" {
		return errors.New("Select the address again using home address search")
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
	// The runner chooses Sale/Rent before media upload. Inherit caller cancellation
	// without adding a form/dropdown time limit.
	if err := chooseMarketplaceHome(ctx, page, "Property type", f.PropertyType); err != nil {
		return err
	}
	if f.HomeOffer == "rent" && f.PrivateRoom {
		if err := marketplaceTextWith(ctx, page, "Occupants", f.Occupants, marketplaceHomeControl); err != nil {
			return err
		}
		if err := chooseMarketplaceHome(ctx, page, "Bathroom type", f.BathroomType); err != nil {
			return err
		}
		if err := marketplaceHomeSwitch(ctx, page, "Room furnished", f.RoomFurnished); err != nil {
			return err
		}
	}
	for _, field := range []struct{ name, value string }{{"Bedrooms", f.Bedrooms}, {"Bathrooms", f.Bathrooms}, {"Price", f.Price}, {"Description", f.Description}, {"Square feet", f.SquareFeet}} {
		if field.value != "" {
			if err := marketplaceTextWith(ctx, page, field.name, field.value, marketplaceHomeControl); err != nil {
				return err
			}
		}
	}
	if err := marketplaceSelectHomeLocation(ctx, page, f.Location); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{{"Laundry", f.Laundry}, {"Parking", f.Parking}, {"Air conditioning", f.AirConditioning}, {"Heating", f.Heating}} {
		if field.value != "" {
			if err := chooseMarketplaceHome(ctx, page, field.name, field.value); err != nil {
				return err
			}
		}
	}
	if f.HomeOffer == "rent" {
		if f.DateAvailable != "" {
			if err := marketplaceHomeDate(ctx, page, f.DateAvailable); err != nil {
				return err
			}
		}
		for _, field := range []struct {
			name string
			on   bool
		}{{"Cat friendly", f.CatFriendly}, {"Dog friendly", f.DogFriendly}} {
			if err := marketplaceHomeSwitch(ctx, page, field.name, field.on); err != nil {
				return err
			}
		}
	}
	if f.PromoteAfterPublish {
		if err := marketplaceCheckWith(ctx, page, "Promote listing after publish", marketplaceHomeControl); err != nil {
			return err
		}
	}
	if f.HomeOffer == "rent" {
		// Confirm both acknowledged pet settings together after other switches
		// have finished rendering, before the runner can advance to Next.
		verify, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := page.Context(verify).Wait(rod.Eval(`(cat,dog)=>{
 return [['Cat friendly',cat],['Dog friendly',dog]].every(([name,on])=>{
  const field=(`+marketplaceHomeControl+`)(name,'input[type="checkbox"][role="switch"]');
  return !!field && (`+marketplaceHomeSwitchState+`).call(field)===on;
 });
}`, f.CatFriendly, f.DogFriendly)); err != nil {
			return fmt.Errorf("Facebook did not retain the selected cat and dog settings: %w", err)
		}
	}
	return nil
}

// Address-first search returns a ranked subset, not every place matching a city.
// Reuse the query that actually produced the selected ID in our picker. Older
// saved addresses can retry exact caption/city queries, always requiring the
// saved page ID or the geocoded address identity.
func marketplaceSelectHomeLocation(ctx context.Context, page *rod.Page, location *MarketplaceLocationHit) error {
	queries := []string{}
	for i, candidate := range []string{location.SearchQuery, location.Label, location.City, strings.SplitN(location.Label, ",", 2)[0]} {
		if i != 0 {
			candidate = strings.TrimSpace(candidate)
		}
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		duplicate := false
		for _, existing := range queries {
			duplicate = duplicate || strings.EqualFold(existing, candidate)
		}
		if !duplicate {
			queries = append(queries, candidate)
		}
	}
	var lastErr error
	for _, query := range queries {
		lastErr = marketplaceSelectLocationQuery(ctx, page, location, query)
		if !errors.Is(lastErr, errMarketplaceLocationNotReturned) {
			return lastErr
		}
	}
	return fmt.Errorf("%w. The saved place was absent from Facebook's house-address results for all attempted queries; select the address again", lastErr)
}
