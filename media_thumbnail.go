package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"
)

// Create a small still locally; the original video is never uploaded or changed.
func videoThumbnail(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("No video supplied")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, errors.New("FFmpeg is required for this video's thumbnail")
	}
	file, err := os.CreateTemp("", "postika-video-thumbnail-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-nostdin", "-i", file.Name(), "-map", "0:v:0", "-frames:v", "1", "-vf", "scale=640:640:force_original_aspect_ratio=decrease", "-q:v", "3", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1").Output()
	if err != nil {
		return nil, errors.New("Could not create video thumbnail")
	}
	return result, nil
}
