package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestSavedGroupsRefreshAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	load := &groupsLoad{user: "123"}
	service := &GroupsService{groupsCachePath: path, groupsLoad: load}
	groups := []Group{{ID: "456", Name: "Saved group", URL: "https://web.facebook.com/groups/456/", Picture: "picture"}}
	service.completeGroupsLoad(context.Background(), load, "123", groups)
	restored := &GroupsService{groupsCachePath: path}
	status := restored.GetGroupsLoadStatus()
	if status.State != "complete" || !reflect.DeepEqual(status.Groups, groups) || restored.groupsLoad.user != "123" {
		t.Fatalf("saved groups or owner not restored: %+v", status)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows ACLs are not represented by Go's Unix permission bits.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions: %v, %v", info, err)
	}
	// A failed partial refresh must not replace the complete list on disk.
	load.set("error", "Refresh failed", []Group{{ID: "789"}})
	fresh := &GroupsService{groupsCachePath: path}
	if !reflect.DeepEqual(fresh.GetGroupsLoadStatus().Groups, groups) {
		t.Fatal("failed refresh replaced saved groups")
	}
	// Cancelled and stale workers cannot overwrite a newer saved result.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	service.completeGroupsLoad(cancelled, load, "123", nil)
	service.completeGroupsLoad(context.Background(), &groupsLoad{}, "123", nil)
	fresh = &GroupsService{groupsCachePath: path}
	if !reflect.DeepEqual(fresh.GetGroupsLoadStatus().Groups, groups) {
		t.Fatal("cancelled or stale refresh replaced saved groups")
	}
	// A successful empty refresh must remove stale groups too.
	service.completeGroupsLoad(context.Background(), load, "999", nil)
	fresh = &GroupsService{groupsCachePath: path}
	status = fresh.GetGroupsLoadStatus()
	if status.State != "complete" || len(status.Groups) != 0 || fresh.groupsLoad.user != "999" {
		t.Fatalf("empty refresh not saved: %+v", status)
	}
	restored.mu.Lock()
	restored.stopGroupsCapture()
	restored.mu.Unlock()
	if restored.GetGroupsLoadStatus().State != "idle" {
		t.Fatal("refresh resurrected saved groups")
	}
}

func TestSavedGroupsReadAndWriteFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "groups.json")
	if err := os.WriteFile(path, []byte("broken JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	service := &GroupsService{groupsCachePath: path}
	if status := service.GetGroupsLoadStatus(); status.State != "error" || len(status.Groups) != 0 {
		t.Fatalf("corrupt cache accepted: %+v", status)
	}
	load := &groupsLoad{}
	service = &GroupsService{groupsCachePath: filepath.Join(path, "invalid.json"), groupsLoad: load}
	service.completeGroupsLoad(context.Background(), load, "123", []Group{{ID: "456"}})
	status := service.GetGroupsLoadStatus()
	if status.State != "complete" || len(status.Groups) != 1 || status.Message == "Loaded and saved 1 groups." {
		t.Fatalf("save failure hid loaded groups or claimed success: %+v", status)
	}
}
