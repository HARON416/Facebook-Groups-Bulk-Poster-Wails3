package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestContactEmailCommands(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		for _, topic := range []string{"Question", "Feedback", "Bug", "Account", "Business", "Other"} {
			t.Run(platform+"/"+topic, func(t *testing.T) {
				launched := false
				err := openContactEmail(context.Background(), platform, topic, func(_ context.Context, name string, args ...string) ([]byte, error) {
					if name == "xdg-mime" {
						return []byte("mail-client.desktop\n"), nil
					}
					want := map[string]string{"linux": "xdg-email", "darwin": "open", "windows": "rundll32"}[platform]
					if name != want {
						t.Fatalf("launcher %q, want %q", name, want)
					}
					uri, err := url.Parse(args[len(args)-1])
					if err != nil {
						t.Fatal(err)
					}
					if uri.Scheme != "mailto" || uri.Opaque != "haronkibetrutoh@gmail.com" || uri.Query().Get("subject") != "[Postika] ["+topic+"] " {
						t.Fatalf("wrong compose URL: %s", uri)
					}
					if strings.Contains(uri.RawQuery, "+") {
						t.Fatal("mailto must encode spaces as %20")
					}
					launched = true
					return nil, nil
				})
				if err != nil || !launched {
					t.Fatalf("launched=%v, error=%v", launched, err)
				}
			})
		}
	}
}

func TestContactEmailReportsFailures(t *testing.T) {
	for _, handler := range []string{"", "mail-client.desktop"} {
		t.Run(handler, func(t *testing.T) {
			err := openContactEmail(context.Background(), "linux", "Question", func(_ context.Context, name string, args ...string) ([]byte, error) {
				if name == "xdg-mime" {
					return []byte(handler), nil
				}
				return nil, errors.New("launcher failed")
			})
			if err == nil {
				t.Fatal("expected missing client or launcher error")
			}
		})
	}
	err := openContactEmail(context.Background(), "linux", "unrecognized", func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("invalid topic executed a command")
		return nil, nil
	})
	if err == nil {
		t.Fatal("invalid topic accepted")
	}
}
