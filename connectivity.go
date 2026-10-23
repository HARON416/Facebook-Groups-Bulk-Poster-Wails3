package main

import (
	"context"
	"net"
	"net/http"
	"time"
)

type ConnectivityStatus struct {
	Known       bool `json:"known"`
	Online      bool `json:"online"`
	ManualPause bool `json:"manualPause"`
	Active      bool `json:"active"`
}

func (s *GroupsService) GetConnectivityStatus() ConnectivityStatus {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	status := s.connectivity
	status.ManualPause = s.postingPaused
	status.Active = s.postingActiveLocked() && s.posting.State != "scheduled" && s.posting.State != "stopping"
	return status
}

func (s *GroupsService) connectionBlockedLocked() bool {
	return s.connectivityEnabled && (!s.connectivity.Known || !s.connectivity.Online)
}

type connectivityEndpoint struct {
	url    string
	status int // Zero accepts any HTTP response from this authenticated HTTPS origin.
}

// Use independent HTTPS origins in parallel. Reaching Facebook (even a login,
// rate-limit or server-error response) proves connectivity; it is not an outage
// of the internet. Dedicated 204 probes still reject captive-portal HTML.
func internetReachable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// A network switch must not reuse a pooled connection from the old route.
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DisableKeepAlives:     true,
		DialContext:           (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return probeInternetReachable(ctx, client, []connectivityEndpoint{
		{"https://www.facebook.com/robots.txt", 0},
		{"https://www.gstatic.com/generate_204", http.StatusNoContent},
		{"https://cp.cloudflare.com/generate_204", http.StatusNoContent},
	})
}

func probeInternetReachable(ctx context.Context, client *http.Client, endpoints []connectivityEndpoint) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan bool, len(endpoints))
	for _, endpoint := range endpoints {
		go func(endpoint connectivityEndpoint) {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.url, nil)
			reachable := false
			if err == nil {
				request.Header.Set("Cache-Control", "no-cache")
				response, err := client.Do(request)
				if response != nil {
					response.Body.Close()
					reachable = err == nil && (response.StatusCode == endpoint.status || endpoint.status == 0 && response.StatusCode >= 200 && response.StatusCode <= 599)
				}
			}
			results <- reachable
		}(endpoint)
	}
	for range endpoints {
		select {
		case <-ctx.Done():
			return false
		case reachable := <-results:
			if reachable {
				return true
			}
		}
	}
	return false
}

// OS/browser connectivity events are hints only. Coalesce wakeups and let the
// long-lived monitor verify the new route rather than trusting navigator.onLine.
func (s *GroupsService) RefreshConnectivity() {
	s.postMu.Lock()
	wake := s.connectivityWake
	s.connectivityGeneration++
	s.postMu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

func (s *GroupsService) startConnectivityMonitor() {
	ctx, cancel := s.operationContext(context.Background())
	s.postMu.Lock()
	s.connectivityEnabled = true
	if s.connectivityProbe == nil {
		s.connectivityProbe = internetReachable
	}
	if s.connectivityWake == nil {
		s.connectivityWake = make(chan struct{}, 1)
	}
	wake := s.connectivityWake
	s.postMu.Unlock()
	go func() {
		defer cancel()
		for {
			s.checkConnectivitySample(ctx, true)
			timer := time.NewTimer(3 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-wake:
				timer.Stop()
			case <-timer.C:
			}
		}
	}()
}

func (s *GroupsService) checkConnectivity(ctx context.Context) {
	s.checkConnectivitySample(ctx, false)
}

func (s *GroupsService) checkConnectivitySample(ctx context.Context, confirmLoss bool) {
	s.connectivityCheckMu.Lock()
	defer s.connectivityCheckMu.Unlock()
	s.postMu.Lock()
	probe := s.connectivityProbe
	generation := s.connectivityGeneration
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
	if generation != s.connectivityGeneration {
		return
	}
	previous := s.connectivity
	if online {
		s.connectivityFailures = 0
	} else {
		s.connectivityFailures++
		// One slow probe during a route/DNS change must not flash an outage.
		// Real posting transport failures use checkConnectivity to pause immediately.
		if confirmLoss && previous.Known && previous.Online && s.connectivityFailures < 2 {
			return
		}
	}
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
