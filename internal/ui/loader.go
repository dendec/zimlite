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
	"github.com/kiwix-sdl/kiwix-sdl/internal/util"
	"github.com/kiwix-sdl/kiwix-sdl/internal/zim"
	"github.com/veandco/go-sdl2/sdl"
)

type VirtualPageGenerator func(path string, loader *DocumentLoader) (*document.Document, error)

type DocumentLoader struct {
	host              LoaderHost
	docCache          map[string]*document.Document
	zimReader         ZimReader
	internetAvailable atomic.Bool
	virtualPages      map[string]VirtualPageGenerator
}

func NewDocumentLoader(host LoaderHost) *DocumentLoader {
	l := &DocumentLoader{
		host:         host,
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
	l.host.navStateClear()
}

func (l *DocumentLoader) checkInternetAsync() {
	go func() {
		hasInternet := menu.CheckInternet()
		if hasInternet != l.internetAvailable.Load() {
			l.internetAvailable.Store(hasInternet)
			if l.host.getNavigator().Current() == "virtual:menu" {
				if doc, err := menu.FileSelector(hasInternet); err == nil {
					l.host.getViewer().SetDocument(doc)
					l.host.getViewer().Relayout()
					_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
				}
			}
		}
	}()
}

// OpenFile loads a document and displays it. Supports .md, .html, .htm, .zim, and virtual:menu.
func (l *DocumentLoader) OpenFile(pathStr string) error {
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

	if isZIM && l.zimReader != nil {
		mainPath := l.zimReader.MainPagePath()
		navKey := "zim:" + mainPath
		l.docCache[navKey] = doc
		l.host.getViewer().SetHasTree(l.zimReader.ArticleCount() > 1)
		l.host.showDocument(doc, navKey)
	} else {
		l.host.showDocument(doc, absPath)
	}

	slog.Info("Successfully loaded document", "path", pathStr, "isZIM", isZIM)
	return nil
}

// loadResource is the static resource loader installed once on the viewer. It
// always resolves relative paths against the current navigator location, so it
// remains correct across navigations without being recreated per OpenFile.
func (l *DocumentLoader) loadResource(rawURL string) ([]byte, error) {
	if util.IsExternalURL(rawURL) {
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
	docPath := l.host.getNavigator().Current()
	if docPath == "" || strings.HasPrefix(docPath, "virtual:") {
		return os.ReadFile(rawURL)
	}
	dir := filepath.Dir(docPath)
	fullPath := filepath.Join(dir, rawURL)
	return os.ReadFile(fullPath)
}

func (l *DocumentLoader) NavigateLink(url string) {
	switch {
	case util.IsExternalURL(url):
		l.openExternalURL(url)
	case strings.HasPrefix(url, "virtual:delete?file="):
		l.handleDeleteFile(url)
	case strings.HasPrefix(url, "virtual:settings?"):
		l.handleSettings(url)
	case strings.HasPrefix(url, "#"):
		l.scrollToAnchor(url[1:])
	case l.zimReader != nil && !strings.HasPrefix(url, "virtual:"):
		l.navigateZIMArticle(url)
	default:
		l.openFileOrFallback(url)
	}
}

func (l *DocumentLoader) openExternalURL(url string) {
	slog.Info("Opening external URL", "url", url)
	_ = exec.Command("xdg-open", url).Start()
}

func (l *DocumentLoader) handleDeleteFile(url string) {
	u, err := neturl.Parse(url)
	if err != nil {
		return
	}
	filename := u.Query().Get("file")
	if filename == "" {
		return
	}
	slog.Info("Deleting file", "filename", filename)
	if err := os.Remove(filename); err != nil {
		slog.Error("Failed to delete file", "filename", filename, "error", err)
	}
	if l.host.getNavigator().Current() == "virtual:menu" {
		_ = l.OpenFile("virtual:menu")
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
	}
}

func (l *DocumentLoader) handleSettings(url string) {
	u, err := neturl.Parse(url)
	if err == nil {
		l.host.HandleSettingsAction(u)
	}
}

func (l *DocumentLoader) scrollToAnchor(anchor string) {
	if y, ok := l.host.getViewer().FindAnchorY(anchor); ok {
		l.host.getScroller().ScrollToY(y)
	} else {
		slog.Warn("Anchor not found", "anchor", anchor)
	}
}

func (l *DocumentLoader) navigateZIMArticle(url string) {
	var referrer string
	current := l.host.getNavigator().Current()
	if strings.HasPrefix(current, "zim:") {
		referrer = strings.TrimPrefix(current, "zim:")
	}
	data, mime, err := l.zimReader.ResolveArticle(url, referrer)
	if err != nil {
		slog.Error("ResolveArticle failed", "url", url, "error", err)
		return
	}
	if !strings.HasPrefix(mime, "text/html") {
		slog.Warn("Unsupported article mime", "mime", mime)
		return
	}
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		slog.Error("Parse error", "error", err)
		return
	}
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
	l.host.showDocument(doc, navKey)
	slog.Info("Navigated to article", "url", navKey)
}

func (l *DocumentLoader) openFileOrFallback(url string) {
	if err := l.OpenFile(url); err != nil {
		slog.Error("Cannot open file", "url", url, "error", err)
	} else {
		slog.Info("Opened file link", "url", url)
	}
}

func (l *DocumentLoader) startDownload(downloadURL, filename string) {
	viewer := l.host.getViewer()
	navigator := l.host.getNavigator()
	slog.Info("Initiating download", "url", downloadURL, "filename", filename)
	go func() {
		err := storage.Download(downloadURL, filename, func(status string) {
			viewer.SetStatusOverride(status)
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
		})
		if err != nil {
			slog.Error("Download failed", "url", downloadURL, "filename", filename, "error", err)
			viewer.SetStatusOverride("❌ Download failed: " + err.Error())
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			return
		}
		slog.Info("Download completed successfully", "filename", filename)
		time.Sleep(3 * time.Second)
		viewer.SetStatusOverride("")
		current := navigator.Current()
		if current == "virtual:menu" || strings.HasPrefix(current, "virtual:library/download") {
			_ = l.OpenFile("virtual:menu")
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
		}
	}()
}
