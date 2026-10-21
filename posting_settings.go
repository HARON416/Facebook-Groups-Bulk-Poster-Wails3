package main

import (
	"context"
	"encoding/json"
	"errors"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type PostingOptions struct {
	MinimumInterval int    `json:"minimumInterval"`
	MaximumInterval int    `json:"maximumInterval"`
	Rewrite         bool   `json:"rewrite"`
	Instructions    string `json:"instructions"`
}
type AppSettings struct {
	PostingOptions
	GeminiModel   string `json:"geminiModel"`
	KeyConfigured bool   `json:"keyConfigured"`
}
type storedSettings struct {
	AppSettings
	APIKey string `json:"apiKey"`
}

func defaultSettings() AppSettings {
	return AppSettings{PostingOptions: PostingOptions{MinimumInterval: 60, MaximumInterval: 120}, GeminiModel: "gemini-flash-latest"}
}
func validatePostingOptions(options PostingOptions) error {
	if options.MinimumInterval < 60 || options.MaximumInterval < options.MinimumInterval || options.MaximumInterval > 86400 {
		return errors.New("Set an interval between 60 and 86400 seconds, with the maximum at least the minimum")
	}
	if len(options.Instructions) > 20000 {
		return errors.New("Rewrite instructions are too long")
	}
	return nil
}
func (s *GroupsService) settingsFile() (string, error) {
	if s.settingsPath != "" {
		return s.settingsPath, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "postika", "settings.json"), nil
}

// Called with settingsMu held. The API key never appears in returned UI settings.
func (s *GroupsService) readSettings() (storedSettings, error) {
	result := storedSettings{AppSettings: defaultSettings()}
	path, err := s.settingsFile()
	if err != nil {
		return result, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, errors.New("Could not read Postika settings")
	}
	if json.Unmarshal(data, &result) != nil {
		return result, errors.New("Could not read Postika settings")
	}
	result.KeyConfigured = result.APIKey != ""
	return result, nil
}
func (s *GroupsService) GetSettings() (AppSettings, error) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	settings, err := s.readSettings()
	return settings.AppSettings, err
}
func (s *GroupsService) SaveSettings(settings AppSettings, key string, clearKey bool) error {
	if err := validatePostingOptions(settings.PostingOptions); err != nil {
		return err
	}
	settings.GeminiModel = strings.TrimSpace(settings.GeminiModel)
	if !regexp.MustCompile(`^[a-zA-Z0-9._-]+$`).MatchString(settings.GeminiModel) {
		return errors.New("Enter a valid Gemini model name")
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	stored, err := s.readSettings()
	if err != nil {
		return err
	}
	if clearKey {
		stored.APIKey = ""
	} else if strings.TrimSpace(key) != "" {
		stored.APIKey = strings.TrimSpace(key)
	}
	stored.AppSettings = settings
	stored.KeyConfigured = stored.APIKey != ""
	path, err := s.settingsFile()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("Could not save Postika settings")
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "settings-*.json")
	if err != nil {
		return errors.New("Could not save Postika settings")
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err != nil {
		return errors.New("Could not save Postika settings")
	}
	return nil
}
func (s *GroupsService) OpenSettings() error {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	if s.app == nil {
		return errors.New("The desktop application is not ready")
	}
	name := "postika-settings"
	url := "/?view=settings"
	title := "Postika · Settings"
	if s.profileID != "" {
		name += "-" + s.profileID
		url += "&profile=" + s.profileID + "&account=" + urlpkg.QueryEscape(s.profileName)
		title += " · " + s.profileName
	}
	if window, ok := s.app.Window.GetByName(name); ok {
		showAuxiliaryWindow(window)
		return nil
	}
	window := s.app.Window.NewWithOptions(application.WebviewWindowOptions{Name: name, Title: title, Frameless: true, Width: 760, Height: 760, MinWidth: 600, MinHeight: 480, URL: url, BackgroundColour: application.NewRGB(248, 249, 250)})
	retainAuxiliaryWindow(s.app, window)
	return nil
}

// Keep the existing entry point available to integrations using saved defaults.
func (s *GroupsService) PublishPost(ctx context.Context, selected []string, body string, images []PostUpload) (*PostingStatus, error) {
	settings, err := s.GetSettings()
	if err != nil {
		return nil, err
	}
	return s.PublishPostWithOptions(ctx, selected, body, images, settings.PostingOptions)
}
