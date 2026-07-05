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

var tmpl = template.Must(template.New("menu").Funcs(template.FuncMap{
	"urlquery": url.QueryEscape,
}).Parse(menuTemplate))

var helpKeyboardTmpl = template.Must(template.New("help_keyboard").Parse(helpKeyboardTemplate))
var helpGamepadTmpl = template.Must(template.New("help_gamepad").Parse(helpGamepadTemplate))
var settingsTmpl = template.Must(template.New("settings").Parse(settingsTemplate))

// FileEntry represents a file in the menu with formatted details.
type FileEntry struct {
	Name        string
	DisplayName string
	Size        string
}

// MenuData holds the data for the menu template.
type MenuData struct {
	InternetAvailable bool
	ZIMs              []FileEntry
	Docs              []FileEntry
	HasContent        bool
}

var (
	zimExtensions = []string{".zim"}
	docExtensions = []string{".md", ".html", ".htm"}
)

// FileSelector generates the main file browsing document.
func FileSelector(internetAvailable bool) (*document.Document, error) {
	zims, mds, err := scanDirectory(".")
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	data := MenuData{
		InternetAvailable: internetAvailable,
		ZIMs:              zims,
		Docs:              mds,
		HasContent:        len(zims) > 0 || len(mds) > 0,
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
func SettingsPage() (*document.Document, error) {
	var buf bytes.Buffer
	cfg := config.Get()
	if err := settingsTmpl.Execute(&buf, cfg); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return markdown.Parse(&buf)
}

func scanDirectory(dir string) (zims []FileEntry, mds []FileEntry, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

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
			DisplayName: truncateName(name, 45),
			Size:        sizeStr,
		}

		if slices.Contains(zimExtensions, ext) {
			zims = append(zims, fe)
		} else if slices.Contains(docExtensions, ext) {
			mds = append(mds, fe)
		}
	}
	return zims, mds, nil
}

func truncateName(name string, maxLen int) string {
	if len(name) > maxLen {
		return name[:maxLen-3] + "..."
	}
	return name
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
