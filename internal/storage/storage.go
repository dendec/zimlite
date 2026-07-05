// Package storage handles opening files, ZIM archives, and downloading.
package storage

import (
	"bytes"
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
	defer func() { _ = f.Close() }()

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

	data, mime, err := zr.MainPage()
	if err != nil {
		zr.Close()
		return nil, nil, err
	}
	if !strings.HasPrefix(mime, "text/html") {
		zr.Close()
		return nil, nil, fmt.Errorf("unsupported main page mime: %s", mime)
	}

	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		zr.Close()
		return nil, nil, err
	}

	return zr, doc, nil
}

// FormatSize returns a human-readable size string (KB, MB, GB, TB).
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), []string{"KB", "MB", "GB", "TB"}[exp])
}

// HTTPClient creates an http.Client with the given timeout.
func HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// ProgressFn is called with status updates during download (e.g. "Downloading file: 45.2%").
type ProgressFn func(status string)

// Download fetches a file from url, saves to filename, and reports progress via onProgress.
// Blocks until complete or error. Runs in a goroutine when called with go.
func Download(url, filename string, onProgress ProgressFn) error {
	client := HTTPClient(30 * time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	tempFilename := filename + ".part"
	out, err := os.Create(tempFilename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = out.Close() }()

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

	_ = out.Close()
	if err := os.Rename(tempFilename, filename); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	onProgress("Download finished successfully!")
	return nil
}
