// Package html converts HTML documents to the internal document model.
// Pipeline: HTML → Markdown → Document (reusing the markdown parser).
package html

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"

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

	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
			MathPlugin(),
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

// MathPlugin intercepts Wikipedia math tags and formats them as inline code.
func MathPlugin() converter.Plugin {
	return &mathPlugin{}
}

type mathPlugin struct{}

func (p *mathPlugin) Name() string { return "math" }

func (p *mathPlugin) Init(c *converter.Converter) error {
	c.Register.RendererFor("span", converter.TagTypeInline, func(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
		isMath := false
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, "mwe-math-element") {
				isMath = true
				break
			}
		}
		if !isMath {
			return converter.RenderTryNext
		}

		// Find the annotation tag containing the latex
		var findTex func(*html.Node) string
		findTex = func(node *html.Node) string {
			if node.Type == html.ElementNode && node.Data == "annotation" {
				for _, a := range node.Attr {
					if a.Key == "encoding" && a.Val == "application/x-tex" {
						if node.FirstChild != nil {
							return node.FirstChild.Data
						}
					}
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if tex := findTex(child); tex != "" {
					return tex
				}
			}
			return ""
		}

		tex := findTex(n)
		if tex != "" {
			tex = strings.TrimPrefix(tex, `{\displaystyle `)
			tex = strings.TrimSuffix(tex, `}`)
			tex = strings.TrimSpace(tex)
			w.WriteString("`")
			w.WriteString(tex)
			w.WriteString("`")
			return converter.RenderSuccess
		}

		return converter.RenderTryNext
	}, converter.PriorityStandard)
	return nil
}
