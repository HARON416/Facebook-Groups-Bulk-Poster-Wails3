package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestMarketplaceLocationCachesAndRecovery(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		var variables struct {
			Params struct {
				Query string `json:"query"`
			} `json:"params"`
		}
		_ = json.Unmarshal([]byte(r.Form.Get("variables")), &variables)
		switch variables.Params.Query {
		case "slow":
			close(started)
			<-r.Context().Done()
			return
		case "temporary":
			_, _ = w.Write([]byte(`{"data":`))
			return
		case "unselectable":
			_, _ = w.Write([]byte(`{"data":{"city_street_search":{"street_results":{"edges":[{"node":{"single_line_address":"Address without a page","page":null}}]}}}}`))
			return
		case "expired":
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(locationFixtureResponse("Vancouver, BC, Canada")))
	}))
	defer server.Close()
	m, err := newProfileService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.ServiceShutdown() })
	s, err := m.session("default")
	if err != nil {
		t.Fatal(err)
	}
	s.marketplaceLocationLoads = make(map[bool]*marketplaceLocationLoad)
	for _, address := range []bool{false, true} {
		kind := "PLACE_TYPEAHEAD"
		if address {
			kind = "STREET_PLACE_TYPEAHEAD"
		}
		variables, _ := json.Marshal(map[string]any{"params": map[string]any{"query": "seed", "search_type": kind}})
		replay, err := newMarketplaceLocationReplay(marketplaceLocationRequest{
			URL: server.URL, Body: "variables=" + url.QueryEscape(string(variables)), User: "12345",
			Headers: http.Header{"Cookie": {"c_user=12345"}, "Content-Type": {"application/x-www-form-urlencoded"}},
		}, []*proto.NetworkCookie{{Name: "c_user", Value: "12345", Domain: "127.0.0.1", Path: "/"}})
		if err != nil {
			t.Fatal(err)
		}
		_, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		close(done)
		load := &marketplaceLocationLoad{replay: replay, cancel: cancel, done: done, status: MarketplaceLocationStatus{State: "ready", Address: address}}
		s.marketplaceLocationLoads[address] = load
		s.marketplaceLocationLoad = load
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, address := range []bool{false, true, false, true} {
		if err := m.OpenMarketplaceLocation(ctx, "default", "Vancouver", address); err != nil {
			t.Fatalf("Cached search tried opening a browser: %v", err)
		}
		if s.browser != nil {
			t.Fatal("Cached search launched Chrome")
		}
		hits, err := m.SearchMarketplaceLocations(ctx, "default", "Vancouver", address)
		if err != nil || len(hits) != 1 {
			t.Fatalf("Switching modes discarded a capture: %+v %v", hits, err)
		}
	}
	if _, err := m.SearchMarketplaceLocations(ctx, "default", "temporary", true); !errors.Is(err, errMarketplaceLocationTemporary) {
		t.Fatalf("Malformed response was treated as a stale connection: %v", err)
	}
	if hits, err := m.SearchMarketplaceLocations(ctx, "default", "unselectable", true); err != nil || len(hits) != 0 {
		t.Fatalf("Unselectable addresses forced recapture: %+v %v", hits, err)
	}
	old := make(chan error, 1)
	go func() { _, err := m.SearchMarketplaceLocations(ctx, "default", "slow", true); old <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("Slow search did not start")
	}
	if hits, err := m.SearchMarketplaceLocations(ctx, "default", "latest", true); err != nil || len(hits) != 1 {
		t.Fatalf("Latest search did not recover: %+v %v", hits, err)
	}
	select {
	case err := <-old:
		if err != nil {
			t.Fatalf("Superseded binding returned a user-visible error: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("Superseded search remained active")
	}
	status, _ := m.GetMarketplaceLocationStatus("default", true)
	if status.State != "ready" || !status.Captured {
		t.Fatalf("Transient/cancelled search discarded capture: %+v", status)
	}
	if _, err := m.SearchMarketplaceLocations(ctx, "default", "expired", true); !errors.Is(err, errMarketplaceLocationConnection) {
		t.Fatalf("Real authentication rejection was ignored: %v", err)
	}
	status, _ = m.GetMarketplaceLocationStatus("default", true)
	city, _ := m.GetMarketplaceLocationStatus("default", false)
	if status.Captured || status.State != "connection_required" || !city.Captured || city.State != "ready" {
		t.Fatalf("Expiry invalidated the wrong mode: address=%+v city=%+v", status, city)
	}
}
