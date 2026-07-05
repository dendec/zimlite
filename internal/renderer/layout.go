package renderer

import (
	"fmt"
	"strings"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

// --- Layout engine ---

func (r *Renderer) relayout() {
	r.lines = nil
	r.links = nil
	r.codeRanges = nil
	r.totalHeight = 0

	if r.doc == nil {
		return
	}

	r.width, r.height = r.window.GetSize()
	maxW := r.width - 2*r.marginX
	if maxW < 100 {
		maxW = 100
	}
	r.contentWidth = maxW

	ls := &layoutState{
		r:    r,
		y:    r.marginY,
		maxW: maxW,
	}

	document.VisitBlocks(r.doc.Blocks, ls)

	if ls.y < r.height {
		ls.y = r.height
	}
	r.totalHeight = ls.y
	r.clampScroll()
	r.clampSelection()
}

type layoutState struct {
	r    *Renderer
	y    int32
	maxW int32
}

func (s *layoutState) VisitHeading(h *document.Heading) {
	fidx := headingFontIdx(h.Level)
	font := s.r.fonts[fidx].font
	tw, th := s.r.measure(h.Content, font)
	s.r.lines = append(s.r.lines, lineEntry{
		text:    h.Content,
		fontIdx: fidx,
		color:   s.r.headingColor,
		x:       s.r.marginX,
		y:       s.y,
		w:       tw,
		h:       th,
	})
	s.y += th + s.r.lineSpacing
}

func (s *layoutState) VisitParagraph(p *document.Paragraph) {
	s.y = s.r.layoutInlines(p.Inlines, s.r.fonts[FontBody].font, FontBody,
		s.r.textColor, s.r.linkColor, s.maxW, s.y)
}

func (s *layoutState) VisitList(l *document.List) {
	font := s.r.fonts[FontBody].font
	for idx, item := range l.Items {
		prefix := "• "
		if l.Ordered {
			prefix = fmt.Sprintf("%d. ", l.Start+idx)
		}
		pw, _ := s.r.measure(prefix, font)
		itemW := s.maxW - s.r.listIndent
		if itemW < 50 {
			itemW = 50
		}
		if idx > 0 {
			s.y += s.r.lineSpacing / 2
		}

		startY := s.y
		newY := s.r.layoutInlines(item, font, FontBody, s.r.textColor, s.r.linkColor, itemW, s.y)

		// Adjust first line of item: prepend prefix, shift subsequent lines.
		for i := len(s.r.lines) - 1; i >= 0; i-- {
			if s.r.lines[i].y >= startY && s.r.lines[i].y < newY {
				if s.r.lines[i].y == startY {
					s.r.lines[i].text = prefix + s.r.lines[i].text
					s.r.lines[i].x = s.r.marginX
					tw, th := s.r.measure(s.r.lines[i].text, font)
					s.r.lines[i].w = tw
					if th > s.r.lines[i].h {
						s.r.lines[i].h = th
					}
				} else {
					s.r.lines[i].x = s.r.marginX + s.r.listIndent
				}
			}
		}
		// Shift link rects for this item.
		for li := range s.r.links {
			lr := &s.r.links[li]
			if lr.rect.Y >= startY && lr.rect.Y < newY {
				if lr.rect.Y == startY {
					lr.rect.X += pw
				} else {
					lr.rect.X += s.r.listIndent
				}
			}
		}
		s.y = newY
	}
}

func (s *layoutState) VisitCodeBlock(c *document.CodeBlock) {
	startCodeY := s.y
	fontMono := s.r.fonts[FontMono].font
	for _, cl := range strings.Split(c.Code, "\n") {
		tw, th := s.r.measure(cl, fontMono)
		s.r.lines = append(s.r.lines, lineEntry{
			text: cl, fontIdx: FontMono, color: s.r.textColor,
			x: s.r.marginX + 8, y: s.y, w: tw, h: th,
		})
		s.y += th + 1
	}
	s.y += s.r.blockSpacing
	s.r.codeRanges = append(s.r.codeRanges, codeBlockRange{startY: startCodeY, endY: s.y})
}

func (s *layoutState) VisitThematicBreak(_ *document.ThematicBreak) {
	s.r.lines = append(s.r.lines, lineEntry{
		fontIdx: FontBody, color: s.r.ruleColor,
		x: s.r.marginX, y: s.y, w: s.maxW, h: 1,
	})
	s.y += 1 + s.r.blockSpacing
}

func (s *layoutState) VisitLink(l *document.Link) {
	font := s.r.fonts[FontBody].font
	pw, ph := s.r.measure(l.Label, font)
	s.r.links = append(s.r.links, linkEntry{
		rect: sdlRect{X: s.r.marginX, Y: s.y, W: pw, H: ph}, url: l.URL, label: l.Label,
	})
	s.r.lines = append(s.r.lines, lineEntry{
		text: l.Label, fontIdx: FontBody, color: s.r.linkColor,
		x: s.r.marginX, y: s.y, w: pw, h: ph,
	})
	s.y += ph + s.r.lineSpacing
}

func (s *layoutState) VisitImage(i *document.Image) {
	alt := i.Alt
	if alt == "" {
		alt = "[image]"
	}
	font := s.r.fonts[FontBody].font
	tw, th := s.r.measure(alt, font)
	s.r.lines = append(s.r.lines, lineEntry{
		text: alt, fontIdx: FontBody, color: sdlColor{R: 150, G: 150, B: 150, A: 255},
		x: s.r.marginX, y: s.y, w: tw, h: th,
	})
	s.y += th + s.r.lineSpacing
}

// --- Inline layout (word wrapping) ---

func (r *Renderer) layoutInlines(inlines []document.Inline, font *ttfFont, fidx FontKind,
	textColor, linkColor sdlColor, maxW int32, startY int32) int32 {

	linkMap := make(map[string]string)
	v := document.NewInlineWordVisitor(&sdlFont{font}, linkMap)
	document.VisitInlines(inlines, v)

	y := startY
	var lineWords []document.Word
	var lineWidth int32

	flushLine := func() {
		if len(lineWords) == 0 {
			return
		}
		var sb strings.Builder
		for _, w := range lineWords {
			sb.WriteString(w.Text)
		}
		text := sb.String()
		tw, th := r.measure(text, font)
		r.lines = append(r.lines, lineEntry{
			text: text, fontIdx: fidx, color: textColor,
			x: r.marginX, y: y, w: tw, h: th,
		})
		y += th + r.lineSpacing
		lineWords = nil
		lineWidth = 0
	}

	for _, w := range v.Words {
		if w.IsHardBreak {
			flushLine()
			continue
		}
		if w.IsSpace && len(lineWords) == 0 {
			continue
		}

		needSpace := int32(0)
		if len(lineWords) > 0 && !lineWords[len(lineWords)-1].IsSpace {
			needSpace = v.SpaceW
		}

		if lineWidth+needSpace+w.PixW > maxW && len(lineWords) > 0 {
			flushLine()
			if w.IsSpace {
				continue
			}
		}

		if len(lineWords) > 0 && !lineWords[len(lineWords)-1].IsSpace && !w.IsSpace {
			lineWords = append(lineWords, document.Word{Text: " ", IsSpace: true, PixW: v.SpaceW, PixH: v.SpaceH})
			lineWidth += v.SpaceW
		}

		lineWords = append(lineWords, w)
		lineWidth += w.PixW

		if w.IsLink {
			cum := int32(0)
			for i := 0; i < len(lineWords)-1; i++ {
				cum += lineWords[i].PixW
			}
			lx := r.marginX + cum
			ly := y + int32(font.Height()) - w.PixH
			if ly < y {
				ly = y
			}
			r.links = append(r.links, linkEntry{
				rect: sdlRect{X: lx, Y: ly, W: w.PixW, H: w.PixH},
				url:  linkMap[w.Text], label: w.Text,
			})
		}
	}

	flushLine()
	return y
}


