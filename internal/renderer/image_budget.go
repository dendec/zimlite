package renderer

import "encoding/binary"

const maxSimpleImagePixels = 1024 * 1024
const maxAnimationBytes = 16 << 20
const maxAnimatedImages = 2

func safeImageSize(w, h int) bool {
	return w > 0 && h > 0 && int64(w)*int64(h) <= maxSimpleImagePixels
}

// Inspect GIF blocks before DecodeAll allocates every frame. Account for both
// decoded palette images and retained RGBA frames plus temporary canvases.
func safeGIFAnimation(data []byte, pixels int64) bool {
	if len(data) < 13 || pixels <= 0 {
		return false
	}
	offset := 13
	if data[10]&0x80 != 0 {
		offset += 3 << ((data[10] & 7) + 1)
	}
	frames := int64(0)
	for offset < len(data) {
		tag := data[offset]
		offset++
		switch tag {
		case 0x3b:
			return frames > 0
		case 0x21:
			offset++ // Extension label, followed by subblocks.
		case 0x2c:
			if offset+9 > len(data) {
				return false
			}
			width := int(binary.LittleEndian.Uint16(data[offset+4:]))
			height := int(binary.LittleEndian.Uint16(data[offset+6:]))
			if !safeImageSize(width, height) || int64(width)*int64(height) > pixels {
				return false
			}
			packed := data[offset+8]
			offset += 9
			if packed&0x80 != 0 {
				offset += 3 << ((packed & 7) + 1)
			}
			offset++ // LZW code size.
			frames++
			if pixels*(frames*5+12) > maxAnimationBytes {
				return false
			}
		default:
			return false
		}
		for {
			if offset >= len(data) {
				return false
			}
			size := int(data[offset])
			offset++
			if size == 0 {
				break
			}
			offset += size
		}
	}
	return false
}
