package renderer

import (
	"sort"

	"github.com/dendec/zimlite/internal/document"
	"github.com/dendec/zimlite/internal/i18n"
	"github.com/dendec/zimlite/internal/svg"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

const (
	blockquoteBorderWidth = 4
	linkUnderlineHeight   = 2
	scrollbarMinThumb     = 20
	scrollbarWidth        = 5
	scrollbarTrackAlpha   = 80
	scrollbarThumbAlpha   = 220
	statusTextXPadding    = 12
	statusTextMaxW        = 24
)

// viewportEach iterates items within the visible viewport.
// getYH returns (y, h) for item at index i. fn is called with index and screenY.
func (r *Renderer) viewportEach(n int, getYH func(i int) (int32, int32), fn func(i int, screenY int32)) {
	if n == 0 {
		return
	}
	startIdx := sort.Search(n, func(i int) bool {
		y, h := getYH(i)
		return y+h > r.scrollY
	})
	vpBottom := r.height - r.getStatusBarHeight()
	for i := startIdx; i < n; i++ {
		y, _ := getYH(i)
		screenY := y - r.scrollY
		if screenY >= vpBottom {
			break
		}
		fn(i, screenY)
	}
}

func (r *Renderer) Render() {
	r.hasActiveAnimations = false
	r.sdlRenderer.SetDrawColor(r.theme.BgColor.R, r.theme.BgColor.G, r.theme.BgColor.B, r.theme.BgColor.A)
	r.sdlRenderer.Clear()
	r.renderImages()
	r.renderBlockquotes()
	r.renderCodeBackgrounds()
	r.renderTables()
	r.renderLinkUnderline()
	r.renderLines()
	r.renderScrollbar()
	r.renderStatusBar()
	r.sdlRenderer.Present()
}

func (r *Renderer) renderImages() {
	r.viewportEach(len(r.layout.imageEntries),
		func(i int) (int32, int32) { return r.layout.imageEntries[i].y, r.layout.imageEntries[i].h },
		func(i int, screenY int32) {
			img := r.layout.imageEntries[i]
			tex, isAnim := r.imgManager.GetTexture(img.url)
			if isAnim {
				r.hasActiveAnimations = true
			}
			if tex != nil {
				r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: img.x, Y: screenY, W: img.w, H: img.h})
			}
		})
}

func (r *Renderer) renderCodeBackgrounds() {
	r.viewportEach(len(r.layout.codeRanges),
		func(i int) (int32, int32) { return r.layout.codeRanges[i].y, r.layout.codeRanges[i].h },
		func(i int, screenY int32) {
			cr := r.layout.codeRanges[i]
			r.sdlRenderer.SetDrawColor(r.theme.CodeBgColor.R, r.theme.CodeBgColor.G, r.theme.CodeBgColor.B, r.theme.CodeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: cr.x, Y: screenY, W: cr.w, H: cr.h})
		})

	r.viewportEach(len(r.layout.codeSpans),
		func(i int) (int32, int32) { return r.layout.codeSpans[i].y, r.layout.codeSpans[i].h },
		func(i int, screenY int32) {
			cs := r.layout.codeSpans[i]
			r.sdlRenderer.SetDrawColor(r.theme.CodeBgColor.R, r.theme.CodeBgColor.G, r.theme.CodeBgColor.B, r.theme.CodeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: cs.x, Y: screenY, W: cs.w, H: cs.h})
		})
}

func (r *Renderer) renderBlockquotes() {
	r.viewportEach(len(r.layout.blockquotes),
		func(i int) (int32, int32) { return r.layout.blockquotes[i].Y, r.layout.blockquotes[i].H },
		func(i int, screenY int32) {
			bq := r.layout.blockquotes[i]
			r.sdlRenderer.SetDrawColor(r.theme.BlockquoteBgColor.R, r.theme.BlockquoteBgColor.G, r.theme.BlockquoteBgColor.B, r.theme.BlockquoteBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: bq.X, Y: screenY, W: bq.W, H: bq.H})
			r.sdlRenderer.SetDrawColor(r.theme.BlockquoteBorderColor.R, r.theme.BlockquoteBorderColor.G, r.theme.BlockquoteBorderColor.B, r.theme.BlockquoteBorderColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: bq.X, Y: screenY, W: blockquoteBorderWidth, H: bq.H})
		})
}
func (r *Renderer) renderTables() {
	for _, table := range r.layout.tables {
		// Fill header row background.
		if table.hasHeader {
			screenY := table.headerRect.Y - r.scrollY
			if screenY > -table.headerRect.H && screenY < r.height-r.getStatusBarHeight() {
				r.sdlRenderer.SetDrawColor(
					r.theme.TableHeaderBgColor.R, r.theme.TableHeaderBgColor.G,
					r.theme.TableHeaderBgColor.B, r.theme.TableHeaderBgColor.A)
				r.sdlRenderer.FillRect(&sdl.Rect{
					X: table.headerRect.X, Y: screenY,
					W: table.headerRect.W, H: table.headerRect.H,
				})
			}
		}
		// Draw subtle horizontal row separators without vertical grid lines.
		r.sdlRenderer.SetDrawColor(r.theme.TableBorderColor.R, r.theme.TableBorderColor.G, r.theme.TableBorderColor.B, r.theme.TableBorderColor.A)
		seenLines := make(map[int32]struct{}, len(table.cellRects)*2)
		for _, cell := range table.cellRects {
			for _, lineY := range []int32{cell.Y, cell.Y + cell.H} {
				if _, seen := seenLines[lineY]; seen {
					continue
				}
				seenLines[lineY] = struct{}{}
				screenY := lineY - r.scrollY
				if screenY < 0 || screenY >= r.height-r.getStatusBarHeight() {
					continue
				}
				r.sdlRenderer.DrawLine(
					table.cellRects[0].X, screenY,
					table.cellRects[len(table.cellRects)-1].X+table.cellRects[len(table.cellRects)-1].W-1, screenY,
				)
			}
		}
	}
}

func (r *Renderer) renderLines() {
	r.viewportEach(len(r.layout.lines),
		func(i int) (int32, int32) { return r.layout.lines[i].y, r.layout.lines[i].h },
		func(i int, screenY int32) {
			line := r.layout.lines[i]
			if line.text == "" {
				if line.h <= 2 {
					r.sdlRenderer.SetDrawColor(line.color.R, line.color.G, line.color.B, line.color.A)
					r.sdlRenderer.FillRect(&sdl.Rect{X: line.x, Y: screenY, W: line.w, H: line.h})
				}
				return
			}
			if line.labelW > 0 {
				r.renderTreeLineParts(line, screenY)
			} else {
				tex := r.renderLineTexture(line)
				if tex != nil {
					_, _, tw, th, _ := tex.Query()
					r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: line.x, Y: screenY, W: tw, H: th})
				}
			}
			if line.isCursor || r.hoveredTreeLine >= 0 && r.isTreeLineHovered(i) {
				if line.labelW > 0 {
					underlineY := screenY + line.h
					r.sdlRenderer.SetDrawColor(r.theme.LinkColor.R, r.theme.LinkColor.G, r.theme.LinkColor.B, r.theme.LinkColor.A)
					r.sdlRenderer.FillRect(&sdl.Rect{X: line.labelX, Y: underlineY, W: line.labelW, H: 1})
				}
			}
		})
}

// renderTreeLineParts draws a tree line with prefix in TextColor, label in LinkColor, suffix in TextColor.
func (r *Renderer) renderTreeLineParts(line lineEntry, screenY int32) {
	font := r.fonts[line.fontIdx].font
	if font == nil {
		return
	}
	runes := []rune(line.text)
	prefixEnd := line.prefixRuneN
	labelEnd := prefixEnd + line.labelRuneN

	prefixText := string(runes[:prefixEnd])
	labelText := string(runes[prefixEnd:labelEnd])
	suffixText := string(runes[labelEnd:])

	if prefixText != "" {
		r.renderColoredText(prefixText, font, line.fontIdx, r.theme.TextColor, line.x, screenY)
	}
	if labelText != "" {
		r.renderColoredText(labelText, font, line.fontIdx, r.theme.LinkColor, line.labelX, screenY)
	}
	if suffixText != "" {
		suffixX := line.labelX + line.labelW
		r.renderColoredText(suffixText, font, line.fontIdx, r.theme.TextColor, suffixX, screenY)
	}
}

type textSegment struct {
	tex     *sdl.Texture
	w, h    int32
	isEmoji bool
}

func (r *Renderer) createTextSegments(text string, font *ttf.Font, fontIdx FontKind, color sdl.Color) ([]textSegment, int32) {
	var segments []textSegment
	var totalW int32
	if text == "" || font == nil {
		return segments, 0
	}
	sz := int32(font.Ascent())
	runes := []rune(text)

	i := 0
	for i < len(runes) {
		hex, consumed, ok := document.EmojiSequence(runes, i)
		var tex *sdl.Texture
		if ok {
			le := lineEntry{isEmoji: true, emojiHex: hex, h: sz}
			tex = r.renderEmojiTexture(le)
		}
		if tex != nil {
			_, _, ew, eh, _ := tex.Query()
			segments = append(segments, textSegment{tex: tex, w: ew, h: eh, isEmoji: true})
			totalW += ew
			i += consumed
			continue
		}
		start := i
		for i < len(runes) {
			hex2, _, ok2 := document.EmojiSequence(runes, i)
			var tex2 *sdl.Texture
			if ok2 {
				le2 := lineEntry{isEmoji: true, emojiHex: hex2, h: sz}
				tex2 = r.renderEmojiTexture(le2)
			}
			if tex2 != nil {
				break
			}
			i++
		}
		textStr := string(runes[start:i])
		key := textureKey{text: textStr, fontIdx: fontIdx, color: color}
		texText := r.textCache.Get(key)
		if texText == nil {
			surf, err := font.RenderUTF8Blended(textStr, color)
			if err != nil {
				continue
			}
			texText, err = r.sdlRenderer.CreateTextureFromSurface(surf)
			surf.Free()
			if err != nil {
				continue
			}
			r.textCache.Set(key, texText)
		}
		_, _, tw, th, _ := texText.Query()
		segments = append(segments, textSegment{tex: texText, w: tw, h: th})
		totalW += tw
	}
	return segments, totalW
}

func (r *Renderer) renderColoredText(text string, font *ttf.Font, fontIdx FontKind, color sdl.Color, x, y int32) {
	segments, _ := r.createTextSegments(text, font, fontIdx, color)
	cx := x
	for _, s := range segments {
		r.sdlRenderer.Copy(s.tex, nil, &sdl.Rect{X: cx, Y: y, W: s.w, H: s.h})
		cx += s.w
	}
}

func (r *Renderer) renderLinkUnderline() {
	if r.selectedLink >= 0 && r.selectedLink < len(r.layout.links) {
		r.drawLinkUnderline(r.selectedLink)
	}
	if r.hoveredLink >= 0 && r.hoveredLink < len(r.layout.links) && r.hoveredLink != r.selectedLink {
		r.drawLinkUnderline(r.hoveredLink)
	}
}

// drawLinkUnderline draws a 2px underline under a link rect.
func (r *Renderer) drawLinkUnderline(idx int) {
	// Build image rect set for O(1) lookup.
	imgSet := make(map[[4]int32]struct{}, len(r.layout.imageEntries))
	for _, img := range r.layout.imageEntries {
		imgSet[[4]int32{img.x, img.y, img.w, img.h}] = struct{}{}
	}
	link := r.layout.links[idx]
	for _, rect := range link.rects {
		if _, ok := imgSet[[4]int32{rect.X, rect.Y, rect.W, rect.H}]; ok {
			continue
		}

		sy := rect.Y - r.scrollY
		if sy < -rect.H || sy > r.height-r.getStatusBarHeight() {
			continue
		}

		underlineY := sy + rect.H
		r.sdlRenderer.SetDrawColor(r.theme.LinkColor.R, r.theme.LinkColor.G, r.theme.LinkColor.B, r.theme.LinkColor.A)
		r.sdlRenderer.FillRect(&sdl.Rect{X: rect.X, Y: underlineY, W: rect.W, H: linkUnderlineHeight})
	}
}

func (r *Renderer) renderStatusBar() {
	r.sdlRenderer.SetDrawColor(r.theme.StatusBarBgColor.R, r.theme.StatusBarBgColor.G, r.theme.StatusBarBgColor.B, r.theme.StatusBarBgColor.A)
	r.sdlRenderer.FillRect(&sdl.Rect{X: 0, Y: r.height - r.getStatusBarHeight(), W: r.width, H: r.getStatusBarHeight()})
	r.sdlRenderer.SetDrawColor(r.theme.StatusBarBorderColor.R, r.theme.StatusBarBorderColor.G, r.theme.StatusBarBorderColor.B, r.theme.StatusBarBorderColor.A)
	r.sdlRenderer.FillRect(&sdl.Rect{X: 0, Y: r.height - r.getStatusBarHeight(), W: r.width, H: 1})

	font := r.fonts[FontBody].font
	if font == nil {
		return
	}

	if r.statusOverride != "" {
		r.renderStatusText(r.statusOverride, statusTextXPadding, r.width-statusTextMaxW)
		return
	}

	// Right: scroll% + link count
	rightText := r.computeRightStatus()
	if rightText == "" {
		if r.treeItems != nil {
			text := i18n.T(r.lang, "status.tree")
			if r.docTitle != "" {
				runes := []rune(r.docTitle)
				if len(runes) > 0 {
					if _, _, ok := document.EmojiSequence(runes, 0); ok {
						text = r.docTitle
					} else {
						text = i18n.T(r.lang, "status.tree") + " " + r.docTitle
					}
				}
			}
			r.renderStatusText(text, statusTextXPadding, r.width-statusTextMaxW)
		}
		return
	}

	rightW, _ := measureText(rightText, font, false, false, false)
	gap := int32(statusTextMaxW)
	rightX := r.width - rightW - statusTextXPadding
	availLeft := rightX - gap - statusTextXPadding
	if availLeft < 20 {
		availLeft = 0
	}

	leftText := r.docTitle
	if leftText != "" {
		runes := []rune(leftText)
		if len(runes) > 0 {
			if _, _, ok := document.EmojiSequence(runes, 0); !ok {
				leftText = "📄 " + leftText
			}
		}
		leftW, _ := measureText(leftText, font, false, false, false)
		if leftW > availLeft {
			runes := []rune(leftText)
			dotW, _ := measureText("...", font, false, false, false)
			n := truncateRunesToWidth(runes, font, availLeft-dotW)
			if n > 0 && n < len(runes) {
				leftText = string(runes[:n]) + "..."
			}
		}
		r.renderStatusText(leftText, statusTextXPadding, availLeft)
	}

	r.renderStatusText(rightText, rightX, rightW)
}

func (r *Renderer) computeRightStatus() string {
	if r.doc == nil {
		return ""
	}

	vpH := r.height - r.getStatusBarHeight()
	totalH := r.layout.totalHeight
	scrollPct := 0
	if totalH > vpH {
		maxScroll := totalH - vpH
		scrollPct = int(float64(r.scrollY) / float64(maxScroll) * 100)
		if scrollPct > 100 {
			scrollPct = 100
		}
	} else {
		scrollPct = 100
	}

	linkCount := len(r.layout.links)
	if linkCount > 0 {
		sel := r.selectedLink + 1
		if sel < 1 {
			sel = 1
		}
		return i18n.Tf(r.lang, "status.scroll_link", scrollPct, sel, linkCount)
	}
	return i18n.Tf(r.lang, "status.scroll_pct", scrollPct)
}

func (r *Renderer) renderStatusText(text string, x int32, maxW int32) {
	if text == "" || maxW <= 0 {
		return
	}
	font := r.fonts[FontBody].font
	segments, totalW := r.createTextSegments(text, font, FontBody, r.theme.TextColor)

	if len(segments) == 0 {
		return
	}

	scale := float64(1)
	if totalW > maxW {
		scale = float64(maxW) / float64(totalW)
	}

	curX := x
	for _, s := range segments {
		dw := int32(float64(s.w) * scale)
		dh := int32(float64(s.h) * scale)
		if dw <= 0 {
			continue
		}
		dstY := r.height - r.getStatusBarHeight() + (r.getStatusBarHeight()-dh)/2
		r.sdlRenderer.Copy(s.tex, nil, &sdl.Rect{X: curX, Y: dstY, W: dw, H: dh})
		curX += dw
	}
}

func (r *Renderer) renderScrollbar() {
	vpHeight := r.height - r.getStatusBarHeight()
	totalH := r.layout.totalHeight

	if totalH <= vpHeight {
		return // No need for scrollbar
	}

	// Calculate thumb size
	thumbH := int32(float64(vpHeight) * float64(vpHeight) / float64(totalH))
	if thumbH < scrollbarMinThumb {
		thumbH = scrollbarMinThumb
	}

	// Calculate thumb position
	maxScroll := totalH - vpHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	var thumbY int32
	if maxScroll > 0 {
		thumbY = int32(float64(r.scrollY) / float64(maxScroll) * float64(vpHeight-thumbH))
	}

	// Draw track.
	r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	r.sdlRenderer.SetDrawColor(r.theme.RuleColor.R, r.theme.RuleColor.G, r.theme.RuleColor.B, scrollbarTrackAlpha)
	r.sdlRenderer.FillRect(&sdl.Rect{X: r.width - scrollbarWidth, Y: 0, W: scrollbarWidth, H: vpHeight})

	// Draw thumb.
	r.sdlRenderer.SetDrawColor(r.theme.RuleColor.R, r.theme.RuleColor.G, r.theme.RuleColor.B, scrollbarThumbAlpha)
	r.sdlRenderer.FillRect(&sdl.Rect{X: r.width - scrollbarWidth, Y: thumbY, W: scrollbarWidth, H: thumbH})
	r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_NONE)
}

func (r *Renderer) renderLineTexture(line lineEntry) *sdl.Texture {
	if line.isEmoji {
		return r.renderEmojiTexture(line)
	}
	return r.renderTextTexture(line)
}

func (r *Renderer) renderEmojiTexture(line lineEntry) *sdl.Texture {
	sz := line.h
	if sz < 4 {
		sz = 4
	}
	ek := emojiCacheKey{hex: line.emojiHex, size: sz}
	tex := r.emojiCache.Get(ek)
	if tex != nil {
		return tex
	}
	data, err := getEmojiSVG(line.emojiHex)
	if err != nil {
		return nil
	}
	img := svg.RenderToSize(data, int(sz), int(sz))
	if img == nil {
		return nil
	}
	return r.createEmojiTexture(img, line.emojiHex, sz)
}

func (r *Renderer) renderTextTexture(line lineEntry) *sdl.Texture {
	key := textureKey{
		text: line.text, fontIdx: line.fontIdx, color: line.color,
		isBold: line.isBold, isItalic: line.isItalic, isCode: line.isCode,
	}
	tex := r.textCache.Get(key)
	if tex != nil {
		return tex
	}
	font := r.fonts[line.fontIdx].font
	if font == nil {
		return nil
	}
	style := fontStyle(line.isBold, line.isItalic)
	oldStyle := font.GetStyle()
	font.SetStyle(style)
	surf, err := font.RenderUTF8Blended(line.text, line.color)
	font.SetStyle(oldStyle)
	if err != nil {
		return nil
	}
	tex, err = r.sdlRenderer.CreateTextureFromSurface(surf)
	surf.Free()
	if err != nil {
		return nil
	}
	r.textCache.Set(key, tex)
	return tex
}

// Type aliases to keep sdl imports contained.
type sdlRect = sdl.Rect
type sdlColor = sdl.Color
