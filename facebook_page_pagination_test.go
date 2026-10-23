package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// Exercise the browser's 2 -> 3 -> 3 batch sizes with opaque server cursors.
// One continuation is interrupted; the same cursor must be retried rather
// than losing that page, duplicating posts, or stopping after the first two.
func TestTimelinePaginationTenPostsWithInterruptedPage(t *testing.T) {
	var requests []string
	var sequences []string
	interrupted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		var variables struct {
			Cursor *string `json:"cursor"`
			Count  int     `json:"count"`
			ID     string  `json:"id"`
		}
		if err := json.Unmarshal([]byte(r.Form.Get("variables")), &variables); err != nil {
			t.Error(err)
			return
		}
		cursor := "initial"
		if variables.Cursor != nil {
			cursor = *variables.Cursor
		}
		requests = append(requests, cursor)
		sequences = append(sequences, r.Form.Get("__req"))
		if variables.ID != "456" || r.Form.Get("doc_id") != "fresh-document" || r.Form.Get("fb_dtsg") != "fresh-token" || r.Header.Get("X-Fb-Lsd") != "fresh-lsd" {
			t.Error("captured session/target values changed")
		}
		if r.Form.Get("__dyn") != "opaque+&=values" || !reflect.DeepEqual(r.Form["repeated"], []string{"one", "two"}) {
			t.Error("opaque captured fields changed")
		}
		if cursor == "initial" && variables.Count != 2 || cursor != "initial" && variables.Count != 3 {
			t.Errorf("cursor=%q count=%d", cursor, variables.Count)
		}
		if cursor == "opaque/next+one==" && !interrupted {
			interrupted = true
			// Simulate a connection ending in the middle of a chunked body.
			connection, buffer, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			defer connection.Close()
			fmt.Fprint(buffer, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n100\r\n{\"data\":")
			_ = buffer.Flush()
			return
		}
		var ids []string
		var next string
		switch cursor {
		case "initial":
			ids, next = []string{"1", "2"}, "opaque/next+one=="
		case "opaque/next+one==":
			ids, next = []string{"3", "4", "5"}, "opaque/next+two=="
		case "opaque/next+two==":
			ids, next = []string{"6", "7", "8"}, "opaque/next+three=="
		case "opaque/next+three==":
			ids = []string{"9", "10", "11"}
		default:
			t.Errorf("unexpected cursor %q", cursor)
		}
		fmt.Fprint(w, timelineResponse(ids, next, next != ""))
		if cursor == "opaque/next+three==" {
			// The final JSON envelope completes this page before the HTTP
			// response ends. The client must close it and return the posts.
			fmt.Fprint(w, "\n"+`{"extensions":{"is_final":true}}`+"\n")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-time.After(6 * time.Second):
				t.Error("client waited for HTTP EOF after the final envelope")
			}
		}
	}))
	defer server.Close()
	request := timelineRequest{URL: server.URL + "/api/graphql/", User: "123", Headers: http.Header{"Cookie": {"c_user=123"}, "Content-Type": {"application/x-www-form-urlencoded"}, "X-Fb-Lsd": {"fresh-lsd"}}, Body: "__req=7&doc_id=fresh-document&fb_dtsg=fresh-token&__dyn=opaque%2B%26%3Dvalues&repeated=one&variables=" + url.QueryEscape(`{"id":"456","count":2,"cursor":"captured-scroll-position","feedLocation":"TIMELINE"}`) + "&repeated=two"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var progress []int
	posts, err := replayFacebookTimeline(ctx, request, []*proto.NetworkCookie{{Name: "c_user", Value: "123", Path: "/"}}, 10, func(posts []FacebookPagePost) { progress = append(progress, len(posts)) })
	if err != nil || len(posts) != 10 {
		t.Fatalf("posts=%d error=%v", len(posts), err)
	}
	for i, post := range posts {
		if post.ID != strconv.Itoa(i+1) {
			t.Errorf("post %d: %s", i, post.ID)
		}
	}
	if !reflect.DeepEqual(progress, []int{2, 5, 8, 10}) {
		t.Fatalf("progress: %v", progress)
	}
	if !reflect.DeepEqual(requests, []string{"initial", "opaque/next+one==", "opaque/next+one==", "opaque/next+two==", "opaque/next+three=="}) {
		t.Fatalf("pagination/retry cursors: %v", requests)
	}
	if !reflect.DeepEqual(sequences, []string{"8", "9", "a", "b", "c"}) {
		t.Fatalf("request counters: %v", sequences)
	}
}

func TestTimelineFinalEnvelopeDoesNotWaitForHTTPEOF(t *testing.T) {
	for _, final := range []string{`{"extensions":{"is_final":true}}`, `{"hasNext":false}`} {
		t.Run(final, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			go func() {
				_, _ = io.WriteString(writer, "for (;;);"+timelineResponse([]string{"1", "2", "3"}, "next", true)+"\n"+final+"\n")
			}()
			done := make(chan error, 1)
			go func() {
				posts, next, more, err := readFacebookPageResponse(reader)
				if err == nil && (len(posts) != 3 || next != "next" || !more) {
					err = fmt.Errorf("incomplete timeline page")
				}
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("reader waited for HTTP EOF after Relay completed the page")
			}
		})
	}
}

func TestTimelineRejectsTruncatedDeferredData(t *testing.T) {
	_, _, _, err := readFacebookPageResponse(strings.NewReader(timelineResponse([]string{"1", "2"}, "next", true) + "\n" + `{"path":["node"],"data":`))
	if err == nil {
		t.Fatal("accepted incomplete deferred data")
	}
}
