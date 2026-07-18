package menu

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/i18n"
	"github.com/dendec/zimlite/internal/markdown"
	"github.com/dendec/zimlite/internal/storage"
)

//go:embed assets/menu.md.tmpl
var menuTemplate string

//go:embed assets/help_keyboard.md.tmpl
var helpKeyboardTemplate string

//go:embed assets/help_gamepad.md.tmpl
var helpGamepadTemplate string

//go:embed assets/settings.md.tmpl
var settingsTemplate string

//go:embed assets/library_languages.md.tmpl
var libraryLanguagesTemplate string

//go:embed assets/library_categories.md.tmpl
var libraryCategoriesTemplate string

//go:embed assets/library_entries.md.tmpl
var libraryEntriesTemplate string

// executeTemplate parses and executes a template text with i18n FuncMap.
func executeTemplate(tmplText string, lang string, data any) (*bytes.Buffer, error) {
	t, err := template.New("").Funcs(template.FuncMap{
		"T":        func(key string) string { return i18n.T(lang, key) },
		"Tf":       func(key string, args ...any) string { return i18n.Tf(lang, key, args...) },
		"urlquery": url.QueryEscape,
	}).Parse(tmplText)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return &buf, nil
}

// FileEntry represents a file in the menu with formatted details.
type FileEntry struct {
	Name        string
	DisplayName string
	Size        string
}

// DownloadEntry represents an active or paused download.
type DownloadEntry struct {
	Name        string
	DisplayName string
	Size        string
	Active      bool
}

// MenuData holds the data for the menu template.
type MenuData struct {
	InternetAvailable bool
	ZIMs              []FileEntry
	Docs              []FileEntry
	Downloads         []DownloadEntry
	HasContent        bool
}

var (
	zimExtensions = []string{".zim"}
	docExtensions = []string{".md", ".html", ".htm"}
)

func FileSelector(lang string, internetAvailable bool) (*document.Document, error) {
	zims, mds, downloads, err := scanDirectory(".")
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	data := MenuData{
		InternetAvailable: internetAvailable,
		ZIMs:              zims,
		Docs:              mds,
		Downloads:         downloads,
		HasContent:        len(zims) > 0 || len(downloads) > 0,
	}

	buf, err := executeTemplate(menuTemplate, lang, data)
	if err != nil {
		return nil, err
	}

	slog.Debug("Generated menu markdown", "markdown", buf.String())

	return markdown.Parse(buf)
}

// HelpPage generates the help and shortcuts document.
func HelpPage(lang string, hasGamepad bool) (*document.Document, error) {
	tmplText := helpKeyboardTemplate
	if hasGamepad {
		tmplText = helpGamepadTemplate
	}

	buf, err := executeTemplate(tmplText, lang, nil)
	if err != nil {
		return nil, err
	}

	return markdown.Parse(buf)
}

type settingsData struct {
	ThemeName string
	ThemePrev string
	ThemeNext string
	FontSize  int
	LangName  string
	LangPrev  string
	LangNext  string
}

var themeIDs = []string{"light", "dark", "sepia"}

// SettingsPage generates the settings document.
func SettingsPage(lang string, cfg config.Config) (*document.Document, error) {
	themeIdx := 0
	for i, id := range themeIDs {
		if id == cfg.Theme {
			themeIdx = i
			break
		}
	}
	n := len(themeIDs)

	langs := i18n.Languages()
	langIdx := 0
	for i, l := range langs {
		if l.Code == cfg.Language {
			langIdx = i
			break
		}
	}
	m := len(langs)

	langName := langs[langIdx].Name
	if langName == "" {
		langName = cfg.Language // fallback to raw ID if unknown
	}

	data := settingsData{
		ThemeName: i18n.T(lang, "theme."+themeIDs[themeIdx]),
		ThemePrev: themeIDs[(themeIdx-1+n)%n],
		ThemeNext: themeIDs[(themeIdx+1)%n],
		FontSize:  cfg.FontSize,
		LangName:  langName,
		LangPrev:  langs[(langIdx-1+m)%m].Code,
		LangNext:  langs[(langIdx+1)%m].Code,
	}

	buf, err := executeTemplate(settingsTemplate, lang, data)
	if err != nil {
		return nil, err
	}

	return markdown.Parse(buf)
}

func scanDirectory(dir string) (zims []FileEntry, mds []FileEntry, downloads []DownloadEntry, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		if ext == ".part" {
			baseName := strings.TrimSuffix(name, ".part")
			var downloaded int64
			if info, err := entry.Info(); err == nil {
				downloaded = info.Size()
			}

			var sizeStr string
			if dInfo, err := storage.LoadDownloadInfo(filepath.Join(dir, baseName+".info")); err == nil && dInfo.TotalSize > 0 {
				sizeStr = formatProgress(downloaded, dInfo.TotalSize)
			} else {
				sizeStr = storage.FormatSize(downloaded) + " / ?"
			}

			downloads = append(downloads, DownloadEntry{
				Name:        baseName,
				DisplayName: strings.TrimSuffix(baseName, ".zim"),
				Size:        sizeStr,
				Active:      storage.Manager.IsActive(baseName),
			})
			continue
		}

		if !slices.Contains(zimExtensions, ext) && !slices.Contains(docExtensions, ext) {
			continue
		}

		var sizeStr string
		if info, err := entry.Info(); err == nil {
			sizeStr = storage.FormatSize(info.Size())
		} else {
			sizeStr = "Unknown"
		}

		fe := FileEntry{
			Name:        name,
			DisplayName: strings.TrimSuffix(name, ".zim"),
			Size:        sizeStr,
		}

		if slices.Contains(zimExtensions, ext) {
			zims = append(zims, fe)
		} else if slices.Contains(docExtensions, ext) {
			mds = append(mds, fe)
		}
	}
	return zims, mds, downloads, nil
}

func formatProgress(current, total int64) string {
	return storage.FormatProgress(current, total)
}

// CheckInternet pings the Kiwix library catalog and returns true if reachable.
func CheckInternet() bool {
	client := storage.HTTPClient(4 * time.Second)
	resp, err := client.Get("https://browse.library.kiwix.org/catalog/v2/languages")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}
