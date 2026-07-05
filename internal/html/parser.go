// Package html converts HTML documents to the internal document model.
// Pipeline: HTML → Markdown → Document (reusing the markdown parser).
package html

import (
	"bytes"
	"io"
	"log"
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

	headingMeta, anchors := extractDocMeta(doc)

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

	result, err := markdown.Parse(strings.NewReader(md))
	if err == nil {
		assignHeadingIDs(result, headingMeta)
		insertAnchors(result, anchors)
	}
	return result, err
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

// headingMeta holds metadata about a heading extracted from the HTML tree
// before it gets converted to markdown (which loses id attributes).
type headingMeta struct {
	Level int
	ID    string
}

// anchorInfo holds a non-heading anchor extracted from the HTML tree.
type anchorInfo struct {
	ID     string
	Prefix string // first 40 chars of the nearest block ancestor text content
}

func findFirstID(n *html.Node) string {
	if id := nodeID(n); id != "" {
		return id
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" {
			if id := nodeID(c); id != "" {
				return id
			}
		}
	}
	return ""
}

func nodeID(n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "id" && attr.Val != "" {
			return attr.Val
		}
	}
	return ""
}

// extractDocMeta walks the HTML tree and returns heading metadata and non-heading
// anchor info in document order. Anchors inside heading elements are skipped
// (they are collected as heading metadata instead).
func extractDocMeta(root *html.Node) ([]headingMeta, []anchorInfo) {
	var headings []headingMeta
	var anchors []anchorInfo

	var walk func(*html.Node, bool)
	walk = func(n *html.Node, insideHeading bool) {
		if n.Type == html.ElementNode {
			level := headingLevel(n.Data)
			if level > 0 {
				headings = append(headings, headingMeta{Level: level, ID: findFirstID(n)})
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c, true)
				}
				return
			}

			if !insideHeading {
				if id := elementAnchorID(n); id != "" {
					prefix := blockTextPrefix(n)
					anchors = append(anchors, anchorInfo{ID: id, Prefix: prefix})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, insideHeading)
		}
	}
	walk(root, false)
	return headings, anchors
}

var headingLevels = map[string]int{
	"h1": 1, "h2": 2, "h3": 3,
	"h4": 4, "h5": 5, "h6": 6,
}

func headingLevel(tag string) int {
	return headingLevels[tag]
}

func elementAnchorID(n *html.Node) string {
	if n.Data == "span" {
		return nodeID(n)
	}
	if n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "name" && attr.Val != "" {
				return attr.Val
			}
		}
	}
	return ""
}

func blockTextPrefix(anchorNode *html.Node) string {
	parent := anchorNode.Parent
	if parent == nil {
		return ""
	}
	text := collectTextExcluding(parent, anchorNode)
	text = strings.TrimSpace(text)
	text = truncateText(text, 40)
	return text
}

// collectTextExcluding walks n's subtree and collects all text nodes,
// excluding the subtree rooted at exclude. The anchor's surrounding text
// (both before and after) is needed for reliable prefix matching.
func collectTextExcluding(n, exclude *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == exclude {
			return
		}
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}

// assignHeadingIDs consumes headingMeta entries in order and assigns IDs to
// matching Heading blocks in the document. Empty-ID entries are consumed normally
// (1:1 correspondence is preserved).
func assignHeadingIDs(doc *document.Document, meta []headingMeta) {
	i := 0
	for _, block := range doc.Blocks {
		h, ok := block.(*document.Heading)
		if !ok {
			continue
		}
		for i < len(meta) && meta[i].Level != h.Level {
			i++
		}
		if i < len(meta) {
			h.ID = meta[i].ID
			i++
		}
	}
}

// insertAnchors inserts Anchor blocks into the document for non-heading anchor
// elements. Each anchorInfo has a text Prefix collected from the nearest block
// ancestor. The function scans non-heading blocks and inserts an Anchor before
// the first block whose text content starts with the prefix.
// Unmatched anchors are inserted at the end of the document.
func insertAnchors(doc *document.Document, anchors []anchorInfo) {
	if len(anchors) == 0 {
		return
	}

	type blockText struct {
		idx  int
		text string
	}
	var blocks []blockText
	for i, b := range doc.Blocks {
		if _, ok := b.(*document.Heading); ok {
			continue
		}
		blocks = append(blocks, blockText{idx: i, text: docBlockTextPrefix(b)})
	}

	// Match anchors to blocks by text prefix and record insertions.
	// anchorIdx → block index in doc.Blocks where it should be inserted before.
	inserts := make(map[int][]string)
	matched := make(map[string]bool)

	for _, a := range anchors {
		if a.Prefix == "" {
			continue
		}
		for _, b := range blocks {
			if b.text != "" && strings.HasPrefix(b.text, a.Prefix) {
				inserts[b.idx] = append(inserts[b.idx], a.ID)
				matched[a.ID] = true
				break
			}
		}
	}

	// Unmatched anchors go at the end.
	var unmatchedIDs []string
	for _, a := range anchors {
		if !matched[a.ID] {
			unmatchedIDs = append(unmatchedIDs, a.ID)
		}
	}
	if len(unmatchedIDs) > 0 {
		log.Printf("html: %d of %d anchors unmatched, appending at document end: %v",
			len(unmatchedIDs), len(anchors), unmatchedIDs)
	}

	// Build new block list with anchors inserted.
	var newBlocks []document.Block
	for i, b := range doc.Blocks {
		if ids, ok := inserts[i]; ok {
			for _, id := range ids {
				newBlocks = append(newBlocks, &document.Anchor{ID: id})
			}
		}
		newBlocks = append(newBlocks, b)
	}
	for _, id := range unmatchedIDs {
		newBlocks = append(newBlocks, &document.Anchor{ID: id})
	}

	doc.Blocks = newBlocks
}

func docBlockTextPrefix(b document.Block) string {
	switch b := b.(type) {
	case *document.Paragraph:
		return paragraphText(b, 40)
	case *document.CodeBlock:
		text := strings.TrimSpace(b.Code)
		text = truncateText(text, 40)
		return text
	case *document.Table:
		if len(b.Rows) > 0 && len(b.Rows[0].Cells) > 0 {
			return paragraphTextInline(b.Rows[0].Cells[0].Inlines, 40)
		}
	case *document.Blockquote:
		if len(b.Blocks) > 0 {
			return docBlockTextPrefix(b.Blocks[0])
		}
	case *document.List:
		if len(b.Entries) > 0 && len(b.Entries[0].Item) > 0 {
			return paragraphTextInline(b.Entries[0].Item, 40)
		}
	}
	return ""
}

func paragraphText(p *document.Paragraph, maxLen int) string {
	return paragraphTextInline(p.Inlines, maxLen)
}

func paragraphTextInline(inlines []document.Inline, maxLen int) string {
	var buf strings.Builder
	for _, inl := range inlines {
		if t, ok := inl.(*document.Text); ok {
			buf.WriteString(t.Content)
		}
	}
	text := strings.TrimSpace(buf.String())
	text = truncateText(text, maxLen)
	return text
}

func truncateText(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return s
}
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
