package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-rod/rod/lib/proto"
)

type postReplay struct {
	progress     func(string)
	client       *http.Client
	user         string
	upload       capturedPhotoUpload
	post         capturedPost
	sequence     uint64
	allowRewrite bool
}

func newPostReplay(cookies []*proto.NetworkCookie, upload capturedPhotoUpload, post capturedPost) (*postReplay, error) {
	if post.User == "" || cookieUser(post.Headers.Get("Cookie")) != post.User || upload.URL != "" && (upload.User != post.User || cookieUser(upload.Headers.Get("Cookie")) != post.User) {
		return nil, errors.New("Upload and post captures must belong to the same logged-in profile")
	}
	if upload.URL != "" && (!isFacebookPhotoUploadURL(upload.URL) || len(upload.Files) != 1 || upload.Files[0].Field == "") {
		return nil, errors.New("The captured image upload format is not supported")
	}
	jar, _ := cookiejar.New(nil)
	for _, cookie := range cookies {
		host := strings.TrimPrefix(cookie.Domain, ".")
		u := &url.URL{Scheme: "https", Host: host, Path: "/"}
		jar.SetCookies(u, []*http.Cookie{{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Secure: cookie.Secure, HttpOnly: cookie.HTTPOnly}})
	}
	for _, endpoint := range []string{upload.URL, post.URL} {
		if endpoint == "" {
			continue
		}
		u, _ := url.Parse(endpoint)
		if cookieUser(cookieHeader(jar.Cookies(u)).Get("Cookie")) != post.User {
			return nil, errors.New("The browser profile changed while capturing posting requests")
		}
	}
	return &postReplay{client: &http.Client{Jar: jar, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, user: post.User, upload: upload, post: post}, nil
}

func randomPostID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func cloneForm(form url.Values) url.Values {
	copy := make(url.Values, len(form))
	for k, v := range form {
		copy[k] = append([]string{}, v...)
	}
	return copy
}
func (replay *postReplay) nextRequest(form url.Values) {
	original, _ := strconv.ParseUint(form.Get("__req"), 36, 64)
	replay.sequence++
	form.Set("__req", strconv.FormatUint(original+replay.sequence, 36))
}
func buildReplayPost(raw, body, group string, photos []string) (url.Values, error) {
	return buildReplayPostWithRewrite(raw, body, group, photos, false)
}
func buildReplayPostWithRewrite(raw, body, group string, photos []string, allowRewrite bool) (url.Values, error) {
	form, err := url.ParseQuery(raw)
	if err != nil {
		return nil, errors.New("Invalid captured post form")
	}
	var variables map[string]any
	decoder := json.NewDecoder(strings.NewReader(form.Get("variables")))
	decoder.UseNumber()
	if decoder.Decode(&variables) != nil || record(variables["input"]) == nil {
		return nil, errors.New("Invalid captured post variables")
	}
	input := record(variables["input"])
	audience := record(input["audience"])
	if audience == nil || stringValue(audience["to_id"]) == "" {
		return nil, errors.New("The captured post has no group target")
	}
	oldGroup := stringValue(audience["to_id"])
	message := record(input["message"])
	if message == nil || stringValue(message["text"]) != body && !allowRewrite {
		return nil, errors.New("The captured post body does not match your draft")
	}
	if stringValue(message["text"]) != body {
		message["text"] = body
		message["ranges"] = []any{}
		normalized := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
		input["composed_text"] = map[string]any{"block_data": []string{"{}"}, "block_depths": []int{0}, "block_types": []int{0}, "blocks": []string{normalized}, "entities": []string{"[]"}, "entity_map": "{}", "inline_styles": []string{"[]"}}
	}
	// Preserve browser serialization when the body is unchanged. A Gemini rewrite
	// rebuilds only caption serialization; relay flags and unknown fields stay intact.
	audience["to_id"] = group
	// Match the extension's replacement of nested group references, including
	// logging and context variables, without changing numeric precision.
	replaceReplayGroup(variables, oldGroup, group)
	attachments := make([]any, 0, len(photos))
	for _, id := range photos {
		attachments = append(attachments, map[string]any{"photo": map[string]any{"id": id}})
	}
	input["attachments"] = attachments
	if logging := record(input["logging"]); logging != nil {
		logging["composer_session_id"] = randomPostID()
	}
	input["client_mutation_id"] = randomPostID()
	encoded, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}
	form.Set("variables", string(encoded))
	return form, nil
}

func replaceReplayGroup(value any, oldGroup, group string) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if text, ok := child.(string); ok && text == oldGroup {
				value[key] = group
			} else {
				replaceReplayGroup(child, oldGroup, group)
			}
		}
	case []any:
		for i, child := range value {
			if text, ok := child.(string); ok && text == oldGroup {
				value[i] = group
			} else {
				replaceReplayGroup(child, oldGroup, group)
			}
		}
	}
}

func (replay *postReplay) send(ctx context.Context, endpoint string, headers http.Header, body []byte) ([]byte, int, error) {
	return replay.sendAttempts(ctx, endpoint, headers, body, 0, nil)
}

// Reuse the exact POST bytes (including the mutation ID) across confirmation
// retries. The caller supplies a classifier for valid but unconfirmed responses.
func (replay *postReplay) sendAttempts(ctx context.Context, endpoint string, headers http.Header, body []byte, initialAttempt int, unconfirmed func([]byte, int) bool) ([]byte, int, error) {
	const retries = 3
	uncertain := initialAttempt > 0
	for attempt := initialAttempt; ; attempt++ {
		data, status, err := replay.sendOnce(ctx, endpoint, headers, body)
		// An explicit HTTP account/session restriction is authoritative even if
		// reading its response body failed.
		if ctx.Err() == nil && (status == 401 || status == 403 || status == 429) {
			return data, status, nil
		}
		var requestFailure *retryableReplayError
		var connectionError *net.OpError
		retry := errors.As(err, &requestFailure)
		if err == nil && unconfirmed != nil && unconfirmed(data, status) {
			retry = true
			err = errors.New("Facebook did not confirm the submission")
		}
		if ctx.Err() != nil || !retry {
			return data, status, err
		}
		if status != 0 || !errors.As(err, &connectionError) || connectionError.Op != "dial" {
			uncertain = true
		}
		if requestFailure != nil {
			offline, waitErr := recoverPostingConnection(ctx)
			if waitErr != nil {
				return data, status, waitErr
			}
			if offline {
				attempt--
				continue
			}
		}
		if attempt == retries {
			return data, status, &replayRetriesExhausted{error: err, uncertain: uncertain}
		}
		if replay.progress != nil {
			replay.progress(fmt.Sprintf("Submission unconfirmed. Retrying request %d/%d: %v", attempt+1, retries, err))
		}
	}
}

type replayRetriesExhausted struct {
	error
	uncertain bool
}

type retryableReplayError struct{ error }

func (err *retryableReplayError) Unwrap() error { return err.error }

func (err *replayRetriesExhausted) Unwrap() error { return err.error }

func (replay *postReplay) sendOnce(ctx context.Context, endpoint string, headers http.Header, body []byte) ([]byte, int, error) {
	if err := postingCheckpoint(ctx); err != nil {
		return nil, 0, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, 0, errors.New("Invalid captured endpoint")
	}
	if cookieUser(cookieHeader(replay.client.Jar.Cookies(u)).Get("Cookie")) != replay.user {
		return nil, 0, errors.New("The active Facebook profile changed or expired")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	request.Header = headers.Clone()
	for _, key := range []string{"Cookie", "Content-Length", "Host", "Accept-Encoding", "Connection", "Transfer-Encoding"} {
		request.Header.Del(key)
	}
	response, err := replay.client.Do(request)
	if err != nil {
		// A url.Error includes the captured URL and its authentication query.
		// Retain the underlying transport failure without logging that URL.
		var requestError *url.Error
		if errors.As(err, &requestError) {
			err = requestError.Err
		}
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		return nil, status, &retryableReplayError{fmt.Errorf("The connection failed; Facebook may have received the request: %w", err)}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024+1))
	if len(data) > 16*1024*1024 {
		return data[:16*1024*1024], response.StatusCode, &retryableReplayError{errors.New("Facebook's confirmation exceeded the 16 MiB response limit; retained response is truncated")}
	}
	if err != nil {
		return data, response.StatusCode, &retryableReplayError{fmt.Errorf("Could not read Facebook's confirmation; retained response is incomplete: %w", err)}
	}
	if cookieUser(cookieHeader(replay.client.Jar.Cookies(u)).Get("Cookie")) != replay.user {
		return data, response.StatusCode, errors.New("The active Facebook profile changed or expired")
	}
	return data, response.StatusCode, nil
}

func (replay *postReplay) uploadImage(ctx context.Context, image PostUpload) (string, PostOutcome) {
	bytesImage, err := base64.StdEncoding.DecodeString(image.Data)
	if err != nil {
		return "", PostOutcome{State: "failed", Message: "Could not read the selected image.", stop: true}
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for key, values := range replay.upload.Fields {
		for _, value := range values {
			if err = writer.WriteField(key, value); err != nil {
				return "", PostOutcome{State: "failed", Message: "Could not prepare the image upload.", stop: true}
			}
		}
	}
	file := replay.upload.Files[0]
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": file.Field, "filename": filepath.Base(image.Name)}))
	header.Set("Content-Type", http.DetectContentType(bytesImage))
	part, err := writer.CreatePart(header)
	if err == nil {
		_, err = part.Write(bytesImage)
	}
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		return "", PostOutcome{State: "failed", Message: "Could not prepare the image upload.", stop: true}
	}
	endpoint, _ := url.Parse(replay.upload.URL)
	query := endpoint.Query()
	replay.nextRequest(query)
	endpoint.RawQuery = query.Encode()
	headers := replay.upload.Headers.Clone()
	headers.Set("Content-Type", writer.FormDataContentType())
	body, status, err := replay.send(ctx, endpoint.String(), headers, buffer.Bytes())
	if err != nil {
		return "", replayRequestFailure("failed", "The image upload could not be confirmed. Posting stopped.", "Image upload", body, status, err)
	}
	id, outcome := parseUploadedPhoto(body, status)
	if id == "" {
		outcome.response = string(body)
	}
	return id, outcome
}
func (replay *postReplay) create(ctx context.Context, body string, photos []string, groupID string) PostOutcome {
	form, err := buildReplayPostWithRewrite(replay.post.Body, body, groupID, photos, replay.allowRewrite)
	if err != nil {
		return PostOutcome{State: "failed", Message: err.Error(), stop: true}
	}
	replay.nextRequest(form)
	response, status, err := replay.sendAttempts(ctx, replay.post.URL, replay.post.Headers, []byte(form.Encode()), 0, func(body []byte, status int) bool {
		return parsePostResponseForGroup(body, status, groupID).State == "unknown"
	})
	if err != nil {
		return replayRequestFailure("unknown", "The post could not be confirmed. Check Facebook before trying again.", "Post request", response, status, err)
	}
	return parsePostResponseForGroup(response, status, groupID)
}

func replayRequestFailure(state, message, phase string, body []byte, status int, err error) PostOutcome {
	responseStatus := "no HTTP response"
	if status != 0 {
		responseStatus = fmt.Sprintf("HTTP %d", status)
	}
	outcome := PostOutcome{
		State: state, Message: message, stop: true, response: string(body),
		diagnostic: fmt.Sprintf("%s failed: %s · %d response bytes retained · %v", phase, responseStatus, len(body), err),
	}
	var exhausted *replayRetriesExhausted
	if errors.As(err, &exhausted) {
		outcome.State = "failed"
		if state == "unknown" && exhausted.uncertain {
			outcome.State = "unknown"
		}
		outcome.stop = false
		outcome.Message = "Submission could not be confirmed after 3 retries. Continuing with the remaining groups."
		outcome.diagnostic += " · initial attempt and 3 retries exhausted"
	}
	return outcome
}
