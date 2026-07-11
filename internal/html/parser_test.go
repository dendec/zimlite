package html

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dendec/zimlite/internal/document"
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
func (tc *textCollector) VisitBlockquote(b *document.Blockquote)       {}
func (tc *textCollector) VisitLink(l *document.Link)                   {}
func (tc *textCollector) VisitImage(i *document.Image)                 {}
func (tc *textCollector) VisitTable(t *document.Table)                 {}
func (tc *textCollector) VisitAnchor(a *document.Anchor)               {}

func TestHeadingIDs(t *testing.T) {
	input := `<!DOCTYPE html>
<html><body>
<h2 id="Introduction">Introduction</h2>
<p>Some text.</p>
<h3 id="Early_life">Early life</h3>
<p>More text.</p>
<h2>No ID heading</h2>
<p>Even more.</p>
<h2 id="Later_career">Later career</h2>
</body></html>`

	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var headings []*document.Heading
	for _, b := range doc.Blocks {
		if h, ok := b.(*document.Heading); ok {
			headings = append(headings, h)
		}
	}

	if len(headings) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(headings))
	}

	tests := []struct {
		idx      int
		level    int
		expected string
	}{
		{0, 2, "Introduction"},
		{1, 3, "Early_life"},
		{2, 2, ""}, // No ID
		{3, 2, "Later_career"},
	}

	for _, tt := range tests {
		if tt.idx >= len(headings) {
			t.Errorf("heading[%d]: missing", tt.idx)
			continue
		}
		h := headings[tt.idx]
		if h.Level != tt.level {
			t.Errorf("heading[%d]: level=%d, want=%d", tt.idx, h.Level, tt.level)
		}
		if h.ID != tt.expected {
			t.Errorf("heading[%d]: ID=%q, want=%q", tt.idx, h.ID, tt.expected)
		}
	}
}

func TestHeadingIDsFromChildSpan(t *testing.T) {
	input := `<!DOCTYPE html>
<html><body>
<h2><span id="History">History</span></h2>
<p>Some text.</p>
<h3><span class="mw-headline" id="Early_life">Early life</span></h3>
<p>More text.</p>
<h2><span id="See_also">See also</span></h2>
</body></html>`

	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var headings []*document.Heading
	for _, b := range doc.Blocks {
		if h, ok := b.(*document.Heading); ok {
			headings = append(headings, h)
		}
	}

	if len(headings) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(headings))
	}

	want := []string{"History", "Early_life", "See_also"}
	for i, id := range want {
		if headings[i].ID != id {
			t.Errorf("heading[%d]: ID=%q, want=%q", i, headings[i].ID, id)
		}
	}
}

func TestNonHeadingAnchors(t *testing.T) {
	input := `<!DOCTYPE html>
<html><body>
<h2 id="Top">Top</h2>
<p>First paragraph<span id="ref1"></span> text here.</p>
<p><span id="ref2"></span>Second paragraph.</p>
<p>Third paragraph <a name="ref3"></a> more text.</p>
</body></html>`

	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	wantAnchors := []string{"ref1", "ref2", "ref3"}
	var found []string
	for _, b := range doc.Blocks {
		if a, ok := b.(*document.Anchor); ok {
			found = append(found, a.ID)
		}
	}

	if len(found) != len(wantAnchors) {
		t.Fatalf("expected %d anchors, got %d: %v", len(wantAnchors), len(found), found)
	}
	for i, want := range wantAnchors {
		if found[i] != want {
			t.Errorf("anchor[%d]: %q, want %q", i, found[i], want)
		}
	}
}

func TestNonHeadingAnchorsSkipHeadingID(t *testing.T) {
	input := `<!DOCTYPE html>
<html><body>
<h2><span id="SectionTitle">Section</span></h2>
<p>Paragraph<span id="inlineRef"></span> text.</p>
</body></html>`

	doc, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var anchorIDs []string
	var headingIDs []string
	for _, b := range doc.Blocks {
		if a, ok := b.(*document.Anchor); ok {
			anchorIDs = append(anchorIDs, a.ID)
		}
		if h, ok := b.(*document.Heading); ok {
			headingIDs = append(headingIDs, h.ID)
		}
	}

	// "SectionTitle" should be on the heading, not an Anchor block
	if len(anchorIDs) != 1 || anchorIDs[0] != "inlineRef" {
		t.Errorf("anchors: %v, want [inlineRef]", anchorIDs)
	}
	if len(headingIDs) != 1 || headingIDs[0] != "SectionTitle" {
		t.Errorf("heading IDs: %v, want [SectionTitle]", headingIDs)
	}
}
