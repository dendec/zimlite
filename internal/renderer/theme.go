package renderer

import "github.com/veandco/go-sdl2/sdl"

type Theme struct {
	BgColor               sdl.Color
	TextColor             sdl.Color
	LinkColor             sdl.Color
	HeadingColor          sdl.Color
	SelBgColor            sdl.Color
	SelImgColor           sdl.Color
	CodeBgColor           sdl.Color
	RuleColor             sdl.Color
	BlockquoteBgColor     sdl.Color
	BlockquoteBorderColor sdl.Color
}

func LightTheme() *Theme {
	return &Theme{
		BgColor:               sdl.Color{R: 245, G: 245, B: 240, A: 255},
		TextColor:             sdl.Color{R: 30, G: 30, B: 30, A: 255},
		LinkColor:             sdl.Color{R: 0, G: 80, B: 180, A: 255},
		HeadingColor:          sdl.Color{R: 50, G: 50, B: 50, A: 255},
		SelBgColor:            sdl.Color{R: 255, G: 230, B: 150, A: 255},
		SelImgColor:           sdl.Color{R: 255, G: 180, B: 0, A: 60},
		CodeBgColor:           sdl.Color{R: 235, G: 235, B: 230, A: 255},
		RuleColor:             sdl.Color{R: 180, G: 180, B: 170, A: 255},
		BlockquoteBgColor:     sdl.Color{R: 240, G: 240, B: 240, A: 255},
		BlockquoteBorderColor: sdl.Color{R: 180, G: 180, B: 180, A: 255},
	}
}

func DarkTheme() *Theme {
	return &Theme{
		BgColor:               sdl.Color{R: 20, G: 22, B: 28, A: 255},
		TextColor:             sdl.Color{R: 220, G: 220, B: 220, A: 255},
		LinkColor:             sdl.Color{R: 100, G: 180, B: 255, A: 255},
		HeadingColor:          sdl.Color{R: 200, G: 210, B: 220, A: 255},
		SelBgColor:            sdl.Color{R: 80, G: 60, B: 20, A: 255},
		SelImgColor:           sdl.Color{R: 150, G: 120, B: 30, A: 70},
		CodeBgColor:           sdl.Color{R: 35, G: 38, B: 45, A: 255},
		RuleColor:             sdl.Color{R: 60, G: 65, B: 70, A: 255},
		BlockquoteBgColor:     sdl.Color{R: 35, G: 38, B: 45, A: 255},
		BlockquoteBorderColor: sdl.Color{R: 80, G: 85, B: 90, A: 255},
	}
}
