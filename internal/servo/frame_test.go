package servo

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadFrameRejectsUntrustedDimensions(t *testing.T) {
	for _, size := range [][2]uint32{{0, 1}, {1, 0}, {0xffffffff, 0xffffffff}, {1281, 720}} {
		var header [12]byte
		copy(header[:], "ZSR1")
		binary.LittleEndian.PutUint32(header[4:8], size[0])
		binary.LittleEndian.PutUint32(header[8:12], size[1])
		if _, err := ReadFrame(bytes.NewReader(header[:])); err == nil {
			t.Fatalf("accepted dimensions %v", size)
		}
	}
}

func TestReadFrameTruncatedAndValid(t *testing.T) {
	data := []byte{'Z', 'S', 'R', '1', 1, 0, 0, 0, 1, 0, 0, 0, 7, 8, 9, 255}
	for n := 0; n < len(data); n++ {
		if _, err := ReadFrame(bytes.NewReader(data[:n])); err == nil {
			t.Fatalf("accepted truncated frame with %d bytes", n)
		}
	}
	frame, err := ReadFrame(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame.Pix, data[12:]) {
		t.Fatal("pixel contents changed")
	}
}
