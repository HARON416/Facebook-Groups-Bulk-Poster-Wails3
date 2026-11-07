package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceLocationScrollBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	capture, err := os.ReadFile("testdata/marketplace/location-suggestions.html")
	if err != nil {
		t.Fatal(err)
	}
	geoBody, err := os.ReadFile("testdata/marketplace/home-geocoded-addresses.json")
	if err != nil {
		t.Fatal(err)
	}
	geoHits, err := parseMarketplaceLocations(geoBody, true)
	if err != nil || len(geoHits) != 3 {
		t.Fatalf("Invalid geocode fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/graphql/" {
			if strings.Contains(r.Referer(), "network-home-geocode") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(geoBody)
				return
			}
			_ = r.ParseForm()
			if strings.Contains(r.Referer(), "network-home-context") {
				var variables struct {
					Params struct {
						Query             string `json:"query"`
						Provider          string `json:"provider"`
						ResultOrdering    string `json:"result_ordering"`
						ViewerCoordinates struct {
							Latitude  float64 `json:"latitude"`
							Longitude float64 `json:"longitude"`
						} `json:"viewer_coordinates"`
					} `json:"params"`
				}
				_ = json.Unmarshal([]byte(r.Form.Get("variables")), &variables)
				p := variables.Params
				if p.Query == "Mombasa, Kenya " && p.Provider == "HERE_THRIFT" && p.ResultOrdering == "ADDRESS_FIRST" && p.ViewerCoordinates.Latitude == 49.2819 && p.ViewerCoordinates.Longitude == -123.1204 {
					if r.Form.Get("fb_dtsg") != "current-token" || r.Form.Get("doc_id") != "current-doc" {
						t.Error("Saved search overwrote current authentication/document fields")
					}
					_, _ = w.Write([]byte(`{"data":{"city_street_search":{"street_results":{"edges":[{"node":{"single_line_address":"Mombasa, Kenya","subtitle":"City","page":{"id":"109627399063416"},"location":{"latitude":-4.04,"longitude":39.67,"reverse_geocode_detailed":{"city":"Mombasa"}}}}]}}}}`))
					return
				}
			}
			subtitle := "City"
			if strings.Contains(r.Form.Get("variables"), "Count") {
				subtitle = "Langley, BC, Canada · 15,952 people checked in here"
			}
			w.Header().Set("Content-Type", "application/json")
			id := "114751268540391"
			if strings.Contains(r.Referer(), "network-home-") && !strings.Contains(r.Form.Get("variables"), `"query":"Langley"`) {
				id = "888888"
			}
			fmt.Fprintf(w, `{"data":{"city_street_search":{"street_results":{"edges":[{"node":{"single_line_address":"Langley, BC, Canada","subtitle":%q,"page":{"id":%q},"location":{"latitude":49.1,"longitude":-122.6,"reverse_geocode_detailed":{"city":"Langley"}}}}]}}}}`, subtitle, id)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<body><main style="display:flex"><div id="sidebar" style="height:180px;width:350px;overflow:auto"><label>Adresse<input type="text" role="combobox" aria-autocomplete="list" autocomplete="off" aria-expanded="false" aria-controls="suggestions" oninput="openResults(this)"></label></div><div id="preview" style="height:180px;overflow:auto"><div style="height:1500px">Preview</div></div></main><script>
window.chosen='';window.clicks=0;window.loaded=false;window.replaced=false;window.scrolls=0;window.offscreenClick=false;window.queries=[];const mode=location.pathname.slice(1);
function row(label,subtitle){const li=document.createElement('li');li.role='option';li.style.height='44px';li.innerHTML='<div role="none"><div role="option"><span dir="auto">'+label+'</span><span>'+subtitle+'</span></div></div>';return li}
function addTargets(menu){menu.append(row('Langley, BC, Canada','4,651 people'),row('Langley, BC, Canada','19,951 people'))}
function openResults(field){
 queries.push(field.value);
 document.getElementById('suggestions')?.remove();field.setAttribute('aria-expanded','true');const menu=document.createElement('ul');menu.role='listbox';menu.id='suggestions';menu.style.margin='0';menu.style.padding='0';
 if(mode==='menu'||mode==='replace'){menu.style.height='120px';menu.style.overflowY='auto'}
 for(let i=0;i<16;i++)menu.append(row('Other '+i,'City'));
 if(mode==='menu'||mode==='replace'){addTargets(menu);loaded=true}
 if(mode==='captured'||mode==='mismatch'){
  menu.innerHTML=window.captured;const actual=menu.querySelector('ul');menu.replaceChildren(...actual.children);menu.setAttribute('aria-busy','false');loaded=true;
 }
 document.getElementById('sidebar').append(menu);
 if(mode.startsWith('network-')){
  menu.setAttribute('aria-busy','true');
  fetch('/api/graphql/',{method:'POST',body:new URLSearchParams({fb_dtsg:'current-token',doc_id:'current-doc',fb_api_req_friendly_name:'MarketplaceSearchAddressDataSourceQuery',variables:JSON.stringify({params:{caller:'MARKETPLACE',query:field.value,search_type:(mode==='network-home-geocode'||mode==='network-home-context')?'STREET_PLACE_TYPEAHEAD':'PLACE_TYPEAHEAD',provider:'HERE_THRIFT',result_ordering:'ADDRESS_FIRST',viewer_coordinates:{latitude:0,longitude:0}}})})}).then(r=>r.json()).then(data=>{
   if(!menu.isConnected)return;
   const node=data.data.city_street_search.street_results.edges[0].node;
   menu.append(row(node.single_line_address,'Different place'),row(node.single_line_address,node.subtitle));loaded=true;menu.setAttribute('aria-busy','false');
  });
 }
}
document.addEventListener('scroll',event=>{if(event.target.id==='sidebar'||event.target.id==='suggestions')scrolls++;
 if((mode==='sidebar'||mode==='virtual')&&!loaded&&event.target.id==='sidebar'&&event.target.scrollTop>100){loaded=true;setTimeout(()=>{
  let menu=document.getElementById('suggestions');if(!menu)return;
  if(mode==='virtual'){const fresh=menu.cloneNode(true);menu.replaceWith(fresh);menu=fresh;replaced=true}
  addTargets(menu);
 },80)}
 if(mode==='replace'&&!replaced&&event.target.id==='suggestions'){replaced=true;const menu=document.getElementById('suggestions');menu.replaceWith(menu.cloneNode(true))}
},true);
document.addEventListener('click',event=>{
 const option=event.target.closest('#suggestions li[role=option]');if(!option)return;
 const rect=option.getBoundingClientRect();const list=document.getElementById('suggestions');const container=(mode==='menu'||mode==='replace')?list:document.getElementById('sidebar');const clip=container.getBoundingClientRect();offscreenClick=rect.top>=clip.bottom||rect.bottom<=clip.top;
 clicks++;chosen=option.textContent;const field=document.querySelector('input[role=combobox]');field.value=option.querySelector('span[dir=auto]').textContent;field.setAttribute('aria-expanded','false');document.getElementById('suggestions').remove();
});
</script>`)
		fmt.Fprintf(w, `<script>window.captured=%q;</script></body>`, string(capture))
	}))
	defer server.Close()
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	for _, mode := range []string{"menu", "sidebar", "virtual", "replace", "missing", "captured", "mismatch", "network-label", "network-count", "network-missing", "network-home-query", "network-home-retry", "network-home-geocode", "network-home-context"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			page, err := service.prepareMarketplaceTestPage(ctx, server.URL+"/"+mode)
			if err != nil {
				t.Fatal(err)
			}
			if err := page.Context(ctx).WaitLoad(); err != nil {
				t.Fatal(err)
			}
			hit := &MarketplaceLocationHit{PageID: "selected", Label: "Langley, BC, Canada", Subtitle: "19,951 people"}
			if mode == "captured" {
				hit.Subtitle = "City"
			}
			if strings.HasPrefix(mode, "network-") {
				hit.PageID = "114751268540391"
				hit.Label = "Langley, British Columbia"
				hit.Subtitle = "City"
				if mode == "network-count" {
					hit.Label = "Langley Count"
					hit.Subtitle = "Langley, BC, Canada · 15,951 people checked in here"
				}
				if strings.HasPrefix(mode, "network-home-") {
					hit.Label = "Langley, BC, Canada"
					hit.City = "Langley"
					hit.SearchType = "STREET_PLACE_TYPEAHEAD"
					if mode == "network-home-query" {
						hit.SearchQuery = "Langley"
					}
				}
				if mode == "network-home-geocode" {
					copy := geoHits[0]
					hit = &copy
					hit.SearchType = "STREET_PLACE_TYPEAHEAD"
					hit.SearchQuery = "nairobi"
				}
				if mode == "network-home-context" {
					hit.Label = "Mombasa, Kenya"
					hit.PageID = "109627399063416"
					hit.City = "Mombasa"
					hit.SearchQuery = "Mombasa, Kenya "
					hit.SearchParams = `{"caller":"MARKETPLACE","query":"Mombasa, Kenya ","search_type":"STREET_PLACE_TYPEAHEAD","provider":"HERE_THRIFT","result_ordering":"ADDRESS_FIRST","geocode_fallback":true,"viewer_coordinates":{"latitude":49.2819,"longitude":-123.1204}}`
				}
				if mode == "network-missing" {
					hit.PageID = "99999"
					err := marketplaceSelectLocation(ctx, page, hit)
					if err == nil || !strings.Contains(err.Error(), "did not return the saved location") {
						t.Fatalf("Missing place identity must not select a different result: %v", err)
					}
					result, err := page.Context(ctx).Eval(`()=>clicks`)
					if err != nil || result.Value.Int() != 0 {
						t.Fatalf("Missing page ID clicked a result: %v, %v", result, err)
					}
					return
				}
			}
			if mode == "mismatch" {
				err := marketplaceSelectLocation(ctx, page, hit)
				if err == nil || !strings.Contains(err.Error(), "do not match the saved location") || !strings.Contains(err.Error(), "15,951 people") {
					t.Fatalf("Expected useful mismatch details, got %v", err)
				}
				result, err := page.Context(ctx).Eval(`()=>({clicks,scrolls})`)
				if err != nil || result.Value.Get("clicks").Int() != 0 || result.Value.Get("scrolls").Int() != 0 {
					t.Fatalf("Completed unmatched list must neither scroll nor select: %v, %v", result, err)
				}
				return
			}
			if mode == "missing" {
				active, stop := context.WithCancel(ctx)
				timer := time.AfterFunc(400*time.Millisecond, stop)
				defer timer.Stop()
				defer stop()
				if err := marketplaceSelectLocation(active, page, hit); !errors.Is(err, context.Canceled) {
					t.Fatalf("Unmatched scrolling did not cancel: %v", err)
				}
			} else {
				selectLocation := marketplaceSelectLocation
				if strings.HasPrefix(mode, "network-home-") {
					selectLocation = marketplaceSelectHomeLocation
				}
				if err := selectLocation(ctx, page, hit); err != nil {
					t.Fatal(err)
				}
			}
			result, err := page.Context(ctx).Eval(`()=>({chosen,clicks,loaded,replaced,queries,preview:document.getElementById('preview').scrollTop,sidebar:document.getElementById('sidebar').scrollTop,scrolls,offscreenClick})`)
			if err != nil {
				t.Fatal(err)
			}
			v := result.Value
			if v.Get("preview").Int() != 0 {
				t.Fatalf("Location search scrolled the preview: %v", v)
			}
			if mode == "missing" {
				if v.Get("clicks").Int() != 0 {
					t.Fatalf("Unmatched option was clicked: %v", v)
				}
				return
			}
			want := "Langley, BC, Canada19,951 people"
			if mode == "captured" || mode == "network-label" || strings.HasPrefix(mode, "network-home-") {
				want = "Langley, BC, CanadaCity"
			}
			if mode == "network-count" {
				want = "Langley, BC, CanadaLangley, BC, Canada · 15,952 people checked in here"
			}
			if mode == "network-home-geocode" {
				want = geoHits[0].Label + geoHits[0].Subtitle
			}
			if mode == "network-home-context" {
				want = "Mombasa, KenyaCity"
			}
			if v.Get("clicks").Int() != 1 || v.Get("chosen").Str() != want || !v.Get("loaded").Bool() {
				t.Fatalf("Wrong scrolled location selected: %v", v)
			}
			if !v.Get("offscreenClick").Bool() {
				t.Fatalf("Fixture should select a clipped/offscreen DOM option: %v", v)
			}
			if (mode == "menu" || mode == "replace" || mode == "captured" || strings.HasPrefix(mode, "network-")) && (v.Get("scrolls").Int() != 0 || v.Get("replaced").Bool()) {
				t.Fatalf("An existing DOM match must be clicked without scrolling: %v", v)
			}
			if mode == "virtual" && !v.Get("replaced").Bool() {
				t.Fatalf("Fixture did not replace suggestions: %v", v)
			}
			if strings.HasPrefix(mode, "network-home-") {
				queries := v.Get("queries").Arr()
				query := "Langley"
				if mode == "network-home-geocode" {
					query = "nairobi"
				}
				if mode == "network-home-context" {
					query = "Mombasa, Kenya "
				}
				if queries[len(queries)-1].Str() != query || (mode == "network-home-query" && queries[0].Str() != "Langley") {
					t.Fatalf("Home search did not reuse/retry the original city query: %v", v)
				}
			}
			if mode == "network-home-context" {
				checkRestored := func() {
					t.Helper()
					result, err := page.Context(ctx).Eval(`async()=>{const response=await fetch('/api/graphql/',{method:'POST',body:new URLSearchParams({fb_api_req_friendly_name:'MarketplaceSearchAddressDataSourceQuery',fb_dtsg:'current-token',doc_id:'current-doc',variables:JSON.stringify({params:{caller:'MARKETPLACE',query:'Mombasa, Kenya ',search_type:'STREET_PLACE_TYPEAHEAD',provider:'HERE_THRIFT',result_ordering:'ADDRESS_FIRST',viewer_coordinates:{latitude:0,longitude:0}}})})});const data=await response.json();return data.data.city_street_search.street_results.edges[0].node.page.id}`)
					if err != nil || result.Value.Str() == hit.PageID {
						t.Fatalf("Saved context interception remained enabled after lookup: %v %v", result, err)
					}
				}
				checkRestored()
				missing := *hit
				missing.PageID = "999999"
				if err := marketplaceSelectHomeLocation(ctx, page, &missing); !errors.Is(err, errMarketplaceLocationNotReturned) {
					t.Fatalf("Unavailable place selected or wrong failure: %v", err)
				}
				checkRestored()
			}
		})
	}
}
