package renderer

import (
	_ "embed"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

//go:embed assets/unifont.otf
var unifont []byte

func loadFontFromBytes(data []byte, size int) (*ttf.Font, error) {
	rw, err := sdl.RWFromMem(data)
	if err != nil {
		return nil, err
	}
	return ttf.OpenFontRW(rw, 1, size)
}

func headingFontIdx(level int) FontKind {
	fonts := [7]FontKind{FontBody, FontH1, FontH2, FontH3, FontH4, FontH5, FontH6}
	if level >= 1 && level <= 6 {
		return fonts[level]
	}
	return FontBody
}

func measureText(text string, font *ttf.Font, isBold, isItalic bool) (int32, int32) {
	if font == nil {
		return 0, 0
	}
	style := ttf.STYLE_NORMAL
	if isBold && isItalic {
		style = ttf.STYLE_BOLD | ttf.STYLE_ITALIC
	} else if isBold {
		style = ttf.STYLE_BOLD
	} else if isItalic {
		style = ttf.STYLE_ITALIC
	}
	oldStyle := font.GetStyle()
	font.SetStyle(style)
	defer font.SetStyle(oldStyle)

	if text == "" {
		return 0, int32(font.Height())
	}
	w, h, err := font.SizeUTF8(text)
	if err != nil {
		return 0, int32(font.Height())
	}
	return int32(w), int32(h)
}
