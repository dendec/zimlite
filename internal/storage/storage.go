// Package storage handles opening files, ZIM archives, and downloading.
package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
)

// OpenFile reads a file (.md, .html, .htm) and returns a Document.
func OpenFile(path string) (*document.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return html.Parse(f)
	default:
		return markdown.Parse(f)
	}
}

// OpenZIM opens a ZIM archive and returns the main page Document.
func OpenZIM(path string) (*zim.Reader, *document.Document, error) {
	zr, err := zim.Open(path)
	if err != nil {
		return nil, nil, err
	}

	doc, err := zr.MainPage()
	if err != nil {
		zr.Close()
		return nil, nil, err
	}

	return zr, doc, nil
}

// ProgressFn is called with status updates during download (e.g. "Downloading file: 45.2%").
type ProgressFn func(status string)

// Download fetches a file from url, saves to filename, and reports progress via onProgress.
// Blocks until complete or error. Runs in a goroutine when called with go.
func Download(url, filename string, onProgress ProgressFn) error {
	client := http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write: %w", writeErr)
			}
			downloaded += int64(n)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("read: %w", readErr)
		}

		select {
		case <-ticker.C:
			percent := 0.0
			if totalSize > 0 {
				percent = float64(downloaded) / float64(totalSize) * 100
			}
			onProgress(fmt.Sprintf("Downloading %s: %.1f%%", filepath.Base(filename), percent))
		default:
		}
	}

	onProgress("Download finished successfully!")
	return nil
}
