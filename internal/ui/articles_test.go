package ui

import (
	"errors"
	"testing"

	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/navigation"
)

type fallbackTestView struct {
	DocViewer
	LinkBrowser
	Scroller
	ArticleViewer
	doc     *document.Document
	state   document.ViewState
	stopped bool
}

func (v *fallbackTestView) SetDocument(doc *document.Document) {
	v.doc = doc
	v.state = document.ViewState{SelectedLink: -1}
}
func (*fallbackTestView) Relayout()                        {}
func (*fallbackTestView) SetStatusOverride(string)         {}
func (v *fallbackTestView) CurrentScrollY() int32          { return v.state.ScrollY }
func (v *fallbackTestView) SelectedLinkIndex() int         { return v.state.SelectedLink }
func (v *fallbackTestView) SetScrollY(y int32)             { v.state.ScrollY = y }
func (v *fallbackTestView) SetSelectedLinkIndex(index int) { v.state.SelectedLink = index }
func (*fallbackTestView) ArticleError() error              { return errors.New("memory pressure") }
func (v *fallbackTestView) StopArticle()                   { v.stopped = true }

type unreadableTestArchive struct{ ZimReader } // Any read panics: fallback must use saved HTML.

func TestMemoryFallbackUsesCachedHTMLAndRestoresState(t *testing.T) {
	location := articleLocation{File: "archive.zim", Entry: "A/article", ZIM: true}
	view := &fallbackTestView{state: document.ViewState{ScrollY: 120, SelectedLink: 2}}
	app := &App{viewer: view, links: view, scroller: view, navigator: navigation.NewSimpleNavigator(), config: config.NewProvider()}
	loader := &DocumentLoader{host: app, zimReader: &unreadableTestArchive{}, fallbackHTML: []byte("<h1>Article</h1><p>Saved original body</p>")}
	app.loader = loader
	key := loader.locationKey(location)
	loader.fallbackKey = key
	loader.locations = map[string]articleLocation{key: location}
	loader.docCache = map[string]*document.Document{"old": {}}
	app.navigator.Open(key)
	app.pollArticle()
	if !view.stopped || view.doc == nil || len(view.doc.Blocks) == 0 {
		t.Fatal("fallback did not stop the helper and show the saved article")
	}
	if view.state.ScrollY != 120 || view.state.SelectedLink != 2 {
		t.Fatalf("lost saved view state: %+v", view.state)
	}
	if loader.fallbackHTML != nil || len(loader.docCache) != 1 {
		t.Fatal("fallback retained old article buffers")
	}
}
