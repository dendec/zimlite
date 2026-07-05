package renderer

import (
	"fmt"
	"strings"

	"github.com/kiwix-sdl/kiwix-sdl/internal/document"
	"github.com/veandco/go-sdl2/ttf"
)

// --- Layout engine ---

const (
	minContentWidth         = 100
	headingBottomMargin     = 4
	minListItemWidth        = 50
	codeBlockPadding        = 8
	codeBlockLeftPadding    = 12
	defaultCodeLineHeight   = 18
	blockquoteIndent        = 16
	blockquotePadding       = 8
	tableCellPadding        = 4
	minTableCellWidth       = 10
	fallbackImageColorValue = 150
	codeSpanPadding         = 2
)

func (r *Renderer) relayout() {
	r.ClearCache()
	r.layout = PageLayout{
		anchorPositions: make(map[string]int32),
	}

	if r.doc == nil {
		return
	}

	r.width, r.height = r.window.GetSize()
	maxW := r.width - 2*r.marginX
	if maxW < minContentWidth {
		maxW = minContentWidth
	}
	r.layout.contentWidth = maxW

	ls := &layoutState{
		r:    r,
		y:    r.marginY,
		maxW: maxW,
	}

	document.VisitBlocks(r.doc.Blocks, ls)

	if ls.y < r.height-statusBarHeight {
		ls.y = r.height - statusBarHeight
	}
	r.layout.totalHeight = ls.y
	r.clampScroll()
	r.clampSelection()
}

type layoutState struct {
	document.BaseBlockVisitor
	r    *Renderer
	y    int32
	maxW int32
}

func (s *layoutState) VisitAnchor(a *document.Anchor) {
	s.r.layout.anchorPositions[a.ID] = s.y
}

func (s *layoutState) VisitHeading(h *document.Heading) {
	fidx := headingFontIdx(h.Level)
	inlines := []document.Inline{&document.Strong{
		Content: []document.Inline{&document.Text{Content: h.Content}},
	}}
	startY := s.y
	s.y = s.r.layoutInlines(inlines, fidx, s.r.theme.HeadingColor, s.r.theme.HeadingColor, s.maxW, s.y, 0, "")

	if h.ID != "" {
		s.r.layout.anchorPositions[h.ID] = startY
	}

	if h.Level == 1 || h.Level == 2 {
		s.y += headingBottomMargin
		s.r.layout.lines = append(s.r.layout.lines, lineEntry{
			fontIdx: FontBody, color: s.r.theme.RuleColor,
			x: s.r.marginX, y: s.y, w: s.maxW, h: 1,
		})
		s.y += 1 + s.r.blockSpacing
	} else {
		s.y += s.r.lineSpacing
	}
}

func (s *layoutState) VisitParagraph(p *document.Paragraph) {
	s.y = s.r.layoutInlines(p.Inlines, FontBody, s.r.theme.TextColor, s.r.theme.LinkColor, s.maxW, s.y, 0, "")
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
		if itemW < minListItemWidth {
			itemW = minListItemWidth
		}
		if idx > 0 {
			s.y += s.r.lineSpacing / 2
		}

		s.y = s.r.layoutInlines(entry.Item, FontBody, s.r.theme.TextColor, s.r.theme.LinkColor, itemW, s.y, indentX, prefix)
	}
	s.y += s.r.blockSpacing
}

// wrapCodeLine splits a code line into chunks that fit within maxW pixels.
// Each chunk is split at word boundaries (space/tab/dash/comma) when possible.
func wrapCodeLine(text string, font *ttf.Font, maxW int32) []string {
	tw, _ := measureText(text, font, false, false, false)
	if tw <= maxW {
		return []string{text}
	}

	var lines []string
	textLeft := text
	for len(textLeft) > 0 {
		tw, _ = measureText(textLeft, font, false, false, false)
		if tw <= maxW {
			lines = append(lines, textLeft)
			break
		}

		runes := []rune(textLeft)
		fitCount := truncateRunesToWidth(runes, font, maxW)

		breakAt := fitCount
		for i := fitCount - 1; i > 0; i-- {
			if runes[i] == ' ' || runes[i] == '\t' || runes[i] == '-' || runes[i] == ',' {
				breakAt = i + 1
				break
			}
		}
		if breakAt > 0 && breakAt < fitCount && breakAt > fitCount/2 {
			fitCount = breakAt
		}

		lines = append(lines, string(runes[:fitCount]))
		textLeft = string(runes[fitCount:])
	}
	return lines
}

func (s *layoutState) VisitCodeBlock(c *document.CodeBlock) {
	startCodeY := s.y
	font := s.r.fonts[FontBody].font
	codeText := strings.TrimSuffix(c.Code, "\n")

	oldMarginX := s.r.marginX
	oldMaxW := s.maxW

	pad := int32(codeBlockPadding)
	leftPad := int32(codeBlockLeftPadding)

	s.y += pad

	maxCodeW := s.maxW - 2*leftPad
	if maxCodeW < pad*6+2 {
		maxCodeW = pad * 6
	}

	addLine := func(text string) {
		tw, th := measureText(text, font, false, false, false)
		if th == 0 {
			th = defaultCodeLineHeight
		}
		s.r.layout.lines = append(s.r.layout.lines, lineEntry{
			text: text, fontIdx: FontBody, color: s.r.theme.TextColor,
			x: s.r.marginX + leftPad, y: s.y, w: tw, h: th,
			isCode: true,
		})
		s.y += th + 1
	}

	for _, cl := range strings.Split(codeText, "\n") {
		if cl == "" {
			addLine(" ")
			continue
		}

		for _, line := range wrapCodeLine(cl, font, maxCodeW) {
			addLine(line)
		}
	}

	s.y += pad

	s.r.layout.codeRanges = append(s.r.layout.codeRanges, codeBlockRange{
		x: oldMarginX, y: startCodeY, w: oldMaxW, h: s.y - startCodeY,
	})

	s.y += s.r.blockSpacing // margin for next block
}

func (s *layoutState) VisitThematicBreak(_ *document.ThematicBreak) {
	s.r.layout.lines = append(s.r.layout.lines, lineEntry{
		fontIdx: FontBody, color: s.r.theme.RuleColor,
		x: s.r.marginX, y: s.y, w: s.maxW, h: 1,
	})
	s.y += 1 + s.r.blockSpacing
}

func (s *layoutState) VisitBlockquote(b *document.Blockquote) {
	oldMarginX := s.r.marginX
	oldMaxW := s.maxW

	s.r.marginX += blockquoteIndent
	s.maxW -= blockquoteIndent

	startY := s.y
	s.y += blockquotePadding // top padding

	document.VisitBlocks(b.Blocks, s)

	// Since child blocks add blockSpacing at their end, we subtract it to not double-pad,
	// or just accept it as bottom padding. Let's subtract blockSpacing and add our own.
	s.y -= s.r.blockSpacing
	s.y += blockquotePadding // bottom padding

	s.r.layout.blockquotes = append(s.r.layout.blockquotes, sdlRect{
		X: oldMarginX, Y: startY, W: oldMaxW, H: s.y - startY,
	})

	s.y += s.r.blockSpacing

	s.r.marginX = oldMarginX
	s.maxW = oldMaxW
}

func (s *layoutState) VisitLink(l *document.Link) {
	inlines := []document.Inline{&document.LinkInline{URL: l.URL, Content: []document.Inline{&document.Text{Content: l.Label}}}}
	s.y = s.r.layoutInlines(inlines, FontBody, s.r.theme.TextColor, s.r.theme.LinkColor, s.maxW, s.y, 0, "")
}

func scaleToFit(imgW, imgH, maxW, maxH int32) (int32, int32) {
	scaleW := float64(maxW) / float64(imgW)
	scaleH := float64(maxH) / float64(imgH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	if scale > 1.0 {
		scale = 1.0
	}
	return int32(float64(imgW) * scale), int32(float64(imgH) * scale)
}

func (s *layoutState) VisitImage(i *document.Image) {
	alt := i.Alt
	if alt == "" {
		alt = "[image]"
	}

	if imgW, imgH, ok := s.r.imgManager.GetDimensions(i.URL); ok && imgW > 0 && imgH > 0 {
		maxImgW := int32(float64(s.r.width) / 2.0)
		if s.maxW < maxImgW {
			maxImgW = s.maxW
		}
		maxImgH := int32(float64(s.r.height) / 2.0)
		targetW, targetH := scaleToFit(imgW, imgH, maxImgW, maxImgH)

		s.r.layout.imageEntries = append(s.r.layout.imageEntries, imageEntry{
			x:   s.r.marginX + (s.maxW-targetW)/2,
			y:   s.y,
			w:   targetW,
			h:   targetH,
			url: i.URL,
		})
		s.y += targetH + s.r.blockSpacing
		return
	}

	// Fallback to alt-text if image load/decode fails
	font := s.r.fonts[FontBody].font
	tw, th := measureText(alt, font, false, false, false)
	s.r.layout.lines = append(s.r.layout.lines, lineEntry{
		text: alt, fontIdx: FontBody, color: sdlColor{R: fallbackImageColorValue, G: fallbackImageColorValue, B: fallbackImageColorValue, A: 255},
		x: s.r.marginX, y: s.y, w: tw, h: th,
	})
	s.y += th + s.r.lineSpacing
}

func rowHasContent(row document.TableRow) bool {
	for _, cell := range row.Cells {
		for _, inline := range cell.Inlines {
			if t, ok := inline.(*document.Text); ok {
				if strings.TrimSpace(t.Content) != "" {
					return true
				}
			} else {
				return true
			}
		}
	}
	return false
}

func (s *layoutState) VisitTable(t *document.Table) {
	if len(t.Rows) == 0 {
		return
	}

	var activeRows []document.TableRow
	for _, row := range t.Rows {
		if row.IsHeader && !rowHasContent(row) {
			continue
		}
		activeRows = append(activeRows, row)
	}

	if len(activeRows) == 0 {
		return
	}

	colCount := len(activeRows[0].Cells)
	if colCount == 0 {
		return
	}

	colWidths := make([]int32, colCount)
	padding := int32(tableCellPadding)
	spaceW, _ := measureText(" ", s.r.fonts[FontBody].font, false, false, false)

	// Pass 1: measure natural column widths
	for _, row := range activeRows {
		for cIdx, cell := range row.Cells {
			if int(cIdx) >= colCount {
				continue
			}
			w := s.r.measureInlinesWidth(cell.Inlines, FontBody) + 2*padding + spaceW
			if w > colWidths[cIdx] {
				colWidths[cIdx] = w
			}
		}
	}

	var totalW int32
	for _, w := range colWidths {
		totalW += w
	}

	// Expand to fill width or shrink proportionally
	if totalW > s.maxW {
		scale := float64(s.maxW) / float64(totalW)
		for i := range colWidths {
			colWidths[i] = int32(float64(colWidths[i]) * scale)
		}
	} else if totalW < s.maxW && colCount > 0 {
		colWidths[0] += s.maxW - totalW
	}

	var tableGrid tableGridEntry

	for _, row := range activeRows {
		maxH := int32(0)
		yStart := s.y

		// Layout text and find row height
		for cIdx, cell := range row.Cells {
			if int(cIdx) >= colCount {
				continue // ignore extra cells
			}

			var cellXOffset int32
			for i := 0; i < cIdx; i++ {
				cellXOffset += colWidths[i]
			}

			cellW := colWidths[cIdx]
			cellX := s.r.marginX + cellXOffset
			cellY := s.y + padding
			cellMaxW := cellW - 2*padding
			if cellMaxW < minTableCellWidth {
				cellMaxW = minTableCellWidth
			}

			// Measure a space to use as left padding for the cell content
			spaceW, _ := measureText(" ", s.r.fonts[FontBody].font, false, false, false)

			// Render cell text with spaceW added to indentX
			bottomY := s.r.layoutInlines(cell.Inlines, FontBody, s.r.theme.TextColor, s.r.theme.LinkColor, cellMaxW-spaceW, cellY, cellX-s.r.marginX+spaceW, "")

			h := bottomY - cellY
			if h > maxH {
				maxH = h
			}
		}

		// Adjust y for next row
		if maxH == 0 {
			maxH = s.r.lineSpacing // empty row fallback
		}
		rowH := maxH + 2*padding

		// Record cell rects
		var cellXOffset int32
		for cIdx := 0; cIdx < colCount; cIdx++ {
			tableGrid.cellRects = append(tableGrid.cellRects, sdlRect{
				X: s.r.marginX + cellXOffset,
				Y: yStart,
				W: colWidths[cIdx],
				H: rowH,
			})
			cellXOffset += colWidths[cIdx]
		}

		s.y += rowH
	}

	s.r.layout.tables = append(s.r.layout.tables, tableGrid)
	s.y += s.r.blockSpacing
}

func (r *Renderer) measureInlinesWidth(inlines []document.Inline, fidx FontKind) int32 {
	measureImg := func(url string) (int32, int32) {
		if w, h, ok := r.imgManager.GetDimensions(url); ok && w > 0 && h > 0 {
			return int32(w), int32(h)
		}
		return 0, 0
	}
	v := document.NewInlineWordVisitor(&sdlFont{r: r, baseIdx: fidx}, measureImg)
	document.VisitInlines(inlines, v)

	var maxW, currW int32
	for _, w := range v.Words {
		if w.IsHardBreak {
			if currW > maxW {
				maxW = currW
			}
			currW = 0
		} else {
			currW += w.PixW
		}
	}
	if currW > maxW {
		maxW = currW
	}
	return maxW
}

func (s *layoutState) flushInlineLine(
	lineWords []document.Word, isFirstLine bool,
	fidx FontKind, textColor, linkColor sdlColor,
	indentX int32, prefix *string, y *int32,
	v *document.InlineWordVisitor, activeLinks map[int]int,
) {
	if len(lineWords) == 0 && *prefix == "" {
		return
	}

	currX := s.r.marginX + indentX

	if isFirstLine && *prefix != "" {
		pFont := s.r.fonts[fidx].font
		pw, ph := measureText(*prefix, pFont, false, false, false)
		s.r.layout.lines = append(s.r.layout.lines, lineEntry{
			text: *prefix, fontIdx: fidx, color: textColor,
			x: currX, y: *y, w: pw, h: ph,
		})
		currX += pw
		*prefix = ""
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
		if w.Text == "" && !w.IsImage && !w.IsEmoji {
			continue
		}

		wordY := *y + (maxH - w.PixH)

		if w.IsImage {
			s.r.layout.imageEntries = append(s.r.layout.imageEntries, imageEntry{
				x: currX, y: wordY, w: w.PixW, h: w.PixH, url: w.ImageURL,
			})
		} else if w.IsEmoji {
			wColor := textColor
			if w.LinkID != 0 {
				wColor = linkColor
			}
			s.r.layout.lines = append(s.r.layout.lines, lineEntry{
				text: w.Text, fontIdx: fidx, color: wColor,
				x: currX, y: wordY, w: w.PixW, h: w.PixH,
				isEmoji: true, emojiHex: w.EmojiHex,
			})
		} else {
			wColor := textColor
			if w.LinkID != 0 {
				wColor = linkColor
			}
			s.r.layout.lines = append(s.r.layout.lines, lineEntry{
				text: w.Text, fontIdx: fidx, color: wColor,
				x: currX, y: wordY, w: w.PixW, h: w.PixH,
				isBold: w.IsBold, isItalic: w.IsItalic, isCode: w.IsCode,
			})
			if w.IsCode {
				s.r.layout.codeSpans = append(s.r.layout.codeSpans, codeSpanRange{
					x: currX - codeSpanPadding, y: wordY - codeSpanPadding,
					w: w.PixW + codeSpanPadding*2, h: w.PixH + codeSpanPadding*2,
				})
			}
		}

		if w.LinkID != 0 {
			idx, ok := activeLinks[w.LinkID]
			if !ok {
				idx = len(s.r.layout.links)
				s.r.layout.links = append(s.r.layout.links, linkEntry{
					url: v.LinkURLs[w.LinkID],
				})
				activeLinks[w.LinkID] = idx
			}
			rect := sdlRect{X: currX, Y: wordY, W: w.PixW, H: w.PixH}
			s.r.layout.links[idx].rects = append(s.r.layout.links[idx].rects, rect)
		}

		currX += w.PixW
	}

	*y += maxH + s.r.lineSpacing
}

func (r *Renderer) layoutInlines(inlines []document.Inline, fidx FontKind,
	textColor, linkColor sdlColor, maxW int32, startY int32, indentX int32, prefix string) int32 {

	measureImg := func(url string) (int32, int32) {
		if w, h, ok := r.imgManager.GetDimensions(url); ok && w > 0 && h > 0 {
			maxImgW := int32(float64(r.width) / 2.0)
			if maxW < maxImgW {
				maxImgW = maxW
			}
			maxImgH := int32(float64(r.height) / 2.0)
			return scaleToFit(w, h, maxImgW, maxImgH)
		}
		return 0, 0
	}

	v := document.NewInlineWordVisitor(&sdlFont{r: r, baseIdx: fidx}, measureImg)
	document.VisitInlines(inlines, v)

	ls := &layoutState{r: r}
	y := startY
	var lineWords []document.Word
	var lineWidth int32
	activeLinks := make(map[int]int)

	flush := func(isFirstLine bool) {
		if len(lineWords) == 0 && prefix == "" {
			return
		}
		ls.flushInlineLine(lineWords, isFirstLine, fidx, textColor, linkColor, indentX, &prefix, &y, v, activeLinks)
		lineWords = nil
		lineWidth = 0
	}

	isFirst := true
	for _, w := range v.Words {
		if w.IsHardBreak {
			flush(isFirst)
			isFirst = false
			continue
		}
		if w.IsSpace && len(lineWords) == 0 {
			continue
		}

		if lineWidth+w.PixW > maxW && len(lineWords) > 0 {
			flush(isFirst)
			isFirst = false
			if w.IsSpace {
				continue
			}
		}

		lineWords = append(lineWords, w)
		lineWidth += w.PixW
	}

	flush(isFirst)
	return y
}
