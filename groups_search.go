package main

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

// GroupSearchPart is plain text, never HTML supplied by a group name.
type GroupSearchPart struct {
	Text    string `json:"text"`
	Matched bool   `json:"matched"`
}

type GroupSearchHit struct {
	ID    string            `json:"id"`
	Parts []GroupSearchPart `json:"parts"`
}

// SearchGroups searches a snapshot of the current groups entirely in Go.
// No server, on-disk index, network connection or separate executable is needed.
func (s *GroupsService) SearchGroups(ctx context.Context, query string) ([]GroupSearchHit, error) {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	s.mu.Lock()
	s.restoreGroupsLocked()
	load, closed := s.groupsLoad, s.closed
	s.mu.Unlock()
	if closed || load == nil {
		return []GroupSearchHit{}, nil
	}
	load.mu.Lock()
	groups := append([]Group{}, load.status.Groups...)
	load.mu.Unlock()
	hits, err := searchGroupNames(ctx, groups, query)
	s.mu.Lock()
	current := s.groupsLoad == load && !s.closed
	s.mu.Unlock()
	if !current {
		return []GroupSearchHit{}, nil
	}
	return hits, err
}

type groupSearchToken struct {
	start, end int // Rune offsets into the original name, for safe highlighting.
	text       []rune
}

func groupSearchTokens(text []rune) []groupSearchToken {
	var tokens []groupSearchToken
	start := -1
	for i := 0; i <= len(text); i++ {
		if i < len(text) && (unicode.IsLetter(text[i]) || unicode.IsNumber(text[i]) || start >= 0 && unicode.IsMark(text[i])) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			tokens = append(tokens, groupSearchToken{start: start, end: i, text: []rune(strings.ToLower(string(text[start:i])))})
			start = -1
		}
	}
	return tokens
}

func searchGroupNames(ctx context.Context, groups []Group, query string) ([]GroupSearchHit, error) {
	terms := groupSearchTokens([]rune(query))
	literal := []rune(strings.ToLower(strings.TrimSpace(query)))
	type rankedHit struct {
		hit   GroupSearchHit
		score int
	}
	ranked := make([]rankedHit, 0, len(groups))
	for _, group := range groups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := []rune(group.Name)
		tokens := groupSearchTokens(name)
		matched := make([]bool, len(name))
		score, found := 0, true
		// Symbol-only queries (such as an emoji) are literal searches, not an
		// empty query that would accidentally return every group.
		if len(terms) == 0 && len(literal) > 0 {
			found = false
			for i := 0; i+len(literal) <= len(name); i++ {
				if strings.ToLower(string(name[i:i+len(literal)])) == string(literal) {
					found = true
					for j := i; j < i+len(literal); j++ {
						matched[j] = true
					}
				}
			}
		}
		for i, term := range terms {
			best := -1
			for _, token := range tokens {
				cost := groupTokenMatch(term.text, token.text, i == len(terms)-1)
				if cost < 0 {
					continue
				}
				if best < 0 || cost < best {
					best = cost
				}
			}
			if best < 0 {
				found = false
				break
			}
			score += best
			// Highlight every equally good occurrence, retaining original case,
			// punctuation and Unicode characters in the returned text segments.
			for _, token := range tokens {
				if groupTokenMatch(term.text, token.text, i == len(terms)-1) == best {
					for j := token.start; j < token.end; j++ {
						matched[j] = true
					}
				}
			}
		}
		if found {
			ranked = append(ranked, rankedHit{hit: GroupSearchHit{ID: group.ID, Parts: groupSearchParts(name, matched)}, score: score})
		}
	}
	// Prefer exact words, then prefixes, then typos. Equal scores retain the
	// loaded group order, so repeated searches do not shuffle the results.
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score < ranked[j].score })
	hits := make([]GroupSearchHit, len(ranked))
	for i, result := range ranked {
		hits[i] = result.hit
	}
	return hits, nil
}

func groupSearchParts(name []rune, matched []bool) []GroupSearchPart {
	if len(name) == 0 {
		return []GroupSearchPart{{Text: ""}}
	}
	var parts []GroupSearchPart
	start := 0
	for i := 1; i <= len(name); i++ {
		if i == len(name) || matched[i] != matched[start] {
			parts = append(parts, GroupSearchPart{Text: string(name[start:i]), Matched: matched[start]})
			start = i
		}
	}
	return parts
}

func groupTokenMatch(query, word []rune, prefix bool) int {
	q, w := string(query), string(word)
	if q == w {
		return 0
	}
	if prefix && strings.HasPrefix(w, q) {
		return 1
	}
	// Avoid broad fuzzy matches for very short queries. Longer terms allow
	// one typo from four characters and two typos from seven characters.
	limit := 0
	if len(query) >= 7 {
		limit = 2
	} else if len(query) >= 4 {
		limit = 1
	}
	// Apply typo tolerance to complete words. Fuzzy short prefixes would make
	// "kaha" match unrelated names such as "Kasarani".
	if limit == 0 || len(word) < len(query)-limit || len(word) > len(query)+limit {
		return -1
	}
	length := len(word)
	// Optimal string alignment distance counts an adjacent transposition as
	// one typo, alongside insertions, deletions and substitutions.
	previous := make([]int, length+1)
	older := make([]int, length+1)
	current := make([]int, length+1)
	for j := range previous {
		previous[j] = j
	}
	for i, letter := range query {
		current[0] = i + 1
		for j := 1; j <= length; j++ {
			cost := 0
			if letter != word[j-1] {
				cost = 1
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
			if i > 0 && j > 1 && letter == word[j-2] && query[i-1] == word[j-1] {
				current[j] = min(current[j], older[j-2]+1)
			}
		}
		older, previous, current = previous, current, older
	}
	distance := previous[length]
	if distance > limit {
		return -1
	}
	return distance*4 + 1
}
