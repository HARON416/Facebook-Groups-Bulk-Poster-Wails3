package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceItemFieldsValidation(t *testing.T) {
	valid := MarketplaceItemFields{Title: "Example", Price: "0", Category: "tools", Condition: "new"}
	if err := validateMarketplaceItemFields(&valid); err != nil {
		t.Fatal(err)
	}
	if err := validateMarketplaceItemFields(nil); err == nil {
		t.Fatal("missing fields accepted")
	}
	for _, change := range []func(*MarketplaceItemFields){
		func(f *MarketplaceItemFields) { f.Title = " " }, func(f *MarketplaceItemFields) { f.Price = "-1" }, func(f *MarketplaceItemFields) { f.Price = "1.5" }, func(f *MarketplaceItemFields) { f.Category = "vehicles" }, func(f *MarketplaceItemFields) { f.Condition = "unknown" }, func(f *MarketplaceItemFields) { f.Tags = make([]string, 21) }, func(f *MarketplaceItemFields) { f.Location = &MarketplaceLocationHit{Label: "unselected"} },
	} {
		fields := valid
		change(&fields)
		if err := validateMarketplaceItemFields(&fields); err == nil {
			t.Fatal("invalid fields accepted")
		}
	}
}

func TestMarketplaceFormattedPrice(t *testing.T) {
	for _, pair := range [][2]string{
		{"$29,999", "29999"}, {"$29,999.00", "29999"}, {"KSh 29,999.00", "29999"},
		{"29.999,00 €", "29999"}, {"29\u202f999,00 €", "29999"}, {"$1.00", "1"},
		{"\u200f٢٩٬٩٩٩٫٠٠ ج", "29999"}, {"₹२९९९९.००", "29999"}, {"$0.00", "0"}, {"Free", "0"}, {"$1,234", "1234"}, {"$29,999", "029999"},
	} {
		if !marketplacePriceEqual(pair[0], pair[1]) {
			t.Errorf("Rejected matching price %q for %q", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{
		{"$29,998", "29999"}, {"$1.50", "1.5"}, {"$1.00", "1.00"}, {"$299.99", "29999"}, {"-$29,999", "29999"},
		{"$29,99,9", "29999"}, {"Free", "20"}, {"unavailable", "0"}, {"$29,,999", "29999"},
		{"29,999,00", "29999"}, {"3e4", "34"},
	} {
		if marketplacePriceEqual(pair[0], pair[1]) {
			t.Errorf("Accepted different or malformed price %q for %q", pair[0], pair[1])
		}
	}
}

const marketplaceItemFieldsFixture = `<body class="UIPage_LoggedIn">
<header><label>Title<input value="outside listing"></label></header>
<div role="listbox" aria-label="Unrelated suggestions"><div role="option"><span>Household</span><span>Used - Good</span><span>List as In Stock</span></div></div>
<div role="dialog" aria-label="Dropdown menu" style="display:none"><div role="button"><span>Household</span></div></div>
<main><div role="form">
<input type="file" multiple accept="image/*" style="display:none" onchange="window.values.photos=this.files.length">
<input type="file" accept="video/*" style="display:none">
<label><span>Title</span><input type="text" aria-required="true" oninput="values.title=this.value"></label>
<label><span>Price</span><input type="text" aria-required="true" autocomplete="off" oninput="values.price=this.value"></label>
<label role="combobox" aria-labelledby="category-name" tabindex="0" onclick="dropdown('category',this)"><span id="category-name">Category</span><span class="selection"></span></label>
<label role="combobox" aria-labelledby="condition-name" aria-haspopup="listbox" aria-required="true" tabindex="0" onclick="dropdown('condition',this)"><span id="condition-name">Condition</span><span class="selection"></span></label>
<label><span>Description</span><textarea oninput="values.description=this.value">untouched</textarea></label>
<div role="button" aria-expanded="false" tabindex="0" onclick="this.setAttribute('aria-expanded','true');document.querySelector('#optional').hidden=false;values.expanded++"><span>More details</span></div>
<div id="optional" hidden>
<label role="combobox" aria-labelledby="availability-name" aria-haspopup="listbox" tabindex="0" onclick="dropdown('availability',this)"><span id="availability-name">Availability</span><span class="selection">List as Single Item</span></label>
<label><span>Product tags</span><div role="list"></div><textarea onkeydown="if(event.key==='Enter'){event.preventDefault();values.tags.push(this.value);const s=document.createElement('span');s.textContent=this.value;this.closest('label').querySelector('[role=list]').append(s);this.value=''}"></textarea></label>
<span id="sku-help">Internal reference</span><label><span>SKU</span><input type="text" aria-describedby="sku-help" oninput="values.sku=this.value"></label>
<input role="combobox" aria-label="Location" value="Default town" oninput="locationOptions()">
<div role="checkbox" aria-checked="true" tabindex="0" onclick="this.setAttribute('aria-checked',this.getAttribute('aria-checked')==='true'?'false':'true');values.meetupClicks++"><span>Public meetup</span></div>
<div role="checkbox" aria-checked="false" tabindex="0" onclick="this.setAttribute('aria-checked','true');values.pickup=true"><span>Door pickup</span></div>
<div role="checkbox" aria-checked="false" tabindex="0" onclick="this.setAttribute('aria-checked','true');values.dropoff=true"><span>Door dropoff</span></div>
</div>
<input type="checkbox" role="switch" aria-label="Promote listing after publish" onchange="values.promote=this.checked">
<input type="checkbox" role="switch" aria-label="Hide from friends" onchange="values.hide=this.checked">
</div>
<h1 id="fixture-step-heading">Item for sale</h1>
<div><div role="img" aria-label="Progress, currently on step 1 of 2"><div><div></div></div><div><div></div></div></div></div>
<div id="fixture-actions"><div role="button" aria-label="Next" tabindex="0" onclick="values.nextClicks=(values.nextClicks||0)+1;document.querySelector('[role=form]').hidden=true;document.querySelector('#fixture-step-heading').textContent='List in more places';this.parentElement.innerHTML='<button aria-label=Previous>Previous</button><button aria-label=Publish onclick=&quot;values.published=true&quot;>Publish</button>'">Next</div></div>
</main>
<script>
window.menuOrder=[];window.menuClicks={};window.values={title:'',price:'',description:'untouched',category:'',condition:'',availability:'single',tags:[],sku:'untouched',expanded:0,location:'default',meetupClicks:0,pickup:false,dropoff:false,promote:false,hide:false,published:false,photos:0};
function dropdown(key,control){
 window.menuClicks[key]=(window.menuClicks[key]||0)+1;
 if(window.ignoreFirstMenuClick && window.menuClicks[key]===1){control.replaceWith(control.cloneNode(true));return}
 if(control.getAttribute('aria-expanded')==='true'){values.closedByExtraClick=true;return}
 document.querySelector('[data-active-menu]')?.remove();
 window.menuOrder.push('open:'+key);
 control.setAttribute('aria-expanded','true');
 const menu=document.createElement('div');menu.setAttribute('data-active-menu','');menu.id='menu-'+Math.random().toString(36).slice(2);
 if(key==='category'){menu.setAttribute('role','dialog');menu.setAttribute('aria-label','Dropdown menu');const group=document.createElement('span');group.textContent='Home & Garden';menu.append(group)}
 else {menu.setAttribute('role','listbox');menu.setAttribute('aria-label',key==='condition'?'Condition':'Select an option');control.setAttribute('aria-controls',menu.id)}
 const choices={category:['Tools','Furniture','Household','Garden','Appliances','Video Games','Books, Movies & Music','Bags & Luggage',"Women's clothing & shoes","Men's clothing & shoes",'Jewelry & Accessories','Health & beauty','Pet Supplies','Baby & kids','Toys & Games','Electronics & computers','Mobile phones','Bicycles','Arts & Crafts','Sports & Outdoors','Auto parts','Musical Instruments','Antiques & Collectibles','Garage Sale','Miscellaneous','Vehicles'],condition:['New','Used - Like New','Used - Good','Used - Fair'],availability:['List as Single Item','List as In Stock']};
 for(const label of choices[key]){
  const option=document.createElement('div');option.setAttribute('role',key==='category'?'button':'option');option.tabIndex=0;
  if(key!=='category')option.setAttribute('aria-selected','false');
  const primary=document.createElement('span');primary.textContent=label;option.append(primary);
  if(label==='Household'||key==='availability'){const hint=document.createElement('span');hint.textContent=key==='availability'?'Description for '+label:'Shipping available';option.append(hint)}
  option.onclick=()=>{values[key]=label;window.menuOrder.push('select:'+key);control.querySelector('.selection').textContent=label;control.setAttribute('aria-expanded','false');control.replaceWith(control.cloneNode(true));menu.remove()};menu.append(option)
 }
 setTimeout(()=>document.body.append(menu),40)
}
function locationOptions(){
 document.querySelector('#location-suggestions')?.remove();
 const field=document.querySelector('input[aria-label="Location"]');
 const query=field.value;
 const menu=document.createElement('ul');menu.id='location-suggestions';menu.setAttribute('role','listbox');menu.setAttribute('aria-label','5 suggested searches');
 field.setAttribute('aria-controls',menu.id);field.setAttribute('aria-expanded','true');
 const choices=query==='Nairobi, Kenya' ? [['Nairobi, Kenya','City'],['Nairobi South, Nairobi Area, Kenya','Nairobi South, Nairobi, Kenya · 35,531 people checked in here'],['Nairobi West, Nairobi','Nairobi West, Nairobi, Kenya · 229,856 people checked in here']] : query==='Duplicate' ? [['Duplicate','City'],['Duplicate','City']] : [['Vancouver, BC, Canada','Province'],['Vancouver, BC, Canada','City']];
 for(const [label,subtitle] of choices){
  const row=document.createElement('li');row.setAttribute('role','option');row.setAttribute('aria-selected','false');row.id='suggestion-'+Math.random().toString(36).slice(2);
  const wrapper=document.createElement('div');wrapper.setAttribute('role','none');wrapper.setAttribute('aria-describedby',row.id);
  const option=document.createElement('div');option.setAttribute('role','option');option.setAttribute('aria-selected','false');
  for(const text of [label,subtitle]){const span=document.createElement('span');span.dir='auto';span.style.display='block';span.textContent=text;option.append(span)}
  option.onclick=()=>{values.location=subtitle;values.locationClicks=(values.locationClicks||0)+1;field.value=label;field.setAttribute('aria-expanded','false');menu.remove()};wrapper.append(option);row.append(wrapper);menu.append(row)
 }
 document.body.append(menu)
}
</script>`

func TestMarketplaceItemFieldsBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fixture := marketplaceItemFieldsFixture
		if r.URL.Path == "/formatted-price" {
			fixture = strings.Replace(fixture, `oninput="values.price=this.value"`, `data-fixture-price oninput="values.price=this.value;this.value=new Intl.NumberFormat('en-US',{style:'currency',currency:'USD',maximumFractionDigits:0}).format(Number(this.value));const fresh=this.cloneNode(true);fresh.value=this.value;this.replaceWith(fresh)"`, 1)
		}
		if r.URL.Path == "/late-click" {
			fixture = strings.Replace(fixture, "window.menuOrder=[];", "window.ignoreFirstMenuClick=true;window.menuOrder=[];", 1)
		}
		if r.URL.Path == "/condition-state-change" {
			fixture = strings.Replace(fixture, "option.onclick=()=>{values[key]=label;", "option.onclick=()=>{if(key==='condition')control.removeAttribute('aria-required');values[key]=label;", 1)
		}
		if r.URL.Path == "/publish-loading" {
			fixture = strings.Replace(fixture, "aria-label=Publish onclick=", "aria-label=Publish aria-disabled=true onclick=", 1)
			fixture += `<script>new MutationObserver(()=>{const button=document.querySelector('#fixture-actions [aria-disabled=true]');if(!button || button.dataset.pending)return;button.dataset.pending='true';setTimeout(()=>{button.removeAttribute('aria-disabled');values.publishReady=true},250)}).observe(document.querySelector('#fixture-actions'),{childList:true})</script>`
		}
		w.Write([]byte(fixture))
	}))
	defer server.Close()
	s := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = s.shutdown(true) })
	for _, all := range []bool{false, true} {
		name := "required-only"
		if all {
			name = "with-optional"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fields := MarketplaceItemFields{Title: "Desk <&> lamp", Price: "0", Category: "household", Condition: "used-good"}
			if all {
				fields.Description = "First line\nSecond line"
				fields.Availability = "in-stock"
				fields.Tags = []string{"desk", "home office"}
				fields.SKU = "SKU-123"
				fields.Location = &MarketplaceLocationHit{PageID: "selected", Label: "Vancouver, BC, Canada", Subtitle: "City"}
				fields.PublicMeetup = true
				fields.DoorPickup = true
				fields.DoorDropoff = true
				fields.PromoteAfterPublish = true
				fields.HideFromFriends = true
			}
			media := MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}
			if err := s.testMarketplaceItemMedia(ctx, server.URL, media); err != nil {
				t.Fatal(err)
			}
			result, err := s.marketplaceTestPage.Context(ctx).Eval(`() => window.values`)
			if err != nil {
				t.Fatal(err)
			}
			v := result.Value
			order, err := s.marketplaceTestPage.Context(ctx).Eval(`() => window.menuOrder.join(',')`)
			if err != nil {
				t.Fatal(err)
			}
			expectedOrder := "open:category,select:category,open:condition,select:condition"
			if all {
				expectedOrder += ",open:availability,select:availability"
			}
			if order.Value.Str() != expectedOrder {
				t.Fatalf("Wrong menu interaction order: %s", order.Value.Str())
			}
			if v.Get("title").Str() != fields.Title || v.Get("price").Str() != "0" || v.Get("category").Str() != "Household" || v.Get("condition").Str() != "Used - Good" || v.Get("photos").Int() != 1 || v.Get("nextClicks").Int() != 1 || v.Get("published").Bool() {
				t.Fatalf("Incorrect required fields: %v", v)
			}
			if all {
				if v.Get("description").Str() != fields.Description || v.Get("availability").Str() != "List as In Stock" || len(v.Get("tags").Arr()) != 2 || v.Get("sku").Str() != "SKU-123" || v.Get("location").Str() != "City" || v.Get("meetupClicks").Int() != 0 || !v.Get("pickup").Bool() || !v.Get("dropoff").Bool() || !v.Get("promote").Bool() || !v.Get("hide").Bool() {
					t.Fatalf("Incorrect optional fields: %v", v)
				}
			} else if v.Get("expanded").Int() != 0 || v.Get("description").Str() != "untouched" || v.Get("sku").Str() != "untouched" || v.Get("location").Str() != "default" {
				t.Fatalf("Empty optional fields were changed: %v", v)
			}
		})
	}
	t.Run("waits-for-publish-readiness-without-publishing", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		fields := MarketplaceItemFields{Title: "Publish readiness", Price: "20", Category: "tools", Condition: "new"}
		if err := s.testMarketplaceItemMedia(ctx, server.URL+"/publish-loading", MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}); err != nil {
			t.Fatal(err)
		}
		result, err := s.marketplaceTestPage.Context(ctx).Eval(`()=>window.values`)
		if err != nil || !result.Value.Get("publishReady").Bool() || result.Value.Get("published").Bool() || result.Value.Get("nextClicks").Int() != 1 {
			t.Fatalf("Test did not wait for Publish readiness without clicking: %v, %v", result, err)
		}
	})
	t.Run("condition-required-state-changes-after-selection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		fields := MarketplaceItemFields{Title: "State change", Price: "20", Category: "tools", Condition: "used-good", Availability: "in-stock", SKU: "REF-1"}
		if err := s.testMarketplaceItemMedia(ctx, server.URL+"/condition-state-change", MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}); err != nil {
			t.Fatal(err)
		}
		result, err := s.marketplaceTestPage.Context(ctx).Eval(`()=>({values,clicks:menuClicks,required:document.querySelector('[aria-labelledby="condition-name"]').hasAttribute('aria-required')})`)
		if err != nil {
			t.Fatal(err)
		}
		v := result.Value
		if v.Get("required").Bool() || v.Get("values").Get("condition").Str() != "Used - Good" || v.Get("values").Get("availability").Str() != "List as In Stock" || v.Get("clicks").Get("condition").Int() != 1 || v.Get("clicks").Get("availability").Int() != 1 || v.Get("values").Get("published").Bool() {
			t.Fatalf("Dropdowns overlapped after required-state change: %v", v)
		}
	})
	t.Run("single-item-availability", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		fields := MarketplaceItemFields{Title: "Single item", Price: "20", Category: "tools", Condition: "new", Availability: "single"}
		if err := s.testMarketplaceItemMedia(ctx, server.URL, MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}); err != nil {
			t.Fatal(err)
		}
		result, err := s.marketplaceTestPage.Context(ctx).Eval(`() => ({availability:window.values.availability,order:window.menuOrder.join(',')})`)
		if err != nil {
			t.Fatal(err)
		}
		if result.Value.Get("availability").Str() != "List as Single Item" || result.Value.Get("order").Str() != "open:category,select:category,open:condition,select:condition,open:availability,select:availability" {
			t.Fatalf("Single Item option was not selected through its menu: %v", result.Value)
		}
	})
	t.Run("retry-ignored-label-click", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		fields := MarketplaceItemFields{Title: "Retry test", Price: "20", Category: "tools", Condition: "new", Availability: "in-stock"}
		media := MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}
		if err := s.testMarketplaceItemMedia(ctx, server.URL+"/late-click", media); err != nil {
			t.Fatal(err)
		}
		result, err := s.marketplaceTestPage.Context(ctx).Eval(`() => ({clicks:window.menuClicks,closed:!!window.values.closedByExtraClick,category:window.values.category,condition:window.values.condition,availability:window.values.availability})`)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"category", "condition", "availability"} {
			if result.Value.Get("clicks").Get(key).Int() != 2 {
				t.Fatalf("Did not retry exactly one ignored %s label click: %v", key, result.Value)
			}
		}
		if result.Value.Get("closed").Bool() || result.Value.Get("category").Str() != "Tools" || result.Value.Get("condition").Str() != "New" || result.Value.Get("availability").Str() != "List as In Stock" {
			t.Fatalf("Dropdown sequence failed: %v", result.Value)
		}
	})

	t.Run("formatted-price-continues-to-dropdowns", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		fields := MarketplaceItemFields{Title: "iPhone 13 Pro Max", Price: "29999", Category: "tools", Condition: "new"}
		media := MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}
		if err := s.testMarketplaceItemMedia(ctx, server.URL+"/formatted-price", media); err != nil {
			t.Fatal(err)
		}
		result, err := s.marketplaceTestPage.Context(ctx).Eval(`() => ({display:document.querySelector('[data-fixture-price]').value,category:values.category,condition:values.condition})`)
		if err != nil {
			t.Fatal(err)
		}
		if result.Value.Get("display").Str() != "$29,999" || result.Value.Get("category").Str() != "Tools" || result.Value.Get("condition").Str() != "New" {
			t.Fatalf("Formatted price stopped the listing flow: %v", result.Value)
		}
	})

	t.Run("nested-nairobi-location-options", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		fields := MarketplaceItemFields{Title: "Nairobi test", Price: "100", Category: "tools", Condition: "new", Location: &MarketplaceLocationHit{PageID: "selected", Label: "Nairobi, Kenya", Subtitle: "City"}}
		if err := s.testMarketplaceItemMedia(ctx, server.URL, MarketplaceTestMedia{Photos: []PostUpload{{Name: "one.png", Data: fixturePNG}}, Fields: &fields}); err != nil {
			t.Fatal(err)
		}
		result, err := s.marketplaceTestPage.Context(ctx).Eval(`() => ({selected:values.location,clicks:values.locationClicks,open:document.querySelector('input[aria-label="Location"]').getAttribute('aria-expanded'),popup:!!document.querySelector('#location-suggestions')})`)
		if err != nil {
			t.Fatal(err)
		}
		if result.Value.Get("selected").Str() != "City" || result.Value.Get("clicks").Int() != 1 || result.Value.Get("open").Str() != "false" || result.Value.Get("popup").Bool() {
			t.Fatalf("Location was not selected exactly once: %v", result.Value)
		}
	})
	t.Run("different-location-rows-remain-ambiguous", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		page, err := s.prepareMarketplaceTestPage(ctx, server.URL)
		if err != nil {
			t.Fatal(err)
		}
		if err = page.Context(ctx).WaitLoad(); err != nil {
			t.Fatal(err)
		}
		if err = marketplaceMoreDetails(ctx, page); err != nil {
			t.Fatal(err)
		}
		if err = marketplaceText(ctx, page, "Location", "Duplicate"); err != nil {
			t.Fatal(err)
		}
		_, err = page.Context(ctx).ElementByJS(rod.Eval(marketplaceOption, "Duplicate", "City"))
		if err == nil || !strings.Contains(err.Error(), "Multiple matching options") {
			t.Fatalf("Two real rows were incorrectly merged: %v", err)
		}
	})

}
