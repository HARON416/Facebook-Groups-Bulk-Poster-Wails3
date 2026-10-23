//go:build linux

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"postika/internal/updateauth"
)

func detectUpdateTarget() (updateTarget, error) {
	executable, err := os.Executable()
	if err != nil {
		return updateTarget{}, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return updateTarget{}, err
	}
	if image, mount := os.Getenv("APPIMAGE"), os.Getenv("APPDIR"); image != "" && mount != "" && strings.HasPrefix(executable, filepath.Clean(mount)+string(filepath.Separator)) {
		image, err = filepath.Abs(image)
		if err != nil {
			return updateTarget{}, err
		}
		image, err = filepath.EvalSymlinks(image)
		if err != nil {
			return updateTarget{}, err
		}
		return updateTarget{Kind: "appimage", Path: image}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, query := range []struct {
		kind, command string
		args          []string
	}{
		{"deb", "dpkg-query", []string{"-S", executable}},
		{"rpm", "rpm", []string{"-qf", "--qf", "%{NAME}", executable}},
		{"arch", "pacman", []string{"-Qoq", executable}},
	} {
		output, err := exec.CommandContext(ctx, query.command, query.args...).Output()
		if err != nil {
			continue
		}
		if query.kind == "deb" && debOwnsExecutable(string(output), executable) || query.kind != "deb" && strings.TrimSpace(string(output)) == "postika" {
			return updateTarget{Kind: query.kind, Path: executable}, nil
		}
	}
	return updateTarget{Kind: "linux-binary", Path: executable}, nil
}
func debOwnsExecutable(output, path string) bool {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		owner, file, ok := strings.Cut(line, ": ")
		if ok && (owner == "postika" || strings.HasPrefix(owner, "postika:")) && file == path {
			return true
		}
	}
	return false
}
func validateDebMetadata(output, version, arch string) error {
	_, err := updateauth.DebMetadata(output, version, arch)
	return err
}
func applyPlatformUpdate(ctx context.Context, target updateTarget, staged, version string, verification *updater.Verification) error {
	if !isSystemPackage(target.Kind) {
		return errors.New("This format uses the Wails updater")
	}
	metadata, err := updateauth.QueryPackage(target.Kind, staged, false)
	if err != nil {
		return err
	}
	if _, err := updateauth.PackageMetadata(target.Kind, metadata, version, runtime.GOARCH); err != nil {
		return err
	}
	if _, err := os.Stat("/usr/bin/pkexec"); err != nil {
		return errors.New("Automatic package updates need PolicyKit for administrator authorization")
	}
	if verification == nil {
		return errors.New("Missing signed package metadata")
	}
	helper := "/usr/lib/postika/update-admin"
	info, err := os.Lstat(helper)
	if err != nil {
		return errors.New("The package-installed update helper is missing")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0022 != 0 || !info.Mode().IsRegular() {
		return errors.New("The update helper must be root-owned and protected against modification")
	}
	args := []string{helper, "-package", staged, "-version", version, "-signature", base64.StdEncoding.EncodeToString(verification.Signature)}
	if target.Kind != "deb" {
		args = append(args, "-format", target.Kind)
	}
	command := exec.Command("/usr/bin/pkexec", args...)
	command.Env = updateHelperEnv(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Package installation was cancelled or failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	actual, err := updateauth.QueryPackage(target.Kind, "", true)
	if err != nil {
		return err
	}
	if _, err := updateauth.PackageMetadata(target.Kind, actual, version, runtime.GOARCH); err != nil {
		return err
	}
	return startLinuxRelaunch(target.Path)
}
func updateHelperEnv(env []string) []string {
	result := make([]string, 0, len(env))
	for _, item := range env {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "APPIMAGE", "APPDIR", "ARGV0", "LD_LIBRARY_PATH", "LD_PRELOAD":
			continue
		}
		result = append(result, item)
	}
	return result
}

// Package installation is already complete. Re-exec the new installed binary
// with a wait flag, rather than copying binaries or implementing another swap.
func startLinuxRelaunch(target string) error {
	dir, err := os.MkdirTemp("", "postika-relaunch-")
	if err != nil {
		return err
	}
	ready := filepath.Join(dir, "ready")
	cmd := exec.Command(target, "--postika-relaunch", strconv.Itoa(os.Getpid()), ready)
	cmd.Env = updateHelperEnv(os.Environ())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			_ = cmd.Process.Release()
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_ = os.RemoveAll(dir)
	return errors.New("The new application did not acknowledge restart; your current session is still open")
}
func waitUpdateParent(ctx context.Context, pid int) error {
	if pid <= 0 {
		return errors.New("Invalid parent process")
	}
	timer := time.NewTicker(25 * time.Millisecond)
	defer timer.Stop()
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func handlePlatformUpdateHelper() {
	if len(os.Args) != 4 || os.Args[1] != "--postika-relaunch" {
		return
	}
	pid, err := strconv.Atoi(os.Args[2])
	ready := os.Args[3]
	if err == nil && pid > 0 && filepath.IsAbs(ready) {
		err = os.WriteFile(ready, []byte("ready"), 0600)
	} else {
		err = errors.New("Invalid relaunch request")
	}
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = waitUpdateParent(ctx, pid)
		cancel()
	}
	_ = os.Remove(ready)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = os.Remove(filepath.Dir(ready))
	os.Args = os.Args[:1]
}
