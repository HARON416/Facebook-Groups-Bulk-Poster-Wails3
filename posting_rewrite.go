package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func rewriteGemini(ctx context.Context, client *http.Client, endpoint string, key, original, instructions string) (string, error) {
	prompt := "Rewrite the following advertisement. Use the original as the only source of facts. Do not invent or change specifications, prices, locations, terms, or contact details. Copy every phone number exactly, preserving its Unicode characters, leading zeros, punctuation and spacing. These accuracy rules override conflicting custom instructions. Return only the rewritten advertisement, without explanations.\n\nStyle instructions:\n" + instructions + "\n\nOriginal advertisement:\n" + original
	payload := map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": prompt}}}}}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("Could not prepare the Gemini request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-goog-api-key", key)
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("Could not reach Gemini. Check your connection")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini rewrite failed (HTTP %d). Check the key, model and account quota", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil || len(data) > 1024*1024 {
		return "", errors.New("Could not read the Gemini rewrite")
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Candidates) == 0 {
		return "", errors.New("Gemini returned no rewritten text")
	}
	if result.Candidates[0].FinishReason != "" && result.Candidates[0].FinishReason != "STOP" {
		return "", errors.New("Gemini did not finish the rewrite")
	}
	var text strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		if !part.Thought {
			text.WriteString(part.Text)
		}
	}
	rewritten := strings.TrimSpace(text.String())
	if rewritten == "" {
		return "", errors.New("Gemini returned no rewritten text")
	}
	return rewritten, nil
}
func (s *GroupsService) rewriteForRun(ctx context.Context, original string, options PostingOptions, settings *storedSettings) (string, error) {
	return s.rewriteForPost(ctx, original, options, settings, http.DefaultClient, "https://generativelanguage.googleapis.com/v1beta/models/"+url.PathEscape(settings.GeminiModel)+":generateContent")
}

func (s *GroupsService) rewriteForPost(ctx context.Context, original string, options PostingOptions, settings *storedSettings, client *http.Client, endpoint string) (string, error) {
	if err := postingCheckpoint(ctx); err != nil {
		return "", err
	}
	if settings.APIKey == "" {
		return original, nil
	}
	s.setPostStatus("Rewriting the post body with Gemini…")
	rewritten, err := rewriteGemini(ctx, client, endpoint, settings.APIKey, original, options.Instructions)
	for err != nil && ctx.Err() == nil {
		offline, waitErr := recoverPostingConnection(ctx)
		if waitErr != nil {
			return "", waitErr
		}
		if !offline {
			break
		}
		rewritten, err = rewriteGemini(ctx, client, endpoint, settings.APIKey, original, options.Instructions)
	}
	if err == nil {
		s.setPostStatus("Gemini rewrite completed.")
		return rewritten, nil
	}
	if ctx.Err() != nil {
		return "", errors.New("Posting stopped.")
	}
	s.postMu.Lock()
	s.postStatus = "Gemini rewrite failed. Using the original text for this post; Gemini will be tried again for the next post."
	s.appendPostingLogLocked("warning", err.Error()+" Using the original text for this post only. Gemini remains enabled for the next post.")
	s.postMu.Unlock()
	if err := postingCheckpoint(ctx); err != nil {
		return "", err
	}
	return original, nil
}
