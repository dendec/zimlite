package ui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kiwix-sdl/zimlite/internal/document"
	"github.com/kiwix-sdl/zimlite/internal/menu"
)

func (l *DocumentLoader) generateLibraryDoc(pathStr string) (*document.Document, error) {
	u, err := url.Parse(strings.Replace(pathStr, "virtual:", "http://localhost/", 1))
	if err != nil {
		return nil, err
	}

	switch u.Path {
	case "/library":
		return menu.LibraryLanguagesPage()
	case "/library/categories":
		return menu.LibraryCategoriesPage(u.Query().Get("lang"), u.Query().Get("name"))
	case "/library/entries":
		page, _ := strconv.Atoi(u.Query().Get("page"))
		return menu.LibraryEntriesPage(u.Query().Get("lang"), u.Query().Get("name"), u.Query().Get("category"), page)
	case "/library/download":
		return l.libraryDownloadPage(u)
	}

	return nil, fmt.Errorf("unknown library path: %s", pathStr)
}

func (l *DocumentLoader) libraryDownloadPage(u *url.URL) (*document.Document, error) {
	downloadURL := u.Query().Get("url")
	filename := u.Query().Get("filename")
	if downloadURL != "" && filename != "" {
		l.startDownload(downloadURL, filename)
		return menu.FileSelector(l.internetAvailable.Load())
	}
	return nil, fmt.Errorf("missing download parameters")
}
