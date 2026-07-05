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
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
	"github.com/veandco/go-sdl2/sdl"
)

// DocRenderer is the interface for rendering documents.
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
}

// DocNavigator manages document history (back/forward).
type DocNavigator interface {
	Open(id string)
	Back() bool
	Current() string
}

// App is the top-level application state.
type App struct {
	renderer  DocRenderer
	navigator DocNavigator
	running   bool
	docCache  map[string]*document.Document
	zimReader *zim.Reader // non-nil when a ZIM archive is open
}

// New creates the app with injected dependencies.
func New(r DocRenderer, n DocNavigator) *App {
	return &App{
		renderer:  r,
		navigator: n,
		running:   false,
		docCache:  make(map[string]*document.Document),
	}
}

func (app *App) shutdown() {
	if app.zimReader != nil {
		app.zimReader.Close()
		app.zimReader = nil
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
	app.shutdown() // close previous ZIM if any

	zr, err := zim.Open(path)
	if err != nil {
		return nil, err
	}
	app.zimReader = zr

	return zr.MainPage()
}

// navigateLink follows a link URL. For ZIM archives, resolves internally.
func (app *App) navigateLink(url string) {
	// Try ZIM resolver first.
	if app.zimReader != nil {
		doc, err := app.zimReader.GetArticle(url)
		if err == nil {
			app.renderer.SetDocument(doc)
			app.navigator.Open("zim:" + url)
			app.renderer.Relayout()
			return
		}
		// If not found in ZIM, fall through to file loading.
	}

	// Try local file.
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
		switch sc {
		case sdl.SCANCODE_UP, sdl.SCANCODE_W, sdl.SCANCODE_KP_8:
			if app.renderer.LinkCount() > 0 {
				app.renderer.SelectPrevLink()
			} else {
				app.renderer.ScrollBy(-40)
			}
		case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
			if app.renderer.LinkCount() > 0 {
				app.renderer.SelectNextLink()
			} else {
				app.renderer.ScrollBy(40)
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
		case sdl.SCANCODE_Q:
			app.running = false
		}

	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_RESIZED ||
			e.Event == sdl.WINDOWEVENT_SIZE_CHANGED {
			app.renderer.Relayout()
		}
	}
}
