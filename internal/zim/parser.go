// Package zim reads ZIM archive files and extracts articles for rendering.
// Pipeline: ZIM → HTML (compressed) → Markdown → Document.
package zim

import (
	"bytes"
	"fmt"

	gozim "github.com/akhenakh/gozim"
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
)

// Reader wraps a gozim ZimReader and provides document-level access.
type Reader struct {
	zr *gozim.ZimReader
}

// Open opens a ZIM file. Use mmap for large files on low-memory devices.
func Open(path string) (*Reader, error) {
	zr, err := gozim.NewReader(path, true)
	if err != nil {
		return nil, fmt.Errorf("open zim: %w", err)
	}
	return &Reader{zr: zr}, nil
}

// MainPage returns the main (welcome) article as a Document.
func (r *Reader) MainPage() (*document.Document, error) {
	article, err := r.zr.MainPage()
	if err != nil {
		return nil, fmt.Errorf("main page: %w", err)
	}
	return r.articleToDoc(article)
}

// GetArticle looks up an article by its ZIM-internal URL path.
func (r *Reader) GetArticle(url string) (*document.Document, error) {
	article, err := r.zr.GetPageNoIndex(url)
	if err != nil {
		return nil, fmt.Errorf("article %q: %w", url, err)
	}
	return r.articleToDoc(article)
}

// Close releases the underlying reader.
func (r *Reader) Close() error {
	return r.zr.Close()
}

func (r *Reader) articleToDoc(a *gozim.Article) (*document.Document, error) {
	data, err := a.Data()
	if err != nil {
		return nil, fmt.Errorf("read article data: %w", err)
	}
	return html.Parse(bytes.NewReader(data))
}
