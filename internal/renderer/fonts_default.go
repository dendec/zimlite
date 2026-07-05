//go:build !windows

package renderer

import (
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/ttf"
)

func openFontSafe(path string, size int) (*ttf.Font, error) {
	return ttf.OpenFont(path, size)
}

func openFontFromMem(data []byte, size int) (*ttf.Font, error) {
	rw, err := sdl.RWFromMem(data)
	if err != nil {
		return nil, err
	}
	return ttf.OpenFontRW(rw, 1, size)
}
