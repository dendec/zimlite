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
	r.blockquotes = nil
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

	if ls.y < r.height-statusBarHeight {
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
	inlines := []document.Inline{&document.Strong{
		Content: []document.Inline{&document.Text{Content: h.Content}},
	}}
	s.y = s.r.layoutInlines(inlines, fidx, s.r.headingColor, s.r.headingColor, s.maxW, s.y, 0, "")

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
	for idx, entry := range l.Entries {
		var prefix string
		if entry.Ordered {
			prefix = fmt.Sprintf("%d. ", entry.Start)
		} else {
			bullets := []string{"• ", "◦ ", "▪ ", "▫ "}
			prefix = bullets[entry.Indent%len(bullets)]
		}

		indentX := s.r.listIndent * int32(entry.Indent)
		itemW := s.maxW - indentX - s.r.listIndent
		if itemW < 50 {
			itemW = 50
		}
		if idx > 0 {
			s.y += s.r.lineSpacing / 2
		}

		s.y = s.r.layoutInlines(entry.Item, FontBody, s.r.textColor, s.r.linkColor, itemW, s.y, indentX, prefix)
	}
	s.y += s.r.blockSpacing
}

func (s *layoutState) VisitCodeBlock(c *document.CodeBlock) {
	startCodeY := s.y
	fontMono := s.r.fonts[FontMono].font
	codeText := strings.TrimSuffix(c.Code, "\n")

	oldMarginX := s.r.marginX
	oldMaxW := s.maxW

	s.y += 8 // top padding

	for _, cl := range strings.Split(codeText, "\n") {
		tw, th := measureText(cl, fontMono, false, false)
		s.r.lines = append(s.r.lines, lineEntry{
			text: cl, fontIdx: FontMono, color: s.r.textColor,
			x: s.r.marginX + 12, y: s.y, w: tw, h: th, // Text indent
			isCode: true,
		})
		s.y += th + 1
	}

	s.y += 8 // bottom padding

	s.r.codeRanges = append(s.r.codeRanges, codeBlockRange{
		x: oldMarginX, y: startCodeY, w: oldMaxW, h: s.y - startCodeY,
	})

	s.y += s.r.blockSpacing // margin for next block
}

func (s *layoutState) VisitThematicBreak(_ *document.ThematicBreak) {
	s.r.lines = append(s.r.lines, lineEntry{
		fontIdx: FontBody, color: s.r.ruleColor,
		x: s.r.marginX, y: s.y, w: s.maxW, h: 1,
	})
	s.y += 1 + s.r.blockSpacing
}

func (s *layoutState) VisitBlockquote(b *document.Blockquote) {
	oldMarginX := s.r.marginX
	oldMaxW := s.maxW

	s.r.marginX += 16
	s.maxW -= 16

	startY := s.y
	s.y += 8 // top padding

	document.VisitBlocks(b.Blocks, s)

	// Since child blocks add blockSpacing at their end, we subtract it to not double-pad,
	// or just accept it as bottom padding. Let's subtract blockSpacing and add our own.
	s.y -= s.r.blockSpacing
	s.y += 8 // bottom padding

	s.r.blockquotes = append(s.r.blockquotes, sdlRect{
		X: oldMarginX, Y: startY, W: oldMaxW, H: s.y - startY,
	})

	s.y += s.r.blockSpacing

	s.r.marginX = oldMarginX
	s.maxW = oldMaxW
}

func (s *layoutState) VisitLink(l *document.Link) {
	inlines := []document.Inline{&document.LinkInline{URL: l.URL, Content: []document.Inline{&document.Text{Content: l.Label}}}}
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

	measureImg := func(url string) (int32, int32) {
		if r.loader != nil && url != "" {
			data, err := r.loader(url)
			if err == nil {
				config, _, errConfig := image.DecodeConfig(bytes.NewReader(data))
				if errConfig == nil && config.Width > 0 && config.Height > 0 {
					w := int32(config.Width)
					h := int32(config.Height)
					scale := float64(maxW) / float64(w)
					if scale > 1.0 {
						scale = 1.0
					}
					return int32(float64(w) * scale), int32(float64(h) * scale)
				}
			}
		}
		return 0, 0
	}

	v := document.NewInlineWordVisitor(&sdlFont{r: r, baseIdx: fidx}, measureImg)
	document.VisitInlines(inlines, v)

	y := startY
	var lineWords []document.Word
	var lineWidth int32
	activeLinks := make(map[int]int)

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
			if w.Text == "" && !w.IsImage {
				continue
			}

			wordY := y + (maxH - w.PixH)

			if w.IsImage {
				r.imageEntries = append(r.imageEntries, imageEntry{
					x: currX, y: wordY, w: w.PixW, h: w.PixH, url: w.ImageURL,
				})
			} else {
				wColor := textColor
				if w.LinkID != 0 {
					wColor = linkColor
				}

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
			}

			if w.LinkID != 0 {
				idx, ok := activeLinks[w.LinkID]
				if !ok {
					idx = len(r.links)
					r.links = append(r.links, linkEntry{
						url: v.LinkURLs[w.LinkID],
					})
					activeLinks[w.LinkID] = idx
				}
				rect := sdlRect{X: currX, Y: wordY, W: w.PixW, H: w.PixH}
				r.links[idx].rects = append(r.links[idx].rects, rect)
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

		if lineWidth+w.PixW > maxW && len(lineWords) > 0 {
			flushLine(isFirst)
			isFirst = false
			if w.IsSpace {
				continue
			}
		}

		lineWords = append(lineWords, w)
		lineWidth += w.PixW
	}

	flushLine(isFirst)
	return y
}
