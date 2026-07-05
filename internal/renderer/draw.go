package renderer

import (
	"bytes"
	_ "golang.org/x/image/webp"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"unsafe"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

func (r *Renderer) Render() {
	r.sdlRenderer.SetDrawColor(r.bgColor.R, r.bgColor.G, r.bgColor.B, r.bgColor.A)
	r.sdlRenderer.Clear()
	r.renderImages()
	r.renderBlockquotes()
	r.renderCodeBackgrounds()
	r.renderLinkHighlight()
	r.renderLines()
	r.renderStatusBar()
	r.sdlRenderer.Present()
}

func (r *Renderer) renderImages() {
	for _, img := range r.imageEntries {
		screenY := img.y - r.scrollY
		if screenY <= -img.h || screenY >= r.height-statusBarHeight {
			continue
		}
		tex, ok := r.imageTextures[img.url]
		if !ok && r.loader != nil {
			data, err := r.loader(img.url)
			if err == nil {
				m, _, errDecode := image.Decode(bytes.NewReader(data))
				if errDecode == nil {
					bounds := m.Bounds()
					w, h := bounds.Dx(), bounds.Dy()
					rgba := image.NewRGBA(image.Rect(0, 0, w, h))
					draw.Draw(rgba, rgba.Bounds(), m, bounds.Min, draw.Src)
					t, errTex := r.sdlRenderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, int32(w), int32(h))
					if errTex == nil && len(rgba.Pix) > 0 {
						t.Update(nil, unsafe.Pointer(&rgba.Pix[0]), rgba.Stride)
						r.imageTextures[img.url] = t
						tex = t
					}
				}
			}
		}
		if tex != nil {
			r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: img.x, Y: screenY, W: img.w, H: img.h})
		}
	}
}

func (r *Renderer) renderCodeBackgrounds() {
	for _, cr := range r.codeRanges {
		screenY := cr.y - r.scrollY
		if screenY > -cr.h && screenY < r.height-statusBarHeight {
			r.sdlRenderer.SetDrawColor(r.codeBgColor.R, r.codeBgColor.G, r.codeBgColor.B, r.codeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: cr.x, Y: screenY, W: cr.w, H: cr.h})
		}
	}
	for _, cs := range r.codeSpans {
		screenY := cs.y - r.scrollY
		if screenY > -cs.h && screenY < r.height-statusBarHeight {
			r.sdlRenderer.SetDrawColor(r.codeBgColor.R, r.codeBgColor.G, r.codeBgColor.B, r.codeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: cs.x, Y: screenY, W: cs.w, H: cs.h})
		}
	}
}

func (r *Renderer) renderBlockquotes() {
	for _, bq := range r.blockquotes {
		screenY := bq.Y - r.scrollY
		if screenY > -bq.H && screenY < r.height-statusBarHeight {
			// Draw background
			r.sdlRenderer.SetDrawColor(r.blockquoteBgColor.R, r.blockquoteBgColor.G, r.blockquoteBgColor.B, r.blockquoteBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: bq.X, Y: screenY, W: bq.W, H: bq.H})
			// Draw thick left border
			r.sdlRenderer.SetDrawColor(r.blockquoteBorderColor.R, r.blockquoteBorderColor.G, r.blockquoteBorderColor.B, r.blockquoteBorderColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: bq.X, Y: screenY, W: 4, H: bq.H})
		}
	}
}

func (r *Renderer) renderLines() {
	for _, line := range r.lines {
		screenY := line.y - r.scrollY
		if screenY < -line.h || screenY > r.height-statusBarHeight {
			continue
		}
		if line.isCursor {
			r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
			r.sdlRenderer.SetDrawColor(r.selBgColor.R, r.selBgColor.G, r.selBgColor.B, r.selBgColor.A)
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
	if r.selectedLink < 0 || r.selectedLink >= len(r.links) {
		return
	}
	link := r.links[r.selectedLink]

	var mergedText []sdl.Rect
	var imgRects []sdl.Rect

	for _, rect := range link.rects {
		isImg := false
		for _, img := range r.imageEntries {
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
		r.sdlRenderer.SetDrawColor(r.selBgColor.R, r.selBgColor.G, r.selBgColor.B, r.selBgColor.A)
		for _, rect := range mergedText {
			r.sdlRenderer.FillRect(&rect)
		}
	}

	if len(imgRects) > 0 {
		r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
		r.sdlRenderer.SetDrawColor(r.selImgColor.R, r.selImgColor.G, r.selImgColor.B, r.selImgColor.A)
		for _, rect := range imgRects {
			r.sdlRenderer.FillRect(&rect)
		}
		r.sdlRenderer.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	}
}

func (r *Renderer) renderStatusBar() {
	r.sdlRenderer.SetDrawColor(r.codeBgColor.R, r.codeBgColor.G, r.codeBgColor.B, r.codeBgColor.A)
	r.sdlRenderer.FillRect(&sdl.Rect{X: 0, Y: r.height - statusBarHeight, W: r.width, H: statusBarHeight})
	r.sdlRenderer.SetDrawColor(r.ruleColor.R, r.ruleColor.G, r.ruleColor.B, r.ruleColor.A)
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
					statusText = "←→:links  ↑↓:scroll  L1/R1:page  A:open  X:back  Start:home  Select:tree  L2/R2:zoom  Menu:exit"
				} else if isMenu {
					statusText = "←→:links  ↑↓:scroll  L1/R1:page  A:open  X:back  L2/R2:zoom  Menu:exit"
				} else {
					statusText = "←→:links  ↑↓:scroll  L1/R1:page  A:open  X:back  Select:menu  L2/R2:zoom  Menu:exit"
				}
			} else {
				statusText = "↑↓:nav  L1/R1:page  A/→:enter  X/←:back  Select:doc  L2/R2:zoom  Menu:exit"
			}
		} else {
			if r.doc != nil {
				if r.hasTree {
					statusText = "←→:links  ↑↓:scroll  PgUp/PgDn:page  ↩:open  ⌫:back  H:home  T:tree  F:menu  D:theme  +/-:zoom  Q:exit"
				} else if isMenu {
					statusText = "←→:links  ↑↓:scroll  PgUp/PgDn:page  ↩:open  ⌫:back  D:theme  +/-:zoom  Q:exit"
				} else {
					statusText = "←→:links  ↑↓:scroll  PgUp/PgDn:page  ↩:open  ⌫:back  F:menu  D:theme  +/-:zoom  Q:exit"
				}
			} else {
				statusText = "↑↓:nav  PgUp/PgDn:page  ↩→:enter  ←⌫:back  T:doc  D:theme  +/-:zoom  Q:exit"
			}
		}
	}

	font := r.fonts[FontBody].font
	if font != nil {
		surf, err := font.RenderUTF8Blended(statusText, r.textColor)
		if err == nil {
			tex, err := r.sdlRenderer.CreateTextureFromSurface(surf)
			surf.Free()
			if err == nil {
				_, _, tw, th, _ := tex.Query()
				r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: 12, Y: r.height - statusBarHeight + (statusBarHeight-th)/2, W: tw, H: th})
				tex.Destroy()
			}
		}
	}
}

// Type aliases to keep sdl imports contained.
type sdlRect = sdl.Rect
type sdlColor = sdl.Color
