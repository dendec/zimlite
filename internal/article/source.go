// Package article provides bounded access to original article resources.
package article

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxHTMLBytes     = 8 << 20
	MaxResourceBytes = 16 << 20
)

var ErrResourceLimit = errors.New("article resource exceeds memory budget")

type Resource struct {
	Data          []byte
	MIME          string
	CanonicalPath string
}

// Source receives decoded paths relative to the source root, never URLs.
// Implementations must check sizes before allocating resource contents.
type Source interface {
	ReadResource(path string) (Resource, error)
	Close() error
}

type prefetchedSource struct {
	Source
	name     string
	resource Resource
}

// WithPrefetchedResource shares a validated, immutable article body with the
// resource server, avoiding a second archive read/decompression. Ownership of
// the underlying Source is unchanged; readers must not modify Resource.Data.
func WithPrefetchedResource(source Source, name string, resource Resource) Source {
	return &prefetchedSource{Source: source, name: name, resource: resource}
}

func (s *prefetchedSource) ReadResource(name string) (Resource, error) {
	if name == s.name {
		return s.resource, nil
	}
	return s.Source.ReadResource(name)
}

type FileSource struct {
	root *os.Root
}

func OpenFileSource(filename string) (*FileSource, string, error) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return nil, "", err
	}
	source, err := OpenDirectorySource(filepath.Dir(abs))
	return source, filepath.Base(abs), err
}

// OpenDirectorySource preserves the collection root across local navigation.
func OpenDirectorySource(directory string) (*FileSource, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open article root: %w", err)
	}
	return &FileSource{root: root}, nil
}

func (s *FileSource) ReadResource(name string) (Resource, error) {
	f, err := s.root.Open(filepath.FromSlash(name))
	if err != nil {
		return Resource{}, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return Resource{}, err
	}
	if !info.Mode().IsRegular() {
		return Resource{}, errors.New("article resource is not a regular file")
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	limit := int64(MaxResourceBytes)
	if strings.HasPrefix(contentType, "text/html") {
		limit = MaxHTMLBytes
	}
	if info.Size() > limit {
		return Resource{}, ErrResourceLimit
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return Resource{}, err
	}
	if int64(len(data)) > limit {
		return Resource{}, ErrResourceLimit
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return Resource{Data: data, MIME: contentType}, nil
}

func (s *FileSource) Close() error { return s.root.Close() }
