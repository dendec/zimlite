package document

import (
	"fmt"
	"strings"
)

var emojiRanges = []struct {
	lo, hi rune
}{
	{0x00A9, 0x00A9},
	{0x00AE, 0x00AE},
	{0x203C, 0x203C},
	{0x2049, 0x2049},
	{0x2122, 0x2122},
	{0x2139, 0x2139},
	{0x2194, 0x2199},
	{0x21A9, 0x21AA},
	{0x231A, 0x231B},
	{0x2328, 0x2328},
	{0x23CF, 0x23CF},
	{0x23E9, 0x23F3},
	{0x23F8, 0x23FA},
	{0x24C2, 0x24C2},
	{0x25AA, 0x25AB},
	{0x25B6, 0x25B6},
	{0x25C0, 0x25C0},
	{0x25FB, 0x25FE},
	{0x2600, 0x27BF},
	{0x2934, 0x2935},
	{0x2B05, 0x2B07},
	{0x2B1B, 0x2B1C},
	{0x2B50, 0x2B50},
	{0x2B55, 0x2B55},
	{0x3030, 0x3030},
	{0x303D, 0x303D},
	{0x3297, 0x3297},
	{0x3299, 0x3299},
	{0x1F300, 0x1F5FF}, // Misc Symbols + Pictographs
	{0x1F600, 0x1F64F}, // Emoticons
	{0x1F680, 0x1F6FF}, // Transport + Map
	{0x1F900, 0x1F9FF}, // Supplemental Symbols
	{0x1FA00, 0x1FA6F}, // Chess Symbols
	{0x1FA70, 0x1FAFF}, // Symbols Extended-A
}

func IsEmojiRune(r rune) bool {
	if r == 0x20E3 || r == 0xFE0F || r == 0x200D {
		return true
	}
	if r >= 0x1F1E6 && r <= 0x1F1FF {
		return true
	}
	if r >= 0x1F3FB && r <= 0x1F3FF {
		return true
	}
	for _, rng := range emojiRanges {
		if r >= rng.lo && r <= rng.hi {
			return true
		}
	}
	return false
}

func canStartEmoji(r rune) bool {
	if !IsEmojiRune(r) {
		return false
	}
	if r == 0xFE0F || r == 0x200D || r == 0x20E3 {
		return false
	}
	if r >= 0x1F3FB && r <= 0x1F3FF {
		return false
	}
	return true
}

func isKeycapBase(r rune) bool {
	return r == 0x0023 || r == 0x002A || (r >= 0x0030 && r <= 0x0039)
}

func emojiHex(runes []rune, start, end int) string {
	var parts []string
	for i := start; i < end; i++ {
		r := runes[i]
		if r == 0xFE0F {
			continue // Twemoji omits VS16 from filenames
		}
		parts = append(parts, fmt.Sprintf("%x", r))
	}
	return strings.Join(parts, "-")
}

func consumePostEmoji(runes []rune, pos int) int {
	n := len(runes)
	if pos < n && runes[pos] == 0xFE0F {
		pos++
	}
	if pos < n && runes[pos] >= 0x1F3FB && runes[pos] <= 0x1F3FF {
		pos++
	}
	return pos
}

func EmojiSequence(runes []rune, pos int) (hexName string, consumed int, ok bool) {
	n := len(runes)
	if pos >= n {
		return "", 0, false
	}

	// Keycap: base (#, *, 0-9) + FE0F + 20E3
	if isKeycapBase(runes[pos]) {
		if pos+2 < n && runes[pos+1] == 0xFE0F && runes[pos+2] == 0x20E3 {
			hexName = emojiHex(runes, pos, pos+3)
			return hexName, 3, true
		}
		return "", 0, false
	}

	if !canStartEmoji(runes[pos]) {
		return "", 0, false
	}

	start := pos
	pos++

	pos = consumePostEmoji(runes, pos)

	for pos < n && runes[pos] == 0x200D {
		pos++
		if pos >= n || !canStartEmoji(runes[pos]) {
			break
		}
		pos++
		pos = consumePostEmoji(runes, pos)
	}

	// Consume trailing VS16, keycap.
	if pos < n && runes[pos] == 0xFE0F {
		pos++
	}
	if pos < n && runes[pos] == 0x20E3 {
		pos++
	}

	end := pos

	// Flag pairs: two regional indicators (not ZWJ-joined).
	if end-start == 1 && start+1 < n &&
		runes[start] >= 0x1F1E6 && runes[start] <= 0x1F1FF &&
		runes[start+1] >= 0x1F1E6 && runes[start+1] <= 0x1F1FF {
		end = start + 2
	}

	consumed = end - start
	hexName = emojiHex(runes, start, end)
	return hexName, consumed, true
}
