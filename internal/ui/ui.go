// Package ui coordinates the document model, renderer, navigation, and input.
package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
	"github.com/veandco/go-sdl2/sdl"
)

// DocRenderer is the interface for rendering documents.
// Concrete implementations live in internal/renderer.
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
// Concrete implementation lives in internal/navigation.
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

// OpenFile loads a markdown file and displays it.
func (app *App) OpenFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	doc, ok := app.docCache[absPath]
	if !ok {
		f, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		doc, err = markdown.Parse(f)
		if err != nil {
			return fmt.Errorf("parse markdown: %w", err)
		}
		app.docCache[absPath] = doc
	}

	app.renderer.SetDocument(doc)
	app.navigator.Open(absPath)
	app.renderer.Relayout()
	return nil
}

// Run starts the main event loop. Blocks until quit.
func (app *App) Run() {
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
				if err := app.OpenFile(url); err != nil {
					fmt.Fprintf(os.Stderr, "Cannot open: %s (%v)\n", url, err)
				}
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
