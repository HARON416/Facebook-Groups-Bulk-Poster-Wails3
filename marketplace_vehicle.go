package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

const marketplaceTestVehicleURL = "https://www.facebook.com/marketplace/create/vehicle"

type MarketplaceVehicleFields struct {
	VehicleType         string                  `json:"vehicleType"`
	Year                string                  `json:"year"`
	Make                string                  `json:"make"`
	Model               string                  `json:"model"`
	Mileage             string                  `json:"mileage"`
	Price               string                  `json:"price"`
	BodyStyle           string                  `json:"bodyStyle"`
	ExteriorColor       string                  `json:"exteriorColor"`
	InteriorColor       string                  `json:"interiorColor"`
	VehicleCondition    string                  `json:"vehicleCondition"`
	FuelType            string                  `json:"fuelType"`
	Transmission        string                  `json:"transmission"`
	CleanTitle          bool                    `json:"cleanTitle"`
	Description         string                  `json:"description"`
	Location            *MarketplaceLocationHit `json:"location"`
	PromoteAfterPublish bool                    `json:"promoteAfterPublish"`
}

// Schemas cover the supplied Car/Truck, Motorcycle, Powersport, RV/Camper, Trailer, Boat, Commercial/Industrial, and Other forms. Required-state
// metadata may change during entry; field positions are checked as a whole.
const marketplaceVehicleControlTemplate = `(names)=>(name,selector)=>{
 const visible=el=>el.getClientRects().length && !el.closest('[hidden],[aria-hidden="true"],[inert],header,nav') && getComputedStyle(el).visibility!=='hidden';
 const roots=[...document.querySelectorAll('main,[role="main"]')].filter(visible);
 const composers=roots.filter(el=>el.querySelector('input[type="file"][accept*="image/"]'));
 const scopes=composers.length?composers:roots;
 if(scopes.length>1)throw new Error('Multiple Marketplace vehicle composers');
 const root=scopes[0];if(!root)return null;
 const all=query=>[...root.querySelectorAll(query)].filter(visible);
 const before=(a,b)=>!!(a.compareDocumentPosition(b)&Node.DOCUMENT_POSITION_FOLLOWING);
 const combos=all('label[role="combobox"][aria-haspopup="listbox"]');
 let matches=[];
 if(name==='Vehicle type') {
  const upload=root.querySelector('input[type="file"][accept*="image/"]');
  matches=upload?combos.filter(el=>before(el,upload)):[];
 } else if(names.includes(name)) {
  if(combos.length!==names.length)return null;
  matches=[combos[names.indexOf(name)]];
 } else if(name==='Location')matches=all('input[role="combobox"][aria-autocomplete="list"]');
 else if(name==='Price')matches=all('input[type="text"][autocomplete="off"]:not([role="combobox"])');
 else if(name==='Description')matches=all('textarea');
 else if(name==='Clean title')matches=all('input[type="checkbox"][name="title_status"]');
 else if(name==='Promote listing after publish')matches=all('input[type="checkbox"][role="switch"]');
 else if(name==='Make'||name==='Model'||name==='Mileage') {
  const prices=all('input[type="text"][autocomplete="off"]:not([role="combobox"])');
  if(combos.length!==names.length || prices.length!==1)return null;
  const freeMake=!names.includes('Make');
  if(name==='Mileage' && freeMake)return null;
  const fields=all('input[type="text"]:not([role="combobox"])').filter(el=>before(combos[freeMake?1:2],el)&&before(el,prices[0]));
  if(fields.length!==2)throw new Error('Unsupported vehicle text-field layout');
  matches=[fields[freeMake?(name==='Make'?0:1):(name==='Model'?0:1)]];
 }
 matches=matches.filter(el=>el.matches(selector));
 if(matches.length>1)throw new Error('Multiple matching controls: '+name);
 return matches[0]||null;
}`

func marketplaceVehicleBasicLayout(kind string) bool {
	switch kind {
	case "powersport", "rv-camper", "trailer", "boat", "commercial-industrial", "other":
		return true
	default:
		return false
	}
}

func marketplaceVehicleFieldNames(kind string) []string {
	if kind == "other" {
		return []string{"Vehicle type", "Year"}
	}
	if kind == "trailer" {
		return []string{"Vehicle type", "Year", "Exterior color", "Interior color"}
	}
	if marketplaceVehicleBasicLayout(kind) {
		return []string{"Vehicle type", "Year", "Exterior color", "Interior color", "Fuel type"}
	}
	if kind == "motorcycle" {
		return []string{"Vehicle type", "Year", "Make", "Exterior color", "Fuel type", "Transmission"}
	}
	return []string{"Vehicle type", "Year", "Make", "Body style", "Exterior color", "Interior color", "Vehicle condition", "Fuel type", "Transmission"}
}

func marketplaceVehicleControlFor(kind string) string {
	names, _ := json.Marshal(marketplaceVehicleFieldNames(kind))
	return "(" + marketplaceVehicleControlTemplate + ")(" + string(names) + ")"
}

func marketplaceVehicleValues(kind, name string) []string {
	if kind == "motorcycle" && name == "Make" {
		return marketplaceMotorcycleMakes
	}
	return marketplaceVehicleOptions[name]
}

func validateMarketplaceVehicleFields(f *MarketplaceVehicleFields) error {
	if f == nil || (f.VehicleType != "car-truck" && f.VehicleType != "motorcycle" && !marketplaceVehicleBasicLayout(f.VehicleType)) {
		return errors.New("This vehicle type is not supported for testing yet")
	}
	year, err := strconv.Atoi(f.Year)
	if err != nil || year < 1901 || year > time.Now().Year()+1 || !marketplaceItemPrice.MatchString(f.Year) || strings.TrimSpace(f.Model) == "" || !marketplaceItemPrice.MatchString(f.Price) || f.Location == nil || f.Location.PageID == "" || strings.TrimSpace(f.Location.Label) == "" {
		return errors.New("Enter a valid year, make, model, price, and selected location")
	}
	if !marketplaceVehicleBasicLayout(f.VehicleType) && !marketplaceItemPrice.MatchString(f.Mileage) {
		return errors.New("Enter valid whole-number mileage")
	}
	for _, field := range []struct {
		name, value string
		required    bool
	}{
		{"Make", f.Make, true}, {"Transmission", f.Transmission, true}, {"Body style", f.BodyStyle, false}, {"Exterior color", f.ExteriorColor, false}, {"Interior color", f.InteriorColor, false}, {"Vehicle condition", f.VehicleCondition, false}, {"Fuel type", f.FuelType, false},
	} {
		if f.VehicleType == "other" && (field.name == "Exterior color" || field.name == "Interior color" || field.name == "Fuel type") {
			continue
		}
		if f.VehicleType != "car-truck" && (field.name == "Body style" || field.name == "Vehicle condition") || f.VehicleType == "motorcycle" && field.name == "Interior color" || marketplaceVehicleBasicLayout(f.VehicleType) && field.name == "Transmission" || f.VehicleType == "trailer" && field.name == "Fuel type" {
			continue
		}
		if marketplaceVehicleBasicLayout(f.VehicleType) && field.name == "Make" {
			if strings.TrimSpace(field.value) == "" {
				return errors.New("Enter a vehicle make")
			}
			continue
		}
		if field.value == "" && !field.required {
			continue
		}
		found := false
		for _, value := range marketplaceVehicleValues(f.VehicleType, field.name) {
			if value == field.value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Choose a valid vehicle %s", field.name)
		}
	}
	return nil
}

func chooseMarketplaceVehicleFor(ctx context.Context, page *rod.Page, kind, name, value string) error {
	values := marketplaceVehicleValues(kind, name)
	for index, key := range values {
		if key == value {
			return marketplaceChooseIndex(ctx, page, name, index, len(values), marketplaceVehicleControlFor(kind))
		}
	}
	return fmt.Errorf("Unsupported vehicle %s value: %s", name, value)
}

func fillMarketplaceVehicleFields(ctx context.Context, page *rod.Page, f MarketplaceVehicleFields) error {
	if err := validateMarketplaceVehicleFields(&f); err != nil {
		return err
	}
	// Use the caller's cancellation context; slow forms have no overall time cap.
	resolver := marketplaceVehicleControlFor(f.VehicleType)
	if f.VehicleType == "other" {
		f.ExteriorColor = ""
		f.InteriorColor = ""
	}
	if f.VehicleType == "trailer" || f.VehicleType == "other" {
		f.FuelType = ""
	}
	if f.VehicleType != "car-truck" {
		f.BodyStyle = ""
		f.VehicleCondition = ""
		f.CleanTitle = false
	}
	if f.VehicleType == "motorcycle" {
		f.InteriorColor = ""
	}
	if marketplaceVehicleBasicLayout(f.VehicleType) {
		f.Mileage = ""
		f.Transmission = ""
	}
	// The shared runner selects the vehicle type before uploading media, since changing
	// the vehicle type may replace the form and clear previously selected files.
	if err := marketplaceSelectLocation(ctx, page, f.Location); err != nil {
		return err
	}
	if err := marketplaceOpenDropdownWith(ctx, page, "Year", 0, resolver); err != nil {
		return err
	}
	// Years roll forward and may use localized digits. Match the live numeric
	// value rather than a hardcoded top year or English accessible name.
	yearScript := `(year)=>{
  const first=(` + marketplaceDropdownOptionFor(resolver) + `)('Year',0,0);if(!first)return null;
  const digits=value=>value.normalize('NFKC').replace(/[\u0660-\u0669\u06f0-\u06f9\u0966-\u096f]/g,c=>String(c.charCodeAt(0)-(c.charCodeAt(0)>=0x966?0x966:c.charCodeAt(0)>=0x6f0?0x6f0:0x660))).trim();
  const options=[...first.closest('[role="listbox"]').querySelectorAll('[role="option"]')].filter(el=>digits(el.textContent)===year);
  if(options.length>1)throw new Error('Multiple matching vehicle years');return options[0]||null;
 }`
	year, err := page.Context(ctx).ElementByJS(rod.Eval(yearScript, f.Year))
	if err != nil {
		return fmt.Errorf("Could not select vehicle year: %w", err)
	}
	if err := marketplaceSelectOption(ctx, page, "Year", year, resolver); err != nil {
		return err
	}
	if marketplaceVehicleBasicLayout(f.VehicleType) {
		if err := marketplaceTextWith(ctx, page, "Make", f.Make, resolver); err != nil {
			return err
		}
	} else if err := chooseMarketplaceVehicleFor(ctx, page, f.VehicleType, "Make", f.Make); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{{"Model", f.Model}, {"Mileage", f.Mileage}, {"Price", f.Price}, {"Description", f.Description}} {
		if field.value != "" {
			if err := marketplaceTextWith(ctx, page, field.name, field.value, resolver); err != nil {
				return err
			}
		}
	}
	for _, field := range []struct{ name, value string }{{"Body style", f.BodyStyle}, {"Exterior color", f.ExteriorColor}, {"Interior color", f.InteriorColor}, {"Vehicle condition", f.VehicleCondition}, {"Fuel type", f.FuelType}, {"Transmission", f.Transmission}} {
		if field.value != "" {
			if err := chooseMarketplaceVehicleFor(ctx, page, f.VehicleType, field.name, field.value); err != nil {
				return err
			}
		}
	}
	for _, field := range []struct {
		name    string
		enabled bool
	}{{"Clean title", f.CleanTitle}, {"Promote listing after publish", f.PromoteAfterPublish}} {
		if field.enabled {
			if err := marketplaceCheckWith(ctx, page, field.name, resolver); err != nil {
				return err
			}
		}
	}
	return nil
}

var marketplaceVehicleOptions = map[string][]string{
	"Vehicle type":      {"car-truck", "motorcycle", "powersport", "rv-camper", "trailer", "boat", "commercial-industrial", "other"},
	"Make":              {"Acura", "Alfa Romeo", "Aston Martin", "Audi", "BMW", "Bentley", "Buick", "CODA", "Cadillac", "Chevrolet", "Chrysler", "Daewoo", "Daihatsu", "Dodge", "Eagle", "Ferrari", "Fiat", "Fisker", "Ford", "Freightliner", "GMC", "Genesis", "Geo", "Honda", "Hummer", "Hyundai", "Infiniti", "Isuzu", "Jaguar", "Jeep", "Kia", "Lamborghini", "Land Rover", "Lexus", "Lincoln", "Lotus", "Lucid", "MINI", "Maserati", "Maybach", "Mazda", "Mclaren", "Mercedes-Benz", "Mercury", "Mitsubishi", "Nissan", "Oldsmobile", "Panoz", "Plymouth", "Polestar", "Pontiac", "Porsche", "Ram", "Rivian", "Rolls-Royce", "SRT", "Saab", "Saturn", "Scion", "Smart", "Subaru", "Suzuki", "Tesla", "Toyota", "Volkswagen", "Volvo"},
	"Body style":        {"coupe", "truck", "sedan", "hatchback", "suv", "convertible", "wagon", "minivan", "small-car", "other"},
	"Exterior color":    {"black", "blue", "brown", "gold", "green", "gray", "pink", "purple", "red", "silver", "orange", "white", "yellow", "charcoal", "off-white", "tan", "beige", "burgundy", "turquoise", "other"},
	"Interior color":    {"black", "blue", "brown", "gold", "green", "gray", "pink", "purple", "red", "silver", "orange", "white", "yellow", "charcoal", "off-white", "tan", "beige", "burgundy", "turquoise", "other"},
	"Vehicle condition": {"excellent", "very-good", "good", "fair", "poor"},
	"Fuel type":         {"diesel", "electric", "gasoline", "flex", "hybrid", "petrol", "plug-in-hybrid", "other"},
	"Transmission":      {"manual", "automatic"},
}
