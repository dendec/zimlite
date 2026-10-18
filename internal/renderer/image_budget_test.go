package renderer

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
)

func TestGIFBudgetInspectsFramesBeforeDecodeAll(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 128, 128), color.Palette{color.Black, color.White})
	for _, count := range []int{2, 250} {
		animation := &gif.GIF{}
		for i := 0; i < count; i++ {
			animation.Image = append(animation.Image, frame)
			animation.Delay = append(animation.Delay, 1)
		}
		var encoded bytes.Buffer
		if err := gif.EncodeAll(&encoded, animation); err != nil {
			t.Fatal(err)
		}
		if safe := safeGIFAnimation(encoded.Bytes(), 128*128); safe != (count == 2) {
			t.Fatalf("frame count=%d, safe=%v", count, safe)
		}
	}
	if safeGIFAnimation([]byte("GIF89a"), 128*128) {
		t.Fatal("accepted truncated GIF")
	}
	if safeImageSize(4096, 4096) {
		t.Fatal("accepted image that exceeds the fallback pixel budget")
	}
}
