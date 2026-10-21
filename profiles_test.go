package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestProfilesPreserveLegacyDataAndIsolateStorage(t *testing.T) {
	root := t.TempDir()
	legacy := &GroupsService{settingsPath: filepath.Join(root, "settings.json"), groupsCachePath: filepath.Join(root, "groups.json")}
	settings := defaultSettings()
	settings.Instructions = "Personal voice"
	if err := legacy.SaveSettings(settings, "personal-key", false); err != nil {
		t.Fatal(err)
	}
	groups := []Group{{ID: "personal-group", Name: "Personal group"}}
	if err := legacy.saveGroups("personal-user", groups); err != nil {
		t.Fatal(err)
	}
	browser := filepath.Join(root, "browser-profile")
	if err := os.MkdirAll(browser, 0700); err != nil {
		t.Fatal(err)
	}
	cookies := filepath.Join(browser, "Cookies")
	if err := os.WriteFile(cookies, []byte("legacy login fixture"), 0600); err != nil {
		t.Fatal(err)
	}

	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	personal, err := m.session("default")
	if err != nil {
		t.Fatal(err)
	}
	if personal.profileDir != browser || personal.settingsPath != legacy.settingsPath || personal.groupsCachePath != legacy.groupsCachePath {
		t.Fatal("legacy paths changed")
	}
	if got := personal.GetGroupsLoadStatus(); !reflect.DeepEqual(got.Groups, groups) {
		t.Fatalf("legacy groups lost: %+v", got)
	}
	got, err := m.GetSettings("default")
	if err != nil || got.Instructions != "Personal voice" || !got.KeyConfigured {
		t.Fatalf("legacy settings lost: %+v, %v", got, err)
	}

	profile, err := m.CreateProfile("Business")
	if err != nil {
		t.Fatal(err)
	}
	business, err := m.session(profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	if business == personal || business.profileDir == personal.profileDir || business.settingsPath == personal.settingsPath || business.groupsCachePath == personal.groupsCachePath {
		t.Fatal("profiles share storage or service state")
	}
	if filepath.Dir(business.profileDir) != filepath.Join(root, "profiles", profile.ID) {
		t.Fatal("browser profile escaped account directory")
	}
	got, err = m.GetSettings(profile.ID)
	if err != nil || got.KeyConfigured || got.Instructions != "" {
		t.Fatal("new profile inherited credentials or preferences")
	}
	if got, _ := m.GetGroupsLoadStatus(profile.ID); len(got.Groups) != 0 {
		t.Fatal("new profile inherited groups")
	}
	settings.Instructions = "Business voice"
	if err := m.SaveSettings(profile.ID, settings, "business-key", false); err != nil {
		t.Fatal(err)
	}
	if err := business.saveGroups("business-user", []Group{{ID: "business-group"}}); err != nil {
		t.Fatal(err)
	}

	restored, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.ServiceShutdown()
	if list := restored.GetProfiles(); list.Selected != profile.ID || len(list.Profiles) != 2 {
		t.Fatalf("registry not restored: %+v", list)
	}
	for _, entry := range []struct{ id, instruction, key, owner, group string }{
		{"default", "Personal voice", "personal-key", "personal-user", "personal-group"},
		{profile.ID, "Business voice", "business-key", "business-user", "business-group"},
	} {
		s, _ := restored.session(entry.id)
		saved, err := s.readSettings()
		if err != nil || saved.Instructions != entry.instruction || saved.APIKey != entry.key {
			t.Fatalf("settings crossed accounts: %+v, %v", saved, err)
		}
		status := s.GetGroupsLoadStatus()
		if len(status.Groups) != 1 || status.Groups[0].ID != entry.group || s.groupsLoad.user != entry.owner {
			t.Fatalf("groups crossed accounts: %+v", status)
		}
	}
	if data, _ := os.ReadFile(cookies); string(data) != "legacy login fixture" {
		t.Fatal("migration modified browser login")
	}
	info, err := os.Stat(filepath.Join(root, "profiles.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("profile registry is not private")
	}
}

func TestProfileJobsRemainBoundAfterSwitchAndControlsAreIsolated(t *testing.T) {
	m, err := newProfileService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	profile, err := m.CreateProfile("Business")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.session("default")
	b, _ := m.session(profile.ID)
	doneA, doneB := make(chan error, 1), make(chan error, 1)
	schedule := func(id string, done chan<- error) {
		result, err := m.ScheduleRichPostWithOptions(context.Background(), id, []string{"group"}, "Draft", "", []PostUpload{{Name: "image.png", Data: fixturePNG}}, defaultSettings().PostingOptions, time.Now().Add(time.Hour).Format(time.RFC3339))
		if err == nil && result.State != "stopped" {
			t.Errorf("unexpected result: %+v", result)
		}
		done <- err
	}
	go schedule("default", doneA)
	go schedule(profile.ID, doneB)
	awaitPostingState(t, a, "scheduled")
	awaitPostingState(t, b, "scheduled")
	if err := m.SelectProfile("default"); err != nil {
		t.Fatal(err)
	}
	if err := m.SelectProfile(profile.ID); err != nil {
		t.Fatal(err)
	}
	list := m.GetProfiles()
	if !list.Profiles[0].Active || !list.Profiles[1].Active {
		t.Fatal("switch lost background job")
	}
	if err := m.CancelPosting(profile.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-doneB:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("business schedule did not stop")
	}
	if !a.GetPostingStatus().Active {
		t.Fatal("stopping business job cancelled personal job")
	}
	if a.browser != nil || b.browser != nil {
		t.Fatal("schedule opened browser before due time")
	}
	if err := m.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-doneA:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown left account job running")
	}
	if _, err := m.GetSettings("default"); err == nil {
		t.Fatal("accepted work after shutdown")
	}
}

func TestProfileValidationAndConcurrentSessionLookup(t *testing.T) {
	m, err := newProfileService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	for _, name := range []string{"", "  ", "DEFAULT PROFILE"} {
		if _, err := m.CreateProfile(name); err == nil {
			t.Fatalf("accepted invalid name %q", name)
		}
	}
	for _, id := range []string{"", "../default", "unknown"} {
		if err := m.SelectProfile(id); err == nil {
			t.Fatalf("accepted unknown profile %q", id)
		}
		if _, err := m.GetSettings(id); err == nil {
			t.Fatalf("fell back to another account for %q", id)
		}
	}
	var wg sync.WaitGroup
	sessions := make(chan *GroupsService, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := m.session("default")
			if err != nil {
				t.Error(err)
			}
			sessions <- s
		}()
	}
	wg.Wait()
	close(sessions)
	var first *GroupsService
	for s := range sessions {
		if first == nil {
			first = s
		}
		if first != s {
			t.Fatal("concurrent calls created duplicate sessions")
		}
	}
	// Failed persistence must not change the live registry.
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	m.root = blocked
	if _, err := m.CreateProfile("Unsaved"); err == nil {
		t.Fatal("claimed to save inaccessible registry")
	}
	if len(m.GetProfiles().Profiles) != 1 {
		t.Fatal("failed save changed live profiles")
	}
}

func TestInvalidProfileRegistryDoesNotOverwriteData(t *testing.T) {
	for _, content := range []string{`broken`, `{"version":2,"selected":"default","profiles":[{"id":"default","name":"Default"}]}`, `{"version":1,"selected":"../escape","profiles":[{"id":"../escape","name":"Escape"}]}`} {
		root := t.TempDir()
		path := filepath.Join(root, "profiles.json")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := newProfileService(root); err == nil {
			t.Fatal("accepted invalid registry")
		}
		if data, _ := os.ReadFile(path); string(data) != content {
			t.Fatal("overwrote invalid registry")
		}
	}
}
