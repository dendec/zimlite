// Package renderer draws document.Document via SDL2.
package renderer

import (
	"container/list"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/document"
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

type emojiCacheKey struct {
	hex  string
	size int32
}

const (
	statusBarPadding    = 2
	defaultMarginX      = 20
	defaultMarginY      = 16
	defaultLineSpacing  = 6
	defaultBlockSpacing = 8
	defaultListIndent   = 16
	maxTextCacheSize    = 1500
	maxEmojiCacheSize   = 512
)

func (r *Renderer) getStatusBarHeight() int32 {
	if r.fonts[FontBody].font == nil {
		return 24
	}
	return int32(r.fonts[FontBody].font.Height()) + statusBarPadding
}

// TextureCache manages GPU texture caching with LRU eviction.
//
// IMPORTANT: TextureCache is NOT safe for concurrent use. All methods must be
// called from the SDL main thread (the render goroutine). Background goroutines
// (animation ticker, downloads) must only interact with SDL via PushEvent.
type cacheEntry[K comparable] struct {
	key K
	tex *sdl.Texture
}

type TextureCache[K comparable] struct {
	items     map[K]*list.Element
	evictList *list.List
	maxSize   int
}

func NewTextureCache[K comparable](maxSize int) *TextureCache[K] {
	return &TextureCache[K]{
		items:     make(map[K]*list.Element),
		evictList: list.New(),
		maxSize:   maxSize,
	}
}

func (c *TextureCache[K]) Get(key K) *sdl.Texture {
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		return ent.Value.(*cacheEntry[K]).tex
	}
	return nil
}

func (c *TextureCache[K]) Set(key K, tex *sdl.Texture) {
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		ent.Value.(*cacheEntry[K]).tex = tex
		return
	}
	ent := &cacheEntry[K]{key: key, tex: tex}
	entry := c.evictList.PushFront(ent)
	c.items[key] = entry
	if c.evictList.Len() > c.maxSize {
		c.Evict()
	}
}

func (c *TextureCache[K]) Evict() {
	remove := c.evictList.Len() / 4
	if remove < 1 {
		remove = 1
	}
	for i := 0; i < remove; i++ {
		ent := c.evictList.Back()
		if ent != nil {
			c.evictList.Remove(ent)
			kv := ent.Value.(*cacheEntry[K])
			delete(c.items, kv.key)
			if kv.tex != nil {
				kv.tex.Destroy()
			}
		}
	}
}

func (c *TextureCache[K]) Clear() {
	c.DestroyAll()
}

func (c *TextureCache[K]) DestroyAll() {
	for _, ent := range c.items {
		kv := ent.Value.(*cacheEntry[K])
		if kv.tex != nil {
			kv.tex.Destroy()
		}
	}
	c.items = make(map[K]*list.Element)
	c.evictList.Init()
}

type ResourceLoader func(url string) ([]byte, error)

// TreeItem describes one item in the tree view for structured rendering.
type TreeItem struct {
	Text       string
	Path       string
	IsLeaf     bool
	IsCursor   bool
	LabelStart int // rune offset where label begins in Text
	LabelEnd   int // rune offset where label ends in Text (exclusive)
}

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

	treeItems []TreeItem // cached for theme toggle

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

	textCache  *TextureCache[textureKey]
	emojiCache *TextureCache[emojiCacheKey]
	imgManager *ImageManager

	baseFontSize        int
	fontPath            string
	docTitle            string
	statusOverride      string
	hasActiveAnimations bool
	hoveredLink         int
	hoveredTreeLine     int // index of hovered tree line (-1 = none)
	arrowCursor         *sdl.Cursor
	handCursor          *sdl.Cursor
	hasPointer          bool // mouse or touch device present
}

type lineEntry struct {
	text        string
	fontIdx     FontKind
	color       sdl.Color
	x           int32
	y           int32
	w           int32
	h           int32
	isBold      bool
	isItalic    bool
	isCode      bool
	isCursor    bool
	isEmoji     bool
	emojiHex    string
	labelX      int32 // pixel X where label starts (for tree underline)
	labelW      int32 // pixel width of label (for tree underline)
	prefixW     int32 // pixel width of tree prefix (connector symbols)
	prefixRuneN int   // rune count of prefix
	labelRuneN  int   // rune count of label (from prefix end)
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
	lines           []lineEntry
	links           []linkEntry
	codeRanges      []codeBlockRange
	codeSpans       []codeSpanRange
	blockquotes     []sdlRect
	imageEntries    []imageEntry
	tables          []tableGridEntry
	totalHeight     int32
	contentWidth    int32
	anchorPositions map[string]int32
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
		marginX:      defaultMarginX,
		marginY:      defaultMarginY,
		lineSpacing:  defaultLineSpacing,
		blockSpacing: defaultBlockSpacing,
		listIndent:   defaultListIndent,
		theme:        LightTheme(),
		light:        true,
		textCache:    NewTextureCache[textureKey](maxTextCacheSize),
		emojiCache:   NewTextureCache[emojiCacheKey](maxEmojiCacheSize),
		imgManager:   NewImageManager(sdlRend),
		baseFontSize: baseFontSize,
		fontPath:     fontPath,
		arrowCursor:  sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW),
		handCursor:   sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_HAND),
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
	r.textCache.Clear()
	r.emojiCache.Clear()
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
	if r.arrowCursor != nil {
		sdl.FreeCursor(r.arrowCursor)
	}
	if r.handCursor != nil {
		sdl.FreeCursor(r.handCursor)
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
	r.hoveredLink = -1
	sdl.SetCursor(r.arrowCursor)
	r.docTitle = extractTitle(doc)
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

// IsLight returns whether the current theme is light.
func (r *Renderer) IsLight() bool {
	return r.light
}

func (r *Renderer) Relayout() {
	r.width, r.height = r.window.GetSize()
	r.relayout()
}

// --- Text line mode (for tree view, etc.) ---

// SetTextLines configures the renderer for simple text-line display mode.
func (r *Renderer) SetTextLines(title string, items []TreeItem) {
	r.treeItems = items
	r.layout = PageLayout{}
	r.doc = nil
	r.selectedLink = -1
	r.hoveredLink = -1
	r.hoveredTreeLine = -1
	if title != "" {
		r.docTitle = title
	}

	font := r.fonts[FontBody].font
	y := r.marginY
	for _, item := range items {
		tw, th := measureText(item.Text, font, false, false, false)
		lineColor := r.theme.TextColor
		var labelX, labelW, prefixW int32
		var prefixRuneN, labelRuneN int
		if item.LabelEnd > item.LabelStart {
			runes := []rune(item.Text)
			prefixText := string(runes[:item.LabelStart])
			labelText := string(runes[item.LabelStart:item.LabelEnd])
			prefixW, _ = measureText(prefixText, font, false, false, false)
			labelW, _ = measureText(labelText, font, false, false, false)
			labelX = r.marginX + prefixW
			prefixRuneN = item.LabelStart
			labelRuneN = item.LabelEnd - item.LabelStart
		}
		r.layout.lines = append(r.layout.lines, lineEntry{
			text: item.Text, fontIdx: FontBody, color: lineColor,
			x: r.marginX, y: y, w: tw, h: th,
			isCursor: item.IsCursor,
			labelX:   labelX, labelW: labelW, prefixW: prefixW,
			prefixRuneN: prefixRuneN, labelRuneN: labelRuneN,
		})
		y += th
	}
	if y < r.height-r.getStatusBarHeight() {
		y = r.height - r.getStatusBarHeight()
	}
	r.layout.totalHeight = y
	r.clampScroll()
}

func (r *Renderer) relayoutTextLines() {
	if r.treeItems == nil {
		return
	}
	r.SetTextLines(r.docTitle, r.treeItems)
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
	} else if screenY+line.h > r.height-r.marginY-r.getStatusBarHeight() {
		r.scrollY = line.y + line.h - r.height + r.marginY + r.getStatusBarHeight()
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
			visibleBottom := r.scrollY + r.height - r.marginY - r.getStatusBarHeight()
			if first.Y < visibleTop {
				r.scrollY = first.Y - r.marginY
			} else if last.Y+last.H > visibleBottom {
				r.scrollY = last.Y + last.H - r.height + r.marginY + r.getStatusBarHeight()
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
	if r.selectedLink < 0 && !r.hasPointer {
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
	r.scrollY -= (r.height - r.getStatusBarHeight()) * 3 / 4
	r.clampScroll()
}

func (r *Renderer) ScrollPageDown() {
	r.scrollY += (r.height - r.getStatusBarHeight()) * 3 / 4
	r.clampScroll()
}

func (r *Renderer) ScrollToTop() { r.scrollY = 0 }
func (r *Renderer) ScrollToBottom() {
	r.scrollY = r.layout.totalHeight - (r.height - r.getStatusBarHeight())
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
	if y, ok := r.layout.anchorPositions[anchor]; ok {
		return y, true
	}

	if decoded, err := url.QueryUnescape(anchor); err == nil && decoded != anchor {
		if y, ok := r.layout.anchorPositions[decoded]; ok {
			return y, true
		}
		anchor = decoded
	}

	normAnchor := strings.ToLower(anchor)
	if y, ok := r.layout.anchorPositions[normAnchor]; ok {
		return y, true
	}

	withSpace := strings.ReplaceAll(normAnchor, "_", " ")
	if y, ok := r.layout.anchorPositions[withSpace]; ok {
		return y, true
	}

	withHyphen := strings.ReplaceAll(normAnchor, "_", "-")
	if y, ok := r.layout.anchorPositions[withHyphen]; ok {
		return y, true
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
		norm := strings.ReplaceAll(strings.ToLower("#"+targetRef), "_", "-")
		if y, ok := r.layout.anchorPositions[norm]; ok {
			return y, true
		}

		expectedPrefix := norm
		if strings.HasPrefix(anchor, "cite_note-") {
			expectedPrefix += "-"
			for key, y := range r.layout.anchorPositions {
				if strings.HasPrefix(key, expectedPrefix) {
					return y, true
				}
			}
		}
	}

	return 0, false
}

func (r *Renderer) clampScroll() {
	maxScroll := r.layout.totalHeight - (r.height - r.getStatusBarHeight())
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
	startIdx := sort.Search(len(r.layout.links), func(i int) bool {
		rects := r.layout.links[i].rects
		if len(rects) == 0 {
			return false
		}
		return rects[len(rects)-1].Y+rects[len(rects)-1].H >= docY
	})
	for i := startIdx; i < len(r.layout.links); i++ {
		link := r.layout.links[i]
		if len(link.rects) > 0 && link.rects[0].Y > docY {
			break
		}
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
	if r.treeItems == nil {
		return -1
	}
	docY := my + r.scrollY
	startIdx := sort.Search(len(r.layout.lines), func(i int) bool {
		line := r.layout.lines[i]
		return line.y+line.h >= docY
	})
	for i := startIdx; i < len(r.layout.lines); i++ {
		line := r.layout.lines[i]
		if line.y > docY {
			break
		}
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
	font := f.r.fonts[f.baseIdx].font
	if font == nil {
		return 0, 0
	}
	return measureText(text, font, isBold, isItalic, isCode)
}

// Zoom adjusts baseFontSize by delta and re-initializes fonts at runtime.
func (r *Renderer) Zoom(delta int) error {
	newSize := r.baseFontSize + delta
	if newSize < config.MinFontSize {
		newSize = config.MinFontSize
	}
	if newSize > config.MaxFontSize {
		newSize = config.MaxFontSize
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
	r.textCache.DestroyAll()

	// Recalculate document layout with new font sizes
	r.relayout()
	return nil
}

// SetStatusOverride sets a custom status bar message to override help legends.
func (r *Renderer) SetStatusOverride(status string) {
	r.statusOverride = status
}

// GetStatusOverride returns the current custom status bar message.
func (r *Renderer) GetStatusOverride() string {
	return r.statusOverride
}

func extractTitle(doc *document.Document) string {
	for _, b := range doc.Blocks {
		if h, ok := b.(*document.Heading); ok && h.Level == 1 {
			runes := []rune(h.Content)
			if len(runes) > 45 {
				return string(runes[:42]) + "..."
			}
			return h.Content
		}
	}
	return ""
}

func (r *Renderer) HasAnimations() bool {
	return r.hasActiveAnimations
}

func (r *Renderer) HandleMouseMove(mx, my int32) {
	docY := my + r.scrollY
	// On first mouse event, mark pointer present and deselect.
	if !r.hasPointer {
		r.hasPointer = true
		r.selectedLink = -1
	}
	prevLink := r.hoveredLink
	r.hoveredLink = -1
	r.hoveredTreeLine = -1
	startIdx := sort.Search(len(r.layout.links), func(i int) bool {
		rects := r.layout.links[i].rects
		if len(rects) == 0 {
			return false
		}
		return rects[len(rects)-1].Y+rects[len(rects)-1].H >= docY
	})
	for i := startIdx; i < len(r.layout.links); i++ {
		link := r.layout.links[i]
		if len(link.rects) > 0 && link.rects[0].Y > docY {
			break
		}
		for _, rect := range link.rects {
			if mx >= rect.X && mx <= rect.X+rect.W &&
				docY >= rect.Y && docY <= rect.Y+rect.H {
				r.hoveredLink = i
				break
			}
		}
		if r.hoveredLink >= 0 {
			break
		}
	}
	if r.hoveredLink != prevLink {
		if r.hoveredLink >= 0 {
			sdl.SetCursor(r.handCursor)
		} else {
			sdl.SetCursor(r.arrowCursor)
		}
	}
	if r.treeItems != nil {
		lStartIdx := sort.Search(len(r.layout.lines), func(i int) bool {
			line := r.layout.lines[i]
			return line.y+line.h >= docY
		})
		for i := lStartIdx; i < len(r.layout.lines); i++ {
			line := r.layout.lines[i]
			if line.y > docY {
				break
			}
			if docY >= line.y && docY <= line.y+line.h {
				r.hoveredTreeLine = i
				return
			}
		}
	}
}

func (r *Renderer) isTreeLineHovered(idx int) bool {
	return idx >= 0 && idx < len(r.layout.lines) && r.hoveredTreeLine == idx
}

func (r *Renderer) HandleMouseLeave() {
	if r.hoveredLink >= 0 {
		r.hoveredLink = -1
		sdl.SetCursor(r.arrowCursor)
	}
}

func (r *Renderer) HandleTouch() {
	if !r.hasPointer {
		r.hasPointer = true
		r.selectedLink = -1
	}
}
