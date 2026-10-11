//go:build cgo

package article

import (
	"errors"
	"strings"

	"github.com/dendec/zimlite/internal/zim"
)

// ZIMSource owns its reader. Server serializes all access and must be closed first.
type ZIMSource struct{ reader *zim.Reader }

func OpenZIMSource(filename string) (*ZIMSource, string, error) {
	r, err := zim.Open(filename)
	if err != nil {
		return nil, "", err
	}
	return &ZIMSource{reader: r}, r.MainPagePath(), nil
}

func (s *ZIMSource) ReadResource(name string) (Resource, error) {
	canonical, mime, size, err := s.reader.ResourceInfo(name)
	if err != nil {
		return Resource{}, err
	}
	if size > MaxResourceBytes || (strings.HasPrefix(mime, "text/html") && size > MaxHTMLBytes) {
		return Resource{}, ErrResourceLimit
	}
	if canonical != name {
		return Resource{MIME: mime, CanonicalPath: canonical}, nil
	}
	data, mime, err := s.reader.GetResourceLimited(name, MaxResourceBytes)
	if err != nil {
		if errors.Is(err, zim.ErrResourceTooLarge) {
			return Resource{}, ErrResourceLimit
		}
		return Resource{}, err
	}
	if strings.HasPrefix(mime, "text/html") && len(data) > MaxHTMLBytes {
		return Resource{}, ErrResourceLimit
	}
	return Resource{Data: data, MIME: mime}, nil
}

func (s *ZIMSource) Close() error { s.reader.Close(); return nil }
