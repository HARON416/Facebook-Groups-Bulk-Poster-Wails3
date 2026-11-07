package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceDropdownNoDeadlineBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<body class="UIPage_LoggedIn"><main>
<label role="combobox" aria-haspopup="listbox" aria-labelledby="type-caption" aria-disabled="true" aria-expanded="false" tabindex="0" onclick="openMenu(this)"><span id="type-caption">種類</span></label>
<input type="file" accept="image/*" hidden>
</main><script>window.clicks=0;function openMenu(control){clicks++;if(control.getAttribute('aria-disabled')==='true')return;control.setAttribute('aria-expanded','true');const menu=document.createElement('div');menu.role='listbox';menu.id='menu';for(let i=0;i<8;i++){const option=document.createElement('div');option.role='option';option.id='fresh___'+i;option.textContent='種類 '+i;menu.append(option)}document.body.append(menu)}</script></body>`))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Context(ctx).Eval(`()=>setTimeout(()=>document.querySelector('label').removeAttribute('aria-disabled'),26000)`); err != nil {
		t.Fatal(err)
	}
	if err := marketplaceOpenDropdownWith(ctx, page, "Vehicle type", 8, marketplaceVehicleControlFor("car-truck")); err != nil {
		t.Fatalf("Slow dropdown should outlive the former 25-second deadline: %v", err)
	}
	result, err := page.Context(ctx).Eval(`()=>window.clicks`)
	if err != nil || result.Value.Int() != 1 {
		t.Fatalf("Expected one enabled-control click: %v %v", result, err)
	}
	if _, err := page.Context(ctx).Eval(`()=>{document.getElementById('menu').remove();const control=document.querySelector('label');control.setAttribute('aria-expanded','false');control.setAttribute('aria-disabled','true');clicks=0}`); err != nil {
		t.Fatal(err)
	}
	stopped, stop := context.WithCancel(ctx)
	stopTimer := time.AfterFunc(100*time.Millisecond, stop)
	if err := marketplaceOpenDropdownWith(stopped, page, "Vehicle type", 8, marketplaceVehicleControlFor("car-truck")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Caller cancellation ignored: %v", err)
	}
	stopTimer.Stop()
	stop()
	done := make(chan error, 1)
	go func() {
		done <- marketplaceOpenDropdownWith(ctx, page, "Vehicle type", 8, marketplaceVehicleControlFor("car-truck"))
	}()
	if err := page.Context(ctx).Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("Browser closure was not detected: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Dropdown wait continued after browser closure")
	}
}

func TestMarketplaceExpandedCaptureBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	capture, err := os.ReadFile("testdata/marketplace/item-expanded.html")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(append([]byte(`<body class="UIPage_LoggedIn">`), capture...))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	// Removing required-state metadata makes Condition overlap the old
	// Availability selector. Also include an unrelated optional dropdown later.
	if _, err := page.Context(ctx).Eval(`()=>{
 document.querySelector('label[aria-labelledby="_r_6e_"]').removeAttribute('aria-required');
 document.querySelector('[role=main]').insertAdjacentHTML('beforeend','<label role="combobox" aria-haspopup="listbox" aria-labelledby="extra-name"><span id="extra-name">別の選択</span></label>');
}`); err != nil {
		t.Fatal(err)
	}
	for name, labelID := range map[string]string{"Condition": "_r_6e_", "Availability": "_r_a9_"} {
		field, err := marketplaceField(ctx, page, name, `label[role="combobox"]`)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := field.Attribute("aria-labelledby")
		if err != nil || actual == nil || *actual != labelID {
			t.Fatalf("Wrong %s control after required-state change: %v, %v", name, actual, err)
		}
	}
	for name, id := range map[string]string{"Title": "_r_5v_", "Price": "_r_63_", "Description": "_r_a2_", "Product tags": "_r_ac_", "SKU": "_r_ag_", "Location": "_r_an_"} {
		field, err := marketplaceField(ctx, page, name, "input,textarea")
		if err != nil {
			t.Fatal(err)
		}
		actual, err := field.Attribute("id")
		if err != nil || actual == nil || *actual != id {
			t.Fatalf("Wrong %s control in expanded capture: %v, %v", name, actual, err)
		}
	}
	// Extra text controls can appear after React input handling. Neither ordinary
	// inputs nor broken/out-of-composer description links identify the SKU.
	if _, err := page.Context(ctx).Eval(`()=>{
 const root=document.querySelector('[role=main]');
 root.insertAdjacentHTML('beforeend','<input id="decoy" type="text"><input type="text" aria-describedby="missing"><input type="text" aria-describedby="outside-help">');
 document.body.insertAdjacentHTML('beforeend','<span id="outside-help">مساعدة</span>');
 const sku=document.getElementById('_r_ag_');
 const help=document.getElementById(sku.getAttribute('aria-describedby'));
 help.id='fresh-help-'+Math.random().toString(36).slice(2);sku.setAttribute('aria-describedby',help.id);sku.id='fresh-sku';
}`); err != nil {
		t.Fatal(err)
	}
	if err := marketplaceText(ctx, page, "SKU", "REF-123"); err != nil {
		t.Fatal(err)
	}
	result, err := page.Context(ctx).Eval(`()=>({sku:document.getElementById('fresh-sku').value,decoy:document.getElementById('decoy').value,title:document.getElementById('_r_5v_').value})`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Value.Get("sku").Str() != "REF-123" || result.Value.Get("decoy").Str() != "" || result.Value.Get("title").Str() != "" {
		t.Fatalf("SKU input affected another field: %v", result.Value)
	}
}

func TestMarketplaceAvailabilityStaleExpandedBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	// Reproduce the supplied control: no popup relationship, a permanently false
	// aria-expanded value, nested selected caption, and indexed descriptive rows.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<body class="UIPage_LoggedIn"><main>
<label role="combobox" tabindex="0" aria-haspopup="listbox" aria-expanded="false" aria-labelledby="dynamic-label" onclick="window.clicks++;if(!menu.hidden){menu.hidden=true;return}setTimeout(()=>menu.hidden=false,250)"><div><span id="dynamic-label">التوفر</span><div tabindex="-1"><div><span id="selection">واحد</span></div></div></div></label>
</main><div id="menu" role="listbox" aria-label="اختيار" hidden>
<div role="option" aria-selected="true" id="fresh___0" tabindex="0" onclick="selectOption(this)"><div><div><span>واحد</span></div><div><span>شرح</span></div></div></div>
<div role="option" aria-selected="false" id="fresh___1" tabindex="-1" onclick="selectOption(this)"><div><div><span>متوفر</span></div><div><span>شرح آخر</span></div></div></div>
</div><div role="listbox"><div role="option">A</div><div role="option">B</div><div role="option">C</div><div role="option">D</div></div>
<script>window.clicks=0;window.selected=-1;function selectOption(el){selected=Number(el.id.split('___')[1]);document.querySelector('#selection').textContent=el.querySelector('span').textContent;menu.hidden=true}</script></body>`))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Context(ctx).Wait(rod.Eval(`()=>typeof window.selected==='number'`)); err != nil {
		t.Fatal(err)
	}
	for _, preopened := range []bool{false, true} {
		for index, value := range []string{"single", "in-stock"} {
			if _, err := page.Context(ctx).Eval(`(opened)=>{menu.hidden=!opened;clicks=0;selected=-1}`, preopened); err != nil {
				t.Fatal(err)
			}
			if index == 0 {
				if _, err := page.Context(ctx).Eval(`()=>{const duplicate=document.querySelector('label').cloneNode(true);document.querySelector('main').append(duplicate);setTimeout(()=>duplicate.remove(),500)}`); err != nil {
					t.Fatal(err)
				}
			}
			if err := marketplaceChoose(ctx, page, "Availability", value); err != nil {
				t.Fatal(err)
			}
			result, err := page.Context(ctx).Eval(`()=>({clicks,selected,hidden:menu.hidden,expanded:document.querySelector('label').getAttribute('aria-expanded')})`)
			if err != nil {
				t.Fatal(err)
			}
			wantClicks := 1
			if preopened {
				wantClicks = 0
			}
			if result.Value.Get("clicks").Int() != wantClicks || result.Value.Get("selected").Int() != index || !result.Value.Get("hidden").Bool() || result.Value.Get("expanded").Str() != "false" {
				t.Fatalf("Incorrect stale-expanded handling (preopened=%v, value=%s): %v", preopened, value, result.Value)
			}
		}
	}
	if _, err := page.Context(ctx).Eval(`()=>{menu.hidden=true;clicks=0;document.querySelector('main').append(document.querySelector('label').cloneNode(true))}`); err != nil {
		t.Fatal(err)
	}
	short, stop := context.WithTimeout(ctx, 500*time.Millisecond)
	defer stop()
	if err := marketplaceOpenDropdown(short, page, "Availability"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Persistent ambiguity should time out: %v", err)
	}
	result, err := page.Context(ctx).Eval(`()=>window.clicks`)
	if err != nil || result.Value.Int() != 0 {
		t.Fatalf("Ambiguous controls must not be clicked: %v, %v", result, err)
	}
}

func TestMarketplaceItemCategoryExtrasBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	capture, err := os.ReadFile("testdata/marketplace/item-category-extras.html")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(capture)
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Context(ctx).WaitLoad(); err != nil {
		t.Fatal(err)
	}
	// Use the supplied DOM structure with anonymized captions. Fail immediately
	// if any ignored category field receives focus, a click, or input.
	_, err = page.Context(ctx).Eval(`()=>{
 window.extraEvents=0;window.availabilityClicks=0;
 for(const id of ['_r_6l_','_r_6o_','_r_6s_','_r_70_']){
  const el=document.getElementById(id);const control=el.closest('label')||el;
  for(const event of ['click','focusin','input'])control.addEventListener(event,()=>extraEvents++);
 }
 const control=document.getElementById('_r_5v_').closest('label');
 control.onclick=()=>{availabilityClicks++;control.setAttribute('aria-expanded','true');const menu=document.createElement('div');menu.role='listbox';menu.id='availability-options';control.setAttribute('aria-controls',menu.id);
 for(let i=0;i<2;i++){const option=document.createElement('div');option.role='option';option.id='fresh___'+i;option.textContent='Option '+i;option.onclick=()=>{window.chosen=i;const selected=document.createElement('span');selected.textContent=option.textContent;control.append(selected);control.setAttribute('aria-expanded','false');menu.remove()};menu.append(option)}document.body.append(menu)};
}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := marketplaceChoose(ctx, page, "Availability", "in-stock"); err != nil {
		t.Fatal(err)
	}
	if err := marketplaceText(ctx, page, "SKU", "REF-123"); err != nil {
		t.Fatal(err)
	}
	result, err := page.Context(ctx).Eval(`()=>({extraEvents,availabilityClicks,chosen,sku:document.getElementById('_r_66_').value})`)
	if err != nil {
		t.Fatal(err)
	}
	v := result.Value
	if v.Get("extraEvents").Int() != 0 || v.Get("availabilityClicks").Int() != 1 || v.Get("chosen").Int() != 1 || v.Get("sku").Str() != "REF-123" {
		t.Fatalf("Category fields were confused with supported fields: %v", v)
	}
}
