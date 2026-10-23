//go:build !linux

package main

import (
	"context"
	"errors"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"os"
	"runtime"
)

func detectUpdateTarget() (updateTarget, error) {
	executable, err := os.Executable()
	if err != nil {
		return updateTarget{}, err
	}
	kind := "unsupported"
	if runtime.GOOS == "windows" {
		kind = "windows"
	}
	if runtime.GOOS == "darwin" {
		kind = "macos"
	}
	return updateTarget{Kind: kind, Path: executable}, nil
}
func applyPlatformUpdate(context.Context, updateTarget, string, string, *updater.Verification) error {
	return errors.New("Unsupported package update")
}
func handlePlatformUpdateHelper() {}
