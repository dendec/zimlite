// Package renderer draws document.Document via SDL2.
package renderer

import (
	"fmt"
	"strings"

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

type textureKey struct {
	text     string
	fontIdx  FontKind
	color    sdl.Color
	isBold   bool
	isItalic bool
	isCode   bool
}

type codeSpanRange struct {
	x, y, w, h int32
}

const statusBarHeight = 24

type ResourceLoader func(url string) ([]byte, error)

type imageEntry struct {
	x, y, w, h int32
	url        string
}

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
	codeSpans    []codeSpanRange
	totalHeight  int32
	contentWidth int32
	textLines    []string // cached for theme toggle

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
	light        bool
	hasTree      bool

	textureCache map[textureKey]*sdl.Texture
	imageEntries []imageEntry
	imageTextures map[string]*sdl.Texture
	loader       ResourceLoader
}

type lineEntry struct {
	text     string
	fontIdx  FontKind
	color    sdl.Color
	x        int32
	y        int32
	w        int32
	h        int32
	isBold   bool
	isItalic bool
	isCode   bool
	isCursor bool
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
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_JOYSTICK); err != nil {
		return nil, fmt.Errorf("sdl init: %w", err)
	}
	if err := ttf.Init(); err != nil {
		sdl.Quit()
		return nil, fmt.Errorf("ttf init: %w", err)
	}

	// Open first joystick/gamepad if available.
	if sdl.NumJoysticks() > 0 {
		sdl.JoystickOpen(0)
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
		light:        true,
		textureCache: make(map[textureKey]*sdl.Texture),
		imageTextures: make(map[string]*sdl.Texture),
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
		var font *ttf.Font
		var err error
		if i == FontMono {
			font, err = loadFontFromBytes(DejaVuSansMono, sizes[i])
		} else if fontPath != "" {
			font, err = ttf.OpenFont(fontPath, sizes[i])
		} else {
			font, err = loadFontFromBytes(DejaVuSans, sizes[i])
		}
		if err != nil {
			r.Destroy()
			return nil, fmt.Errorf("open font size %d: %w", sizes[i], err)
		}
		r.fonts[i] = fontSlot{font: font, size: sizes[i]}
	}

	return r, nil
}

func (r *Renderer) ClearCache() {
	if r.textureCache != nil {
		for k, tex := range r.textureCache {
			if tex != nil {
				tex.Destroy()
			}
			delete(r.textureCache, k)
		}
	}
	if r.imageTextures != nil {
		for k, tex := range r.imageTextures {
			if tex != nil {
				tex.Destroy()
			}
			delete(r.imageTextures, k)
		}
	}
}

func (r *Renderer) SetResourceLoader(loader ResourceLoader) {
	r.loader = loader
}

func (r *Renderer) Destroy() {
	r.ClearCache()
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

func (r *Renderer) SetDocument(doc *document.Document) {
	r.doc = doc
	r.scrollY = 0
	r.selectedLink = -1
	r.relayout()
}

func (r *Renderer) SetHasTree(has bool) {
	r.hasTree = has
}

// ToggleTheme switches between light and dark color schemes.
func (r *Renderer) ToggleTheme() {
	r.light = !r.light
	if r.light {
		r.bgColor = sdl.Color{R: 245, G: 245, B: 240, A: 255}
		r.textColor = sdl.Color{R: 30, G: 30, B: 30, A: 255}
		r.linkColor = sdl.Color{R: 0, G: 80, B: 180, A: 255}
		r.headingColor = sdl.Color{R: 50, G: 50, B: 50, A: 255}
		r.selBgColor = sdl.Color{R: 255, G: 230, B: 150, A: 255}
		r.codeBgColor = sdl.Color{R: 235, G: 235, B: 230, A: 255}
		r.ruleColor = sdl.Color{R: 180, G: 180, B: 170, A: 255}
	} else {
		r.bgColor = sdl.Color{R: 20, G: 22, B: 28, A: 255}
		r.textColor = sdl.Color{R: 220, G: 220, B: 220, A: 255}
		r.linkColor = sdl.Color{R: 100, G: 180, B: 255, A: 255}
		r.headingColor = sdl.Color{R: 200, G: 210, B: 220, A: 255}
		r.selBgColor = sdl.Color{R: 80, G: 60, B: 20, A: 255}
		r.codeBgColor = sdl.Color{R: 35, G: 38, B: 45, A: 255}
		r.ruleColor = sdl.Color{R: 60, G: 65, B: 70, A: 255}
	}
	r.relayout()
	r.relayoutTextLines()
}

func (r *Renderer) Relayout() {
	r.width, r.height = r.window.GetSize()
	r.relayout()
}

// --- Text line mode (for tree view, etc.) ---

// SetTextLines configures the renderer for simple text-line display mode.
func (r *Renderer) SetTextLines(lines []string) {
	r.textLines = lines
	r.lines = nil
	r.links = nil
	r.codeRanges = nil
	r.doc = nil
	r.selectedLink = -1

	font := r.fonts[FontBody].font
	y := r.marginY
	for _, text := range lines {
		isCursor := false
		displayText := text
		if strings.HasPrefix(text, ">") {
			isCursor = true
			displayText = text[1:]
		}
		tw, th := measureText(displayText, font, false, false)
		r.lines = append(r.lines, lineEntry{
			text: displayText, fontIdx: FontBody, color: r.textColor,
			x: r.marginX, y: y, w: tw, h: th,
			isCursor: isCursor,
		})
		y += th + r.lineSpacing
	}
	if y < r.height-statusBarHeight {
		y = r.height - statusBarHeight
	}
	r.totalHeight = y
	r.clampScroll()
}

func (r *Renderer) relayoutTextLines() {
	if r.textLines == nil {
		return
	}
	r.SetTextLines(r.textLines)
}

// ScrollToLine ensures the given line index is visible.
func (r *Renderer) ScrollToLine(lineIdx int) {
	if lineIdx < 0 || lineIdx >= len(r.lines) {
		return
	}
	line := r.lines[lineIdx]
	screenY := line.y - r.scrollY
	if screenY < r.marginY {
		r.scrollY = line.y - r.marginY
	} else if screenY+line.h > r.height-r.marginY {
		r.scrollY = line.y + line.h - r.height + r.marginY
	}
	r.clampScroll()
}

// --- Link API ---

func (r *Renderer) LinkCount() int          { return len(r.links) }
func (r *Renderer) SelectNextLink()          { r.moveLink(+1) }
func (r *Renderer) SelectPrevLink()          { r.moveLink(-1) }
func (r *Renderer) SelectedLinkURL() string {
	if r.selectedLink < 0 || r.selectedLink >= len(r.links) {
		return ""
	}
	return r.links[r.selectedLink].url
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
		visibleBottom := r.scrollY + r.height - r.marginY - statusBarHeight
		if link.rect.Y < visibleTop {
			r.scrollY = link.rect.Y - r.marginY
		} else if link.rect.Y+link.rect.H > visibleBottom {
			r.scrollY = link.rect.Y + link.rect.H - r.height + r.marginY + statusBarHeight
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

func (r *Renderer) ScrollBy(delta int32) {
	r.scrollY += delta
	r.clampScroll()
}

func (r *Renderer) ScrollPageUp() {
	r.scrollY -= (r.height - statusBarHeight) * 3 / 4
	r.clampScroll()
}

func (r *Renderer) ScrollPageDown() {
	r.scrollY += (r.height - statusBarHeight) * 3 / 4
	r.clampScroll()
}

func (r *Renderer) ScrollToTop()    { r.scrollY = 0 }
func (r *Renderer) ScrollToBottom() { r.scrollY = r.totalHeight - (r.height - statusBarHeight); r.clampScroll() }

func (r *Renderer) clampScroll() {
	maxScroll := r.totalHeight - (r.height - statusBarHeight)
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

func (r *Renderer) HandleClick(mx, my int32) string {
	docY := my + r.scrollY
	for i, link := range r.links {
		if mx >= link.rect.X && mx <= link.rect.X+link.rect.W &&
			docY >= link.rect.Y && docY <= link.rect.Y+link.rect.H {
			r.selectedLink = i
			return link.url
		}
	}
	return ""
}

// sdlFont adapts *ttf.Font to document.Font for measurement.
type sdlFont struct {
	r       *Renderer
	baseIdx FontKind
}

func (f *sdlFont) Measure(text string, isBold, isItalic, isCode bool) (int32, int32) {
	var font *ttf.Font
	if isCode {
		font = f.r.fonts[FontMono].font
	} else {
		font = f.r.fonts[f.baseIdx].font
	}
	if font == nil {
		return 0, 0
	}
	return measureText(text, font, isBold, isItalic)
}

func (r *Renderer) measureHeading(text string, fidx FontKind, isBold, isItalic bool) (int32, int32) {
	font := r.fonts[fidx].font
	return measureText(text, font, isBold, isItalic)
}
