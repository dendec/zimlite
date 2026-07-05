// Package document defines the internal document model.
// Renderer works only with these types — never with markdown or HTML directly.
package document

// Document is the universal internal representation.
type Document struct {
	Blocks []Block
}

// --- Block types ---

type Block interface{ blockMarker() }

type Heading struct {
	Level   int // 1–6
	Content string
}

func (*Heading) blockMarker() {}

type Paragraph struct {
	Inlines []Inline
}

func (*Paragraph) blockMarker() {}

type List struct {
	Items   [][]Inline
	Ordered bool
	Start   int
	Indent  int
}

func (*List) blockMarker() {}

type Link struct {
	URL   string
	Label string
}

func (*Link) blockMarker() {}

type Image struct {
	Alt string
	URL string
}

func (*Image) blockMarker() {}

type CodeBlock struct {
	Language string
	Code     string
}

func (*CodeBlock) blockMarker() {}

type ThematicBreak struct{}

func (*ThematicBreak) blockMarker() {}

// --- Inline types ---

type Inline interface{ inlineMarker() }

type Text struct {
	Content string
}

func (*Text) inlineMarker() {}

type LinkInline struct {
	URL   string
	Label string
}

func (*LinkInline) inlineMarker() {}

type Emphasis struct {
	Content []Inline
}

func (*Emphasis) inlineMarker() {}

type Strong struct {
	Content []Inline
}

func (*Strong) inlineMarker() {}

type Code struct {
	Content string
}

func (*Code) inlineMarker() {}

type SoftBreak struct{}

func (*SoftBreak) inlineMarker() {}

type HardBreak struct{}

func (*HardBreak) inlineMarker() {}

// --- Visitor interfaces (OCP: new block/inline types require no changes to renderer) ---

// BlockVisitor is implemented by layout engines, exporters, etc.
type BlockVisitor interface {
	VisitHeading(h *Heading)
	VisitParagraph(p *Paragraph)
	VisitList(l *List)
	VisitCodeBlock(c *CodeBlock)
	VisitThematicBreak(t *ThematicBreak)
	VisitLink(l *Link)
	VisitImage(i *Image)
}

// VisitBlocks dispatches each block to the visitor.
func VisitBlocks(blocks []Block, v BlockVisitor) {
	for _, b := range blocks {
		switch b := b.(type) {
		case *Heading:
			v.VisitHeading(b)
		case *Paragraph:
			v.VisitParagraph(b)
		case *List:
			v.VisitList(b)
		case *CodeBlock:
			v.VisitCodeBlock(b)
		case *ThematicBreak:
			v.VisitThematicBreak(b)
		case *Link:
			v.VisitLink(b)
		case *Image:
			v.VisitImage(b)
		}
	}
}

// InlineWordVisitor converts inlines into a flat word list for word-wrapping layout.
type InlineWordVisitor struct {
	Words   []Word
	LinkMap map[string]string // label → URL, populated for link tracking
	Font    Font
	SpaceW  int32
	SpaceH  int32
}

// Font abstracts font measurement needed by the inline visitor.
type Font interface {
	Measure(text string, isBold, isItalic, isCode bool) (w, h int32)
}

// Word is a unit produced from inlines for line-breaking.
type Word struct {
	Text        string
	IsSpace     bool
	PixW        int32
	PixH        int32
	IsLink      bool
	IsHardBreak bool
	IsBold      bool
	IsItalic    bool
	IsCode      bool
}

func NewInlineWordVisitor(f Font, linkMap map[string]string) *InlineWordVisitor {
	sw, sh := f.Measure(" ", false, false, false)
	return &InlineWordVisitor{
		Font:    f,
		LinkMap: linkMap,
		SpaceW:  sw,
		SpaceH:  sh,
	}
}

// VisitInlines flattens all inlines into Words.
func VisitInlines(inlines []Inline, v *InlineWordVisitor) {
	visitInlinesStyled(inlines, v, false, false, false)
}

func visitInlinesStyled(inlines []Inline, v *InlineWordVisitor, isBold, isItalic, isCode bool) {
	for _, inl := range inlines {
		switch i := inl.(type) {
		case *Text:
			parts := splitWords(i.Content)
			for n, p := range parts {
				w, h := v.Font.Measure(p, isBold, isItalic, isCode)
				v.Words = append(v.Words, Word{
					Text: p, PixW: w, PixH: h,
					IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
				})
				if n < len(parts)-1 {
					v.Words = append(v.Words, Word{
						Text: " ", IsSpace: true, PixW: v.SpaceW, PixH: v.SpaceH,
						IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
					})
				}
			}
		case *LinkInline:
			w, h := v.Font.Measure(i.Label, isBold, isItalic, isCode)
			v.Words = append(v.Words, Word{
				Text: i.Label, PixW: w, PixH: h, IsLink: true,
				IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
			})
			if v.LinkMap != nil {
				v.LinkMap[i.Label] = i.URL
			}
		case *Emphasis:
			visitInlinesStyled(i.Content, v, isBold, true, isCode)
		case *Strong:
			visitInlinesStyled(i.Content, v, true, isItalic, isCode)
		case *Code:
			w, h := v.Font.Measure(i.Content, isBold, isItalic, true)
			v.Words = append(v.Words, Word{
				Text: i.Content, PixW: w, PixH: h,
				IsBold: isBold, IsItalic: isItalic, IsCode: true,
			})
		case *SoftBreak:
			v.Words = append(v.Words, Word{
				Text: " ", IsSpace: true, PixW: v.SpaceW, PixH: v.SpaceH,
				IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
			})
		case *HardBreak:
			v.Words = append(v.Words, Word{
				IsHardBreak: true,
				IsBold:      isBold, IsItalic: isItalic, IsCode: isCode,
			})
		}
	}
}

func splitWords(text string) []string {
	var ws []string
	cur := ""
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' {
			if cur != "" {
				ws = append(ws, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		ws = append(ws, cur)
	}
	return ws
}
