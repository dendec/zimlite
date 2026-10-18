// Package servo owns the helper protocol. It does not depend on SDL.
package servo

import (
	"encoding/binary"
	"errors"
	"image"
	"io"
)

const MaxPixels = 1280 * 720

// ReadFrame validates dimensions before allocating any helper-controlled data.
func ReadFrame(r io.Reader) (*image.RGBA, error) {
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	if string(header[:4]) != "ZSR1" {
		return nil, errors.New("unsupported Servo frame protocol")
	}
	w := binary.LittleEndian.Uint32(header[4:8])
	h := binary.LittleEndian.Uint32(header[8:12])
	if w == 0 || h == 0 || w > 1280 || h > 1280 || uint64(w)*uint64(h) > MaxPixels {
		return nil, errors.New("servo viewport exceeds memory budget")
	}
	frame := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	if _, err := io.ReadFull(r, frame.Pix); err != nil {
		return nil, err
	}
	return frame, nil
}
