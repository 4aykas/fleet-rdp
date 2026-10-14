// Command mkicon draws an .ico of uncompressed 32-bit DIBs: by default the
// template's icon (the FleetView invader in green on a dark grey tile), or
// with -host the invader of that name in its hash colour (the maker's own
// icon). The dashboard and the maker overwrite these pixels in place for each
// launcher, which only works because no entry is PNG and every size is fixed.
package main

import (
	"bytes"
	"encoding/binary"

	"github.com/4aykas/fleet-rdp/invader"
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

const defaultColour = "#4ade80"

// renderFor returns the pixels for one size: the default icon for an empty
// host, else the host's invader in its hash colour.
func renderFor(host string) func(size int) []byte {
	if host == "" {
		g := invader.Parse(defaultShape)
		return func(size int) []byte { return invader.Render(g, defaultColour, invader.NoOrgTile, size) }
	}
	g := invader.Grid(host)
	c := invader.ColourFor(host)
	return func(size int) []byte { return invader.Render(g, c, invader.NoOrgTile, size) }
}

// buildICO writes an .ico whose every entry is a 32-bit DIB: header with
// height 2w, BGRA rows bottom-up, then an all-zero AND mask.
func buildICO(render func(size int) []byte, sizes []int) []byte {
	var images [][]byte
	for _, w := range sizes {
		rgba := render(w)
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
