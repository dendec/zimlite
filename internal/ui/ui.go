// Package ui coordinates the document model, renderer, navigation, and input.
package ui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/menu"
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
	"github.com/kiwix-sdl/kiwix-sdl/internal/storage"
	"github.com/kiwix-sdl/kiwix-sdl/internal/trie"
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
	"github.com/veandco/go-sdl2/sdl"
)

// DocViewer is the interface for core rendering operations.
type DocViewer interface {
	SetDocument(doc *document.Document)
	SetResourceLoader(loader renderer.ResourceLoader)
	Relayout()
	Render()
	ToggleTheme()
	Zoom(delta int) error
	SetStatusOverride(status string)
	SetHasTree(has bool)
	FindAnchorY(anchor string) (int32, bool)
}

// LinkBrowser is the interface for navigating hyperlinks in a document.
type LinkBrowser interface {
	LinkCount() int
	SelectPrevLink()
	SelectNextLink()
	SelectedLinkURL() string
	HandleClick(mx, my int32) string
}

// Scroller is the interface for scrolling and tree-view display.
type Scroller interface {
	ScrollBy(delta int32)
	ScrollPageUp()
	ScrollPageDown()
	ScrollToY(y int32)
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
	modeDoc appMode = iota
	modeTree
)

// App is the top-level application state.
type App struct {
	viewer            DocViewer
	links             LinkBrowser
	scroller          Scroller
	navigator         DocNavigator
	running           bool
	mode              appMode
	docCache          map[string]*document.Document
	zimReader         ZimReader
	navState          *trie.NavState
	gamepad           GamepadState
	internetAvailable bool
}

// New creates the app with injected dependencies.
func New(viewer DocViewer, links LinkBrowser, scroller Scroller, n DocNavigator) *App {
	return &App{
		viewer:    viewer,
		links:     links,
		scroller:  scroller,
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

func (app *App) checkInternetAsync() {
	go func() {
		if menu.CheckInternet() {
			app.internetAvailable = true
			if app.navigator.Current() == "virtual:menu" {
				if doc, err := menu.FileSelector(true); err == nil {
					app.viewer.SetDocument(doc)
					app.viewer.Relayout()
					sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
				}
			}
		}
	}()
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
		app.viewer.SetDocument(doc)
		app.viewer.Relayout()
	}
}

func (app *App) goHome() {
	if app.zimReader != nil {
		mainPath := app.zimReader.MainPagePath()
		navKey := "zim:" + mainPath
		if doc, ok := app.docCache[navKey]; ok {
			app.mode = modeDoc
			app.viewer.SetDocument(doc)
			app.navigator.Open(navKey)
			app.viewer.Relayout()
		}
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

// OpenFile loads a document and displays it. Supports .md, .html, .htm, .zim, and virtual:menu.
func (app *App) OpenFile(path string) error {
	var absPath string
	var isZIM bool
	var doc *document.Document
	var err error

	if path == "virtual:menu" {
		absPath = "virtual:menu"
		app.shutdown()
		doc, err = menu.FileSelector(app.internetAvailable)
		if err != nil {
			return err
		}
		app.docCache[absPath] = doc
	} else if strings.HasPrefix(path, "virtual:library") {
		absPath = path
		app.shutdown()
		doc, err = app.generateLibraryDoc(path)
		if err != nil {
			return err
		}
		app.docCache[absPath] = doc
	} else {
		absPath, err = filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		isZIM = strings.ToLower(filepath.Ext(absPath)) == ".zim"
		if !isZIM {
			app.shutdown()
		}

		var ok bool
		doc, ok = app.docCache[absPath]
		if !ok || (isZIM && app.zimReader == nil) {
			if isZIM {
				var zr *zim.Reader
				app.shutdown()
				zr, doc, err = storage.OpenZIM(absPath)
				if err != nil {
					return err
				}
				app.zimReader = zr
			} else {
				doc, err = storage.OpenFile(absPath)
				if err != nil {
					return err
				}
			}
			app.docCache[absPath] = doc
		}
	}

	app.mode = modeDoc
	app.viewer.SetResourceLoader(func(rawURL string) ([]byte, error) {
		if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
			client := http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(rawURL)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		}
		if app.zimReader != nil {
			data, _, err := app.zimReader.ResolveResource(rawURL)
			return data, err
		}
		docPath := app.navigator.Current()
		if docPath == "" {
			docPath = absPath
		}
		if strings.HasPrefix(docPath, "virtual:") {
			return os.ReadFile(rawURL)
		}
		dir := filepath.Dir(docPath)
		fullPath := filepath.Join(dir, rawURL)
		return os.ReadFile(fullPath)
	})

	app.viewer.SetDocument(doc)
	if isZIM && app.zimReader != nil {
		mainPath := app.zimReader.MainPagePath()
		navKey := "zim:" + mainPath
		app.docCache[navKey] = doc
		app.navigator.Open(navKey)
	} else {
		app.navigator.Open(absPath)
	}

	hasTree := app.zimReader != nil && app.zimReader.ArticleCount() > 1
	app.viewer.SetHasTree(hasTree)

	app.viewer.Relayout()
	return nil
}

func (app *App) navigateLink(url string) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "//") {
		fmt.Fprintf(os.Stderr, "Opening external URL: %s\n", url)
		_ = exec.Command("xdg-open", url).Start()
		return
	}
	if strings.HasPrefix(url, "#") {
		anchor := url[1:]
		if y, ok := app.viewer.FindAnchorY(anchor); ok {
			app.scroller.ScrollToY(y)
		} else {
			fmt.Fprintf(os.Stderr, "Anchor not found: %s\n", url)
		}
		return
	}

	if app.zimReader != nil && !strings.HasPrefix(url, "virtual:") {
		var referrer string
		current := app.navigator.Current()
		if strings.HasPrefix(current, "zim:") {
			referrer = strings.TrimPrefix(current, "zim:")
		}
		doc, err := app.zimReader.ResolveArticle(url, referrer)
		if err == nil {
			// Store full resolved path so ../ links work across levels.
			resolved := url
			if !strings.HasPrefix(url, "A/") && !strings.HasPrefix(url, "C/") &&
				!strings.HasPrefix(url, "I/") && !strings.HasPrefix(url, "M/") &&
				!strings.HasPrefix(url, "X/") && !strings.HasPrefix(url, "-/") {
				if referrer != "" {
					resolved = path.Join(referrer, url)
				}
			}
			navKey := "zim:" + resolved
			app.docCache[navKey] = doc
			app.mode = modeDoc
			app.viewer.SetDocument(doc)
			app.navigator.Open(navKey)
			app.viewer.Relayout()
			return
		}
		fmt.Fprintf(os.Stderr, "ResolveArticle(%q) failed: %v\n", url, err)
		return
	}
	if err := app.OpenFile(url); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open: %s (%v)\n", url, err)
	}
}

// Run starts the main event loop. Blocks until quit.
func (app *App) Run() {
	defer app.shutdown()
	app.checkInternetAsync()
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
		app.viewer.Render()
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
		debugEvent("KEY", int(sc), 0)

		// Global keys (work in both modes).
		switch sc {
		case sdl.SCANCODE_Q:
			app.running = false
			return
		case sdl.SCANCODE_H: // H = go home
			app.goHome()
			return
		case sdl.SCANCODE_F: // F = open file menu
			_ = app.OpenFile("virtual:menu")
			return
		case sdl.SCANCODE_RETURN2, sdl.SCANCODE_T: // T = toggle tree mode
			app.toggleMode()
			return
		case sdl.SCANCODE_D: // D = toggle dark/light theme
			app.viewer.ToggleTheme()
			return
		case sdl.SCANCODE_EQUALS, sdl.SCANCODE_KP_PLUS: // + = zoom in
			_ = app.viewer.Zoom(1)
			return
		case sdl.SCANCODE_MINUS, sdl.SCANCODE_KP_MINUS: // - = zoom out
			_ = app.viewer.Zoom(-1)
			return
		case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
			// Global back — also works as doc back.
		}

		// Mode-specific handling.
		if app.mode == modeTree {
			app.processTreeKey(sc)
		} else {
			app.processDocKey(sc)
		}

	case *sdl.JoyAxisEvent, *sdl.JoyButtonEvent, *sdl.JoyHatEvent:
		if action, ok := app.gamepad.TranslateEvent(event, app.mode); ok {
			if action != ActionNone {
				var val int16
				if ax, ok := event.(*sdl.JoyAxisEvent); ok {
					val = ax.Value
				}
				app.executeGamepadAction(action, val)
			}
		}

	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_RESIZED ||
			e.Event == sdl.WINDOWEVENT_SIZE_CHANGED {
			app.viewer.Relayout()
		}

	case *sdl.MouseWheelEvent:
		app.scroller.ScrollBy(-scrollStep * e.Y)

	case *sdl.MouseButtonEvent:
		if e.Type == sdl.MOUSEBUTTONDOWN && e.Button == sdl.BUTTON_LEFT && app.mode == modeDoc {
			url := app.links.HandleClick(e.X, e.Y)
			if url != "" {
				app.navigateLink(url)
			}
		}
	}
}

func (app *App) toggleMode() {
	if app.zimReader == nil {
		_ = app.OpenFile("virtual:menu")
		return
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
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
		app.navState.MoveDown()
	case sdl.SCANCODE_RIGHT, sdl.SCANCODE_KP_6:
		app.navState.ActionRight()
	case sdl.SCANCODE_RETURN, sdl.SCANCODE_KP_ENTER:
		fmt.Fprintf(os.Stderr, "ENTER: label=%q, isLeaf=%v, path=%q\n", app.navState.Cursor.Label(), app.navState.CursorIsLeaf(), app.navState.CursorPath())
		if app.navState.CursorIsLeaf() {
			// Open article.
			path := app.navState.CursorPath()
			if path != "" {
				app.navigateLink(path)
			}
		} else {
			app.navState.ActionRight()
		}
	case sdl.SCANCODE_LEFT, sdl.SCANCODE_KP_4:
		app.navState.ActionLeft()
	case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
		app.navState.ActionLeft()
	case sdl.SCANCODE_PAGEUP:
		app.scroller.ScrollPageUp()
	case sdl.SCANCODE_PAGEDOWN:
		app.scroller.ScrollPageDown()
	}
	if app.mode == modeTree {
		app.renderTree()
	}
}

const scrollStep = 40

func (app *App) processDocKey(sc sdl.Scancode) {
	switch sc {
	case sdl.SCANCODE_UP, sdl.SCANCODE_W, sdl.SCANCODE_KP_8:
		app.scroller.ScrollBy(-scrollStep)
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
		app.scroller.ScrollBy(scrollStep)
	case sdl.SCANCODE_LEFT, sdl.SCANCODE_KP_4:
		app.links.SelectPrevLink()
	case sdl.SCANCODE_RIGHT, sdl.SCANCODE_KP_6:
		app.links.SelectNextLink()
	case sdl.SCANCODE_PAGEUP:
		app.scroller.ScrollPageUp()
	case sdl.SCANCODE_PAGEDOWN:
		app.scroller.ScrollPageDown()
	case sdl.SCANCODE_RETURN, sdl.SCANCODE_KP_ENTER:
		url := app.links.SelectedLinkURL()
		if url != "" {
			app.navigateLink(url)
		}
	case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_BACKSPACE:
		if app.navigator.Back() {
			prevPath := app.navigator.Current()
			if prevPath == "virtual:menu" {
				_ = app.OpenFile("virtual:menu")
				return
			}
			if doc, ok := app.docCache[prevPath]; ok {
				app.viewer.SetDocument(doc)
				app.viewer.Relayout()
			}
		} else if app.zimReader != nil {
			app.enterTreeMode()
		} else {
			if app.navigator.Current() != "virtual:menu" {
				_ = app.OpenFile("virtual:menu")
			}
		}
	}
}

func (app *App) processJoyA() {
	if app.mode == modeTree {
		if app.navState.CursorIsLeaf() {
			path := app.navState.CursorPath()
			if path != "" {
				app.navigateLink(path)
			}
		} else {
			app.navState.ActionRight()
			app.renderTree()
		}
	} else {
		url := app.links.SelectedLinkURL()
		if url != "" {
			app.navigateLink(url)
		}
	}
}

func (app *App) processJoyB() {
	if app.mode == modeTree {
		app.navState.ActionLeft()
		app.renderTree()
	} else if app.navigator.Back() {
		prevPath := app.navigator.Current()
		if prevPath == "virtual:menu" {
			_ = app.OpenFile("virtual:menu")
			return
		}
		if doc, ok := app.docCache[prevPath]; ok {
			app.viewer.SetDocument(doc)
			app.viewer.Relayout()
		}
	} else {
		if app.navigator.Current() != "virtual:menu" {
			_ = app.OpenFile("virtual:menu")
		}
	}
}

func (app *App) executeGamepadAction(action Action, val int16) {
	switch action {
	case ActionOpenEnter:
		app.processJoyA()
	case ActionBack:
		app.processJoyB()
	case ActionScrollUp:
		if app.mode == modeTree {
			app.navState.MoveUp()
			app.renderTree()
		} else {
			if val != 0 {
				app.scroller.ScrollBy(-scrollStep * int32(-val/16000))
			} else {
				app.scroller.ScrollBy(-scrollStep)
			}
		}
	case ActionScrollDown:
		if app.mode == modeTree {
			app.navState.MoveDown()
			app.renderTree()
		} else {
			if val != 0 {
				app.scroller.ScrollBy(scrollStep * int32(val/16000))
			} else {
				app.scroller.ScrollBy(scrollStep)
			}
		}
	case ActionPageUp:
		app.scroller.ScrollPageUp()
	case ActionPageDown:
		app.scroller.ScrollPageDown()
	case ActionToggleTree:
		app.toggleMode()
	case ActionGoHome:
		app.goHome()
	case ActionQuit:
		app.running = false
	case ActionZoomIn:
		_ = app.viewer.Zoom(1)
	case ActionZoomOut:
		_ = app.viewer.Zoom(-1)
	case ActionSelectPrevLink:
		app.links.SelectPrevLink()
	case ActionSelectNextLink:
		app.links.SelectNextLink()
	}
}

func debugEvent(kind string, code int, val int) {
	if os.Getenv("KIWIX_DEBUG_INPUT") != "" {
		fmt.Fprintf(os.Stderr, "[INPUT] %s code=%d val=%d\n", kind, code, val)
	}
}

func (app *App) startDownload(downloadURL, filename string) {
	go func() {
		err := storage.Download(downloadURL, filename, func(status string) {
			app.viewer.SetStatusOverride(status)
			sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
		})
		if err != nil {
			app.viewer.SetStatusOverride("Download failed: " + err.Error())
			sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			return
		}
		time.Sleep(3 * time.Second)
		app.viewer.SetStatusOverride("")
		if app.navigator.Current() == "virtual:menu" {
			if doc, err := menu.FileSelector(true); err == nil {
				app.viewer.SetDocument(doc)
				app.viewer.Relayout()
			}
		}
	}()
}
