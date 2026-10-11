// Package ui provides interfaces for dependency injection.
package ui

import (
	"github.com/dendec/zimlite/internal/article"
	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/trie"
)

// ArticleViewer is optional: virtual pages and Markdown keep using DocViewer.
type ArticleViewer interface {
	// OpenArticle takes ownership of Source, including on failure.
	OpenArticle(article.Source, string) error
	StopArticle()
	ArticleActive() bool
	ArticleError() error
	PollArticleNavigation() string
	ResolveArticleURL(string) (string, string, bool)
	ScrollToAnchor(string)
}

// ZimReader is the interface for reading ZIM archives. Implemented by zim.Reader.
type ZimReader interface {
	trie.TitleIndex
	Close()
	MainPagePath() string
	ResolveArticlePath(rawURL string, referrer string) (string, error)
	ResolveResourceLimited(rawURL string, limit uint64) ([]byte, string, error)
	FilePath() string
}

// DocRenderer handles document rendering and display.
type DocRenderer interface {
	SetDocument(doc *document.Document)
	SetResourceLoader(loader document.ResourceLoader)
	Relayout()
	Render()
	ToggleTheme()
	SetTheme(name string)
	SetLanguage(lang string)
	Zoom(delta int) error
	SetHasTree(has bool)
	FindAnchorY(anchor string) (int32, bool)
	HasAnimations() bool
	MarkLinkVisited(url string)
}

// DocInput handles mouse and touch input on the document view.
type DocInput interface {
	HandleMouseMove(mx, my int32)
	HandleMouseLeave()
	HandleTouch()
}

// StatusBar controls the status bar overlay.
type StatusBar interface {
	SetStatusOverride(status string)
	GetStatusOverride() string
}

// DocViewer is the combined interface for backward compatibility.
// Prefer the narrower interfaces above for new code.
type DocViewer interface {
	DocRenderer
	DocInput
	StatusBar
}

// LinkBrowser is the interface for navigating hyperlinks in a document.
type LinkBrowser interface {
	LinkCount() int
	SelectPrevLink()
	SelectNextLink()
	SelectedLinkURL() string
	SelectedLinkIndex() int
	SetSelectedLinkIndex(idx int)
	HandleClick(mx, my int32) string
}

// Scroller is the interface for scrolling and tree-view display.
type Scroller interface {
	ScrollBy(delta int32)
	ScrollPageUp()
	ScrollPageDown()
	ScrollToY(y int32)
	CurrentScrollY() int32
	SetScrollY(scrollY int32)
	SetTextLines(title string, items []document.TreeItem)
	ScrollToLine(lineIdx int)
	HandleTreeClick(mx, my int32) int
}

// TableScroller is an optional document view capability.
type TableScroller interface {
	ScrollTableHorizontally(direction int32) bool
}
