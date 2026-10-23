package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
)

// FacebookPagePost contains display data only. Raw stories remain in Go for
// subsequent processing and never expose request headers or session values.
type FacebookPagePost struct {
	ID     string          `json:"id"`
	Text   string          `json:"text"`
	URL    string          `json:"url"`
	Images []string        `json:"images"`
	Story  json.RawMessage `json:"-"`
	photos []facebookPagePhoto
}

func facebookPageURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	invalid := errors.New("Enter a Facebook page URL, such as https://web.facebook.com/swiftlineMotors")
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", invalid
	}
	switch strings.ToLower(u.Hostname()) {
	case "facebook.com", "web.facebook.com", "www.facebook.com", "m.facebook.com", "mbasic.facebook.com":
	default:
		return "", invalid
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	valid := false
	if len(parts) == 1 && parts[0] != "" {
		valid = true
		for _, c := range parts[0] {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.') {
				valid = false
			}
		}
		for _, reserved := range strings.Fields("groups events watch reel reels stories marketplace login share photo photos photo.php permalink.php story.php posts videos gaming search help settings pages checkpoint") {
			if strings.EqualFold(parts[0], reserved) {
				valid = false
			}
		}
		if parts[0] == "profile.php" {
			valid = decimalID(u.Query().Get("id"))
		}
	}
	if len(parts) == 3 && parts[0] == "pages" && parts[1] != "" {
		valid = decimalID(parts[2])
	}
	if !valid {
		return "", invalid
	}
	u.Scheme, u.Host, u.Fragment = "https", "web.facebook.com", ""
	u.Path, u.RawPath = "/"+strings.Join(parts, "/"), ""
	if u.Path == "/profile.php" {
		u.RawQuery = url.Values{"id": {u.Query().Get("id")}}.Encode()
	} else {
		u.RawQuery = ""
	}
	return u.String(), nil
}

func decimalID(value string) bool {
	if value == "" {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Only the timeline connection is accepted: nested comment/reaction connections
// must never supply the pagination cursor. Relay can send multiple envelopes.
func parseFacebookPageResponse(body []byte) ([]FacebookPagePost, string, bool, error) {
	return readFacebookPageResponse(bytes.NewReader(body))
}

// Relay responses are streams of JSON envelopes. The final envelope completes
// the page even when the server keeps the HTTP stream open for deferred work.
func readFacebookPageResponse(source io.Reader) ([]FacebookPagePost, string, bool, error) {
	reader := bufio.NewReader(source)
	for {
		value, err := reader.ReadByte()
		if err != nil {
			return nil, "", false, &timelineReadError{cause: err}
		}
		if value != ' ' && value != '\n' && value != '\r' && value != '\t' {
			_ = reader.UnreadByte()
			break
		}
	}
	guard := "for (;;);"
	if prefix, _ := reader.Peek(len(guard)); string(prefix) == guard {
		_, _ = reader.Discard(len(guard))
	}
	decoder := json.NewDecoder(reader)
	var edges []json.RawMessage
	var info *struct {
		End  string `json:"end_cursor"`
		More *bool  `json:"has_next_page"`
	}
	found := false
	var inspect func(json.RawMessage) error
	inspect = func(raw json.RawMessage) error {
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil || object == nil {
			return nil
		}
		for key, value := range object {
			if key == "timeline_list_feed_units" || key == "timeline_feed_units" {
				var connection struct {
					Edges []json.RawMessage `json:"edges"`
					Info  json.RawMessage   `json:"page_info"`
				}
				if json.Unmarshal(value, &connection) != nil {
					return errors.New("Facebook returned an unreadable timeline")
				}
				found = true
				edges = append(edges, connection.Edges...)
				if len(connection.Info) > 0 && string(connection.Info) != "null" {
					if json.Unmarshal(connection.Info, &info) != nil {
						return errors.New("Facebook returned invalid timeline pagination")
					}
				}
			} else if key == "node" || key == "data" {
				if err := inspect(value); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var merged any
	for {
		var envelope struct {
			Data       json.RawMessage   `json:"data"`
			Errors     []json.RawMessage `json:"errors"`
			Error      json.RawMessage   `json:"error"`
			Path       []json.RawMessage `json:"path"`
			HasNext    *bool             `json:"hasNext"`
			Extensions struct {
				Final bool `json:"is_final"`
			} `json:"extensions"`
		}
		err := decoder.Decode(&envelope)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", false, &timelineReadError{cause: err}
		}
		if len(envelope.Errors) > 0 || len(envelope.Error) > 0 && string(envelope.Error) != "null" && string(envelope.Error) != "0" {
			return nil, "", false, errors.New("Facebook rejected the timeline query. Check your login and fetch again")
		}
		if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			var data any
			patchDecoder := json.NewDecoder(strings.NewReader(string(envelope.Data)))
			patchDecoder.UseNumber()
			if patchDecoder.Decode(&data) != nil {
				return nil, "", false, errors.New("Facebook returned invalid timeline data")
			}
			merged, err = mergeTimelinePatch(merged, envelope.Path, data)
			if err != nil {
				return nil, "", false, err
			}
		}
		if envelope.Extensions.Final || envelope.HasNext != nil && !*envelope.HasNext {
			break
		}

	}
	combined, err := json.Marshal(merged)
	if err != nil {
		return nil, "", false, errors.New("Could not read the complete timeline")
	}
	if err := inspect(combined); err != nil {
		return nil, "", false, err
	}

	if !found || info == nil || info.More == nil {
		return nil, "", false, errors.New("Facebook's timeline response format was not recognized")
	}
	posts := []FacebookPagePost{}
	for _, edge := range edges {
		var entry struct {
			Node json.RawMessage `json:"node"`
		}
		if json.Unmarshal(edge, &entry) != nil {
			return nil, "", false, errors.New("Facebook returned an unreadable timeline entry")
		}
		var node map[string]any
		if json.Unmarshal(entry.Node, &node) != nil {
			continue
		}
		kind, _ := node["__typename"].(string)
		if kind != "Story" {
			continue
		}
		post := FacebookPagePost{Images: []string{}, Story: entry.Node}
		post.ID = findStoryString(node, "post_id")
		if post.ID == "" {
			post.ID, _ = node["id"].(string)
		}
		if post.ID == "" {
			return nil, "", false, errors.New("Facebook returned a story without an identifier")
		}
		post.URL = findStoryString(node, "wwwURL")
		if post.URL == "" {
			post.URL = findStoryString(node, "url")
		}
		if message, ok := node["message"].(map[string]any); ok {
			post.Text, _ = message["text"].(string)
		}
		if post.Text == "" {
			post.Text = findMessage(node)
		}
		post.photos = collectFacebookPhotos(node)
		for _, photo := range post.photos {
			post.Images = append(post.Images, photo.image.uri)
		}
		posts = append(posts, post)
	}
	return posts, info.End, *info.More, nil
}

func findStoryString(node map[string]any, key string) string {
	if value, ok := node[key].(string); ok {
		return value
	}
	// Story content is nested here in Comet. Avoid following actor/feedback URLs.
	if sections, ok := node["comet_sections"].(map[string]any); ok {
		for _, name := range []string{"content", "context_layout"} {
			if section, ok := sections[name].(map[string]any); ok {
				if story, ok := section["story"].(map[string]any); ok {
					if value := findStoryString(story, key); value != "" {
						return value
					}
				}
			}
		}
	}
	return ""
}

func findMessage(value any) string {
	switch node := value.(type) {
	case map[string]any:
		if message, ok := node["message"].(map[string]any); ok {
			if text, ok := message["text"].(string); ok {
				return text
			}
		}
		for _, key := range []string{"comet_sections", "content", "story"} {
			if text := findMessage(node[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

// Deferred story fields and streamed edges are merged before reading posts, so
// a later message/image payload enriches its original story rather than being
// counted as another post.
func mergeTimelinePatch(current any, path []json.RawMessage, data any) (any, error) {
	if len(path) > 128 {
		return nil, errors.New("Facebook returned an invalid timeline patch")
	}
	if len(path) == 0 {
		next, ok := data.(map[string]any)
		if !ok {
			return data, nil
		}
		object, _ := current.(map[string]any)
		if object == nil {
			object = map[string]any{}
		}
		for key, value := range next {
			merged, err := mergeTimelinePatch(object[key], nil, value)
			if err != nil {
				return nil, err
			}
			object[key] = merged
		}
		return object, nil
	}
	var key string
	if json.Unmarshal(path[0], &key) == nil {
		object, _ := current.(map[string]any)
		if object == nil {
			object = map[string]any{}
		}
		value, err := mergeTimelinePatch(object[key], path[1:], data)
		if err != nil {
			return nil, err
		}
		object[key] = value
		return object, nil
	}
	var index int
	if json.Unmarshal(path[0], &index) != nil || index < 0 || index > 100000 {
		return nil, errors.New("Facebook returned an invalid timeline patch index")
	}
	list, _ := current.([]any)
	if index >= len(list) {
		list = append(list, make([]any, index-len(list)+1)...)
	}
	value, err := mergeTimelinePatch(list[index], path[1:], data)
	if err != nil {
		return nil, err
	}
	list[index] = value
	return list, nil
}
