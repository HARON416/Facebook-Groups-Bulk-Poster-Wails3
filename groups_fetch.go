package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// This is the operation name, not a session value or a document ID.
const joinedGroupsQuery = "GroupsCometAllJoinedGroupsSectionPaginationQuery"

// Group contains only display data, never browser credentials.
type Group struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Picture string `json:"picture"`
}

type GroupsLoadStatus struct {
	State   string  `json:"state"`
	Message string  `json:"message"`
	Groups  []Group `json:"groups"`
}

type groupsLoad struct {
	mu     sync.Mutex
	user   string
	status GroupsLoadStatus
	cancel context.CancelFunc
	done   chan struct{}
}

func (load *groupsLoad) set(state, message string, groups []Group) {
	load.mu.Lock()
	defer load.mu.Unlock()
	load.status = GroupsLoadStatus{State: state, Message: message, Groups: append([]Group{}, groups...)}
}

func (s *GroupsService) GetGroupsLoadStatus() GroupsLoadStatus {
	s.mu.Lock()
	s.restoreGroupsLocked()
	load := s.groupsLoad
	s.mu.Unlock()
	if load == nil {
		return GroupsLoadStatus{State: "idle", Groups: []Group{}}
	}
	load.mu.Lock()
	defer load.mu.Unlock()
	return GroupsLoadStatus{State: load.status.State, Message: load.status.Message, Groups: append([]Group{}, load.status.Groups...)}
}

// Called with s.mu held, immediately before clicking the unselected sort item.
// The listener is installed synchronously so even a fast request is captured.
func (s *GroupsService) startGroupsCapture() {
	s.stopGroupsCapture()
	ctx, cancel := s.operationContext(context.Background())
	load := &groupsLoad{cancel: cancel, done: make(chan struct{})}
	load.set("waiting", "Waiting for the groups request…", nil)
	s.groupsLoad = load
	page := s.page.Context(ctx)
	target := joinedGroupsURL
	if s.targetURL != "" {
		target = s.targetURL
	}
	origin, _ := url.Parse(target)

	type captured struct {
		endpoint string
		body     string
		headers  http.Header
		user     string
	}
	requests := make(chan captured, 1)
	failures := make(chan struct{}, 1)
	var candidate *captured
	var requestID proto.NetworkRequestID
	var finished bool
	var browserStatus int
	extras := map[proto.NetworkRequestID]proto.NetworkHeaders{}
	fail := func(message string) bool {
		load.set("error", message, nil)
		failures <- struct{}{}
		return true
	}
	complete := func() bool {
		if candidate == nil || !finished || extras[requestID] == nil {
			return false
		}
		candidate.headers = mergeReplayHeaders(candidate.headers, extras[requestID])
		if candidate.headers.Get("Cookie") == "" || cookieUser(candidate.headers.Get("Cookie")) != candidate.user {
			return fail("The captured wire cookies do not match the groups request. Click Load groups to recapture the active session.")
		}
		load.mu.Lock()
		load.user = candidate.user
		load.mu.Unlock()
		response, err := (proto.NetworkGetResponseBody{RequestID: requestID}).Call(page.Context(ctx))
		if err != nil {
			return fail("Could not verify the browser's groups response. Click Load groups to try again.")
		}
		body := []byte(response.Body)
		if response.Base64Encoded {
			body, err = base64.StdEncoding.DecodeString(response.Body)
		}
		if err != nil || len(body) > 16*1024*1024 {
			return fail("Could not read the browser's groups response.")
		}
		if browserStatus != http.StatusOK {
			return fail(fmt.Sprintf("The browser's groups request failed (HTTP %d). Check Facebook in Chrome and try again.", browserStatus))
		}
		if _, _, _, err := parseGroupsResponse(body); err != nil {
			return fail("The browser's own groups request failed: " + err.Error())
		}
		requests <- *candidate
		return true
	}
	wait := page.EachEvent(func(event *proto.NetworkRequestWillBeSent) bool {
		if candidate != nil {
			return false
		}
		request := event.Request
		if request == nil || request.Method != http.MethodPost {
			return false
		}
		endpoint, err := url.Parse(request.URL)
		if err != nil || endpoint.Scheme != origin.Scheme || endpoint.Host != origin.Host || endpoint.Path != "/api/graphql/" {
			return false
		}
		body := request.PostData
		// CDP can omit or truncate postData in the basic event. Read the complete
		// body explicitly before accepting the capture.
		data, bodyErr := (proto.NetworkGetRequestPostData{RequestID: event.RequestID}).Call(page.Context(ctx))
		if bodyErr == nil {
			body = data.PostData
		}
		form, err := url.ParseQuery(body)
		if err != nil || form.Get("fb_api_req_friendly_name") != joinedGroupsQuery {
			return false
		}
		if bodyErr != nil {
			return fail("Could not capture the complete groups POST body. Click Load groups to try again.")
		}
		var variables map[string]json.RawMessage
		if json.Unmarshal([]byte(form.Get("variables")), &variables) != nil || variables == nil {
			return false
		}
		if cursor, ok := variables["cursor"]; ok && string(cursor) != "null" && string(cursor) != `""` {
			return false
		}
		if form.Get("doc_id") == "" {
			return false
		}
		user := form.Get("__user")
		if user == "" || (form.Get("av") != "" && form.Get("av") != user) {
			return fail("The groups request does not match the active Facebook profile. Click Load groups to try again.")
		}
		requestID = event.RequestID
		candidate = &captured{endpoint: request.URL, body: body, headers: mergeReplayHeaders(nil, request.Headers), user: user}
		return complete()
	}, func(event *proto.NetworkRequestWillBeSentExtraInfo) bool {
		// ExtraInfo may arrive before or after the basic event. Correlate by ID;
		// never combine headers or cookies from two different GraphQL requests.
		if candidate != nil && event.RequestID != requestID {
			return false
		}
		if len(extras) >= 256 {
			for id := range extras {
				delete(extras, id)
				break
			}
		}
		extras[event.RequestID] = event.Headers
		return complete()
	}, func(event *proto.NetworkResponseReceived) {
		if candidate != nil && event.RequestID == requestID && event.Response != nil {
			browserStatus = event.Response.Status
		}
	}, func(event *proto.NetworkLoadingFinished) bool {
		if candidate != nil && event.RequestID == requestID {
			finished = true
			return complete()
		}
		return false
	}, func(event *proto.NetworkLoadingFailed) bool {
		if candidate != nil && event.RequestID == requestID {
			return fail("The browser could not complete the groups request. Check Facebook in Chrome and try again.")
		}
		return false
	})
	go func() {
		defer close(load.done)
		listenerDone := make(chan struct{})
		go func() { defer close(listenerDone); wait() }()
		defer func() { cancel(); <-listenerDone }()
		select {
		case request := <-requests:
			load.set("waiting", "Groups request captured. Waiting for the page to finish loading…", nil)
			settledPage := page.Context(ctx)
			err := settledPage.WaitLoad()
			if err == nil {
				// Sorting updates the existing page asynchronously. A completed
				// GraphQL response alone does not mean its UI has finished rendering.
				err = settledPage.WaitDOMStable(time.Second, 0)
			}
			if err != nil {
				load.set("error", "The groups page did not finish loading and settle. Check Chrome and click Load groups to try again.", nil)
				return
			}
			snapshot, err := (proto.NetworkGetCookies{Urls: []string{request.endpoint}}).Call(page.Context(ctx))
			if err != nil || browserUser(snapshot.Cookies) != request.user {
				load.set("error", "Could not preserve the captured Facebook session. Click Load groups to try again.", nil)
				return
			}
			s.mu.Lock()
			if s.groupsLoad != load || ctx.Err() != nil {
				s.mu.Unlock()
				return
			}
			s.closeBrowserConnection()
			s.mu.Unlock()
			load.set("loading", "Browser closed. Loading your groups…", nil)
			groups, err := fetchJoinedGroups(ctx, snapshot.Cookies, request.endpoint, request.body, request.headers, request.user, func(groups []Group) { load.set("loading", fmt.Sprintf("Loaded %d groups…", len(groups)), groups) })
			if err != nil {
				load.set("error", err.Error(), groups)
			} else {
				s.completeGroupsLoad(ctx, load, request.user, groups)
			}
		case <-failures:
			// A mismatched profile is already reported by the listener.
		case <-ctx.Done():
			load.set("error", "Group loading was stopped. Click Load groups to try again.", nil)
		}
	}()
}

// Called with s.mu held. Old jobs cannot publish into a newer load object.
func (s *GroupsService) stopGroupsCapture() {
	s.groupsCacheLoaded = true
	if s.groupsLoad != nil {
		if s.groupsLoad.cancel != nil {
			s.groupsLoad.cancel()
		}
		s.groupsLoad = nil
	}
}

func browserUser(cookies []*proto.NetworkCookie) string {
	for _, cookie := range cookies {
		if cookie.Name == "c_user" {
			return cookie.Value
		}
	}
	return ""
}

func fetchJoinedGroups(ctx context.Context, cookies []*proto.NetworkCookie, endpoint string, rawBody string, headers http.Header, user string, progress func([]Group)) ([]Group, error) {
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.New("Could not prepare the groups endpoint")
	}
	jar, _ := cookiejar.New(nil)
	seed := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		seed = append(seed, &http.Cookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Secure: cookie.Secure, HttpOnly: cookie.HTTPOnly})
	}
	jar.SetCookies(endpointURL, seed)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	var groups = []Group{}
	seenIDs := map[string]bool{}
	seenCursors := map[string]bool{}
	body := rawBody // First replay preserves the original bytes and cookie header.
	firstPage := true
	for {
		if err := ctx.Err(); err != nil {
			return groups, errors.New("Group loading was stopped. Click Load groups to try again")
		}

		if cookieUser(cookieHeader(jar.Cookies(endpointURL)).Get("Cookie")) != user {
			return nil, errors.New("The captured Facebook session changed during loading. Click Load groups to try again")
		}

		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
		if err != nil {
			return groups, errors.New("Could not prepare the groups request")
		}
		request.Header = headers.Clone()
		if !firstPage {
			// Subsequent pages use the captured session plus server cookie updates.
			request.Header.Del("Cookie")
			for _, cookie := range jar.Cookies(endpointURL) {
				request.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
			}
		}
		response, err := client.Do(request)
		if err != nil {
			return groups, errors.New("Could not fetch groups. Check your connection and click Load groups to try again")
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024+1))
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			if firstPage {
				return groups, fmt.Errorf("The browser's groups request succeeded, but Go replay returned HTTP %d. Click Load groups to try again", response.StatusCode)
			}
			return groups, fmt.Errorf("Facebook rejected the groups request (HTTP %d). Check your login and try Load groups again", response.StatusCode)
		}
		if readErr != nil || len(data) > 16*1024*1024 {
			return groups, errors.New("Could not read the groups response")
		}
		batch, next, more, err := parseGroupsResponse(data)
		if err != nil {
			if firstPage {
				return groups, fmt.Errorf("The browser's groups request succeeded, but the Go replay failed: %w", err)
			}
			return groups, err
		}
		firstPage = false

		jar.SetCookies(endpointURL, response.Cookies())
		if cookieUser(cookieHeader(jar.Cookies(endpointURL)).Get("Cookie")) != user {
			return nil, errors.New("The Facebook session changed during loading. Click Load groups to fetch the current profile")
		}

		for _, group := range batch {
			if !seenIDs[group.ID] {
				seenIDs[group.ID] = true
				groups = append(groups, group)
			}
		}
		progress(groups)
		if !more {
			return groups, nil
		}
		if next == "" || seenCursors[next] {
			return groups, errors.New("Facebook returned an invalid or repeated pagination cursor. Click Load groups to try again")
		}
		seenCursors[next] = true
		body, err = replaceGroupsCursor(rawBody, next)
		if err != nil {
			return groups, err
		}
		// Sequential paging leaves the browser responsive and avoids bursts.
		select {
		case <-ctx.Done():
			return groups, errors.New("Group loading was stopped")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func parseGroupsResponse(body []byte) ([]Group, string, bool, error) {
	// Facebook may prepend an anti-JSON-hijacking guard or stream JSON envelopes.
	text := strings.TrimSpace(string(body))
	text = strings.TrimSpace(strings.TrimPrefix(text, "for (;;);"))
	decoder := json.NewDecoder(strings.NewReader(text))
	type connection struct {
		Edges []struct {
			Node struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				URL     string `json:"url"`
				Type    string `json:"__typename"`
				Picture struct {
					URI string `json:"uri"`
				} `json:"profile_picture"`
			} `json:"node"`
		} `json:"edges"`
		PageInfo *struct {
			EndCursor string `json:"end_cursor"`
			HasNext   *bool  `json:"has_next_page"`
		} `json:"page_info"`
	}
	var list *connection
	var queryError error
	for {
		var envelope struct {
			Errors []json.RawMessage `json:"errors"`
			Error  json.RawMessage   `json:"error"`
			Data   struct {
				Viewer struct {
					AllJoined struct {
						List *connection `json:"tab_groups_list"`
					} `json:"all_joined_groups"`
				} `json:"viewer"`
			} `json:"data"`
		}
		err := decoder.Decode(&envelope)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", false, errors.New("Facebook returned an unreadable groups response. Check your login and try again")
		}
		if len(envelope.Errors) > 0 || (len(envelope.Error) > 0 && string(envelope.Error) != "null" && string(envelope.Error) != "0") {
			queryError = groupsQueryError(envelope.Errors, envelope.Error)
		}
		if envelope.Data.Viewer.AllJoined.List != nil {
			list = envelope.Data.Viewer.AllJoined.List
		}
	}
	if list == nil || list.PageInfo == nil || list.PageInfo.HasNext == nil {
		if queryError != nil {
			return nil, "", false, queryError
		}
		return nil, "", false, errors.New("Facebook's groups response format was not recognized")
	}
	groups := []Group{}
	for _, edge := range list.Edges {
		node := edge.Node
		if node.Type != "Group" || node.ID == "" || node.Name == "" {
			return nil, "", false, errors.New("Facebook returned an unexpected group entry")
		}
		groups = append(groups, Group{ID: node.ID, Name: node.Name, URL: node.URL, Picture: node.Picture.URI})
	}
	return groups, list.PageInfo.EndCursor, *list.PageInfo.HasNext, nil
}

func cookieHeader(cookies []*http.Cookie) http.Header {
	request := &http.Request{Header: make(http.Header)}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	return request.Header
}
