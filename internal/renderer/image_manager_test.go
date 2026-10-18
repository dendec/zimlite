package renderer

import (
	"bytes"
	"image"
	"image/png"
	"sync"
	"testing"
)

func TestClearImageCacheRefreshesDimensions(t *testing.T) {
	manager := NewImageManager(nil)
	width := 2
	manager.SetLoader(func(string) ([]byte, error) {
		var buf bytes.Buffer
		err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, 3)))
		return buf.Bytes(), err
	})
	if w, _, ok := manager.GetDimensions("shared.png"); !ok || w != 2 {
		t.Fatalf("initial width=%d, ok=%v", w, ok)
	}
	width = 5 // Same relative URL in the next document.
	manager.ClearCache()
	if w, _, ok := manager.GetDimensions("shared.png"); !ok || w != 5 {
		t.Fatalf("new document width=%d, ok=%v", w, ok)
	}
}

func TestStatusConcurrentUpdates(t *testing.T) {
	var renderer Renderer
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Go(func() {
			for j := 0; j < 1000; j++ {
				renderer.SetStatusOverride("downloading")
				_ = renderer.GetStatusOverride()
			}
		})
	}
	workers.Wait()
}
