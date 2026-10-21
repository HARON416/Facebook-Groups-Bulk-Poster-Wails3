package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPostingResponseAvailableOnlyByExplicitLookup(t *testing.T) {
	s := &GroupsService{postingRunID: 7}
	body := `for (;;);{"data":{"story_create":null},"errors":[{"code":1357006,"description":"response detail","large":7694986411746282026}]}`
	outcome := parsePostResponseForGroup([]byte(body), 200, "target")
	s.recordOutcome(outcome)
	page := s.GetPostingLogs(1, 50)
	entry := page.Entries[len(page.Entries)-1]
	if !entry.HasResponse {
		t.Fatal("response has no copy action")
	}
	encoded, _ := json.Marshal(page)
	if strings.Contains(string(encoded), "response detail") {
		t.Fatal("raw response leaked through polling")
	}
	copied, err := s.postingResponse(7, entry.ID)
	if err != nil || !strings.Contains(copied, "response detail") || !strings.Contains(copied, "7694986411746282026") || !strings.Contains(copied, "\n  ") {
		t.Fatalf("copy lost response values or formatting: %s %v", copied, err)
	}
	for _, id := range []int{0, -1, len(page.Entries) + 1} {
		if _, err := s.postingResponse(7, id); err == nil {
			t.Fatal("invalid log ID accepted")
		}
	}
	if _, err := s.postingResponse(6, entry.ID); err == nil {
		t.Fatal("old run accepted")
	}
	if _, err := s.postingResponse(7, 1); err == nil {
		t.Fatal("milestone has no response")
	}
}

func TestPostingResponseCopyPreservesInvalidAndStreamBodies(t *testing.T) {
	for _, body := range []string{"<html>Unexpected response</html>", `{"data":{}}` + "\n" + `{"extensions":{"is_final":true}}`} {
		s := &GroupsService{postingRunID: 1}
		s.recordOutcome(PostOutcome{State: "unknown", response: body})
		copied, err := s.postingResponse(1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(body, "<") && copied != body {
			t.Fatal("non-JSON response changed")
		}
		if strings.HasPrefix(body, "{") && !strings.Contains(copied, `"is_final": true`) {
			t.Fatal("stream data lost")
		}
	}
}
