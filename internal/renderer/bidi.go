package renderer

import (
	"path/filepath"

	"github.com/benoitkugler/textprocessing/fribidi"

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/textdraw"
	"github.com/dendec/zimlite/internal/textinput"
)

func isRTLRune(r rune) bool {
	kind := fribidi.GetBidiType(r)
	return kind == fribidi.RTL || kind == fribidi.AL
}

func hasRTLText(text string) bool {
	for _, r := range text {
		if isRTLRune(r) {
			return true
		}
	}
	return false
}

func paragraphIsRTL(words []document.Word) bool {
	for _, word := range words {
		if word.IsCode || word.IsEmoji {
			continue
		}
		for _, r := range word.Text {
			switch fribidi.GetBidiType(r) {
			case fribidi.RTL, fribidi.AL:
				return true
			case fribidi.LTR:
				return false
			}
		}
	}
	return false
}

// visualWordOrder resolves numbers, neutrals, brackets and explicit bidi controls
// together for the wrapped line. Keep each styled word in logical order for
// HarfBuzz shaping, and use FriBidi's character mapping to position the words.
func visualWordOrder(words []document.Word, rtlBase bool) []int {
	var chars []rune
	var owners []int
	appendRune := func(ch rune, owner int) {
		chars = append(chars, ch)
		owners = append(owners, owner)
	}
	for i, word := range words {
		// Inline code is an LTR isolate, including punctuation and numbers.
		if word.IsCode {
			appendRune('\u2066', i)
		}
		if word.IsEmoji || word.IsImage || word.Text == "" {
			appendRune('\ufffc', i)
		} else {
			for _, ch := range word.Text {
				appendRune(ch, i)
			}
		}
		if word.IsCode {
			appendRune('\u2069', i)
		}
	}
	if len(chars) == 0 {
		return nil
	}
	base := fribidi.ParType(fribidi.LTR)
	if rtlBase {
		base = fribidi.RTL
	}
	visual, _ := fribidi.LogicalToVisual(0, chars, &base)
	order := make([]int, 0, len(words))
	seen := make([]bool, len(words))
	for _, logical := range visual.VisualToLogical {
		owner := owners[logical]
		if !seen[owner] {
			order = append(order, owner)
			seen[owner] = true
		}
	}
	return order
}

func (r *Renderer) renderShapedText(text string, size int) ([]byte, int, int) {
	fontPath := r.fontPath
	if fontPath == "" {
		fontPath = filepath.Join(textinput.RuntimeDir(), "fonts", "unifont.otf")
	}
	return textdraw.RenderFont(textinput.RuntimeDir(), fontPath, text, size, 4096)
}
