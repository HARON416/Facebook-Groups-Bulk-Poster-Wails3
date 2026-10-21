package main

import (
	"context"
	"errors"
	"time"
)

type scheduledPostingKey struct{}

func parsePostingSchedule(value string, now time.Time) (time.Time, error) {
	at, err := time.Parse(time.RFC3339, value)
	if err != nil || !at.After(now) {
		return time.Time{}, errors.New("Choose a future date and time for your post")
	}
	return at, nil
}

// Scheduling uses the same run lock, draft snapshot, stop control and posting
// pipeline as Post now. No browser or network work starts before the due time.
func (s *GroupsService) ScheduleRichPostWithOptions(ctx context.Context, selected []string, body, bodyHTML string, images []PostUpload, options PostingOptions, scheduledAt string) (*PostingStatus, error) {
	at, err := parsePostingSchedule(scheduledAt, time.Now())
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, scheduledPostingKey{}, at)
	return s.PublishRichPostWithOptions(ctx, selected, body, bodyHTML, images, options)
}

func (s *GroupsService) waitScheduledPosting(ctx context.Context, at time.Time) error {
	timer := time.NewTimer(time.Until(at))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	s.postMu.Lock()
	if ctx.Err() != nil {
		s.postMu.Unlock()
		return ctx.Err()
	}
	s.posting.State = "posting"
	s.postStatus = "Scheduled time reached. Starting posting…"
	s.appendPostingLogLocked("info", s.postStatus)
	s.signalPostingLocked()
	s.postMu.Unlock()
	return postingCheckpoint(ctx)
}
