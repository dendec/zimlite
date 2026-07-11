package renderer

import (
	_ "embed"
	"log/slog"

	"github.com/dendec/zimlite/internal/document"
	"github.com/veandco/go-sdl2/ttf"
)

//go:embed assets/unifont.otf
var unifont []byte

func headingFontIdx(level int) FontKind {
	fonts := [7]FontKind{FontBody, FontH1, FontH2, FontH3, FontH4, FontH5, FontH6}
	if level >= 1 && level <= 6 {
		return fonts[level]
	}
	return FontBody
}

func fontStyle(isBold, isItalic bool) int {
	if isBold && isItalic {
		return ttf.STYLE_BOLD | ttf.STYLE_ITALIC
	}
	if isBold {
		return ttf.STYLE_BOLD
	}
	if isItalic {
		return ttf.STYLE_ITALIC
	}
	return ttf.STYLE_NORMAL
}

func measureText(text string, font *ttf.Font, isBold, isItalic, isCode bool) (int32, int32) {
	if font == nil {
		return 0, 0
	}
	style := fontStyle(isBold, isItalic)
	oldStyle := font.GetStyle()
	font.SetStyle(style)
	defer font.SetStyle(oldStyle)

	if text == "" {
		return 0, int32(font.Height())
	}
	if !isCode {
		runes := []rune(text)
		if _, consumed, ok := document.EmojiSequence(runes, 0); ok && consumed == len(runes) {
			a := int32(font.Ascent())
			return a, a
		}
	}
	w, h, err := font.SizeUTF8(text)
	if err != nil {
		return 0, int32(font.Height())
	}
	return int32(w), int32(h)
}

// truncateRunesToWidth returns the largest rune count such that
// text[:n] fits within maxW pixels. Always returns ≥ 1.
func truncateRunesToWidth(runes []rune, font *ttf.Font, maxW int32) int {
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		w, _ := measureText(string(runes[:mid]), font, false, false, false)
		if w <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo < 1 {
		lo = 1
	}
	return lo
}

func loadFonts(baseSize int, fontPath string) ([fontCount]fontSlot, error) {
	var fonts [fontCount]fontSlot

	sizes := [fontCount]int{
		FontBody: baseSize,
		FontH1:   baseSize + 5,
		FontH2:   baseSize + 4,
		FontH3:   baseSize + 3,
		FontH4:   baseSize + 2,
		FontH5:   baseSize + 1,
		FontH6:   baseSize,
	}

	for i := FontKind(0); i < fontCount; i++ {
		var font *ttf.Font
		var err error
		slog.Info("loadFonts: loading", "index", int(i), "size", sizes[i], "fromMem", fontPath == "")
		if fontPath != "" {
			font, err = openFontSafe(fontPath, sizes[i])
		} else {
			font, err = openFontFromMem(unifont, sizes[i])
		}
		if err != nil {
			for j := FontKind(0); j < i; j++ {
				if fonts[j].font != nil {
					fonts[j].font.Close()
				}
			}
			return fonts, err
		}
		slog.Info("loadFonts: loaded ok", "index", int(i))
		fonts[i] = fontSlot{font: font, size: sizes[i]}
	}

	slog.Info("loadFonts: all fonts loaded")
	return fonts, nil
}
