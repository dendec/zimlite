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

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/html"
	"github.com/dendec/zimlite/internal/menu"
	"github.com/dendec/zimlite/internal/storage"
	"github.com/dendec/zimlite/internal/util"
	"github.com/dendec/zimlite/internal/zim"
	"github.com/veandco/go-sdl2/sdl"
)

type VirtualPageGenerator func(path string, loader *DocumentLoader) (*document.Document, error)

type DocumentLoader struct {
	host              LoaderHost
	docCache          map[string]*document.Document
	zimReader         ZimReader
	internetAvailable atomic.Bool
	pendingMenuReload atomic.Bool
	pendingDownload   atomic.Bool
	downloadFilename  string // set by goroutine before pendingDownload.Store(true)
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
		return menu.SettingsPage(l.host.getConfig().Get())
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
			l.pendingMenuReload.Store(true)
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
		}
	}()
}

// applyPendingMenuReload runs on the main SDL thread when the internet check
// completes. It must NOT be called from background goroutines.
func (l *DocumentLoader) applyPendingMenuReload() {
	if !l.pendingMenuReload.CompareAndSwap(true, false) {
		return
	}
	if l.host.getNavigator().Current() != "virtual:menu" {
		return
	}
	doc, err := menu.FileSelector(l.internetAvailable.Load())
	if err != nil {
		slog.Error("Failed to generate menu after internet check", "error", err)
		return
	}
	l.host.ReloadCurrentDocument(doc)
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
	case strings.HasPrefix(url, "virtual:download/start?file="):
		l.handleDownloadStart(url)
	case strings.HasPrefix(url, "virtual:download/stop?file="):
		l.handleDownloadStop(url)
	case strings.HasPrefix(url, "virtual:download/delete?file="):
		l.handleDownloadDelete(url)
	case strings.HasPrefix(url, "virtual:library/download?"):
		l.handleLibraryDownload(url)
	default:
		l.openFileOrFallback(url)
	}
}

func (l *DocumentLoader) openExternalURL(url string) {
	slog.Info("Opening external URL", "url", url)
	if err := exec.Command("xdg-open", url).Start(); err != nil {
		slog.Error("Failed to open external URL", "url", url, "error", err)
	}
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
		l.pendingMenuReload.Store(true)
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
	}
}

func (l *DocumentLoader) handleDownloadStart(url string) {
	u, err := neturl.Parse(url)
	if err != nil {
		return
	}
	filename := u.Query().Get("file")
	if filename == "" {
		return
	}
	l.startDownload("", filename)

	if l.host.getNavigator().Current() == "virtual:menu" {
		l.pendingMenuReload.Store(true)
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
	}
}

func (l *DocumentLoader) handleDownloadStop(url string) {
	u, err := neturl.Parse(url)
	if err != nil {
		return
	}
	filename := u.Query().Get("file")
	if filename == "" {
		return
	}
	storage.Manager.Stop(filename)

	if l.host.getNavigator().Current() == "virtual:menu" {
		l.pendingMenuReload.Store(true)
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
	}
}

func (l *DocumentLoader) handleDownloadDelete(url string) {
	u, err := neturl.Parse(url)
	if err != nil {
		return
	}
	filename := u.Query().Get("file")
	if filename == "" {
		return
	}
	storage.Manager.Stop(filename)

	_ = os.Remove(filename + ".part")
	_ = os.Remove(filename + ".info")

	if l.host.getNavigator().Current() == "virtual:menu" {
		l.pendingMenuReload.Store(true)
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
	}
}

func (l *DocumentLoader) handleLibraryDownload(urlStr string) {
	u, err := neturl.Parse(urlStr)
	if err != nil {
		return
	}
	downloadURL := u.Query().Get("url")
	filename := u.Query().Get("filename")
	if downloadURL != "" && filename != "" {
		l.startDownload(downloadURL, filename)
		// Small delay to ensure the .part file is created on disk
		time.Sleep(100 * time.Millisecond)
		// We go to virtual:menu so the user can see the download
		if err := l.OpenFile("virtual:menu"); err != nil {
			slog.Error("Cannot open menu", "error", err)
		}
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
	slog.Info("Initiating download", "url", downloadURL, "filename", filename)

	// Show immediate feedback
	viewer.SetStatusOverride("⏳ Connecting... " + util.Truncate(filename, 40))
	_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})

	// Pre-register in the manager so the UI instantly shows "Stop" button
	storage.Manager.Add(filename, func() {})

	// Clear menu from cache so it reloads and shows the new .part file when navigated back
	delete(l.docCache, "virtual:menu")

	go func() {
		defer storage.Manager.Remove(filename) // Cleanup in case Download fails early
		var lastMenuRefresh time.Time
		var err error

		for attempt := 1; attempt <= 5; attempt++ {
			err = storage.Download(downloadURL, filename, func(status string) {
				viewer.SetStatusOverride(status)

				// Refresh menu at most once per 2 seconds to avoid excessive layout calls
				if time.Since(lastMenuRefresh) > 2*time.Second {
					l.pendingMenuReload.Store(true)
					lastMenuRefresh = time.Now()
				}

				_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			})

			if err == nil || !strings.Contains(err.Error(), "idle timeout") {
				break
			}

			slog.Warn("Download timed out, retrying...", "filename", filename, "attempt", attempt)
			viewer.SetStatusOverride(fmt.Sprintf("⏳ Connection lost. Retrying... (%d/5)", attempt))
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
			time.Sleep(2 * time.Second)
		}

		if err != nil {
			if strings.Contains(err.Error(), "context canceled") {
				slog.Info("Download stopped by user", "filename", filename)
				msg := "🛑 Download stopped"
				viewer.SetStatusOverride(msg)
				l.pendingMenuReload.Store(true) // Ensure UI updates back to "Start" button
				_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})

				go func() {
					time.Sleep(10 * time.Second)
					if viewer.GetStatusOverride() == msg {
						viewer.SetStatusOverride("")
						_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
					}
				}()
				return
			}

			slog.Error("Download failed", "url", downloadURL, "filename", filename, "error", err)

			errStr := err.Error()
			if strings.HasPrefix(errStr, "download: Get \"") {
				if idx := strings.Index(errStr, "\": "); idx != -1 {
					errStr = errStr[idx+3:]
				}
			}

			errMsg := util.Truncate(errStr, 60)
			msg := "❌ Download failed: " + errMsg
			viewer.SetStatusOverride(msg)
			l.pendingMenuReload.Store(true) // Ensure UI updates back to "Start" button
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})

			go func() {
				time.Sleep(10 * time.Second)
				if viewer.GetStatusOverride() == msg {
					viewer.SetStatusOverride("")
					_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
				}
			}()
			return
		}

		slog.Info("Download completed successfully", "filename", filename)
		// Store filename then signal main thread. All SDL state (SetDocument,
		// Relayout, sleep, status clear) must happen on the main thread.
		l.downloadFilename = filename
		l.pendingDownload.Store(true)
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
	}()
}

// applyPendingDownloadCompletion runs on the main SDL thread after a download
// finishes. It must NOT be called from background goroutines.
func (l *DocumentLoader) applyPendingDownloadCompletion() {
	if !l.pendingDownload.CompareAndSwap(true, false) {
		return
	}
	time.Sleep(3 * time.Second)
	l.host.getViewer().SetStatusOverride("")
	current := l.host.getNavigator().Current()
	if current == "virtual:menu" || strings.HasPrefix(current, "virtual:library/download") {
		if err := l.OpenFile("virtual:menu"); err != nil {
			slog.Error("Failed to reload menu after download", "error", err)
		}
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: sdl.USEREVENT})
	}
}
