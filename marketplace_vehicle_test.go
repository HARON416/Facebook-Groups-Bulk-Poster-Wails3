package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func validMarketplaceVehicle() MarketplaceVehicleFields {
	return MarketplaceVehicleFields{VehicleType: "car-truck", Year: "2020", Make: "Toyota", Model: "Hilux", Mileage: "50000", Price: "2500000", Transmission: "automatic", Location: &MarketplaceLocationHit{PageID: "selected", Label: "Nairobi, Kenya", Subtitle: "مدينة"}}
}

func TestMarketplaceVehicleValidation(t *testing.T) {
	valid := validMarketplaceVehicle()
	if err := validateMarketplaceVehicleFields(&valid); err != nil {
		t.Fatal(err)
	}
	if err := validateMarketplaceVehicleFields(nil); err == nil {
		t.Fatal("Missing vehicle accepted")
	}
	for _, change := range []func(*MarketplaceVehicleFields){
		func(f *MarketplaceVehicleFields) { f.VehicleType = "unsupported" }, func(f *MarketplaceVehicleFields) { f.Make = "Unknown" }, func(f *MarketplaceVehicleFields) { f.Year = "1899" }, func(f *MarketplaceVehicleFields) { f.Model = " " }, func(f *MarketplaceVehicleFields) { f.Mileage = "-1" }, func(f *MarketplaceVehicleFields) { f.Price = "1.2" }, func(f *MarketplaceVehicleFields) { f.Transmission = "other" }, func(f *MarketplaceVehicleFields) { f.Location = nil }, func(f *MarketplaceVehicleFields) { f.BodyStyle = "invalid" },
	} {
		f := valid
		change(&f)
		if err := validateMarketplaceVehicleFields(&f); err == nil {
			t.Fatalf("Invalid vehicle accepted: %+v", f)
		}
	}
	photos := make([]PostUpload, 20)
	for i := range photos {
		photos[i] = PostUpload{Name: "car.png", Data: fixturePNG}
	}
	dir, _, _, err := stageMarketplaceTestMedia(MarketplaceTestMedia{Photos: photos, VehicleFields: &valid})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if _, _, _, err := stageMarketplaceTestMedia(MarketplaceTestMedia{Photos: append(photos, photos[0]), VehicleFields: &valid}); err == nil {
		t.Fatal("21 photos accepted")
	}
	if _, _, _, err := stageMarketplaceTestMedia(MarketplaceTestMedia{Photos: photos}); err == nil {
		t.Fatal("Item photo limit changed")
	}
}

func TestMarketplaceMotorcycleValidation(t *testing.T) {
	f := validMarketplaceVehicle()
	f.VehicleType = "motorcycle"
	f.Make = "Yamaha"
	// Hidden Car/Truck draft fields do not apply to the Motorcycle form.
	f.BodyStyle, f.InteriorColor, f.VehicleCondition = "stale", "stale", "stale"
	f.CleanTitle = true
	if err := validateMarketplaceVehicleFields(&f); err != nil {
		t.Fatal(err)
	}
	for _, mileage := range []string{"1.5", "-1", "1e3", "", "1,000"} {
		invalid := f
		invalid.Mileage = mileage
		if err := validateMarketplaceVehicleFields(&invalid); err == nil {
			t.Fatalf("Invalid motorcycle mileage accepted: %q", mileage)
		}
	}
	f.Make = "Toyota"
	if err := validateMarketplaceVehicleFields(&f); err == nil {
		t.Fatal("Car manufacturer accepted as motorcycle make")
	}
}

func TestMarketplacePowersportValidation(t *testing.T) {
	testMarketplaceBasicVehicleValidation(t, "powersport")
}

func TestMarketplaceRVCamperValidation(t *testing.T) {
	testMarketplaceBasicVehicleValidation(t, "rv-camper")
}

func TestMarketplaceTrailerValidation(t *testing.T) {
	testMarketplaceBasicVehicleValidation(t, "trailer")
}

func TestMarketplaceBoatValidation(t *testing.T) { testMarketplaceBasicVehicleValidation(t, "boat") }

func TestMarketplaceCommercialIndustrialValidation(t *testing.T) {
	testMarketplaceBasicVehicleValidation(t, "commercial-industrial")
}

func TestMarketplaceOtherValidation(t *testing.T) { testMarketplaceBasicVehicleValidation(t, "other") }

func testMarketplaceBasicVehicleValidation(t *testing.T, kind string) {
	t.Helper()
	f := validMarketplaceVehicle()
	f.VehicleType, f.Make = kind, "Custom Vehicle Brand"
	f.Mileage, f.Transmission = "", ""
	f.InteriorColor = "beige"
	if kind == "other" {
		f.ExteriorColor = "stale"
		f.InteriorColor = "stale"
	}
	if kind == "trailer" || kind == "other" {
		f.FuelType = "stale"
	}
	// Values retained from another vehicle type must not gate or fill Powersport.
	f.BodyStyle, f.VehicleCondition = "stale", "stale"
	f.CleanTitle = true
	if err := validateMarketplaceVehicleFields(&f); err != nil {
		t.Fatal(err)
	}
	f.Mileage, f.Transmission = "stale", "stale"
	if err := validateMarketplaceVehicleFields(&f); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*MarketplaceVehicleFields){
		func(f *MarketplaceVehicleFields) { f.Make = " " },
		func(f *MarketplaceVehicleFields) { f.Model = "" },
		func(f *MarketplaceVehicleFields) { f.Price = "1.5" },
		func(f *MarketplaceVehicleFields) { f.Location = nil },
	} {
		invalid := f
		change(&invalid)
		if err := validateMarketplaceVehicleFields(&invalid); err == nil {
			t.Fatalf("Invalid basic vehicle fields accepted: %+v", invalid)
		}
	}
	if kind != "other" {
		invalid := f
		invalid.InteriorColor = "invalid"
		if err := validateMarketplaceVehicleFields(&invalid); err == nil {
			t.Fatal("Invalid interior color accepted")
		}
	}
	if kind != "trailer" && kind != "other" {
		invalid := f
		invalid.FuelType = "invalid"
		if err := validateMarketplaceVehicleFields(&invalid); err == nil {
			t.Fatal("Invalid fuel type accepted")
		}
	}
}

func TestMarketplaceCarTruckBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "car-truck")
}

func TestMarketplaceMotorcycleBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "motorcycle")
}

func TestMarketplacePowersportBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "powersport")
}

func TestMarketplaceRVCamperBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "rv-camper")
}

func TestMarketplaceTrailerBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "trailer")
}

func TestMarketplaceBoatBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "boat")
}

func TestMarketplaceCommercialIndustrialBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "commercial-industrial")
}

func TestMarketplaceOtherBrowserIntegration(t *testing.T) {
	testMarketplaceVehicleBrowserIntegration(t, "other")
}

func testMarketplaceVehicleBrowserIntegration(t *testing.T, kind string) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	capture, err := os.ReadFile("testdata/marketplace/" + kind + ".html")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"Vehicle type", "Year", "Make", "Body style", "Exterior color", "Interior color", "Vehicle condition", "Fuel type", "Transmission"}
	if kind == "motorcycle" {
		names = []string{"Vehicle type", "Year", "Make", "Exterior color", "Fuel type", "Transmission"}
	}
	if kind != "car-truck" && kind != "motorcycle" {
		names = []string{"Vehicle type", "Year", "Exterior color", "Interior color", "Fuel type"}
	}
	if kind == "trailer" {
		names = []string{"Vehicle type", "Year", "Exterior color", "Interior color"}
	}
	if kind == "other" {
		names = []string{"Vehicle type", "Year"}
	}
	choices := make([][]string, len(names))
	for i, name := range names {
		choices[i] = marketplaceVehicleValues(kind, name)
	}
	for year := 2027; year >= 1901; year-- {
		choices[1] = append(choices[1], fmt.Sprint(year))
	}
	data, _ := json.Marshal(choices)
	harness := `<script>
window.values={selected:{},nextClicks:0,published:0};
const main=document.querySelector('[role=main]');
const choices=CHOICES;
const combos=[...main.querySelectorAll('label[role=combobox]')];
const arabic=s=>s.replace(/[0-9]/g,c=>String.fromCharCode(0x660+Number(c)));
combos.forEach((control,field)=>{
 const caption=document.createElement('span');caption.dataset.selection='';control.append(caption);
 control.onclick=()=>{
  if(document.querySelector('[data-popup]')){values.extraClick=true;return}
  control.setAttribute('aria-expanded','false');
  const menu=document.createElement('div');menu.role='listbox';menu.dataset.popup='';menu.id='popup-'+Math.random().toString(36).slice(2);
  choices[field].forEach((key,index)=>{
   const option=document.createElement('div');option.role='option';option.tabIndex=0;option.id='random-'+field+'___'+index;option.setAttribute('aria-selected','false');
   const label=document.createElement('span');label.textContent=field===1?arabic(key):'اختيار '+field+' '+index;option.append(label);
   option.onclick=()=>{if(field===0){values.photos=0;values.video=0}values.selected[field]=key;control.querySelector('[data-selection]').textContent=label.textContent;control.setAttribute('aria-expanded','false');menu.remove();control.replaceWith(control.cloneNode(true))};
   menu.prepend(option);
  });
  setTimeout(()=>document.body.append(menu),50);
 };
});
const locationInput=main.querySelector('input[role=combobox]');
locationInput.oninput=()=>{
 const list=document.createElement('ul');list.role='listbox';list.id='suggestions';locationInput.setAttribute('aria-controls',list.id);locationInput.setAttribute('aria-expanded','true');
 const row=document.createElement('li');row.role='option';row.innerHTML='<span dir="auto">Nairobi, Kenya</span><span>مدينة</span>';
 row.onclick=()=>{locationInput.value='Nairobi, Kenya';locationInput.setAttribute('aria-expanded','false');list.remove();values.location=true};list.append(row);document.body.append(list);
};
main.querySelector('input[accept*="image/"]').onchange=function(){values.photos=this.files.length};
main.querySelector('input[accept*="video/"]').onchange=function(){values.video=this.files.length};
const progress=main.querySelector('[role=img]');progress.style.height='4px';
let wrapper=progress.parentElement;
while(wrapper && (!wrapper.nextElementSibling || !wrapper.nextElementSibling.querySelector('[role=button]')))wrapper=wrapper.parentElement;
const actions=wrapper.nextElementSibling;
const next=actions.querySelector('[role=button]');
next.removeAttribute('aria-disabled');next.onclick=()=>{
 values.nextClicks++;
 values.make=main.querySelector('#_r_kc_')?.value;values.model=main.querySelector('#_r_du_').value;values.mileage=main.querySelector('#_r_gb_')?.value||'';values.price=main.querySelector('#_r_e2_').value;values.description=main.querySelector('textarea').value;
 values.clean=main.querySelector('[name=title_status]')?.checked||false;values.promote=main.querySelector('[role=switch]').checked;
 const destination=document.createElement('div');destination.role='main';destination.innerHTML='<h1>次の画面</h1><div><div role="img"><div><div></div></div><div><div></div></div></div></div><div><button>戻る</button><button onclick="values.published++">公開</button></div>';
 main.replaceWith(destination);
};
window.fixtureReady=true;
</script>`
	harness = strings.Replace(harness, "CHOICES", string(data), 1)
	if kind == "trailer" || kind == "boat" || kind == "commercial-industrial" || kind == "other" {
		harness = strings.NewReplacer("#_r_kc_", "#_r_i6_", "#_r_du_", "#_r_ia_", "#_r_e2_", "#_r_ie_").Replace(harness)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html dir="rtl"><body class="UIPage_LoggedIn">`, string(capture), harness)
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	videoData, err := os.ReadFile("frontend/testdata/vehicle-short.webm")
	if err != nil {
		t.Fatal(err)
	}
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprint("optional-", all), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			f := validMarketplaceVehicle()
			if kind == "motorcycle" {
				f.VehicleType = kind
				f.Make = "Yamaha"
				f.Model = "MT-07"
			}
			if kind != "car-truck" && kind != "motorcycle" {
				f.VehicleType = kind
				f.Make = "Custom Powersport Make"
				f.Model = "ATV 850"
				f.Mileage = ""
				f.Transmission = ""
			}
			photos := []PostUpload{{Name: "car.png", Data: fixturePNG}}
			media := MarketplaceTestMedia{Photos: photos, VehicleFields: &f}
			if all {
				f.BodyStyle = "truck"
				f.ExteriorColor = "black"
				f.InteriorColor = "beige"
				f.VehicleCondition = "excellent"
				f.FuelType = "diesel"
				f.CleanTitle = true
				f.PromoteAfterPublish = true
				f.Description = "وصف السيارة"
				for len(media.Photos) < 20 {
					media.Photos = append(media.Photos, photos[0])
				}
				media.Video = &PostUpload{Name: "clip.webm", Data: base64.StdEncoding.EncodeToString(videoData)}
			}
			if err := service.testMarketplaceItemMedia(ctx, server.URL, media); err != nil {
				t.Fatal(err)
			}
			result, err := service.marketplaceTestPage.Context(ctx).Eval(`()=>values`)
			if err != nil {
				t.Fatal(err)
			}
			v := result.Value
			if v.Get("make").Str() != f.Make && (kind != "car-truck" && kind != "motorcycle") {
				t.Fatalf("Free-text make not retained: %v", v)
			}
			if v.Get("model").Str() != f.Model || v.Get("mileage").Str() != f.Mileage || v.Get("price").Str() != f.Price || v.Get("nextClicks").Int() != 1 || v.Get("published").Int() != 0 || v.Get("extraClick").Bool() || !v.Get("location").Bool() || v.Get("photos").Int() != len(media.Photos) {
				t.Fatalf("Incorrect vehicle fill: %v", v)
			}
			transmissionSlot := "8"
			if kind == "motorcycle" {
				transmissionSlot = "5"
			}
			required := map[string]string{"0": f.VehicleType, "1": f.Year, "2": f.Make, transmissionSlot: f.Transmission}
			if kind != "car-truck" && kind != "motorcycle" {
				required = map[string]string{"0": f.VehicleType, "1": f.Year}
			}
			for slot, value := range required {
				if v.Get("selected").Map()[slot].Str() != value {
					t.Fatalf("Wrong vehicle selection %s: %v", slot, v)
				}
			}
			if all {
				expected := map[string]string{"3": f.BodyStyle, "4": f.ExteriorColor, "5": f.InteriorColor, "6": f.VehicleCondition, "7": f.FuelType}
				if kind == "motorcycle" {
					expected = map[string]string{"3": f.ExteriorColor, "4": f.FuelType}
				}
				if kind != "car-truck" && kind != "motorcycle" {
					expected = map[string]string{"2": f.ExteriorColor, "3": f.InteriorColor, "4": f.FuelType}
				}
				if kind == "trailer" {
					expected = map[string]string{"2": f.ExteriorColor, "3": f.InteriorColor}
				}
				if kind == "other" {
					expected = map[string]string{}
				}
				for slot, value := range expected {
					if v.Get("selected").Map()[slot].Str() != value {
						t.Fatalf("Wrong optional selection %s: %v", slot, v)
					}
				}
				if v.Get("clean").Bool() != (kind == "car-truck") || !v.Get("promote").Bool() || v.Get("description").Str() != f.Description || v.Get("video").Int() != 1 {
					t.Fatalf("Incorrect optional fields: %v", v)
				}
			} else if len(v.Get("selected").Map()) != len(required) || v.Get("clean").Bool() || v.Get("promote").Bool() {
				t.Fatalf("Empty preferences were changed: %v", v)
			}
		})
	}
}
