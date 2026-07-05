package renderer

import (
	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

func (r *Renderer) Render() {
	r.sdlRenderer.SetDrawColor(r.theme.BgColor.R, r.theme.BgColor.G, r.theme.BgColor.B, r.theme.BgColor.A)
	r.sdlRenderer.Clear()
	r.renderImages()
	r.renderBlockquotes()
	r.renderCodeBackgrounds()
	r.renderTables(false)
	r.renderLinkHighlight()
	r.renderLines()
	r.renderScrollbar()
	r.renderStatusBar()
	r.sdlRenderer.Present()
}

func (r *Renderer) renderImages() {
	for _, img := range r.layout.imageEntries {
		screenY := img.y - r.scrollY
		if screenY <= -img.h || screenY >= r.height-statusBarHeight {
			continue
		}
		tex := r.imgManager.GetTexture(img.url)
		if tex != nil {
			r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: img.x, Y: screenY, W: img.w, H: img.h})
		}
	}
}

func (r *Renderer) renderCodeBackgrounds() {
	for _, cr := range r.layout.codeRanges {
		screenY := cr.y - r.scrollY
		if screenY > -cr.h && screenY < r.height-statusBarHeight {
			r.sdlRenderer.SetDrawColor(r.theme.CodeBgColor.R, r.theme.CodeBgColor.G, r.theme.CodeBgColor.B, r.theme.CodeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: cr.x, Y: screenY, W: cr.w, H: cr.h})
		}
	}
	for _, cs := range r.layout.codeSpans {
		screenY := cs.y - r.scrollY
		if screenY > -cs.h && screenY < r.height-statusBarHeight {
			r.sdlRenderer.SetDrawColor(r.theme.CodeBgColor.R, r.theme.CodeBgColor.G, r.theme.CodeBgColor.B, r.theme.CodeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: cs.x, Y: screenY, W: cs.w, H: cs.h})
		}
	}
}

func (r *Renderer) renderBlockquotes() {
	for _, bq := range r.layout.blockquotes {
		screenY := bq.Y - r.scrollY
		if screenY > -bq.H && screenY < r.height-statusBarHeight {
			// Draw background
			r.sdlRenderer.SetDrawColor(r.theme.BlockquoteBgColor.R, r.theme.BlockquoteBgColor.G, r.theme.BlockquoteBgColor.B, r.theme.BlockquoteBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: bq.X, Y: screenY, W: bq.W, H: bq.H})
			// Draw thick left border
			r.sdlRenderer.SetDrawColor(r.theme.BlockquoteBorderColor.R, r.theme.BlockquoteBorderColor.G, r.theme.BlockquoteBorderColor.B, r.theme.BlockquoteBorderColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: bq.X, Y: screenY, W: 4, H: bq.H})
		}
	}
}

func (r *Renderer) renderTables(visible bool) {
	if !visible {
		return
	}
	r.sdlRenderer.SetDrawColor(r.theme.RuleColor.R, r.theme.RuleColor.G, r.theme.RuleColor.B, r.theme.RuleColor.A)
	for _, table := range r.layout.tables {
		for _, cellRect := range table.cellRects {
			screenY := cellRect.Y - r.scrollY
			if screenY > -cellRect.H && screenY < r.height-statusBarHeight {
				r.sdlRenderer.DrawRect(&sdl.Rect{X: cellRect.X, Y: screenY, W: cellRect.W, H: cellRect.H})
			}
		}
	}
}

func (r *Renderer) renderLines() {
	for _, line := range r.layout.lines {
		screenY := line.y - r.scrollY
		if screenY < -line.h || screenY > r.height-statusBarHeight {
			continue
		}
		if line.isCursor {
			r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
			r.sdlRenderer.SetDrawColor(r.theme.SelBgColor.R, r.theme.SelBgColor.G, r.theme.SelBgColor.B, r.theme.SelBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: 0, Y: screenY, W: r.width, H: line.h})
			r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_NONE)
		}
		if line.text == "" {
			if line.h <= 2 {
				r.sdlRenderer.SetDrawColor(line.color.R, line.color.G, line.color.B, line.color.A)
				r.sdlRenderer.FillRect(&sdl.Rect{X: line.x, Y: screenY, W: line.w, H: line.h})
			}
			continue
		}
		key := textureKey{text: line.text, fontIdx: line.fontIdx, color: line.color, isBold: line.isBold, isItalic: line.isItalic, isCode: line.isCode}
		tex, ok := r.textureCache[key]
		if !ok {
			var font *ttf.Font
			if line.isCode {
				font = r.fonts[FontMono].font
			} else {
				font = r.fonts[line.fontIdx].font
			}
			if font == nil {
				continue
			}
			style := ttf.STYLE_NORMAL
			if line.isBold && line.isItalic {
				style = ttf.STYLE_BOLD | ttf.STYLE_ITALIC
			} else if line.isBold {
				style = ttf.STYLE_BOLD
			} else if line.isItalic {
				style = ttf.STYLE_ITALIC
			}
			oldStyle := font.GetStyle()
			font.SetStyle(style)
			surf, err := font.RenderUTF8Blended(line.text, line.color)
			font.SetStyle(oldStyle)
			if err != nil {
				continue
			}
			tex, err = r.sdlRenderer.CreateTextureFromSurface(surf)
			surf.Free()
			if err != nil {
				continue
			}
			r.textureCache[key] = tex
		}
		_, _, tw, th, _ := tex.Query()
		r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: line.x, Y: screenY, W: tw, H: th})
	}
}

func (r *Renderer) renderLinkHighlight() {
	if r.selectedLink < 0 || r.selectedLink >= len(r.layout.links) {
		return
	}
	link := r.layout.links[r.selectedLink]

	var mergedText []sdl.Rect
	var imgRects []sdl.Rect

	for _, rect := range link.rects {
		isImg := false
		for _, img := range r.layout.imageEntries {
			if img.x == rect.X && img.y == rect.Y && img.w == rect.W && img.h == rect.H {
				isImg = true
				break
			}
		}

		sy := rect.Y - r.scrollY
		if sy < -rect.H || sy > r.height-statusBarHeight {
			continue
		}
		newR := sdl.Rect{X: rect.X - 2, Y: sy - 1, W: rect.W + 4, H: rect.H + 2}

		if isImg {
			imgRects = append(imgRects, newR)
		} else {
			if len(mergedText) > 0 {
				last := &mergedText[len(mergedText)-1]
				if last.Y == newR.Y {
					newRight := newR.X + newR.W
					lastRight := last.X + last.W
					if newRight > lastRight {
						last.W = newRight - last.X
					}
					continue
				}
			}
			mergedText = append(mergedText, newR)
		}
	}

	if len(mergedText) > 0 {
		r.sdlRenderer.SetDrawColor(r.theme.SelBgColor.R, r.theme.SelBgColor.G, r.theme.SelBgColor.B, r.theme.SelBgColor.A)
		for _, rect := range mergedText {
			r.sdlRenderer.FillRect(&rect)
		}
	}

	if len(imgRects) > 0 {
		r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
		r.sdlRenderer.SetDrawColor(r.theme.SelImgColor.R, r.theme.SelImgColor.G, r.theme.SelImgColor.B, r.theme.SelImgColor.A)
		for _, rect := range imgRects {
			r.sdlRenderer.FillRect(&rect)
		}
		r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	}
}

func (r *Renderer) renderStatusBar() {
	r.sdlRenderer.SetDrawColor(r.theme.CodeBgColor.R, r.theme.CodeBgColor.G, r.theme.CodeBgColor.B, r.theme.CodeBgColor.A)
	r.sdlRenderer.FillRect(&sdl.Rect{X: 0, Y: r.height - statusBarHeight, W: r.width, H: statusBarHeight})
	r.sdlRenderer.SetDrawColor(r.theme.RuleColor.R, r.theme.RuleColor.G, r.theme.RuleColor.B, r.theme.RuleColor.A)
	r.sdlRenderer.FillRect(&sdl.Rect{X: 0, Y: r.height - statusBarHeight, W: r.width, H: 1})

	var statusText string
	if r.statusOverride != "" {
		statusText = r.statusOverride
	} else {
		isMenu := false
		if r.doc != nil && len(r.doc.Blocks) > 0 {
			if h, ok := r.doc.Blocks[0].(*document.Heading); ok {
				isMenu = h.Level == 1 && h.Content == "Kiwix SDL Document Menu"
			}
		}
		if sdl.NumJoysticks() > 0 {
			if r.doc != nil {
				if r.hasTree {
					statusText = "←→:links  ↑↓:scroll  L1/R1:page  A:open  B:back  Y:home  X:tree  Select:theme  L2/R2:zoom  Start:exit"
				} else if isMenu {
					statusText = "←→:links  ↑↓:scroll  L1/R1:page  A:open  B:back  Select:theme  L2/R2:zoom  Start:exit"
				} else {
					statusText = "←→:links  ↑↓:scroll  L1/R1:page  A:open  B:back  Y:menu  Select:theme  L2/R2:zoom  Start:exit"
				}
			} else {
				statusText = "↑↓:nav  L1/R1:page  A/→:enter  B/←:back  X:doc  Select:theme  L2/R2:zoom  Start:exit"
			}
		} else {
			if r.doc != nil {
				if r.hasTree {
					statusText = "WASD/Arrows:nav  Space:page  Enter/LMB:open  Back/RMB:back  M:menu  T:tree  C:theme  +/-:zoom  Q:exit"
				} else if isMenu {
					statusText = "WASD/Arrows:nav  Space:page  Enter/LMB:open  Back/RMB:back  C:theme  +/-:zoom  Q:exit"
				} else {
					statusText = "WASD/Arrows:nav  Space:page  Enter/LMB:open  Back/RMB:back  M:menu  C:theme  +/-:zoom  Q:exit"
				}
			} else {
				statusText = "WS/Up/Dn:nav  Space:page  Enter/LMB:open  Back/RMB:back  T:doc  C:theme  +/-:zoom  Q:exit"
			}
		}
	}

	font := r.fonts[FontBody].font
	if font != nil {
		surf, err := font.RenderUTF8Blended(statusText, r.theme.TextColor)
		if err == nil {
			tex, err := r.sdlRenderer.CreateTextureFromSurface(surf)
			surf.Free()
			if err == nil {
				_, _, tw, th, _ := tex.Query()
				w, h := tw, th
				maxWidth := r.width - 24
				if w > maxWidth && maxWidth > 0 {
					ratio := float64(maxWidth) / float64(w)
					w = maxWidth
					h = int32(float64(h) * ratio)
				}
				r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: 12, Y: r.height - statusBarHeight + (statusBarHeight-h)/2, W: w, H: h})
				tex.Destroy()
			}
		}
	}
}

func (r *Renderer) renderScrollbar() {
	vpHeight := r.height - statusBarHeight
	totalH := r.layout.totalHeight

	if totalH <= vpHeight {
		return // No need for scrollbar
	}

	// Calculate thumb size
	thumbH := int32(float64(vpHeight) * float64(vpHeight) / float64(totalH))
	if thumbH < 20 {
		thumbH = 20
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

	// Draw track (optional, using theme.CodeBgColor or RuleColor)
	// r.sdlRenderer.SetDrawColor(r.theme.RuleColor.R, r.theme.RuleColor.G, r.theme.RuleColor.B, 100)
	// r.sdlRenderer.FillRect(&sdl.Rect{X: r.width - 8, Y: 0, W: 8, H: vpHeight})

	// Draw thumb
	r.sdlRenderer.SetDrawColor(r.theme.RuleColor.R, r.theme.RuleColor.G, r.theme.RuleColor.B, 200)
	r.sdlRenderer.FillRect(&sdl.Rect{X: r.width - 6, Y: thumbY, W: 6, H: thumbH})
}

// Type aliases to keep sdl imports contained.
type sdlRect = sdl.Rect
type sdlColor = sdl.Color
