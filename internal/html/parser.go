// Package html converts HTML documents to the internal document model.
// Pipeline: HTML → Markdown → Document (reusing the markdown parser).
package html

import (
	"io"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/kiwix-sdl/kiwix-sdl/internal/markdown"
)

// Parse reads HTML from r and returns a Document.
func Parse(r io.Reader) (*document.Document, error) {
	mdBytes, err := htmltomarkdown.ConvertReader(r)
	if err != nil {
		return nil, err
	}

	return markdown.Parse(strings.NewReader(string(mdBytes)))
}
