// Package main creates a portable Windows distribution without an installer.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	exe := flag.String("exe", "bin/postika.exe", "Application executable")
	license := flag.String("license", "LICENSE", "Distribution license")
	out := flag.String("out", "bin/postika-windows-amd64-portable.zip", "Output ZIP")
	flag.Parse()
	if err := bundle(*out, *exe, *license); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bundle(output, executable, license string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	for _, entry := range []struct{ path, name string }{{executable, "postika.exe"}, {license, "LICENSE.txt"}} {
		if err := add(archive, entry.path, entry.name); err != nil {
			_ = archive.Close()
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return file.Close()
}

func add(archive *zip.Writer, source, name string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name, header.Method = name, zip.Deflate
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}
