package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

func TestReplayHeadersIncludeExtraInfo(t *testing.T) {
	base := http.Header{"X-Fb-Lsd": []string{"basic"}, "Accept": []string{"*/*"}}
	headers := mergeReplayHeaders(base, proto.NetworkHeaders{
		"x-fb-lsd": gson.New("wire"), "priority": gson.New("u=1, i"), "cookie": gson.New("xs=encoded%3Asession; c_user=fresh-user"), "future-header": gson.New("opaque"),
		"accept-encoding": gson.New("gzip, br, zstd"), "content-length": gson.New("999"), ":authority": gson.New("example.test"), "connection": gson.New("custom-hop"), "custom-hop": gson.New("omit"),
	})
	if headers.Get("X-Fb-Lsd") != "wire" || headers.Get("Priority") == "" || headers.Get("Future-Header") != "opaque" || cookieUser(headers.Get("Cookie")) != "fresh-user" {
		t.Fatal("wire headers were not preserved")
	}
	if base.Get("X-Fb-Lsd") != "basic" {
		t.Fatal("capture mutated basic headers")
	}
	for _, key := range []string{"Accept-Encoding", "Content-Length", ":authority", "Connection", "Custom-Hop"} {
		if headers.Get(key) != "" {
			t.Fatalf("transport header %s was retained", key)
		}
	}
}

func TestReplaceCursorPreservesEveryOtherField(t *testing.T) {
	raw := `__dyn=opaque%2fvalue&repeated=one&lsd=a%20b&variables=` + url.QueryEscape(`{"count":20,"cursor":null,"ordering":["captured"],"opaque_integer":18446744073709551615,"extra":{"nested":true}}`) + `&repeated=two&future=hello+world&fb_dtsg=token%3aopaque`
	result, err := replaceGroupsCursor(raw, "cursor +/&=")
	if err != nil {
		t.Fatal(err)
	}
	originalFields := strings.Split(raw, "&")
	resultFields := strings.Split(result, "&")
	for i, field := range originalFields {
		if !strings.HasPrefix(field, "variables=") && field != resultFields[i] {
			t.Fatalf("changed field %d", i)
		}
	}
	original, _ := url.ParseQuery(raw)
	updated, _ := url.ParseQuery(result)
	var oldVariables, newVariables map[string]json.RawMessage
	_ = json.Unmarshal([]byte(original.Get("variables")), &oldVariables)
	_ = json.Unmarshal([]byte(updated.Get("variables")), &newVariables)
	delete(oldVariables, "cursor")
	cursor := newVariables["cursor"]
	delete(newVariables, "cursor")
	if !reflect.DeepEqual(oldVariables, newVariables) {
		t.Fatal("non-cursor variables changed")
	}
	var value string
	_ = json.Unmarshal(cursor, &value)
	if value != "cursor +/&=" {
		t.Fatal("cursor encoding changed")
	}
	for _, raw := range []string{"x=y", "variables=bad", "variables=null", "variables=%7B%7D&variables=%7B%7D"} {
		if _, err := replaceGroupsCursor(raw, "next"); err == nil {
			t.Fatal("invalid variables accepted")
		}
	}
}

func TestGroupsResponseErrorsAndPartialData(t *testing.T) {
	_, _, _, err := parseGroupsResponse([]byte(`{"data":null,"errors":[{"message":"private token","code":1675004}]}`))
	if err == nil || !strings.Contains(err.Error(), "1675004") || strings.Contains(err.Error(), "private token") {
		t.Fatalf("unsafe or unhelpful error: %v", err)
	}
	var response map[string]any
	_ = json.Unmarshal([]byte(groupResponse([]string{"one"}, "", false)), &response)
	response["errors"] = []map[string]any{{"message": "unrelated field failed", "code": 123}}
	body, _ := json.Marshal(response)
	groups, _, _, err := parseGroupsResponse(body)
	if err != nil || len(groups) != 1 {
		t.Fatalf("valid group data rejected for unrelated errors: %v", err)
	}
}
