// Package ui coordinates the document model, renderer, navigation, and input.
package ui

import (
	"fmt"
	"log/slog"
	neturl "net/url"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/i18n"
	"github.com/dendec/zimlite/internal/menu"
	"github.com/dendec/zimlite/internal/navigation"
	"github.com/dendec/zimlite/internal/trie"
	"github.com/veandco/go-sdl2/sdl"
)

type appMode int

const (
	modeDoc appMode = iota
	modeTree
)

const (
	maxTreeExpansionLevel = 100
	frameDuration         = 33 * time.Millisecond
)

// App is the top-level application state.
type App struct {
	viewer    DocViewer
	links     LinkBrowser
	scroller  Scroller
	navigator navigation.Navigator
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
func New(viewer DocViewer, links LinkBrowser, scroller Scroller, n navigation.Navigator, cfg config.Provider) *App {
	app := &App{
		viewer:    viewer,
		links:     links,
		scroller:  scroller,
		navigator: n,
		config:    cfg,
		mode:      modeDoc,
		stopCh:    make(chan struct{}),
	}
	if provider, ok := viewer.(interface{ GameController() *sdl.GameController }); ok {
		app.gamepad.SetController(provider.GameController())
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

// restoreView applies history state only after the target document loads.
func (app *App) restoreView(key string, state document.ViewState) error {
	if _, ok := app.loader.locations[key]; ok {
		if err := app.loader.restoreArticle(key, false); err != nil {
			return err
		}
	} else if doc, ok := app.loader.docCache[key]; ok {
		app.loader.shutdown()
		app.viewer.SetHasTree(false)
		app.mode = modeDoc
		app.viewer.SetDocument(doc)
		app.viewer.Relayout()
	} else if err := app.loader.OpenFile(key); err != nil {
		return err
	}
	app.scroller.SetScrollY(state.ScrollY)
	app.links.SetSelectedLinkIndex(state.SelectedLink)
	app.navigator.UpdateCurrentState(state)
	return nil
}

func (app *App) goBack() {
	if app.loader.cancelPendingDelete() {
		return
	}
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
		if err := app.restoreView(prevPath, state); err != nil {
			slog.Error("Cannot restore document", "path", prevPath, "error", err)
			app.viewer.SetStatusOverride(err.Error())
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
	if viewer, ok := app.viewer.(ArticleViewer); ok {
		viewer.StopArticle()
	}
	if app.navState == nil {
		root, err := trie.NewIndexedTree(app.loader.zimReader)
		if err != nil || len(root.Children()) == 0 {
			if err := app.loader.restoreArticle(app.navigator.Current(), false); err != nil {
				slog.Error("Restore article after empty tree", "error", err)
			}
			if err != nil {
				slog.Error("Cannot load article tree", "error", err)
				app.viewer.SetStatusOverride(i18n.Tf(app.config.Get().Language, "tree.load_error", err))
			}
			return
		}
		app.navState = trie.NewNavState(root)
	}
	app.mode = modeTree
	app.renderTree()
}

func (app *App) exitTreeMode() {
	app.mode = modeDoc
	// Restore last viewed document from history.
	for i := 0; i < maxTreeExpansionLevel && app.navigator.Current() == "virtual:tree"; i++ {
		app.navigator.Back()
	}
	prevPath := app.navigator.Current()
	state := app.navigator.CurrentState()
	if err := app.restoreView(prevPath, state); err != nil {
		slog.Error("Cannot restore document after tree", "path", prevPath, "error", err)
		app.viewer.SetStatusOverride(err.Error())
	}
}

func (app *App) goHome() {
	if app.loader.zimReader == nil {
		return
	}
	location := articleLocation{File: app.loader.zimReader.FilePath(), Entry: app.loader.zimReader.MainPagePath(), ZIM: true}
	if err := app.loader.displayArticle(location, false); err != nil {
		slog.Error("Cannot open main page", "error", err)
	}
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
	items := make([]document.TreeItem, 0, len(lines))
	for _, l := range lines {
		text := l.TreePrefix + l.Label
		labelStart := len([]rune(l.TreePrefix))
		labelEnd := labelStart + len([]rune(l.Label))
		if l.Suffix != "" {
			text += " (" + l.Suffix + ")"
		}
		items = append(items, document.TreeItem{
			Text:       text,
			Path:       l.Path,
			IsLeaf:     l.IsLeaf,
			IsCursor:   l.IsCursor,
			LabelStart: labelStart,
			LabelEnd:   labelEnd,
		})
	}
	title := ""
	if app.loader.zimReader != nil {
		title = filepath.Base(app.loader.zimReader.FilePath())
	}
	app.scroller.SetTextLines(title, items)
	if cursor := app.navState.Cursor; cursor != nil && cursor.LoadError() != nil {
		app.viewer.SetStatusOverride(i18n.Tf(app.config.Get().Language, "tree.load_error", cursor.LoadError()))
	} else {
		app.viewer.SetStatusOverride("")
	}
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
	app.loader.cacheDocument(app.navigator.Current(), doc)
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
	Theme       string
	Renderer    string
	Language    string
	FontSize    int // delta
	HasTheme    bool
	HasRenderer bool
	HasLang     bool
	HasFs       bool
}

func parseSettingsURL(u *neturl.URL) settingsChange {
	sc := settingsChange{
		Theme:    u.Query().Get("theme"),
		Renderer: u.Query().Get("renderer"),
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
	sc.HasRenderer = sc.Renderer == config.ArticleRendererNative || sc.Renderer == config.ArticleRendererServo
	sc.HasTheme = sc.Theme != ""
	sc.HasLang = sc.Language != ""
	return sc
}

func applySettings(cfg config.Provider, sc settingsChange) (themeChanged, anyChanged bool) {
	cfg.Update(func(c *config.Config) {
		if sc.HasRenderer && sc.Renderer != c.ArticleRenderer {
			c.ArticleRenderer = sc.Renderer
			anyChanged = true
		}
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
		app.viewer.SetTheme(sc.Theme)
	}
	if sc.HasLang {
		app.viewer.SetLanguage(sc.Language)
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
		lang := app.config.Get().Language
		doc, err := menu.SettingsPage(lang, app.config.Get())
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
		ticker := time.NewTicker(frameDuration)
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
		app.pollArticle()
		event := sdl.WaitEventTimeout(int(frameDuration / time.Millisecond))
		if event == nil {
			app.input.Update(time.Now())
			app.loader.applyPendingMenuReload()
			app.loader.applyPendingDownloadCompletion()
			app.viewer.Render()
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
		app.input.Update(time.Now())
		app.loader.applyPendingMenuReload()
		app.loader.applyPendingDownloadCompletion()
		app.viewer.Render()
	}
}
