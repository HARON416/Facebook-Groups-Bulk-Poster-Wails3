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
	return MarketplaceHomeFields{HomeOffer: "sale", PropertyType: "house", Bedrooms: "3", Bathrooms: "2.5", Price: "950000", Address: "123 Cedar Street, Victoria, BC", Location: &MarketplaceLocationHit{PageID: "chosen", Label: "123 Cedar Street, Victoria, BC", Subtitle: "Address"}, Description: "Maison à vendre"}
}

func TestMarketplaceHomeValidation(t *testing.T) {
	f := validMarketplaceHome()
	if err := validateMarketplaceHomeFields(&f); err != nil {
		t.Fatal(err)
	}
	if err := validateMarketplaceHomeFields(nil); err == nil {
		t.Fatal("Missing home accepted")
	}
	for _, change := range []func(*MarketplaceHomeFields){
		func(f *MarketplaceHomeFields) { f.HomeOffer = "rent" }, func(f *MarketplaceHomeFields) { f.PropertyType = "invalid" }, func(f *MarketplaceHomeFields) { f.Bedrooms = "1.5" }, func(f *MarketplaceHomeFields) { f.Bathrooms = "2.25" }, func(f *MarketplaceHomeFields) { f.Price = "950000.5" }, func(f *MarketplaceHomeFields) { f.Address = "different address" }, func(f *MarketplaceHomeFields) { f.Location = nil }, func(f *MarketplaceHomeFields) { f.Description = " " }, func(f *MarketplaceHomeFields) { f.SquareFeet = "-1" }, func(f *MarketplaceHomeFields) { f.Heating = "invalid" },
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
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	capture, err := os.ReadFile("testdata/marketplace/home-sale.html")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"Home offer", "Property type", "Laundry", "Parking", "Air conditioning", "Heating"}
	choices := make([][]string, len(names))
	for i, name := range names {
		choices[i] = marketplaceHomeOptions[name]
	}
	data, _ := json.Marshal(choices)
	harness := `<script>
window.values={selected:{},photos:0,video:0,nextClicks:0,published:0};
const main=document.querySelector('[role=main]');const choices=CHOICES;
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
 document.getElementById('suggestions')?.remove();const list=document.createElement('ul');list.id='suggestions';list.role='listbox';addressInput.setAttribute('aria-controls',list.id);addressInput.setAttribute('aria-expanded','true');
 for(const subtitle of ['City','Address']){const row=document.createElement('li');row.role='option';row.innerHTML='<span dir="auto">123 Cedar Street, Victoria, BC</span><span>'+subtitle+'</span>';row.onclick=()=>{values.addressKind=subtitle;addressInput.value='123 Cedar Street, Victoria, BC';addressInput.setAttribute('aria-expanded','false');list.remove()};list.append(row)}document.body.append(list);
};
main.querySelector('input[accept*="image/"]').onchange=function(){values.photos=this.files.length};main.querySelector('input[accept*="video/"]').onchange=function(){values.video=this.files.length};
const progress=main.querySelector('[role=img]');progress.style.height='4px';let wrapper=progress.parentElement;while(wrapper&&(!wrapper.nextElementSibling||!wrapper.nextElementSibling.querySelector('[role=button]')))wrapper=wrapper.parentElement;
const next=wrapper.nextElementSibling.querySelector('[role=button]');next.removeAttribute('aria-disabled');next.onclick=()=>{
 values.nextClicks++;values.bedrooms=document.getElementById('_r_4t_').value;values.bathrooms=document.getElementById('_r_51_').value;values.price=document.getElementById('_r_55_').value;values.area=document.getElementById('_r_5l_').value;values.description=main.querySelector('textarea').value;values.promote=main.querySelector('[role=switch]').checked;
 const destination=document.createElement('div');destination.role='main';destination.innerHTML='<h1>次の画面</h1><div><div role="img"><div><div></div></div><div><div></div></div></div></div><div><button>戻る</button><button onclick="values.published++">公開</button></div>';main.replaceWith(destination);
};
</script>`
	harness = strings.Replace(harness, "CHOICES", string(data), 1)
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
			f := validMarketplaceHome()
			media := MarketplaceTestMedia{Photos: []PostUpload{{Name: "home.png", Data: fixturePNG}}, HomeFields: &f}
			if all {
				f.SquareFeet = "1250.5"
				f.Laundry = "in-unit"
				f.Parking = "garage"
				f.AirConditioning = "central"
				f.Heating = "electric"
				f.PromoteAfterPublish = true
				for len(media.Photos) < 50 {
					media.Photos = append(media.Photos, media.Photos[0])
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
			if v.Get("bedrooms").Str() != f.Bedrooms || v.Get("bathrooms").Str() != f.Bathrooms || v.Get("price").Str() != f.Price || v.Get("description").Str() != f.Description || v.Get("addressKind").Str() != "Address" || v.Get("photos").Int() != len(media.Photos) || v.Get("nextClicks").Int() != 1 || v.Get("published").Int() != 0 || v.Get("extraClick").Bool() {
				t.Fatalf("Incorrect home fill: %v", v)
			}
			expected := map[string]string{"0": "sale", "1": "house"}
			if all {
				expected["2"] = f.Laundry
				expected["3"] = f.Parking
				expected["4"] = f.AirConditioning
				expected["5"] = f.Heating
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
		})
	}
}
