//go:build windows

package renderer

// On Windows we deliberately do NOT spin up a raw Win32 CreateThread "big stack"
// worker to call TTF_OpenFont. Doing so runs C code that raises ordinary SEH
// exceptions (guard-page stack growth, etc.) on a thread the Go runtime does
// not manage; Go's process-wide exception handler then lets those escape and
// the OS terminates the process with the raw exception code (0x80000001
// STATUS_GUARD_PAGE / 0xC0000005 / 0xC00000FD). Instead we call TTF_OpenFont on
// the normal cgo path, which runs on a Go-managed thread. The thread's stack
// reserve is raised via `-extldflags=-Wl,--stack,0x4000000` in Dockerfile.windows.

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
