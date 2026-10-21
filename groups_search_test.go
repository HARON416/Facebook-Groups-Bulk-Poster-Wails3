package main

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestEmbeddedGroupSearch(t *testing.T) {
	service := &GroupsService{}
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	load := &groupsLoad{cancel: func() {}}
	groups := []Group{{ID: "one", Name: "Kahawa West market"}, {ID: "two", Name: "Kasarani residents"}}
	load.set("complete", "", groups)
	service.groupsLoad = load
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ids, err := service.SearchGroups(ctx, "kaha")
	if err != nil || !reflect.DeepEqual(searchHitIDs(ids), []string{"one"}) {
		t.Fatalf("prefix: ids=%v error=%v", ids, err)
	}
	ids, err = service.SearchGroups(ctx, "kahwa")
	if err != nil || !reflect.DeepEqual(searchHitIDs(ids), []string{"one"}) {
		t.Fatalf("typo: ids=%v error=%v", ids, err)
	}
	if len(ids) != 1 {
		t.Fatal("missing typo result")
	}
	var full, matched string
	for _, part := range ids[0].Parts {
		full += part.Text
		if part.Matched {
			matched += part.Text
		}
	}
	if full != groups[0].Name || matched == "" {
		t.Fatalf("typo highlights: %+v", ids[0])
	}
	ids, err = service.SearchGroups(ctx, "zzzzmissing")
	if err != nil || len(ids) != 0 {
		t.Fatalf("no matches: ids=%v error=%v", ids, err)
	}
	// Newly fetched groups become searchable immediately.
	groups = append(groups, Group{ID: "three", Name: "Kahawa neighbours"})
	load.set("loading", "", groups)
	ids, err = service.SearchGroups(ctx, "kaha")
	if err != nil || len(ids) != 2 {
		t.Fatalf("incremental index: ids=%v error=%v", ids, err)
	}
	// A profile refresh cannot return groups from the previous profile.
	next := &groupsLoad{cancel: func() {}}
	next.set("complete", "", []Group{{ID: "new", Name: "Ruiru community"}})
	service.groupsLoad = next
	ids, err = service.SearchGroups(ctx, "kahawa")
	if err != nil || len(ids) != 0 {
		t.Fatalf("profile isolation: ids=%v error=%v", ids, err)
	}
	// Return all matches, including result sets larger than 250 groups.
	many := []Group{}
	for i := 0; i < 300; i++ {
		many = append(many, Group{ID: fmt.Sprintf("bulk-%d", i), Name: fmt.Sprintf("Community group %d", i)})
	}
	next.set("complete", "", many)
	ids, err = service.SearchGroups(ctx, "community")
	if err != nil || len(ids) != 300 {
		t.Fatalf("pagination: count=%d error=%v", len(ids), err)
	}
	for _, id := range ids {
		if id.ID == "new" {
			t.Fatal("removed group remained searchable")
		}
	}
}

func searchHitIDs(hits []GroupSearchHit) []string {
	ids := make([]string, len(hits))
	for i, hit := range hits {
		ids[i] = hit.ID
	}
	return ids
}

func TestGroupSearchMatchingAndHighlights(t *testing.T) {
	for _, test := range []struct {
		name, query, matched string
		want                 bool
	}{
		{"Kahawa West", " KAHAWA ", "Kahawa", true},
		{"Kahawa West", "kaha", "Kahawa", true},
		{"Kahawa West", "kahwa", "Kahawa", true},
		{"Kahawa West", "kahwaa", "Kahawa", true},
		{"Kahawa West", "kahaxa", "Kahawa", true},
		{"Kahawa West", "kahawaa", "Kahawa", true},
		{"Community market", "comunty", "Community", true},
		{"Kahawa West", "west kaha", "KahawaWest", true},
		{"Kahawa-West", "kahawa we", "KahawaWest", true},
		{"Kahawa West", "kahawa ruiru", "", false},
		{"Kahawa West", "ahawa", "Kahawa", true}, // One deletion, not substring matching.
		{"Kahawa West", "hawa", "", false},
		{"Car sales", "cat", "", false}, // No fuzzy expansion of short terms.
		{"<script> & KAHĀWA 🔥", "kahāwa", "KAHĀWA", true},
		{"Москва автомобили", "моск", "Москва", true},
		{"東京 車販売", "東京", "東京", true},
		{"Kahawa kahawa", "kahawa", "Kahawakahawa", true},
		{"Kahawa West", "zzzzmissing", "", false},
		{"Kahawa 🔥 cars", "🔥", "🔥", true},
		{"Kahawa cars", "🔥", "", false},
		{"Kahawa West", "...", "", false},
	} {
		t.Run(test.name+"/"+test.query, func(t *testing.T) {
			hits, err := searchGroupNames(context.Background(), []Group{{ID: "one", Name: test.name}}, test.query)
			if err != nil || (len(hits) == 1) != test.want {
				t.Fatalf("matches: %+v, %v", hits, err)
			}
			if !test.want {
				return
			}
			var full, matched string
			for _, part := range hits[0].Parts {
				full += part.Text
				if part.Matched {
					matched += part.Text
				}
			}
			if full != test.name || matched != test.matched {
				t.Fatalf("highlight: %+v", hits[0].Parts)
			}
		})
	}
}

func TestGroupSearchRankingAndEmptyQuery(t *testing.T) {
	groups := []Group{{ID: "typo", Name: "Kahaxa"}, {ID: "prefix", Name: "Kahawani"}, {ID: "exact", Name: "Kahawa"}, {ID: "exact-two", Name: "Kahawa West"}}
	hits, err := searchGroupNames(context.Background(), groups, "kahawa")
	if err != nil || !reflect.DeepEqual(searchHitIDs(hits), []string{"exact", "exact-two", "prefix", "typo"}) {
		t.Fatalf("ranking: %+v, %v", hits, err)
	}
	hits, err = searchGroupNames(context.Background(), groups, "  ")
	if err != nil || !reflect.DeepEqual(searchHitIDs(hits), []string{"typo", "prefix", "exact", "exact-two"}) {
		t.Fatalf("empty query: %+v, %v", hits, err)
	}
	for _, hit := range hits {
		for _, part := range hit.Parts {
			if part.Matched {
				t.Fatal("empty query highlighted text")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = searchGroupNames(ctx, groups, "kahawa"); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestGroupSearchRenamedGroups(t *testing.T) {
	load := &groupsLoad{}
	service := &GroupsService{groupsLoad: load}
	load.set("complete", "", []Group{{ID: "one", Name: "Kahawa"}})
	if hits, err := service.SearchGroups(context.Background(), "kahawa"); err != nil || len(hits) != 1 {
		t.Fatal(hits, err)
	}
	load.set("complete", "", []Group{{ID: "one", Name: "Ruiru"}})
	if hits, err := service.SearchGroups(context.Background(), "kahawa"); err != nil || len(hits) != 0 {
		t.Fatal("stale name", hits, err)
	}
	if hits, err := service.SearchGroups(context.Background(), "ruiru"); err != nil || len(hits) != 1 {
		t.Fatal("new name missing", hits, err)
	}
}

func BenchmarkEmbeddedGroupSearch(b *testing.B) {
	groups := make([]Group, 5000)
	for i := range groups {
		groups[i] = Group{ID: fmt.Sprint(i), Name: fmt.Sprintf("Kahawa community car market %d", i)}
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := searchGroupNames(context.Background(), groups, "kahwa car"); err != nil {
			b.Fatal(err)
		}
	}
}
