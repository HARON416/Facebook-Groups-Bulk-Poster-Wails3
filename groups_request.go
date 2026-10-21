package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-rod/rod/lib/proto"
)

// Keep every application header, including unknown future headers and Priority.
// Go manages host, framing, and gzip decoding; replaying Chrome's br/zstd
// negotiation would require decoding those encodings ourselves.
func mergeReplayHeaders(base http.Header, extra proto.NetworkHeaders) http.Header {
	headers := base.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	for key, value := range extra {
		if strings.HasPrefix(key, ":") {
			continue
		} // HTTP/2 pseudo headers.
		headers.Set(key, value.Str())
	}
	for _, key := range strings.Split(headers.Get("Connection"), ",") {
		headers.Del(strings.TrimSpace(key))
	}
	for _, key := range []string{"Host", "Content-Length", "Accept-Encoding", "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		headers.Del(key)
	}
	return headers
}

func cookieUser(header string) string {
	request := &http.Request{Header: http.Header{"Cookie": []string{header}}}
	cookie, err := request.Cookie("c_user")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// Replace only the encoded variables field. Preserve every other form field,
// its position, repeated values, and original escaping exactly as captured.
func replaceGroupsCursor(rawBody, cursor string) (string, error) {
	fields := strings.Split(rawBody, "&")
	found := false
	for i, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		name, err := url.QueryUnescape(key)
		if err != nil || name != "variables" {
			continue
		}
		if found || !ok {
			return "", errors.New("The captured groups variables are ambiguous")
		}
		found = true
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			return "", errors.New("The captured groups variables are invalid")
		}
		var variables map[string]json.RawMessage
		if json.Unmarshal([]byte(decoded), &variables) != nil || variables == nil {
			return "", errors.New("The captured groups variables are invalid")
		}
		variables["cursor"], _ = json.Marshal(cursor)
		encoded, err := json.Marshal(variables)
		if err != nil {
			return "", errors.New("Could not encode the groups cursor")
		}
		fields[i] = key + "=" + url.QueryEscape(string(encoded))
	}
	if !found {
		return "", errors.New("The captured request has no groups variables")
	}
	return strings.Join(fields, "&"), nil
}

// Report numeric codes only. Facebook error messages can contain private data.
func groupsQueryError(items []json.RawMessage, topLevel json.RawMessage) error {
	code := ""
	numeric := func(raw json.RawMessage) string {
		text := strings.Trim(string(raw), `"`)
		if _, err := strconv.ParseUint(text, 10, 64); err == nil {
			return text
		}
		return ""
	}
	code = numeric(topLevel)
	for _, raw := range items {
		var item struct {
			Code       json.RawMessage `json:"code"`
			Extensions struct {
				Code      json.RawMessage `json:"code"`
				ErrorCode json.RawMessage `json:"error_code"`
			} `json:"extensions"`
		}
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		for _, rawCode := range []json.RawMessage{item.Code, item.Extensions.Code, item.Extensions.ErrorCode} {
			if value := numeric(rawCode); value != "" {
				code = value
				break
			}
		}
		if code != "" {
			break
		}
	}
	detail := ""
	if code != "" {
		detail = fmt.Sprintf(" (code %s)", code)
	}
	return fmt.Errorf("Facebook rejected the groups query%s. Click Load groups to capture fresh values", detail)
}
