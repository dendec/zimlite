package renderer

import (
	"reflect"
	"testing"

	"github.com/dendec/zimlite/internal/document"
)

func bidiWords(texts ...string) []document.Word {
	words := make([]document.Word, len(texts))
	for i, text := range texts {
		words[i] = document.Word{Text: text, IsSpace: text == " "}
	}
	return words
}

func TestParagraphDirectionIgnoresNumbersAndPunctuation(t *testing.T) {
	for _, tt := range []struct {
		text string
		rtl  bool
	}{
		{"2026 عام جديد", true},
		{"١٢٣ عام جديد", true},
		{"(2026) שלום", true},
		{"، English", false},
		{"2026 English عام", false},
		{"123", false},
	} {
		if got := paragraphIsRTL(bidiWords(tt.text)); got != tt.rtl {
			t.Errorf("%q: RTL=%v, want %v", tt.text, got, tt.rtl)
		}
	}
	words := []document.Word{{Text: "code", IsCode: true}, {Text: "عام"}}
	if !paragraphIsRTL(words) {
		t.Fatal("inline code changed the paragraph direction")
	}
}

func TestVisualWordOrderWithNumbersAndMixedScripts(t *testing.T) {
	for _, tt := range []struct {
		name  string
		words []document.Word
		rtl   bool
		want  []int
	}{
		{"leading number", bidiWords("2026", " ", "عام", " ", "جديد"), true, []int{4, 3, 2, 1, 0}},
		{"European numbers", bidiWords("عام", " ", "10", " ", "20"), true, []int{4, 3, 2, 1, 0}},
		{"Arabic numbers", bidiWords("عام", " ", "١٠", " ", "٢٠"), true, []int{4, 3, 2, 1, 0}},
		{"LTR with Arabic and number", bidiWords("hello", " ", "عام", " ", "10"), false, []int{0, 1, 4, 3, 2}},
		{"Hebrew", bidiWords("שלום", " ", "10", " ", "20"), true, []int{4, 3, 2, 1, 0}},
		{"Latin", bidiWords("hello", " ", "20", " ", "world"), false, []int{0, 1, 2, 3, 4}},
		{"code isolate", []document.Word{{Text: "عام"}, {Text: " "}, {Text: "x = 10", IsCode: true}}, true, []int{2, 1, 0}},
		{"object", []document.Word{{Text: "عام"}, {IsImage: true}, {Text: "جديد"}}, true, []int{2, 1, 0}},
		{"empty", nil, false, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := visualWordOrder(tt.words, tt.rtl); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("order=%v, want %v", got, tt.want)
			}
		})
	}
}
