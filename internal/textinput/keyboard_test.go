package textinput

import (
	"reflect"
	"testing"
)

func TestArchiveLanguages(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want []string
	}{
		{"rus,zho", []string{"rus", "zho", "eng"}},
		{" RU ; eng,rus\nuk ", []string{"rus", "eng", "ukr"}},
		{"", []string{"eng"}},
		{"mul,und,zxx", []string{"eng"}},
		{"zzz", []string{"zzz", "eng"}},
	} {
		if got := Languages(tt.raw); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: %v != %v", tt.raw, got, tt.want)
		}
	}
}
func TestPanelBudgetAndHitTesting(t *testing.T) {
	for _, size := range [][2]int32{{640, 480}, {641, 481}, {1280, 720}, {320, 240}, {800, 600}} {
		g := Panel(size[0], size[1], 16)
		g.Layout = Keyboard(0, nil)
		if g.H*10 > size[1]*3 {
			t.Fatalf("panel exceeds 30%%: %+v", g)
		}
		if g.Y+g.H != size[1] {
			t.Fatal("panel is not bottom aligned")
		}
		if size == [2]int32{640, 480} && g.H != 144 {
			t.Fatal("640x480 requires a 144px panel")
		}
		for i := 0; i < len(g.Layout.Keys); i++ {
			x, y, w, h := g.KeyRect(i)
			if y < g.Y || y+h > size[1] || x+w > size[0] {
				t.Fatalf("key %d outside panel", i)
			}
			for _, point := range [][2]int32{{x, y}, {x + w - 1, y + h - 1}, {x + w/2, y + h/2}} {
				if got := g.Hit(point[0], point[1]); got != i {
					t.Fatalf("%v key %d point %v maps to %d", size, i, point, got)
				}
			}
		}
	}
}
func TestUnicodeBackspace(t *testing.T) {
	for _, suffix := range []string{"é", "e\u0301", "👨‍👩‍👧‍👦", "🇯🇵", "क्ष", "한"} {
		if got := Backspace("hello" + suffix); got != "hello" {
			t.Errorf("delete %q left %q", suffix, got)
		}
	}
	if Backspace("") != "" {
		t.Fatal("empty deletion")
	}
}
func TestBoundedInputAndProtocol(t *testing.T) {
	if got := Append("hello", "\n\tworld"); got != "helloworld" {
		t.Fatal(got)
	}
	for _, invalid := range []string{"", "S\t00", "S\t\t\t0\t999999999", "S\tzz\t\t0\t0"} {
		if _, err := parseState(invalid); err == nil {
			t.Errorf("accepted invalid response %q", invalid)
		}
	}
	st, err := parseState("S\te4b8ad\t\t0\t0")
	if err != nil || st.Text != "中" {
		t.Fatalf("%+v %v", st, err)
	}
}
