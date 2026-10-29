package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const marketplaceTestItemURL = "https://www.facebook.com/marketplace/create/item"
const marketplacePhotoInput = `:is([role="main"], [role="form"], main) input[type="file"][accept*="image/"]`
const marketplaceVideoInput = `:is([role="main"], [role="form"], main) input[type="file"][accept*="video/"]`

type MarketplaceTestMedia struct {
	Photos []PostUpload           `json:"photos"`
	Video  *PostUpload            `json:"video"`
	Fields *MarketplaceItemFields `json:"fields"`
}

func init() {
	application.RegisterBindingMethodID((*ProfileService).TestMarketplaceItemMedia, 4270654618)
}

func stageMarketplaceTestMedia(media MarketplaceTestMedia) (dir string, photos []string, video string, err error) {
	if len(media.Photos) == 0 && media.Video == nil {
		return "", nil, "", errors.New("Select at least one image or video")
	}
	if len(media.Photos) > 10 {
		return "", nil, "", errors.New("You may only select a maximum of 10 photos")
	}
	dir, photos, err = stagePostImages(media.Photos)
	if err != nil {
		return "", nil, "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	if media.Video != nil {
		var data []byte
		data, err = base64.StdEncoding.DecodeString(media.Video.Data)
		if err != nil || len(data) == 0 || !strings.HasPrefix(http.DetectContentType(data), "video/") {
			err = errors.New("Could not read the selected video")
			return
		}
		name := filepath.Base(media.Video.Name)
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = "video"
		}
		videoDir := filepath.Join(dir, "video")
		if err = os.Mkdir(videoDir, 0700); err != nil {
			return
		}
		video = filepath.Join(videoDir, name)
		err = os.WriteFile(video, data, 0600)
	}
	return
}

func (m *ProfileService) TestMarketplaceItemMedia(ctx context.Context, profileID string, media MarketplaceTestMedia) error {
	if err := validateMarketplaceItemFields(media.Fields); err != nil {
		return err
	}
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	return s.testMarketplaceItemMedia(ctx, marketplaceTestItemURL, media)
}

func (s *GroupsService) testMarketplaceItemMedia(ctx context.Context, target string, media MarketplaceTestMedia) error {
	if media.Fields != nil {
		if err := validateMarketplaceItemFields(media.Fields); err != nil {
			return err
		}
	}
	ctx, cancel := s.operationContext(ctx)
	defer cancel()
	s.mu.Lock()
	if s.closed || s.marketplaceTesting {
		s.mu.Unlock()
		return errors.New("Another Marketplace test is running or the account is closed")
	}
	s.marketplaceTesting = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.marketplaceTesting = false; s.mu.Unlock() }()
	dir, photos, video, err := stageMarketplaceTestMedia(media)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(dir)
		}
	}()
	page, err := s.prepareMarketplaceTestPage(ctx, target)
	if err != nil {
		return err
	}
	if err = waitForFacebookLogin(ctx, page, nil); err != nil {
		return err
	}
	info, err := page.Context(ctx).Info()
	if err != nil {
		return err
	}
	wanted, _ := url.Parse(target)
	current, _ := url.Parse(info.URL)
	if current == nil || wanted == nil || current.Host != wanted.Host || current.Path != wanted.Path {
		if err = page.Context(ctx).Navigate(target); err != nil {
			return err
		}
		if err = waitForFacebookLogin(ctx, page, nil); err != nil {
			return err
		}
	}
	if len(photos) > 0 {
		input, findErr := marketplaceMediaInput(ctx, page, marketplacePhotoInput)
		if findErr != nil {
			return findErr
		}
		if err = input.SetFiles(photos); err != nil {
			return err
		}
	}
	if video != "" {
		// React may replace the inputs after photo selection; find it afresh.
		input, findErr := marketplaceMediaInput(ctx, page, marketplaceVideoInput)
		if findErr != nil {
			return findErr
		}
		if err = input.SetFiles([]string{video}); err != nil {
			return err
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// Chrome may read selected files asynchronously. Retain them until the next
	// test or browser shutdown, instead of deleting them immediately after SetFiles.
	s.mu.Lock()
	oldDir := s.marketplaceTestDir
	if s.closed {
		s.mu.Unlock()
		return errors.New("Postika is shutting down")
	}
	s.marketplaceTestDir = dir
	keep = true
	s.mu.Unlock()
	if oldDir != "" {
		_ = os.RemoveAll(oldDir)
	}
	if media.Fields != nil {
		return fillMarketplaceItemFields(ctx, page, *media.Fields)
	}
	return nil
}

func (s *GroupsService) prepareMarketplaceTestPage(ctx context.Context, target string) (*rod.Page, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("Postika is shutting down")
	}
	if err := s.ensureBrowser(ctx); err != nil {
		return nil, err
	}
	created := false
	if s.marketplaceTestPage != nil {
		if _, err := (proto.TargetGetTargetInfo{TargetID: s.marketplaceTestPage.TargetID}).Call(s.browser.Context(ctx)); err != nil {
			s.marketplaceTestPage = nil
		}
	}
	if s.marketplaceTestPage == nil {
		targetInfo, err := (proto.TargetCreateTarget{URL: target}).Call(s.browser.Context(ctx))
		if err != nil {
			return nil, err
		}
		page, err := s.browser.PageFromTarget(targetInfo.TargetID)
		if err != nil {
			_, _ = (proto.TargetCloseTarget{TargetID: targetInfo.TargetID}).Call(s.browser.Context(ctx))
			return nil, err
		}
		s.marketplaceTestPage = page
		created = true
	}
	page := s.marketplaceTestPage
	if !created {
		if err := page.Context(ctx).Navigate(target); err != nil {
			return nil, err
		}
	}
	if !s.headless {
		if err := page.Context(ctx).SetWindow(&proto.BrowserBounds{WindowState: proto.BrowserWindowStateMaximized}); err != nil {
			return nil, err
		}
	}
	if _, err := page.Context(ctx).Activate(); err != nil {
		return nil, err
	}
	return page, nil
}

func marketplaceMediaInput(ctx context.Context, page *rod.Page, selector string) (*rod.Element, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := (proto.TargetGetTargetInfo{TargetID: page.TargetID}).Call(page.Browser().Context(ctx)); err != nil {
			return nil, errors.New("Facebook was closed before media could be added")
		}
		inputs, err := page.Context(ctx).Elements(selector)
		if err == nil {
			if len(inputs) == 1 {
				return inputs[0], nil
			}
			if len(inputs) > 1 {
				return nil, errors.New("Multiple Marketplace upload controls were found")
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
