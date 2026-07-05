// Package ui coordinates the document model, renderer, navigation, and input.
package ui

import (
	"bytes"
	"fmt"
	"log/slog"
	neturl "net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kiwix-sdl/kiwix-sdl/internal/config"
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
	"github.com/kiwix-sdl/kiwix-sdl/internal/menu"
	"github.com/kiwix-sdl/kiwix-sdl/internal/trie"
	"github.com/veandco/go-sdl2/sdl"
)

type appMode int

const (
	modeDoc appMode = iota
	modeTree
)

// App is the top-level application state.
type App struct {
	viewer    DocViewer
	links     LinkBrowser
	scroller  Scroller
	navigator DocNavigator
	running   atomic.Bool
	mode      appMode
	navState  *trie.NavState
	gamepad   GamepadState

	loader *DocumentLoader
	input  *InputController
	stopCh chan struct{}
}

// New creates the app with injected dependencies.
func New(viewer DocViewer, links LinkBrowser, scroller Scroller, n DocNavigator) *App {
	app := &App{
		viewer:    viewer,
		links:     links,
		scroller:  scroller,
		navigator: n,
		mode:      modeDoc,
		stopCh:    make(chan struct{}),
	}
	app.loader = NewDocumentLoader(app)
	app.input = NewInputController(app)
	// Install a static resource loader once. It always resolves paths from the
	// current navigator state, so it stays valid across navigations.
	app.viewer.SetResourceLoader(app.loader.loadResource)
	return app
}

func (app *App) shutdown() {
	close(app.stopCh)
	app.loader.shutdown()
}

// saveCurrentState records the current scroll position and selected link into
// the navigator's current history entry.
func (app *App) saveCurrentState() {
	app.navigator.UpdateCurrentState(document.ViewState{
		ScrollY:      app.scroller.CurrentScrollY(),
		SelectedLink: app.links.SelectedLinkIndex(),
	})
}

func (app *App) showDocument(doc *document.Document, navKey string) {
	app.mode = modeDoc
	app.saveCurrentState()
	app.viewer.SetDocument(doc)
	app.navigator.Open(navKey)
	app.viewer.Relayout()
}

func (app *App) restoreCachedDocument(doc *document.Document, state document.ViewState) {
	app.mode = modeDoc
	app.viewer.SetDocument(doc)
	app.viewer.Relayout()
	app.scroller.SetScrollY(state.ScrollY)
	app.links.SetSelectedLinkIndex(state.SelectedLink)
}

func (app *App) goBack() {
	if app.mode == modeTree {
		app.exitTreeMode()
		return
	}

	app.saveCurrentState()
	if ok, state := app.navigator.Back(); ok {
		prevPath := app.navigator.Current()
		if prevPath == "virtual:menu" {
			_ = app.loader.OpenFile("virtual:menu")
			return
		}
		if prevPath == "virtual:tree" {
			app.enterTreeMode()
			return
		}
		if doc, ok := app.loader.docCache[prevPath]; ok {
			app.restoreCachedDocument(doc, state)
		}
	} else if app.loader.zimReader != nil && app.mode == modeDoc {
		app.enterTreeMode()
	} else {
		if app.navigator.Current() != "virtual:menu" {
			_ = app.loader.OpenFile("virtual:menu")
		}
	}
}

func (app *App) enterTreeMode() {
	if app.loader.zimReader == nil {
		return
	}
	app.saveCurrentState()
	if app.navState == nil {
		articles := app.loader.zimReader.ListArticles()
		if len(articles) == 0 {
			return
		}
		root := trie.NewTree(articles)
		app.navState = trie.NewNavState(root)
	}
	app.mode = modeTree
	app.renderTree()
}

func (app *App) exitTreeMode() {
	app.mode = modeDoc
	// Restore last viewed document from history.
	for app.navigator.Current() == "virtual:tree" {
		app.navigator.Back()
	}
	prevPath := app.navigator.Current()
	if doc, ok := app.loader.docCache[prevPath]; ok {
		state := app.navigator.CurrentState()
		app.restoreCachedDocument(doc, state)
	}
}

func (app *App) goHome() {
	if app.loader.zimReader == nil {
		return
	}
	mainPath := app.loader.zimReader.MainPagePath()
	navKey := "zim:" + mainPath
	doc, ok := app.loader.docCache[navKey]
	if !ok {
		data, mime, err := app.loader.zimReader.MainPage()
		if err != nil {
			slog.Error("goHome: cannot load main page", "error", err)
			return
		}
		if !strings.HasPrefix(mime, "text/html") {
			slog.Error("goHome: main page not HTML", "mime", mime)
			return
		}
		doc, err = html.Parse(bytes.NewReader(data))
		if err != nil {
			slog.Error("goHome: cannot parse main page", "error", err)
			return
		}
		app.loader.docCache[navKey] = doc
	}
	app.showDocument(doc, navKey)
}

func (app *App) toggleMode() {
	if app.loader.zimReader == nil {
		_ = app.loader.OpenFile("virtual:menu")
		return
	}
	if app.mode == modeTree {
		app.exitTreeMode()
	} else {
		app.enterTreeMode()
	}
}

func (app *App) renderTree() {
	if app.navState == nil {
		return
	}
	lines := app.navState.VisibleNodes()
	out := make([]string, 0, len(lines))
	cursorIdx := -1
	for i, l := range lines {
		entry := l.TreePrefix + l.Label
		if l.Suffix != "" {
			entry += " (" + l.Suffix + ")"
		}
		if l.IsCursor {
			entry = ">" + entry
			cursorIdx = i
		}
		out = append(out, entry)
	}
	app.scroller.SetTextLines(out)
	if cursorIdx >= 0 {
		app.scroller.ScrollToLine(cursorIdx)
	}
}

// OpenFile delegates to the DocumentLoader to open a file or virtual path.
func (app *App) OpenFile(path string) error {
	return app.loader.OpenFile(path)
}

// ReloadCurrentDocument reloads the current document while preserving scroll and selection.
func (app *App) ReloadCurrentDocument(doc *document.Document) {
	app.loader.docCache[app.navigator.Current()] = doc
	sy := app.scroller.CurrentScrollY()
	sel := app.links.SelectedLinkIndex()
	app.viewer.SetDocument(doc)
	app.scroller.SetScrollY(sy)
	app.links.SetSelectedLinkIndex(sel)
	app.viewer.Relayout()
	_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
}

// HandleSettingsAction parses settings URL and updates config and UI.
func (app *App) HandleSettingsAction(u *neturl.URL) {
	themeParam := u.Query().Get("theme")
	langParam := u.Query().Get("lang")
	fsParam := u.Query().Get("fontsize")

	var fsDelta int
	hasFsDelta := false
	if fsParam != "" {
		if _, err := fmt.Sscanf(fsParam, "%d", &fsDelta); err != nil {
			slog.Warn("Invalid fontsize value", "value", fsParam)
		} else {
			hasFsDelta = true
		}
	}

	changed := false
	themeChanged := false
	config.Update(func(c *config.Config) {
		if themeParam != "" && themeParam != c.Theme {
			c.Theme = themeParam
			changed = true
			themeChanged = true
		}
		if langParam != "" && langParam != c.Language {
			c.Language = langParam
			changed = true
		}
		if hasFsDelta {
			c.FontSize += fsDelta
			if c.FontSize < 10 {
				c.FontSize = 10
			}
			if c.FontSize > 32 {
				c.FontSize = 32
			}
			changed = true
		}
	})

	if themeChanged {
		app.viewer.ToggleTheme() // Update UI immediately
	}
	if hasFsDelta {
		_ = app.viewer.Zoom(fsDelta)
	}
	if changed {
		_ = config.Save()
		// Reload the settings page inline without pushing to history
		doc, _ := menu.SettingsPage()
		app.ReloadCurrentDocument(doc)
	}
}

// Run starts the main event loop. Blocks until quit.
func (app *App) Run() {
	defer app.shutdown()
	app.loader.checkInternetAsync()
	app.running.Store(true)

	// Background ticker to wake up the event loop for animations at ~30 FPS
	go func() {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-app.stopCh:
				return
			}
			if app.viewer != nil && app.viewer.HasAnimations() {
				// Push a dummy event to wake up sdl.WaitEvent()
				_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			}
		}
	}()

	for app.running.Load() {
		event := sdl.WaitEvent()
		if event == nil {
			continue
		}
		app.input.ProcessEvent(event)
		for {
			ev := sdl.PollEvent()
			if ev == nil {
				break
			}
			app.input.ProcessEvent(ev)
		}
		app.viewer.Render()
	}
}
