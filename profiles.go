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

var profileNameURLPattern = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.-]*://|www\.|(?:[a-z0-9-]+\.)*facebook\.com(?:[/?#]|$))`)

// ProfileService routes every operation by an explicit, stable profile ID.
// Switching the visible workspace never changes a session's storage or jobs.
type ProfileService struct {
	updateGate sync.RWMutex
	mu         sync.Mutex
	root       string
	registry   profileRegistry
	sessions   map[string]*GroupsService
	app        *application.App
	monitor    bool
	closed     bool
}

func newProfileService(root string) (*ProfileService, error) {
	if root == "" {
		dir, err := defaultProfileRoot()
		if err != nil {
			return nil, err
		}
		root = dir
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
	if m.registry.Version != 1 || (len(seen) == 0 && m.registry.Selected != "") || (len(seen) > 0 && !seen[m.registry.Selected]) {
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
		s.updateGate = &m.updateGate
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
	if err := validateProfileName(name); err != nil {
		return AccountProfile{}, err
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

func validateProfileName(name string) error {
	if name == "" || len([]rune(name)) > 80 {
		return errors.New("Enter an account name of 1 to 80 characters")
	}
	if profileNameURLPattern.MatchString(name) {
		return errors.New("Account names cannot be URLs")
	}
	return nil
}

func (m *ProfileService) RenameProfile(id, name string) (AccountProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return AccountProfile{}, errors.New("Postika is shutting down")
	}
	name = strings.TrimSpace(name)
	if err := validateProfileName(name); err != nil {
		return AccountProfile{}, err
	}
	index := -1
	for i, profile := range m.registry.Profiles {
		if profile.ID == id {
			index = i
		} else if strings.EqualFold(profile.Name, name) {
			return AccountProfile{}, errors.New("An account profile with this name already exists")
		}
	}
	if index < 0 {
		return AccountProfile{}, errors.New("Account profile was not found")
	}
	registry := m.registry
	registry.Profiles = append([]AccountProfile{}, registry.Profiles...)
	registry.Profiles[index].Name = name
	if err := m.saveRegistry(registry); err != nil {
		return AccountProfile{}, err
	}
	m.registry = registry
	if session := m.sessions[id]; session != nil {
		// Names are metadata: renaming must not wait on a browser/queue mutex.
		session.profileNameMu.Lock()
		session.profileName = name
		session.profileNameMu.Unlock()
	}
	return registry.Profiles[index], nil
}

// DeleteProfile removes only the selected account's storage. The legacy default
// shares the app root, so its paths must be enumerated rather than deleting root.
func (m *ProfileService) DeleteProfile(id string) error {
	if !m.updateGate.TryRLock() {
		return errors.New("Postika is restarting to install an update")
	}
	defer m.updateGate.RUnlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("Postika is shutting down")
	}
	registry := m.registry
	registry.Profiles = make([]AccountProfile, 0, len(m.registry.Profiles))
	found := false
	for _, profile := range m.registry.Profiles {
		if profile.ID == id {
			found = true
		} else {
			registry.Profiles = append(registry.Profiles, profile)
		}
	}
	if !found {
		return errors.New("Account profile was not found")
	}
	if registry.Selected == id {
		registry.Selected = ""
		if len(registry.Profiles) > 0 {
			registry.Selected = registry.Profiles[0].ID
		}
	}
	// Fail before touching any account data if the registry cannot be saved.
	if err := m.saveRegistry(registry); err != nil {
		return err
	}
	if s := m.sessions[id]; s != nil {
		s.CancelPosting()
		_ = s.shutdown(true)
		// Cancellation wakes scheduled/paused queues too. Wait for their finalizers
		// before dropping logs and preventing late settings writes from old callers.
		s.postRunMu.Lock()
		s.postRunMu.Unlock()
		s.settingsMu.Lock()
		s.settingsDeleted = true
		s.settingsMu.Unlock()
		s.postMu.Lock()
		s.posting = PostingStatus{}
		s.postingLogs = nil
		s.postLedger = nil
		s.postMu.Unlock()
		s.mu.Lock()
		s.facebookPageLoad = nil
		s.mu.Unlock()
		delete(m.sessions, id)
	}
	paths := []string{filepath.Join(m.root, "profiles", id)}
	if id == "default" {
		paths = append(paths, filepath.Join(m.root, "browser-profile"), filepath.Join(m.root, "groups.json"), filepath.Join(m.root, "settings.json"))
	}
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			// Keep the account addressable so the user can retry incomplete cleanup.
			if restoreErr := m.saveRegistry(m.registry); restoreErr != nil {
				m.registry = registry
				return fmt.Errorf("delete account data: %w (restore registry: %v)", err, restoreErr)
			}
			return fmt.Errorf("delete account data: %w", err)
		}
	}
	m.registry = registry
	return nil
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
