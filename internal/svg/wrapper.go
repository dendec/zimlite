package svg

/*
#cgo CXXFLAGS: -std=c++17
#include "lunasvg_c.h"
*/
import "C"
import (
	"image"
	"unsafe"
)

// Render decodes SVG data and rasterizes it into an RGBA image.
// It returns nil if the SVG cannot be decoded or rendered.
func Render(data []byte) *image.RGBA {
	if len(data) == 0 {
		return nil
	}

	var w, h C.int
	cData := (*C.char)(unsafe.Pointer(&data[0]))
	cLen := C.int(len(data))

	pixels := C.lunasvg_render(cData, cLen, &w, &h)
	if pixels == nil {
		return nil
	}
	defer C.lunasvg_free(pixels)

	gw := int(w)
	gh := int(h)
	if gw == 0 || gh == 0 {
		return nil
	}

	stride := gw * 4
	img := image.NewRGBA(image.Rect(0, 0, gw, gh))
	// Copy pixel data from C memory to Go memory
	src := unsafe.Slice((*byte)(pixels), gh*stride)
	copy(img.Pix, src)

	return img
}
