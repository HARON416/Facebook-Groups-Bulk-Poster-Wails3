package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func locationFixtureResponse(label string) string {
	edges := []any{}
	if label != "" {
		edges = append(edges, map[string]any{"node": map[string]any{"single_line_address": label, "subtitle": "City", "page": map[string]string{"id": "114751268540391"}, "location": map[string]any{"latitude": -1.28333, "longitude": 36.8167, "reverse_geocode_detailed": map[string]string{"city": "Nairobi", "postal_code": ""}}}})
	}
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"city_street_search": map[string]any{"street_results": map[string]any{"edges": edges}}}})
	return string(body)
}
func TestMarketplaceLocationResponseAndBody(t *testing.T) {
	geoBody, err := os.ReadFile("testdata/marketplace/home-geocoded-addresses.json")
	if err != nil {
		t.Fatal(err)
	}
	geo, err := parseMarketplaceLocations(geoBody, true)
	if err != nil || len(geo) != 3 {
		t.Fatalf("Geocoded addresses disappeared: %+v %v", geo, err)
	}
	keys := map[string]bool{}
	for _, hit := range geo {
		if hit.PageID != "" || hit.AddressKey == "" || keys[hit.AddressKey] {
			t.Fatalf("Invalid geocoded identity: %+v", hit)
		}
		keys[hit.AddressKey] = true
	}
	if _, err := parseMarketplaceLocations(geoBody); !errors.Is(err, errMarketplaceLocationConnection) {
		t.Fatalf("City search accepted page-less addresses: %v", err)
	}
	geoAgain, _ := parseMarketplaceLocations(geoBody, true)
	if !marketplaceSameLocation(&geo[0], geoAgain[0]) || marketplaceSameLocation(&geo[0], geoAgain[1]) || marketplaceSameLocation(&MarketplaceLocationHit{}, geo[0]) {
		t.Fatal("Geocoded identity is unstable or accepts empty IDs")
	}
	home := validMarketplaceHome()
	home.Address = geo[0].Label
	home.Location = &geo[0]
	home.Location.SearchType = "STREET_PLACE_TYPEAHEAD"
	if err := validateMarketplaceHomeFields(&home); err != nil {
		t.Fatalf("Valid provider address rejected: %v", err)
	}
	home.Location.Longitude++
	if err := validateMarketplaceHomeFields(&home); err == nil {
		t.Fatal("Changed coordinates retained an unrelated address key")
	}
	addressBody, err := os.ReadFile("testdata/marketplace/home-address-response.json")
	if err != nil {
		t.Fatal(err)
	}
	addresses, err := parseMarketplaceLocations(addressBody)
	if err != nil || len(addresses) != 3 || addresses[0].PageID != "10883078204" || addresses[0].PostalCode != "V6Z" || addresses[1].Label != addresses[2].Label || addresses[1].PageID == addresses[2].PageID {
		t.Fatalf("House address response lost identity/details: %+v, %v", addresses, err)
	}
	for _, label := range []string{"", "Nairobi, Kenya"} {
		hits, err := parseMarketplaceLocations([]byte("for (;;);" + locationFixtureResponse(label) + "\n" + `{"extensions":{"is_final":true}}`))
		if err != nil {
			t.Fatal(err)
		}
		if label == "" && len(hits) != 0 {
			t.Fatal("empty results changed")
		}
		if label != "" && (len(hits) != 1 || hits[0].Label != label || hits[0].PageID != "114751268540391" || hits[0].Latitude != -1.28333 || hits[0].City != "Nairobi") {
			t.Fatalf("wrong location: %+v", hits)
		}
	}
	for _, body := range []string{`{"data":null}`, `{"errors":[{"message":"private fixture token"}]}`, `<html>login</html>`} {
		if _, err := parseMarketplaceLocations([]byte(body)); !errors.Is(err, errMarketplaceLocationConnection) || strings.Contains(err.Error(), "private") {
			t.Fatalf("invalid response error: %v", err)
		}
	}
	raw := "opaque=a%2fb&repeated=one&variables=" + url.QueryEscape(`{"params":{"caller":"MARKETPLACE","query":"N","unknown":18446744073709551615,"country_filter":null},"preserve":true}`) + "&repeated=two&fb_dtsg=fresh%3atoken"
	body, err := marketplaceLocationBody(raw, "N'Djamena & Nairobi")
	if err != nil || !strings.HasPrefix(body, "opaque=a%2fb&repeated=one&variables=") || !strings.HasSuffix(body, "&repeated=two&fb_dtsg=fresh%3atoken") {
		t.Fatal("request bytes were not preserved")
	}
	form, _ := url.ParseQuery(body)
	var decoded struct {
		Params struct {
			Query string `json:"query"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(form.Get("variables")), &decoded); err != nil || decoded.Params.Query != "N'Djamena & Nairobi" || !strings.Contains(form.Get("variables"), "18446744073709551615") {
		t.Fatal("query or opaque numbers changed")
	}
	if _, err := marketplaceLocationBody(raw+"&variables=%7B%7D", "X"); err == nil {
		t.Fatal("ambiguous variables accepted")
	}
	savedParams := `{"caller":"MARKETPLACE","search_type":"STREET_PLACE_TYPEAHEAD","provider":"HERE_THRIFT","result_ordering":"ADDRESS_FIRST","viewer_coordinates":{"latitude":49.2819,"longitude":-123.1204}}`
	contextBody, err := marketplaceLocationBody(raw, "Mombasa, Kenya ", savedParams)
	if err != nil {
		t.Fatal(err)
	}
	contextForm, _ := url.ParseQuery(contextBody)
	var restored struct {
		Params map[string]json.RawMessage `json:"params"`
	}
	_ = json.Unmarshal([]byte(contextForm.Get("variables")), &restored)
	if string(restored.Params["query"]) != `"Mombasa, Kenya "` || restored.Params["country_filter"] != nil || contextForm.Get("fb_dtsg") != "fresh:token" || len(contextForm["repeated"]) != 2 {
		t.Fatalf("Saved params changed current authentication, merged live params, or trimmed query: %s", contextBody)
	}
	if _, err := marketplaceLocationBody(raw, "Mombasa", `{"search_type":"PLACE_TYPEAHEAD"}`); err == nil {
		t.Fatal("City search params accepted as a saved house context")
	}
}

func TestMarketplaceLocationCaptureReplayIntegration(t *testing.T) {
	testMarketplaceLocationCaptureReplay(t, false, false)
}

func TestMarketplaceLocationSlowCaptureIntegration(t *testing.T) {
	testMarketplaceLocationCaptureReplay(t, true, false)
}

func TestMarketplaceAddressCaptureReplayIntegration(t *testing.T) {
	testMarketplaceLocationCaptureReplay(t, false, true)
}

func testMarketplaceLocationCaptureReplay(t *testing.T, slow, address bool) {
	t.Helper()
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome required")
	}
	targetPath := "/marketplace/create/item"
	searchType := "PLACE_TYPEAHEAD"
	if address {
		targetPath = "/marketplace/create/rental"
		searchType = "STREET_PLACE_TYPEAHEAD"
	}
	var visits, replays, verifications atomic.Int64
	var reject atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(204)
			return
		}
		if r.URL.Path == "/api/graphql/" {
			if err := r.ParseForm(); err != nil {
				t.Error(err)
				return
			}
			var variables struct {
				Params struct {
					Query             string             `json:"query"`
					SearchType        string             `json:"search_type"`
					ResultOrdering    string             `json:"result_ordering"`
					Provider          string             `json:"provider"`
					GeocodeFallback   bool               `json:"geocode_fallback"`
					ViewerCoordinates map[string]float64 `json:"viewer_coordinates"`
				} `json:"params"`
			}
			_ = json.Unmarshal([]byte(r.Form.Get("variables")), &variables)
			if variables.Params.SearchType != searchType {
				t.Error("search type changed")
			}
			if address && (variables.Params.ResultOrdering != "ADDRESS_FIRST" || variables.Params.Provider != "HERE_THRIFT" || !variables.Params.GeocodeFallback || variables.Params.ViewerCoordinates["latitude"] != 49.2819) {
				t.Error("home search configuration lost")
			}
			// Facebook may reject/abort the empty focus request while the typed
			// query succeeds. Only the typed request is a capture candidate.
			if variables.Params.Query == "" {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if slow && visits.Load() == 1 && variables.Params.Query == "N" {
				delay := time.Duration(0)
				if r.Form.Get("__req") == "a" {
					delay = 31 * time.Second
				} else if verifications.Add(1) == 1 {
					delay = 16 * time.Second
				}
				if delay > 0 {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(delay):
					}
				}
				if r.Form.Get("__req") != "a" && verifications.Load() == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
			}

			if variables.Params.Query == "Nairobi" || variables.Params.Query == "Nakuru" {
				replays.Add(1)
				cookie, _ := r.Cookie("c_user")
				xs, _ := r.Cookie("xs")
				if cookie == nil || cookie.Value != "12345" || xs == nil || r.Header.Get("X-Fixture") != "captured" || r.Form.Get("doc_id") != "fresh-1" || r.Form.Get("fb_dtsg") != "fresh-fixture-token" || len(r.Form["repeat"]) != 2 || len(r.Form.Get("opaque")) != 90000 {
					t.Error("replay lost live request values")
				}
				if variables.Params.Query == "Nakuru" && xs.Value != "rotated" {
					t.Error("replay ignored rotated cookies")
				}
				http.SetCookie(w, &http.Cookie{Name: "xs", Value: "rotated", Path: "/", HttpOnly: true})
			}
			if reject.Load() {
				w.WriteHeader(403)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			label := ""
			if variables.Params.Query != "" {
				label = "Nairobi, Kenya"
			}
			_, _ = w.Write([]byte(locationFixtureResponse(label)))
			return
		}
		visit := visits.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "c_user", Value: "12345", Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: "xs", Value: "fresh", Path: "/", HttpOnly: true})
		details, hidden := marketplaceDetailsFixture, "hidden"
		modeParams := "search_type:'PLACE_TYPEAHEAD',country_filter:null,integration_strategy:'STRING_MATCH',page_category:['CITY','SUBCITY','NEIGHBORHOOD','POSTAL_CODE']"
		if address {
			details = ""
			hidden = ""
			modeParams = "search_type:'STREET_PLACE_TYPEAHEAD',geocode_fallback:true,integration_strategy:'STRING_MATCH',provider:'HERE_THRIFT',result_ordering:'ADDRESS_FIRST',viewer_coordinates:{latitude:49.2819,longitude:-123.1204}"
		}
		inputEvent := "oninput"
		if address {
			inputEvent = "onchange"
		}
		fmt.Fprintf(w, `<body class="UIPage_LoggedIn"><aside role="complementary">%s<input id="fixture-location" type="text" role="combobox" aria-autocomplete="list" autocomplete="off" aria-label="Location" %s onfocus="send('')" %s="send(this.value)"></aside><script>function send(query){const f=new URLSearchParams({__user:'12345',av:'12345',doc_id:%q,fb_dtsg:'fresh-fixture-token',__req:'a',opaque:'x'.repeat(90000),fb_api_req_friendly_name:%q,variables:JSON.stringify({params:{caller:'MARKETPLACE',query,%s}})});f.append('repeat','one');f.append('repeat','two');fetch('/api/graphql/',{method:'POST',headers:{'content-type':'application/x-www-form-urlencoded','x-fixture':'captured','x-fb-friendly-name':%q},body:f})}</script></body>`, details, hidden, inputEvent, fmt.Sprintf("fresh-%d", visit), marketplaceAddressQuery, modeParams, marketplaceAddressQuery)
	}))
	defer server.Close()
	m, err := newProfileService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.ServiceShutdown(); time.Sleep(200 * time.Millisecond) })
	s, err := m.session("default")
	if err != nil {
		t.Fatal(err)
	}
	s.headless = true
	s.chromePath = chrome
	ctx, cancel := context.WithTimeout(context.Background(), 85*time.Second)
	defer cancel()
	if err := s.openMarketplaceLocation(ctx, server.URL+targetPath, "N"); err != nil {
		t.Fatal(err)
	}
	wait := 12 * time.Second
	if slow {
		wait = 70 * time.Second
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		status, _ := m.GetMarketplaceLocationStatus("default", address)
		if status.State == "ready" && status.Captured {
			break
		}
		if status.State == "error" || status.State == "connection_required" {
			t.Fatalf("capture failed: %+v", status)
		}
		time.Sleep(25 * time.Millisecond)
	}
	status, _ := m.GetMarketplaceLocationStatus("default", address)
	if !status.Captured {
		t.Fatalf("request not captured: %+v", status)
	}
	s.marketplaceLocationLoad.mu.Lock()
	capturedBody := s.marketplaceLocationLoad.replay.request.Body
	s.marketplaceLocationLoad.mu.Unlock()
	capturedForm, _ := url.ParseQuery(capturedBody)
	if !strings.Contains(capturedForm.Get("variables"), `"query":"N"`) {
		t.Fatal("captured the empty focus request instead of the typed query")
	}
	if slow && verifications.Load() < 2 {
		t.Fatal("temporary verification failure was not retried")
	}
	if s.browser != nil || s.marketplaceLocationPage != nil {
		t.Fatal("dedicated capture browser was left open")
	}
	if _, err := m.SearchMarketplaceLocations(ctx, "default", "Wrong mode", !address); !errors.Is(err, errMarketplaceLocationConnection) {
		t.Fatal("cross-mode capture reused")
	}
	for _, query := range []string{"Nairobi", "Nakuru"} {
		hits, err := m.SearchMarketplaceLocations(ctx, "default", query, address)
		if err != nil || len(hits) != 1 || hits[0].City != "Nairobi" || hits[0].SearchType != searchType || hits[0].SearchQuery != query {
			t.Fatalf("search %s: %+v %v", query, hits, err)
		}
		if address {
			var params struct {
				Query             string             `json:"query"`
				ViewerCoordinates map[string]float64 `json:"viewer_coordinates"`
			}
			if json.Unmarshal([]byte(hits[0].SearchParams), &params) != nil || params.Query != query || params.ViewerCoordinates["latitude"] != 49.2819 || strings.Contains(hits[0].SearchParams, "fresh-fixture-token") {
				t.Fatal("House result did not retain its search context independently of authentication")
			}
		}
	}
	if replays.Load() != 2 || visits.Load() != 1 {
		t.Fatalf("Unexpected query requests or form reloads: requests=%d visits=%d", replays.Load(), visits.Load())
	}
	other, err := m.CreateProfile("Other account")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SearchMarketplaceLocations(ctx, other.ID, "Nairobi", address); !errors.Is(err, errMarketplaceLocationConnection) {
		t.Fatal("another account reused the captured session")
	}
	reject.Store(true)
	if _, err := m.SearchMarketplaceLocations(ctx, "default", "Expired", address); !errors.Is(err, errMarketplaceLocationConnection) {
		t.Fatalf("expired session accepted: %v", err)
	}
	status, _ = m.GetMarketplaceLocationStatus("default", address)
	if status.State != "connection_required" || status.Captured {
		t.Fatal("expired capture was retained")
	}

	reject.Store(false)
	if err := s.ensureBrowserPage(ctx); err != nil {
		t.Fatal(err)
	}
	mainPage := s.page
	if err := s.openMarketplaceLocation(ctx, server.URL+targetPath, "N"); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		status, _ = m.GetMarketplaceLocationStatus("default", address)
		if status.State == "ready" && status.Captured {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if s.browser == nil || s.page != mainPage || s.marketplaceLocationPage != nil {
		t.Fatal("capture cleanup did not preserve the shared browser tab")
	}
	if _, err := m.SearchMarketplaceLocations(ctx, "default", "Reconnected", address); err != nil {
		t.Fatalf("fresh capture did not recover search: %v", err)
	}
	if err := (proto.NetworkSetCookies{Cookies: []*proto.NetworkCookieParam{{Name: "c_user", Value: "99999", URL: server.URL, Path: "/", HTTPOnly: true}}}).Call(mainPage.Context(ctx)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SearchMarketplaceLocations(ctx, "default", "Wrong account", address); !errors.Is(err, errMarketplaceLocationConnection) {
		t.Fatal("changed browser account reused the old template")
	}
}

func TestMarketplaceSearchCancelsOlderQuery(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if strings.Contains(r.Form.Get("variables"), "slow") {
			close(started)
			select {
			case <-r.Context().Done():
				close(cancelled)
			case <-time.After(5 * time.Second):
			}
			return
		}
		_, _ = w.Write([]byte(locationFixtureResponse("Nairobi, Kenya")))
	}))
	defer server.Close()
	raw := "variables=" + url.QueryEscape(`{"params":{"query":"","caller":"MARKETPLACE"}}`)
	replay, err := newMarketplaceLocationReplay(marketplaceLocationRequest{URL: server.URL, Body: raw, User: "12345", Headers: http.Header{"Cookie": {"c_user=12345"}, "Content-Type": {"application/x-www-form-urlencoded"}}}, []*proto.NetworkCookie{{Name: "c_user", Value: "12345", Domain: "127.0.0.1", Path: "/"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	old := make(chan error, 1)
	go func() { _, err := replay.search(ctx, "slow"); old <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("old query did not start")
	}
	hits, err := replay.search(ctx, "Nairobi")
	if err != nil || len(hits) != 1 {
		t.Fatalf("latest search failed: %+v %v", hits, err)
	}
	select {
	case err := <-old:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("old query not cancelled: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("old query remained active")
	}
	select {
	case <-cancelled:
	case <-ctx.Done():
		t.Fatal("old HTTP request was not cancelled")
	}
}
