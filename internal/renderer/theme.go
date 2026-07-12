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
	TableHeaderBgColor    sdl.Color
	TableBorderColor      sdl.Color
}

func LightTheme() *Theme {
	return &Theme{
		BgColor:      sdl.Color{R: 255, G: 255, B: 255, A: 255}, // Clean white
		TextColor:    sdl.Color{R: 32, G: 33, B: 36, A: 255},    // Dark grey for readability
		LinkColor:    sdl.Color{R: 26, G: 115, B: 232, A: 255},  // Vibrant modern blue
		HeadingColor: sdl.Color{R: 0, G: 0, B: 0, A: 255},       // Pure black for contrast

		SelImgColor:           sdl.Color{R: 26, G: 115, B: 232, A: 60},
		CodeBgColor:           sdl.Color{R: 245, G: 245, B: 245, A: 255},
		RuleColor:             sdl.Color{R: 218, G: 220, B: 224, A: 255},
		BlockquoteBgColor:     sdl.Color{R: 248, G: 249, B: 250, A: 255},
		BlockquoteBorderColor: sdl.Color{R: 26, G: 115, B: 232, A: 255}, // accent blue
		StatusBarBgColor:      sdl.Color{R: 240, G: 241, B: 244, A: 255},
		StatusBarBorderColor:  sdl.Color{R: 210, G: 212, B: 216, A: 255},
		TableHeaderBgColor:    sdl.Color{R: 232, G: 237, B: 246, A: 255},
		TableBorderColor:      sdl.Color{R: 225, G: 227, B: 231, A: 255},
	}
}

func DarkTheme() *Theme {
	return &Theme{
		BgColor:      sdl.Color{R: 18, G: 20, B: 24, A: 255}, // Deep dark slate
		TextColor:    sdl.Color{R: 214, G: 216, B: 220, A: 255},
		LinkColor:    sdl.Color{R: 100, G: 180, B: 255, A: 255}, // Bright readable blue
		HeadingColor: sdl.Color{R: 240, G: 242, B: 246, A: 255},

		SelImgColor:           sdl.Color{R: 100, G: 180, B: 255, A: 60},
		CodeBgColor:           sdl.Color{R: 26, G: 28, B: 35, A: 255},
		RuleColor:             sdl.Color{R: 42, G: 46, B: 54, A: 255},
		BlockquoteBgColor:     sdl.Color{R: 28, G: 31, B: 38, A: 255},
		BlockquoteBorderColor: sdl.Color{R: 100, G: 180, B: 255, A: 255}, // accent blue
		StatusBarBgColor:      sdl.Color{R: 24, G: 26, B: 32, A: 255},
		StatusBarBorderColor:  sdl.Color{R: 42, G: 46, B: 54, A: 255},
		TableHeaderBgColor:    sdl.Color{R: 32, G: 38, B: 50, A: 255},
		TableBorderColor:      sdl.Color{R: 35, G: 39, B: 47, A: 255},
	}
}

func SepiaTheme() *Theme {
	return &Theme{
		BgColor:      sdl.Color{R: 244, G: 235, B: 212, A: 255}, // warm parchment
		TextColor:    sdl.Color{R: 52, G: 36, B: 18, A: 255},    // dark brown
		LinkColor:    sdl.Color{R: 110, G: 65, B: 18, A: 255},   // warm amber
		HeadingColor: sdl.Color{R: 36, G: 22, B: 6, A: 255},     // deep brown

		SelImgColor:           sdl.Color{R: 110, G: 65, B: 18, A: 60},
		CodeBgColor:           sdl.Color{R: 232, G: 218, B: 188, A: 255},
		RuleColor:             sdl.Color{R: 192, G: 170, B: 132, A: 255},
		BlockquoteBgColor:     sdl.Color{R: 237, G: 225, B: 196, A: 255},
		BlockquoteBorderColor: sdl.Color{R: 150, G: 100, B: 45, A: 255}, // warm amber accent
		StatusBarBgColor:      sdl.Color{R: 228, G: 212, B: 180, A: 255},
		StatusBarBorderColor:  sdl.Color{R: 175, G: 152, B: 110, A: 255},
		TableHeaderBgColor:    sdl.Color{R: 222, G: 204, B: 166, A: 255},
		TableBorderColor:      sdl.Color{R: 210, G: 190, B: 153, A: 255},
	}
}
