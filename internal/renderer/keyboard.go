package renderer

import (
	"fmt"
	"log/slog"
	"unicode"
	"unsafe"

	"github.com/dendec/zimlite/internal/config"
	"github.com/dendec/zimlite/internal/textdraw"
	"github.com/dendec/zimlite/internal/textinput"
	"github.com/veandco/go-sdl2/sdl"
)

type keyboardTexture struct {
	texture *sdl.Texture
	w, h    int32
}

func (r *Renderer) KeyboardGeometry() textinput.Geometry {
	g := textinput.Panel(r.width, r.height, r.keyboardFontSize())
	bottom := max(int32(0), r.height-r.getStatusBarHeight())
	if r.keyboard != nil {
		if len(r.keyboard.Candidates) == 0 && r.keyboard.Status == "" {
			g.H -= g.CandidatesH
			g.CandidatesH = 0
		}
		g.Layout = r.keyboard.Layout
	}
	g.H = min(max(int32(0), g.H), bottom)
	g.Y = bottom - g.H
	return g
}

func (r *Renderer) keyboardFontSize() int {
	return max(config.MinFontSize, min(config.MaxFontSize, r.baseFontSize))
}

// Results occupy the area above the keyboard and persistent status bar.
func (r *Renderer) contentHeight() int32 {
	height := r.height - r.getStatusBarHeight()
	if r.keyboard != nil {
		height = min(height, r.KeyboardGeometry().Y)
	}
	return max(int32(1), height)
}
func (r *Renderer) clearKeyboardTextures() {
	for _, t := range r.keyboardTextures {
		t.texture.Destroy()
	}
	r.keyboardTextures = nil
	textdraw.Clear()
}
func (r *Renderer) SetKeyboard(view *textinput.View) {
	changedVisibility := (r.keyboard == nil) != (view == nil)
	defer func() {
		if changedVisibility && r.window != nil {
			r.relayout()
		}
	}()
	if view == nil {
		if r.keyboard != nil {
			r.clearKeyboardTextures()
		}
		r.keyboard = nil
		r.clampScroll()
		return
	}
	if r.keyboard != nil && r.keyboard.Language != view.Language {
		r.clearKeyboardTextures()
	}
	copy := *view
	copy.Layout.Keys = append([]textinput.Key(nil), view.Layout.Keys...)
	copy.Candidates = append([]string(nil), view.Candidates...)
	r.keyboard = &copy
	r.clampScroll()
}
func (r *Renderer) keyboardText(text string, rect sdl.Rect, center bool) {
	r.keyboardTextColor(text, rect, center, r.theme.TextColor)
}
func (r *Renderer) keyboardTextColor(text string, rect sdl.Rect, center bool, color sdl.Color) {
	if text == "" || rect.W < 1 || rect.H < 1 {
		return
	}
	// Keep the visible edit tail bounded; committed text remains intact in the model.
	runes := []rune(text)
	if len(runes) > 180 {
		text = "…" + string(runes[len(runes)-180:])
	}
	size := r.keyboardFontSize()
	key := fmt.Sprintf("%d:%s", size, text)
	if r.keyboardTextures == nil {
		r.keyboardTextures = map[string]keyboardTexture{}
	}
	cached, ok := r.keyboardTextures[key]
	if !ok {
		if len(r.keyboardTextures) >= 128 {
			r.clearKeyboardTextures()
			r.keyboardTextures = map[string]keyboardTexture{}
		}
		pixels, w, h := textdraw.Render(textinput.RuntimeDir(), text, size, 4096)
		if len(pixels) > 0 {
			// Transparent font ascender padding should not shrink small key captions.
			top, bottom := 0, h
			rowVisible := func(y int) bool {
				for x := 0; x < w; x++ {
					if pixels[(y*w+x)*4+3] != 0 {
						return true
					}
				}
				return false
			}
			for top < bottom && !rowVisible(top) {
				top++
			}
			for bottom > top && !rowVisible(bottom-1) {
				bottom--
			}
			if bottom > top {
				pixels = pixels[top*w*4 : bottom*w*4]
				h = bottom - top
			}
			tex, err := r.sdlRenderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STATIC, int32(w), int32(h))
			if err == nil {
				if err = tex.Update(nil, unsafe.Pointer(&pixels[0]), w*4); err == nil {
					tex.SetBlendMode(sdl.BLENDMODE_BLEND)
					cached = keyboardTexture{tex, int32(w), int32(h)}
					r.keyboardTextures[key] = cached
					ok = true
				} else {
					tex.Destroy()
				}
			}
		}
	}
	if !ok {
		// English input remains usable even if the optional runtime is not installed.
		tex := r.renderTextTexture(lineEntry{text: text, fontIdx: FontBody, color: sdl.Color{R: 255, G: 255, B: 255, A: 255}})
		if tex == nil {
			return
		}
		_, _, w, h, err := tex.Query()
		if err != nil {
			return
		}
		cached = keyboardTexture{tex, w, h}
	}
	tex := cached.texture
	if err := tex.SetColorMod(color.R, color.G, color.B); err != nil {
		return
	}
	w, h := cached.w, cached.h
	if h > rect.H {
		w = w * rect.H / h
		h = rect.H
	}
	var src *sdl.Rect
	if w > rect.W {
		if center {
			h = max(int32(1), h*rect.W/w)
			w = rect.W
		} else {
			// A trailing crop follows Latin/LTR editing; RTL paragraphs begin at the left.
			crop := cached.w * rect.W / w
			x := cached.w - crop
			for _, ch := range text {
				if unicode.IsLetter(ch) {
					if unicode.In(ch, unicode.Arabic, unicode.Hebrew) {
						x = 0
					}
					break
				}
			}
			src = &sdl.Rect{X: x, W: crop, H: cached.h}
			w = rect.W
		}
	}
	x := rect.X
	if center {
		x += (rect.W - w) / 2
	}
	r.sdlRenderer.Copy(tex, src, &sdl.Rect{X: x, Y: rect.Y + (rect.H-h)/2, W: w, H: h})
}

func (r *Renderer) keyboardFill(rect sdl.Rect, color sdl.Color) {
	r.sdlRenderer.SetDrawColor(color.R, color.G, color.B, 255)
	r.sdlRenderer.FillRect(&rect)
}
func (r *Renderer) renderKeyboard() {
	v := r.keyboard
	if v == nil {
		return
	}
	g := r.KeyboardGeometry()
	if g.H < 24 || g.W < 1 {
		return
	}
	r.keyboardFill(sdl.Rect{X: g.X, Y: g.Y, W: g.W, H: g.H}, r.theme.StatusBarBgColor)
	// Separate the editable query from a compact language/method badge.
	label := v.Language
	labelW := g.W / 3
	badge := sdl.Rect{X: g.X + g.W - labelW - 5, Y: g.Y + 3, W: labelW, H: g.QueryH - 4}
	r.keyboardFill(badge, r.theme.TableHeaderBgColor)
	r.keyboardText(label, badge, true)
	field := sdl.Rect{X: g.X + 5, Y: g.Y + 3, W: g.W - labelW - 15, H: g.QueryH - 4}
	r.keyboardFill(field, r.theme.BgColor)
	r.keyboardText(v.Text+v.Preedit+"│", sdl.Rect{X: field.X + 5, Y: field.Y, W: field.W - 10, H: field.H}, false)
	r.sdlRenderer.SetDrawColor(r.theme.LinkColor.R, r.theme.LinkColor.G, r.theme.LinkColor.B, 255)
	r.sdlRenderer.DrawLine(field.X+4, field.Y+field.H-1, field.X+field.W-5, field.Y+field.H-1)
	strip := sdl.Rect{X: g.X + 5, Y: g.Y + g.QueryH, W: g.W - 10, H: g.CandidatesH}
	if len(v.Candidates) == 0 {
		r.keyboardText(v.Status, strip, false)
	} else {
		width := strip.W / int32(len(v.Candidates))
		for i, candidate := range v.Candidates {
			rect := sdl.Rect{X: strip.X + int32(i)*width, Y: strip.Y + 1, W: width - 3, H: strip.H - 2}
			color := r.theme.TextColor
			if i == v.Candidate {
				background := r.theme.TableHeaderBgColor
				if v.Focus < 0 {
					background = r.theme.LinkColor
					color = r.theme.BgColor
				}
				r.keyboardFill(rect, background)
			}
			r.keyboardTextColor(candidate, rect, true, color)
		}
	}
	keyInset := max(int32(1), int32(r.keyboardFontSize()/8))
	for i, key := range v.Layout.Keys {
		x, y, w, h := g.KeyRect(i)
		rect := sdl.Rect{X: x + keyInset, Y: y + keyInset, W: w - 2*keyInset, H: h - 2*keyInset}
		background, foreground := r.theme.CodeBgColor, r.theme.TextColor
		if key.Action != textinput.Character && key.Action != textinput.Literal {
			background = r.theme.TableHeaderBgColor
		}
		focused := i == v.Focus
		if focused {
			background = r.theme.LinkColor
			foreground = r.theme.BgColor
		}
		if focused {
			if key.Action == textinput.Language {
				background = r.theme.BgColor
			}
			r.keyboardFill(rect, r.theme.TextColor)
			rect.X++
			rect.Y++
			rect.W -= 2
			rect.H -= 2
		}
		r.keyboardFill(rect, background)
		if key.Action == textinput.Shift && v.Layer == 1 && !focused {
			foreground = r.theme.LinkColor
		}
		if r.keyboardIcon(key.Action, rect, foreground, v.Layer) {
			continue
		}
		r.keyboardTextColor(key.Label, sdl.Rect{X: rect.X + 3, Y: rect.Y, W: rect.W - 6, H: rect.H}, true, foreground)
	}
}

// Reuse the embedded emoji renderer for language; draw the remaining controls as vectors.
func (r *Renderer) keyboardIcon(action textinput.KeyAction, rect sdl.Rect, color sdl.Color, layer int) bool {
	if action == textinput.Language {
		size := max(int32(4), min(int32(r.keyboardFontSize()), rect.W-6, rect.H-4))
		texture := r.renderEmojiTexture(lineEntry{isEmoji: true, emojiHex: "1f310", h: size})
		if texture == nil {
			return false
		}
		r.sdlRenderer.Copy(texture, nil, &sdl.Rect{X: rect.X + (rect.W-size)/2, Y: rect.Y + (rect.H-size)/2, W: size, H: size})
		return true
	}
	if action != textinput.Delete && action != textinput.Submit && (action != textinput.Shift || layer >= 2) {
		return false
	}
	r.sdlRenderer.SetDrawColor(color.R, color.G, color.B, 255)
	cx, cy := rect.X+rect.W/2, rect.Y+rect.H/2
	radius := max(int32(2), min(int32(r.keyboardFontSize()/2), (rect.H-3)/2, (rect.W-6)/3))
	point := func(x, y int32) sdl.Point { return sdl.Point{X: cx + x*radius/8, Y: cy + y*radius/8} }
	line := func(points ...sdl.Point) bool {
		if err := r.sdlRenderer.DrawLines(points); err != nil {
			slog.Debug("draw SDL on-screen keyboard icon", "error", err)
			return false
		}
		return true
	}
	switch action {
	case textinput.Delete:
		if !line(point(-9, 0), point(-3, -7), point(9, -7), point(9, 7), point(-3, 7), point(-9, 0)) ||
			!line(point(-1, -3), point(5, 3)) || !line(point(5, -3), point(-1, 3)) {
			return false
		}
	case textinput.Shift:
		if !line(point(-7, 0), point(0, -7), point(7, 0), point(3, 0), point(3, 7), point(-3, 7), point(-3, 0), point(-7, 0)) {
			return false
		}
		if layer == 1 {
			if !line(point(-5, 8), point(5, 8)) {
				return false
			}
		}
	case textinput.Submit:
		if !line(point(7, -6), point(7, 2), point(-7, 2)) || !line(point(-2, -3), point(-7, 2), point(-2, 7)) {
			return false
		}
	}
	return true
}
