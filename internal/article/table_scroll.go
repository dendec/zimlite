package article

import (
	"bytes"

	"golang.org/x/net/html"
)

const tableScrollContainer = `<div class="zimlite-table-scroll" style="display:block;width:100%;max-width:100%;box-sizing:border-box;overflow-x:auto;overflow-y:hidden;overscroll-behavior-x:contain">`

// containWideTables wraps each top-level HTML table in its own horizontal
// scroller. The tokenizer preserves the original article markup byte-for-byte
// apart from the inserted containers, without building a second DOM tree.
func containWideTables(data []byte) []byte {
	if !hasHTMLTable(data) {
		return data
	}

	var output bytes.Buffer
	output.Grow(len(data) + 64)
	tokenizer := html.NewTokenizer(bytes.NewReader(data))
	depth := 0
	for {
		tokenType := tokenizer.Next()
		raw := tokenizer.Raw()
		if tokenType == html.ErrorToken {
			if len(raw) > 0 {
				_, _ = output.Write(raw)
			}
			break
		}

		name, _ := tokenizer.TagName()
		closeWrapper := false
		if tokenType == html.StartTagToken || tokenType == html.SelfClosingTagToken {
			if bytes.EqualFold(name, []byte("table")) {
				if depth == 0 {
					output.WriteString(tableScrollContainer)
				}
				depth++
			}
		} else if tokenType == html.EndTagToken && bytes.EqualFold(name, []byte("table")) && depth > 0 {
			depth--
			closeWrapper = depth == 0
		}

		_, _ = output.Write(raw)
		if closeWrapper {
			output.WriteString("</div>")
		}
	}

	// Close a wrapper when the source omits a closing table tag. The browser
	// will apply its normal implied table-end handling to the original markup.
	if depth > 0 {
		output.WriteString("</div>")
	}
	return output.Bytes()
}

func hasHTMLTable(data []byte) bool {
	tokenizer := html.NewTokenizer(bytes.NewReader(data))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			return false
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		name, _ := tokenizer.TagName()
		if bytes.EqualFold(name, []byte("table")) {
			return true
		}
	}
}
