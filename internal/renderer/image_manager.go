package renderer

import (
	"bytes"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"strings"
	"time"
	"unsafe"

	"github.com/kiwix-sdl/kiwix-sdl/internal/svg"
	"github.com/veandco/go-sdl2/sdl"
	_ "golang.org/x/image/webp"
)

type AnimatedTexture struct {
	Frames    []*image.RGBA
	Delays    []time.Duration
	TotalTime time.Duration
	Texture   *sdl.Texture
	LastFrame int
	Width     int32
	Height    int32
}

type ImageManager struct {
	loader     ResourceLoader
	textures   map[string]*sdl.Texture
	animated   map[string]*AnimatedTexture
	dimensions map[string]struct{ w, h int32 }
	renderer   *sdl.Renderer
	startTime  time.Time
}

func NewImageManager(renderer *sdl.Renderer) *ImageManager {
	return &ImageManager{
		textures:   make(map[string]*sdl.Texture),
		animated:   make(map[string]*AnimatedTexture),
		dimensions: make(map[string]struct{ w, h int32 }),
		renderer:   renderer,
		startTime:  time.Now(),
	}
}

func (m *ImageManager) SetLoader(loader ResourceLoader) {
	m.loader = loader
}

func isSVG(url string, data []byte) bool {
	return strings.HasSuffix(strings.ToLower(url), ".svg") || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<?xml")) || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<svg"))
}

func isGIF(url string, data []byte) bool {
	if len(data) >= 3 && string(data[:3]) == "GIF" {
		return true
	}
	return strings.HasSuffix(strings.ToLower(url), ".gif")
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
		slog.Warn("Loader failed for image", "url", url, "error", err)
		return 0, 0, false
	}

	if isSVG(url, data) {
		img := svg.Render(data)
		if img == nil {
			slog.Error("LunaSVG Decode failed", "url", url)
			return 0, 0, false
		}

		w := int32(img.Bounds().Dx())
		h := int32(img.Bounds().Dy())
		m.dimensions[url] = struct{ w, h int32 }{w, h}
		return w, h, true
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		slog.Error("DecodeConfig failed", "url", url, "format", format, "error", err, "len", len(data))
		return 0, 0, false
	}

	m.dimensions[url] = struct{ w, h int32 }{int32(config.Width), int32(config.Height)}
	return int32(config.Width), int32(config.Height), true
}

// GetTexture returns the SDL texture for an image and a boolean indicating if it's animated.
func (m *ImageManager) GetTexture(url string) (*sdl.Texture, bool) {
	if anim, ok := m.animated[url]; ok {
		m.updateAnimation(anim)
		return anim.Texture, true
	}

	if tex, ok := m.textures[url]; ok {
		return tex, false
	}

	if m.loader == nil || url == "" {
		return nil, false
	}

	data, err := m.loader(url)
	if err != nil {
		return nil, false
	}

	if isSVG(url, data) {
		return m.loadSVG(url, data), false
	}

	if isGIF(url, data) {
		return m.loadGIF(url, data)
	}

	return m.loadStaticImage(url, data), false
}

func (m *ImageManager) updateAnimation(anim *AnimatedTexture) {
	if anim.TotalTime == 0 || len(anim.Frames) == 0 {
		return
	}
	elapsed := time.Since(m.startTime) % anim.TotalTime
	var currentFrame int
	var acc time.Duration
	for i, d := range anim.Delays {
		acc += d
		if elapsed < acc {
			currentFrame = i
			break
		}
	}
	if currentFrame != anim.LastFrame {
		anim.LastFrame = currentFrame
		anim.Texture.Update(nil, unsafe.Pointer(&anim.Frames[currentFrame].Pix[0]), anim.Frames[currentFrame].Stride)
	}
}

func (m *ImageManager) loadSVG(url string, data []byte) *sdl.Texture {
	img := svg.Render(data)
	if img == nil {
		return nil
	}
	return m.createTextureFromImage(url, img)
}

func (m *ImageManager) loadStaticImage(url string, data []byte) *sdl.Texture {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil || img == nil {
		return nil
	}
	return m.createTextureFromImage(url, img)
}

func (m *ImageManager) loadGIF(url string, data []byte) (*sdl.Texture, bool) {
	parsedGIF, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil || len(parsedGIF.Image) == 0 {
		return nil, false
	}
	if len(parsedGIF.Image) == 1 {
		// Single frame GIF
		return m.createTextureFromImage(url, parsedGIF.Image[0]), false
	}

	bounds := parsedGIF.Image[0].Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	anim := &AnimatedTexture{Width: int32(w), Height: int32(h)}
	tex, err := m.renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, int32(w), int32(h))
	if err != nil {
		return nil, false
	}
	tex.SetBlendMode(sdl.BLENDMODE_BLEND)
	anim.Texture = tex

	canvas := image.NewRGBA(image.Rect(0, 0, w, h))

	for i, frame := range parsedGIF.Image {
		var prevState *image.RGBA
		var disposal byte = gif.DisposalNone
		if i < len(parsedGIF.Disposal) {
			disposal = parsedGIF.Disposal[i]
		}

		if disposal == gif.DisposalPrevious {
			prevState = image.NewRGBA(canvas.Bounds())
			copy(prevState.Pix, canvas.Pix)
		}

		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		finalFrame := image.NewRGBA(canvas.Bounds())
		copy(finalFrame.Pix, canvas.Pix)
		anim.Frames = append(anim.Frames, finalFrame)

		delayMs := 100
		if i < len(parsedGIF.Delay) && parsedGIF.Delay[i] > 0 {
			delayMs = parsedGIF.Delay[i] * 10
		}
		delayDur := time.Duration(delayMs) * time.Millisecond
		anim.Delays = append(anim.Delays, delayDur)
		anim.TotalTime += delayDur

		if disposal == gif.DisposalBackground {
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		} else if disposal == gif.DisposalPrevious && prevState != nil {
			copy(canvas.Pix, prevState.Pix)
		}
	}

	if len(anim.Frames) > 0 {
		anim.Texture.Update(nil, unsafe.Pointer(&anim.Frames[0].Pix[0]), anim.Frames[0].Stride)
	}

	m.animated[url] = anim
	m.dimensions[url] = struct{ w, h int32 }{int32(w), int32(h)}
	return anim.Texture, true
}

func (m *ImageManager) createTextureFromImage(url string, img image.Image) *sdl.Texture {
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
	for k, anim := range m.animated {
		if anim.Texture != nil {
			anim.Texture.Destroy()
		}
		delete(m.animated, k)
	}
	// We do not delete dimensions as they can be reused without re-decoding
}

func (m *ImageManager) Destroy() {
	m.ClearCache()
}
