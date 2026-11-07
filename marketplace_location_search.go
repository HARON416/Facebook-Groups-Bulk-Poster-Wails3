package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

const marketplaceAddressQuery = "MarketplaceSearchAddressDataSourceQuery"

var errMarketplaceLocationTemporary = errors.New("Could not search locations. Please try again.")

var errMarketplaceLocationConnection = errors.New("Location search needs a fresh Facebook connection")

type MarketplaceLocationHit struct {
	PageID       string  `json:"pageID"`
	Label        string  `json:"label"`
	Subtitle     string  `json:"subtitle"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	City         string  `json:"city"`
	PostalCode   string  `json:"postalCode"`
	SearchType   string  `json:"searchType,omitempty"`
	SearchQuery  string  `json:"searchQuery,omitempty"`
	AddressKey   string  `json:"addressKey,omitempty"`
	SearchParams string  `json:"searchParams,omitempty"`
}

// Provider street addresses need not have a Facebook page. Keep their local
// selection identity separate from pageID; never invent a Facebook page ID.
func marketplaceGeocodedAddressKey(label string, lat, lon float64) string {
	label = strings.ToLower(strings.Join(strings.Fields(label), " "))
	if label == "" || math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		return ""
	}
	canonical := label + "\x00" + strconv.FormatFloat(lat, 'f', 6, 64) + "\x00" + strconv.FormatFloat(lon, 'f', 6, 64)
	return fmt.Sprintf("address:%x", sha256.Sum256([]byte(canonical)))
}

func marketplaceSameLocation(saved *MarketplaceLocationHit, live MarketplaceLocationHit) bool {
	if saved.PageID != "" {
		return saved.PageID == live.PageID
	}
	return saved.AddressKey != "" && live.PageID == "" && saved.AddressKey == live.AddressKey
}

func parseMarketplaceLocations(body []byte, address ...bool) ([]MarketplaceLocationHit, error) {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(bytes.TrimSpace(body), []byte("for (;;);"))))
	var results []MarketplaceLocationHit
	found := false
	seen := map[string]bool{}
	for {
		var envelope struct {
			Data *struct {
				Search *struct {
					Streets *struct {
						Edges *[]struct {
							Node *struct {
								Label    string `json:"single_line_address"`
								Subtitle string `json:"subtitle"`
								Page     *struct {
									ID string `json:"id"`
								} `json:"page"`
								Location *struct {
									Latitude  *float64 `json:"latitude"`
									Longitude *float64 `json:"longitude"`
									Reverse   struct {
										City       string `json:"city"`
										PostalCode string `json:"postal_code"`
									} `json:"reverse_geocode_detailed"`
								} `json:"location"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"street_results"`
				} `json:"city_street_search"`
			} `json:"data"`
			Errors []json.RawMessage `json:"errors"`
			Error  json.RawMessage   `json:"error"`
		}
		if err := decoder.Decode(&envelope); err == io.EOF {
			break
		} else if err != nil {
			return nil, errMarketplaceLocationConnection
		}
		if len(envelope.Errors) > 0 || len(envelope.Error) > 0 && string(envelope.Error) != "null" {
			return nil, errMarketplaceLocationConnection
		}
		if envelope.Data == nil || envelope.Data.Search == nil || envelope.Data.Search.Streets == nil || envelope.Data.Search.Streets.Edges == nil {
			continue
		}
		found = true
		edges := *envelope.Data.Search.Streets.Edges
		for _, edge := range edges {
			n := edge.Node
			if n == nil || strings.TrimSpace(n.Label) == "" || n.Location == nil || n.Location.Latitude == nil || n.Location.Longitude == nil {
				continue
			}
			lat, lon := *n.Location.Latitude, *n.Location.Longitude
			if math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
				return nil, errMarketplaceLocationConnection
			}
			pageID, addressKey := "", ""
			if n.Page != nil && decimalID(n.Page.ID) {
				pageID = n.Page.ID
			} else if n.Page == nil && len(address) > 0 && address[0] {
				addressKey = marketplaceGeocodedAddressKey(n.Label, lat, lon)
			} else {
				continue
			}
			key := pageID + addressKey
			if key != "" && !seen[key] {
				results = append(results, MarketplaceLocationHit{PageID: pageID, AddressKey: addressKey, Label: n.Label, Subtitle: n.Subtitle, Latitude: lat, Longitude: lon, City: n.Location.Reverse.City, PostalCode: n.Location.Reverse.PostalCode})
				seen[key] = true
			}
		}
		if len(edges) > 0 && len(results) == 0 && !(len(address) > 0 && address[0]) {
			return nil, errMarketplaceLocationConnection
		}
	}
	if !found {
		return nil, errMarketplaceLocationConnection
	}
	if results == nil {
		results = []MarketplaceLocationHit{}
	}
	return results, nil
}

func marketplaceLocationBody(raw, query string, contextParams ...string) (string, error) {
	fields := strings.Split(raw, "&")
	found := false
	for i, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		name, err := url.QueryUnescape(key)
		if err != nil || name != "variables" {
			continue
		}
		if found || !ok {
			return "", errMarketplaceLocationConnection
		}
		found = true
		decoded, err := url.QueryUnescape(value)
		var variables, params map[string]json.RawMessage
		if err != nil || json.Unmarshal([]byte(decoded), &variables) != nil || variables == nil || json.Unmarshal(variables["params"], &params) != nil || params == nil {
			return "", errMarketplaceLocationConnection
		}
		if len(contextParams) > 0 && contextParams[0] != "" {
			params = nil
			if json.Unmarshal([]byte(contextParams[0]), &params) != nil || params == nil || string(params["caller"]) != `"MARKETPLACE"` || string(params["search_type"]) != `"STREET_PLACE_TYPEAHEAD"` {
				return "", errors.New("Invalid saved house address search context")
			}
		}
		params["query"], _ = json.Marshal(query)
		variables["params"], _ = json.Marshal(params)
		encoded, err := json.Marshal(variables)
		if err != nil {
			return "", errMarketplaceLocationConnection
		}
		fields[i] = key + "=" + url.QueryEscape(string(encoded))
	}
	if !found {
		return "", errMarketplaceLocationConnection
	}
	return strings.Join(fields, "&"), nil
}

type marketplaceLocationRequest struct {
	URL, Body, User string
	Headers         http.Header
}
type marketplaceLocationReplay struct {
	revoked      bool
	control      sync.Mutex
	activeCancel context.CancelFunc
	generation   uint64
	mu           sync.Mutex
	request      marketplaceLocationRequest
	endpoint     *url.URL
	jar          http.CookieJar
	client       *http.Client
	sequence     uint64
}

func newMarketplaceLocationReplay(request marketplaceLocationRequest, cookies []*proto.NetworkCookie) (*marketplaceLocationReplay, error) {
	endpoint, err := url.Parse(request.URL)
	if err != nil || request.User == "" || cookieUser(request.Headers.Get("Cookie")) != request.User || browserUser(cookies) != request.User {
		return nil, errMarketplaceLocationConnection
	}
	jar, _ := cookiejar.New(nil)
	var seed []*http.Cookie
	for _, cookie := range cookies {
		seed = append(seed, &http.Cookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Secure: cookie.Secure, HttpOnly: cookie.HTTPOnly})
	}
	jar.SetCookies(endpoint, seed)
	return &marketplaceLocationReplay{request: request, endpoint: endpoint, jar: jar, client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (replay *marketplaceLocationReplay) search(ctx context.Context, query string, verify ...func(context.Context) error) ([]MarketplaceLocationHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	replay.control.Lock()
	if replay.revoked {
		replay.control.Unlock()
		cancel()
		return nil, errMarketplaceLocationConnection
	}
	if replay.activeCancel != nil {
		replay.activeCancel()
	}
	replay.generation++
	generation := replay.generation
	replay.activeCancel = cancel
	replay.control.Unlock()
	defer func() {
		cancel()
		replay.control.Lock()
		if replay.generation == generation {
			replay.activeCancel = nil
		}
		replay.control.Unlock()
	}()
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(verify) > 0 {
		if err := verify[0](ctx); err != nil {
			return nil, err
		}
	}
	if cookieUser(cookieHeader(replay.jar.Cookies(replay.endpoint)).Get("Cookie")) != replay.request.User {
		return nil, errMarketplaceLocationConnection
	}
	body, err := marketplaceLocationBody(replay.request.Body, query)
	if err != nil {
		return nil, err
	}
	replay.sequence++
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, replay.request.URL, strings.NewReader(timelineRequestSequence(body, replay.sequence)))
	if err != nil {
		return nil, errMarketplaceLocationConnection
	}
	request.Header = replay.request.Headers.Clone()
	request.Header.Del("Cookie")
	for _, cookie := range replay.jar.Cookies(replay.endpoint) {
		request.AddCookie(cookie)
	}
	response, err := replay.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errMarketplaceLocationTemporary
	}
	defer response.Body.Close()
	replay.jar.SetCookies(replay.endpoint, response.Cookies())
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if response.StatusCode == 401 || response.StatusCode == 403 || response.StatusCode >= 300 && response.StatusCode < 400 || cookieUser(cookieHeader(replay.jar.Cookies(replay.endpoint)).Get("Cookie")) != replay.request.User {
		return nil, errMarketplaceLocationConnection
	}
	if response.StatusCode >= 500 || response.StatusCode == 408 || response.StatusCode == 429 {
		return nil, errMarketplaceLocationTemporary
	}
	if response.StatusCode != 200 {
		return nil, errors.New("Could not search locations. Please try again.")
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errMarketplaceLocationTemporary
	}
	if len(bodyBytes) > 8*1024*1024 {
		return nil, errors.New("Could not read location search results")
	}
	form, _ := url.ParseQuery(replay.request.Body)
	var variables struct {
		Params struct {
			SearchType string `json:"search_type"`
		} `json:"params"`
	}
	_ = json.Unmarshal([]byte(form.Get("variables")), &variables)
	var rawVariables map[string]json.RawMessage
	_ = json.Unmarshal([]byte(form.Get("variables")), &rawVariables)
	var savedParams map[string]json.RawMessage
	_ = json.Unmarshal(rawVariables["params"], &savedParams)
	if savedParams != nil {
		savedParams["query"], _ = json.Marshal(query)
	}
	searchParams, _ := json.Marshal(savedParams)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	hits, err := parseMarketplaceLocations(bodyBytes, variables.Params.SearchType == "STREET_PLACE_TYPEAHEAD")
	if err != nil {
		// A malformed/transient search payload does not prove the login expired.
		// Keep the capture; authentication failures are checked above.
		return nil, errMarketplaceLocationTemporary
	}
	for i := range hits {
		hits[i].SearchType = variables.Params.SearchType
		hits[i].SearchQuery = query
		if variables.Params.SearchType == "STREET_PLACE_TYPEAHEAD" {
			hits[i].SearchParams = string(searchParams)
		}
	}
	return hits, nil
}
func (m *ProfileService) SearchMarketplaceLocations(ctx context.Context, profileID, query string, address bool) ([]MarketplaceLocationHit, error) {
	s, err := m.session(profileID)
	if err != nil {
		return nil, err
	}
	if !address {
		query = strings.TrimSpace(query)
	}
	if strings.TrimSpace(query) == "" {
		return []MarketplaceLocationHit{}, nil
	}
	if len(query) > 1024 {
		return nil, errors.New("Enter a town or city")
	}
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	ctx, deadline := context.WithTimeout(ctx, 45*time.Second)
	defer deadline()
	s.mu.Lock()
	load := s.locationLoadForMode(address)
	browser := s.browser
	s.mu.Unlock()
	if load == nil {
		return nil, errMarketplaceLocationConnection
	}
	load.mu.Lock()
	replay := load.replay
	modeMatches := load.status.Address == address
	load.mu.Unlock()
	if replay == nil || !modeMatches {
		return nil, errMarketplaceLocationConnection
	}
	// Browser and replay can rotate cookies independently. Observe the live
	// account before replay, without requiring Chrome to remain open forever.
	verify := func(active context.Context) error {
		if browser != nil {
			probe, stop := context.WithTimeout(active, 2*time.Second)
			cookies, cookieErr := browser.Context(probe).GetCookies()
			stop()
			if active.Err() != nil {
				return active.Err()
			}
			if cookieErr == nil {
				var matching []*proto.NetworkCookie
				for _, cookie := range cookies {
					domain := strings.TrimPrefix(cookie.Domain, ".")
					if replay.endpoint.Hostname() == domain || strings.HasSuffix(replay.endpoint.Hostname(), "."+domain) {
						matching = append(matching, cookie)
					}
				}
				if browserUser(matching) != replay.request.User {
					return errMarketplaceLocationConnection
				}
			}
		}
		return nil
	}
	hits, err := replay.search(ctx, query, verify)
	if errors.Is(err, context.Canceled) {
		// A newer keystroke routinely supersedes a typeahead request. Return
		// quietly across the binding; the UI ignores stale search generations.
		return []MarketplaceLocationHit{}, nil
	}
	if errors.Is(err, errMarketplaceLocationConnection) {
		load.invalidateLocationReplay(replay)
	}
	return hits, err
}
func (load *marketplaceLocationLoad) invalidateLocationReplay(replay *marketplaceLocationReplay) {
	load.mu.Lock()
	defer load.mu.Unlock()
	if load.replay == replay {
		replay.stop()
		load.replay = nil
		load.status.State = "connection_required"
		load.status.Message = ""
	}
}

func (replay *marketplaceLocationReplay) stop() {
	replay.control.Lock()
	defer replay.control.Unlock()
	replay.revoked = true
	if replay.activeCancel != nil {
		replay.activeCancel()
	}
}
