// Command mkicon draws the template's default icon: the FleetView invader in
// green on a dark grey tile, as an .ico of uncompressed 32-bit DIBs. The
// dashboard overwrites these pixels in place for each download, which only
// works because no entry is PNG and every size is fixed.
package main

import (
	"bytes"
	"encoding/binary"
)

// Sizes are the RT_ICON entries the dashboard expects (16, 32, 48, 256).
var Sizes = []int{16, 32, 48, 256}

var defaultShape = [8]string{
	"..#..#..",
	".######.",
	"##.##.##",
	"########",
	".######.",
	".#.##.#.",
	"#......#",
	".#....#.",
}

var (
	invaderRGB = [3]byte{0x4a, 0xde, 0x80}
	tileRGB    = [3]byte{0x2a, 0x2a, 0x2a}
)

// render returns size*size RGBA pixels, top-down: a rounded tile with the
// shape drawn in whole-pixel cells. The geometry matches the dashboard's
// renderIcon (dashboard/src/utils/invader.ts).
func render(shape [8]string, size int) []byte {
	pad := 0
	if size > 16 {
		pad = (size*125 + 500) / 1000 // round(size * 0.125)
	}
	cell := (size - 2*pad) / 8
	off := (size - 8*cell) / 2
	r := 2
	if size > 16 {
		r = (size*2 + 5) / 10 // round(size * 0.2)
	}
	px := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			if !insideRounded(x, y, size, r) {
				continue // transparent
			}
			c := tileRGB
			cx, cy := x-off, y-off
			if cx >= 0 && cy >= 0 && cx < 8*cell && cy < 8*cell && shape[cy/cell][cx/cell] == '#' {
				c = invaderRGB
			}
			px[i], px[i+1], px[i+2], px[i+3] = c[0], c[1], c[2], 0xff
		}
	}
	return px
}

func insideRounded(x, y, size, r int) bool {
	fx, fy, fr := float64(x)+0.5, float64(y)+0.5, float64(r)
	dx := max(fr-fx, fx-(float64(size)-fr), 0)
	dy := max(fr-fy, fy-(float64(size)-fr), 0)
	return dx*dx+dy*dy <= fr*fr
}

// buildICO writes an .ico whose every entry is a 32-bit DIB: header with
// height 2w, BGRA rows bottom-up, then an all-zero AND mask.
func buildICO(shape [8]string, sizes []int) []byte {
	var images [][]byte
	for _, w := range sizes {
		rgba := render(shape, w)
		var b bytes.Buffer
		hdr := make([]byte, 40)
		binary.LittleEndian.PutUint32(hdr[0:], 40)
		binary.LittleEndian.PutUint32(hdr[4:], uint32(w))
		binary.LittleEndian.PutUint32(hdr[8:], uint32(2*w))
		binary.LittleEndian.PutUint16(hdr[12:], 1)
		binary.LittleEndian.PutUint16(hdr[14:], 32)
		b.Write(hdr)
		for y := w - 1; y >= 0; y-- {
			for x := 0; x < w; x++ {
				i := (y*w + x) * 4
				b.Write([]byte{rgba[i+2], rgba[i+1], rgba[i], rgba[i+3]})
			}
		}
		b.Write(make([]byte, (w+31)/32*4*w))
		images = append(images, b.Bytes())
	}
	var out bytes.Buffer
	head := make([]byte, 6)
	binary.LittleEndian.PutUint16(head[2:], 1)
	binary.LittleEndian.PutUint16(head[4:], uint16(len(sizes)))
	out.Write(head)
	offset := 6 + 16*len(sizes)
	for i, w := range sizes {
		e := make([]byte, 16)
		e[0], e[1] = byte(w%256), byte(w%256) // 0 means 256
		binary.LittleEndian.PutUint16(e[4:], 1)
		binary.LittleEndian.PutUint16(e[6:], 32)
		binary.LittleEndian.PutUint32(e[8:], uint32(len(images[i])))
		binary.LittleEndian.PutUint32(e[12:], uint32(offset))
		out.Write(e)
		offset += len(images[i])
	}
	for _, img := range images {
		out.Write(img)
	}
	return out.Bytes()
}
