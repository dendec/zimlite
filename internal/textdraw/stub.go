//go:build !cgo || windows

package textdraw

func Render(root, text string, size, maxWidth int) ([]byte, int, int) { return nil, 0, 0 }
func Clear()                                                          {}
