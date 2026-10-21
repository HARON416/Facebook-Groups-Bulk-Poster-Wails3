package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// Display-only outcomes. Cookies, captured bodies and tokens never leave Go.
type PostOutcome struct {
	Group      Group  `json:"group"`
	State      string `json:"state"`
	Message    string `json:"message"`
	PostID     string `json:"postId"`
	URL        string `json:"url"`
	Code       string `json:"code"`
	stop       bool
	diagnostic string
	response   string
}

func facebookObjects(body []byte) ([]map[string]any, bool) {
	body = bytes.TrimSpace(body)
	if bytes.HasPrefix(body, []byte("for (;;);")) {
		body = bytes.TrimSpace(body[len("for (;;);"):])
	}
	if bytes.HasPrefix(body, []byte("for(;;);")) {
		body = bytes.TrimSpace(body[len("for(;;);"):])
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var objects []map[string]any
	for {
		var object map[string]any
		err := decoder.Decode(&object)
		if err == io.EOF {
			break
		}
		if err != nil {
			return objects, false
		}
		objects = append(objects, object)
	}
	return objects, len(objects) > 0
}
func record(v any) map[string]any { m, _ := v.(map[string]any); return m }
func stringValue(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return string(v)
	}
	return ""
}
func visitRecords(v any, fn func(map[string]any)) {
	switch v := v.(type) {
	case map[string]any:
		fn(v)
		for _, child := range v {
			visitRecords(child, fn)
		}
	case []any:
		for _, child := range v {
			visitRecords(child, fn)
		}
	}
}

func parsePostResponse(body []byte, status int) PostOutcome {
	result := PostOutcome{State: "unknown", Message: "Facebook did not confirm this post. Check the group before trying again.", stop: true}
	objects, valid := facebookObjects(body)
	var failure *PostOutcome
	failurePriority := -1
	var confirmed *PostOutcome
	for _, object := range objects {
		// Error codes are more reliable than translated labels. English text is a
		// fallback for the UI/API variants also handled by the extension.
		// Story data contains captions, notices and unrelated nested content.
		// Only the response envelope carries mutation errors; caption text must
		// never turn an accepted post into a restriction or failure.
		envelope := make(map[string]any, len(object))
		for key, value := range object {
			if key != "data" {
				envelope[key] = value
			}
		}
		visitRecords(envelope, func(m map[string]any) {
			code := stringValue(m["code"])
			apiCode := stringValue(m["api_error_code"])
			if code == "" {
				code = stringValue(m["api_error_code"])
			}
			if code == "" {
				code = stringValue(m["error"])
			}
			message := stringValue(m["message"]) + " " + stringValue(m["errorDescription"]) + " " + stringValue(m["errorSummary"])
			lower := strings.ToLower(message)
			f := PostOutcome{State: "failed", Code: code}
			priority := 0
			if code != "" {
				priority = 1
			}
			switch {
			case code == "1390008" || code == "368" || apiCode == "368" || strings.Contains(lower, "we limit how often") || strings.Contains(lower, "temporarily blocked") || strings.Contains(lower, "temporarily restricted") || strings.Contains(lower, "posting limit") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "posting has been limited") || strings.Contains(lower, "posting has been restricted"):
				f.Message = "Facebook has temporarily limited posting. Try again later."
				f.stop = true
				priority = 3
			case code == "1357006":
				f.Message = "Facebook could not post to this group. Continuing with the remaining groups."
				priority = 2
			case strings.Contains(lower, "limit for pending content"):
				f.Message = "This group has reached its pending-content limit."
				priority = 2
			case code == "190" || code == "102":
				f.Message = "Your Facebook session needs login or verification. Load groups again."
				f.stop = true
				priority = 3
			case m["errorSummary"] != nil || m["error"] != nil || lenErrorList(m["errors"]) > 0:
				f.Message = "Facebook rejected this post."
				f.stop = true
			default:
				return
			}
			if failure == nil || priority > failurePriority || priority == failurePriority && f.stop && !failure.stop {
				failure = &f
				failurePriority = priority
			}
		})
		// Only consider the create mutation's payload, never unrelated feed posts.
		visitRecords(object["data"], func(m map[string]any) {
			create := record(m["story_create"])
			if create == nil {
				return
			}
			story := record(create["story"])
			id := stringValue(create["post_id"])
			if id == "" {
				id = stringValue(story["post_id"])
			}
			if id == "" {
				id = stringValue(story["legacy_story_hideable_id"])
			}
			if id == "" {
				id = stringValue(story["id"])
			}
			link := stringValue(create["url"])
			if link == "" {
				link = stringValue(story["url"])
			}
			pending := strings.Contains(link, "/pending_posts/") || story["if_viewer_can_learn_more_about_pending_post"] != nil && story["if_viewer_can_learn_more_about_pending_post"] != false || record(story["to"])["if_viewer_can_see_pending_post_in_mall"] != nil && record(story["to"])["if_viewer_can_see_pending_post_in_mall"] != false
			sections := record(story["comet_sections"])
			pending = pending || strings.Contains(stringValue(record(sections["layout"])["__typename"]), "Pending") || strings.Contains(stringValue(record(sections["outer_footer"])["__typename"]), "Pending")
			if id == "" && !pending && !strings.Contains(link, "/posts/") && !strings.Contains(link, "/permalink/") {
				return
			}
			r := PostOutcome{State: "posted", Message: "Post published.", PostID: id, URL: link}
			if pending {
				r.State = "pending"
				r.Message = "Post submitted for admin approval."
			}
			confirmed = &r
		})
	}
	if confirmed != nil && failure != nil {
		return result // The caller handles bounded retries for unconfirmed results.
	}
	if valid && status >= 200 && status < 300 && confirmed != nil {
		return *confirmed
	}
	if failure != nil {
		return *failure
	}
	if status == 429 {
		return PostOutcome{State: "failed", Message: "Facebook has temporarily limited posting. Try again later.", stop: true}
	}
	if status == 401 || status == 403 {
		return PostOutcome{State: "failed", Message: "Facebook requires login or verification. Load groups again.", stop: true}
	}
	return result
}

func lenErrorList(value any) int { list, _ := value.([]any); return len(list) }

// A returned story alone proves neither the intended group nor publication.
// Keep unverified submissions distinct from success; the posting runner decides
// when to retry and retains exhausted uncertain submissions in its ledger.
func parsePostResponseForGroup(body []byte, status int, groupID string) PostOutcome {
	result := parsePostResponse(body, status)
	result.response = string(body)
	result.diagnostic = postResponseDiagnostic(body, status, groupID)
	if result.State != "posted" && result.State != "pending" {
		return result
	}
	unknown := PostOutcome{State: "unknown", Message: "Facebook returned a story, but did not confirm its group or publication state. Posting stopped. Check Facebook before retrying.", stop: true, diagnostic: result.diagnostic, response: result.response}
	objects, _ := facebookObjects(body)
	matched, mismatch, publicationKnown := false, false, false
	for _, object := range objects {
		visitRecords(object["data"], func(m map[string]any) {
			create := record(m["story_create"])
			if create == nil {
				return
			}
			story := record(create["story"])
			groupAddresses := map[string]bool{groupID: true}
			for _, target := range []map[string]any{record(story["to"]), record(story["target_group"])} {
				if id := stringValue(target["id"]); id != "" {
					matched = matched || id == groupID
					mismatch = mismatch || id != groupID
					// Facebook permalinks may use a custom group address. Only
					// trust an alias tied to the requested ID in this response.
					if id == groupID {
						if address, valid := facebookGroupAddress(stringValue(target["url"])); valid && address != "" && !numericGroupAddress(address) {
							groupAddresses[address] = true
						}
					}
				}
			}
			for _, raw := range []string{stringValue(create["url"]), stringValue(story["url"]), stringValue(story["permalink_url"])} {
				if raw == "" {
					continue
				}
				address, valid := facebookGroupAddress(raw)
				if !valid {
					mismatch = true
					continue
				}
				if address != "" {
					matched = matched || groupAddresses[address]
					mismatch = mismatch || !groupAddresses[address]
				}
			}
			// Missing fields are not the same as explicit null/false fields.
			learnMore, hasLearnMore := story["if_viewer_can_learn_more_about_pending_post"]
			pendingMall, hasPendingMall := record(story["to"])["if_viewer_can_see_pending_post_in_mall"]
			layout := stringValue(record(record(story["comet_sections"])["layout"])["__typename"])
			publicationKnown = publicationKnown || layout == "CometStoryDefaultLayoutStrategy" ||
				hasLearnMore && hasPendingMall && (learnMore == nil || learnMore == false) && (pendingMall == nil || pendingMall == false)
		})
	}
	if mismatch {
		unknown.Message = "Facebook returned a post for a different group than requested. Posting stopped. Check Facebook before retrying."
		return unknown
	}
	if !matched || result.State == "posted" && !publicationKnown {
		if matched {
			unknown.PostID = result.PostID
			unknown.URL = cleanPostLink(result.URL)
		}
		return unknown
	}
	result.URL = cleanPostLink(result.URL)
	return result
}

func facebookGroupAddress(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		(u.Hostname() != "web.facebook.com" && u.Hostname() != "www.facebook.com" && u.Hostname() != "facebook.com") {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "groups" {
		return parts[1], true
	}
	return "", true
}

func numericGroupAddress(value string) bool {
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return value != ""
}

func cleanPostLink(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		(u.Hostname() != "web.facebook.com" && u.Hostname() != "www.facebook.com" && u.Hostname() != "facebook.com") {
		return ""
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

func postResponseDiagnostic(body []byte, status int, groupID string) string {
	// Log selected structural metadata only. Never serialize the response, which
	// also contains captions, tracking values, profile details and signed links.
	safeID := func(value string) string {
		if value == "" || len(value) > 128 {
			return "missing"
		}
		for _, c := range value {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_:+=/", c)) {
				return "unavailable"
			}
		}
		return value
	}
	flag := func(m map[string]any, key string) string {
		v, ok := m[key]
		if !ok {
			return "missing"
		}
		if v == nil || v == false {
			return "no"
		}
		return "yes"
	}
	returned, postID, layout, approval, mall := "missing", "missing", "missing", "missing", "missing"
	objects, _ := facebookObjects(body)
	for _, object := range objects {
		visitRecords(object["data"], func(m map[string]any) {
			create := record(m["story_create"])
			if create == nil {
				return
			}
			story := record(create["story"])
			returned = safeID(stringValue(record(story["to"])["id"]))
			if returned == "missing" {
				returned = safeID(stringValue(record(story["target_group"])["id"]))
			}
			postID = safeID(stringValue(create["post_id"]))
			if postID == "missing" {
				postID = safeID(stringValue(story["post_id"]))
			}
			if postID == "missing" {
				postID = safeID(stringValue(story["legacy_story_hideable_id"]))
			}
			layout = safeID(stringValue(record(record(story["comet_sections"])["layout"])["__typename"]))
			approval = flag(story, "if_viewer_can_learn_more_about_pending_post")
			mall = flag(record(story["to"]), "if_viewer_can_see_pending_post_in_mall")
		})
	}
	return fmt.Sprintf("API confirmation: HTTP %d · requested group %s · returned group %s · post ID %s · layout %s · pending flags %s/%s", status, safeID(groupID), returned, postID, layout, approval, mall)
}

func parseUploadedPhoto(body []byte, status int) (string, PostOutcome) {
	objects, valid := facebookObjects(body)
	if valid && len(objects) == 1 && status >= 200 && status < 300 {
		object := objects[0]
		id := stringValue(record(object["payload"])["photoID"])
		if stringValue(object["__ar"]) == "1" && id != "" && object["error"] == nil {
			return id, PostOutcome{}
		}
	}
	result := parsePostResponse(body, status)
	if result.State == "unknown" {
		result.State = "failed"
		result.Message = "Facebook did not confirm the image upload."
	}
	return "", result
}
