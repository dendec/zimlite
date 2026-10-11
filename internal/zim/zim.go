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

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/util"
)

// Reader wraps a libzim Archive handle.
type Reader struct {
	handle       C.zim_archive_t
	rootPrefix   string
	mainPagePath string
	filePath     string
}

var knownNamespacePrefixes = [...]string{"A/", "C/", "I/", "M/", "X/", "-/"}

// HasNamespacePrefix reports whether p already contains a ZIM namespace prefix.
func HasNamespacePrefix(p string) bool {
	for _, prefix := range knownNamespacePrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
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

	return &Reader{handle: h, rootPrefix: rootPrefix, mainPagePath: mainPagePath, filePath: filePath}, nil
}

// FilePath returns the file path of the ZIM archive.
func (r *Reader) FilePath() string {
	return r.filePath
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

// ArticleAt reads a single title-index entry, not its article body.
func (r *Reader) ArticleAt(index int) (document.ArticleEntry, error) {
	if r.handle == nil || index < 0 || index > 2147483647 {
		return document.ArticleEntry{}, errors.New("invalid article index")
	}
	entry := C.zim_get_article_at(r.handle, C.int(index))
	if entry == nil {
		return document.ArticleEntry{}, fmt.Errorf("read article index %d", index)
	}
	defer C.zim_entry_free(entry)
	title := C.zim_entry_get_title(entry)
	defer C.free(unsafe.Pointer(title))
	path := C.zim_entry_get_path(entry)
	defer C.free(unsafe.Pointer(path))
	return document.ArticleEntry{Title: C.GoString(title), Path: C.GoString(path)}, nil
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
	if util.IsExternalURL(rawURL) {
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
		for _, prev := range candidates {
			if prev == c {
				return
			}
		}
		candidates = append(candidates, c)
	}

	// 1. Exact match (handles ZIM paths that actually start with '/')
	add(decoded)

	// 2. Cleaned match (handles HTML absolute links which often have leading '/')
	cleaned := strings.TrimPrefix(decoded, "/")
	for strings.HasPrefix(cleaned, "../") {
		cleaned = cleaned[3:]
	}
	add(cleaned)

	if referrer != "" {
		add(path.Join(path.Dir(referrer), cleaned))
	}
	if r.rootPrefix != "" {
		add(path.Join(r.rootPrefix, cleaned))
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
var ErrResourceTooLarge = errors.New("ZIM resource exceeds size limit")

func (r *Reader) GetResource(path string) ([]byte, string, error) {
	return r.GetResourceLimited(path, 0)
}

// GetResourceLimited rejects oversized entries before copying their contents.
// A zero limit preserves the existing unrestricted reader behavior.
func (r *Reader) GetResourceLimited(path string, limit uint64) ([]byte, string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	entry := C.zim_get_entry_by_path(r.handle, cPath)
	if entry == nil {
		return nil, "", fmt.Errorf("resource %q: not found", path)
	}
	defer C.zim_entry_free(entry)

	return r.entryToDataLimited(entry, limit)
}

// ResolveResource searches namespaces to resolve a relative resource path.
func (r *Reader) ResolveResource(rawURL string) ([]byte, string, error) {
	return r.ResolveResourceLimited(rawURL, 0)
}

// ResolveResourceLimited bounds the copy before decompressing resource contents.
func (r *Reader) ResolveResourceLimited(rawURL string, limit uint64) ([]byte, string, error) {
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
		data, mime, err := r.GetResourceLimited(c, limit)
		if errors.Is(err, ErrResourceTooLarge) {
			return nil, "", err
		}
		if err == nil {
			return data, mime, nil
		}
	}
	return nil, "", fmt.Errorf("resource not found: %s", rawURL)
}

func (r *Reader) entryToData(entry C.zim_entry_t) ([]byte, string, error) {
	return r.entryToDataLimited(entry, 0)
}

func (r *Reader) entryToDataLimited(entry C.zim_entry_t, limit uint64) ([]byte, string, error) {
	item := C.zim_entry_get_item(entry, 1) // follow redirects
	if item == nil {
		return nil, "", errors.New("cannot get item")
	}
	defer C.zim_item_free(item)
	if limit > 0 && uint64(C.zim_item_get_size(item)) > limit {
		return nil, "", fmt.Errorf("%w: %d bytes", ErrResourceTooLarge, limit)
	}

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

// ResourceInfo follows redirects without copying or decompressing entry data.
func (r *Reader) ResourceInfo(name string) (canonical, mime string, size uint64, err error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	entry := C.zim_get_entry_by_path(r.handle, cName)
	if entry == nil {
		return "", "", 0, fmt.Errorf("resource %q: not found", name)
	}
	defer C.zim_entry_free(entry)
	item := C.zim_entry_get_item(entry, 1)
	if item == nil {
		return "", "", 0, errors.New("cannot get resource item")
	}
	defer C.zim_item_free(item)
	cPath := C.zim_item_get_path(item)
	defer C.free(unsafe.Pointer(cPath))
	cMime := C.zim_item_get_mimetype(item)
	defer C.free(unsafe.Pointer(cMime))
	if cPath == nil || cMime == nil {
		return "", "", 0, errors.New("cannot get resource metadata")
	}
	return C.GoString(cPath), C.GoString(cMime), uint64(C.zim_item_get_size(item)), nil
}

// ResolveArticlePath resolves native links to the canonical browser document URL.
func (r *Reader) ResolveArticlePath(rawURL, referrer string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	decoded := u.Path
	cleaned := strings.TrimPrefix(decoded, "/")
	candidates := []string{decoded, cleaned}
	if referrer != "" && !strings.HasPrefix(decoded, "/") {
		candidates = append(candidates, path.Join(path.Dir(referrer), decoded))
	}
	candidates = append(candidates, path.Join(r.rootPrefix, cleaned))
	for _, candidate := range candidates {
		canonical, mime, _, err := r.ResourceInfo(candidate)
		if err == nil && strings.HasPrefix(mime, "text/html") {
			return canonical, nil
		}
	}
	return "", fmt.Errorf("article not found: %s", rawURL)
}
