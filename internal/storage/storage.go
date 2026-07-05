// Package storage handles opening files and ZIM archives.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
