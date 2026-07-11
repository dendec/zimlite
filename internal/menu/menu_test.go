package menu

import (
	"strings"
	"testing"

	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/document"
)

// extractInlineText recursively extracts text from any inline type.
func extractInlineText(inl document.Inline) string {
	switch v := inl.(type) {
	case *document.Text:
		return v.Content
	case *document.Strong:
		return extractInlines(v.Content)
	case *document.Emphasis:
		return extractInlines(v.Content)
	case *document.LinkInline:
		return extractInlines(v.Content)
	case *document.Code:
		return v.Content
	}
	return ""
}

func extractInlines(inlines []document.Inline) string {
	var sb strings.Builder
	for _, inl := range inlines {
		sb.WriteString(extractInlineText(inl))
	}
	return sb.String()
}

// docText extracts all text content from a Document for assertion.
func docText(doc *document.Document) string {
	var sb strings.Builder
	for _, b := range doc.Blocks {
		switch v := b.(type) {
		case *document.Heading:
			sb.WriteString(v.Content)
			sb.WriteByte('\n')
		case *document.Paragraph:
			sb.WriteString(extractInlines(v.Inlines))
			sb.WriteByte('\n')
		case *document.List:
			for _, entry := range v.Entries {
				sb.WriteString(extractInlines(entry.Item))
				sb.WriteByte('\n')
			}
		case *document.Table:
			for _, row := range v.Rows {
				for _, cell := range row.Cells {
					sb.WriteString(extractInlines(cell.Inlines))
				}
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String()
}

func TestSettingsPage_Russian(t *testing.T) {
	cfg := config.Config{Theme: "dark", Language: "ru", FontSize: 16}
	doc, err := SettingsPage("ru", cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := docText(doc)
	if !strings.Contains(text, "Настройки") {
		t.Errorf("settings page in ru missing Russian title, got: %s", text)
	}
	if !strings.Contains(text, "Тёмная") {
		t.Errorf("settings page in ru missing Russian theme name, got: %s", text)
	}
}

func TestSettingsPage_English(t *testing.T) {
	cfg := config.Config{Theme: "light", Language: "en", FontSize: 16}
	doc, err := SettingsPage("en", cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := docText(doc)
	if !strings.Contains(text, "Settings") {
		t.Errorf("settings page in en missing English title, got: %s", text)
	}
}

func TestHelpPage_Russian(t *testing.T) {
	doc, err := HelpPage("ru", false)
	if err != nil {
		t.Fatal(err)
	}
	text := docText(doc)
	if !strings.Contains(text, "Помощь") {
		t.Errorf("help page in ru missing Russian title, got: %s", text)
	}
	if !strings.Contains(text, "Навигация") {
		t.Errorf("help page in ru missing Russian navigation, got: %s", text)
	}
}

func TestHelpPage_Gamepad_Russian(t *testing.T) {
	doc, err := HelpPage("ru", true)
	if err != nil {
		t.Fatal(err)
	}
	text := docText(doc)
	if !strings.Contains(text, "Помощь") {
		t.Errorf("gamepad help in ru missing Russian title, got: %s", text)
	}
}

func TestFileSelector_English(t *testing.T) {
	doc, err := FileSelector("en", false)
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("FileSelector returned nil doc")
	}
}
