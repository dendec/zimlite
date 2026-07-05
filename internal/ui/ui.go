// Package ui coordinates the document model, renderer, navigation, and input.
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
	"github.com/kiwix-sdl/kiwix-sdl/internal/trie"
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
	"github.com/veandco/go-sdl2/sdl"
)

// DocRenderer is the interface for rendering documents and tree views.
type DocRenderer interface {
	SetDocument(doc *document.Document)
	Relayout()
	Render()
	LinkCount() int
	SelectPrevLink()
	SelectNextLink()
	SelectedLinkURL() string
	ScrollBy(delta int32)
	ScrollPageUp()
	ScrollPageDown()
	SetTextLines(lines []string)
	ScrollToLine(lineIdx int)
}

// DocNavigator manages document history (back/forward).
type DocNavigator interface {
	Open(id string)
	Back() bool
	Current() string
}

type appMode int

const (
	modeDoc  appMode = iota
	modeTree
)

// App is the top-level application state.
type App struct {
	renderer  DocRenderer
	navigator DocNavigator
	running   bool
	mode      appMode
	docCache  map[string]*document.Document
	zimReader *zim.Reader
	navState  *trie.NavState
}

// New creates the app with injected dependencies.
func New(r DocRenderer, n DocNavigator) *App {
	return &App{
		renderer:  r,
		navigator: n,
		running:   false,
		mode:      modeDoc,
		docCache:  make(map[string]*document.Document),
	}
}

func (app *App) shutdown() {
	if app.zimReader != nil {
		app.zimReader.Close()
		app.zimReader = nil
	}
	app.navState = nil
}

func (app *App) enterTreeMode() {
	if app.zimReader == nil {
		return
	}
	articles := app.zimReader.ListArticles()
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
	if doc, ok := app.docCache[prevPath]; ok {
		app.renderer.SetDocument(doc)
		app.renderer.Relayout()
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
		prefix := "  "
		if l.IsLeaf {
			prefix = "* "
		} else if l.IsExpanded {
			prefix = "- "
		} else {
			prefix = "+ "
		}
		entry := indent + prefix + l.Label
		if l.Suffix != "" {
			entry += " (" + l.Suffix + ")"
		}
		if l.IsCursor {
			entry = ">" + entry[1:]
			cursorIdx = i
		}
		out = append(out, entry)
	}
	app.renderer.SetTextLines(out)
	if cursorIdx >= 0 {
		app.renderer.ScrollToLine(cursorIdx)
	}
}

// OpenFile loads a document and displays it. Supports .md, .html, .htm, .zim.
func (app *App) OpenFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	doc, ok := app.docCache[absPath]
	if !ok {
		ext := strings.ToLower(filepath.Ext(absPath))
		switch ext {
		case ".zim":
			doc, err = app.openZIM(absPath)
		default:
			doc, err = app.openFile(absPath)
		}
		if err != nil {
			return err
		}
		app.docCache[absPath] = doc
	}

	app.mode = modeDoc
	app.renderer.SetDocument(doc)
	app.navigator.Open(absPath)
	app.renderer.Relayout()
	return nil
}

func (app *App) openFile(path string) (*document.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return html.Parse(f)
	default:
		return markdown.Parse(f)
	}
}

func (app *App) openZIM(path string) (*document.Document, error) {
	app.shutdown()

	zr, err := zim.Open(path)
	if err != nil {
		return nil, err
	}
	app.zimReader = zr

	return zr.MainPage()
}

func (app *App) navigateLink(url string) {
	if app.zimReader != nil {
		doc, err := app.zimReader.ResolveArticle(url)
		if err == nil {
			app.mode = modeDoc
			app.renderer.SetDocument(doc)
			app.navigator.Open("zim:" + url)
			app.renderer.Relayout()
			return
		}
		return
	}
	if err := app.OpenFile(url); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open: %s (%v)\n", url, err)
	}
}

// Run starts the main event loop. Blocks until quit.
func (app *App) Run() {
	defer app.shutdown()
	app.running = true
	for app.running {
		event := sdl.WaitEvent()
		if event == nil {
			continue
		}
		app.processEvent(event)
		for {
			ev := sdl.PollEvent()
			if ev == nil {
				break
			}
			app.processEvent(ev)
		}
		app.renderer.Render()
	}
}

func (app *App) processEvent(event sdl.Event) {
	switch e := event.(type) {
	case *sdl.QuitEvent:
		app.running = false

	case *sdl.KeyboardEvent:
		if e.Type != sdl.KEYDOWN {
			return
		}
		sc := e.Keysym.Scancode

		// Global keys (work in both modes).
		switch sc {
		case sdl.SCANCODE_Q:
			app.running = false
			return
		case sdl.SCANCODE_RETURN2, sdl.SCANCODE_T: // Start / T = toggle tree mode
			app.toggleMode()
			return
		}

		// Mode-specific handling.
		if app.mode == modeTree {
			app.processTreeKey(sc)
		} else {
			app.processDocKey(sc)
		}

	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_RESIZED ||
			e.Event == sdl.WINDOWEVENT_SIZE_CHANGED {
			app.renderer.Relayout()
		}
	}
}

func (app *App) toggleMode() {
	if app.zimReader == nil {
		return // no ZIM open, tree unavailable
	}
	if app.mode == modeTree {
		app.exitTreeMode()
	} else {
		app.enterTreeMode()
	}
}

func (app *App) processTreeKey(sc sdl.Scancode) {
	switch sc {
	case sdl.SCANCODE_UP, sdl.SCANCODE_W, sdl.SCANCODE_KP_8:
		app.navState.MoveUp()
		app.renderTree()
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
		app.navState.MoveDown()
		app.renderTree()
	case sdl.SCANCODE_RIGHT, sdl.SCANCODE_KP_6, sdl.SCANCODE_RETURN, sdl.SCANCODE_KP_ENTER:
		if app.navState.CursorIsLeaf() {
			// Open article.
			path := app.navState.CursorPath()
			if path != "" {
				app.navigateLink(path)
			}
		} else {
			app.navState.ExpandCurrent()
			app.renderTree()
		}
	case sdl.SCANCODE_LEFT, sdl.SCANCODE_KP_4, sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
		app.navState.CollapseCurrent()
		app.renderTree()
	case sdl.SCANCODE_PAGEUP:
		app.renderer.ScrollPageUp()
	case sdl.SCANCODE_PAGEDOWN:
		app.renderer.ScrollPageDown()
	}
}

const scrollStep = 40

func (app *App) processDocKey(sc sdl.Scancode) {
	switch sc {
	case sdl.SCANCODE_UP, sdl.SCANCODE_W, sdl.SCANCODE_KP_8:
		if app.renderer.LinkCount() > 0 {
			app.renderer.SelectPrevLink()
		} else {
			app.renderer.ScrollBy(-scrollStep)
		}
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
		if app.renderer.LinkCount() > 0 {
			app.renderer.SelectNextLink()
		} else {
			app.renderer.ScrollBy(scrollStep)
		}
	case sdl.SCANCODE_PAGEUP:
		app.renderer.ScrollPageUp()
	case sdl.SCANCODE_PAGEDOWN:
		app.renderer.ScrollPageDown()
	case sdl.SCANCODE_RETURN, sdl.SCANCODE_KP_ENTER:
		url := app.renderer.SelectedLinkURL()
		if url != "" {
			app.navigateLink(url)
		}
	case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
		if app.navigator.Back() {
			prevPath := app.navigator.Current()
			if doc, ok := app.docCache[prevPath]; ok {
				app.renderer.SetDocument(doc)
				app.renderer.Relayout()
			}
		}
	}
}
