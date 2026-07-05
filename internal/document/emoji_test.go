package document

import (
	"testing"
)

func TestEmojiSequence(t *testing.T) {
	cases := []struct {
		input        string
		wantHex      string
		wantConsumed int
		wantOK       bool
	}{
		{"😀", "1f600", 1, true},
		{"🚀", "1f680", 1, true},
		{"❤️", "2764", 2, true}, // heart + VS16, VS16 omitted in hex
		{"\U0001f468\u200d\U0001f469\u200d\U0001f467\u200d\U0001f466",
			"1f468-200d-1f469-200d-1f467-200d-1f466", 7, true}, // ZWJ family
		{"#️⃣", "23-20e3", 3, true},    // keycap
		{"0️⃣", "30-20e3", 3, true},    // keycap
		{"*️⃣", "2a-20e3", 3, true},    // keycap
		{"👍🏿", "1f44d-1f3ff", 2, true}, // thumbs up + skin tone
		{"🇺🇸", "1f1fa-1f1f8", 2, true}, // US flag
		{"a", "", 0, false},
		{" ", "", 0, false},
	}
	for _, c := range cases {
		runes := []rune(c.input)
		hex, consumed, ok := EmojiSequence(runes, 0)
		if ok != c.wantOK || consumed != c.wantConsumed || hex != c.wantHex {
			t.Errorf("EmojiSequence(%q) = (%q, %d, %v), want (%q, %d, %v)",
				c.input, hex, consumed, ok, c.wantHex, c.wantConsumed, c.wantOK)
		}
	}
}

func TestTokenizerNoHang(t *testing.T) {
	hazardInputs := []string{
		"#️⃣ test",
		"lone FE0F: \ufe0f",
		"lone ZWJ: \u200d",
		"lone skin: \U0001f3fb",
		"mixed: 😀#️⃣👍 hello *️⃣",
		"keycaps: 0️⃣1️⃣2️⃣3️⃣4️⃣5️⃣6️⃣7️⃣8️⃣9️⃣🔟",
	}
	for _, s := range hazardInputs {
		tokens := tokenizeText(s)
		if len(tokens) == 0 {
			t.Errorf("tokenizeText(%q) returned empty tokens", s)
		}
	}
}

func TestTokenizeText(t *testing.T) {
	input := "hello 😀 world"
	tokens := tokenizeText(input)
	var got []string
	for _, tok := range tokens {
		kind := "txt"
		if tok.IsSpace {
			kind = "spc"
		} else if tok.IsEmoji {
			kind = "emo"
		}
		got = append(got, kind+":"+tok.Text)
	}
	t.Logf("tokens: %v", got)
	if len(tokens) != 5 {
		t.Fatalf("expected 5 tokens, got %d: %v", len(tokens), got)
	}
}
