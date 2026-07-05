// Package zim reads ZIM archives via libzim C++ library (cgo bridge).
// Pipeline: ZIM → HTML → Markdown → Document.
package zim

/*
#cgo CXXFLAGS: -std=c++17 -I.
#include <stdlib.h>
#include "bridge.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"unsafe"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

// Reader wraps a libzim Archive handle.
type Reader struct {
	handle       C.zim_archive_t
	rootPrefix   string
	mainPagePath string
}

// Open opens a ZIM file. Caller must Close().
func Open(filePath string) (*Reader, error) {
	cPath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cPath))

	h := C.zim_open(cPath)
	if h == nil {
		return nil, fmt.Errorf("open zim: %s", filePath)
	}

	entry := C.zim_get_main_entry(h)
	rootPrefix := "A" // default fallback
	mainPagePath := ""
	if entry != nil {
		defer C.zim_entry_free(entry)
		cPathStr := C.zim_entry_get_path(entry)
		mainPagePath = C.GoString(cPathStr)
		C.free(unsafe.Pointer(cPathStr))
	}

	// Follow main page redirect to get real namespace prefix.
	redirect := C.zim_get_main_page_redirect(h)
	if redirect != nil {
		realPath := C.GoString(redirect)
		C.free(unsafe.Pointer(redirect))
		rootPrefix = path.Dir(realPath)
		mainPagePath = realPath
	} else if mainPagePath != "" {
		rootPrefix = path.Dir(mainPagePath)
	}

	slog.Debug("Opening ZIM archive", "file", filePath, "rootPrefix", rootPrefix, "mainPath", mainPagePath)

	return &Reader{handle: h, rootPrefix: rootPrefix, mainPagePath: mainPagePath}, nil
}

// MainPagePath returns the real (redirected) main page path.
func (r *Reader) MainPagePath() string {
	if r.mainPagePath != "" {
		return r.mainPagePath
	}
	entry := C.zim_get_main_entry(r.handle)
	if entry == nil {
		return ""
	}
	defer C.zim_entry_free(entry)
	redirect := C.zim_get_main_page_redirect(r.handle)
	if redirect != nil {
		res := C.GoString(redirect)
		C.free(unsafe.Pointer(redirect))
		return res
	}
	cPathStr := C.zim_entry_get_path(entry)
	res := C.GoString(cPathStr)
	C.free(unsafe.Pointer(cPathStr))
	return res
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
func (r *Reader) ListArticles() []document.ArticleEntry {
	var count C.int
	entries := C.zim_list_articles(r.handle, &count)
	if entries == nil || count == 0 {
		return nil
	}
	defer C.zim_free_article_list(entries, count)

	result := make([]document.ArticleEntry, int(count))
	base := unsafe.Pointer(entries)
	size := unsafe.Sizeof(*entries)
	for i := 0; i < int(count); i++ {
		p := (*C.zim_article_entry_t)(unsafe.Add(base, uintptr(i)*size))
		result[i] = document.ArticleEntry{
			Title: C.GoString(p.title),
			Path:  C.GoString(p.path),
		}
	}
	return result
}

// MainPage returns the main article content and mime.
func (r *Reader) MainPage() ([]byte, string, error) {
	entry := C.zim_get_main_entry(r.handle)
	if entry == nil {
		return nil, "", errors.New("main page: no main entry")
	}
	defer C.zim_entry_free(entry)
	return r.entryToData(entry)
}

// GetArticle looks up an article by its ZIM-internal path.
func (r *Reader) GetArticle(path string) ([]byte, string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	entry := C.zim_get_entry_by_path(r.handle, cPath)
	if entry == nil {
		slog.Debug("GetArticle not found", "path", path)
		return nil, "", fmt.Errorf("article %q: not found", path)
	}
	defer C.zim_entry_free(entry)
	return r.entryToData(entry)
}

// ResolveArticle tries to resolve an article by URL and referrer.
func (r *Reader) ResolveArticle(rawURL string, referrer string) ([]byte, string, error) {
	// Skip external URLs.
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") ||
		strings.HasPrefix(rawURL, "//") {
		return nil, "", fmt.Errorf("external URL: %s", rawURL)
	}

	// Strip query parameters and anchors/fragments
	pathOnly := rawURL
	if idx := strings.IndexAny(pathOnly, "?#"); idx != -1 {
		pathOnly = pathOnly[:idx]
	}

	decoded, err := url.PathUnescape(pathOnly)
	if err != nil {
		decoded = pathOnly
	}

	slog.Debug("Resolving article", "raw", rawURL, "referrer", referrer, "rootPrefix", r.rootPrefix, "decoded", decoded)

	// Deduplicate.
	var candidates []string
	add := func(c string) {
		c = strings.TrimPrefix(c, "/")
		for strings.HasPrefix(c, "../") {
			c = c[3:]
		}
		for _, prev := range candidates {
			if prev == c {
				return
			}
		}
		candidates = append(candidates, c)
	}

	add(decoded)

	if referrer != "" {
		add(path.Join(path.Dir(referrer), decoded))
	}
	if r.rootPrefix != "" {
		add(path.Join(r.rootPrefix, decoded))
	}

	for _, c := range candidates {
		data, mime, err := r.GetArticle(c)
		if err == nil {
			return data, mime, nil
		}
	}

	return nil, "", fmt.Errorf("article not found: %s", rawURL)
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

	cMime := C.zim_item_get_mimetype(item)
	mime := C.GoString(cMime)
	C.free(unsafe.Pointer(cMime))

	var size C.int
	cData := C.zim_item_get_content(item, &size)
	if cData == nil {
		return nil, "", errors.New("empty content")
	}
	data := C.GoBytes(unsafe.Pointer(cData), size)
	C.free(unsafe.Pointer(cData))
	return data, mime, nil
}

// ResolveResource searches namespaces to resolve a relative resource path.
func (r *Reader) ResolveResource(rawURL string) ([]byte, string, error) {
	decoded, err := url.PathUnescape(rawURL)
	if err != nil {
		decoded = rawURL
	}

	clean := decoded
	for strings.HasPrefix(clean, "./") {
		clean = clean[2:]
	}
	for strings.HasPrefix(clean, "../") {
		clean = clean[3:]
	}
	clean = strings.TrimPrefix(clean, "/")

	candidates := []string{
		clean,
		"-/" + clean,
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

func (r *Reader) entryToData(entry C.zim_entry_t) ([]byte, string, error) {
	item := C.zim_entry_get_item(entry, 1) // follow redirects
	if item == nil {
		return nil, "", errors.New("cannot get item")
	}
	defer C.zim_item_free(item)

	cMime := C.zim_item_get_mimetype(item)
	mime := C.GoString(cMime)
	C.free(unsafe.Pointer(cMime))

	var size C.int
	cContent := C.zim_item_get_content(item, &size)
	if cContent == nil {
		return nil, "", errors.New("empty content")
	}
	data := C.GoBytes(unsafe.Pointer(cContent), size)
	C.free(unsafe.Pointer(cContent))

	return data, mime, nil
}
