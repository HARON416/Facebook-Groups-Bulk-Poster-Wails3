package main

import (
	"os"
	"path/filepath"
	"runtime"
)

func defaultProfileRoot() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, appStorageName), nil
}

// WebKitGTK's default network session uses XDG data/cache paths. Set these
// before creating the application so dev IndexedDB and preferences never open
// the installed app's WebKit databases. This only affects this process/children.
func prepareDevelopmentStorage() error {
	if !developmentBuild || runtime.GOOS != "linux" {
		return nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	data := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(data) {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		data = filepath.Join(home, ".local", "share")
	}
	if err := os.Setenv("XDG_DATA_HOME", filepath.Join(data, appStorageName)); err != nil {
		return err
	}
	return os.Setenv("XDG_CACHE_HOME", filepath.Join(cache, appStorageName))
}
