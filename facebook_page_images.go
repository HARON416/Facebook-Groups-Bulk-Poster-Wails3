package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type facebookPhotoImage struct {
	uri      string
	area     float64
	original bool
}
type facebookPagePhoto struct {
	id       string
	mediaset string
	image    facebookPhotoImage
}

func photoImage(node map[string]any) facebookPhotoImage {
	best := facebookPhotoImage{}
	for _, key := range []string{"original_image", "full_size_image", "photo_image", "image", "images"} {
		candidates := []any{node[key]}
		if list, ok := node[key].([]any); ok {
			candidates = list
		}
		for _, value := range candidates {
			image, ok := value.(map[string]any)
			if !ok {
				continue
			}
			uri, _ := image["uri"].(string)
			if uri == "" {
				uri, _ = image["source"].(string)
			}
			parsed, err := url.Parse(uri)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
				continue
			}
			width, _ := image["width"].(float64)
			height, _ := image["height"].(float64)
			candidate := facebookPhotoImage{uri: uri, area: width * height, original: key == "original_image" || key == "full_size_image"}
			if best.uri == "" || candidate.original && !best.original || candidate.original == best.original && candidate.area > best.area {
				best = candidate
			}
		}
	}
	return best
}

// Preserve attachment-array order and select one variant per photo, rather
// than treating each resized version (or a video poster/avatar) as a photo.
func collectFacebookPhotos(story any) []facebookPagePhoto {
	photos := []facebookPagePhoto{}
	seen := map[string]int{}
	var walk func(any, bool, string)
	walk = func(value any, attachment bool, mediaset string) {
		switch node := value.(type) {
		case []any:
			for _, child := range node {
				walk(child, attachment, mediaset)
			}
		case map[string]any:
			if raw, ok := node["url"].(string); ok {
				if token := facebookPhotoMediaset(raw); token != "" {
					mediaset = token
				}
			}
			kind, _ := node["__typename"].(string)
			if attachment && (kind == "Video" || kind == "VideoAttachment") {
				return
			}
			if attachment && (kind == "" || kind == "Photo") {
				image := photoImage(node)
				if image.uri != "" {
					id, _ := node["id"].(string)
					if !decimalID(id) {
						id = ""
					}
					if id == "" {
						if raw, ok := node["url"].(string); ok {
							id = facebookPhotoID(raw)
						}
					}
					key := id
					if key == "" {
						key = image.uri
					}
					if index, ok := seen[key]; ok {
						old := photos[index].image
						if image.original && !old.original || image.original == old.original && image.area > old.area {
							photos[index].image = image
						}
						if photos[index].mediaset == "" {
							photos[index].mediaset = mediaset
						}
					} else {
						seen[key] = len(photos)
						photos = append(photos, facebookPagePhoto{id: id, mediaset: mediaset, image: image})
					}
				}
			}
			keys := make([]string, 0, len(node))
			for key := range node {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				switch key {
				case "feedback", "actors", "author", "owner", "profile_picture", "image", "photo_image", "original_image", "full_size_image", "images", "thumbnail", "preferred_thumbnail":
					continue
				}
				walk(node[key], attachment || key == "attachments", mediaset)
			}
		}
	}
	walk(story, false, "")
	return photos
}

func facebookPhotoID(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return ""
	}
	switch u.Hostname() {
	case "facebook.com", "web.facebook.com", "www.facebook.com", "m.facebook.com":
	default:
		return ""
	}
	if decimalID(u.Query().Get("fbid")) {
		return u.Query().Get("fbid")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for _, part := range parts {
		if part == "photos" && decimalID(parts[len(parts)-1]) {
			return parts[len(parts)-1]
		}
	}
	return ""
}

// Keep the collection supplied in Facebook's photo URL when available.
func facebookPhotoMediaset(raw string) string {
	if facebookPhotoID(raw) == "" {
		return ""
	}
	u, _ := url.Parse(raw)
	return u.Query().Get("set")
}

// Read Relay envelopes through the final marker, merging deferred photo fields.
func readFacebookPhotoResponse(source io.Reader, photoID string) (facebookPhotoImage, error) {
	reader := bufio.NewReader(source)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return facebookPhotoImage{}, err
		}
		if !strings.ContainsRune(" \r\n\t", rune(b)) {
			_ = reader.UnreadByte()
			break
		}
	}
	if prefix, _ := reader.Peek(9); string(prefix) == "for (;;);" {
		_, _ = reader.Discard(9)
	}
	decoder := json.NewDecoder(reader)
	var merged any
	for {
		var envelope struct {
			Data       json.RawMessage   `json:"data"`
			Path       []json.RawMessage `json:"path"`
			Errors     []json.RawMessage `json:"errors"`
			Error      json.RawMessage   `json:"error"`
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
			return facebookPhotoImage{}, err
		}
		if len(envelope.Errors) > 0 || len(envelope.Error) > 0 && string(envelope.Error) != "null" && string(envelope.Error) != "0" {
			return facebookPhotoImage{}, errors.New("Facebook rejected the photo query. Check your login and fetch again")
		}
		if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			var data any
			if err := json.Unmarshal(envelope.Data, &data); err != nil {
				return facebookPhotoImage{}, err
			}
			merged, err = mergeTimelinePatch(merged, envelope.Path, data)
			if err != nil {
				return facebookPhotoImage{}, err
			}
		}
		if envelope.Extensions.Final || envelope.HasNext != nil && !*envelope.HasNext {
			break
		}
	}
	data, _ := merged.(map[string]any)
	media, _ := data["currMedia"].(map[string]any)
	id, _ := media["id"].(string)
	kind, _ := media["__typename"].(string)
	if id == photoID && (kind == "Photo" || kind == "") {
		if image := photoImage(media); image.uri != "" {
			return image, nil
		}
	}
	return facebookPhotoImage{}, errors.New("Facebook did not return the requested viewer photo. Fetch again")
}

// Preserve every opaque field outside variables, including repeated fields.
func facebookPhotoBody(raw, photoID, mediaset string) (string, error) {
	fields := strings.Split(raw, "&")
	found := false
	for i, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		name, err := url.QueryUnescape(key)
		if err != nil || name != "variables" {
			continue
		}
		if !ok || found {
			return "", errors.New("The captured photo variables are ambiguous")
		}
		found = true
		decoded, err := url.QueryUnescape(value)
		var variables map[string]json.RawMessage
		if err != nil || json.Unmarshal([]byte(decoded), &variables) != nil || variables == nil {
			return "", errors.New("The captured photo variables are invalid")
		}
		variables["nodeID"], _ = json.Marshal(photoID)
		variables["mediasetToken"], _ = json.Marshal(mediaset)
		// A focused comment from the initially opened photo belongs to that photo.
		if _, ok := variables["focusCommentID"]; ok {
			variables["focusCommentID"] = json.RawMessage("null")
		}
		encoded, _ := json.Marshal(variables)
		fields[i] = key + "=" + url.QueryEscape(string(encoded))
	}
	if !found {
		return "", errors.New("The captured request has no photo variables")
	}
	return strings.Join(fields, "&"), nil
}

func resolveFacebookPostImages(ctx context.Context, client *http.Client, jar http.CookieJar, endpoint *url.URL, captured timelineRequest, post FacebookPagePost, cache map[string]string) (FacebookPagePost, error) {
	resolved := make([]string, 0, len(post.photos))
	for index, photo := range post.photos {
		if err := ctx.Err(); err != nil {
			return post, err
		}
		if captured.PhotoQuery == nil && photo.image.original {
			resolved = append(resolved, photo.image.uri)
			continue
		}
		if photo.id == "" {
			return post, fmt.Errorf("Could not resolve full-size image %d for a fetched post. Facebook did not include its photo ID", index+1)
		}
		if cached := cache[photo.id]; cached != "" {
			resolved = append(resolved, cached)
			continue
		}
		if captured.PhotoQuery == nil {
			return post, errors.New("No photo-viewer query was captured. Fetch again to capture full-size photos")
		}
		mediaset := photo.mediaset
		if mediaset == "" && decimalID(post.ID) {
			mediaset = "pcb." + post.ID
		}
		if mediaset == "" {
			return post, errors.New("Facebook did not include the photo collection for a fetched post")
		}
		full, err := replayFacebookPhoto(ctx, client, jar, captured.PhotoQuery, photo.id, mediaset)
		if err != nil {
			return post, err
		}
		cache[photo.id] = full.uri
		resolved = append(resolved, full.uri)
	}
	post.Images = resolved
	return post, nil
}

func replayFacebookPhoto(ctx context.Context, client *http.Client, jar http.CookieJar, captured *timelineRequest, photoID, mediaset string) (facebookPhotoImage, error) {
	endpoint, err := url.Parse(captured.URL)
	if err != nil {
		return facebookPhotoImage{}, errors.New("Could not prepare the photo endpoint")
	}
	body, err := facebookPhotoBody(captured.Body, photoID, mediaset)
	if err != nil {
		return facebookPhotoImage{}, err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return facebookPhotoImage{}, err
		}
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return facebookPhotoImage{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
		captured.Sequence++
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, captured.URL, strings.NewReader(timelineRequestSequence(body, captured.Sequence)))
		if err != nil {
			return facebookPhotoImage{}, errors.New("Could not prepare the photo query")
		}
		request.Header = captured.Headers.Clone()
		request.Header.Del("Cookie")
		for _, cookie := range jar.Cookies(endpoint) {
			request.AddCookie(cookie)
		}
		if captured.User == "" || cookieUser(request.Header.Get("Cookie")) != captured.User {
			return facebookPhotoImage{}, errors.New("The Facebook session changed while fetching photos. Fetch again")
		}
		response, err := client.Do(request)
		if err != nil {
			lastErr = errors.New("Could not fetch the Facebook photo. Check your connection and fetch again")
			continue
		}
		jar.SetCookies(endpoint, response.Cookies())
		if cookieUser(cookieHeader(jar.Cookies(endpoint)).Get("Cookie")) != captured.User {
			response.Body.Close()
			return facebookPhotoImage{}, errors.New("The Facebook session changed while fetching photos. Fetch again")
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			lastErr = fmt.Errorf("Facebook rejected the photo query (HTTP %d). Fetch again", response.StatusCode)
			if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
				continue
			}
			return facebookPhotoImage{}, lastErr
		}
		limited := &timelineLimitedReader{source: response.Body, remaining: 32 * 1024 * 1024}
		image, readErr := readFacebookPhotoResponse(limited, photoID)
		response.Body.Close()
		if readErr == nil {
			return image, nil
		}
		if errors.Is(readErr, io.ErrUnexpectedEOF) || errors.Is(readErr, io.EOF) {
			lastErr = errors.New("Facebook's photo response was interrupted. Fetch again")
			continue
		}
		return facebookPhotoImage{}, errors.New("Could not read the requested full-size Facebook photo. Check your login and fetch again")
	}
	return facebookPhotoImage{}, lastErr
}
