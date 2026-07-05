// Package storage handles opening files, ZIM archives, and downloading.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
func Download(url, filename string, onProgress ProgressFn) (err error) {
	client := HTTPClient(0)

	tempFilename := filename + ".part"
	infoFilename := filename + ".info"

	var startBytes int64
	var totalSize int64

	if url == "" {
		info, err := LoadDownloadInfo(infoFilename)
		if err != nil {
			return fmt.Errorf("load info: %w", err)
		}
		url = info.URL
		totalSize = info.TotalSize
	}

	if stat, err := os.Stat(tempFilename); err == nil {
		startBytes = stat.Size()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	Manager.Add(filename, cancel)
	defer Manager.Remove(filename)

	flags := os.O_CREATE | os.O_WRONLY
	if startBytes > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	out, createErr := os.OpenFile(tempFilename, flags, 0644)
	if createErr != nil {
		return fmt.Errorf("create file: %w", createErr)
	}
	defer func() { _ = out.Close() }()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if reqErr != nil {
		return fmt.Errorf("request: %w", reqErr)
	}

	if startBytes > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startBytes))
	}

	resp, doErr := client.Do(req)
	if doErr != nil {
		return fmt.Errorf("download: %w", doErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if startBytes > 0 && resp.StatusCode != http.StatusPartialContent {
		startBytes = 0
	}

	if totalSize == 0 || (startBytes == 0 && resp.ContentLength > 0) {
		if resp.ContentLength > 0 {
			totalSize = startBytes + resp.ContentLength
		}
		_ = SaveDownloadInfo(infoFilename, DownloadInfo{
			URL:       url,
			TotalSize: totalSize,
		})
	}

	var downloaded atomic.Int64
	downloaded.Store(startBytes)
	var lastRead atomic.Int64
	lastRead.Store(time.Now().UnixNano())

	var isIdleTimeout atomic.Bool
	done := make(chan struct{})
	go monitorDownload(done, filename, totalSize, &downloaded, &lastRead, cancel, &isIdleTimeout, onProgress)

	err = copyStream(resp.Body, out, &downloaded, &lastRead)
	close(done)

	if isIdleTimeout.Load() {
		return fmt.Errorf("idle timeout")
	}

	if err != nil {
		return err
	}

	_ = out.Close()
	if renameErr := os.Rename(tempFilename, filename); renameErr != nil {
		return fmt.Errorf("rename: %w", renameErr)
	}
	_ = os.Remove(infoFilename)

	onProgress("✅ Download finished successfully!")
	return nil
}

func monitorDownload(done <-chan struct{}, filename string, totalSize int64, downloaded, lastRead *atomic.Int64, cancel context.CancelFunc, isIdleTimeout *atomic.Bool, onProgress ProgressFn) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastBytes int64
	lastTime := time.Now()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			currentBytes := downloaded.Load()
			lr := time.Unix(0, lastRead.Load())

			// 10 seconds idle timeout
			if time.Since(lr) > 10*time.Second {
				isIdleTimeout.Store(true)
				cancel()
				return
			}

			diffBytes := currentBytes - lastBytes
			diffTime := t.Sub(lastTime).Seconds()
			var speedStr string
			if diffTime > 0 {
				speed := float64(diffBytes) / diffTime
				speedStr = FormatSize(int64(speed)) + "/s"
			}

			percent := 0.0
			if totalSize > 0 {
				percent = float64(currentBytes) / float64(totalSize) * 100
			}
			onProgress(fmt.Sprintf("⬇ Downloading %s: %.1f%% (%s)", filepath.Base(filename), percent, speedStr))

			lastBytes = currentBytes
			lastTime = t
		}
	}
}

func copyStream(r io.Reader, w io.Writer, downloaded, lastRead *atomic.Int64) error {
	buf := make([]byte, 32*1024)
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write: %w", writeErr)
			}
			downloaded.Add(int64(n))
			lastRead.Store(time.Now().UnixNano())
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return fmt.Errorf("read: %w", readErr)
		}
	}
}
