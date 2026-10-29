package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestBuildIdentityAndAccountStorage(t *testing.T) {
	wantName, wantInstance, wantLinux, wantStorage := "Postika", "app.postika.desktop", "app.postika", "postika"
	if developmentBuild {
		wantName, wantInstance, wantLinux, wantStorage = "Postika Dev", "app.postika.desktop.dev", "app.postika.dev", "postika-dev"
	}
	if appDisplayName != wantName || appInstanceID != wantInstance || appLinuxID != wantLinux || appStorageName != wantStorage {
		t.Fatal("incorrect build identity")
	}
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("APPDATA", config)
	t.Setenv("HOME", config)
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	installed, err := newProfileService(filepath.Join(base, "postika"))
	if err != nil {
		t.Fatal(err)
	}
	defer installed.ServiceShutdown()
	if _, err := installed.CreateProfile("Installed account"); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(installed.root, "profiles.json")
	before, err := os.ReadFile(registry)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := newProfileService("")
	if err != nil {
		t.Fatal(err)
	}
	defer profiles.ServiceShutdown()
	if profiles.root != filepath.Join(base, wantStorage) {
		t.Fatalf("profile root: %s", profiles.root)
	}
	if developmentBuild {
		if len(profiles.registry.Profiles) != 1 || profiles.registry.Profiles[0].Name != "Default profile" {
			t.Fatal("development imported installed accounts")
		}
		if _, err := profiles.CreateProfile("Dev account"); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(registry)
		if err != nil || string(after) != string(before) {
			t.Fatal("development changed the installed registry")
		}
	} else if len(profiles.registry.Profiles) != 2 {
		t.Fatal("production no longer reads installed accounts")
	}
}

func TestDevelopmentWebkitStorage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("WebKitGTK uses XDG paths on Linux")
	}
	for _, dataHome := range []string{"", filepath.Join(t.TempDir(), "data")} {
		t.Run(dataHome, func(t *testing.T) {
			home, cache := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", dataHome)
			t.Setenv("XDG_CACHE_HOME", cache)
			if err := prepareDevelopmentStorage(); err != nil {
				t.Fatal(err)
			}
			wantData, wantCache := dataHome, cache
			if developmentBuild {
				if wantData == "" {
					wantData = filepath.Join(home, ".local", "share")
				}
				wantData = filepath.Join(wantData, "postika-dev")
				wantCache = filepath.Join(cache, "postika-dev")
			}
			if os.Getenv("XDG_DATA_HOME") != wantData || os.Getenv("XDG_CACHE_HOME") != wantCache {
				t.Fatal("incorrect WebKit storage paths")
			}
		})
	}
}

func TestDevelopmentUpdatesDisabled(t *testing.T) {
	if !developmentBuild {
		t.Skip("production keeps the normal updater")
	}
	// A nil app ensures construction never initializes the native updater.
	service, err := newUpdateService(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !service.disabled || service.engine != nil || service.GetUpdateStatus().State != "disabled" {
		t.Fatal("development updater is enabled")
	}
	if err := service.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	if service.done != nil {
		t.Fatal("development started update polling")
	}
	if _, err := service.CheckForUpdates(context.Background()); err == nil {
		t.Fatal("development allowed a manual check")
	}
	if err := service.DownloadUpdate(); err == nil {
		t.Fatal("development allowed an update download")
	}
	if err := service.RestartToUpdate(context.Background()); err == nil {
		t.Fatal("development allowed update installation")
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
}
