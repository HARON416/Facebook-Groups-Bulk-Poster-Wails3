package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/go-rod/rod/lib/launcher"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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
	if err != nil {
		t.Fatal(err)
	}
	// Windows ACLs are not represented by Go's Unix permission bits.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
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

func TestProfileURLNamesCannotCreateOrSelectAccounts(t *testing.T) {
	root := t.TempDir()
	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	before, err := os.ReadFile(filepath.Join(root, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"https://web.facebook.com/swiftlineMotors", "  HTTPS://WEB.FACEBOOK.COM/page  ", "http://example.com/account", "www.facebook.com/page", "facebook.com/page", "m.facebook.com/profile.php?id=123"} {
		if _, err := m.CreateProfile(name); err == nil || err.Error() != "Account names cannot be URLs" {
			t.Fatalf("URL name accepted or unclear error for %q: %v", name, err)
		}
	}
	if list := m.GetProfiles(); len(list.Profiles) != 1 || list.Selected != "default" {
		t.Fatalf("rejected URL changed accounts: %+v", list)
	}
	after, err := os.ReadFile(filepath.Join(root, "profiles.json"))
	if err != nil || string(after) != string(before) {
		t.Fatal("rejected URL changed the saved registry", err)
	}
	for _, name := range []string{"0718448461", "Business: Nairobi", "Biashara ya familia", "متجر"} {
		if _, err := m.CreateProfile(name); err != nil {
			t.Fatalf("valid account name %q rejected: %v", name, err)
		}
	}
}

func TestLegacyURLNamedProfilesRemainReadable(t *testing.T) {
	root := t.TempDir()
	legacy := `{"version":1,"selected":"0123456789abcdef0123456789abcdef","profiles":[{"id":"default","name":"Default profile"},{"id":"0123456789abcdef0123456789abcdef","name":"https://web.facebook.com/swiftlineMotors"}]}`
	if err := os.WriteFile(filepath.Join(root, "profiles.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	if list := m.GetProfiles(); len(list.Profiles) != 2 || list.Selected != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("legacy accounts were lost: %+v", list)
	}
}

func TestRenameProfilePreservesIdentityStorageAndActiveQueue(t *testing.T) {
	root := t.TempDir()
	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	profile, err := m.CreateProfile("Business")
	if err != nil {
		t.Fatal(err)
	}
	s, err := m.session(profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir, cache, settings := s.profileDir, s.groupsCachePath, s.settingsPath
	if err := s.saveGroups("fixture-user", []Group{{ID: "group", Name: "Saved group"}}); err != nil {
		t.Fatal(err)
	}
	_, cancel := testPostingContext(s)
	defer cancel()
	if err := m.SelectProfile("default"); err != nil {
		t.Fatal(err)
	}
	// Browser work may hold this mutex throughout an active queue. Metadata must
	// remain editable without replacing the session or blocking on browser work.
	s.mu.Lock()
	renamed := make(chan error, 1)
	go func() {
		next, err := m.RenameProfile(profile.ID, "  Family business  ")
		if err == nil && (next.ID != profile.ID || next.Name != "Family business") {
			err = fmt.Errorf("unexpected renamed profile: %+v", next)
		}
		renamed <- err
	}()
	select {
	case err := <-renamed:
		if err != nil {
			s.mu.Unlock()
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		s.mu.Unlock()
		t.Fatal("rename blocked on active browser/queue mutex")
	}
	s.mu.Unlock()
	if list := m.GetProfiles(); list.Selected != "default" || len(list.Profiles) != 2 || list.Profiles[1].Name != "Family business" || !list.Profiles[1].Active {
		t.Fatalf("rename changed selection or queue: %+v", list)
	}
	current, _ := m.session(profile.ID)
	if current != s || s.profileDir != dir || s.groupsCachePath != cache || s.settingsPath != settings || len(s.GetGroupsLoadStatus().Groups) != 1 {
		t.Fatal("rename replaced profile storage or session")
	}
	s.profileNameMu.RLock()
	name := s.profileName
	s.profileNameMu.RUnlock()
	if name != "Family business" {
		t.Fatal("retained session name is stale")
	}
	if _, err := m.RenameProfile("default", "Personal"); err != nil {
		t.Fatal(err)
	}
	restored, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.ServiceShutdown()
	if list := restored.GetProfiles(); list.Selected != "default" || list.Profiles[0].ID != "default" || list.Profiles[0].Name != "Personal" || list.Profiles[1].ID != profile.ID || list.Profiles[1].Name != "Family business" {
		t.Fatalf("renames did not survive restart: %+v", list)
	}
}

func TestRenameProfileValidationAndFailedSave(t *testing.T) {
	root := t.TempDir()
	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	profile, err := m.CreateProfile("Business")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := m.session(profile.ID)
	before := m.GetProfiles()
	for _, tc := range []struct{ id, name string }{{profile.ID, ""}, {profile.ID, "  "}, {profile.ID, strings.Repeat("a", 81)}, {profile.ID, "DEFAULT PROFILE"}, {profile.ID, "https://facebook.com/page"}, {"unknown", "New name"}, {"../default", "New name"}} {
		if _, err := m.RenameProfile(tc.id, tc.name); err == nil {
			t.Fatalf("accepted invalid rename: %+v", tc)
		}
	}
	if _, err := m.RenameProfile(profile.ID, "Business"); err != nil {
		t.Fatal("unchanged name rejected", err)
	}
	if !reflect.DeepEqual(before, m.GetProfiles()) {
		t.Fatal("invalid rename changed profiles")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	m.root = blocked
	if _, err := m.RenameProfile(profile.ID, "Unsaved"); err == nil {
		t.Fatal("claimed to persist rename to inaccessible registry")
	}
	if !reflect.DeepEqual(before, m.GetProfiles()) {
		t.Fatal("failed save changed live registry")
	}
	s.profileNameMu.RLock()
	name := s.profileName
	s.profileNameMu.RUnlock()
	if name != "Business" {
		t.Fatal("failed save changed session name")
	}
}

func TestDeleteProfileRemovesOnlyItsDataAndStopsQueue(t *testing.T) {
	root := t.TempDir()
	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	profile, err := m.CreateProfile("Delete me")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := m.session(profile.ID)
	other, _ := m.session("default")
	for _, session := range []*GroupsService{s, other} {
		if err := session.SaveSettings(defaultSettings(), "secret", false); err != nil {
			t.Fatal(err)
		}
		if err := session.saveGroups("user", []Group{{ID: "group", Name: "Group"}}); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(session.profileDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(session.profileDir, "Cookies"), []byte("login"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := m.ScheduleRichPostWithOptions(context.Background(), profile.ID, []string{"group"}, "Draft", "", []PostUpload{{Name: "image.png", Data: fixturePNG}}, defaultSettings().PostingOptions, time.Now().Add(time.Hour).Format(time.RFC3339))
		done <- err
	}()
	awaitPostingState(t, s, "scheduled")
	if err := m.DeleteProfile(profile.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("delete left scheduled queue running")
	}
	if _, err := os.Stat(filepath.Dir(s.profileDir)); !os.IsNotExist(err) {
		t.Fatal("deleted profile files remain", err)
	}
	if !s.closed || len(s.postingLogs) != 0 || s.postingCancel != nil {
		t.Fatal("deleted session retained jobs or logs")
	}
	if _, err := m.session(profile.ID); err == nil {
		t.Fatal("deleted session can be reopened")
	}
	if err := s.SaveSettings(defaultSettings(), "late", false); err == nil {
		t.Fatal("late settings save recreated deleted data")
	}
	if data, err := os.ReadFile(filepath.Join(other.profileDir, "Cookies")); err != nil || string(data) != "login" {
		t.Fatal("other login changed", err)
	}
	if settings, err := other.GetSettings(); err != nil || !settings.KeyConfigured {
		t.Fatal("other settings changed", err)
	}
	list := m.GetProfiles()
	if list.Selected != "default" || len(list.Profiles) != 1 {
		t.Fatalf("wrong fallback: %+v", list)
	}
	restored, err := newProfileService(root)
	if err != nil || len(restored.GetProfiles().Profiles) != 1 {
		t.Fatal("deletion not persisted", err)
	}
}

func TestDeleteDefaultAndLastProfile(t *testing.T) {
	root := t.TempDir()
	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	other, _ := m.CreateProfile("Keep me")
	for _, path := range []string{"browser-profile/Cookies", "groups.json", "settings.json", "profiles/" + other.ID + "/browser/Cookies", "global-file"} {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.DeleteProfile("default"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"browser-profile", "groups.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatal("default data retained", path, err)
		}
	}
	for _, path := range []string{"profiles.json", "global-file", "profiles/" + other.ID + "/browser/Cookies"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatal("unrelated data removed", path, err)
		}
	}
	if err := m.DeleteProfile(other.ID); err != nil {
		t.Fatal(err)
	}
	list := m.GetProfiles()
	if list.Selected != "" || len(list.Profiles) != 0 {
		t.Fatalf("last deletion manufactured another account: %+v", list)
	}
	restored, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.GetProfiles().Profiles) != 0 {
		t.Fatal("empty registry reset on restart")
	}
	fresh, err := restored.CreateProfile("Fresh login")
	if err != nil || fresh.ID == other.ID || fresh.ID == "default" {
		t.Fatal("cannot add fresh account", err)
	}
}

func TestDeleteProfileRejectsUnknownAndPreservesDataOnRegistryFailure(t *testing.T) {
	root := t.TempDir()
	m, err := newProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "../", "missing"} {
		if err := m.DeleteProfile(id); err == nil {
			t.Fatal("accepted unknown profile", id)
		}
	}
	s, _ := m.session("default")
	if err := s.SaveSettings(defaultSettings(), "key", false); err != nil {
		t.Fatal(err)
	}
	// A directory in place of profiles.json makes the final atomic rename fail.
	if err := os.Remove(filepath.Join(root, "profiles.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "profiles.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteProfile("default"); err == nil {
		t.Fatal("ignored registry failure")
	}
	if s.closed || len(m.GetProfiles().Profiles) != 1 {
		t.Fatal("failed deletion closed or removed profile")
	}
	if settings, err := s.GetSettings(); err != nil || !settings.KeyConfigured {
		t.Fatal("failed deletion changed data", err)
	}
}

func TestDeleteProfileBrowserIntegration(t *testing.T) {
	if !*browserIntegration {
		t.Skip("enable with -browser-integration")
	}
	chrome, found := launcher.LookPath()
	if !found {
		t.Fatal("Chrome required")
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><body>Local profile deletion fixture</body></html>")
	}))
	defer fixture.Close()
	m, err := newProfileService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.ServiceShutdown()
	profile, err := m.CreateProfile("Browser deletion")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := m.session(profile.ID)
	s.chromePath, s.headless, s.targetURL = chrome, true, fixture.URL
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := m.OpenJoinedGroups(ctx, profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.profileDir); err != nil {
		t.Fatal("browser profile not created", err)
	}
	if err := m.DeleteProfile(profile.ID); err != nil {
		t.Fatal(err)
	}
	if s.browser != nil || s.launcher != nil || !s.closed {
		t.Fatal("deleted profile left Chrome attached")
	}
	if _, err := os.Stat(filepath.Dir(s.profileDir)); !os.IsNotExist(err) {
		t.Fatal("Chrome data remains after deletion", err)
	}
}
