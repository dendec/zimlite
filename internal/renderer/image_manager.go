package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"unsafe"

	"github.com/kiwix-sdl/kiwix-sdl/internal/svg"
	"github.com/veandco/go-sdl2/sdl"
	_ "golang.org/x/image/webp"
)

type ImageManager struct {
	loader     ResourceLoader
	textures   map[string]*sdl.Texture
	dimensions map[string]struct{ w, h int32 }
	renderer   *sdl.Renderer
}

func NewImageManager(renderer *sdl.Renderer) *ImageManager {
	return &ImageManager{
		textures:   make(map[string]*sdl.Texture),
		dimensions: make(map[string]struct{ w, h int32 }),
		renderer:   renderer,
	}
}

func (m *ImageManager) SetLoader(loader ResourceLoader) {
	m.loader = loader
}

// GetDimensions returns original image dimensions. If not in cache, loads it via DecodeConfig.
func (m *ImageManager) GetDimensions(url string) (int32, int32, bool) {
	if dim, ok := m.dimensions[url]; ok {
		return dim.w, dim.h, true
	}

	if m.loader == nil || url == "" {
		return 0, 0, false
	}

	data, err := m.loader(url)
	if err != nil {
		fmt.Printf("[DEBUG] Loader failed for %s: %v\n", url, err)
		return 0, 0, false
	}

	isSVG := strings.HasSuffix(strings.ToLower(url), ".svg") || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<?xml")) || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<svg"))

	if isSVG {
		img := svg.Render(data)
		if img == nil {
			fmt.Printf("[DEBUG] LunaSVG Decode failed for %s\n", url)
			return 0, 0, false
		}

		w := int32(img.Bounds().Dx())
		h := int32(img.Bounds().Dy())
		m.dimensions[url] = struct{ w, h int32 }{w, h}
		return w, h, true
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		fmt.Printf("[DEBUG] DecodeConfig failed for %s: format=%s err=%v len=%d\n", url, format, err, len(data))
		return 0, 0, false
	}

	m.dimensions[url] = struct{ w, h int32 }{int32(config.Width), int32(config.Height)}
	return int32(config.Width), int32(config.Height), true
}

// GetTexture returns the SDL texture for an image.
func (m *ImageManager) GetTexture(url string) *sdl.Texture {
	if tex, ok := m.textures[url]; ok {
		return tex
	}

	if m.loader == nil || url == "" {
		return nil
	}

	data, err := m.loader(url)
	if err != nil {
		return nil
	}

	var img image.Image
	isSVG := strings.HasSuffix(strings.ToLower(url), ".svg") || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<?xml")) || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<svg"))

	if isSVG {
		img = svg.Render(data)
	} else {
		var err error
		img, _, err = image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil
		}
	}

	if img == nil {
		return nil
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)

	tex, err := m.renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, int32(w), int32(h))
	if err == nil && len(rgba.Pix) > 0 {
		tex.SetBlendMode(sdl.BLENDMODE_BLEND)
		tex.Update(nil, unsafe.Pointer(&rgba.Pix[0]), rgba.Stride)
		m.textures[url] = tex
		m.dimensions[url] = struct{ w, h int32 }{int32(w), int32(h)}
		return tex
	}

	return nil
}

func (m *ImageManager) ClearCache() {
	for k, tex := range m.textures {
		if tex != nil {
			tex.Destroy()
		}
		delete(m.textures, k)
	}
	// We do not delete dimensions as they can be reused without re-decoding
}

func (m *ImageManager) Destroy() {
	m.ClearCache()
}
