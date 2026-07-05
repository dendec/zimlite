// Package document defines the internal document model.
// Renderer works only with these types — never with markdown or HTML directly.
package document

// Document represents a fully parsed markdown/html document.
type Document struct {
	Blocks []Block
}

// ArticleEntry holds the title and internal path of a library article.
type ArticleEntry struct {
	Title string
	Path  string
}

// ViewState captures the user's reading position and interactions within a document.
type ViewState struct {
	ScrollY      int32
	SelectedLink int
}

// --- Block types ---

type Block interface{ blockMarker() }

type Heading struct {
	Level   int // 1–6
	Content string
	ID      string // original HTML id attribute, empty if not available
}

func (*Heading) blockMarker() {}

type Paragraph struct {
	Inlines []Inline
}

func (*Paragraph) blockMarker() {}

type ListEntry struct {
	Item    []Inline
	Ordered bool
	Start   int
	Indent  int
}

type List struct {
	Entries []ListEntry
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

func (c *CodeBlock) blockMarker() {}

type Table struct {
	Rows []TableRow
}

func (t *Table) blockMarker() {}

type TableRow struct {
	IsHeader bool
	Cells    []TableCell
}

type TableCell struct {
	Inlines []Inline
}

type ThematicBreak struct{}

func (*ThematicBreak) blockMarker() {}

type Blockquote struct {
	Blocks []Block
}

func (*Blockquote) blockMarker() {}

type Anchor struct {
	ID string
}

func (*Anchor) blockMarker() {}

// --- Inline types ---

type Inline interface{ inlineMarker() }

type Text struct {
	Content string
}

func (*Text) inlineMarker() {}

type LinkInline struct {
	URL     string
	Content []Inline
}

func (*LinkInline) inlineMarker() {}

type ImageInline struct {
	Alt string
	URL string
}

func (*ImageInline) inlineMarker() {}

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
	VisitBlockquote(b *Blockquote)
	VisitLink(l *Link)
	VisitImage(i *Image)
	VisitTable(t *Table)
	VisitAnchor(a *Anchor)
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
		case *Blockquote:
			v.VisitBlockquote(b)
		case *Link:
			v.VisitLink(b)
		case *Image:
			v.VisitImage(b)
		case *Table:
			v.VisitTable(b)
		case *Anchor:
			v.VisitAnchor(b)
		}
	}
}

// InlineWordVisitor converts inlines into a flat word list for word-wrapping layout.
type InlineWordVisitor struct {
	Words        []Word
	LinkURLs     map[int]string // LinkID → URL, populated for link tracking
	Font         Font
	MeasureImage func(url string) (int32, int32)
	SpaceW       int32
	SpaceH       int32
	NextLinkID   int
}

// Font abstracts font measurement needed by the inline visitor.
type Font interface {
	Measure(text string, isBold, isItalic, isCode bool) (w, h int32)
}

// Word is a unit produced from inlines for line-breaking.
type Word struct {
	Text        string
	IsSpace     bool
	IsImage     bool
	IsEmoji     bool
	EmojiHex    string
	ImageURL    string
	PixW        int32
	PixH        int32
	LinkID      int
	IsHardBreak bool
	IsBold      bool
	IsItalic    bool
	IsCode      bool
}

func NewInlineWordVisitor(f Font, measureImage func(string) (int32, int32)) *InlineWordVisitor {
	sw, sh := f.Measure(" ", false, false, false)
	return &InlineWordVisitor{
		Font:         f,
		MeasureImage: measureImage,
		LinkURLs:     make(map[int]string),
		SpaceW:       sw,
		SpaceH:       sh,
		NextLinkID:   1,
	}
}

// VisitInlines flattens all inlines into Words.
func VisitInlines(inlines []Inline, v *InlineWordVisitor) {
	visitInlinesStyled(inlines, v, false, false, false, 0)
}

func visitInlinesStyled(inlines []Inline, v *InlineWordVisitor, isBold, isItalic, isCode bool, linkID int) {
	for _, inl := range inlines {
		switch i := inl.(type) {
		case *Text:
			tokens := tokenizeText(i.Content)
			for _, t := range tokens {
				if t.IsSpace {
					v.Words = append(v.Words, Word{
						Text: " ", IsSpace: true, PixW: v.SpaceW, PixH: v.SpaceH,
						IsBold: isBold, IsItalic: isItalic, IsCode: isCode, LinkID: linkID,
					})
				} else if t.IsEmoji {
					w, h := v.Font.Measure(t.Text, isBold, isItalic, isCode)
					v.Words = append(v.Words, Word{
						Text: t.Text, IsEmoji: true, EmojiHex: t.EmojiHex,
						PixW: w, PixH: h, LinkID: linkID,
						IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
					})
				} else {
					w, h := v.Font.Measure(t.Text, isBold, isItalic, isCode)
					v.Words = append(v.Words, Word{
						Text: t.Text, PixW: w, PixH: h, LinkID: linkID,
						IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
					})
				}
			}
		case *LinkInline:
			id := v.NextLinkID
			v.NextLinkID++
			if v.LinkURLs != nil {
				v.LinkURLs[id] = i.URL
			}
			visitInlinesStyled(i.Content, v, isBold, isItalic, isCode, id)
		case *ImageInline:
			var w, h int32
			if v.MeasureImage != nil {
				w, h = v.MeasureImage(i.URL)
			}
			if w == 0 || h == 0 {
				altText := "[" + i.Alt + "]"
				w, h = v.Font.Measure(altText, isBold, isItalic, isCode)
				v.Words = append(v.Words, Word{
					Text: altText, PixW: w, PixH: h, LinkID: linkID,
					IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
				})
			} else {
				v.Words = append(v.Words, Word{
					IsImage: true, ImageURL: i.URL, PixW: w, PixH: h, LinkID: linkID,
				})
			}
		case *Emphasis:
			visitInlinesStyled(i.Content, v, isBold, true, isCode, linkID)
		case *Strong:
			visitInlinesStyled(i.Content, v, true, isItalic, isCode, linkID)
		case *Code:
			tokens := tokenizeText(i.Content)
			for _, t := range tokens {
				if t.IsSpace {
					v.Words = append(v.Words, Word{
						Text: " ", IsSpace: true, PixW: v.SpaceW, PixH: v.SpaceH,
						IsBold: isBold, IsItalic: isItalic, IsCode: true, LinkID: linkID,
					})
				} else {
					w, h := v.Font.Measure(t.Text, isBold, isItalic, true)
					v.Words = append(v.Words, Word{
						Text: t.Text, PixW: w, PixH: h, LinkID: linkID,
						IsBold: isBold, IsItalic: isItalic, IsCode: true,
					})
				}
			}
		case *SoftBreak:
			v.Words = append(v.Words, Word{
				Text: " ", IsSpace: true, PixW: v.SpaceW, PixH: v.SpaceH,
				IsBold: isBold, IsItalic: isItalic, IsCode: isCode, LinkID: linkID,
			})
		case *HardBreak:
			v.Words = append(v.Words, Word{
				IsHardBreak: true, LinkID: linkID,
				IsBold: isBold, IsItalic: isItalic, IsCode: isCode,
			})
		}
	}
}

type textToken struct {
	Text     string
	IsSpace  bool
	IsEmoji  bool
	EmojiHex string
}

func tokenizeText(text string) []textToken {
	var tokens []textToken
	runes := []rune(text)
	n := len(runes)
	i := 0
	for i < n {
		r := runes[i]
		if r == ' ' || r == '\t' || r == '\n' {
			tokens = append(tokens, textToken{Text: " ", IsSpace: true})
			for i < n && (runes[i] == ' ' || runes[i] == '\t' || runes[i] == '\n') {
				i++
			}
			continue
		}
		if hex, consumed, ok := EmojiSequence(runes, i); ok {
			tokens = append(tokens, textToken{
				Text:     string(runes[i : i+consumed]),
				IsEmoji:  true,
				EmojiHex: hex,
			})
			i += consumed
			continue
		}
		start := i
		for i < n && runes[i] != ' ' && runes[i] != '\t' && runes[i] != '\n' && !canStartEmoji(runes[i]) {
			i++
		}
		if i > start {
			tokens = append(tokens, textToken{Text: string(runes[start:i])})
		} else {
			// Safety: advance by 1 if nothing matched (should not happen).
			tokens = append(tokens, textToken{Text: string(runes[i])})
			i++
		}
	}
	return tokens
}
