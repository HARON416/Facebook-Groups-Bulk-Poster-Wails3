package main

import (
	"context"
	"net/http"
	"time"
)

type ConnectivityStatus struct {
	Known       bool `json:"known"`
	Online      bool `json:"online"`
	ManualPause bool `json:"manualPause"`
}

func (s *GroupsService) GetConnectivityStatus() ConnectivityStatus {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	status := s.connectivity
	status.ManualPause = s.postingPaused
	return status
}

func (s *GroupsService) connectionBlockedLocked() bool {
	return s.connectivityEnabled && (!s.connectivity.Known || !s.connectivity.Online)
}

// Probe outside the browser: the queue continues after Chrome has closed.
// HTTPS and disabled redirects prevent a captive portal from appearing online.
func internetReachable(ctx context.Context) bool {
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, endpoint := range []string{"https://www.gstatic.com/generate_204", "https://cp.cloudflare.com/generate_204"} {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		response, err := client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusNoContent {
				return true
			}
		}
		if ctx.Err() != nil {
			return false
		}
	}
	return false
}

func (s *GroupsService) startConnectivityMonitor() {
	ctx, cancel := s.operationContext(context.Background())
	s.postMu.Lock()
	s.connectivityEnabled = true
	s.connectivityProbe = internetReachable
	s.postMu.Unlock()
	go func() {
		defer cancel()
		for {
			s.checkConnectivity(ctx)
			timer := time.NewTimer(3 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (s *GroupsService) checkConnectivity(ctx context.Context) {
	s.connectivityCheckMu.Lock()
	defer s.connectivityCheckMu.Unlock()
	s.postMu.Lock()
	probe := s.connectivityProbe
	s.postMu.Unlock()
	if probe == nil || ctx.Err() != nil {
		return
	}
	online := probe(ctx)
	if ctx.Err() != nil {
		return
	}
	s.postMu.Lock()
	defer s.postMu.Unlock()
	previous := s.connectivity
	s.connectivity.Known, s.connectivity.Online = true, online
	if previous.Known && previous.Online == online {
		return
	}
	if s.postingActiveLocked() && s.posting.State != "scheduled" && s.posting.State != "stopping" {
		if !online {
			s.posting.State = "pausing"
			s.appendPostingLogLocked("warning", "Internet connection lost. Queue paused until the connection returns.")
		} else if previous.Known {
			message := "Internet connection restored. Continuing the queue."
			if s.postingPaused {
				message = "Internet connection restored. The queue remains manually paused."
			}
			s.appendPostingLogLocked("info", message)
		}
	}
	s.signalPostingLocked()
}

// A transport failure triggers an immediate check, before spending another
// confirmation retry. The caller retains the same request and mutation ID.
func recoverPostingConnection(ctx context.Context) (bool, error) {
	s, ok := ctx.Value(postingCheckpointKey{}).(*GroupsService)
	if !ok {
		return false, ctx.Err()
	}
	s.checkConnectivity(ctx)
	s.postMu.Lock()
	blocked := s.connectionBlockedLocked()
	s.postMu.Unlock()
	if !blocked {
		return false, ctx.Err()
	}
	return true, postingCheckpoint(ctx)
}
