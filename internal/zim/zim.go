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

// Close releases the archive.
func (r *Reader) Close() {
	if r.handle != nil {
		C.zim_close(r.handle)
		r.handle = nil
	}
}

// ArticleCount returns the number of articles in the archive.
func (r *Reader) ArticleCount() int {
	return int(C.zim_get_article_count(r.handle))
}

// TitleByIndex returns the title and path of the article at the given title-order index.
func (r *Reader) TitleByIndex(idx int) (title string, path string, err error) {
	entry := C.zim_get_entry_by_title_index(r.handle, C.int(idx))
	if entry == nil {
		return "", "", fmt.Errorf("no entry at index %d", idx)
	}
	defer C.zim_entry_free(entry)
	title = C.GoString(C.zim_entry_get_title(entry))
	path = C.GoString(C.zim_entry_get_path(entry))
	return title, path, nil
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
