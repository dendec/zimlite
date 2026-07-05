package html

import (
	"strings"
	"testing"

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
