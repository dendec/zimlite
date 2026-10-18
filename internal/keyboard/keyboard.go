// Package keyboard detects whether automatic on-screen input is needed.
package keyboard

import "os"

// Present is evaluated when a text field receives focus, allowing hotplugging.
// KIWIX_PHYSICAL_KEYBOARD overrides detection for remote and unusual setups.
func Present() bool {
	switch os.Getenv("KIWIX_PHYSICAL_KEYBOARD") {
	case "0":
		return false
	case "1":
		return true
	}
	return present()
}
