package ui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/menu"
)

func (l *DocumentLoader) generateLibraryDoc(pathStr string) (*document.Document, error) {
	u, err := url.Parse(strings.Replace(pathStr, "virtual:", "http://localhost/", 1))
	if err != nil {
		return nil, err
	}

	lang := l.uiLang()

	switch u.Path {
	case "/library":
		return menu.LibraryLanguagesPage(lang)
	case "/library/categories":
		return menu.LibraryCategoriesPage(lang, u.Query().Get("lang"), u.Query().Get("name"))
	case "/library/entries":
		page, err := strconv.Atoi(u.Query().Get("page"))
		if err != nil || page < 0 {
			page = 0
		}
		return menu.LibraryEntriesPage(lang, u.Query().Get("lang"), u.Query().Get("name"), u.Query().Get("category"), page)
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
		return menu.FileSelector(l.uiLang(), l.internetAvailable.Load())
	}
	return nil, fmt.Errorf("missing download parameters")
}
