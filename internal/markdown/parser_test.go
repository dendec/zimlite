package markdown

import (
	"strings"
	"testing"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

func TestParseHeadings(t *testing.T) {
	md := `# Heading 1
## Heading 2
### Heading 3`

	doc, err := Parse(strings.NewReader(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []struct {
		level   int
		content string
	}{
		{1, "Heading 1"},
		{2, "Heading 2"},
		{3, "Heading 3"},
	}

	if len(doc.Blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(doc.Blocks))
	}

	for i, w := range want {
		h, ok := doc.Blocks[i].(*document.Heading)
		if !ok {
			t.Errorf("block %d: not a heading", i)
			continue
		}
		if h.Level != w.level {
			t.Errorf("block %d level: got %d, want %d", i, h.Level, w.level)
		}
		if h.Content != w.content {
			t.Errorf("block %d content: got %q, want %q", i, h.Content, w.content)
		}
	}
}

func TestParseParagraph(t *testing.T) {
	md := `This is a paragraph with **bold** and *italic* text and a [link](https://example.com).`

	doc, err := Parse(strings.NewReader(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(doc.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(doc.Blocks))
	}

	p, ok := doc.Blocks[0].(*document.Paragraph)
	if !ok {
		t.Fatal("block 0 not a paragraph")
	}

	if len(p.Inlines) < 1 {
		t.Fatal("paragraph has no inlines")
	}

	// Check we have at least a Text, Strong, Text, Emphasis, Text, LinkInline.
	hasStrong := false
	hasEmphasis := false
	hasLink := false
	for _, inl := range p.Inlines {
		switch inl.(type) {
		case *document.Strong:
			hasStrong = true
		case *document.Emphasis:
			hasEmphasis = true
		case *document.LinkInline:
			hasLink = true
		}
	}

	if !hasStrong {
		t.Error("expected bold text not found")
	}
	if !hasEmphasis {
		t.Error("expected italic text not found")
	}
	if !hasLink {
		t.Error("expected link not found")
	}
}

func TestParseList(t *testing.T) {
	md := `* Item one
* Item two
* Item three`

	doc, err := Parse(strings.NewReader(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(doc.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(doc.Blocks))
	}

	l, ok := doc.Blocks[0].(*document.List)
	if !ok {
		t.Fatal("block 0 not a list")
	}

	if len(l.Entries) == 0 {
		t.Fatal("list has no entries")
	}
	if l.Entries[0].Ordered {
		t.Error("expected unordered list")
	}
	if len(l.Entries) != 3 {
		t.Fatalf("got %d items, want 3", len(l.Entries))
	}
	for i, entry := range l.Entries {
		if len(entry.Item) == 0 {
			t.Errorf("item %d has no inline content", i)
		}
	}
}

func TestParseOrderedList(t *testing.T) {
	md := `1. First
2. Second
3. Third`

	doc, err := Parse(strings.NewReader(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	l, ok := doc.Blocks[0].(*document.List)
	if !ok {
		t.Fatal("block 0 not a list")
	}
	if len(l.Entries) == 0 {
		t.Fatal("list has no entries")
	}
	if !l.Entries[0].Ordered {
		t.Error("expected ordered list")
	}
	if l.Entries[0].Start != 1 {
		t.Errorf("start: got %d, want 1", l.Entries[0].Start)
	}
}

func TestParseCodeBlock(t *testing.T) {
	md := "```go\nfunc main() {\n\tprintln(\"hello\")\n}\n```"

	doc, err := Parse(strings.NewReader(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	cb, ok := doc.Blocks[0].(*document.CodeBlock)
	if !ok {
		t.Fatal("block 0 not a code block")
	}
	if cb.Language != "go" {
		t.Errorf("language: got %q, want %q", cb.Language, "go")
	}
}

func TestParseThematicBreak(t *testing.T) {
	md := "Before\n\n---\n\nAfter"

	doc, err := Parse(strings.NewReader(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	found := false
	for _, b := range doc.Blocks {
		if _, ok := b.(*document.ThematicBreak); ok {
			found = true
		}
	}
	if !found {
		t.Error("thematic break not found")
	}
}

func TestParseEmpty(t *testing.T) {
	doc, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc == nil {
		t.Fatal("nil document")
	}
}

func TestParseRawHTML(t *testing.T) {
	// Goldmark parses <tag> as ast.RawHTML — ensure we emit it as text.
	md := `Price: <100 & >50`
	doc, err := Parse(strings.NewReader(md))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Blocks) == 0 {
		t.Fatal("no blocks")
	}
	p, ok := doc.Blocks[0].(*document.Paragraph)
	if !ok {
		t.Fatalf("expected paragraph, got %T", doc.Blocks[0])
	}

	var texts []string
	for _, inl := range p.Inlines {
		if t, ok := inl.(*document.Text); ok {
			texts = append(texts, t.Content)
		}
	}
	joined := strings.Join(texts, "")
	if !strings.Contains(joined, "<100") {
		t.Errorf("missing <100 in output: %q", joined)
	}
	if !strings.Contains(joined, ">50") {
		t.Errorf("missing >50 in output: %q", joined)
	}
}
