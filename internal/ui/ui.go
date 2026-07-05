// Package ui coordinates the document model, renderer, navigation, and input.
package ui

import (
	"strings"

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
	running   bool
	mode      appMode
	navState  *trie.NavState
	gamepad   GamepadState

	loader *DocumentLoader
	input  *InputController
}

// New creates the app with injected dependencies.
func New(viewer DocViewer, links LinkBrowser, scroller Scroller, n DocNavigator) *App {
	app := &App{
		viewer:    viewer,
		links:     links,
		scroller:  scroller,
		navigator: n,
		running:   false,
		mode:      modeDoc,
	}
	app.loader = NewDocumentLoader(app)
	app.input = NewInputController(app)
	return app
}

func (app *App) shutdown() {
	app.loader.shutdown()
}

func (app *App) enterTreeMode() {
	if app.loader.zimReader == nil {
		return
	}
	articles := app.loader.zimReader.ListArticles()
	if len(articles) == 0 {
		return
	}
	root := trie.NewTree(articles)
	app.navState = trie.NewNavState(root)
	app.mode = modeTree
	app.renderTree()
}

func (app *App) exitTreeMode() {
	app.mode = modeDoc
	// Restore last viewed document from history.
	prevPath := app.navigator.Current()
	if doc, ok := app.loader.docCache[prevPath]; ok {
		app.viewer.SetDocument(doc)
		app.viewer.Relayout()
	}
}

func (app *App) goHome() {
	if app.loader.zimReader != nil {
		mainPath := app.loader.zimReader.MainPagePath()
		navKey := "zim:" + mainPath
		if doc, ok := app.loader.docCache[navKey]; ok {
			app.mode = modeDoc
			app.viewer.SetDocument(doc)
			app.navigator.Open(navKey)
			app.viewer.Relayout()
		}
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
	out := make([]string, 0, len(lines))
	cursorIdx := -1
	for i, l := range lines {
		indent := strings.Repeat("  ", l.Indent)
		var prefix string
		if l.IsLeaf {
			prefix = "• "
		} else if l.IsExpanded {
			prefix = "▾ "
		} else {
			prefix = "▸ "
		}
		entry := indent + prefix + l.Label
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

// Run starts the main event loop. Blocks until quit.
func (app *App) Run() {
	defer app.shutdown()
	app.loader.checkInternetAsync()
	app.running = true
	for app.running {
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
