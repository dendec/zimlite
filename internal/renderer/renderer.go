// Package renderer draws document.Document via SDL2.
package renderer

import (
	"fmt"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

// FontKind identifies a pre-loaded font.
type FontKind int

const (
	FontBody FontKind = iota
	FontH1
	FontH2
	FontH3
	FontH4
	FontH5
	FontH6
	FontMono
	fontCount
)

type fontSlot struct {
	font *ttf.Font
	size int
}

// Renderer owns the SDL window and renders documents.
type Renderer struct {
	window      *sdl.Window
	sdlRenderer *sdl.Renderer
	fonts       [fontCount]fontSlot

	lines        []lineEntry
	links        []linkEntry
	codeRanges   []codeBlockRange
	totalHeight  int32
	contentWidth int32

	doc *document.Document

	scrollY      int32
	selectedLink int

	width        int32
	height       int32
	marginX      int32
	marginY      int32
	lineSpacing  int32
	blockSpacing int32
	listIndent   int32

	bgColor      sdl.Color
	textColor    sdl.Color
	linkColor    sdl.Color
	headingColor sdl.Color
	selBgColor   sdl.Color
	codeBgColor  sdl.Color
	ruleColor    sdl.Color
}

type lineEntry struct {
	text    string
	fontIdx FontKind
	color   sdl.Color
	x       int32
	y       int32
	w       int32
	h       int32
}

type linkEntry struct {
	rect  sdl.Rect
	url   string
	label string
}

type codeBlockRange struct {
	startY int32
	endY   int32
}

// New creates a Renderer.
func New(title string, winW, winH int32, fontPath string, baseFontSize int) (*Renderer, error) {
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "1")
	sdl.SetHint("SDL_HINT_KEY_REPEAT_DELAY", "300")
	sdl.SetHint("SDL_HINT_KEY_REPEAT_INTERVAL", "40")
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return nil, fmt.Errorf("sdl init: %w", err)
	}
	if err := ttf.Init(); err != nil {
		sdl.Quit()
		return nil, fmt.Errorf("ttf init: %w", err)
	}

	window, err := sdl.CreateWindow(title,
		sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		winW, winH,
		sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE)
	if err != nil {
		ttf.Quit()
		sdl.Quit()
		return nil, fmt.Errorf("create window: %w", err)
	}

	sdlRend, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
	if err != nil {
		window.Destroy()
		ttf.Quit()
		sdl.Quit()
		return nil, fmt.Errorf("create renderer: %w", err)
	}

	r := &Renderer{
		window:       window,
		sdlRenderer:  sdlRend,
		selectedLink: -1,
		width:        winW,
		height:       winH,
		marginX:      20,
		marginY:      16,
		lineSpacing:  6,
		blockSpacing: 12,
		listIndent:   24,
		bgColor:      sdl.Color{R: 245, G: 245, B: 240, A: 255},
		textColor:    sdl.Color{R: 30, G: 30, B: 30, A: 255},
		linkColor:    sdl.Color{R: 0, G: 80, B: 180, A: 255},
		headingColor: sdl.Color{R: 50, G: 50, B: 50, A: 255},
		selBgColor:   sdl.Color{R: 255, G: 230, B: 150, A: 255},
		codeBgColor:  sdl.Color{R: 235, G: 235, B: 230, A: 255},
		ruleColor:    sdl.Color{R: 180, G: 180, B: 170, A: 255},
	}

	sizes := [fontCount]int{
		FontBody: baseFontSize,
		FontH1:   baseFontSize + 14,
		FontH2:   baseFontSize + 10,
		FontH3:   baseFontSize + 6,
		FontH4:   baseFontSize + 3,
		FontH5:   baseFontSize + 1,
		FontH6:   baseFontSize - 1,
		FontMono: baseFontSize,
	}
	for i := FontKind(0); i < fontCount; i++ {
		font, err := ttf.OpenFont(fontPath, sizes[i])
		if err != nil {
			r.Destroy()
			return nil, fmt.Errorf("open font size %d: %w", sizes[i], err)
		}
		r.fonts[i] = fontSlot{font: font, size: sizes[i]}
	}

	return r, nil
}

func (r *Renderer) Destroy() {
	for i := range r.fonts {
		if r.fonts[i].font != nil {
			r.fonts[i].font.Close()
		}
	}
	if r.sdlRenderer != nil {
		r.sdlRenderer.Destroy()
	}
	if r.window != nil {
		r.window.Destroy()
	}
	ttf.Quit()
	sdl.Quit()
}

func (r *Renderer) GetWindow() *sdl.Window { return r.window }
func (r *Renderer) GetSize() (w, h int32)  { return r.width, r.height }

func (r *Renderer) SetDocument(doc *document.Document) {
	r.doc = doc
	r.scrollY = 0
	r.selectedLink = -1
	r.relayout()
}

func (r *Renderer) Relayout() {
	r.width, r.height = r.window.GetSize()
	r.relayout()
}

// --- Link API ---

func (r *Renderer) LinkCount() int               { return len(r.links) }
func (r *Renderer) SelectedLink() int             { return r.selectedLink }
func (r *Renderer) SelectLink(idx int)            { r.clampLinkIndex(idx); r.selectedLink = idx }
func (r *Renderer) SelectNextLink()               { r.moveLink(+1) }
func (r *Renderer) SelectPrevLink()               { r.moveLink(-1) }
func (r *Renderer) SelectedLinkURL() string {
	if r.selectedLink < 0 || r.selectedLink >= len(r.links) {
		return ""
	}
	return r.links[r.selectedLink].url
}

func (r *Renderer) clampLinkIndex(idx int) {
	if len(r.links) == 0 {
		r.selectedLink = -1
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(r.links) {
		idx = len(r.links) - 1
	}
}

func (r *Renderer) moveLink(delta int) {
	if len(r.links) == 0 {
		return
	}
	r.selectedLink += delta
	if r.selectedLink < 0 {
		r.selectedLink = 0
	}
	if r.selectedLink >= len(r.links) {
		r.selectedLink = len(r.links) - 1
	}
	if r.selectedLink >= 0 && r.selectedLink < len(r.links) {
		link := r.links[r.selectedLink]
		visibleTop := r.scrollY + r.marginY
		visibleBottom := r.scrollY + r.height - r.marginY
		if link.rect.Y < visibleTop {
			r.scrollY = link.rect.Y - r.marginY
		} else if link.rect.Y+link.rect.H > visibleBottom {
			r.scrollY = link.rect.Y + link.rect.H - r.height + r.marginY
		}
		r.clampScroll()
	}
}

func (r *Renderer) clampSelection() {
	if len(r.links) == 0 {
		r.selectedLink = -1
		return
	}
	if r.selectedLink < 0 {
		r.selectedLink = 0
	}
	if r.selectedLink >= len(r.links) {
		r.selectedLink = len(r.links) - 1
	}
}

// --- Scroll API ---

func (r *Renderer) ScrollY() int32 { return r.scrollY }

func (r *Renderer) ScrollBy(delta int32) {
	r.scrollY += delta
	r.clampScroll()
}

func (r *Renderer) ScrollPageUp() {
	r.scrollY -= r.height * 3 / 4
	r.clampScroll()
}

func (r *Renderer) ScrollPageDown() {
	r.scrollY += r.height * 3 / 4
	r.clampScroll()
}

func (r *Renderer) ScrollToTop()    { r.scrollY = 0 }
func (r *Renderer) ScrollToBottom() { r.scrollY = r.totalHeight - r.height; r.clampScroll() }

func (r *Renderer) clampScroll() {
	maxScroll := r.totalHeight - r.height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if r.scrollY < 0 {
		r.scrollY = 0
	}
	if r.scrollY > maxScroll {
		r.scrollY = maxScroll
	}
}

// --- Font helpers ---

func headingFontIdx(level int) FontKind {
	fonts := [7]FontKind{FontBody, FontH1, FontH2, FontH3, FontH4, FontH5, FontH6}
	if level >= 1 && level <= 6 {
		return fonts[level]
	}
	return FontBody
}

// sdlFont adapts *ttf.Font to document.Font for measurement.
type sdlFont struct{ font *ttf.Font }

func (f *sdlFont) Measure(text string) (int32, int32) {
	if text == "" {
		return 0, int32(f.font.Height())
	}
	w, h, err := f.font.SizeUTF8(text)
	if err != nil {
		return 0, int32(f.font.Height())
	}
	return int32(w), int32(h)
}

func (r *Renderer) measure(text string, font *ttf.Font) (int32, int32) {
	return (&sdlFont{font}).Measure(text)
}
