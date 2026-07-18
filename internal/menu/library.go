package menu

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/i18n"
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

// AtomFeed represents the root feed element in an Atom/OPDS catalog.
type AtomFeed struct {
	XMLName      xml.Name    `xml:"feed"`
	Title        string      `xml:"title"`
	Links        []AtomLink  `xml:"link"`
	TotalResults int         `xml:"totalResults"`
	Entries      []AtomEntry `xml:"entry"`
}

func renderErrorDoc(lang, section string, err error) (*document.Document, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", i18n.Tf(lang, "library.error_loading", section))
	fmt.Fprint(&sb, i18n.Tf(lang, "library.error_message", err))
	sb.WriteString(i18n.T(lang, "library.back_to_menu"))
	sb.WriteString("\n")
	return markdown.Parse(strings.NewReader(sb.String()))
}

const maxFeedSize = 10 << 20 // 10 MiB

func fetchFeed(urlStr string) (*AtomFeed, error) {
	client := storage.HTTPClient(5 * time.Second)
	resp, err := client.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", urlStr, resp.StatusCode)
	}

	var feed AtomFeed
	dec := xml.NewDecoder(io.LimitReader(resp.Body, maxFeedSize))
	if err := dec.Decode(&feed); err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", urlStr, err)
	}
	return &feed, nil
}

// --- category count cache (disk-backed, 24h TTL) ---

type categoryCount struct {
	Count int   `json:"count"`
	TS    int64 `json:"ts"`
}

type categoryCache struct {
	path string
	ttl  time.Duration

	mu   sync.RWMutex
	data map[string]map[string]categoryCount // lang → category → {count, ts}
}

func newCategoryCache(dir string, ttl time.Duration) *categoryCache {
	return &categoryCache{
		path: filepath.Join(dir, "library_cache.json"),
		ttl:  ttl,
		data: make(map[string]map[string]categoryCount),
	}
}

func (c *categoryCache) load() {
	raw, err := os.ReadFile(c.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read library cache", "path", c.path, "error", err)
		}
		return
	}
	var loaded map[string]map[string]categoryCount
	if err := json.Unmarshal(raw, &loaded); err != nil {
		slog.Warn("Failed to parse library cache, ignoring", "path", c.path, "error", err)
		return
	}
	if loaded != nil {
		c.data = loaded
	}
}

func (c *categoryCache) save() {
	c.mu.RLock()
	snapshot := make(map[string]map[string]categoryCount, len(c.data))
	for lang, cats := range c.data {
		cp := make(map[string]categoryCount, len(cats))
		for k, v := range cats {
			cp[k] = v
		}
		snapshot[lang] = cp
	}
	c.mu.RUnlock()

	raw, err := json.Marshal(snapshot)
	if err != nil {
		slog.Warn("Failed to marshal library cache", "error", err)
		return
	}

	dir := filepath.Dir(c.path)
	tmp, err := os.CreateTemp(dir, "library_cache_*.tmp")
	if err != nil {
		slog.Warn("Failed to create cache tmp file", "dir", dir, "error", err)
		return
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // cleanup if rename already succeeded is harmless

	if _, err := tmp.Write(raw); err != nil {
		slog.Warn("Failed to write cache tmp", "path", tmpPath, "error", err)
		_ = tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		slog.Warn("Failed to close cache tmp", "path", tmpPath, "error", err)
		return
	}
	if err := os.Rename(tmpPath, c.path); err != nil {
		slog.Warn("Failed to rename cache file", "from", tmpPath, "to", c.path, "error", err)
		return
	}
	// rename succeeded — prevent deferred Remove
	tmpPath = ""
}

func (c *categoryCache) get(lang, category string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cats, ok := c.data[lang]
	if !ok {
		return 0, false
	}
	cc, ok := cats[category]
	if !ok || time.Since(time.Unix(cc.TS, 0)) > c.ttl {
		return 0, false
	}
	return cc.Count, true
}

func (c *categoryCache) set(lang, category string, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.data[lang] == nil {
		c.data[lang] = make(map[string]categoryCount)
	}
	c.data[lang][category] = categoryCount{Count: count, TS: time.Now().Unix()}
}

// singleton — created once from executable dir
var (
	libCache     *categoryCache
	libCacheOnce sync.Once
)

func getLibCache() *categoryCache {
	libCacheOnce.Do(func() {
		dir := "."
		if exe, err := os.Executable(); err == nil {
			dir = filepath.Dir(exe)
		}
		libCache = newCategoryCache(dir, 24*time.Hour)
		libCache.load()
	})
	return libCache
}

// --- pages ---

func LibraryLanguagesPage(lang string) (*document.Document, error) {
	feed, err := fetchFeed("https://browse.library.kiwix.org/catalog/v2/languages")
	if err != nil {
		return renderErrorDoc(lang, i18n.T(lang, "library.section_languages"), err)
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

	type langData struct {
		UILang string
		*AtomFeed
	}
	data := langData{UILang: lang, AtomFeed: feed}

	buf, err := executeTemplate(libraryLanguagesTemplate, lang, data)
	if err != nil {
		return nil, err
	}

	return markdown.Parse(buf)
}

type LibraryCategoriesData struct {
	UILang       string
	Language     string
	LanguageName string
	Categories   []LibraryCategory
}

type LibraryCategory struct {
	Title    string
	Category string
	Count    int
}

func LibraryCategoriesPage(lang, catalogLang, name string) (*document.Document, error) {
	categories, err := fetchFeed("https://browse.library.kiwix.org/catalog/v2/categories")
	if err != nil {
		return renderErrorDoc(lang, i18n.T(lang, "library.section_categories"), err)
	}

	hiddenCategories := map[string]bool{
		"phet": true, // interactive JS simulations
		"ted":  true, // video content
		"mooc": true, // video courses and interactive tests
	}

	// Build list of visible categories
	var visible []LibraryCategory
	for _, entry := range categories.Entries {
		categoryID := strings.ToLower(entry.Title)
		if hiddenCategories[categoryID] {
			continue
		}
		visible = append(visible, LibraryCategory{
			Title:    entry.Title,
			Category: categoryID,
		})
	}

	// Fetch counts in parallel, using cache where available
	type result struct {
		index int
		count int
	}

	cache := getLibCache()
	var wg sync.WaitGroup
	ch := make(chan result, len(visible))

	for i, cat := range visible {
		if cached, ok := cache.get(catalogLang, cat.Category); ok {
			visible[i].Count = cached
			continue
		}
		wg.Add(1)
		go func(idx int, categoryID string) {
			defer wg.Done()
			feedURL := entriesURL(catalogLang, categoryID, 1, 0)
			f, err := fetchFeed(feedURL)
			if err != nil {
				slog.Warn("Failed to fetch category count", "category", categoryID, "error", err)
				ch <- result{idx, -1}
				return
			}
			cache.set(catalogLang, categoryID, f.TotalResults)
			ch <- result{idx, f.TotalResults}
		}(i, cat.Category)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		if r.count >= 0 {
			visible[r.index].Count = r.count
		}
	}

	cache.save()

	// Filter out categories with 0 archives
	data := LibraryCategoriesData{
		UILang:       lang,
		Language:     catalogLang,
		LanguageName: name,
	}
	for _, cat := range visible {
		if cat.Count > 0 {
			data.Categories = append(data.Categories, cat)
		}
	}

	// Sort by count descending
	sort.Slice(data.Categories, func(i, j int) bool {
		return data.Categories[i].Count > data.Categories[j].Count
	})

	buf, err := executeTemplate(libraryCategoriesTemplate, lang, data)
	if err != nil {
		return nil, err
	}

	return markdown.Parse(buf)
}

type LibraryEntriesData struct {
	UILang       string
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

const pageSize = 50

func LibraryEntriesPage(lang, catalogLang, name, category string, page int) (*document.Document, error) {
	if page < 0 {
		page = 0
	}
	if page > 1000 { // sanity cap to avoid int overflow on start
		page = 1000
	}
	start := page * pageSize
	feed, err := fetchFeed(entriesURL(catalogLang, category, pageSize, start))
	if err != nil {
		return renderErrorDoc(lang, i18n.T(lang, "library.section_archives"), err)
	}

	data := LibraryEntriesData{
		UILang:       lang,
		Language:     catalogLang,
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
		filename := entry.Title + ".zim"
		if uDirect, err := url.Parse(directURL); err == nil {
			if base := path.Base(uDirect.Path); base != "" && base != "." && base != "/" {
				filename = base
			}
		}

		data.Entries = append(data.Entries, LibraryEntry{
			Title:       entry.Title,
			Summary:     entry.Summary,
			Size:        sizeStr,
			Filename:    filename,
			DownloadURL: directURL,
		})
	}

	if feed.TotalResults > 0 {
		data.HasNextPage = start+pageSize < feed.TotalResults
	} else {
		data.HasNextPage = len(feed.Entries) == pageSize
	}

	buf, err := executeTemplate(libraryEntriesTemplate, lang, data)
	if err != nil {
		return nil, err
	}

	slog.Debug("Generated library menu", "content", buf.String())
	return markdown.Parse(buf)
}

// entriesURL builds the OPDS entries endpoint with properly encoded query params.
func entriesURL(lang, category string, count, start int) string {
	q := url.Values{
		"lang":     {lang},
		"category": {category},
		"count":    {fmt.Sprint(count)},
	}
	if start > 0 {
		q.Set("start", fmt.Sprint(start))
	}
	return "https://browse.library.kiwix.org/catalog/v2/entries?" + q.Encode()
}
