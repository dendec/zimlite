// Package ui provides interfaces for dependency injection.
package ui

import (
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
)

// ZimReader is the interface for reading ZIM archives. Implemented by zim.Reader.
type ZimReader interface {
	Close()
	ArticleCount() int
	ListArticles() []zim.ArticleEntry
	MainPage() (*document.Document, error)
	MainPagePath() string
	ResolveArticle(rawURL string, referrer string) (*document.Document, error)
	ResolveResource(rawURL string) ([]byte, string, error)
}
