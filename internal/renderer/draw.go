package renderer

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

// --- Rendering ---

func (r *Renderer) Render() {
	r.sdlRenderer.SetDrawColor(r.bgColor.R, r.bgColor.G, r.bgColor.B, r.bgColor.A)
	r.sdlRenderer.Clear()

	// Draw code block backgrounds first (behind text).
	for _, cr := range r.codeRanges {
		screenY := cr.startY - r.scrollY
		screenH := cr.endY - cr.startY
		if screenY > -screenH && screenY < r.height {
			r.sdlRenderer.SetDrawColor(r.codeBgColor.R, r.codeBgColor.G, r.codeBgColor.B, r.codeBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{
				X: r.marginX - 4, Y: screenY - 4,
				W: r.contentWidth + 8, H: screenH + 8,
			})
		}
	}

	// Draw lines.
	for _, line := range r.lines {
		screenY := line.y - r.scrollY
		if screenY < -line.h || screenY > r.height {
			continue
		}
		if line.text == "" {
			if line.h <= 2 {
				r.sdlRenderer.SetDrawColor(line.color.R, line.color.G, line.color.B, line.color.A)
				r.sdlRenderer.FillRect(&sdl.Rect{X: line.x, Y: screenY, W: line.w, H: line.h})
			}
			continue
		}
		font := r.fonts[line.fontIdx].font
		if font == nil {
			continue
		}
		surf, err := font.RenderUTF8Blended(line.text, line.color)
		if err != nil {
			continue
		}
		tex, err := r.sdlRenderer.CreateTextureFromSurface(surf)
		surf.Free()
		if err != nil {
			continue
		}
		_, _, tw, th, _ := tex.Query()
		r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: line.x, Y: screenY, W: tw, H: th})
		tex.Destroy()
	}

	// Highlight selected link.
	if r.selectedLink >= 0 && r.selectedLink < len(r.links) {
		link := r.links[r.selectedLink]
		sy := link.rect.Y - r.scrollY
		if sy >= -link.rect.H && sy <= r.height {
			r.sdlRenderer.SetDrawColor(r.selBgColor.R, r.selBgColor.G, r.selBgColor.B, r.selBgColor.A)
			r.sdlRenderer.FillRect(&sdl.Rect{
				X: link.rect.X - 2, Y: sy - 1,
				W: link.rect.W + 4, H: link.rect.H + 2,
			})
			font := r.fonts[FontBody].font
			surf, err := font.RenderUTF8Blended(link.label, r.linkColor)
			if err == nil {
				tex, err := r.sdlRenderer.CreateTextureFromSurface(surf)
				surf.Free()
				if err == nil {
					_, _, tw, th, _ := tex.Query()
					r.sdlRenderer.Copy(tex, nil, &sdl.Rect{X: link.rect.X, Y: sy, W: tw, H: th})
					tex.Destroy()
				}
			}
		}
	}

	r.sdlRenderer.Present()
}

// Type aliases to keep sdl imports contained.
type sdlRect = sdl.Rect
type sdlColor = sdl.Color
type ttfFont = ttf.Font
