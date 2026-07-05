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

// Render decodes SVG data and rasterizes it into an RGBA image at the SVG's
// intrinsic size. It returns nil if the SVG cannot be decoded or rendered.
func Render(data []byte) *image.RGBA {
	pixels, w, h := renderRaw(data, 0, 0)
	if pixels == nil {
		return nil
	}
	defer C.lunasvg_free(pixels)
	return pixelsToImage(pixels, w, h)
}

// RenderToSize decodes SVG data and rasterizes it to exactly targetW×targetH pixels.
// It returns nil if the SVG cannot be decoded or rendered.
func RenderToSize(data []byte, targetW, targetH int) *image.RGBA {
	if targetW <= 0 || targetH <= 0 {
		return nil
	}
	pixels, w, h := renderRaw(data, C.int(targetW), C.int(targetH))
	if pixels == nil {
		return nil
	}
	defer C.lunasvg_free(pixels)
	return pixelsToImage(pixels, w, h)
}

func renderRaw(data []byte, targetW, targetH C.int) (pixels unsafe.Pointer, w, h int) {
	if len(data) == 0 {
		return nil, 0, 0
	}
	cData := (*C.char)(unsafe.Pointer(&data[0]))
	cLen := C.int(len(data))

	var outW, outH C.int
	if targetW > 0 && targetH > 0 {
		pixels = C.lunasvg_render_to_size(cData, cLen, targetW, targetH, &outW, &outH)
	} else {
		pixels = C.lunasvg_render(cData, cLen, &outW, &outH)
	}
	if pixels == nil {
		return nil, 0, 0
	}
	gw := int(outW)
	gh := int(outH)
	if gw == 0 || gh == 0 {
		C.lunasvg_free(pixels)
		return nil, 0, 0
	}
	return pixels, gw, gh
}

func pixelsToImage(pixels unsafe.Pointer, w, h int) *image.RGBA {
	stride := w * 4
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	src := unsafe.Slice((*byte)(pixels), h*stride)
	copy(img.Pix, src)
	return img
}
