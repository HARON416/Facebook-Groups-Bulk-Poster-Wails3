package main

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ContactService opens a compose window and reports launcher failures instead
// of silently discarding the operating system's exit status.
type ContactService struct{}

func (*ContactService) OpenEmail(topic string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return openContactEmail(ctx, runtime.GOOS, topic, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	})
}

type contactCommand func(context.Context, string, ...string) ([]byte, error)

func openContactEmail(ctx context.Context, platform, topic string, run contactCommand) error {
	switch topic {
	case "Question", "Feedback", "Bug", "Account", "Business", "Other":
	default:
		return fmt.Errorf("unknown contact topic")
	}
	query := url.Values{"subject": {"[Postika] [" + topic + "] "}}
	uri := "mailto:haronkibetrutoh@gmail.com?" + strings.ReplaceAll(query.Encode(), "+", "%20")
	var name string
	var args []string
	switch platform {
	case "linux":
		// xdg-open may return success without opening anything when there is
		// no registered email application. Check the association first.
		handler, err := run(ctx, "xdg-mime", "query", "default", "x-scheme-handler/mailto")
		if err != nil || strings.TrimSpace(string(handler)) == "" {
			return fmt.Errorf("no default email application is configured")
		}
		name, args = "xdg-email", []string{uri}
	case "darwin":
		name, args = "open", []string{uri}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", uri}
	default:
		return fmt.Errorf("email launching is unsupported on this platform")
	}
	if _, err := run(ctx, name, args...); err != nil {
		return fmt.Errorf("could not open email application: %w", err)
	}
	return nil
}
