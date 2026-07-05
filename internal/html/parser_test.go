package html

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

func TestParseHTML(t *testing.T) {
	input := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<h1>Hello World</h1>
<p>This is a <strong>bold</strong> paragraph with a <a href="https://example.com">link</a>.</p>
<ul>
<li>Item one</li>
<li>Item two</li>
</ul>
</body>
</html>`

	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc == nil {
		t.Fatal("nil document")
	}
	if len(doc.Blocks) == 0 {
		t.Fatal("no blocks in document")
	}

	// Should contain at least: heading, paragraph, list
	foundHeading := false
	foundParagraph := false
	foundList := false
	for _, b := range doc.Blocks {
		switch b.(type) {
		case *document.Heading:
			foundHeading = true
		case *document.Paragraph:
			foundParagraph = true
		case *document.List:
			foundList = true
		}
	}
	if !foundHeading {
		t.Error("heading not found")
	}
	if !foundParagraph {
		t.Error("paragraph not found")
	}
	if !foundList {
		t.Error("list not found")
	}
}

func TestHTMLEntityDecoding(t *testing.T) {
	input := `<p>&lt;hello&gt; &amp; &quot;world&quot;</p>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var texts []string
	document.VisitBlocks(doc.Blocks, &textCollector{&texts})
	joined := strings.Join(texts, "")

	for _, entity := range []struct {
		name     string
		expected string
		r        rune
	}{
		{"&lt;", "<", '<'},
		{"&gt;", ">", '>'},
		{"&amp;", "&", '&'},
		{"&quot;", "\"", '"'},
	} {
		if !strings.ContainsRune(joined, entity.r) {
			t.Errorf("missing %q in output: %q", entity.expected, joined)
		}
	}

	// Verify &lt; is NOT present literally.
	if strings.Contains(joined, "&lt;") || strings.Contains(joined, "&gt;") ||
		strings.Contains(joined, "&amp;") || strings.Contains(joined, "&quot;") {
		t.Errorf("raw entity found in output: %q", joined)
	}
}

func TestHTMLEntityNumeric(t *testing.T) {
	input := `<p>code: &#36; &#x3C; &#62;</p>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var texts []string
	document.VisitBlocks(doc.Blocks, &textCollector{&texts})
	joined := strings.Join(texts, "")

	for _, want := range []rune{'$', '<', '>'} {
		if !strings.ContainsRune(joined, want) {
			t.Errorf("missing %q in output: %q", want, joined)
		}
	}
}

func TestHTMLNonBreakingSpace(t *testing.T) {
	input := `<p>hello&nbsp;world</p>`
	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var texts []string
	document.VisitBlocks(doc.Blocks, &textCollector{&texts})
	joined := strings.Join(texts, "")

	r, _ := utf8.DecodeRuneInString("\u00a0")
	if !strings.ContainsRune(joined, r) {
		t.Errorf("non-breaking space not decoded: %q", joined)
	}
}

type textCollector struct {
	out *[]string
}

func (tc *textCollector) VisitHeading(h *document.Heading) {
	*tc.out = append(*tc.out, h.Content)
}
func (tc *textCollector) VisitParagraph(p *document.Paragraph) {
	for _, inl := range p.Inlines {
		if t, ok := inl.(*document.Text); ok {
			*tc.out = append(*tc.out, t.Content)
		}
	}
}
func (tc *textCollector) VisitList(l *document.List)                   {}
func (tc *textCollector) VisitCodeBlock(c *document.CodeBlock)         {}
func (tc *textCollector) VisitThematicBreak(t *document.ThematicBreak) {}
func (tc *textCollector) VisitLink(l *document.Link)                   {}
func (tc *textCollector) VisitImage(i *document.Image)                 {}
