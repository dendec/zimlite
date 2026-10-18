package article

import (
	"strings"
	"testing"
)

func TestTableContainmentPreservesSurroundingMarkup(t *testing.T) {
	for _, markup := range []string{
		`<p>before</p><table class="wide"><tr><td>cell</td></tr></table><p>after</p>`,
		`<p>before</p><TABLE><tr><td><table><tr><td>nested</td></tr></table></td></tr></TABLE><p>after</p>`,
	} {
		got := string(containWideTables([]byte(markup)))
		if strings.Count(got, tableScrollContainer) != 1 {
			t.Fatalf("expected one top-level table scroller: %s", got)
		}
		restored := strings.Replace(got, tableScrollContainer, "", 1)
		restored = strings.Replace(restored, "</div>", "", 1)
		if restored != markup {
			t.Fatalf("original markup changed: %s", restored)
		}
	}
}

func TestTableContainmentSkipsScriptAndEscapedText(t *testing.T) {
	markup := `<script>const text="<table>";</script><p>&lt;table&gt;</p>`
	if got := string(containWideTables([]byte(markup))); got != markup {
		t.Fatalf("non-table content changed: %s", got)
	}
}
