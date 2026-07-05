package svg

import (
	"testing"
)

func TestRenderSVG(t *testing.T) {
	// A simple SVG string with standard px units
	svgData := []byte(`
		<svg width="100px" height="50px" xmlns="http://www.w3.org/2000/svg">
			<rect width="100%" height="100%" fill="red" />
		</svg>
	`)

	img := Render(svgData)
	if img == nil {
		t.Fatal("Render() returned nil for valid SVG")
	}

	if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 50 {
		t.Errorf("Expected dimensions 100x50, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestRenderSVGWithExEmUnits(t *testing.T) {
	// A simple SVG string using ex and em units (Wikipedia math style)
	// 1 ex = 8px, 1 em = 16px.
	// So 2ex = 16px, 1.5em = 24px.
	svgData := []byte(`
		<svg width="2ex" height="1.5em" xmlns="http://www.w3.org/2000/svg">
			<rect width="100%" height="100%" fill="blue" />
		</svg>
	`)

	img := Render(svgData)
	if img == nil {
		t.Fatal("Render() returned nil for SVG with ex/em units")
	}

	if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 24 {
		t.Errorf("Expected dimensions 16x24, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestRenderInvalidSVG(t *testing.T) {
	svgData := []byte("not an svg file")

	img := Render(svgData)
	if img != nil {
		t.Error("Render() should return nil for invalid SVG data")
	}
}
