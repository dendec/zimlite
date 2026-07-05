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

	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	preprocessTables(doc)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return nil, err
	}

	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
			MathPlugin(),
		),
	)
	mdBytes, err := conv.ConvertReader(&buf)
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

func preprocessTables(doc *html.Node) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			hasTh := false
			hasTd := false
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					switch c.Data {
					case "th":
						hasTh = true
					case "td":
						hasTd = true
					}
				}
			}
			if hasTh && hasTd {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "th" {
						c.AppendChild(&html.Node{
							Type: html.TextNode,
							Data: ": ",
						})
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
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

		// Find the fallback image tag
		var findImg func(*html.Node) *html.Node
		findImg = func(node *html.Node) *html.Node {
			if node.Type == html.ElementNode && node.Data == "img" {
				for _, a := range node.Attr {
					if a.Key == "class" && strings.Contains(a.Val, "mwe-math-fallback-image-inline") {
						return node
					}
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if img := findImg(child); img != nil {
					return img
				}
			}
			return nil
		}

		img := findImg(n)
		if img != nil {
			var alt, src string
			for _, a := range img.Attr {
				switch a.Key {
				case "alt":
					alt = a.Val
				case "src":
					src = a.Val
				}
			}

			alt = strings.TrimPrefix(alt, `{\displaystyle `)
			alt = strings.TrimSuffix(alt, `}`)
			alt = strings.TrimSpace(alt)

			_, _ = w.WriteString("![")
			_, _ = w.WriteString(alt)
			_, _ = w.WriteString("](")
			_, _ = w.WriteString(src)
			_, _ = w.WriteString(")")
			return converter.RenderSuccess
		}

		return converter.RenderTryNext
	}, converter.PriorityStandard)
	return nil
}
