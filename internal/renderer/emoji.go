package renderer

import (
	"embed"
	"image"
	"log/slog"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

//go:embed assets/emoji/*.svg
var emojiFS embed.FS

func (r *Renderer) createEmojiTexture(img *image.RGBA, hex string, size int32) *sdl.Texture {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	tex, err := r.sdlRenderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, int32(w), int32(h))
	if err != nil {
		slog.Error("create emoji texture", "hex", hex, "error", err)
		return nil
	}
	tex.SetBlendMode(sdl.BLENDMODE_BLEND)
	tex.Update(nil, unsafe.Pointer(&img.Pix[0]), img.Stride)
	r.emojiCache[emojiCacheKey{hex: hex, size: size}] = tex
	return tex
}
