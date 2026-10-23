// Package main creates updater ZIPs with exactly one .app root, preserving
// executable modes and internal symlinks without adding __MACOSX sidecars.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	app := flag.String("app", "bin/postika.app", "Existing complete app bundle")
	out := flag.String("out", "bin/postika-darwin.app.zip", "Updater ZIP")
	flag.Parse()
	if err := bundle(*app, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func bundle(app, output string) error {
	app, err := filepath.Abs(app)
	if err != nil {
		return err
	}
	info, err := os.Stat(app)
	if err != nil {
		return err
	}
	if !info.IsDir() || !strings.HasSuffix(app, ".app") {
		return fmt.Errorf("Input must be an .app bundle")
	}
	for _, relative := range []string{"Contents/Info.plist", "Contents/MacOS/postika"} {
		if _, err := os.Stat(filepath.Join(app, relative)); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	err = filepath.WalkDir(app, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(filepath.Dir(app), path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if entry.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			resolved := filepath.Join(filepath.Dir(path), link)
			inside, err := filepath.Rel(app, resolved)
			if err != nil || filepath.IsAbs(link) || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
				return fmt.Errorf("Bundle symlink escapes the app: %s", relative)
			}
			_, err = io.WriteString(writer, link)
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Unsupported bundle entry: %s", relative)
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, source)
		_ = source.Close()
		return copyErr
	})
	closeErr := archive.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return file.Close()
}
