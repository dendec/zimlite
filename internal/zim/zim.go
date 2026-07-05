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
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"unsafe"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
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
		mainPagePath = C.GoString(C.zim_entry_get_path(entry))
	}

	// Follow main page redirect to get real namespace prefix.
	redirect := C.zim_get_main_page_redirect(h)
	if redirect != nil {
		realPath := C.GoString(redirect)
		rootPrefix = path.Dir(realPath)
		mainPagePath = realPath
	} else if mainPagePath != "" {
		rootPrefix = path.Dir(mainPagePath)
	}

	debug := os.Getenv("KIWIX_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "[Open] file=%q rootPrefix=%q mainPath=%q\n",
			filePath, rootPrefix, mainPagePath)
	}

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
		return C.GoString(redirect)
	}
	return C.GoString(C.zim_entry_get_path(entry))
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
		debug := os.Getenv("KIWIX_DEBUG") != ""
		if debug {
			fmt.Fprintf(os.Stderr, "[GetArticle] NOT FOUND: %q\n", path)
		}
		return nil, fmt.Errorf("article %q: not found", path)
	}
	defer C.zim_entry_free(entry)
	return r.entryToDoc(entry)
}

// ResolveArticle tries multiple path formats to find an article.
// ZIM links may lose namespace prefix (A/) or extension during HTML→MD conversion.
func (r *Reader) ResolveArticle(rawURL string, referrer string) (*document.Document, error) {
	// Skip external URLs.
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") ||
		strings.HasPrefix(rawURL, "//") {
		return nil, fmt.Errorf("external URL: %s", rawURL)
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

	debug := os.Getenv("KIWIX_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "[ResolveArticle] raw=%q referrer=%q rootPrefix=%q decoded=%q\n",
			rawURL, referrer, r.rootPrefix, decoded)
	}

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

	add(pathOnly)
	add(decoded)

	// If already has namespace prefix, try directly.
	if hasNamespace(decoded) {
		add(decoded + ".html")
		base := strings.TrimSuffix(decoded, "/")
		if base != decoded {
			add(base)
			add(base + ".html")
		}
		if debug {
			fmt.Fprintf(os.Stderr, "[ResolveArticle] namespace-prefixed candidates: %v\n", candidates)
		}
		for _, c := range candidates {
			doc, err := r.GetArticle(c)
			if err == nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[ResolveArticle] OK=%q\n", c)
				}
				return doc, nil
			}
			if debug {
				fmt.Fprintf(os.Stderr, "[ResolveArticle] try %q → %v\n", c, err)
			}
		}
		return nil, fmt.Errorf("article not found: %s", rawURL)
	}

	// Build absolute paths.
	if referrer != "" {
		add(path.Join(referrer, decoded))
		add(path.Join(referrer, decoded) + ".html")
		add(path.Join(path.Dir(referrer), decoded))
		add(path.Join(path.Dir(referrer), decoded) + ".html")
	}
	if r.rootPrefix != "" {
		add(path.Join(r.rootPrefix, decoded))
		add(path.Join(r.rootPrefix, decoded) + ".html")
	}

	// With trailing slash.
	n := len(candidates)
	for i := 0; i < n; i++ {
		if !strings.HasSuffix(candidates[i], "/") && candidates[i] != "" {
			add(candidates[i] + "/")
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[ResolveArticle] all candidates: %v\n", candidates)
	}
	for _, c := range candidates {
		doc, err := r.GetArticle(c)
		if err == nil {
			if debug {
				fmt.Fprintf(os.Stderr, "[ResolveArticle] OK=%q\n", c)
			}
			return doc, nil
		}
		if debug {
			fmt.Fprintf(os.Stderr, "[ResolveArticle] try %q → %v\n", c, err)
		}
	}

	return nil, fmt.Errorf("article not found: %s", rawURL)
}

func hasNamespace(s string) bool {
	for _, ns := range []string{"A/", "C/", "I/", "M/", "X/", "-/"} {
		if strings.HasPrefix(s, ns) {
			return true
		}
	}
	return false
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
