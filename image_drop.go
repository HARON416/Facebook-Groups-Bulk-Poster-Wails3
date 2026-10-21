package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const imageDropTarget = "post-images"
const imageDropEvent = "postika:images-dropped"

type droppedImage struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Data         []byte `json:"data"` // JSON encodes bytes as base64.
	LastModified int64  `json:"lastModified"`
}

type imageDropResult struct {
	Target    string         `json:"target"`
	ProfileID string         `json:"profileID,omitempty"`
	Images    []droppedImage `json:"images"`
	Errors    []string       `json:"errors"`
}

// Chain imports in drop order without blocking the native event callback.
type imageDropQueue struct {
	mu   sync.Mutex
	tail <-chan struct{}
}

func (q *imageDropQueue) enqueue(importImages func()) {
	q.mu.Lock()
	previous := q.tail
	done := make(chan struct{})
	q.tail = done
	q.mu.Unlock()
	go func() {
		defer close(done)
		if previous != nil {
			<-previous
		}
		importImages()
	}()
}

func registerImageFileDrop(app *application.App, window *application.WebviewWindow) {
	queue := &imageDropQueue{}
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		target := event.Context().DropTargetDetails()
		if target == nil || !strings.HasPrefix(target.ElementID, imageDropTarget+"-") {
			return
		}
		profileID := strings.TrimPrefix(target.ElementID, imageDropTarget+"-")
		paths := append([]string{}, event.Context().DroppedFiles()...)
		queue.enqueue(func() {
			result := readDroppedImages(paths)
			result.ProfileID = profileID
			app.Event.Emit(imageDropEvent, result)
		})
	})
}

// Only called for files supplied by a native drop, not a frontend path reader.
func readDroppedImages(paths []string) imageDropResult {
	result := imageDropResult{Target: imageDropTarget, Images: []droppedImage{}, Errors: []string{}}
	for _, path := range paths {
		image, err := readDroppedImage(path)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		result.Images = append(result.Images, image)
	}
	return result
}

func readDroppedImage(path string) (droppedImage, error) {
	name := filepath.Base(path)
	info, err := os.Stat(path)
	if err != nil {
		return droppedImage{}, fmt.Errorf("Could not read %s. Try browsing for it again.", name)
	}
	if !info.Mode().IsRegular() {
		return droppedImage{}, fmt.Errorf("%s is not an image file. Drop individual images, not folders.", name)
	}
	file, err := os.Open(path)
	if err != nil {
		return droppedImage{}, fmt.Errorf("Could not read %s. Try browsing for it again.", name)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return droppedImage{}, fmt.Errorf("%s is not an image file. Drop individual images, not folders.", name)
	}
	header := make([]byte, 512)
	n, err := io.ReadFull(file, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return droppedImage{}, fmt.Errorf("Could not read %s. Try browsing for it again.", name)
	}
	header = header[:n]
	imageType := http.DetectContentType(header)
	switch imageType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return droppedImage{}, fmt.Errorf("Unsupported file: %s. Choose JPG, PNG, WebP, or GIF images.", name)
	}
	rest, err := io.ReadAll(file)
	if err != nil {
		return droppedImage{}, fmt.Errorf("Could not read %s. Try browsing for it again.", name)
	}
	return droppedImage{Name: name, Type: imageType, Data: append(header, rest...), LastModified: info.ModTime().UnixMilli()}, nil
}
