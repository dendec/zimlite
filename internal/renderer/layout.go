package renderer

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
)

// --- Layout engine ---

func (r *Renderer) relayout() {
	r.ClearCache()
	r.lines = nil
	r.links = nil
	r.codeRanges = nil
	r.codeSpans = nil
	r.imageEntries = nil
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

	if ls.y < r.height - statusBarHeight {
		ls.y = r.height - statusBarHeight
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
	charW := int32(font.Height() / 2) // monospace Unifont: char width ≈ height/2
	if charW < 1 { charW = 1 }
	maxChars := s.maxW / charW
	if maxChars < 10 { maxChars = 10 }
	lines := wrapText(h.Content, int(maxChars))
	for _, line := range lines {
		tw, th := s.r.measureHeading(line, fidx, true, false)
		s.r.lines = append(s.r.lines, lineEntry{
			text: line, fontIdx: fidx,
			color:  s.r.headingColor,
			x:      s.r.marginX,
			y:      s.y,
			w:      tw,
			h:      th,
			isBold: true,
		})
		s.y += th + 1
	}
	s.y -= 1
	if h.Level == 1 || h.Level == 2 {
		s.y += 4
		s.r.lines = append(s.r.lines, lineEntry{
			fontIdx: FontBody, color: s.r.ruleColor,
			x: s.r.marginX, y: s.y, w: s.maxW, h: 1,
		})
		s.y += 1 + s.r.blockSpacing
	} else {
		s.y += s.r.lineSpacing
	}
}

func (s *layoutState) VisitParagraph(p *document.Paragraph) {
	s.y = s.r.layoutInlines(p.Inlines, FontBody, s.r.textColor, s.r.linkColor, s.maxW, s.y, 0, "")
	s.y += s.r.blockSpacing
}

func (s *layoutState) VisitList(l *document.List) {
	for idx, item := range l.Items {
		var prefix string
		if l.Ordered {
			prefix = fmt.Sprintf("%d. ", l.Start+idx)
		} else {
			bullets := []string{"• ", "o ", "▪ ", "▫ "}
			prefix = bullets[l.Indent%len(bullets)]
		}
		
		indentX := s.r.listIndent * int32(l.Indent)
		itemW := s.maxW - indentX - s.r.listIndent
		if itemW < 50 {
			itemW = 50
		}
		if idx > 0 {
			s.y += s.r.lineSpacing / 2
		}

		s.y = s.r.layoutInlines(item, FontBody, s.r.textColor, s.r.linkColor, itemW, s.y, indentX, prefix)
	}
	s.y += s.r.blockSpacing
}

func (s *layoutState) VisitCodeBlock(c *document.CodeBlock) {
	startCodeY := s.y
	fontMono := s.r.fonts[FontMono].font
	for _, cl := range strings.Split(c.Code, "\n") {
		tw, th := measureText(cl, fontMono, false, false)
		s.r.lines = append(s.r.lines, lineEntry{
			text: cl, fontIdx: FontMono, color: s.r.textColor,
			x: s.r.marginX + 8, y: s.y, w: tw, h: th,
			isCode: true,
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
	inlines := []document.Inline{&document.LinkInline{URL: l.URL, Label: l.Label}}
	s.y = s.r.layoutInlines(inlines, FontBody, s.r.textColor, s.r.linkColor, s.maxW, s.y, 0, "")
}

func (s *layoutState) VisitImage(i *document.Image) {
	alt := i.Alt
	if alt == "" {
		alt = "[image]"
	}

	if s.r.loader != nil && i.URL != "" {
		data, err := s.r.loader(i.URL)
		if err == nil {
			config, _, errConfig := image.DecodeConfig(bytes.NewReader(data))
			if errConfig == nil && config.Width > 0 && config.Height > 0 {
				imgW := int32(config.Width)
				imgH := int32(config.Height)

				// Aspect ratio scale down
				scale := float64(s.maxW) / float64(imgW)
				if scale > 1.0 {
					scale = 1.0 // don't upscale
				}
				targetW := int32(float64(imgW) * scale)
				targetH := int32(float64(imgH) * scale)

				s.r.imageEntries = append(s.r.imageEntries, imageEntry{
					x:   s.r.marginX + (s.maxW-targetW)/2,
					y:   s.y,
					w:   targetW,
					h:   targetH,
					url: i.URL,
				})
				s.y += targetH + s.r.blockSpacing
				return
			}
		}
	}

	// Fallback to alt-text if image load/decode fails
	font := s.r.fonts[FontBody].font
	tw, th := measureText(alt, font, false, false)
	s.r.lines = append(s.r.lines, lineEntry{
		text: alt, fontIdx: FontBody, color: sdlColor{R: 150, G: 150, B: 150, A: 255},
		x: s.r.marginX, y: s.y, w: tw, h: th,
	})
	s.y += th + s.r.lineSpacing
}

func (r *Renderer) layoutInlines(inlines []document.Inline, fidx FontKind,
	textColor, linkColor sdlColor, maxW int32, startY int32, indentX int32, prefix string) int32 {

	linkMap := make(map[string]string)
	v := document.NewInlineWordVisitor(&sdlFont{r: r, baseIdx: fidx}, linkMap)
	document.VisitInlines(inlines, v)

	y := startY
	var lineWords []document.Word
	var lineWidth int32

	flushLine := func(isFirstLine bool) {
		if len(lineWords) == 0 && prefix == "" {
			return
		}
		
		currX := r.marginX + indentX
		
		if isFirstLine && prefix != "" {
			pFont := r.fonts[fidx].font
			pw, ph := measureText(prefix, pFont, false, false)
			r.lines = append(r.lines, lineEntry{
				text: prefix, fontIdx: fidx, color: textColor,
				x: currX, y: y, w: pw, h: ph,
			})
			currX += pw
			prefix = "" // clear so it only draws on first line
		}

		var maxH int32
		for _, w := range lineWords {
			if w.PixH > maxH {
				maxH = w.PixH
			}
		}
		if maxH == 0 {
			maxH = v.SpaceH
		}

		for _, w := range lineWords {
			if w.Text == "" {
				continue
			}
			wColor := textColor
			if w.IsLink {
				wColor = linkColor
			}
			
			wordY := y + (maxH - w.PixH)
			
			r.lines = append(r.lines, lineEntry{
				text: w.Text, fontIdx: fidx, color: wColor,
				x: currX, y: wordY, w: w.PixW, h: w.PixH,
				isBold: w.IsBold, isItalic: w.IsItalic, isCode: w.IsCode,
			})
			
			if w.IsCode {
				r.codeSpans = append(r.codeSpans, codeSpanRange{
					x: currX - 2, y: wordY - 2,
					w: w.PixW + 4, h: w.PixH + 4,
				})
			}
			
			if w.IsLink {
				r.links = append(r.links, linkEntry{
					rect: sdlRect{X: currX, Y: wordY, W: w.PixW, H: w.PixH},
					url:  linkMap[w.Text], label: w.Text,
				})
			}

			currX += w.PixW
		}
		
		y += maxH + r.lineSpacing
		lineWords = nil
		lineWidth = 0
	}

	isFirst := true
	for _, w := range v.Words {
		if w.IsHardBreak {
			flushLine(isFirst)
			isFirst = false
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
			flushLine(isFirst)
			isFirst = false
			if w.IsSpace {
				continue
			}
		}

		if len(lineWords) > 0 && !lineWords[len(lineWords)-1].IsSpace && !w.IsSpace {
			lineWords = append(lineWords, document.Word{
				Text: " ", IsSpace: true, PixW: v.SpaceW, PixH: v.SpaceH,
				IsBold: w.IsBold, IsItalic: w.IsItalic, IsCode: w.IsCode,
			})
			lineWidth += v.SpaceW
		}

		lineWords = append(lineWords, w)
		lineWidth += w.PixW
	}

	flushLine(isFirst)
	return y
}

func wrapText(text string, maxChars int) []string {
	if maxChars <= 0 || len([]rune(text)) <= maxChars {
		return []string{text}
	}
	var lines []string
	runes := []rune(text)
	for len(runes) > 0 {
		end := maxChars
		if end > len(runes) { end = len(runes) }
		lines = append(lines, string(runes[:end]))
		runes = runes[end:]
	}
	return lines
}


