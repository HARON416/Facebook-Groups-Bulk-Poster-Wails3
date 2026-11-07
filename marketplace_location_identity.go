package main

import "strings"

type marketplaceLocationRowIdentity struct {
	Index int `json:"index"`
	Count int `json:"count"`
}

// Facebook renders typeahead results in response order. Locate the saved ID or
// geocoded identity first, then retain its position within identical captions.
func marketplaceLocationRowFor(hits []MarketplaceLocationHit, selected int) *marketplaceLocationRowIdentity {
	norm := func(value string) string { return strings.ToLower(strings.Join(strings.Fields(value), " ")) }
	target := hits[selected]
	row := &marketplaceLocationRowIdentity{}
	for i, hit := range hits {
		if norm(hit.Label) == norm(target.Label) && norm(hit.Subtitle) == norm(target.Subtitle) {
			if i == selected {
				row.Index = row.Count
			}
			row.Count++
		}
	}
	return row
}
