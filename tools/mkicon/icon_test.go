package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestICOHoldsFourDIBs(t *testing.T) {
	ico := buildICO(renderFor(""), Sizes)
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
			binary.LittleEndian.Uint16(img[12:]) != 1 ||
			binary.LittleEndian.Uint16(img[14:]) != 32 ||
			binary.LittleEndian.Uint32(img[16:]) != 0 {
			t.Fatalf("size %d: not an uncompressed single-plane 32-bit DIB with height 2w", w)
		}
		maskRow := (w + 31) / 32 * 4
		if size != 40+w*w*4+maskRow*w {
			t.Fatalf("size %d: data size %d", w, size)
		}
	}
}

func TestRenderCornersAndBody(t *testing.T) {
	px := renderFor("")(48) // RGBA, top-down
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

// The default icon must not change when its drawing moved into package
// invader (hash of the .ico before the move).
func TestDefaultIconUnchanged(t *testing.T) {
	sum := sha256.Sum256(buildICO(renderFor(""), Sizes))
	if got := hex.EncodeToString(sum[:]); got != "6212aaae190072855ed1867652447fffe49d25f69bab64d934c391c32cdd15fb" {
		t.Fatalf("default icon changed: %s", got)
	}
}

func TestHostIconDiffersFromDefault(t *testing.T) {
	if bytes.Equal(buildICO(renderFor("fleet-rdp-maker"), Sizes), buildICO(renderFor(""), Sizes)) {
		t.Fatal("-host drew the default icon")
	}
}
