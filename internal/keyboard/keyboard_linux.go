package keyboard

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func present() bool {
	paths, _ := filepath.Glob("/sys/class/input/event*/device/capabilities/key")
	for _, path := range paths {
		// PortMaster's gptokeyb creates a uinput keyboard for controller mapping.
		// It cannot provide physical text entry and must not suppress the panel.
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil && strings.Contains(resolved, "/devices/virtual/input/") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if typingKeys(strings.Fields(string(data))) {
			return true
		}
	}
	return false
}

// Linux prints native unsigned long words, most significant word first.
// Requiring letter keys excludes handheld buttons exposed as a keyboard.
func typingKeys(words []string) bool {
	for _, code := range []int{16, 28, 30, 44, 57} { // Q, Enter, A, Z, Space
		index := len(words) - 1 - code/strconv.IntSize
		if index < 0 {
			return false
		}
		word, err := strconv.ParseUint(words[index], 16, strconv.IntSize)
		if err != nil || word&(uint64(1)<<uint(code%strconv.IntSize)) == 0 {
			return false
		}
	}
	return true
}
