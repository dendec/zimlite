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
	"strings"
	"text/template"
	"time"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
)

//go:embed assets/menu.md.tmpl
var menuTemplate string

var tmpl = template.Must(template.New("menu").Funcs(template.FuncMap{
	"urlquery": url.QueryEscape,
}).Parse(menuTemplate))

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

		if !contains(zimExtensions, ext) && !contains(docExtensions, ext) {
			continue
		}

		var sizeStr string
		if info, err := entry.Info(); err == nil {
			sizeStr = formatSize(info.Size())
		} else {
			sizeStr = "Unknown"
		}

		fe := FileEntry{
			Name:        name,
			DisplayName: truncateName(name, 45),
			Size:        sizeStr,
		}

		if contains(zimExtensions, ext) {
			zims = append(zims, fe)
		} else if contains(docExtensions, ext) {
			mds = append(mds, fe)
		}
	}
	return zims, mds, nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), []string{"KB", "MB", "GB", "TB"}[exp])
}

func truncateName(name string, maxLen int) string {
	if len(name) > maxLen {
		return name[:maxLen-3] + "..."
	}
	return name
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// CheckInternet pings the Kiwix library catalog and returns true if reachable.
func CheckInternet() bool {
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://browse.library.kiwix.org/catalog/v2/languages")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
