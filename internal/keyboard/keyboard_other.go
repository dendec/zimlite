//go:build !linux

package keyboard

// Desktop platforms use physical input by default; F3 still opens the panel.
func present() bool { return true }
