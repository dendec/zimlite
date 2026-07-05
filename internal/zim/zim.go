// Package zim reads ZIM archives via libzim C++ library (cgo bridge).
// Pipeline: ZIM → HTML → Markdown → Document.
package zim

/*
#cgo CXXFLAGS: -std=c++17 -I. -I../../lib/libzim_linux-x86_64-9.7.0/include
#cgo LDFLAGS: -L../../lib/libzim_linux-x86_64-9.7.0/lib/x86_64-linux-gnu -lzim -Wl,-rpath,'$$ORIGIN/../../lib/libzim_linux-x86_64-9.7.0/lib/x86_64-linux-gnu'
#include <stdlib.h>
#include "bridge.h"
*/
import "C"
import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unsafe"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
)

// Reader wraps a libzim Archive handle.
type Reader struct {
	handle C.zim_archive_t
}

// Open opens a ZIM file. Caller must Close().
func Open(path string) (*Reader, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	h := C.zim_open(cPath)
	if h == nil {
		return nil, fmt.Errorf("open zim: %s", path)
	}
	return &Reader{handle: h}, nil
}

// ArticleEntry holds the title and internal path of an article.
type ArticleEntry struct {
	Title string
	Path  string
}

// Close releases the archive.
func (r *Reader) Close() {
	if r.handle != nil {
		C.zim_close(r.handle)
		r.handle = nil
	}
}

// ArticleCount returns the number of articles.
func (r *Reader) ArticleCount() int {
	return int(C.zim_get_article_count(r.handle))
}

// ListArticles returns all article titles and paths in title order.
// Uses libzim's iterByTitle() — only FRONT_ARTICLE entries, no JS/CSS/images.
func (r *Reader) ListArticles() []ArticleEntry {
	var count C.int
	entries := C.zim_list_articles(r.handle, &count)
	if entries == nil || count == 0 {
		return nil
	}
	defer C.zim_free_article_list(entries, count)

	result := make([]ArticleEntry, int(count))
	base := unsafe.Pointer(entries)
	size := unsafe.Sizeof(*entries)
	for i := 0; i < int(count); i++ {
		p := (*C.zim_article_entry_t)(unsafe.Add(base, uintptr(i)*size))
		result[i] = ArticleEntry{
			Title: C.GoString(p.title),
			Path:  C.GoString(p.path),
		}
	}
	return result
}

// MainPage returns the main article as a Document.
func (r *Reader) MainPage() (*document.Document, error) {
	entry := C.zim_get_main_entry(r.handle)
	if entry == nil {
		return nil, errors.New("main page: no main entry")
	}
	defer C.zim_entry_free(entry)
	return r.entryToDoc(entry)
}

// GetArticle looks up an article by its ZIM-internal path.
func (r *Reader) GetArticle(path string) (*document.Document, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	entry := C.zim_get_entry_by_path(r.handle, cPath)
	if entry == nil {
		return nil, fmt.Errorf("article %q: not found", path)
	}
	defer C.zim_entry_free(entry)
	return r.entryToDoc(entry)
}

// ResolveArticle tries multiple path formats to find an article.
// ZIM links may lose namespace prefix (A/) or extension during HTML→MD conversion.
func (r *Reader) ResolveArticle(rawURL string) (*document.Document, error) {
	// Skip external URLs.
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") ||
		strings.HasPrefix(rawURL, "//") {
		return nil, fmt.Errorf("external URL: %s", rawURL)
	}

	decoded, err := url.PathUnescape(rawURL)
	if err != nil {
		decoded = rawURL
	}

	candidates := []string{
		rawURL,                        // as-is
		decoded,                       // decoded
		"A/" + decoded,                // old namespace prefix
		decoded + ".html",             // with extension
		"A/" + decoded + ".html",      // old namespace + extension
	}

	for _, c := range candidates {
		doc, err := r.GetArticle(c)
		if err == nil {
			return doc, nil
		}
	}

	return nil, fmt.Errorf("article not found: %s", rawURL)
}

// GetResource retrieves raw bytes and mimetype of any ZIM entry (e.g. images, css).
func (r *Reader) GetResource(path string) ([]byte, string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	entry := C.zim_get_entry_by_path(r.handle, cPath)
	if entry == nil {
		return nil, "", fmt.Errorf("resource %q: not found", path)
	}
	defer C.zim_entry_free(entry)

	item := C.zim_entry_get_item(entry, 1)
	if item == nil {
		return nil, "", errors.New("cannot get item")
	}
	defer C.zim_item_free(item)

	mime := C.GoString(C.zim_item_get_mimetype(item))

	var size C.int
	content := C.zim_item_get_content(item, &size)
	if content == nil {
		return nil, "", errors.New("empty content")
	}
	data := C.GoBytes(unsafe.Pointer(content), size)
	return data, mime, nil
}

// ResolveResource searches namespaces to resolve a relative resource path.
func (r *Reader) ResolveResource(rawURL string) ([]byte, string, error) {
	decoded, err := url.PathUnescape(rawURL)
	if err != nil {
		decoded = rawURL
	}
	
	clean := decoded
	for strings.HasPrefix(clean, "../") {
		clean = clean[3:]
	}
	clean = strings.TrimPrefix(clean, "/")
	
	candidates := []string{
		clean,
		"I/" + clean,
		"images/" + clean,
	}
	
	for _, c := range candidates {
		data, mime, err := r.GetResource(c)
		if err == nil {
			return data, mime, nil
		}
	}
	return nil, "", fmt.Errorf("resource not found: %s", rawURL)
}

func (r *Reader) entryToDoc(entry C.zim_entry_t) (*document.Document, error) {
	item := C.zim_entry_get_item(entry, 1) // follow redirects
	if item == nil {
		return nil, errors.New("cannot get item")
	}
	defer C.zim_item_free(item)

	mime := C.GoString(C.zim_item_get_mimetype(item))
	if !isHTML(mime) {
		return nil, fmt.Errorf("unsupported mime type: %s", mime)
	}

	var size C.int
	content := C.zim_item_get_content(item, &size)
	if content == nil {
		return nil, errors.New("empty content")
	}
	data := C.GoBytes(unsafe.Pointer(content), size)

	return html.Parse(bytes.NewReader(data))
}

func isHTML(mime string) bool {
	return mime == "text/html" || mime == "text/html; charset=utf-8" ||
		mime == "text/html;charset=utf-8" || mime == "text/html; charset=UTF-8"
}
