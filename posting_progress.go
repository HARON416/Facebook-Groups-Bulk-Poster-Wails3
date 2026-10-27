package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

func (s *GroupsService) beginPostingGroup(group Group) {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	if s.posting.Current != 0 || s.postingItemStarted.IsZero() {
		s.postingItemStarted = time.Now()
	}
	s.posting.Current = len(s.posting.Results) + 1
	s.posting.CurrentGroup = group
	s.postStatus = fmt.Sprintf("Group %d/%d: %s", s.posting.Current, s.posting.Total, group.Name)
}
func (s *GroupsService) waitPostingInterval(ctx context.Context, options PostingOptions) error {
	span := options.MaximumInterval - options.MinimumInterval + 1
	number, err := rand.Int(rand.Reader, big.NewInt(int64(span)))
	if err != nil {
		return err
	}
	return s.waitPostingDelay(ctx, time.Duration(options.MinimumInterval+int(number.Int64()))*time.Second)
}
func (s *GroupsService) waitPostingDelay(ctx context.Context, duration time.Duration) error {
	remaining := duration
	defer func() { s.postMu.Lock(); s.posting.WaitSeconds = 0; s.postMu.Unlock() }()
	for remaining > 0 {
		if err := postingCheckpoint(ctx); err != nil {
			return err
		}
		s.postMu.Lock()
		s.posting.WaitSeconds = int((remaining + time.Second - 1) / time.Second)
		s.postStatus = fmt.Sprintf("Next post in %02d:%02d · %s", s.posting.WaitSeconds/60, s.posting.WaitSeconds%60, s.posting.CurrentGroup.Name)
		s.postMu.Unlock()
		step := 100 * time.Millisecond
		if remaining < step {
			step = remaining
		}
		timer := time.NewTimer(step)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		s.postMu.Lock()
		paused := s.postingPaused || s.connectionBlockedLocked()
		s.postMu.Unlock()
		if !paused {
			remaining -= step
		}
	}
	return postingCheckpoint(ctx)
}
