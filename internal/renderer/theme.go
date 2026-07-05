package renderer

import "github.com/veandco/go-sdl2/sdl"

type Theme struct {
	BgColor      sdl.Color
	TextColor    sdl.Color
	LinkColor    sdl.Color
	HeadingColor sdl.Color

	SelImgColor           sdl.Color
	CodeBgColor           sdl.Color
	RuleColor             sdl.Color
	BlockquoteBgColor     sdl.Color
	BlockquoteBorderColor sdl.Color
	StatusBarBgColor      sdl.Color
	StatusBarBorderColor  sdl.Color
}

func LightTheme() *Theme {
	return &Theme{
		BgColor:      sdl.Color{R: 255, G: 255, B: 255, A: 255}, // Clean white
		TextColor:    sdl.Color{R: 32, G: 33, B: 36, A: 255},    // Dark grey for readability
		LinkColor:    sdl.Color{R: 26, G: 115, B: 232, A: 255},  // Vibrant modern blue
		HeadingColor: sdl.Color{R: 0, G: 0, B: 0, A: 255},       // Pure black for contrast

		SelImgColor:           sdl.Color{R: 26, G: 115, B: 232, A: 60},
		CodeBgColor:           sdl.Color{R: 245, G: 245, B: 245, A: 255},
		RuleColor:             sdl.Color{R: 230, G: 230, B: 230, A: 255}, // Subtle light gray separator
		BlockquoteBgColor:     sdl.Color{R: 248, G: 249, B: 250, A: 255},
		BlockquoteBorderColor: sdl.Color{R: 200, G: 200, B: 200, A: 255},
		StatusBarBgColor:      sdl.Color{R: 240, G: 240, B: 240, A: 255},
		StatusBarBorderColor:  sdl.Color{R: 200, G: 200, B: 200, A: 255},
	}
}

func DarkTheme() *Theme {
	return &Theme{
		BgColor:      sdl.Color{R: 18, G: 20, B: 24, A: 255}, // Deep dark slate
		TextColor:    sdl.Color{R: 220, G: 220, B: 220, A: 255},
		LinkColor:    sdl.Color{R: 100, G: 180, B: 255, A: 255}, // Bright readable blue
		HeadingColor: sdl.Color{R: 240, G: 240, B: 240, A: 255},

		SelImgColor:           sdl.Color{R: 100, G: 180, B: 255, A: 60},
		CodeBgColor:           sdl.Color{R: 28, G: 31, B: 38, A: 255},
		RuleColor:             sdl.Color{R: 45, G: 50, B: 55, A: 255}, // Subtle dark separator
		BlockquoteBgColor:     sdl.Color{R: 35, G: 35, B: 35, A: 255},
		BlockquoteBorderColor: sdl.Color{R: 80, G: 80, B: 80, A: 255},
		StatusBarBgColor:      sdl.Color{R: 30, G: 30, B: 30, A: 255},
		StatusBarBorderColor:  sdl.Color{R: 60, G: 60, B: 60, A: 255},
	}
}
