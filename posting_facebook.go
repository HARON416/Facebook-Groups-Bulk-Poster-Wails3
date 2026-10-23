package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type facebookPostingQueueKey struct{}
type postingDraft struct {
	body   string
	images []PostUpload
}

func (s *GroupsService) PublishFacebookPostsWithOptions(ctx context.Context, selected []string, posts []FacebookPagePost, options PostingOptions) (*PostingStatus, error) {
	if len(selected) == 0 {
		return nil, errors.New("Select at least one group")
	}
	if len(posts) == 0 {
		return nil, errors.New("Fetch at least one post before posting")
	}
	snapshot := make([]FacebookPagePost, len(posts))
	for i, post := range posts {
		if strings.TrimSpace(post.Text) == "" {
			return nil, fmt.Errorf("Add text to fetched post %d before posting", i+1)
		}
		snapshot[i] = FacebookPagePost{ID: post.ID, Text: post.Text, Images: append([]string{}, post.Images...)}
		for _, image := range post.Images {
			if !facebookPostingImageURL(image) {
				return nil, errors.New("A fetched image URL is invalid. Fetch the posts again")
			}
		}
	}
	ctx = context.WithValue(ctx, facebookPostingQueueKey{}, snapshot)
	return s.PublishPostWithOptions(ctx, selected, snapshot[0].Text, nil, options)
}

func (s *GroupsService) ScheduleFacebookPostsWithOptions(ctx context.Context, selected []string, posts []FacebookPagePost, options PostingOptions, scheduledAt string) (*PostingStatus, error) {
	at, err := parsePostingSchedule(scheduledAt, time.Now())
	if err != nil {
		return nil, err
	}
	return s.PublishFacebookPostsWithOptions(context.WithValue(ctx, scheduledPostingKey{}, at), selected, posts, options)
}

func facebookPostingImageURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && (strings.HasSuffix(u.Hostname(), ".fbcdn.net") || strings.HasSuffix(u.Hostname(), ".fbsbx.com"))
}

// Resolve a snapshot once per run, after the scheduled time and under the same
// stop/pause controls as publishing. Never forward Facebook session cookies to
// image downloads or follow redirects outside Facebook's image CDN.
func (s *GroupsService) facebookPostingDrafts(ctx context.Context, posts []FacebookPagePost) ([]postingDraft, error) {
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !facebookPostingImageURL(req.URL.String()) {
			return errors.New("unexpected image redirect")
		}
		return nil
	}}
	return downloadFacebookPostingDrafts(ctx, client, posts, func(message string) { s.setPostStatus(message) })
}

func downloadFacebookPostingDrafts(ctx context.Context, client *http.Client, posts []FacebookPagePost, progress func(string)) ([]postingDraft, error) {
	drafts := make([]postingDraft, 0, len(posts))
	cache := map[string]PostUpload{}
	for i, post := range posts {
		draft := postingDraft{body: post.Text, images: []PostUpload{}}
		for j, raw := range post.Images {
			if err := postingCheckpoint(ctx); err != nil {
				return nil, err
			}
			image, ok := cache[raw]
			if !ok {
				if !facebookPostingImageURL(raw) {
					return nil, errors.New("A fetched image URL is invalid. Fetch the posts again")
				}
				if progress != nil {
					progress(fmt.Sprintf("Downloading full-size image %d of %d for post %d of %d…", j+1, len(post.Images), i+1, len(posts)))
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
				if err != nil {
					return nil, errors.New("Could not prepare a fetched image")
				}
				response, err := client.Do(req)
				if err != nil {
					return nil, errors.New("Could not download a fetched image. Check your connection or fetch the posts again")
				}
				data, readErr := io.ReadAll(io.LimitReader(response.Body, 32*1024*1024+1))
				response.Body.Close()
				if response.StatusCode != http.StatusOK || readErr != nil || len(data) > 32*1024*1024 {
					return nil, errors.New("Could not download a full-size image. Fetch the posts again")
				}
				extension := ""
				switch http.DetectContentType(data) {
				case "image/jpeg":
					extension = "jpg"
				case "image/png":
					extension = "png"
				case "image/gif":
					extension = "gif"
				case "image/webp":
					extension = "webp"
				}
				if extension == "" {
					return nil, errors.New("Facebook returned an unsupported image. Fetch the posts again")
				}
				image = PostUpload{Name: fmt.Sprintf("facebook-%d-%d.%s", i+1, j+1, extension), Data: base64.StdEncoding.EncodeToString(data)}
				cache[raw] = image
			}
			draft.images = append(draft.images, image)
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

func postingDraftFingerprint(drafts []postingDraft) string {
	if len(drafts) == 1 {
		return draftFingerprint(drafts[0].body, drafts[0].images)
	}
	var combined strings.Builder
	for _, draft := range drafts {
		combined.WriteString(draftFingerprint(draft.body, draft.images))
	}
	return draftFingerprint(combined.String(), nil)
}

func rotatingPostingDraft(drafts []postingDraft, ordinal int) (postingDraft, int) {
	index := ordinal % len(drafts)
	return drafts[index], index
}
