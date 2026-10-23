package main

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"postika/internal/updateauth"
)

func isSystemPackage(kind string) bool { return kind == "deb" || kind == "rpm" || kind == "arch" }

// Wails' binary updater stages beside the executable. Package-managed installs
// are root-owned: stage their signed package in the user cache and delegate the
// actual transaction to the installed package helper instead of swapping files.
type packageUpdateEngine struct {
	publicKey   []byte
	provider    updater.Provider
	request     updater.CheckRequest
	pending     *updater.Release
	staged, dir string
}

func (e *packageUpdateEngine) Check(ctx context.Context) (*updater.Release, error) {
	release, err := e.provider.Check(ctx, e.request)
	if err == nil {
		e.pending = release
	}
	return release, err
}
func (e *packageUpdateEngine) DownloadedPath() string { return e.staged }
func (e *packageUpdateEngine) Restart(context.Context) error {
	return errors.New("System packages must be installed by their package manager")
}
func (e *packageUpdateEngine) DownloadAndInstall(ctx context.Context) error {
	r := e.pending
	if r == nil || r.Verification == nil {
		return errors.New("No signed package is pending")
	}
	if r.Artifact.Filename == "" || strings.ContainsAny(r.Artifact.Filename, "/\\") {
		return errors.New("Invalid package filename")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	root := filepath.Join(cache, "postika", "updates")
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(root, "package-*")
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(dir)
		}
	}()
	// Provider filenames have already passed the path/type checks in Check.
	path := filepath.Join(dir, r.Artifact.Filename)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha512.New()
	if err := e.provider.Download(ctx, r, io.MultiWriter(file, hash), func(int64, int64) {}); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(hash.Sum(nil), r.Verification.Digest) != 1 {
		return errors.New("Package checksum mismatch")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := updateauth.VerifyWithKey(file, r.Verification.Signature, e.publicKey); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if e.dir != "" {
		_ = os.RemoveAll(e.dir)
	}
	e.staged, e.dir = path, dir
	success = true
	return nil
}
