package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type PostingStatus struct {
	Active         bool          `json:"active"`
	PauseRequested bool          `json:"pauseRequested"`
	Offline        bool          `json:"offline"`
	ScheduledAt    string        `json:"scheduledAt"`
	State          string        `json:"state"`
	Message        string        `json:"message"`
	Results        []PostOutcome `json:"results"`
	Total          int           `json:"total"`
	Processed      int           `json:"processed"`
	Posted         int           `json:"posted"`
	Pending        int           `json:"pending"`
	Failed         int           `json:"failed"`
	Remaining      int           `json:"remaining"`
	Current        int           `json:"current"`
	CurrentGroup   Group         `json:"currentGroup"`
	ElapsedSeconds int           `json:"elapsedSeconds"`
	ItemSeconds    int           `json:"itemSeconds"`
	WaitSeconds    int           `json:"waitSeconds"`
}

func (s *GroupsService) GetPostingStatus() PostingStatus {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	return s.postingStatusLocked()
}
func (s *GroupsService) CancelPosting() {
	s.postMu.Lock()
	cancel := s.postingCancel
	if s.postingActiveLocked() && s.posting.State != "stopping" {
		s.posting.State = "stopping"
		s.postingPaused = false
		s.appendPostingLogLocked("info", "Stopping posting…")
		s.signalPostingLocked()
	}
	s.postMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (s *GroupsService) recordOutcome(outcome PostOutcome) {
	s.postMu.Lock()
	s.posting.Results = append(s.posting.Results, outcome)
	level := "info"
	if outcome.State == "skipped" {
		level = "warning"
	}
	if outcome.State == "failed" || outcome.State == "unknown" {
		level = "error"
	}
	message := outcome.Group.Name + ": " + outcome.Message
	if outcome.diagnostic != "" {
		s.appendPostingLogLocked("debug", outcome.diagnostic)
	}
	if outcome.State == "posted" {
		message = "Posted successfully to " + outcome.Group.Name + "."
	} else if outcome.State == "pending" {
		message = outcome.Group.Name + ": Your post is pending. Your post is awaiting admin approval. If the admin team approves it, it'll become visible in the group."
	}
	if outcome.Code != "" {
		message += " (Code " + outcome.Code + ")"
	}
	if (outcome.State == "posted" || outcome.State == "pending" || outcome.State == "unknown") && outcome.URL != "" {
		message += " " + outcome.URL
	}
	if !s.postingItemStarted.IsZero() {
		message += fmt.Sprintf(" · Item time %ds", int(time.Since(s.postingItemStarted).Seconds()))
	}
	s.appendPostingLogLocked(level, message)
	s.postingLogs[len(s.postingLogs)-1].Outcome = outcome.State
	s.postingLogs[len(s.postingLogs)-1].Diagnostic = outcome.diagnostic
	if outcome.response != "" {
		entry := &s.postingLogs[len(s.postingLogs)-1]
		entry.HasResponse = true
		entry.response = outcome.response
	}
	s.postMu.Unlock()
}
func draftFingerprint(body string, images []PostUpload) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d:%s", len(body), body)
	for _, image := range images {
		fmt.Fprintf(h, "%d:%s", len(image.Data), image.Data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// PublishPost counts the browser's first post once, then replays fresh, private
// templates for the remaining selected groups. Unconfirmed requests get up to
// three retries with the same mutation ID, then the queue proceeds.
func (s *GroupsService) PublishPostWithOptions(ctx context.Context, selected []string, body string, images []PostUpload, options PostingOptions) (*PostingStatus, error) {
	if !s.postRunMu.TryLock() {
		return nil, errors.New("Posting is already running")
	}
	defer s.postRunMu.Unlock()
	if strings.TrimSpace(body) == "" || len(images) == 0 {
		return nil, errors.New("Add a post body and at least one image")
	}
	if err := validatePostingOptions(options); err != nil {
		return nil, err
	}
	s.settingsMu.Lock()
	settings, settingsErr := s.readSettings()
	s.settingsMu.Unlock()
	if settingsErr != nil {
		return nil, settingsErr
	}
	if options.Rewrite && settings.APIKey == "" {
		return nil, errors.New("Add a Gemini API key in Settings before enabling rewriting")
	}
	originalBody := body
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	ctx = context.WithValue(ctx, postingCheckpointKey{}, s)
	s.postMu.Lock()
	s.postingCancel = cancel
	s.posting = PostingStatus{State: "posting", Results: []PostOutcome{}}
	s.postingStarted = time.Now()
	s.postingItemStarted = s.postingStarted
	unique := map[string]bool{}
	for _, id := range selected {
		unique[id] = true
	}
	s.posting.Total = len(unique)
	s.postingPaused = false
	s.postingLogs = nil
	s.postingRunID++
	s.postStatus = "Setting up automatic posting using your first Facebook post..."
	scheduledAt, scheduled := ctx.Value(scheduledPostingKey{}).(time.Time)
	if scheduled {
		s.posting.State = "scheduled"
		s.posting.ScheduledAt = scheduledAt.Format(time.RFC3339)
		s.postStatus = "Queue scheduled for " + s.posting.ScheduledAt + ". Keep Postika open until posting finishes."
	}
	s.signalPostingLocked()
	s.appendPostingLogLocked("info", s.postStatus)
	s.postMu.Unlock()
	defer func() {
		s.postMu.Lock()
		s.postingCancel = nil
		s.postingPaused = false
		s.posting.ElapsedSeconds = int(time.Since(s.postingStarted).Seconds())
		if !s.postingItemStarted.IsZero() {
			s.posting.ItemSeconds = int(time.Since(s.postingItemStarted).Seconds())
		}
		s.posting.WaitSeconds = 0
		s.signalPostingLocked()
		s.postMu.Unlock()
	}()
	finish := func(state, message string) *PostingStatus {
		s.postMu.Lock()
		s.posting.State = state
		s.postingPaused = false
		s.postStatus = message
		s.posting.ElapsedSeconds = int(time.Since(s.postingStarted).Seconds())
		if !s.postingItemStarted.IsZero() {
			s.posting.ItemSeconds = int(time.Since(s.postingItemStarted).Seconds())
		}
		s.posting.WaitSeconds = 0
		level := "info"
		if state == "stopped" && ctx.Err() == nil {
			level = "error"
		}
		s.appendPostingLogLocked(level, message)
		result := s.postingStatusLocked()
		s.postMu.Unlock()
		return &result
	}
	if scheduled {
		if err := s.waitScheduledPosting(ctx, scheduledAt); err != nil {
			return finish("stopped", "Scheduled posting cancelled."), nil
		}
	}
	// Previously submitted groups are accounted for without blocking other groups.
	// A prior uncertain submission has no retained request to retry safely here.
	s.mu.Lock()
	ownerBefore := ""
	if s.groupsLoad != nil {
		s.groupsLoad.mu.Lock()
		ownerBefore = s.groupsLoad.user
		s.groupsLoad.mu.Unlock()
	}
	priorFingerprint := ownerBefore + ":" + draftFingerprint(body, images) + ":"
	remaining := []string{}
	seenSelected := map[string]bool{}
	for _, id := range selected {
		if seenSelected[id] {
			continue
		}
		seenSelected[id] = true
		if previous, ok := s.postLedger[priorFingerprint+id]; ok {
			if previous.State == "unknown" {
				previous.stop = false
				previous.Message = "Earlier submission is unconfirmed. Skipping this group and continuing with the remaining groups."
			}
			s.recordOutcome(previous)
		} else {
			remaining = append(remaining, id)
		}
	}
	s.mu.Unlock()
	if len(remaining) == 0 && len(selected) > 0 {
		return finish("complete", "All selected groups were already attempted. Review unconfirmed results on Facebook."), nil
	}
	selected = remaining
	if err := postingCheckpoint(ctx); err != nil {
		return finish("stopped", "Posting stopped."), nil
	}
	if options.Rewrite {
		var rewriteErr error
		body, rewriteErr = s.rewriteForRun(ctx, originalBody, options, &settings)
		if rewriteErr != nil {
			return finish("stopped", rewriteErr.Error()), nil
		}
	}
	prepared, err := s.OpenPostComposer(ctx, selected, body, images)
	if err != nil {
		return finish("stopped", err.Error()), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	page := s.page.Context(ctx)
	if s.photoUploads == nil {
		return finish("stopped", "No fresh image upload request was captured."), nil
	}
	s.photoUploads.mu.Lock()
	var upload *capturedPhotoUpload
	for _, entry := range s.photoUploads.requests {
		if entry.wire && entry.finished {
			copy := *entry
			upload = &copy
			break
		}
	}
	s.photoUploads.mu.Unlock()
	if upload == nil {
		return finish("stopped", "No complete image upload request was captured."), nil
	}
	s.groupsLoad.mu.Lock()
	owner := s.groupsLoad.user
	s.groupsLoad.mu.Unlock()
	if owner == "" || owner != upload.User {
		return finish("stopped", "The current profile differs from the profile used to load groups. Load groups again."), nil
	}
	if s.postLedger == nil {
		s.postLedger = map[string]PostOutcome{}
	}
	fingerprint := upload.User + ":" + draftFingerprint(originalBody, images) + ":"
	for _, group := range prepared.Skipped {
		s.recordOutcome(PostOutcome{Group: group, State: "skipped", Message: "No available post composer."})
	}
	key := fingerprint + prepared.OpenedGroup.ID
	s.beginPostingGroup(prepared.OpenedGroup)
	if previous, ok := s.postLedger[key]; ok {
		s.recordOutcome(previous)
		return finish("stopped", "This draft has already been submitted to the first group. It was not submitted again."), nil
	}
	// Confirm every browser upload, not just a successful HTTP status.
	s.photoUploads.mu.Lock()
	for _, entry := range s.photoUploads.requests {
		if entry.User != owner {
			err = errors.New("Image uploads belong to a different profile")
			break
		}
		if entry.finished {
			if id, _ := parseUploadedPhoto(entry.Response, entry.Status); id == "" {
				err = errors.New("Facebook did not confirm all images. Check the composer.")
				break
			}
		}
	}
	s.photoUploads.mu.Unlock()
	if err != nil {
		return finish("stopped", err.Error()), nil
	}
	// Arm capture before waiting for the action so an enabled button can be
	// clicked immediately, without waiting for the entire dialog to settle.
	capture := startPostCapture(ctx, page, prepared.OpenedGroup.ID)
	defer capture.stop()
	if capture.startErr != nil {
		return finish("stopped", capture.startErr.Error()), nil
	}
	s.setPostStatus("Waiting for Facebook's Post button…")
	button, err := waitPostSubmit(ctx, page, true)
	if err != nil {
		return finish("stopped", err.Error()), nil
	}
	unknown := PostOutcome{Group: prepared.OpenedGroup, State: "unknown", Message: "This post was submitted, but confirmation is uncertain. Check Facebook before retrying.", stop: true}
	// Reserve before clicking; a connection loss after the click must never cause
	// an automatic second submission of this draft to this group.
	s.postLedger[key] = unknown
	if err := postingCheckpoint(ctx); err != nil {
		delete(s.postLedger, key)
		return finish("stopped", "Posting stopped."), nil
	}
	s.setPostStatus("Submitting the first post and capturing fresh request values…")
	if err = button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		s.recordOutcome(unknown)
		return finish("stopped", unknown.Message), nil
	}
	s.setPostStatus("Post button clicked")
	s.setPostStatus("Posting...")
	template, err := capture.wait(ctx, page, s.setPostStatus)
	if err != nil && (template == nil || ctx.Err() != nil) {
		unknown.Message = err.Error()
		s.postLedger[key] = unknown
		s.recordOutcome(unknown)
		return finish("stopped", unknown.Message), nil
	}
	captureErr := err
	cookies, err := s.browser.Context(ctx).GetCookies()
	if err != nil {
		return finish("stopped", "The session could not be saved for posting retries and remaining groups."), nil
	}
	replay, err := newPostReplay(cookies, *upload, *template)
	if err != nil {
		return finish("stopped", err.Error()), nil
	}
	replay.allowRewrite = options.Rewrite
	replay.progress = s.setPostStatus
	first := parsePostResponseForGroup(template.Response, template.Status, prepared.OpenedGroup.ID)
	if captureErr != nil {
		first = replayRequestFailure("unknown", "The first post could not be confirmed.", "Browser post", template.Response, template.Status, captureErr)
	}
	firstRetried := first.State == "unknown"
	if firstRetried {
		s.setPostStatus("First submission unconfirmed. Retrying request 1/3 with the captured mutation ID…")
		response, status, retryErr := replay.sendAttempts(ctx, template.URL, template.Headers, []byte(template.Body), 1, func(body []byte, status int) bool {
			return parsePostResponseForGroup(body, status, prepared.OpenedGroup.ID).State == "unknown"
		})
		if retryErr != nil {
			first = replayRequestFailure("unknown", "The first post could not be confirmed.", "Browser post retry", response, status, retryErr)
		} else {
			first = parsePostResponseForGroup(response, status, prepared.OpenedGroup.ID)
		}
	}
	first.Group = prepared.OpenedGroup
	s.recordOutcome(first)
	if first.State == "posted" || first.State == "pending" || first.State == "unknown" {
		s.postLedger[key] = first
	} else {
		delete(s.postLedger, key)
	}
	if first.stop {
		return finish("stopped", first.Message), nil
	}
	if !firstRetried && (first.State == "posted" || first.State == "pending") {
		s.setPostStatus("First post confirmed. Waiting for the Facebook composer to close…")
		if err = waitPostedComposerClosed(ctx, page); err != nil {
			return finish("stopped", err.Error()), nil
		}
	}
	if _, formatted := ctx.Value(richPostBodyKey{}).(richPostBody); formatted {
		body, err = capturedRichPostText(template.Body, body)
		if err != nil {
			return finish("stopped", err.Error()), nil
		}
	}
	// Validate body/serialization before any further uploads or publication.
	if _, err = buildReplayPost(template.Body, body, prepared.OpenedGroup.ID, []string{}); err != nil {
		return finish("stopped", err.Error()), nil
	}
	capture.stop()
	s.setPostStatus("Fresh upload and post request values captured from " + prepared.OpenedGroup.Name + ".")
	s.setPostStatus("Closing Chrome and posting to the remaining groups…")
	s.closeBrowserConnection()
	skipped := map[string]bool{prepared.OpenedGroup.ID: true}
	for _, group := range prepared.Skipped {
		skipped[group.ID] = true
	}
	for _, group := range prepared.Groups {
		if skipped[group.ID] {
			continue
		}
		if err := postingCheckpoint(ctx); err != nil {
			return finish("stopped", "Posting stopped."), nil
		}
		if ctx.Err() != nil {
			return finish("stopped", "Posting stopped."), nil
		}
		key = fingerprint + group.ID
		if previous, ok := s.postLedger[key]; ok {
			s.recordOutcome(previous)
			continue
		}
		s.beginPostingGroup(group)
		if err := s.waitPostingInterval(ctx, options); err != nil {
			return finish("stopped", "Posting stopped."), nil
		}
		postBody := body
		if options.Rewrite {
			postBody, err = s.rewriteForRun(ctx, originalBody, options, &settings)
			if err != nil {
				return finish("stopped", err.Error()), nil
			}
		}
		photos := []string{}
		failed := false
		s.setPostStatus("Preparing post 1/1 for " + group.Name + "...")
		s.setPostStatus("Uploading images from post 1/1 for " + group.Name + "...")
		for imageIndex, image := range images {
			if err := postingCheckpoint(ctx); err != nil {
				return finish("stopped", "Posting stopped."), nil
			}
			s.setPostStatus(fmt.Sprintf("%s: uploading image %d of %d…", group.Name, imageIndex+1, len(images)))
			id, outcome := replay.uploadImage(ctx, image)
			if id == "" {
				outcome.Group = group
				s.recordOutcome(outcome)
				if outcome.stop {
					return finish("stopped", outcome.Message), nil
				}
				failed = true
				break
			}
			photos = append(photos, id)
		}
		if failed {
			continue
		}
		s.setPostStatus("Posting reusable post 1/1 to " + group.Name + "...")
		if err := postingCheckpoint(ctx); err != nil {
			return finish("stopped", "Posting stopped."), nil
		}
		unknown.Group = group
		s.postLedger[key] = unknown
		outcome := replay.create(ctx, postBody, photos, group.ID)
		outcome.Group = group
		s.recordOutcome(outcome)
		if outcome.State == "posted" || outcome.State == "pending" || outcome.State == "unknown" {
			s.postLedger[key] = outcome
		} else {
			delete(s.postLedger, key)
		}
		if outcome.stop {
			return finish("stopped", outcome.Message), nil
		}
	}
	status := s.GetPostingStatus()
	posted, pending, failed := 0, 0, 0
	for _, outcome := range status.Results {
		switch outcome.State {
		case "posted":
			posted++
		case "pending":
			pending++
		case "failed", "skipped", "unknown":
			failed++
		}
	}
	return finish("complete", fmt.Sprintf("Finished: %d published, %d awaiting approval, %d failed or skipped.", posted, pending, failed)), nil
}
