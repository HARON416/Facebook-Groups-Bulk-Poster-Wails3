package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type AccountProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProfileActivity struct {
	AccountProfile
	Active bool   `json:"active"`
	State  string `json:"state"`
}

type ProfileList struct {
	Selected string            `json:"selected"`
	Profiles []ProfileActivity `json:"profiles"`
}

type profileRegistry struct {
	Version  int              `json:"version"`
	Selected string           `json:"selected"`
	Profiles []AccountProfile `json:"profiles"`
}

// ProfileService routes every operation by an explicit, stable profile ID.
// Switching the visible workspace never changes a session's storage or jobs.
type ProfileService struct {
	mu       sync.Mutex
	root     string
	registry profileRegistry
	sessions map[string]*GroupsService
	app      *application.App
	monitor  bool
	closed   bool
}

func newProfileService(root string) (*ProfileService, error) {
	if root == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(dir, "postika")
	}
	m := &ProfileService{root: root, sessions: map[string]*GroupsService{}}
	data, err := os.ReadFile(filepath.Join(root, "profiles.json"))
	if os.IsNotExist(err) {
		// Keep legacy paths in place: no copying live Chrome databases or logins.
		m.registry = profileRegistry{Version: 1, Selected: "default", Profiles: []AccountProfile{{ID: "default", Name: "Default profile"}}}
		if err := m.saveRegistry(m.registry); err != nil {
			return nil, err
		}
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read account profiles: %w", err)
	}
	if err := json.Unmarshal(data, &m.registry); err != nil {
		return nil, errors.New("Could not read account profiles; your existing data has been preserved")
	}
	validID := regexp.MustCompile(`^(default|[a-f0-9]{32})$`)
	seen := map[string]bool{}
	for _, profile := range m.registry.Profiles {
		if !validID.MatchString(profile.ID) || seen[profile.ID] || strings.TrimSpace(profile.Name) == "" {
			return nil, errors.New("Invalid account profile registry; your existing data has been preserved")
		}
		seen[profile.ID] = true
	}
	if m.registry.Version != 1 || !seen[m.registry.Selected] {
		return nil, errors.New("Unsupported or incomplete account profile registry")
	}
	return m, nil
}

func (m *ProfileService) saveRegistry(registry profileRegistry) error {
	if err := os.MkdirAll(m.root, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(registry)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(m.root, ".profiles-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(m.root, "profiles.json"))
}

func (m *ProfileService) session(id string) (*GroupsService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("Postika is shutting down")
	}
	for _, profile := range m.registry.Profiles {
		if profile.ID != id {
			continue
		}
		if s := m.sessions[id]; s != nil {
			return s, nil
		}
		dir := filepath.Join(m.root, "profiles", id)
		browser := filepath.Join(dir, "browser")
		if id == "default" {
			dir = m.root
			browser = filepath.Join(dir, "browser-profile")
		}
		s := &GroupsService{profileID: id, profileName: profile.Name, app: m.app,
			profileDir: browser, settingsPath: filepath.Join(dir, "settings.json"), groupsCachePath: filepath.Join(dir, "groups.json")}
		m.sessions[id] = s
		if m.monitor {
			s.startConnectivityMonitor()
		}
		return s, nil
	}
	return nil, errors.New("Account profile was not found")
}

func (m *ProfileService) GetProfiles() ProfileList {
	m.mu.Lock()
	result := ProfileList{Selected: m.registry.Selected, Profiles: []ProfileActivity{}}
	sessions := make([]*GroupsService, 0, len(m.registry.Profiles))
	for _, profile := range m.registry.Profiles {
		result.Profiles = append(result.Profiles, ProfileActivity{AccountProfile: profile})
		sessions = append(sessions, m.sessions[profile.ID])
	}
	m.mu.Unlock()
	for i, s := range sessions {
		if s != nil {
			status := s.GetPostingStatus()
			result.Profiles[i].Active = status.Active
			result.Profiles[i].State = status.State
		}
	}
	return result
}

func (m *ProfileService) SelectProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("Postika is shutting down")
	}
	for _, profile := range m.registry.Profiles {
		if profile.ID != id {
			continue
		}
		registry := m.registry
		registry.Selected = id
		if err := m.saveRegistry(registry); err != nil {
			return err
		}
		m.registry = registry
		return nil
	}
	return errors.New("Account profile was not found")
}

func (m *ProfileService) CreateProfile(name string) (AccountProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return AccountProfile{}, errors.New("Postika is shutting down")
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return AccountProfile{}, errors.New("Enter an account name of 1 to 80 characters")
	}
	for _, existing := range m.registry.Profiles {
		if strings.EqualFold(name, existing.Name) {
			return AccountProfile{}, errors.New("An account profile with this name already exists")
		}
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return AccountProfile{}, err
	}
	profile := AccountProfile{ID: hex.EncodeToString(random[:]), Name: name}
	registry := m.registry
	registry.Profiles = append(append([]AccountProfile{}, registry.Profiles...), profile)
	registry.Selected = profile.ID
	if err := m.saveRegistry(registry); err != nil {
		return AccountProfile{}, err
	}
	m.registry = registry
	return profile, nil
}

func (m *ProfileService) ServiceShutdown() error {
	m.mu.Lock()
	m.closed = true
	sessions := make([]*GroupsService, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	for _, s := range sessions {
		_ = s.ServiceShutdown()
	}
	return nil
}
