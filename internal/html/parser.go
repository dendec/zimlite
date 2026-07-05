// Package html converts HTML documents to the internal document model.
// Pipeline: HTML → Markdown → Document (reusing the markdown parser).
package html

import (
	"bytes"
	"html"
	"io"
	"os"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
)

// Parse reads HTML from r and returns a Document.
func Parse(r io.Reader) (*document.Document, error) {
	if os.Getenv("KIWIX_DEBUG") != "" {
		data, err := io.ReadAll(r)
		if err == nil {
			_ = os.WriteFile("/tmp/debug_kiwix.html", data, 0644)
			r = bytes.NewReader(data)
		}
	}

	conv := htmltomarkdown.NewConverter(
		htmltomarkdown.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
		),
	)

	mdBytes, err := conv.ConvertReader(r)
	if err != nil {
		return nil, err
	}

	// Decode any remaining HTML entities (&lt; → <, &gt; → >, &amp; → &, ...).
	md := html.UnescapeString(string(mdBytes))

	if os.Getenv("KIWIX_DEBUG") != "" {
		_ = os.WriteFile("/tmp/debug_kiwix.md", []byte(md), 0644)
	}

	return markdown.Parse(strings.NewReader(md))
}
