package main

import "testing"

func TestCapturedRichPostText(t *testing.T) {
	text, err := capturedRichPostText(postFixtureForm(), "First\n\nSecond")
	if err != nil || text != "First\nSecond" {
		t.Fatalf("paragraph serialization: %q %v", text, err)
	}
	if _, err := capturedRichPostText(postFixtureForm(), "First\nOther text"); err == nil {
		t.Fatal("different draft accepted")
	}
	if _, err := capturedRichPostText("variables={}", ""); err == nil {
		t.Fatal("missing message accepted")
	}
}
