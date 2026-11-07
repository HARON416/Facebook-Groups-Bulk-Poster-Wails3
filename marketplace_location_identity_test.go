package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
)

func TestMarketplaceHomeLocationIdentityBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	service := &GroupsService{profileDir: t.TempDir(), chromePath: chrome, headless: true}
	t.Cleanup(func() { _ = service.shutdown(true) })
	for _, mode := range []string{"coordinates", "page-id", "missing-id"} {
		t.Run(mode, func(t *testing.T) {
			label, subtitle := "Langley Byp, Langley, BC V3A, Canada", "Langley Byp, Langley, BC V3A, Canada"
			if mode != "coordinates" {
				label, subtitle = "Langley, BC, Canada", "City"
			}
			edges := []any{}
			for i, lat := range []float64{49.11341, 49.11252} {
				var page any
				if mode != "coordinates" {
					page = map[string]string{"id": []string{"109472262413206", "208446059219004"}[i]}
				}
				edges = append(edges, map[string]any{"node": map[string]any{
					"single_line_address": label, "subtitle": subtitle, "page": page,
					"location": map[string]any{"latitude": lat, "longitude": -122.67403},
				}})
			}
			body, err := json.Marshal(map[string]any{"data": map[string]any{"city_street_search": map[string]any{"street_results": map[string]any{"edges": edges}}}})
			if err != nil {
				t.Fatal(err)
			}
			hits, err := parseMarketplaceLocations(body, true)
			if err != nil || len(hits) != 2 {
				t.Fatalf("Invalid fixture: %v", err)
			}
			hit := hits[1]
			hit.SearchType = "STREET_PLACE_TYPEAHEAD"
			if mode == "missing-id" {
				hit.PageID = "999999"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/graphql/" {
					time.Sleep(250 * time.Millisecond)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(body)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `<body><main><label><input role="combobox" aria-autocomplete="list" aria-controls="results" aria-expanded="false" oninput="search(this)"></label></main><script>
window.chosen=null;window.clicks=0;
function row(label,subtitle,index){const li=document.createElement('li');li.role='option';li.dataset.index=index;li.innerHTML='<div role="option"><span dir="auto"></span><span></span></div>';li.querySelectorAll('span')[0].textContent=label;li.querySelectorAll('span')[1].textContent=subtitle;return li}
function search(input){
 document.querySelector('#results')?.remove();input.setAttribute('aria-expanded','true');const menu=document.createElement('ul');menu.role='listbox';menu.id='results';menu.setAttribute('aria-busy','false');document.querySelector('main').append(menu);
 // A stale row has exactly the saved caption. It must never be clicked.
 menu.append(row(%q,%q,'stale'));
 fetch('/api/graphql/',{method:'POST',body:new URLSearchParams({fb_api_req_friendly_name:'MarketplaceSearchAddressDataSourceQuery',variables:JSON.stringify({params:{caller:'MARKETPLACE',query:input.value,search_type:'STREET_PLACE_TYPEAHEAD'}})})}).then(r=>r.json()).then(data=>{
  const nodes=data.data.city_street_search.street_results.edges.map(e=>e.node);menu.replaceChildren(row(nodes[0].single_line_address,nodes[0].subtitle,0));
  // The response arrives before React finishes rendering the duplicate rows.
  setTimeout(()=>menu.append(row(nodes[1].single_line_address,nodes[1].subtitle,1)),600);
 });
}
document.addEventListener('click',event=>{const option=event.target.closest('li[role="option"]');if(!option)return;window.clicks++;window.chosen=option.dataset.index;const field=document.querySelector('input');field.value=option.querySelector('span').textContent;field.setAttribute('aria-expanded','false');document.querySelector('#results').remove()});
</script></body>`, label, subtitle)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			page, err := service.prepareMarketplaceTestPage(ctx, server.URL)
			if err != nil {
				t.Fatal(err)
			}
			if err := page.Context(ctx).WaitLoad(); err != nil {
				t.Fatal(err)
			}
			err = marketplaceSelectLocationQuery(ctx, page, &hit, "langley")
			if mode == "missing-id" {
				if !errors.Is(err, errMarketplaceLocationNotReturned) {
					t.Fatalf("Expected missing identity, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			result, err := page.Context(ctx).Eval(`()=>({chosen:window.chosen,clicks:window.clicks})`)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "missing-id" {
				if result.Value.Get("clicks").Int() != 0 {
					t.Fatalf("Clicked stale result despite missing identity: %v", result.Value)
				}
			} else if result.Value.Get("chosen").Str() != "1" || result.Value.Get("clicks").Int() != 1 {
				t.Fatalf("Did not select the exact second identity once: %v", result.Value)
			}
		})
	}
}
