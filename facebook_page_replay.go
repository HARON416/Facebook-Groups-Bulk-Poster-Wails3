package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// Change only the cursor in variables. Opaque values, repeated form fields,
// document IDs, and all other captured bytes outside variables stay intact.
func timelineBody(raw string, cursor *string) (string, error) {
	fields := strings.Split(raw, "&")
	found := false
	for i, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		name, err := url.QueryUnescape(key)
		if err != nil || name != "variables" {
			continue
		}
		if found || !ok {
			return "", errors.New("The captured timeline variables are ambiguous")
		}
		found = true
		decoded, err := url.QueryUnescape(value)
		var variables map[string]json.RawMessage
		if err != nil || json.Unmarshal([]byte(decoded), &variables) != nil || variables == nil {
			return "", errors.New("The captured timeline variables are invalid")
		}
		variables["cursor"], _ = json.Marshal(cursor)
		// The supplied browser requests use count=2 initially, then count=3
		// for continuation requests. Keep the opaque cursor exactly as returned.
		if cursor != nil {
			variables["count"] = json.RawMessage("3")
		}
		encoded, err := json.Marshal(variables)
		if err != nil {
			return "", err
		}
		fields[i] = key + "=" + url.QueryEscape(string(encoded))
	}
	if !found {
		return "", errors.New("The captured request has no timeline variables")
	}
	return strings.Join(fields, "&"), nil
}

const maxTimelineResponseBytes = 128 * 1024 * 1024

type timelineReadError struct{ cause error }

func (err *timelineReadError) Error() string {
	return "Facebook's timeline response was interrupted. Fetch again"
}
func (err *timelineReadError) Unwrap() error { return err.cause }

// Advance Chrome's base-36 request counter for each request and retry. Preserve
// all other opaque form fields, including their escaping and repeated values.
func timelineRequestSequence(body string, sequence uint64) string {
	fields := strings.Split(body, "&")
	for i, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		name, err := url.QueryUnescape(key)
		if err != nil || name != "__req" || !ok {
			continue
		}
		decoded, err := url.QueryUnescape(value)
		initial, parseErr := strconv.ParseUint(decoded, 36, 64)
		if err == nil && parseErr == nil {
			fields[i] = key + "=" + strconv.FormatUint(initial+sequence, 36)
		}
	}
	return strings.Join(fields, "&")
}

type timelineLimitedReader struct {
	source    io.Reader
	remaining int64
}

func (reader *timelineLimitedReader) Read(buffer []byte) (int, error) {
	if reader.remaining <= 0 {
		return 0, errors.New("timeline response size limit reached")
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	n, err := reader.source.Read(buffer)
	reader.remaining -= int64(n)
	return n, err
}

func fetchTimelinePage(ctx context.Context, client *http.Client, jar http.CookieJar, endpoint *url.URL, captured timelineRequest, body string, sequence *uint64) ([]FacebookPagePost, string, bool, error) {
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, "", false, err
		}
		(*sequence)++
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, captured.URL, strings.NewReader(timelineRequestSequence(body, *sequence)))
		if err != nil {
			return nil, "", false, errors.New("Could not prepare the timeline request")
		}
		request.Header = captured.Headers.Clone()
		// Photo capture and earlier requests may have rotated session cookies.
		request.Header.Del("Cookie")
		for _, cookie := range jar.Cookies(endpoint) {
			request.AddCookie(cookie)
		}
		response, requestErr := client.Do(request)
		var readErr error
		if requestErr == nil {
			jar.SetCookies(endpoint, response.Cookies())
			if cookieUser(cookieHeader(jar.Cookies(endpoint)).Get("Cookie")) != captured.User {
				response.Body.Close()
				return nil, "", false, errors.New("The Facebook session changed while fetching posts. Fetch again")
			}
			if response.StatusCode == http.StatusOK {
				limited := &timelineLimitedReader{source: response.Body, remaining: maxTimelineResponseBytes}
				batch, next, more, err := readFacebookPageResponse(limited)
				response.Body.Close()
				if err == nil {
					return batch, next, more, nil
				}
				if limited.remaining == 0 {
					return nil, "", false, errors.New("Facebook's timeline response exceeded 128 MB")
				}
				var interrupted *timelineReadError
				if !errors.As(err, &interrupted) {
					return nil, "", false, err
				}
				var syntax *json.SyntaxError
				if errors.As(err, &syntax) {
					return nil, "", false, err
				}
				readErr = err
			} else {
				response.Body.Close()
				if response.StatusCode != http.StatusTooManyRequests && response.StatusCode < 500 {
					return nil, "", false, fmt.Errorf("Facebook rejected the timeline request (HTTP %d). Check your login and fetch again", response.StatusCode)
				}
				readErr = fmt.Errorf("Facebook's timeline request failed (HTTP %d)", response.StatusCode)
			}
		} else {
			readErr = errors.New("Could not fetch Facebook posts. Check your connection and fetch again")
		}
		if attempt == 2 {
			return nil, "", false, readErr
		}
		select {
		case <-ctx.Done():
			return nil, "", false, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	return nil, "", false, errors.New("Could not fetch the next timeline page")
}

func replayFacebookTimeline(ctx context.Context, captured timelineRequest, cookies []*proto.NetworkCookie, target int, progress func([]FacebookPagePost)) ([]FacebookPagePost, error) {
	endpoint, err := url.Parse(captured.URL)
	if err != nil {
		return nil, errors.New("Could not prepare the timeline endpoint")
	}
	jar, _ := cookiejar.New(nil)
	seed := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		seed = append(seed, &http.Cookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Secure: cookie.Secure, HttpOnly: cookie.HTTPOnly})
	}
	jar.SetCookies(endpoint, seed)
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	posts := []FacebookPagePost{}
	ids := map[string]bool{}
	cursors := map[string]bool{}
	var cursor *string
	var sequence uint64
	photoURLs := map[string]string{}
	for {
		if err := ctx.Err(); err != nil {
			return posts, err
		}
		if captured.User == "" || cookieUser(captured.Headers.Get("Cookie")) != captured.User || cookieUser(cookieHeader(jar.Cookies(endpoint)).Get("Cookie")) != captured.User {
			return nil, errors.New("The Facebook session changed. Fetch again with the current account")
		}
		body, err := timelineBody(captured.Body, cursor)
		if err != nil {
			return posts, err
		}
		batch, next, more, err := fetchTimelinePage(ctx, client, jar, endpoint, captured, body, &sequence)
		if err != nil {
			return posts, err
		}
		for _, post := range batch {
			if !ids[post.ID] {
				post, err = resolveFacebookPostImages(ctx, client, jar, endpoint, captured, post, photoURLs)
				if err != nil {
					return posts, err
				}
				ids[post.ID] = true
				posts = append(posts, post)
				if len(posts) == target {
					break
				}
			}
		}
		if progress != nil {
			progress(posts)
		}
		if len(posts) >= target || !more {
			return posts, nil
		}
		if next == "" || cursors[next] {
			return posts, errors.New("Facebook returned an invalid or repeated timeline cursor. Fetch again")
		}
		cursors[next] = true
		cursor = &next
		select {
		case <-ctx.Done():
			return posts, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
