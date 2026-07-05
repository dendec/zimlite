// Package markdown converts markdown source into the internal document model.
// It uses goldmark for parsing and walks the AST to produce document.Document.
package markdown

import (
	"io"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Parse reads markdown from r and returns a Document.
func Parse(r io.Reader) (*document.Document, error) {
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	md := goldmark.New()
	// Use goldmark's parser directly to get the AST, then walk it.
	parser := md.Parser()
	root := parser.Parse(text.NewReader(src))
	if root == nil {
		return &document.Document{}, nil
	}

	conv := &converter{
		source:   src,
		document: &document.Document{},
	}

	err = ast.Walk(root, conv.walker)
	if err != nil {
		return nil, err
	}

	return conv.document, nil
}

type converter struct {
	source   []byte
	document *document.Document
}

func (c *converter) text(node *ast.Text) string {
	if node.Segment.IsEmpty() {
		return ""
	}
	return string(node.Text(c.source))
}

func (c *converter) walker(n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		// After children: some nodes (like list items) need post-processing.
		return ast.WalkContinue, nil
	}

	switch node := n.(type) {
	case *ast.Document:
		// Root node — just walk children.
	case *ast.Heading:
		c.document.Blocks = append(c.document.Blocks, &document.Heading{
			Level:   node.Level,
			Content: c.collectText(node),
		})
		return ast.WalkSkipChildren, nil

	case *ast.Paragraph:
		inlines := c.collectInlines(n)
		if len(inlines) > 0 {
			c.document.Blocks = append(c.document.Blocks, &document.Paragraph{
				Inlines: inlines,
			})
		}
		return ast.WalkSkipChildren, nil

	case *ast.List:
		blocks := c.convertList(node)
		c.document.Blocks = append(c.document.Blocks, blocks...)
		return ast.WalkSkipChildren, nil

	case *ast.FencedCodeBlock:
		lang := string(node.Language(c.source))
		lines := node.Lines()
		code := ""
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			code += string(seg.Value(c.source))
		}
		c.document.Blocks = append(c.document.Blocks, &document.CodeBlock{
			Language: lang,
			Code:     code,
		})
		return ast.WalkSkipChildren, nil

	case *ast.ThematicBreak:
		c.document.Blocks = append(c.document.Blocks, &document.ThematicBreak{})

	case *ast.Blockquote:
		// Treat blockquote contents as a flat walk (inline children processed as paragraphs).
		// For MVP, we walk into blockquote so inner paragraphs appear normally.
		return ast.WalkContinue, nil

	default:
		// Unknown block — ignore.
	}

	return ast.WalkContinue, nil
}

// collectText gathers all text content from a node's descendants (plain text only, no formatting).
func (c *converter) collectText(n ast.Node) string {
	var result string
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		ast.Walk(child, func(childNode ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			if txt, ok := childNode.(*ast.Text); ok {
				result += c.text(txt)
			}
			if _, ok := childNode.(*ast.CodeSpan); ok {
				// skip children handled above
				return ast.WalkSkipChildren, nil
			}
			return ast.WalkContinue, nil
		})
	}
	return result
}

// collectInlines walks a paragraph or similar container and returns inline elements.
func (c *converter) collectInlines(n ast.Node) []document.Inline {
	var inlines []document.Inline
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		inlines = append(inlines, c.convertInline(child)...)
	}
	return inlines
}

// collectItemInlines walks a list item and collects inline content from its children.
func (c *converter) collectItemInlines(li *ast.ListItem) []document.Inline {
	// List items in goldmark: the item node contains block children (paragraphs, etc.).
	// For each child paragraph/textblock, collect its inlines.
	var allInlines []document.Inline
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		if p, ok := child.(*ast.Paragraph); ok {
			itemInlines := c.collectInlines(p)
			if len(allInlines) > 0 && len(itemInlines) > 0 {
				allInlines = append(allInlines, &document.SoftBreak{})
			}
			allInlines = append(allInlines, itemInlines...)
		} else if tb, ok := child.(*ast.TextBlock); ok {
			itemInlines := c.collectInlines(tb)
			if len(allInlines) > 0 && len(itemInlines) > 0 {
				allInlines = append(allInlines, &document.SoftBreak{})
			}
			allInlines = append(allInlines, itemInlines...)
		}
	}
	return allInlines
}

// convertInline converts a goldmark AST inline node to document.Inline.
// May return multiple inlines for nested structures.
func (c *converter) convertInline(n ast.Node) []document.Inline {
	switch node := n.(type) {
	case *ast.Text:
		inlines := []document.Inline{&document.Text{Content: c.text(node)}}
		if node.HardLineBreak() {
			inlines = append(inlines, &document.HardBreak{})
		} else if node.SoftLineBreak() {
			inlines = append(inlines, &document.SoftBreak{})
		}
		return inlines

	case *ast.Link:
		label := c.collectText(node)
		return []document.Inline{&document.LinkInline{
			URL:   string(node.Destination),
			Label: label,
		}}

	case *ast.Image:
		alt := c.collectText(node)
		return []document.Inline{&document.LinkInline{
			URL:   string(node.Destination),
			Label: alt,
		}}

	case *ast.Emphasis:
		// Level 1 = italic, Level 2 = bold
		var inner []document.Inline
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			inner = append(inner, c.convertInline(child)...)
		}
		if node.Level == 2 {
			return []document.Inline{&document.Strong{Content: inner}}
		}
		return []document.Inline{&document.Emphasis{Content: inner}}

	case *ast.CodeSpan:
		content := c.collectText(node)
		return []document.Inline{&document.Code{Content: content}}

	case *ast.String:
		return []document.Inline{&document.Text{Content: string(node.Value)}}

	// Line breaks handled via ast.Text flags (SoftLineBreak/HardLineBreak).

	default:
		// Unknown inline — skip with no output.
	}

	return nil
}

// Ensure text package is used (needed for Parse call).
var _ = text.NewReader

func listDepth(n ast.Node) int {
	depth := 0
	for p := n.Parent(); p != nil; p = p.Parent() {
		if _, ok := p.(*ast.List); ok {
			depth++
		}
	}
	return depth
}

func (c *converter) convertList(node *ast.List) []document.Block {
	var blocks []document.Block
	list := &document.List{
		Ordered: node.IsOrdered(),
		Start:   node.Start,
		Indent:  listDepth(node),
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if li, ok := child.(*ast.ListItem); ok {
			itemInlines := c.collectItemInlines(li)
			list.Items = append(list.Items, itemInlines)

			// Find nested lists inside this list item
			for sub := li.FirstChild(); sub != nil; sub = sub.NextSibling() {
				if subList, ok := sub.(*ast.List); ok {
					nested := c.convertList(subList)
					blocks = append(blocks, nested...)
				}
			}
		}
	}
	return append([]document.Block{list}, blocks...)
}
