package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
)

type richPostBodyKey struct{}
type richPostBody struct{ text, html string }

// Reuse Facebook's exact captured text for subsequent groups. Its structured
// text and style offsets stay untouched; the submitted draft is not compared
// against Facebook's serialized representation after publication.
func capturedRichPostText(raw string) (string, error) {
	form, err := url.ParseQuery(raw)
	var variables struct {
		Input struct {
			Message *struct {
				Text string `json:"text"`
			} `json:"message"`
		} `json:"input"`
	}
	if err != nil || json.Unmarshal([]byte(form.Get("variables")), &variables) != nil || variables.Input.Message == nil {
		return "", errors.New("Invalid captured formatted post")
	}
	return variables.Input.Message.Text, nil
}

// PublishRichPostWithOptions lets Facebook's own editor translate supported
// HTML into its native format. Replays retain that captured formatting.
func (s *GroupsService) PublishRichPostWithOptions(ctx context.Context, selected []string, body, bodyHTML string, images []PostUpload, options PostingOptions) (*PostingStatus, error) {
	if len(bodyHTML) > 1024*1024 {
		return nil, errors.New("Formatted post is too large")
	}
	// Rewriting changes the text, so the original formatting no longer applies.
	if bodyHTML != "" && !options.Rewrite {
		ctx = context.WithValue(ctx, richPostBodyKey{}, richPostBody{body, bodyHTML})
	}
	return s.PublishPostWithOptions(ctx, selected, body, images, options)
}
