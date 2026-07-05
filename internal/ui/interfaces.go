// Package ui provides interfaces for dependency injection.
package ui

import (
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
)

// ZimReader is the interface for reading ZIM archives. Implemented by zim.Reader.
type ZimReader interface {
	Close()
	ArticleCount() int
	ListArticles() []document.ArticleEntry
	MainPage() (*document.Document, error)
	MainPagePath() string
	ResolveArticle(rawURL string, referrer string) (*document.Document, error)
	ResolveResource(rawURL string) ([]byte, string, error)
}

// DocViewer is the interface for core rendering operations.
type DocViewer interface {
	SetDocument(doc *document.Document)
	SetResourceLoader(loader renderer.ResourceLoader)
	Relayout()
	Render()
	ToggleTheme()
	Zoom(delta int) error
	SetStatusOverride(status string)
	SetHasTree(has bool)
	FindAnchorY(anchor string) (int32, bool)
}

// LinkBrowser is the interface for navigating hyperlinks in a document.
type LinkBrowser interface {
	LinkCount() int
	SelectPrevLink()
	SelectNextLink()
	SelectedLinkURL() string
	HandleClick(mx, my int32) string
}

// Scroller is the interface for scrolling and tree-view display.
type Scroller interface {
	ScrollBy(delta int32)
	ScrollPageUp()
	ScrollPageDown()
	ScrollToY(y int32)
	SetTextLines(lines []string)
	ScrollToLine(lineIdx int)
}

// DocNavigator manages document history (back/forward).
type DocNavigator interface {
	Open(id string)
	Back() bool
	Current() string
}
