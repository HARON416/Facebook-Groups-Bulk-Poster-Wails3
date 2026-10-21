package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Save display data and its profile owner, never captured requests or cookies.
type savedGroups struct {
	Version int     `json:"version"`
	User    string  `json:"user"`
	Groups  []Group `json:"groups"`
}

func (s *GroupsService) groupsCacheFile() (string, error) {
	if s.groupsCachePath != "" {
		return s.groupsCachePath, nil
	}
	if s.profileDir != "" {
		return filepath.Join(s.profileDir, "groups.json"), nil
	}
	settings, err := s.settingsFile()
	return filepath.Join(filepath.Dir(settings), "groups.json"), err
}

// Called with mu held. Restore once so a refresh cannot resurrect old results.
func (s *GroupsService) restoreGroupsLocked() {
	if s.groupsCacheLoaded || s.closed {
		return
	}
	s.groupsCacheLoaded = true
	if s.groupsLoad != nil {
		return
	}
	path, err := s.groupsCacheFile()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	var saved savedGroups
	if err != nil || json.Unmarshal(data, &saved) != nil || saved.Version != 1 || saved.User == "" {
		load := &groupsLoad{}
		load.set("error", "Could not restore saved groups. Click Load groups to refresh.", nil)
		s.groupsLoad = load
		return
	}
	load := &groupsLoad{user: saved.User}
	load.set("complete", fmt.Sprintf("Restored %d saved groups. Load groups to refresh the list.", len(saved.Groups)), saved.Groups)
	s.groupsLoad = load
}

func (s *GroupsService) saveGroups(user string, groups []Group) error {
	path, err := s.groupsCacheFile()
	if err != nil {
		return err
	}
	data, err := json.Marshal(savedGroups{Version: 1, User: user, Groups: append([]Group{}, groups...)})
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".groups-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func (s *GroupsService) completeGroupsLoad(ctx context.Context, load *groupsLoad, user string, groups []Group) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.groupsLoad != load || ctx.Err() != nil {
		return
	}
	s.groupsCacheLoaded = true
	message := fmt.Sprintf("Loaded and saved %d groups.", len(groups))
	if err := s.saveGroups(user, groups); err != nil {
		message = fmt.Sprintf("Loaded %d groups, but could not save them. Load groups again to retry saving.", len(groups))
	}
	load.set("complete", message, groups)
}
