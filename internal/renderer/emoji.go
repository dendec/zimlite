package renderer

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"image"
	"io"
	"log/slog"
	"sync"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

//go:embed assets/emoji.zip
var emojiZip []byte

var (
	emojiIdxOnce sync.Once
	emojiIdx     map[string]*zip.File
)

func buildEmojiIndex() {
	r, err := zip.NewReader(bytes.NewReader(emojiZip), int64(len(emojiZip)))
	if err != nil {
		slog.Error("open emoji zip", "error", err)
		return
	}
	emojiIdx = make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		name := f.Name
		// Strip "emoji/" prefix and ".svg" suffix
		if len(name) > 10 && name[:6] == "emoji/" {
			name = name[6 : len(name)-4]
			emojiIdx[name] = f
		}
	}
}

func getEmojiSVG(hex string) ([]byte, error) {
	emojiIdxOnce.Do(buildEmojiIndex)
	f, ok := emojiIdx[hex]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	_ = rc.Close()
	return io.ReadAll(rc)
}

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
