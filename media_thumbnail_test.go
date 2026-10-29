package main

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"testing"
)

func TestVideoThumbnail(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	var data []byte
	if path := os.Getenv("POSTIKA_THUMBNAIL_TEST_VIDEO"); path != "" {
		data, err = os.ReadFile(path)
	} else {
		data, err = exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=red:s=320x180:d=1", "-f", "matroska", "-c:v", "ffv1", "pipe:1").Output()
	}
	if err != nil {
		t.Fatal(err)
	}
	poster, err := videoThumbnail(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(poster))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width > 640 || config.Height > 640 || config.Width == 0 || config.Height == 0 {
		t.Fatalf("Unexpected poster size: %dx%d", config.Width, config.Height)
	}
	t.Logf("Generated JPEG poster: %dx%d, %d bytes", config.Width, config.Height, len(poster))
	if path := os.Getenv("POSTIKA_THUMBNAIL_TEST_OUTPUT"); path != "" {
		if err := os.WriteFile(path, poster, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVideoThumbnailRejectsInvalidInput(t *testing.T) {
	if _, err := videoThumbnail(context.Background(), nil); err == nil {
		t.Fatal("empty input accepted")
	}
	if _, err := videoThumbnail(context.Background(), []byte("not a video")); err == nil {
		t.Fatal("invalid input accepted")
	}
}
