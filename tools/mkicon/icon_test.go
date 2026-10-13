package main

import (
	"encoding/binary"
	"testing"
)

func TestICOHoldsFourDIBs(t *testing.T) {
	ico := buildICO(defaultShape, Sizes)
	if binary.LittleEndian.Uint16(ico[2:]) != 1 || int(binary.LittleEndian.Uint16(ico[4:])) != len(Sizes) {
		t.Fatalf("header: type/count wrong")
	}
	for i, w := range Sizes {
		e := ico[6+16*i:]
		size := int(binary.LittleEndian.Uint32(e[8:]))
		off := int(binary.LittleEndian.Uint32(e[12:]))
		img := ico[off : off+size]
		if binary.LittleEndian.Uint32(img[0:]) != 40 ||
			int(int32(binary.LittleEndian.Uint32(img[4:]))) != w ||
			int(int32(binary.LittleEndian.Uint32(img[8:]))) != 2*w ||
			binary.LittleEndian.Uint16(img[14:]) != 32 {
			t.Fatalf("size %d: not a 32-bit DIB with height 2w", w)
		}
		maskRow := (w + 31) / 32 * 4
		if size != 40+w*w*4+maskRow*w {
			t.Fatalf("size %d: data size %d", w, size)
		}
	}
}

func TestRenderCornersAndBody(t *testing.T) {
	px := render(defaultShape, 48) // RGBA, top-down
	if a := px[3]; a != 0 {
		t.Fatalf("top-left corner alpha = %d, want 0 (rounded tile)", a)
	}
	// 48 px: cell 4, offset 8. Row 3 of the default shape is all filled, so
	// cell (3, 3) is invader-coloured.
	x, y := 8+3*4+1, 8+3*4+1
	i := (y*48 + x) * 4
	if px[i] != 0x4a || px[i+1] != 0xde || px[i+2] != 0x80 || px[i+3] != 0xff {
		t.Fatalf("body pixel = %v, want #4ade80", px[i:i+4])
	}
}
