package main

import (
	"context"
	"encoding/json"
	"errors"
	urlpkg "net/url"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type PostingLog struct {
	ID          int    `json:"id"`
	Level       string `json:"level"`
	Outcome     string `json:"outcome,omitempty"`
	Timestamp   string `json:"timestamp"`
	Message     string `json:"message"`
	HasResponse bool   `json:"hasResponse"`
	Diagnostic  string `json:"diagnostic,omitempty"`
	response    string
}
type PostingLogsPage struct {
	RunID    uint64        `json:"runId"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Total    int           `json:"total"`
	Entries  []PostingLog  `json:"entries"`
	Status   PostingStatus `json:"status"`
}

// Raw response bodies stay in Go and are copied only on an explicit click.
// Polling the logs never serializes request headers, cookies or response bodies.
func (s *GroupsService) appendPostingLogLocked(level, message string) {
	s.postingLogs = append(s.postingLogs, PostingLog{ID: len(s.postingLogs) + 1, Level: level, Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Message: message})
}

func (s *GroupsService) postingResponse(runID uint64, logID int) (string, error) {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	if runID != s.postingRunID || logID < 1 || logID > len(s.postingLogs) {
		return "", errors.New("This response is no longer available for the current run")
	}
	response := s.postingLogs[logID-1].response
	if response == "" {
		return "", errors.New("No Facebook response was captured for this entry")
	}
	// Pretty-print complete JSON streams; preserve non-JSON bodies for diagnosis.
	objects, valid := facebookObjects([]byte(response))
	if valid {
		var formatted []string
		for _, object := range objects {
			data, err := json.MarshalIndent(object, "", "  ")
			if err != nil {
				return response, nil
			}
			formatted = append(formatted, string(data))
		}
		return strings.Join(formatted, "\n"), nil
	}
	return response, nil
}

func (s *GroupsService) CopyPostingResponse(runID uint64, logID int) error {
	response, err := s.postingResponse(runID, logID)
	if err != nil {
		return err
	}
	if s.app == nil {
		return errors.New("The desktop clipboard is not ready")
	}
	if !s.app.Clipboard.SetText(response) {
		return errors.New("Could not copy the Facebook response to the clipboard")
	}
	return nil
}
func (s *GroupsService) postingStatusLocked() PostingStatus {
	status := s.posting
	status.Message = s.postStatus
	status.Active = s.postingActiveLocked()
	status.Offline = s.connectivityEnabled && (!s.connectivity.Known || !s.connectivity.Online)
	status.PauseRequested = s.postingPaused || status.Offline
	status.Results = append([]PostOutcome{}, status.Results...)
	status.Processed = len(status.Results)
	for _, result := range status.Results {
		switch result.State {
		case "posted":
			status.Posted++
		case "pending":
			status.Pending++
		case "failed", "skipped", "unknown":
			status.Failed++
		}
	}
	status.Remaining = status.Total - status.Processed
	if status.Remaining < 0 {
		status.Remaining = 0
	}
	if status.Active && !s.postingStarted.IsZero() {
		status.ElapsedSeconds = int(time.Since(s.postingStarted).Seconds())
	}
	if status.Active && !s.postingItemStarted.IsZero() {
		status.ItemSeconds = int(time.Since(s.postingItemStarted).Seconds())
	}
	return status
}
func (s *GroupsService) postingActiveLocked() bool {
	return s.postingCancel != nil && (s.posting.State == "scheduled" || s.posting.State == "posting" || s.posting.State == "pausing" || s.posting.State == "paused" || s.posting.State == "stopping")
}
func (s *GroupsService) GetPostingLogs(page, pageSize int) PostingLogsPage {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	total := len(s.postingLogs)
	last := (total + pageSize - 1) / pageSize
	if last < 1 {
		last = 1
	}
	if page < 1 {
		page = 1
	}
	if page > last {
		page = last
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	return PostingLogsPage{RunID: s.postingRunID, Page: page, PageSize: pageSize, Total: total, Entries: append([]PostingLog{}, s.postingLogs[start:end]...), Status: s.postingStatusLocked()}
}

// All accounts share one retained viewer; serialize creation across sessions.
var postingLogsWindowMu sync.Mutex

func (s *GroupsService) OpenPostingLogs() error {
	postingLogsWindowMu.Lock()
	defer postingLogsWindowMu.Unlock()
	if s.app == nil {
		return errors.New("The desktop application is not ready")
	}
	name := "postika-posting-logs"
	url := "/?view=posting-logs"
	title := "Postika · Posting logs"
	if s.profileID != "" {
		url += "&profile=" + s.profileID + "&account=" + urlpkg.QueryEscape(s.profileName)
		title += " · " + s.profileName
	}
	if window, ok := s.app.Window.GetByName(name); ok {
		profileID, _ := json.Marshal(s.profileID)
		profileName, _ := json.Marshal(s.profileName)
		window.ExecJS(`(() => { const url = new URL(window.location.href); url.searchParams.set('profile', ` + string(profileID) + ` || 'default'); url.searchParams.set('account', ` + string(profileName) + ` || 'Default profile'); window.history.replaceState(null, '', url); window.dispatchEvent(new Event('postika:logs-profile')); })()`)
		showAuxiliaryWindow(window)
		return nil
	}
	window := s.app.Window.NewWithOptions(application.WebviewWindowOptions{Name: name, Title: title, Frameless: true, Width: 1100, Height: 750, MinWidth: 760, MinHeight: 480, URL: url, BackgroundColour: application.NewRGB(248, 249, 250)})
	retainAuxiliaryWindow(s.app, window)
	return nil
}
func (s *GroupsService) signalPostingLocked() {
	if s.postingChanged != nil {
		close(s.postingChanged)
	}
	s.postingChanged = make(chan struct{})
}
func (s *GroupsService) PausePosting() {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	if !s.postingActiveLocked() || s.posting.State == "scheduled" || s.posting.State == "stopping" || s.postingPaused {
		return
	}
	s.postingPaused = true
	s.posting.State = "pausing"
	s.appendPostingLogLocked("info", "Pause requested. The current operation will finish before pausing.")
	s.signalPostingLocked()
}
func (s *GroupsService) ResumePosting() {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	if !s.postingActiveLocked() || !s.postingPaused || s.posting.State == "stopping" {
		return
	}
	s.postingPaused = false
	if !s.connectionBlockedLocked() {
		s.posting.State = "posting"
	}
	s.appendPostingLogLocked("info", "Posting resumed.")
	s.signalPostingLocked()
}

type postingCheckpointKey struct{}

func postingCheckpoint(ctx context.Context) error {
	if service, ok := ctx.Value(postingCheckpointKey{}).(*GroupsService); ok {
		return service.waitPosting(ctx)
	}
	return ctx.Err()
}

// Pause is cooperative: an in-flight request is allowed to finish so a post's
// result remains known. Stop cancels both waiting and in-flight operations.
func (s *GroupsService) waitPosting(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.postMu.Lock()
		if !s.postingPaused && !s.connectionBlockedLocked() {
			if s.posting.State == "paused" {
				s.posting.State = "posting"
			}
			s.postMu.Unlock()
			return ctx.Err()
		}
		if s.posting.State != "paused" {
			s.posting.State = "paused"
			s.appendPostingLogLocked("info", "Posting paused.")
		}
		changed := s.postingChanged
		s.postMu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}
