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
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
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
	config    config.Provider

	loader *DocumentLoader
	input  *InputController
	stopCh chan struct{}
}

// New creates the app with injected dependencies.
func New(viewer DocViewer, links LinkBrowser, scroller Scroller, n DocNavigator, cfg config.Provider) *App {
	app := &App{
		viewer:    viewer,
		links:     links,
		scroller:  scroller,
		navigator: n,
		config:    cfg,
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
	for i := 0; i < 100 && app.navigator.Current() == "virtual:tree"; i++ {
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
	items := make([]renderer.TreeItem, 0, len(lines))
	for _, l := range lines {
		text := l.TreePrefix + l.Label
		labelStart := len([]rune(l.TreePrefix))
		labelEnd := labelStart + len([]rune(l.Label))
		if l.Suffix != "" {
			text += " (" + l.Suffix + ")"
		}
		items = append(items, renderer.TreeItem{
			Text:       text,
			Path:       l.Path,
			IsLeaf:     l.IsLeaf,
			IsCursor:   l.IsCursor,
			LabelStart: labelStart,
			LabelEnd:   labelEnd,
		})
	}
	app.scroller.SetTextLines(items)
	// Scroll to the cursor item if visible.
	for i, item := range items {
		if item.IsCursor {
			app.scroller.ScrollToLine(i)
			break
		}
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

// settingsChange describes the result of parsing a settings URL.
type settingsChange struct {
	Theme    string
	Language string
	FontSize int // delta
	HasTheme bool
	HasLang  bool
	HasFs    bool
}

func parseSettingsURL(u *neturl.URL) settingsChange {
	sc := settingsChange{
		Theme:    u.Query().Get("theme"),
		Language: u.Query().Get("lang"),
	}
	fsParam := u.Query().Get("fontsize")
	if fsParam != "" {
		if _, err := fmt.Sscanf(fsParam, "%d", &sc.FontSize); err != nil {
			slog.Warn("Invalid fontsize value", "value", fsParam)
		} else {
			sc.HasFs = true
		}
	}
	sc.HasTheme = sc.Theme != ""
	sc.HasLang = sc.Language != ""
	return sc
}

func applySettings(cfg config.Provider, sc settingsChange) (themeChanged, anyChanged bool) {
	cfg.Update(func(c *config.Config) {
		if sc.HasTheme && sc.Theme != c.Theme {
			c.Theme = sc.Theme
			themeChanged = true
			anyChanged = true
		}
		if sc.HasLang && sc.Language != c.Language {
			c.Language = sc.Language
			anyChanged = true
		}
		if sc.HasFs {
			c.FontSize += sc.FontSize
			if c.FontSize < config.MinFontSize {
				c.FontSize = config.MinFontSize
			}
			if c.FontSize > config.MaxFontSize {
				c.FontSize = config.MaxFontSize
			}
			anyChanged = true
		}
	})
	return
}

// HandleSettingsAction parses settings URL and updates config and UI.
func (app *App) HandleSettingsAction(u *neturl.URL) {
	sc := parseSettingsURL(u)
	themeChanged, anyChanged := applySettings(app.config, sc)

	if themeChanged {
		app.viewer.ToggleTheme()
	}
	if sc.HasFs {
		if err := app.viewer.Zoom(sc.FontSize); err != nil {
			slog.Error("Zoom failed", "delta", sc.FontSize, "error", err)
		}
	}
	if anyChanged {
		if err := app.config.Save(); err != nil {
			slog.Error("Config save failed", "error", err)
		}
		doc, err := menu.SettingsPage(app.config.Get())
		if err != nil {
			slog.Error("Settings page generation failed", "error", err)
			return
		}
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
