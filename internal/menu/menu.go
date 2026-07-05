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

	"github.com/kiwix-sdl/kiwix-sdl/internal/config"
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
	"github.com/kiwix-sdl/kiwix-sdl/internal/storage"
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

var tmpl = template.Must(template.New("menu").Funcs(template.FuncMap{
	"urlquery": url.QueryEscape,
}).Parse(menuTemplate))

var helpKeyboardTmpl = template.Must(template.New("help_keyboard").Parse(helpKeyboardTemplate))
var helpGamepadTmpl = template.Must(template.New("help_gamepad").Parse(helpGamepadTemplate))
var settingsTmpl = template.Must(template.New("settings").Parse(settingsTemplate))
var libraryLanguagesTmpl = template.Must(template.New("library_languages").Funcs(template.FuncMap{"urlquery": url.QueryEscape}).Parse(libraryLanguagesTemplate))
var libraryCategoriesTmpl = template.Must(template.New("library_categories").Funcs(template.FuncMap{"urlquery": url.QueryEscape}).Parse(libraryCategoriesTemplate))
var libraryEntriesTmpl = template.Must(template.New("library_entries").Funcs(template.FuncMap{"urlquery": url.QueryEscape}).Parse(libraryEntriesTemplate))

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

func FileSelector(internetAvailable bool) (*document.Document, error) {
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

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	slog.Debug("Generated menu markdown", "markdown", buf.String())

	return markdown.Parse(&buf)
}

// HelpPage generates the help and shortcuts document.
func HelpPage(hasGamepad bool) (*document.Document, error) {
	var buf bytes.Buffer
	var err error
	if hasGamepad {
		err = helpGamepadTmpl.Execute(&buf, nil)
	} else {
		err = helpKeyboardTmpl.Execute(&buf, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return markdown.Parse(&buf)
}

// SettingsPage generates the settings document.
func SettingsPage(cfg config.Config) (*document.Document, error) {
	var buf bytes.Buffer
	if err := settingsTmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return markdown.Parse(&buf)
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
	const unit = 1024
	if total < unit {
		return fmt.Sprintf("%d / %d B", current, total)
	}
	div, exp := int64(unit), 0
	for n := total / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f / %.1f %s", float64(current)/float64(div), float64(total)/float64(div), []string{"kB", "MB", "GB", "TB"}[exp])
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
