package renderer

import (
	"bytes"
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
	r.renderCodeBackgrounds()
	r.renderLines()
	r.renderLinkHighlight()
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
		screenY := cr.startY - r.scrollY
		screenH := cr.endY - cr.startY
		if screenY > -screenH && screenY < r.height-statusBarHeight {
			r.sdlRenderer.SetDrawColor(r.codeBgColor.R, r.codeBgColor.G, r.codeBgColor.B, r.codeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: r.marginX - 4, Y: screenY - 4, W: r.contentWidth + 8, H: screenH + 8})
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

func (r *Renderer) renderLines() {
	for _, line := range r.lines {
		screenY := line.y - r.scrollY
		if screenY < -line.h || screenY > r.height-statusBarHeight {
			continue
		}
		if line.isCursor {
			r.sdlRenderer.SetDrawColor(r.selBgColor.R, r.selBgColor.G, r.selBgColor.B, r.selBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{X: 0, Y: screenY, W: r.width, H: line.h})
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
			if line.isCode { font = r.fonts[FontMono].font } else { font = r.fonts[line.fontIdx].font }
			if font == nil { continue }
			style := ttf.STYLE_NORMAL
			if line.isBold && line.isItalic { style = ttf.STYLE_BOLD | ttf.STYLE_ITALIC
			} else if line.isBold { style = ttf.STYLE_BOLD
			} else if line.isItalic { style = ttf.STYLE_ITALIC }
			oldStyle := font.GetStyle()
			font.SetStyle(style)
			surf, err := font.RenderUTF8Blended(line.text, line.color)
			font.SetStyle(oldStyle)
			if err != nil { continue }
			tex, err = r.sdlRenderer.CreateTextureFromSurface(surf)
			surf.Free()
			if err != nil { continue }
			r.textureCache[key] = tex
		}
		_, _, tw, th, _ := tex.Query()
		r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: line.x, Y: screenY, W: tw, H: th})
	}
}

func (r *Renderer) renderLinkHighlight() {
	if r.selectedLink < 0 || r.selectedLink >= len(r.links) { return }
	link := r.links[r.selectedLink]
	sy := link.rect.Y - r.scrollY
	if sy < -link.rect.H || sy > r.height-statusBarHeight { return }
	r.sdlRenderer.SetDrawColor(r.selBgColor.R, r.selBgColor.G, r.selBgColor.B, r.selBgColor.A)
	r.sdlRenderer.FillRect(&sdl.Rect{X: link.rect.X - 2, Y: sy - 1, W: link.rect.W + 4, H: link.rect.H + 2})
	key := textureKey{text: link.label, fontIdx: FontBody, color: r.linkColor}
	tex, ok := r.textureCache[key]
	if !ok {
		font := r.fonts[FontBody].font
		if font != nil {
			surf, err := font.RenderUTF8Blended(link.label, r.linkColor)
			if err == nil {
				tex, err = r.sdlRenderer.CreateTextureFromSurface(surf)
				surf.Free()
				if err == nil { r.textureCache[key] = tex }
			}
		}
	}
	if tex != nil {
		_, _, tw, th, _ := tex.Query()
		r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: link.rect.X, Y: sy, W: tw, H: th})
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
