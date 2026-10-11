package ui

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/dendec/zimlite/internal/article"
	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/html"
	"github.com/dendec/zimlite/internal/i18n"
	"github.com/dendec/zimlite/internal/profile"
	"github.com/dendec/zimlite/internal/zim"
)

type articleLocation struct {
	File, Root, Entry string
	ZIM               bool
}

func (app *App) showArticle(source article.Source, entry, navKey string) error {
	viewer, ok := app.viewer.(ArticleViewer)
	if !ok {
		_ = source.Close()
		return errors.New("article viewer unavailable")
	}
	app.saveCurrentState()
	if err := viewer.OpenArticle(source, entry); err != nil {
		return err
	}
	app.mode = modeDoc
	app.navigator.Open(navKey)
	return nil
}
func (l *DocumentLoader) cacheDocument(key string, doc *document.Document) {
	if l.docCache == nil {
		l.docCache = make(map[string]*document.Document)
	}
	for i, old := range l.cacheKeys {
		if old == key {
			l.cacheKeys = append(l.cacheKeys[:i], l.cacheKeys[i+1:]...)
			break
		}
	}
	l.cacheKeys = append(l.cacheKeys, key)
	l.docCache[key] = doc
	for len(l.cacheKeys) > 4 {
		delete(l.docCache, l.cacheKeys[0])
		l.cacheKeys = l.cacheKeys[1:]
	}
}
func (l *DocumentLoader) rememberLocation(key string, location articleLocation) {
	if l.locations == nil {
		l.locations = make(map[string]articleLocation)
	}
	for i, old := range l.locationKeys {
		if old == key {
			l.locationKeys = append(l.locationKeys[:i], l.locationKeys[i+1:]...)
			break
		}
	}
	l.locationKeys = append(l.locationKeys, key)
	l.locations[key] = location
	for len(l.locationKeys) > 64 {
		delete(l.locations, l.locationKeys[0])
		l.locationKeys = l.locationKeys[1:]
	}
}
func (l *DocumentLoader) locationKey(location articleLocation) string {
	if !location.ZIM {
		return location.File
	}
	return "zim:" + url.PathEscape(location.File) + "?article=" + url.QueryEscape(location.Entry)
}
func (l *DocumentLoader) ensureArchive(filename string) error {
	if l.zimReader != nil && l.zimReader.FilePath() == filename {
		return nil
	}
	l.shutdown()
	reader, err := zim.Open(filename)
	if err != nil {
		return err
	}
	l.zimReader = reader
	return nil
}
func (l *DocumentLoader) openArticleFile(filename string) error {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	location := articleLocation{File: abs, Root: filepath.Dir(abs), Entry: filepath.Base(abs), ZIM: strings.EqualFold(filepath.Ext(abs), ".zim")}
	if location.ZIM {
		if err := l.ensureArchive(abs); err != nil {
			return err
		}
		location.Entry = l.zimReader.MainPagePath()
	} else {
		l.shutdown()
	}
	return l.displayArticle(location, false)
}
func (l *DocumentLoader) displayArticle(location articleLocation, forceNative bool) error {
	started := time.Now()
	profile.Mark("article.open.begin", map[string]any{"file": location.File, "entry": location.Entry, "zim": location.ZIM, "force_native": forceNative})
	var source article.Source
	var err error
	if location.ZIM {
		if err = l.ensureArchive(location.File); err != nil {
			return err
		}
		source, _, err = article.OpenZIMSource(location.File)
	} else {
		if l.zimReader != nil {
			l.shutdown()
		}
		source, err = article.OpenDirectorySource(location.Root)
	}
	if err != nil {
		return err
	}
	// Preflight document size/MIME and canonical redirects before starting a helper.
	profile.Mark("article.source.ready", map[string]any{"elapsed_ms": time.Since(started).Milliseconds()})
	var resource article.Resource
	for redirects := 0; redirects < 16; redirects++ {
		profile.Mark("article.preflight.begin", map[string]any{"entry": location.Entry})
		resource, err = source.ReadResource(location.Entry)
		profile.Mark("article.preflight.end", map[string]any{"bytes": len(resource.Data), "mime": resource.MIME, "elapsed_ms": time.Since(started).Milliseconds(), "success": err == nil})
		if err != nil {
			_ = source.Close()
			return err
		}
		if resource.CanonicalPath == "" || resource.CanonicalPath == location.Entry {
			break
		}
		location.Entry = resource.CanonicalPath
		if redirects == 15 {
			_ = source.Close()
			return errors.New("article redirect limit exceeded")
		}
	}
	if !strings.HasPrefix(resource.MIME, "text/html") {
		_ = source.Close()
		return fmt.Errorf("unsupported article MIME: %s", resource.MIME)
	}
	key := l.locationKey(location)
	l.rememberLocation(key, location)
	l.host.getViewer().SetHasTree(location.ZIM && l.zimReader.ArticleCount() > 1)
	var fallback error
	if !forceNative && l.host.getConfig().Get().ArticleRenderer == config.ArticleRendererServo {
		fallback = l.host.showArticle(source, location.Entry, key)
		if fallback == nil {
			return nil
		}
		// showArticle consumes and closes Source on startup failure.
	} else {
		_ = source.Close()
	}
	doc, err := html.Parse(bytes.NewReader(resource.Data))
	if err != nil {
		return err
	}
	l.cacheDocument(key, doc)
	l.host.showDocument(doc, key)
	if fallback != nil {
		slog.Warn("Servo unavailable, using SDL", "error", fallback)
		l.host.getViewer().SetStatusOverride(i18n.Tf(l.uiLang(), "servo.fallback", fallback.Error()))
	}
	return nil
}
func (l *DocumentLoader) restoreArticle(key string, forceNative bool) error {
	if location, ok := l.locations[key]; ok {
		return l.displayArticle(location, forceNative)
	}
	return l.OpenFile(key)
}
func (l *DocumentLoader) navigateServoURL(raw string) bool {
	viewer, ok := l.host.getViewer().(ArticleViewer)
	if !ok || !viewer.ArticleActive() {
		return false
	}
	entry, fragment, local := viewer.ResolveArticleURL(raw)
	if !local {
		return false
	}
	location, ok := l.locations[l.host.getNavigator().Current()]
	if !ok {
		return false
	}
	if entry == location.Entry {
		if fragment != "" {
			viewer.ScrollToAnchor(fragment)
		}
		return true
	}
	if !location.ZIM {
		location.File = filepath.Join(location.Root, filepath.FromSlash(entry))
	}
	location.Entry = entry
	if err := l.displayArticle(location, false); err != nil {
		slog.Error("Cannot open Servo link", "error", err)
		l.host.getViewer().SetStatusOverride(err.Error())
		return true
	}
	if fragment != "" {
		l.scrollToAnchor(fragment)
	}
	return true
}
func (app *App) pollArticle() {
	viewer, ok := app.viewer.(ArticleViewer)
	if !ok {
		return
	}
	if err := viewer.ArticleError(); err != nil {
		profile.Mark("article.fallback", map[string]any{"error": err.Error()})
		state := document.ViewState{ScrollY: app.scroller.CurrentScrollY(), SelectedLink: app.links.SelectedLinkIndex()}
		viewer.StopArticle()
		if fallbackErr := app.loader.restoreArticle(app.navigator.Current(), true); fallbackErr != nil {
			slog.Error("Cannot restore SDL article", "error", fallbackErr)
		} else {
			app.scroller.SetScrollY(state.ScrollY)
			app.links.SetSelectedLinkIndex(state.SelectedLink)
		}
		app.viewer.SetStatusOverride(i18n.Tf(app.config.Get().Language, "servo.fallback", err.Error()))
		return
	}
	if link := viewer.PollArticleNavigation(); link != "" {
		app.loader.NavigateLink(link)
	}
}
