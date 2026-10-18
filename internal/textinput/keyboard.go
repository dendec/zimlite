package textinput

import (
	"github.com/clipperhouse/uax29/v2/graphemes"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxTextBytes = 2048
const Rows = 4

type KeyAction uint8

const (
	Character KeyAction = iota
	Literal
	Shift
	Language
	Symbols
	Delete
	Space
	Submit
	Pick
	PreviousPage
	NextPage
	ClosePicker
)

type Key struct {
	Label, Value     string
	Action           KeyAction
	Row, Start, Span int
}
type Layout struct {
	Keys  []Key
	Units int
}
type View struct {
	Text, Preedit, Language, Method, Status string
	Layout                                  Layout
	Layer                                   int
	Candidates                              []string
	Candidate, Focus                        int
}

// At 640x480 the default 16px font gives a 144px panel, including query and candidates.
type Geometry struct {
	X, Y, W, H, QueryH, CandidatesH int32
	Layout                          Layout
}

func Panel(width, height int32, fontSize int) Geometry {
	width = max(int32(0), width)
	height = max(int32(0), height)
	if fontSize <= 0 {
		fontSize = 16
	}
	baseW := min(int32(640), width)
	baseH := min(int32(144), height*3/10)
	w := min(width, int32(640), baseW*int32(fontSize)/16)
	h := min(height, baseH*int32(fontSize)/16)
	return Geometry{X: (width - w) / 2, Y: height - h, W: w, H: h, QueryH: h / 6, CandidatesH: h * 5 / 36}
}
func (g Geometry) KeyRect(index int) (x, y, w, h int32) {
	if index < 0 || index >= len(g.Layout.Keys) || g.Layout.Units <= 0 {
		return
	}
	k := g.Layout.Keys[index]
	units := int32(g.Layout.Units)
	top := g.Y + g.QueryH + g.CandidatesH
	height := g.H - g.QueryH - g.CandidatesH
	x = g.X + int32(k.Start)*g.W/units
	end := g.X + int32(k.Start+k.Span)*g.W/units
	y = top + int32(k.Row)*height/Rows
	return x, y, end - x, top + int32(k.Row+1)*height/Rows - y
}
func (g Geometry) Hit(x, y int32) int {
	for i := range g.Layout.Keys {
		left, top, w, h := g.KeyRect(i)
		if x >= left && x < left+w && y >= top && y < top+h {
			return i
		}
	}
	return -1
}

var letters = []string{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "[", "]", "a", "s", "d", "f", "g", "h", "j", "k", "l", ";", "'", "\\", "z", "x", "c", "v", "b", "n", "m", ",", ".", "/", "-", "="}

func (l *Layout) add(row, start, span int, label, value string, action KeyAction) {
	l.Keys = append(l.Keys, Key{Label: label, Value: value, Action: action, Row: row, Start: start, Span: span})
}

// Keyboard keeps IME physical key identities while arranging their glyphs in
// mobile-style rows. Extra punctuation keys that produce script letters remain
// available (e.g. Russian х/ъ/ж/э/б/ю and Indic combining marks).
func Keyboard(layer int, labels []string) Layout {
	var rows [3][]Key
	if layer >= 2 {
		values := [][]string{
			{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
			{"@", "#", "$", "%", "&", "-", "+", "(", ")", "/"},
			{"*", "\"", "'", ":", ";", "!", "?", "="},
		}
		if layer == 3 {
			values = [][]string{
				{"[", "]", "{", "}", "<", ">", "^", "_", "|", "~"},
				{"\\", "`", "€", "£", "¥", "¢", "°", "•", "×", "÷"},
				{"é", "è", "ê", "ä", "ö", "ü", "ñ", "ç"},
			}
		}
		for row := range rows {
			for _, v := range values[row] {
				rows[row] = append(rows[row], Key{Label: v, Value: v, Action: Character})
			}
		}
	} else {
		for row := range rows {
			for col := 0; col < 12; col++ {
				index := row*12 + col
				label, value := letters[index], letters[index]
				if layer == 1 {
					label = shifted(label)
					value = shifted(value)
				}
				if len(labels) >= 72 {
					label = labels[(layer%2)*36+index]
				}
				// Base QWERTY letters plus extra keys mapped to native script glyphs.
				if col >= []int{10, 9, 7}[row] && !strings.ContainsFunc(label, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsMark(r) }) {
					continue
				}
				rows[row] = append(rows[row], Key{Label: label, Value: value, Action: Character})
			}
		}
	}
	columns := max(len(rows[0]), len(rows[1]), len(rows[2])+2)
	l := Layout{Units: columns * 2}
	for row, keys := range rows {
		start := (l.Units - len(keys)*2) / 2
		if row == 2 {
			side := columns - len(keys)
			label := "⇧"
			switch layer {
			case 2:
				label = "#+="
			case 3:
				label = "123"
			}
			l.add(2, 0, side, label, "", Shift)
			start = side
		}
		for _, k := range keys {
			l.add(row, start, 2, k.Label, k.Value, k.Action)
			start += 2
		}
		if row == 2 {
			l.add(2, start, columns-len(keys), "⌫", "", Delete)
		}
	}
	label := "?123"
	if layer >= 2 {
		label = "ABC"
	}
	l.add(3, 0, 2, label, "", Symbols)
	l.add(3, 2, 2, "🌐", "", Language)
	l.add(3, 4, 1, ",", ",", Literal)
	l.add(3, 5, l.Units-10, "", " ", Space)
	l.add(3, l.Units-5, 1, ".", ".", Literal)
	l.add(3, l.Units-4, 4, "↵", "", Submit)
	return l
}
func Picker(labels, values []string) Layout {
	l := Layout{Units: 12}
	for i, label := range labels {
		if i >= 9 {
			break
		}
		l.add(i/3, (i%3)*4, 4, label, values[i], Pick)
	}
	l.add(3, 0, 4, "‹", "", PreviousPage)
	l.add(3, 4, 4, "↩", "", ClosePicker)
	l.add(3, 8, 4, "›", "", NextPage)
	return l
}

// Move preserves the key's horizontal position when moving between unequal rows.
func (l Layout) Move(focus, direction int) int {
	if focus < 0 || focus >= len(l.Keys) {
		return 0
	}
	current := l.Keys[focus]
	if direction == 1 || direction == 2 {
		indices := []int{}
		position := 0
		for i, k := range l.Keys {
			if k.Row == current.Row {
				if i == focus {
					position = len(indices)
				}
				indices = append(indices, i)
			}
		}
		delta := 1
		if direction == 1 {
			delta = -1
		}
		return indices[(position+delta+len(indices))%len(indices)]
	}
	center := 2*current.Start + current.Span
	delta := 1
	if direction == 3 {
		delta = -1
	}
	for step := 1; step < Rows; step++ {
		row := (current.Row + delta*step + Rows) % Rows
		best, distance := -1, 1<<30
		for i, k := range l.Keys {
			if k.Row == row {
				d := 2*k.Start + k.Span - center
				if d < 0 {
					d = -d
				}
				if d < distance {
					best, distance = i, d
				}
			}
		}
		if best >= 0 {
			return best
		}
	}
	return focus
}

// Shift uses US physical key symbols before the selected IME maps the key.
func shifted(key string) string {
	const plain = "[];'\\,./-="
	const upper = "{}:\"|<>?_+"
	if i := strings.Index(plain, key); i >= 0 && len(key) == 1 {
		return string(upper[i])
	}
	return strings.ToUpper(key)
}
func Append(text, value string) string {
	if !utf8.ValidString(value) || len(text)+len(value) > MaxTextBytes {
		return text
	}
	value = strings.Map(func(r rune) rune {
		if r < ' ' || r == 127 {
			return -1
		}
		return r
	}, value)
	return text + value
}
func Backspace(text string) string {
	g := graphemes.FromString(text)
	last := 0
	for g.Next() {
		last = g.Start()
	}
	return text[:last]
}
