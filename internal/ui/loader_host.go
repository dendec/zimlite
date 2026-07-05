package ui

import (
	"net/url"

	"github.com/kiwix-sdl/kiwix-sdl/internal/config"
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

// LoaderHost is the interface DocumentLoader uses to communicate back to the App.
// This breaks the bidirectional dependency: loader depends on the interface, not *App.
type LoaderHost interface {
	showDocument(doc *document.Document, navKey string)
	getNavigator() DocNavigator
	getViewer() DocViewer
	getScroller() Scroller
	getLinks() LinkBrowser
	getConfig() config.Provider
	HandleSettingsAction(u *url.URL)
	navStateClear()
	ReloadCurrentDocument(doc *document.Document)
}

// Ensure App implements LoaderHost at compile time.
var _ LoaderHost = (*App)(nil)

func (app *App) getNavigator() DocNavigator { return app.navigator }
func (app *App) getViewer() DocViewer       { return app.viewer }
func (app *App) getScroller() Scroller      { return app.scroller }
func (app *App) getLinks() LinkBrowser      { return app.links }
func (app *App) getConfig() config.Provider { return app.config }
func (app *App) navStateClear()             { app.navState = nil }
