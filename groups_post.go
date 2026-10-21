package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Use the supplied composer classes, never its translated label. The full class
// list includes layout utilities shared with unrelated controls.
const groupComposerSelector = `div[role="button"][tabindex="0"].x1i10hfl.x1ejq31n.x18oe1m7.x1sy0etr.xstzfhl:has(> div.xi81zsa.x1lkfr7t.xkjl1po.x1mzt3pk.xh8yej3.x13faqbe > span.x1lliihq.x6ikm8r.x10wlt62.x1n2onr6):has(> div[role="none"][data-visualcompletion="ignore"])`

var errGroupComposerUnavailable = errors.New("This group has no available post composer")

type PostPreparation struct {
	Groups         []Group `json:"groups"`
	OpenedGroup    Group   `json:"openedGroup"`
	UploadRequests int     `json:"uploadRequests"`
	Skipped        []Group `json:"skipped"`
}

// OpenPostComposer tries the shuffled queue until it finds a composer, then fills
// that draft without submitting. Login, navigation and ambiguous controls stop it.
func (s *GroupsService) OpenPostComposer(ctx context.Context, selectedIDs []string, body string, images []PostUpload) (*PostPreparation, error) {
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	s.setPostStatus("Opening browser…")
	s.mu.Lock()
	s.restoreGroupsLocked()
	defer s.mu.Unlock()
	s.photoUploads = nil
	if s.closed {
		return nil, errors.New("Postika is shutting down")
	}
	if s.groupsLoad == nil {
		return nil, errors.New("Load your groups before opening a post")
	}
	s.groupsLoad.mu.Lock()
	loaded := append([]Group{}, s.groupsLoad.status.Groups...)
	s.groupsLoad.mu.Unlock()
	byID := make(map[string]Group, len(loaded))
	for _, group := range loaded {
		byID[group.ID] = group
	}
	selected := make([]Group, 0, len(selectedIDs))
	seen := map[string]bool{}
	for _, id := range selectedIDs {
		group, ok := byID[id]
		if !ok {
			return nil, errors.New("A selected group is no longer available. Load groups again")
		}
		if !seen[id] {
			selected = append(selected, group)
			seen[id] = true
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("Select at least one group")
	}
	rand.Shuffle(len(selected), func(i, j int) { selected[i], selected[j] = selected[j], selected[i] })

	if err := postingCheckpoint(ctx); err != nil {
		return nil, err
	}
	err := s.ensureBrowserPage(ctx)
	if err != nil {
		return nil, err
	}
	skipped := []Group{}
	for _, group := range selected {
		if err := postingCheckpoint(ctx); err != nil {
			return nil, err
		}
		s.setPostStatus("Opening " + group.Name + "…")
		target, err := facebookGroupURL(group)
		if err != nil {
			return nil, err
		}
		page := s.page.Context(ctx)
		err = page.Navigate(target)
		if err == nil {
			_, err = page.Activate()
		}
		if err != nil {
			return nil, fmt.Errorf("open selected group: %w", err)
		}
		s.setPostStatus("Waiting for the group page and opening its composer…")
		err = clickGroupComposer(ctx, page)
		if errors.Is(err, errGroupComposerUnavailable) && ctx.Err() == nil {
			skipped = append(skipped, group)
			continue
		}
		if err != nil {
			return nil, err
		}
		s.photoUploads = nil
		var uploads *photoUploadCapture
		if len(images) > 0 {
			if s.photoUploadWatcher != nil {
				uploads = s.photoUploadWatcher(ctx, page)
			} else {
				uploads = startPhotoUploadCapture(ctx, page, isFacebookPhotoUploadURL)
			}
			s.photoUploads = uploads
		}
		err = fillGroupComposer(ctx, page, body, images, &s.postImageDir, s.setPostStatus)
		if uploads != nil {
			count := 0
			if err == nil {
				count, err = uploads.wait(ctx, len(images), s.setPostStatus)
			}
			uploads.stop()
			if err != nil {
				return nil, err
			}
			return &PostPreparation{Groups: selected, OpenedGroup: group, Skipped: skipped, UploadRequests: count}, nil
		}
		if err != nil {
			return nil, err
		}
		return &PostPreparation{Groups: selected, OpenedGroup: group, Skipped: skipped}, nil
	}
	return nil, fmt.Errorf("None of the %d selected groups has an available post composer", len(selected))

}

func facebookGroupURL(group Group) (string, error) {
	u, err := url.Parse(group.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		(u.Hostname() != "web.facebook.com" && u.Hostname() != "www.facebook.com" && u.Hostname() != "facebook.com") ||
		!strings.HasPrefix(u.Path, "/groups/") || strings.Trim(u.Path[len("/groups/"):], "/") == "" ||
		strings.Trim(u.Path, "/") == "groups/joins" {
		return "", errors.New("The selected group has an invalid Facebook URL. Load groups again")
	}
	return u.String(), nil
}

func clickGroupComposer(ctx context.Context, page *rod.Page) error {
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("wait for group page: %w", err)
	}
	if err := page.WaitDOMStable(time.Second, 0); err != nil {
		return fmt.Errorf("wait for group DOM: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	login, err := page.Eval(facebookLoginSelectorScript)
	if err != nil {
		return fmt.Errorf("inspect group login: %w", err)
	}
	if login.Value.Str() == "logged_out" || login.Value.Str() == "login_required" {
		return errors.New("Log in or complete Facebook verification in Chrome, then click Post again")
	}
	if err := postingCheckpoint(ctx); err != nil {
		return err
	}
	elements, err := page.Elements(groupComposerSelector)
	if err != nil {
		return fmt.Errorf("inspect group composer: %w", err)
	}
	var visible []*rod.Element
	for _, element := range elements {
		shown, err := element.Visible()
		if err != nil {
			return fmt.Errorf("inspect composer visibility: %w", err)
		}
		if shown {
			visible = append(visible, element)
		}
	}
	if len(visible) == 0 {
		return errGroupComposerUnavailable
	}
	if len(visible) > 1 {
		return errors.New("Found multiple matching post composers on the group page")
	}
	disabled, err := visible[0].Attribute("aria-disabled")
	if err != nil {
		return fmt.Errorf("inspect composer state: %w", err)
	}
	if disabled != nil && *disabled == "true" {
		return errGroupComposerUnavailable
	}
	if err := visible[0].Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("open group post composer: %w", err)
	}
	return nil
}

// Status has its own mutex so it can be polled while browser work holds mu.
func (s *GroupsService) GetPostPreparationStatus() string {
	s.postMu.Lock()
	defer s.postMu.Unlock()
	return s.postStatus
}
func (s *GroupsService) setPostStatus(message string) {
	s.postMu.Lock()
	if s.postingCancel != nil && s.postStatus != message {
		s.appendPostingLogLocked("info", message)
	}
	s.postStatus = message
	s.postMu.Unlock()
}
