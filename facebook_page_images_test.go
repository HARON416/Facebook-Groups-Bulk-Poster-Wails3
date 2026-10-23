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

func TestCollectFacebookPhotosVariantsAndOrder(t *testing.T) {
	var story any
	err := json.Unmarshal([]byte(`{"attachments":[{"media":{"__typename":"Photo","id":"42","image":{"uri":"https://cdn.example.test/42-preview.jpg","width":320,"height":240},"photo_image":{"uri":"https://cdn.example.test/42-large.jpg","width":2048,"height":1536},"thumbnail":{"uri":"https://cdn.example.test/42-tiny.jpg"}}},{"subattachments":{"nodes":[{"media":{"__typename":"Photo","id":"43","image":{"uri":"https://cdn.example.test/43.jpg"}}},{"media":{"__typename":"Photo","id":"44","image":{"uri":"https://cdn.example.test/44.jpg"}}}]}},{"media":{"__typename":"Video","id":"45","image":{"uri":"https://cdn.example.test/video-poster.jpg"}}},{"media":{"__typename":"Photo","id":"42","image":{"uri":"https://cdn.example.test/42-small.jpg","width":100,"height":100}}}],"actors":[{"image":{"uri":"https://cdn.example.test/avatar.jpg"}}]}`), &story)
	if err != nil {
		t.Fatal(err)
	}
	photos := collectFacebookPhotos(story)
	if len(photos) != 3 || photos[0].id != "42" || photos[1].id != "43" || photos[2].id != "44" || photos[0].image.uri != "https://cdn.example.test/42-large.jpg" {
		t.Fatalf("photos: %+v", photos)
	}
}

func TestPhotoResponseSelectsMatchingViewerImage(t *testing.T) {
	response := `for (;;);{"data":{"currMedia":{"__typename":"Photo","id":"42","image":{"uri":"https://cdn.example.test/viewer.jpg?signature=keep%2Bthis","width":1600,"height":1200},"owner":{"image":{"uri":"https://cdn.example.test/avatar.jpg"}}}},"hasNext":true}` + "\n" + `{"path":["currMedia"],"data":{"photo_image":{"uri":"https://cdn.example.test/larger.jpg","width":2048,"height":1536}},"extensions":{"is_final":true}}`
	image, err := readFacebookPhotoResponse(strings.NewReader(response), "42")
	if err != nil || image.uri != "https://cdn.example.test/larger.jpg" {
		t.Fatalf("image=%+v err=%v", image, err)
	}
	for _, body := range []string{`{"data":{"currMedia":{"id":"43","image":{"uri":"https://cdn.example.test/wrong.jpg"}}}}`, `{"errors":[{"message":"private session details"}]}`, `{"data":{"currMedia":null}}`, `{"data":`} {
		if _, err := readFacebookPhotoResponse(strings.NewReader(body), "42"); err == nil {
			t.Fatal("accepted invalid or unrelated image")
		}
	}
}

func photoTimelineResponse(ids []string, cursor string, more bool) string {
	var response map[string]any
	_ = json.Unmarshal([]byte(timelineResponse(ids, cursor, more)), &response)
	edges := response["data"].(map[string]any)["node"].(map[string]any)["timeline_list_feed_units"].(map[string]any)["edges"].([]any)
	for i, edge := range edges {
		story := edge.(map[string]any)["node"].(map[string]any)["comet_sections"].(map[string]any)["content"].(map[string]any)["story"].(map[string]any)
		story["attachments"] = []any{map[string]any{"media": map[string]any{"__typename": "Photo", "id": ids[i] + "00", "url": "https://web.facebook.com/photo/?fbid=" + ids[i] + "00&set=pcb." + ids[i], "image": map[string]any{"uri": "https://cdn.example.test/preview.jpg", "width": 300, "height": 200}}}}
	}
	data, _ := json.Marshal(response)
	return string(data)
}

func TestTimelineReplaysCapturedPhotoQuery(t *testing.T) {
	for _, scenario := range []string{"resolved", "wrong-photo", "session-changed", "rejected", "retry"} {
		t.Run(scenario, func(t *testing.T) {
			photoRequests := 0
			fullURL := "https://cdn.example.test/viewer.jpg?stp=dst-jpg_tt6&signature=keep%2Bthis"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/graphql/" {
					t.Errorf("unexpected photo-page navigation: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				user, err := r.Cookie("c_user")
				if err != nil || user.Value != "123" {
					t.Error("lost captured session")
				}
				_ = r.ParseForm()
				if r.Form.Get("fb_api_req_friendly_name") != facebookPhotoQuery {
					fmt.Fprint(w, photoTimelineResponse([]string{"1", "2"}, "", false))
					return
				}
				photoRequests++
				if r.Form.Get("__req") != strconv.FormatInt(int64(10+photoRequests), 36) {
					t.Error("photo request counter did not advance")
				}
				var variables map[string]any
				_ = json.Unmarshal([]byte(r.Form.Get("variables")), &variables)
				id, _ := variables["nodeID"].(string)
				if variables["mediasetToken"] != "pcb."+strings.TrimSuffix(id, "00") || variables["scale"] != float64(1) || variables["renderLocation"] != "comet_media_viewer" || r.Form.Get("doc_id") != "fresh-photo-doc" || r.Form.Get("fb_dtsg") != "fresh-photo-token" || r.Header.Get("X-Fb-Friendly-Name") != facebookPhotoQuery || r.Form.Get("opaque") != "keep+exact" || !reflect.DeepEqual(r.Form["repeat"], []string{"one", "two"}) {
					t.Error("lost fresh photo fields or wrong collection")
				}
				if photoRequests > 1 {
					xs, _ := r.Cookie("xs")
					if xs == nil || xs.Value != "rotated" {
						t.Error("lost rotated cookies")
					}
				}
				http.SetCookie(w, &http.Cookie{Name: "xs", Value: "rotated", Path: "/"})
				if scenario == "session-changed" {
					http.SetCookie(w, &http.Cookie{Name: "c_user", Value: "other", Path: "/"})
				}
				if scenario == "rejected" {
					fmt.Fprint(w, `{"errors":[{"message":"do not expose this token"}]}`)
					return
				}
				if scenario == "retry" && photoRequests == 1 {
					w.WriteHeader(503)
					return
				}
				if scenario == "wrong-photo" {
					id = "999"
				}
				fmt.Fprintf(w, `{"data":{"currMedia":{"__typename":"Photo","id":%q,"image":{"uri":%q,"width":1600,"height":1200}}},"extensions":{"is_final":true}}`, id, fullURL)
			}))
			defer server.Close()
			headers := http.Header{"Cookie": {"c_user=123; xs=session"}, "Content-Type": {"application/x-www-form-urlencoded"}}
			photoHeaders := headers.Clone()
			photoHeaders.Set("X-Fb-Friendly-Name", facebookPhotoQuery)
			photo := &timelineRequest{URL: server.URL + "/api/graphql/", User: "123", Headers: photoHeaders, Body: "doc_id=fresh-photo-doc&fb_dtsg=fresh-photo-token&__req=a&opaque=keep%2Bexact&repeat=one&repeat=two&fb_api_req_friendly_name=" + facebookPhotoQuery + "&variables=" + url.QueryEscape(`{"nodeID":"999","mediasetToken":"pcb.999","scale":1,"renderLocation":"comet_media_viewer","focusCommentID":"old-photo-comment"}`)}
			request := timelineRequest{URL: server.URL + "/api/graphql/", User: "123", Headers: headers, Body: "doc_id=fresh&variables=" + url.QueryEscape(`{"id":"456","count":2,"cursor":null}`), PhotoQuery: photo}
			posts, err := replayFacebookTimeline(context.Background(), request, []*proto.NetworkCookie{{Name: "c_user", Value: "123", Path: "/"}, {Name: "xs", Value: "session", Path: "/"}}, 2, nil)
			if scenario != "resolved" && scenario != "retry" {
				if err == nil {
					t.Fatal("silently accepted invalid photo response")
				}
				return
			}
			expected := 2
			if scenario == "retry" {
				expected = 3
			}
			if err != nil || len(posts) != 2 || photoRequests != expected {
				t.Fatalf("posts=%d photoRequests=%d err=%v", len(posts), photoRequests, err)
			}
			for _, post := range posts {
				if !reflect.DeepEqual(post.Images, []string{fullURL}) {
					t.Fatalf("viewer URLs: %v", post.Images)
				}
			}
		})
	}
}

func TestFullSizePhotosDoNotFallBackToUnidentifiedPreview(t *testing.T) {
	post := FacebookPagePost{photos: []facebookPagePhoto{{image: facebookPhotoImage{uri: "https://cdn.example.test/tiny.jpg"}}}}
	_, err := resolveFacebookPostImages(context.Background(), nil, nil, nil, timelineRequest{}, post, nil)
	if err == nil || !strings.Contains(err.Error(), "photo ID") {
		t.Fatalf("expected explicit full-size resolution failure, got %v", err)
	}
}

func TestPhotoResponseStopsAtFinalMarker(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	go func() {
		_, _ = io.WriteString(writer, `{"data":{"currMedia":{"id":"42","image":{"uri":"https://cdn.example.test/42.jpg","width":1600,"height":1200}}},"extensions":{"is_final":true}}`+"\n")
		// Keep the stream open, as Facebook can after a Relay final marker.
	}()
	done := make(chan error, 1)
	go func() { _, err := readFacebookPhotoResponse(reader, "42"); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("waited for HTTP EOF after final photo data")
	}
}

func TestPhotoBodyPreservesOpaqueFields(t *testing.T) {
	raw := `__req=az&opaque=keep%2bexact&repeat=one&repeat=two&variables=` + url.QueryEscape(`{"nodeID":"1","mediasetToken":"pcb.2","scale":1,"provider":{"keep":true},"focusCommentID":null}`)
	body, err := facebookPhotoBody(raw, "42", "pcb.99")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(body, "__req=az&opaque=keep%2bexact&repeat=one&repeat=two&variables=") {
		t.Fatal("opaque field bytes changed")
	}
	form, _ := url.ParseQuery(body)
	var variables map[string]any
	_ = json.Unmarshal([]byte(form.Get("variables")), &variables)
	if variables["nodeID"] != "42" || variables["mediasetToken"] != "pcb.99" || variables["provider"].(map[string]any)["keep"] != true || variables["scale"] != float64(1) {
		t.Fatal("variables were lost")
	}
}
