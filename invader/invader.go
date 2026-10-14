// Package invader draws the launcher icon: a pixel invader from a hostname, in
// a palette colour, on a rounded tile. It is a port of the FleetView
// dashboard's dashboard/src/utils/invader.ts, which is the reference; the
// shared vector testdata/invader-v1.json keeps the two identical to the pixel.
package invader

import (
	"math"
	"strconv"
)

// Colours is the dashboard's INVADER_COLOURS (index = servers.icon_colour).
var Colours = [10]string{
	"#ff5a5f", "#ff9f1c", "#ffd60a", "#a3e635", "#2ec27e",
	"#2dd4bf", "#38bdf8", "#6b8cff", "#b388ff", "#ff6ec7",
}

// NoOrgTile is the tile of a server with no organization.
const NoOrgTile = "#2a2a2a"

// fallback is the FleetView invader, used when no generated shape passes.
var fallback = [8]string{"..#..#..", ".######.", "##.##.##", "########", ".######.", ".#.##.#.", "#......#", ".#....#."}

// profile is the chance of a filled cell per row: antennae, wide body, legs.
var profile = [8]float64{0.35, 0.5, 0.75, 0.85, 0.85, 0.7, 0.5, 0.45}

func fnv1a(s string) uint32 {
	h := uint32(0x811c9dc5)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 0x01000193
	}
	return h
}

func xorshift(seed uint32) func() float64 {
	x := seed
	if x == 0 {
		x = 1
	}
	return func() float64 {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		return float64(x) / 4294967296
	}
}

// ColourFor is the dashboard's colourFor(null, host): the stable hash colour
// of a server the Worker has not assigned one to.
func ColourFor(host string) string {
	return Colours[fnv1a("colour:"+host)%uint32(len(Colours))]
}

// Grid is the invader for host. Same host, same invader.
func Grid(host string) [8][8]bool {
	r := xorshift(fnv1a(host))
	for attempt := 0; attempt < 50; attempt++ {
		var g [8][8]bool
		for y := 0; y < 8; y++ {
			for x := 0; x < 4; x++ {
				p := profile[y]
				if x == 0 {
					p *= 0.6
				}
				if r() < p {
					g[y][x], g[y][7-x] = true, true
				}
			}
		}
		for y := 2; y <= 5; y++ {
			g[y][3], g[y][4] = true, true // the body never falls apart
		}
		ey := 2 + int(math.Floor(r()*2)) // eyes on row 2 or 3
		g[ey][2], g[ey][5] = false, false
		g[ey][1], g[ey][6] = true, true
		g[ey-1][2], g[ey-1][5] = true, true
		g[ey+1][2], g[ey+1][5] = true, true
		filled, emptyRows := 0, 0
		for y := 0; y < 8; y++ {
			empty := true
			for x := 0; x < 8; x++ {
				if g[y][x] {
					filled++
					empty = false
				}
			}
			if empty {
				emptyRows++
			}
		}
		if filled >= 26 && filled <= 46 && emptyRows <= 1 {
			return g
		}
	}
	return Parse(fallback)
}

// Parse reads rows of '#' (filled) and anything else (empty).
func Parse(rows [8]string) [8][8]bool {
	var g [8][8]bool
	for y, row := range rows {
		for x := 0; x < 8 && x < len(row); x++ {
			g[y][x] = row[x] == '#'
		}
	}
	return g
}

// Rows is the inverse of Parse.
func Rows(g [8][8]bool) [8]string {
	var out [8]string
	for y := range g {
		b := make([]byte, 8)
		for x := range g[y] {
			b[x] = '.'
			if g[y][x] {
				b[x] = '#'
			}
		}
		out[y] = string(b)
	}
	return out
}

// rgb reads exactly '#' and six hex digits (either case).
func rgb(hex string) ([3]byte, bool) {
	if len(hex) != 7 || hex[0] != '#' {
		return [3]byte{}, false
	}
	for i := 1; i < 7; i++ {
		c := hex[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return [3]byte{}, false
		}
	}
	n, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return [3]byte{}, false
	}
	return [3]byte{byte(n >> 16), byte(n >> 8), byte(n)}, true
}

// round is JavaScript's Math.round for non-negative values.
func round(f float64) int { return int(math.Floor(f + 0.5)) }

// Render returns size*size RGBA pixels, top-down: a rounded tile with the
// grid in whole-pixel cells so the art stays sharp at every size. Pixels
// outside the rounded tile are transparent. It returns nil when colour or tile
// is not '#' plus six hex digits.
func Render(g [8][8]bool, colour, tile string, size int) []byte {
	pad := 0
	rad := 2
	if size > 16 {
		pad = round(float64(size) * 0.125)
		rad = round(float64(size) * 0.2)
	}
	cell := (size - 2*pad) / 8
	off := (size - 8*cell) / 2
	ic, ok1 := rgb(colour)
	tc, ok2 := rgb(tile)
	if !ok1 || !ok2 {
		return nil
	}
	fr, fs := float64(rad), float64(size)
	out := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			dx := math.Max(math.Max(fr-fx, fx-(fs-fr)), 0)
			dy := math.Max(math.Max(fr-fy, fy-(fs-fr)), 0)
			if dx*dx+dy*dy > fr*fr {
				continue
			}
			cx, cy := x-off, y-off
			on := cx >= 0 && cy >= 0 && cx < 8*cell && cy < 8*cell && g[cy/cell][cx/cell]
			c := tc
			if on {
				c = ic
			}
			i := (y*size + x) * 4
			out[i], out[i+1], out[i+2], out[i+3] = c[0], c[1], c[2], 0xff
		}
	}
	return out
}
