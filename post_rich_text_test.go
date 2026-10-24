package main

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
)

func TestCapturedRichPostText(t *testing.T) {
	text, err := capturedRichPostText(postFixtureForm())
	if err != nil || text != "First\nSecond" {
		t.Fatalf("captured serialization: %q %v", text, err)
	}
	for _, raw := range []string{"variables={}", "variables=invalid", "variables=%zz", `variables={"input":{"message":null}}`} {
		if _, err := capturedRichPostText(raw); err == nil {
			t.Fatalf("invalid captured message accepted: %s", raw)
		}
	}
}

func TestCapturedRichPostReplaysFacebookSerializationUnchanged(t *testing.T) {
	blocks := []string{"LIPA MDOGO MDOGO", "DEPOSIT + FIRST WEEK: 23,399", "CALL OR WHATSAPP 0712000000"}
	// The wire message may differ from the draft or be empty while composed_text
	// holds the content. Neither case should stop a confirmed posting run.
	for _, wireText := range []string{"serialized representation", "", "🔥 LIPA MDOGO MDOGO\n📞 CALL OR WHATSAPP 0712000000"} {
		raw := richRequestFixture(wireText, blocks)
		body, err := capturedRichPostText(raw)
		if err != nil || body != wireText {
			t.Fatalf("Facebook serialization rejected: %q %v", body, err)
		}
		replay, err := buildReplayPost(raw, body, "next-group", []string{"new-photo"})
		if err != nil {
			t.Fatal(err)
		}
		before, _ := url.ParseQuery(raw)
		var expected, actual map[string]any
		json.Unmarshal([]byte(before.Get("variables")), &expected)
		json.Unmarshal([]byte(replay.Get("variables")), &actual)
		for _, field := range []string{"message", "composed_text"} {
			if !reflect.DeepEqual(record(expected["input"])[field], record(actual["input"])[field]) {
				t.Fatalf("replay rebuilt %s", field)
			}
		}
		if stringValue(record(record(actual["input"])["audience"])["to_id"]) != "next-group" {
			t.Fatal("replay did not target the next group")
		}
	}
}

func richRequestFixture(text string, blocks []string) string {
	form, _ := url.ParseQuery(postFixtureForm())
	var variables map[string]any
	json.Unmarshal([]byte(form.Get("variables")), &variables)
	input := record(variables["input"])
	input["message"] = map[string]any{"text": text, "ranges": []any{map[string]any{"offset": 0, "length": 4, "type": "bold"}}}
	input["composed_text"] = map[string]any{"blocks": blocks, "block_types": []string{"unstyled"}, "inline_styles": []string{`[{"offset":0,"length":4,"style":"BOLD"}]`}}
	encoded, _ := json.Marshal(variables)
	form.Set("variables", string(encoded))
	return form.Encode()
}
