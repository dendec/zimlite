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

	layout PageLayout

	textLines []string // cached for theme toggle

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

	theme   *Theme
	light   bool
	hasTree bool

	textureCache map[textureKey]*sdl.Texture
	imgManager   *ImageManager

	baseFontSize        int
	fontPath            string
	statusOverride      string
	hasActiveAnimations bool
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
	rects []sdl.Rect
	url   string
}

type codeBlockRange struct {
	x, y, w, h int32
}

type tableGridEntry struct {
	cellRects []sdlRect
}

type PageLayout struct {
	lines        []lineEntry
	links        []linkEntry
	codeRanges   []codeBlockRange
	codeSpans    []codeSpanRange
	blockquotes  []sdlRect
	imageEntries []imageEntry
	tables       []tableGridEntry
	totalHeight  int32
	contentWidth int32
}

// New creates a Renderer.
func New(title string, winW, winH int32, fontPath string, baseFontSize int) (*Renderer, error) {
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "1")
	sdl.SetHint("SDL_HINT_KEY_REPEAT_DELAY", "300")
	sdl.SetHint("SDL_HINT_KEY_REPEAT_INTERVAL", "40")
	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_GAMECONTROLLER); err != nil {
		return nil, fmt.Errorf("sdl init: %w", err)
	}
	if err := ttf.Init(); err != nil {
		sdl.Quit()
		return nil, fmt.Errorf("ttf init: %w", err)
	}

	// Open first joystick/gamepad if available.
	if sdl.NumJoysticks() > 0 {
		sdl.GameControllerOpen(0)
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
		listIndent:   16,
		theme:        LightTheme(),
		light:        true,
		textureCache: make(map[textureKey]*sdl.Texture),
		imgManager:   NewImageManager(sdlRend),
		baseFontSize: baseFontSize,
		fontPath:     fontPath,
	}

	fonts, err := loadFonts(baseFontSize, fontPath)
	if err != nil {
		r.Destroy()
		return nil, fmt.Errorf("open fonts: %w", err)
	}
	r.fonts = fonts

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
	if r.imgManager != nil {
		r.imgManager.ClearCache()
	}
}

func (r *Renderer) SetResourceLoader(loader ResourceLoader) {
	if r.imgManager != nil {
		r.imgManager.SetLoader(loader)
	}
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
		r.theme = LightTheme()
	} else {
		r.theme = DarkTheme()
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
	r.layout = PageLayout{}
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
		r.layout.lines = append(r.layout.lines, lineEntry{
			text: displayText, fontIdx: FontBody, color: r.theme.TextColor,
			x: r.marginX, y: y, w: tw, h: th,
			isCursor: isCursor,
		})
		y += th + r.lineSpacing
	}
	if y < r.height-statusBarHeight {
		y = r.height - statusBarHeight
	}
	r.layout.totalHeight = y
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
	if lineIdx < 0 || lineIdx >= len(r.layout.lines) {
		return
	}
	line := r.layout.lines[lineIdx]
	screenY := line.y - r.scrollY
	if screenY < r.marginY {
		r.scrollY = line.y - r.marginY
	} else if screenY+line.h > r.height-r.marginY-statusBarHeight {
		r.scrollY = line.y + line.h - r.height + r.marginY + statusBarHeight
	}
	r.clampScroll()
}

// --- Link API ---

func (r *Renderer) LinkCount() int  { return len(r.layout.links) }
func (r *Renderer) SelectNextLink() { r.moveLink(+1) }
func (r *Renderer) SelectPrevLink() { r.moveLink(-1) }
func (r *Renderer) SelectedLinkURL() string {
	if r.selectedLink < 0 || r.selectedLink >= len(r.layout.links) {
		return ""
	}
	return r.layout.links[r.selectedLink].url
}

func (r *Renderer) SelectedLinkIndex() int {
	return r.selectedLink
}

func (r *Renderer) SetSelectedLinkIndex(idx int) {
	r.selectedLink = idx
	r.clampSelection()
}

func (r *Renderer) moveLink(delta int) {
	if len(r.layout.links) == 0 {
		return
	}
	r.selectedLink += delta
	if r.selectedLink < 0 {
		r.selectedLink = 0
	}
	if r.selectedLink >= len(r.layout.links) {
		r.selectedLink = len(r.layout.links) - 1
	}
	if r.selectedLink >= 0 && r.selectedLink < len(r.layout.links) {
		link := r.layout.links[r.selectedLink]
		if len(link.rects) > 0 {
			first := link.rects[0]
			last := link.rects[len(link.rects)-1]
			visibleTop := r.scrollY + r.marginY
			visibleBottom := r.scrollY + r.height - r.marginY - statusBarHeight
			if first.Y < visibleTop {
				r.scrollY = first.Y - r.marginY
			} else if last.Y+last.H > visibleBottom {
				r.scrollY = last.Y + last.H - r.height + r.marginY + statusBarHeight
			}
			r.clampScroll()
		}
	}
}

func (r *Renderer) clampSelection() {
	if len(r.layout.links) == 0 {
		r.selectedLink = -1
		return
	}
	if r.selectedLink < 0 {
		r.selectedLink = 0
	}
	if r.selectedLink >= len(r.layout.links) {
		r.selectedLink = len(r.layout.links) - 1
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

func (r *Renderer) ScrollToTop() { r.scrollY = 0 }
func (r *Renderer) ScrollToBottom() {
	r.scrollY = r.layout.totalHeight - (r.height - statusBarHeight)
	r.clampScroll()
}

func (r *Renderer) ScrollToY(y int32) {
	r.scrollY = y - r.marginY
	r.clampScroll()
}

func (r *Renderer) CurrentScrollY() int32 {
	return r.scrollY
}

func (r *Renderer) SetScrollY(scrollY int32) {
	r.scrollY = scrollY
	r.clampScroll()
}

func (r *Renderer) FindAnchorY(anchor string) (int32, bool) {
	// 1. Try to find a heading
	headingText := strings.ReplaceAll(strings.ToLower(anchor), "_", " ")
	for _, line := range r.layout.lines {
		if line.fontIdx == FontH1 || line.fontIdx == FontH2 || line.fontIdx == FontH3 || line.fontIdx == FontH4 {
			if strings.ToLower(line.text) == headingText {
				return line.y, true
			}
		}
	}

	// 2. Try to find a matching back-link for footnotes
	var targetRef string
	if strings.HasPrefix(anchor, "cite_note-") {
		targetRef = "cite_ref-" + anchor[len("cite_note-"):]
	} else if strings.HasPrefix(anchor, "cite_ref-") {
		targetRef = "cite_note-" + anchor[len("cite_ref-"):]
		if idx := strings.LastIndex(targetRef, "-"); idx > len("cite_note-") {
			targetRef = targetRef[:idx]
		}
	}

	if targetRef != "" {
		norm := func(s string) string {
			return strings.ReplaceAll(strings.ToLower(s), "_", "-")
		}
		expected := norm("#" + targetRef)
		expectedPrefix := expected
		if strings.HasPrefix(anchor, "cite_note-") {
			expectedPrefix += "-"
		}

		for _, l := range r.layout.links {
			nURL := norm(l.url)
			if nURL == expected || strings.HasPrefix(nURL, expectedPrefix) {
				if len(l.rects) > 0 {
					return l.rects[0].Y, true
				}
			}
		}
	}

	return 0, false
}

func (r *Renderer) clampScroll() {
	maxScroll := r.layout.totalHeight - (r.height - statusBarHeight)
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
	for i, link := range r.layout.links {
		for _, rect := range link.rects {
			if mx >= rect.X && mx <= rect.X+rect.W &&
				docY >= rect.Y && docY <= rect.Y+rect.H {
				r.selectedLink = i
				return link.url
			}
		}
	}
	return ""
}

func (r *Renderer) HandleTreeClick(mx, my int32) int {
	if r.textLines == nil {
		return -1
	}
	docY := my + r.scrollY
	for i, line := range r.layout.lines {
		if docY >= line.y && docY <= line.y+line.h {
			return i
		}
	}
	return -1
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

// Zoom adjusts baseFontSize by delta and re-initializes fonts at runtime.
func (r *Renderer) Zoom(delta int) error {
	newSize := r.baseFontSize + delta
	if newSize < 10 {
		newSize = 10
	}
	if newSize > 32 {
		newSize = 32
	}
	if newSize == r.baseFontSize {
		return nil
	}
	r.baseFontSize = newSize

	// Close old fonts
	for i := 0; i < int(fontCount); i++ {
		if r.fonts[i].font != nil {
			r.fonts[i].font.Close()
		}
	}

	fonts, err := loadFonts(r.baseFontSize, r.fontPath)
	if err != nil {
		return fmt.Errorf("zoom load fonts: %w", err)
	}
	r.fonts = fonts

	// Destroy cached text textures to prevent stale text sizes/images
	for _, tex := range r.textureCache {
		tex.Destroy()
	}
	r.textureCache = make(map[textureKey]*sdl.Texture)

	// Recalculate document layout with new font sizes
	r.relayout()
	return nil
}

// SetStatusOverride sets a custom status bar message to override help legends.
func (r *Renderer) SetStatusOverride(status string) {
	r.statusOverride = status
}

func (r *Renderer) HasAnimations() bool {
	return r.hasActiveAnimations
}
