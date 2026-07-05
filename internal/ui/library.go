package ui

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
	"github.com/kiwix-sdl/kiwix-sdl/internal/menu"
	"github.com/kiwix-sdl/kiwix-sdl/internal/storage"
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

// NextPage returns the href of the rel="next" link, or "" if this is the last page.
func (f *AtomFeed) NextPage() string {
	for _, l := range f.Links {
		if l.Rel == "next" {
			return l.Href
		}
	}
	return ""
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

func (l *DocumentLoader) generateLibraryDoc(pathStr string) (*document.Document, error) {
	u, err := url.Parse(strings.Replace(pathStr, "virtual:", "http://localhost/", 1))
	if err != nil {
		return nil, err
	}

	switch u.Path {
	case "/library":
		feed, err := fetchFeed("https://browse.library.kiwix.org/catalog/v2/languages")
		if err != nil {
			return renderErrorDoc("languages", err)
		}
		var sb strings.Builder
		sb.WriteString("# 🌐 Kiwix Online Library - Languages\n\n")
		sb.WriteString("👇 Select a language to browse categories:\n\n")
		sb.WriteString("[🔙 Back to Files](virtual:menu)\n\n")
		sort.Slice(feed.Entries, func(i, j int) bool {
			if feed.Entries[i].Count != feed.Entries[j].Count {
				return feed.Entries[i].Count > feed.Entries[j].Count
			}
			return feed.Entries[i].Title < feed.Entries[j].Title
		})
		for _, entry := range feed.Entries {
			if entry.Language != "" {
				fmt.Fprintf(&sb, "* [%s (%d archives)](virtual:library/categories?lang=%s)\n", entry.Title, entry.Count, entry.Language)
			}
		}
		return markdown.Parse(strings.NewReader(sb.String()))

	case "/library/categories":
		lang := u.Query().Get("lang")
		categories, err := fetchFeed("https://browse.library.kiwix.org/catalog/v2/categories")
		if err != nil {
			return renderErrorDoc("categories", err)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "# 🌐 Kiwix Online Library - Categories (%s)\n\n", lang)
		sb.WriteString("👇 Select a category:\n\n")
		sb.WriteString("[🔙 Back to Languages](virtual:library)\n\n")
		for _, entry := range categories.Entries {
			category := strings.ToLower(entry.Title)
			fmt.Fprintf(&sb, "* [%s](virtual:library/entries?lang=%s&category=%s)\n", entry.Title, lang, category)
		}
		return markdown.Parse(strings.NewReader(sb.String()))

	case "/library/entries":
		lang := u.Query().Get("lang")
		category := u.Query().Get("category")
		page, _ := strconv.Atoi(u.Query().Get("page"))
		if page < 0 {
			page = 0
		}
		start := page * 50
		feed, err := fetchFeed(fmt.Sprintf("https://browse.library.kiwix.org/catalog/v2/entries?start=%d&count=50&lang=%s&category=%s", start, lang, category))
		if err != nil {
			return renderErrorDoc("archives", err)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "# 🌐 Kiwix Online Library - ZIM Archives (%s / %s)\n\n", lang, category)
		fmt.Fprintf(&sb, "[🔙 Back to Categories](virtual:library/categories?lang=%s)\n\n", lang)
		if len(feed.Entries) == 0 {
			if page > 0 {
				sb.WriteString("*📭 No more archives on this page.*\n")
			} else {
				sb.WriteString("*📭 No archives found in this language and category.*\n")
			}
		} else {
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
				fmt.Fprintf(&sb, "### %s\n\n", entry.Title)
				if entry.Summary != "" {
					fmt.Fprintf(&sb, "*📝 Description*: %s\n", entry.Summary)
				}
				fmt.Fprintf(&sb, "*💾 Size*: %s\n", sizeStr)
				escURL := url.QueryEscape(directURL)
				escFile := url.QueryEscape(filename)
				label := "⬇ Download " + filename
				fmt.Fprintf(&sb, "[%s](virtual:library/download?url=%s&filename=%s)\n", label, escURL, escFile)
				sb.WriteString("\n---\n\n")
			}
			// Pagination nav.
			if page > 0 {
				fmt.Fprintf(&sb, "[◀ Previous Page](virtual:library/entries?lang=%s&category=%s&page=%d)  ", lang, category, page-1)
			}
			if len(feed.Entries) == 50 {
				fmt.Fprintf(&sb, "[Next Page ▶](virtual:library/entries?lang=%s&category=%s&page=%d)", lang, category, page+1)
			}
			sb.WriteString("\n")
		}
		slog.Debug("Generated library menu", "content", sb.String())
		return markdown.Parse(strings.NewReader(sb.String()))

	case "/library/download":
		downloadURL := u.Query().Get("url")
		filename := u.Query().Get("filename")
		if downloadURL != "" && filename != "" {
			l.startDownload(downloadURL, filename)
			return menu.FileSelector(l.internetAvailable.Load())
		}
	}

	return nil, fmt.Errorf("unknown library path: %s", pathStr)
}
