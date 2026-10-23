//go:build linux

// Root-owned, package-installed helper. It never executes user-supplied commands
// and authenticates a root-private copy before running the package manager.
package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"postika/internal/updateauth"
)

func main() {
	source := flag.String("package", "", "Downloaded package")
	format := flag.String("format", "deb", "deb, rpm, or arch")
	signature := flag.String("signature", "", "Base64 Ed25519ph signature")
	version := flag.String("version", "", "Expected upstream version")
	flag.Parse()
	if err := install(*source, *signature, *version, *format); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func install(source, signature, version, kind string) error {
	if kind != "deb" && kind != "rpm" && kind != "arch" {
		return fmt.Errorf("Unsupported package format")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("Administrator authorization is required")
	}
	if !filepath.IsAbs(source) || version == "" {
		return fmt.Errorf("Invalid update arguments")
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}
	// Never ask apt to install a user-writable file after verification. A new
	// root-private directory prevents replacement between verification/install.
	dir, err := os.MkdirTemp("/var/tmp", "postika-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	extension := map[string]string{"deb": ".deb", "rpm": ".rpm", "arch": ".pkg.tar.zst"}[kind]
	staged := filepath.Join(dir, "postika"+extension)
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	const maxPackageBytes = 2 << 30
	copied, err := io.Copy(out, io.LimitReader(in, maxPackageBytes+1))
	if err != nil {
		return err
	}
	if copied > maxPackageBytes {
		return fmt.Errorf("Downloaded package exceeds the update size limit")
	}
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := updateauth.Verify(out, sig); err != nil {
		return fmt.Errorf("Package signature rejected: %w", err)
	}
	if err := out.Sync(); err != nil {
		return err
	}
	metadata, err := updateauth.QueryPackage(kind, staged, false)
	if err != nil {
		return err
	}
	fullVersion, err := updateauth.PackageMetadata(kind, metadata, version, runtime.GOARCH)
	if err != nil {
		return err
	}
	installed, err := updateauth.QueryPackage(kind, "", true)
	if err != nil {
		return fmt.Errorf("Postika must already be installed: %w", err)
	}
	installedVersion, err := updateauth.PackageMetadata(kind, installed, "", runtime.GOARCH)
	if err != nil {
		return err
	}
	healthy := true
	if kind == "deb" {
		status, err := exec.Command("/usr/bin/dpkg-query", "-W", "-f=${Status}", "postika").Output()
		healthy = err == nil && strings.TrimSpace(string(status)) == "install ok installed"
	}
	if fullVersion == installedVersion && healthy {
		return nil
	}
	var cmd *exec.Cmd
	switch kind {
	case "deb":
		if err := exec.Command("/usr/bin/dpkg", "--compare-versions", fullVersion, "ge", installedVersion).Run(); err != nil {
			return fmt.Errorf("Package downgrade rejected")
		}
		cmd = exec.Command("/usr/bin/apt-get", "install", "--yes", "--reinstall", "--no-remove", "--no-install-recommends", "-o", "Dpkg::Options::=--force-confold", staged)
	case "rpm":
		// upgrade never requests a downgrade; leave distro signature policy on.
		cmd = exec.Command("/usr/bin/dnf", "upgrade", "--assumeyes", staged)
	case "arch":
		compared, err := exec.Command("/usr/bin/vercmp", fullVersion, installedVersion).Output()
		if err != nil {
			return err
		}
		order, err := strconv.Atoi(strings.TrimSpace(string(compared)))
		if err != nil || order < 0 {
			return fmt.Errorf("Package downgrade rejected")
		}
		cmd = exec.Command("/usr/bin/pacman", "-U", "--noconfirm", staged)
	}
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive"}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return err
	}
	actual, err := updateauth.QueryPackage(kind, "", true)
	if err != nil {
		return err
	}
	_, err = updateauth.PackageMetadata(kind, actual, version, runtime.GOARCH)
	return err
}
