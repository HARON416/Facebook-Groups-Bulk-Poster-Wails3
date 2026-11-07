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

func validMarketplaceHome() MarketplaceHomeFields {
	return MarketplaceHomeFields{HomeOffer: "sale", PropertyType: "house", Bedrooms: "3", Bathrooms: "2.5", Price: "950000", Address: "123 Cedar Street, Victoria, BC", Location: &MarketplaceLocationHit{PageID: "123456789", Label: "123 Cedar Street, Victoria, BC", Subtitle: "Address", SearchType: "STREET_PLACE_TYPEAHEAD"}, Description: "Maison à vendre"}
}

func TestMarketplaceHomeValidation(t *testing.T) {
	f := validMarketplaceHome()
	if err := validateMarketplaceHomeFields(&f); err != nil {
		t.Fatal(err)
	}
	if err := validateMarketplaceHomeFields(nil); err == nil {
		t.Fatal("Missing home accepted")
	}
	rental := f
	rental.HomeOffer = "rent"
	rental.DateAvailable = "2027-04-21"
	if err := validateMarketplaceHomeFields(&rental); err != nil {
		t.Fatalf("Whole-property rental rejected: %v", err)
	}
	rental.PrivateRoom = true
	if err := validateMarketplaceHomeFields(&rental); err == nil {
		t.Fatal("Private-room rental without room details accepted")
	}
	rental.Occupants, rental.BathroomType = "2", "private"
	if err := validateMarketplaceHomeFields(&rental); err != nil {
		t.Fatalf("Valid private-room rental rejected: %v", err)
	}
	for _, values := range [][2]string{{"-1", "private"}, {"1.5", "shared"}, {"2", "unknown"}} {
		invalid := rental
		invalid.Occupants, invalid.BathroomType = values[0], values[1]
		if err := validateMarketplaceHomeFields(&invalid); err == nil {
			t.Fatalf("Invalid private-room details accepted: %+v", invalid)
		}
	}
	rental.PrivateRoom = false
	rental.DateAvailable = "2027-02-30"
	if err := validateMarketplaceHomeFields(&rental); err == nil {
		t.Fatal("Invalid rental date accepted")
	}
	for _, source := range []string{"", "PLACE_TYPEAHEAD"} {
		legacy := validMarketplaceHome()
		legacy.Location.SearchType = source
		if err := validateMarketplaceHomeFields(&legacy); err == nil {
			t.Fatal("A home reused a city-search selection")
		}
	}
	for _, change := range []func(*MarketplaceHomeFields){
		func(f *MarketplaceHomeFields) { f.HomeOffer = "invalid" }, func(f *MarketplaceHomeFields) { f.PropertyType = "invalid" }, func(f *MarketplaceHomeFields) { f.Bedrooms = "1.5" }, func(f *MarketplaceHomeFields) { f.Bathrooms = "2.25" }, func(f *MarketplaceHomeFields) { f.Price = "950000.5" }, func(f *MarketplaceHomeFields) { f.Address = "different address" }, func(f *MarketplaceHomeFields) { f.Location = nil }, func(f *MarketplaceHomeFields) { f.Description = " " }, func(f *MarketplaceHomeFields) { f.SquareFeet = "-1" }, func(f *MarketplaceHomeFields) { f.Heating = "invalid" },
	} {
		invalid := f
		change(&invalid)
		if err := validateMarketplaceHomeFields(&invalid); err == nil {
			t.Fatalf("Invalid home accepted: %+v", invalid)
		}
	}
	photos := make([]PostUpload, 50)
	for i := range photos {
		photos[i] = PostUpload{Name: "home.png", Data: fixturePNG}
	}
	dir, _, _, err := stageMarketplaceTestMedia(MarketplaceTestMedia{Photos: photos, HomeFields: &f})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if _, _, _, err := stageMarketplaceTestMedia(MarketplaceTestMedia{Photos: append(photos, photos[0]), HomeFields: &f}); err == nil {
		t.Fatal("51 home photos accepted")
	}
	if _, _, _, err := stageMarketplaceTestMedia(MarketplaceTestMedia{Photos: photos}); err == nil {
		t.Fatal("Item media limit changed")
	}
	vehicle := validMarketplaceVehicle()
	service := &GroupsService{}
	if err := service.testMarketplaceItemMedia(context.Background(), marketplaceTestHomeURL, MarketplaceTestMedia{HomeFields: &f, VehicleFields: &vehicle}); err == nil {
		t.Fatal("Mixed home/vehicle payload accepted")
	}
}

func TestMarketplaceHomeSaleBrowserIntegration(t *testing.T) {
	testMarketplaceHomeBrowserIntegration(t, "sale")
}

func TestMarketplaceHomeRentBrowserIntegration(t *testing.T) {
	testMarketplaceHomeBrowserIntegration(t, "rent")
}

func TestMarketplaceHomeRoomBrowserIntegration(t *testing.T) {
	testMarketplaceHomeBrowserIntegration(t, "room")
}

func testMarketplaceHomeBrowserIntegration(t *testing.T, offer string) {
	t.Helper()
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	capture, err := os.ReadFile("testdata/marketplace/home-" + offer + ".html")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"Home offer", "Property type", "Laundry", "Parking", "Air conditioning", "Heating"}
	privateRoom := offer == "room"
	if privateRoom {
		names = []string{"Home offer", "Property type", "Bathroom type", "Laundry", "Parking", "Air conditioning", "Heating"}
	}
	choices := make([][]string, len(names))
	for i, name := range names {
		choices[i] = marketplaceHomeOptions[name]
	}
	data, _ := json.Marshal(choices)
	harness := `<script>
window.values={selected:{},photos:0,video:0,nextClicks:0,published:0};
const main=document.querySelector('[role=main]');const choices=CHOICES;
const ids=FIELD_IDS;
const switches=[...main.querySelectorAll('input[role=switch]')];
const privateRental=switches.length===5;const petSlot=privateRental?2:1;
let roomControls=[];
if(privateRental){
 // The supplied room capture was already filled. Model a fresh listing before
 // exercising required-only versus optional fields.
 for(const field of main.querySelectorAll('input[type=text],textarea'))field.value='';
 switches[0].checked=false;switches[0].setAttribute('aria-checked','false');
 roomControls=[document.getElementById('_r_a7_').closest('label'),main.querySelectorAll('label[role=combobox]')[2],switches[1]];
 for(const field of roomControls)field.hidden=true;
 switches[1].checked=new URLSearchParams(location.search).get('prefill')==='true';switches[1].setAttribute('aria-checked',String(switches[1].checked));
}
if(switches.length>=4){
 const prefill=new URLSearchParams(location.search).get('prefill')==='true';
 for(const field of switches.slice(petSlot,petSlot+2)){field.checked=prefill;field.setAttribute('aria-checked',String(prefill))}
 const date=main.querySelector('input[aria-autocomplete=none]');date.onblur=()=>{
  values.date=date.value;const parts=date.value.split('/').map(Number);const british=document.documentElement.lang==='en-GB';
  const month=['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'][parts[british?1:0]-1];const day=parts[british?0:1];
  date.value=british?day+' '+month+' '+parts[2]:month+' '+day+', '+parts[2];
 };
}
for(const field of switches){field.onchange=()=>{
 field.setAttribute('aria-checked',String(field.checked));
 if(privateRental&&field===switches[0]){for(const control of roomControls)control.hidden=!field.checked;values.photos=0;values.video=0;values.modeChanges=(values.modeChanges||0)+1}
 if(field===switches.at(-1)&&new URLSearchParams(location.search).get('reset-pets')==='true'){switches[petSlot].checked=false;switches[petSlot].setAttribute('aria-checked','false')}
}}
[...main.querySelectorAll('label[role=combobox]')].forEach((control,field)=>{
 const caption=document.createElement('span');caption.dataset.selection='';control.append(caption);
 control.onclick=()=>{
  if(document.querySelector('[data-popup]')){values.extraClick=true;return}
  const menu=document.createElement('div');menu.role='listbox';menu.dataset.popup='';
  choices[field].forEach((value,index)=>{
   const option=document.createElement('div');option.role='option';option.tabIndex=0;option.id='changed-'+field+'___'+index;option.setAttribute('aria-selected','false');
   const label=document.createElement('span');label.textContent='選択 '+field+' '+index;option.append(label);
   option.onclick=()=>{if(field===0){values.photos=0;values.video=0}values.selected[field]=value;control.querySelector('[data-selection]').textContent=label.textContent;control.setAttribute('aria-expanded','false');menu.remove();control.replaceWith(control.cloneNode(true))};menu.prepend(option);
  });setTimeout(()=>document.body.append(menu),40);
 };
});
const addressInput=main.querySelector('input[role=combobox]');
addressInput.oninput=()=>{
 fetch('/api/graphql/',{method:'POST',body:new URLSearchParams({fb_api_req_friendly_name:'MarketplaceSearchAddressDataSourceQuery',variables:JSON.stringify({params:{caller:'MARKETPLACE',query:addressInput.value,search_type:'STREET_PLACE_TYPEAHEAD'}})})});
 document.getElementById('suggestions')?.remove();const list=document.createElement('ul');list.id='suggestions';list.role='listbox';addressInput.setAttribute('aria-controls',list.id);addressInput.setAttribute('aria-expanded','true');
 for(const subtitle of ['City','Address']){const row=document.createElement('li');row.role='option';row.innerHTML='<span dir="auto">123 Cedar Street, Victoria, BC</span><span>'+subtitle+'</span>';row.onclick=()=>{values.addressKind=subtitle;addressInput.value='123 Cedar Street, Victoria, BC';addressInput.setAttribute('aria-expanded','false');list.remove()};list.append(row)}document.body.append(list);
};
main.querySelector('input[accept*="image/"]').onchange=function(){values.photos=this.files.length};main.querySelector('input[accept*="video/"]').onchange=function(){values.video=this.files.length};
const progress=main.querySelector('[role=img]');progress.style.height='4px';let wrapper=progress.parentElement;while(wrapper&&(!wrapper.nextElementSibling||!wrapper.nextElementSibling.querySelector('[role=button]')))wrapper=wrapper.parentElement;
const next=wrapper.nextElementSibling.querySelector('[role=button]');next.removeAttribute('aria-disabled');next.onclick=()=>{
 values.nextClicks++;values.bedrooms=document.getElementById(ids[0]).value;values.bathrooms=document.getElementById(ids[1]).value;values.price=document.getElementById(ids[2]).value;values.area=document.getElementById(ids[3]).value;values.description=main.querySelector('textarea').value;values.promote=switches.at(-1).checked;
 if(switches.length>=4){values.privateRoom=switches[0].getAttribute('aria-checked')==='true';values.cat=switches[petSlot].getAttribute('aria-checked')==='true';values.dog=switches[petSlot+1].getAttribute('aria-checked')==='true'}
 if(privateRental){values.occupants=document.getElementById('_r_a7_').value;values.furnished=switches[1].getAttribute('aria-checked')==='true'}
 const destination=document.createElement('div');destination.role='main';destination.innerHTML='<h1>次の画面</h1><div><div role="img"><div><div></div></div><div><div></div></div></div></div><div><button>戻る</button><button onclick="values.published++">公開</button></div>';main.replaceWith(destination);
};
</script>`
	harness = strings.Replace(harness, "CHOICES", string(data), 1)
	ids := []string{"_r_4t_", "_r_51_", "_r_55_", "_r_5l_"}
	if offer == "rent" {
		ids = []string{"_r_2k_", "_r_2o_", "_r_63_", "_r_3c_"}
	} else if privateRoom {
		ids = []string{"_r_8d_", "_r_8h_", "_r_8l_", "_r_95_"}
	}
	fieldIDs, _ := json.Marshal(ids)
	harness = strings.Replace(harness, "FIELD_IDS", string(fieldIDs), 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/graphql/" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"city_street_search":{"street_results":{"edges":[{"node":{"single_line_address":"123 Cedar Street, Victoria, BC","subtitle":"Address","page":{"id":"123456789"},"location":{"latitude":48.4,"longitude":-123.3}}}]}}}}`)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		locale := r.URL.Query().Get("locale")
		if locale == "" {
			locale = "en-US"
		}
		fmt.Fprintf(w, `<html dir="rtl" lang=%q><body class="UIPage_LoggedIn">%s%s`, locale, string(capture), harness)
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	videoData, err := os.ReadFile("frontend/testdata/vehicle-short.webm")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		all       bool
		locale    string
		resetPets bool
	}{{false, "en-US", false}, {true, "en-US", false}}
	if offer == "rent" || privateRoom {
		cases = append(cases, struct {
			all       bool
			locale    string
			resetPets bool
		}{true, "en-GB", false}, struct {
			all       bool
			locale    string
			resetPets bool
		}{true, "en-US", true})
	}
	for _, tc := range cases {
		all := tc.all
		t.Run(fmt.Sprintf("optional-%v-%s-reset-pets-%t", all, tc.locale, tc.resetPets), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			f := validMarketplaceHome()
			f.HomeOffer = offer
			if privateRoom {
				f.HomeOffer, f.PrivateRoom = "rent", true
				f.Occupants, f.BathroomType = "2", "private"
			}
			media := MarketplaceTestMedia{Photos: []PostUpload{{Name: "home.png", Data: fixturePNG}}, HomeFields: &f}
			if all {
				f.SquareFeet = "1250.5"
				f.Laundry = "in-unit"
				f.Parking = "garage"
				f.AirConditioning = "central"
				f.Heating = "electric"
				f.PromoteAfterPublish = true
				if f.HomeOffer == "rent" {
					f.DateAvailable = "2027-04-21"
					f.CatFriendly, f.DogFriendly = true, true
					if privateRoom {
						f.BathroomType, f.RoomFurnished = "shared", true
					}
				}
				for len(media.Photos) < 50 {
					media.Photos = append(media.Photos, media.Photos[0])
				}
				media.Video = &PostUpload{Name: "clip.webm", Data: base64.StdEncoding.EncodeToString(videoData)}
			}
			err := service.testMarketplaceItemMedia(ctx, fmt.Sprintf("%s?locale=%s&prefill=%t&reset-pets=%t", server.URL, tc.locale, !all, tc.resetPets), media)
			if tc.resetPets {
				if err == nil || !strings.Contains(err.Error(), "cat and dog settings") {
					t.Fatalf("A later pet-state reset must fail preparation: %v", err)
				}
				result, readErr := service.marketplaceTestPage.Context(ctx).Eval(`()=>values.nextClicks===0 && values.published===0`)
				if readErr != nil || !result.Value.Bool() {
					t.Fatalf("Advanced past an unconfirmed pet setting: %v %v", result, readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.marketplaceTestPage.Context(ctx).Eval(`()=>values`)
			if err != nil {
				t.Fatal(err)
			}
			v := result.Value
			if v.Get("bedrooms").Str() != f.Bedrooms || v.Get("bathrooms").Str() != f.Bathrooms || v.Get("price").Str() != f.Price || v.Get("description").Str() != f.Description || v.Get("addressKind").Str() != "Address" || v.Get("photos").Int() != len(media.Photos) || v.Get("nextClicks").Int() != 1 || v.Get("published").Int() != 0 || v.Get("extraClick").Bool() {
				t.Fatalf("Incorrect home fill: %v", v)
			}
			expected := map[string]string{"0": f.HomeOffer, "1": "house"}
			if privateRoom {
				expected["2"] = f.BathroomType
			}
			if all {
				for i, name := range names {
					value := map[string]string{"Laundry": f.Laundry, "Parking": f.Parking, "Air conditioning": f.AirConditioning, "Heating": f.Heating}[name]
					if value != "" {
						expected[fmt.Sprint(i)] = value
					}
				}
			}
			if len(v.Get("selected").Map()) != len(expected) {
				t.Fatalf("Unexpected home selections: %v", v)
			}
			for slot, value := range expected {
				if v.Get("selected").Map()[slot].Str() != value {
					t.Fatalf("Wrong home selection %s: %v", slot, v)
				}
			}
			if v.Get("area").Str() != f.SquareFeet || v.Get("promote").Bool() != all || v.Get("video").Int() != map[bool]int{false: 0, true: 1}[all] {
				t.Fatalf("Incorrect home optional fields: %v", v)
			}
			if f.HomeOffer == "rent" {
				if v.Get("privateRoom").Bool() != f.PrivateRoom || v.Get("cat").Bool() != f.CatFriendly || v.Get("dog").Bool() != f.DogFriendly {
					t.Fatalf("Incorrect rental switches: %v", v)
				}
				if privateRoom && (v.Get("occupants").Str() != f.Occupants || v.Get("furnished").Bool() != f.RoomFurnished || v.Get("modeChanges").Int() != 1) {
					t.Fatalf("Incorrect private-room fields/mode preparation: %v", v)
				}
				if all {
					wantDate := "04/21/2027"
					if tc.locale == "en-GB" {
						wantDate = "21/04/2027"
					}
					if v.Get("date").Str() != wantDate {
						t.Fatalf("Wrong availability date: got %s, want %s", v.Get("date").Str(), wantDate)
					}
				}
			}
		})
	}
}
