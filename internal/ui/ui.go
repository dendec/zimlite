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
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
	"github.com/kiwix-sdl/kiwix-sdl/internal/trie"
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
	"github.com/veandco/go-sdl2/sdl"
)

// DocRenderer is the interface for rendering documents and tree views.
type DocRenderer interface {
	SetDocument(doc *document.Document)
	SetResourceLoader(loader renderer.ResourceLoader)
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
	ToggleTheme()
	HandleClick(mx, my int32) string
	SetHasTree(has bool)
	Zoom(delta int) error
	SetStatusOverride(status string)
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
	renderer  DocRenderer
	navigator DocNavigator
	running   bool
	mode      appMode
	docCache  map[string]*document.Document
	zimReader *zim.Reader
	navState  *trie.NavState
	gamepad   GamepadState
	internetAvailable bool
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

func (app *App) checkInternetAsync() {
	go func() {
		client := http.Client{Timeout: 4 * time.Second}
		resp, err := client.Get("https://browse.library.kiwix.org/catalog/v2/languages")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				app.internetAvailable = true
				if app.navigator.Current() == "virtual:menu" {
					if doc, err := app.generateFileSelectorDoc(); err == nil {
						app.renderer.SetDocument(doc)
						app.renderer.Relayout()
						sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
					}
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
		app.renderer.SetDocument(doc)
		app.renderer.Relayout()
	}
}

func (app *App) goHome() {
	if app.zimReader != nil {
		mainPath := app.zimReader.MainPagePath()
		navKey := "zim:" + mainPath
		if doc, ok := app.docCache[navKey]; ok {
			app.mode = modeDoc
			app.renderer.SetDocument(doc)
			app.navigator.Open(navKey)
			app.renderer.Relayout()
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
	app.renderer.SetTextLines(out)
	if cursorIdx >= 0 {
		app.renderer.ScrollToLine(cursorIdx)
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
		doc, err = app.generateFileSelectorDoc()
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
		if !ok {
			if isZIM {
				doc, err = app.openZIM(absPath)
			} else {
				doc, err = app.openFile(absPath)
			}
			if err != nil {
				return err
			}
			app.docCache[absPath] = doc
		}
	}

	app.mode = modeDoc
	app.renderer.SetResourceLoader(func(rawURL string) ([]byte, error) {
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

	app.renderer.SetDocument(doc)
	if isZIM && app.zimReader != nil {
		mainPath := app.zimReader.MainPagePath()
		navKey := "zim:" + mainPath
		app.docCache[navKey] = doc
		app.navigator.Open(navKey)
	} else {
		app.navigator.Open(absPath)
	}

	hasTree := app.zimReader != nil && app.zimReader.ArticleCount() > 1
	app.renderer.SetHasTree(hasTree)

	app.renderer.Relayout()
	return nil
}

func (app *App) generateFileSelectorDoc() (*document.Document, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Kiwix SDL Document Menu\n\n")
	sb.WriteString("Select a document or ZIM archive to open:\n\n")

	if app.internetAvailable {
		sb.WriteString("## Online Library\n")
		sb.WriteString("* [Browse and Download ZIM Archives](virtual:library)\n\n")
	} else {
		sb.WriteString("## Online Library\n")
		sb.WriteString("*Online library is available when internet is connected.*\n\n")
	}

	var zims []string
	var mds []string

	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".zim":
			zims = append(zims, name)
		case ".md", ".html", ".htm":
			if !strings.HasPrefix(name, ".") {
				mds = append(mds, name)
			}
		}
	}

	if _, err := os.Stat("portmaster/Welcome.md"); err == nil {
		mds = append(mds, "portmaster/Welcome.md")
	}

	if len(zims) > 0 {
		sb.WriteString("## ZIM Archives\n")
		for _, f := range zims {
			fmt.Fprintf(&sb, "* [%s](%s)\n", f, f)
		}
		sb.WriteString("\n")
	}

	if len(mds) > 0 {
		sb.WriteString("## Documents\n")
		for _, f := range mds {
			label := filepath.Base(f)
			fmt.Fprintf(&sb, "* [%s](%s)\n", label, f)
		}
		sb.WriteString("\n")
	}

	if len(zims) == 0 && len(mds) == 0 {
		sb.WriteString("*No ZIM or Markdown files found in the current directory.*\n")
	}

	return markdown.Parse(strings.NewReader(sb.String()))
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
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "//") {
		fmt.Fprintf(os.Stderr, "Opening external URL: %s\n", url)
		_ = exec.Command("xdg-open", url).Start()
		return
	}
	if strings.HasPrefix(url, "#") {
		fmt.Fprintf(os.Stderr, "Anchor link clicked: %s\n", url)
		return
	}

	if app.zimReader != nil {
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
			app.renderer.SetDocument(doc)
			app.navigator.Open(navKey)
			app.renderer.Relayout()
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
			app.renderer.ToggleTheme()
			return
		case sdl.SCANCODE_EQUALS, sdl.SCANCODE_KP_PLUS: // + = zoom in
			_ = app.renderer.Zoom(1)
			return
		case sdl.SCANCODE_MINUS, sdl.SCANCODE_KP_MINUS: // - = zoom out
			_ = app.renderer.Zoom(-1)
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
			app.renderer.Relayout()
		}

	case *sdl.MouseWheelEvent:
		app.renderer.ScrollBy(-scrollStep * e.Y)

	case *sdl.MouseButtonEvent:
		if e.Type == sdl.MOUSEBUTTONDOWN && e.Button == sdl.BUTTON_LEFT && app.mode == modeDoc {
			url := app.renderer.HandleClick(e.X, e.Y)
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
		app.renderer.ScrollPageUp()
	case sdl.SCANCODE_PAGEDOWN:
		app.renderer.ScrollPageDown()
	}
	if app.mode == modeTree {
		app.renderTree()
	}
}

const scrollStep = 40

func (app *App) processDocKey(sc sdl.Scancode) {
	switch sc {
	case sdl.SCANCODE_UP, sdl.SCANCODE_W, sdl.SCANCODE_KP_8:
		app.renderer.ScrollBy(-scrollStep)
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S, sdl.SCANCODE_KP_2:
		app.renderer.ScrollBy(scrollStep)
	case sdl.SCANCODE_LEFT, sdl.SCANCODE_KP_4:
		app.renderer.SelectPrevLink()
	case sdl.SCANCODE_RIGHT, sdl.SCANCODE_KP_6:
		app.renderer.SelectNextLink()
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
		url := app.renderer.SelectedLinkURL()
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
		if doc, ok := app.docCache[prevPath]; ok {
			app.renderer.SetDocument(doc)
			app.renderer.Relayout()
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
				app.renderer.ScrollBy(-scrollStep * int32(-val/16000))
			} else {
				app.renderer.ScrollBy(-scrollStep)
			}
		}
	case ActionScrollDown:
		if app.mode == modeTree {
			app.navState.MoveDown()
			app.renderTree()
		} else {
			if val != 0 {
				app.renderer.ScrollBy(scrollStep * int32(val/16000))
			} else {
				app.renderer.ScrollBy(scrollStep)
			}
		}
	case ActionPageUp:
		app.renderer.ScrollPageUp()
	case ActionPageDown:
		app.renderer.ScrollPageDown()
	case ActionToggleTree:
		app.toggleMode()
	case ActionGoHome:
		app.goHome()
	case ActionQuit:
		app.running = false
	case ActionZoomIn:
		_ = app.renderer.Zoom(1)
	case ActionZoomOut:
		_ = app.renderer.Zoom(-1)
	case ActionSelectPrevLink:
		app.renderer.SelectPrevLink()
	case ActionSelectNextLink:
		app.renderer.SelectNextLink()
	}
}

func debugEvent(kind string, code int, val int) {
	if os.Getenv("KIWIX_DEBUG_INPUT") != "" {
		fmt.Fprintf(os.Stderr, "[INPUT] %s code=%d val=%d\n", kind, code, val)
	}
}

func (app *App) startDownload(downloadURL, filename string) {
	go func() {
		client := http.Client{}
		resp, err := client.Get(downloadURL)
		if err != nil {
			app.renderer.SetStatusOverride("Download failed: " + err.Error())
			sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			return
		}
		defer resp.Body.Close()

		out, err := os.Create(filename)
		if err != nil {
			app.renderer.SetStatusOverride("Create file failed: " + err.Error())
			sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			return
		}
		defer out.Close()

		totalSize := resp.ContentLength
		var downloaded int64
		buf := make([]byte, 32*1024)

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				_, writeErr := out.Write(buf[:n])
				if writeErr != nil {
					app.renderer.SetStatusOverride("Write failed: " + writeErr.Error())
					sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
					return
				}
				downloaded += int64(n)
			}
			if readErr != nil {
				if readErr == io.EOF {
					break
				}
				app.renderer.SetStatusOverride("Download read failed: " + readErr.Error())
				sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
				return
			}

			select {
			case <-ticker.C:
				percent := 0.0
				if totalSize > 0 {
					percent = float64(downloaded) / float64(totalSize) * 100
				}
				app.renderer.SetStatusOverride(fmt.Sprintf("Downloading %s: %.1f%%", filename, percent))
				sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			default:
			}
		}

		app.renderer.SetStatusOverride("Download finished successfully!")
		sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
		time.Sleep(3 * time.Second)
		app.renderer.SetStatusOverride("")

		if app.navigator.Current() == "virtual:menu" {
			if doc, err := app.generateFileSelectorDoc(); err == nil {
				app.renderer.SetDocument(doc)
				app.renderer.Relayout()
			}
		}
	}()
}
