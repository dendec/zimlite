// Package menu generates virtual markdown documents for navigation menus.
package menu

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
)

// FileSelector generates the main file browsing document.
func FileSelector(internetAvailable bool) (*document.Document, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# Kiwix SDL Document Menu\n\n")
	sb.WriteString("Select a document or ZIM archive to open:\n\n")

	if internetAvailable {
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
			mds = append(mds, name)
		}
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
		sb.WriteString("*No documents or ZIM archives found in current directory.*\n")
	}

	return markdown.Parse(strings.NewReader(sb.String()))
}

// CheckInternet pings the Kiwix library catalog and returns true if reachable.
func CheckInternet() bool {
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://browse.library.kiwix.org/catalog/v2/languages")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
