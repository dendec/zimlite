package menu

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/markdown"
	"github.com/dendec/zimlite/internal/storage"
)

// AtomLink represents a link element in an Atom entry or feed.
type AtomLink struct {
	Rel    string `xml:"rel,attr"`
	Href   string `xml:"href,attr"`
	Type   string `xml:"type,attr"`
	Title  string `xml:"title,attr"`
	Length int64  `xml:"length,attr"`
}

// AtomEntry represents a single entry in an Atom/OPDS catalog feed.
type AtomEntry struct {
	Title    string     `xml:"title"`
	Language string     `xml:"http://purl.org/dc/terms/ language"`
	Count    int        `xml:"http://purl.org/syndication/thread/1.0 count"`
	Summary  string     `xml:"summary"`
	Category string     `xml:"category"`
	Links    []AtomLink `xml:"link"`
}

// AtomFeed represents the root feed element of an Atom/OPDS catalog.
type AtomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Links   []AtomLink  `xml:"link"`
	Entries []AtomEntry `xml:"entry"`
}

func renderErrorDoc(section string, err error) (*document.Document, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# ❌ Error loading %s\n\n", section)
	fmt.Fprintf(&sb, "An error occurred while communicating with the Kiwix library catalog:\n\n`%v`\n\n", err)
	sb.WriteString("[🔙 Back to Menu](virtual:menu)\n")
	return markdown.Parse(strings.NewReader(sb.String()))
}

func fetchFeed(urlStr string) (*AtomFeed, error) {
	client := storage.HTTPClient(5 * time.Second)
	resp, err := client.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var feed AtomFeed
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

func LibraryLanguagesPage() (*document.Document, error) {
	feed, err := fetchFeed("https://browse.library.kiwix.org/catalog/v2/languages")
	if err != nil {
		return renderErrorDoc("languages", err)
	}

	for i := range feed.Entries {
		title := feed.Entries[i].Title
		if title == "" {
			continue
		}
		r, size := utf8.DecodeRuneInString(title)
		if r != utf8.RuneError {
			feed.Entries[i].Title = string(unicode.ToUpper(r)) + title[size:]
		}
	}

	sort.Slice(feed.Entries, func(i, j int) bool {
		if feed.Entries[i].Count != feed.Entries[j].Count {
			return feed.Entries[i].Count > feed.Entries[j].Count
		}
		return feed.Entries[i].Title < feed.Entries[j].Title
	})

	var buf bytes.Buffer
	if err := libraryLanguagesTmpl.Execute(&buf, feed); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return markdown.Parse(&buf)
}

type LibraryCategoriesData struct {
	Language     string
	LanguageName string
	Categories   []LibraryCategory
}

type LibraryCategory struct {
	Title    string
	Category string
}

func LibraryCategoriesPage(lang, name string) (*document.Document, error) {
	categories, err := fetchFeed("https://browse.library.kiwix.org/catalog/v2/categories")
	if err != nil {
		return renderErrorDoc("categories", err)
	}

	data := LibraryCategoriesData{
		Language:     lang,
		LanguageName: name,
	}

	hiddenCategories := map[string]bool{
		"phet": true, // interactive JS simulations
		"ted":  true, // video content
		"mooc": true, // video courses and interactive tests
	}

	for _, entry := range categories.Entries {
		categoryID := strings.ToLower(entry.Title)
		if hiddenCategories[categoryID] {
			continue
		}

		data.Categories = append(data.Categories, LibraryCategory{
			Title:    entry.Title,
			Category: categoryID,
		})
	}

	var buf bytes.Buffer
	if err := libraryCategoriesTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return markdown.Parse(&buf)
}

type LibraryEntriesData struct {
	Language     string
	LanguageName string
	Category     string
	Page         int
	PrevPage     int
	NextPage     int
	HasNextPage  bool
	Entries      []LibraryEntry
}

type LibraryEntry struct {
	Title       string
	Summary     string
	Size        string
	Filename    string
	DownloadURL string
}

func LibraryEntriesPage(lang, name, category string, page int) (*document.Document, error) {
	if page < 0 {
		page = 0
	}
	start := page * 50
	feed, err := fetchFeed(fmt.Sprintf("https://browse.library.kiwix.org/catalog/v2/entries?start=%d&count=50&lang=%s&category=%s", start, lang, category))
	if err != nil {
		return renderErrorDoc("archives", err)
	}

	data := LibraryEntriesData{
		Language:     lang,
		LanguageName: name,
		Category:     category,
		Page:         page,
		PrevPage:     page - 1,
		NextPage:     page + 1,
	}

	for _, entry := range feed.Entries {
		var downloadURL string
		var sizeBytes int64
		for _, link := range entry.Links {
			if link.Rel == "http://opds-spec.org/acquisition/open-access" && link.Type == "application/x-zim" {
				downloadURL = link.Href
				sizeBytes = link.Length
			}
		}
		if downloadURL == "" {
			continue
		}
		sizeStr := storage.FormatSize(sizeBytes)
		directURL := strings.Replace(downloadURL, ".zim.meta4", ".zim", 1)
		uDirect, _ := url.Parse(directURL)
		filename := entry.Title + ".zim"
		if uDirect != nil {
			filename = path.Base(uDirect.Path)
		}

		data.Entries = append(data.Entries, LibraryEntry{
			Title:       entry.Title,
			Summary:     entry.Summary,
			Size:        sizeStr,
			Filename:    filename,
			DownloadURL: directURL,
		})
	}

	if len(feed.Entries) == 50 {
		data.HasNextPage = true
	}

	var buf bytes.Buffer
	if err := libraryEntriesTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	slog.Debug("Generated library menu", "content", buf.String())
	return markdown.Parse(&buf)
}
