package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImageDropsFinishInDropOrder(t *testing.T) {
	queue := &imageDropQueue{}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	results := make(chan string, 2)
	queue.enqueue(func() {
		close(firstStarted)
		<-releaseFirst
		results <- "first"
	})
	<-firstStarted
	queue.enqueue(func() { results <- "second" })
	select {
	case result := <-results:
		close(releaseFirst)
		t.Fatalf("later drop overtook a pending import: %s", result)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	for _, expected := range []string{"first", "second"} {
		select {
		case result := <-results:
			if result != expected {
				t.Fatalf("got %s, want %s", result, expected)
			}
		case <-time.After(time.Second):
			t.Fatal("queued drop did not finish")
		}
	}
}

func TestDroppedImagesValidateContentAndPreserveFiles(t *testing.T) {
	dir := t.TempDir()
	image, err := base64.StdEncoding.DecodeString(fixturePNG)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(dir, "first.png"), filepath.Join(dir, "not-an-image.jpg"), dir, filepath.Join(dir, "missing.png"), filepath.Join(dir, "second.bin")}
	for _, path := range []string{paths[0], paths[4]} {
		if err := os.WriteFile(path, image, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(paths[1], []byte("text masquerading as an image"), 0600); err != nil {
		t.Fatal(err)
	}
	result := readDroppedImages(paths)
	if result.Target != imageDropTarget || len(result.Images) != 2 || len(result.Errors) != 3 {
		t.Fatalf("incorrect mixed drop: %+v", result)
	}
	for i, name := range []string{"first.png", "second.bin"} {
		file := result.Images[i]
		info, _ := os.Stat(paths[[]int{0, 4}[i]])
		if file.Name != name || file.Type != "image/png" || !bytes.Equal(file.Data, image) || file.LastModified != info.ModTime().UnixMilli() {
			t.Fatalf("dropped file changed: %+v", file)
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil || strings.Contains(string(encoded), dir) || !strings.Contains(string(encoded), fixturePNG) {
		t.Fatal("drop payload leaked paths or lost image bytes", err)
	}
}

func TestDroppedImageFormats(t *testing.T) {
	for _, test := range []struct{ mime, header string }{{"image/jpeg", "\xff\xd8\xff"}, {"image/gif", "GIF89a"}, {"image/webp", "RIFF\x00\x00\x00\x00WEBPVP"}} {
		t.Run(test.mime, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image")
			if err := os.WriteFile(path, []byte(test.header), 0600); err != nil {
				t.Fatal(err)
			}
			image, err := readDroppedImage(path)
			if err != nil || image.Type != test.mime {
				t.Fatalf("format not accepted: %s, %v", image.Type, err)
			}
		})
	}
}
