package ui

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	neturl "net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/html"
	"github.com/kiwix-sdl/kiwix-sdl/internal/menu"
	"github.com/kiwix-sdl/kiwix-sdl/internal/storage"
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
	"github.com/veandco/go-sdl2/sdl"
)

type VirtualPageGenerator func(path string, loader *DocumentLoader) (*document.Document, error)

type DocumentLoader struct {
	app               *App
	docCache          map[string]*document.Document
	zimReader         ZimReader
	internetAvailable atomic.Bool
	virtualPages      map[string]VirtualPageGenerator
}

func NewDocumentLoader(app *App) *DocumentLoader {
	l := &DocumentLoader{
		app:          app,
		docCache:     make(map[string]*document.Document),
		virtualPages: make(map[string]VirtualPageGenerator),
	}
	l.internetAvailable.Store(true)
	l.registerVirtualPages()
	return l
}

func (l *DocumentLoader) registerVirtualPages() {
	l.virtualPages["virtual:menu"] = func(path string, l *DocumentLoader) (*document.Document, error) {
		return menu.FileSelector(l.internetAvailable.Load())
	}
	l.virtualPages["virtual:help"] = func(path string, l *DocumentLoader) (*document.Document, error) {
		return menu.HelpPage(sdl.NumJoysticks() > 0)
	}
	l.virtualPages["virtual:settings"] = func(path string, l *DocumentLoader) (*document.Document, error) {
		return menu.SettingsPage()
	}
	l.virtualPages["virtual:library"] = func(path string, l *DocumentLoader) (*document.Document, error) {
		return l.generateLibraryDoc(path)
	}
}

func (l *DocumentLoader) shutdown() {
	if l.zimReader != nil {
		l.zimReader.Close()
		l.zimReader = nil
	}
	l.app.navState = nil
}

func (l *DocumentLoader) checkInternetAsync() {
	go func() {
		hasInternet := menu.CheckInternet()
		if hasInternet != l.internetAvailable.Load() {
			l.internetAvailable.Store(hasInternet)
			if l.app.navigator.Current() == "virtual:menu" {
				if doc, err := menu.FileSelector(hasInternet); err == nil {
					l.app.viewer.SetDocument(doc)
					l.app.viewer.Relayout()
					_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
				}
			}
		}
	}()
}

// OpenFile loads a document and displays it. Supports .md, .html, .htm, .zim, and virtual:menu.
func (l *DocumentLoader) OpenFile(pathStr string) error {
	app := l.app
	var absPath string
	var isZIM bool
	var doc *document.Document
	var err error

	if strings.HasPrefix(pathStr, "virtual:") {
		absPath = pathStr
		l.shutdown()

		var generator VirtualPageGenerator
		for prefix, gen := range l.virtualPages {
			if strings.HasPrefix(pathStr, prefix) {
				generator = gen
				break
			}
		}

		if generator == nil {
			return fmt.Errorf("unknown virtual page: %s", pathStr)
		}

		doc, err = generator(pathStr, l)
		if err != nil {
			return err
		}
		l.docCache[absPath] = doc
	} else {
		absPath, err = filepath.Abs(pathStr)
		if err != nil {
			absPath = pathStr
		}

		isZIM = strings.ToLower(filepath.Ext(absPath)) == ".zim"
		if !isZIM {
			l.shutdown()
		}

		var ok bool
		doc, ok = l.docCache[absPath]
		if !ok || (isZIM && l.zimReader == nil) {
			if isZIM {
				var zr *zim.Reader
				l.shutdown()
				zr, doc, err = storage.OpenZIM(absPath)
				if err != nil {
					return err
				}
				l.zimReader = zr
			} else {
				doc, err = storage.OpenFile(absPath)
				if err != nil {
					return err
				}
			}
			l.docCache[absPath] = doc
		}
	}

	app.mode = modeDoc
	app.viewer.SetResourceLoader(func(rawURL string) ([]byte, error) {
		if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
			client := storage.HTTPClient(10 * time.Second)
			resp, err := client.Get(rawURL)
			if err != nil {
				return nil, err
			}
			defer func() { _ = resp.Body.Close() }()
			return io.ReadAll(resp.Body)
		}
		if l.zimReader != nil {
			data, _, err := l.zimReader.ResolveResource(rawURL)
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

	app.navigator.UpdateCurrentState(document.ViewState{
		ScrollY:      app.scroller.CurrentScrollY(),
		SelectedLink: app.links.SelectedLinkIndex(),
	})
	app.viewer.SetDocument(doc)
	if isZIM && l.zimReader != nil {
		mainPath := l.zimReader.MainPagePath()
		navKey := "zim:" + mainPath
		l.docCache[navKey] = doc
		app.navigator.Open(navKey)
	} else {
		app.navigator.Open(absPath)
	}

	hasTree := l.zimReader != nil && l.zimReader.ArticleCount() > 1
	app.viewer.SetHasTree(hasTree)

	app.viewer.Relayout()
	slog.Info("Successfully loaded document", "path", pathStr, "isZIM", isZIM)
	return nil
}

func (l *DocumentLoader) NavigateLink(url string) {
	app := l.app
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "//") {
		slog.Info("Opening external URL", "url", url)
		_ = exec.Command("xdg-open", url).Start()
		return
	}
	if strings.HasPrefix(url, "virtual:delete?file=") {
		u, err := neturl.Parse(url)
		if err == nil {
			filename := u.Query().Get("file")
			if filename != "" {
				slog.Info("Deleting file", "filename", filename)
				if err := os.Remove(filename); err != nil {
					slog.Error("Failed to delete file", "filename", filename, "error", err)
				}
				if app.navigator.Current() == "virtual:menu" {
					_ = l.OpenFile("virtual:menu")
					_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
				}
			}
		}
		return
	}
	if strings.HasPrefix(url, "virtual:settings?") {
		u, err := neturl.Parse(url)
		if err == nil {
			app.HandleSettingsAction(u)
		}
		return
	}
	if strings.HasPrefix(url, "#") {
		anchor := url[1:]
		if y, ok := app.viewer.FindAnchorY(anchor); ok {
			app.scroller.ScrollToY(y)
		} else {
			slog.Warn("Anchor not found", "anchor", url)
		}
		return
	}

	if l.zimReader != nil && !strings.HasPrefix(url, "virtual:") {
		var referrer string
		current := app.navigator.Current()
		if strings.HasPrefix(current, "zim:") {
			referrer = strings.TrimPrefix(current, "zim:")
		}
		data, mime, err := l.zimReader.ResolveArticle(url, referrer)
		if err == nil {
			if !strings.HasPrefix(mime, "text/html") {
				slog.Warn("Unsupported article mime", "mime", mime)
				return
			}
			doc, err := html.Parse(bytes.NewReader(data))
			if err != nil {
				slog.Error("Parse error", "error", err)
				return
			}

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
			l.docCache[navKey] = doc
			app.mode = modeDoc
			app.navigator.UpdateCurrentState(document.ViewState{
				ScrollY:      app.scroller.CurrentScrollY(),
				SelectedLink: app.links.SelectedLinkIndex(),
			})
			app.viewer.SetDocument(doc)
			app.navigator.Open(navKey)
			app.viewer.Relayout()
			slog.Info("Navigated to article", "url", navKey)
			return
		}
		slog.Error("ResolveArticle failed", "url", url, "error", err)
		return
	}
	if err := l.OpenFile(url); err != nil {
		slog.Error("Cannot open file", "url", url, "error", err)
	} else {
		slog.Info("Opened file link", "url", url)
	}
}

func (l *DocumentLoader) startDownload(downloadURL, filename string) {
	app := l.app
	slog.Info("Initiating download", "url", downloadURL, "filename", filename)
	go func() {
		err := storage.Download(downloadURL, filename, func(status string) {
			app.viewer.SetStatusOverride(status)
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
		})
		if err != nil {
			slog.Error("Download failed", "url", downloadURL, "filename", filename, "error", err)
			app.viewer.SetStatusOverride("❌ Download failed: " + err.Error())
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			return
		}
		slog.Info("Download completed successfully", "filename", filename)
		time.Sleep(3 * time.Second)
		app.viewer.SetStatusOverride("")
		current := app.navigator.Current()
		if current == "virtual:menu" || strings.HasPrefix(current, "virtual:library/download") {
			_ = l.OpenFile("virtual:menu")
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
		}
	}()
}
