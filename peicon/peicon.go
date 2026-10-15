// Package peicon overwrites the launcher template's icon pixels in place. It
// is a port of the FleetView dashboard's dashboard/src/utils/peIcon.ts.
// build.ps1 gives the template four RT_ICON entries (16, 32, 48, 256 px),
// each an uncompressed 32-bit DIB, so new pixels fit without moving a byte.
// Anything else (no .rsrc, a PNG entry, another size or depth, an offset out
// of bounds) means this is not that template: nothing is written, 0 comes
// back, and the launcher keeps its default icon.
package peicon

import "encoding/binary"

var iconSizes = map[int]bool{16: true, 32: true, 48: true, 256: true}

const (
	rtIcon = 3
	subdir = 0x80000000
)

type slot struct{ offset, size int }

type entry struct{ id, target uint32 }

func findSlots(exe []byte) []slot {
	le := binary.LittleEndian
	ok := func(o, n int) bool { return o >= 0 && n >= 0 && o+n <= len(exe) }
	u16 := func(o int) int { return int(le.Uint16(exe[o:])) }
	u32 := func(o int) uint32 { return le.Uint32(exe[o:]) }

	if !ok(0, 0x40) || u16(0) != 0x5a4d {
		return nil
	}
	pe := int(u32(0x3c))
	if !ok(pe, 24) || u32(pe) != 0x00004550 {
		return nil
	}
	sections := u16(pe + 6)
	optSize := u16(pe + 20)
	opt := pe + 24
	if !ok(opt, optSize) || optSize < 2 {
		return nil
	}
	var dirs int
	switch u16(opt) {
	case 0x20b:
		dirs = opt + 112
	case 0x10b:
		dirs = opt + 96
	default:
		return nil
	}
	if !ok(dirs+16, 8) || dirs+24 > opt+optSize {
		return nil
	}
	rsrcRVA := u32(dirs + 16)
	if rsrcRVA == 0 {
		return nil
	}
	secTable := opt + optSize
	if !ok(secTable, sections*40) {
		return nil
	}
	toFile := func(rva uint32) int {
		for i := 0; i < sections; i++ {
			s := secTable + i*40
			va := u32(s + 12)
			// Only bytes the file really holds (SizeOfRawData): the loader zero-fills
			// the rest of a larger VirtualSize, and there is nothing to overwrite there.
			raw := u32(s + 16)
			if rva >= va && uint64(rva) < uint64(va)+uint64(raw) {
				return int(u32(s+20)) + int(rva-va)
			}
		}
		return -1
	}
	base := toFile(rsrcRVA)
	if base < 0 || !ok(base, 16) {
		return nil
	}
	entries := func(dir uint32) []entry {
		d := base + int(dir)
		if !ok(d, 16) {
			return nil
		}
		count := u16(d+12) + u16(d+14)
		if !ok(d+16, count*8) {
			return nil
		}
		out := make([]entry, count)
		for i := range out {
			out[i] = entry{u32(d + 16 + 8*i), u32(d + 20 + 8*i)}
		}
		return out
	}

	var icons *entry
	for _, e := range entries(0) {
		if e.id == rtIcon && e.target&subdir != 0 {
			icons = &e
			break
		}
	}
	if icons == nil {
		return nil
	}
	names := entries(icons.target &^ subdir)
	if names == nil {
		return nil
	}
	var slots []slot
	for _, name := range names {
		if name.target&subdir == 0 {
			return nil
		}
		langs := entries(name.target &^ subdir)
		if langs == nil {
			return nil
		}
		for _, lang := range langs {
			if lang.target&subdir != 0 || !ok(base+int(lang.target), 16) {
				return nil
			}
			off := toFile(u32(base + int(lang.target)))
			n := int(u32(base + int(lang.target) + 4))
			if off < 0 || !ok(off, n) || n < 40 {
				return nil
			}
			w := int(int32(u32(off + 4)))
			if u32(off) != 40 || !iconSizes[w] || int(int32(u32(off+8))) != 2*w || u16(off+14) != 32 {
				return nil
			}
			if n < 40+w*w*4+(w+31)/32*4*w {
				return nil
			}
			slots = append(slots, slot{off, w})
		}
	}
	return slots
}

// safeFindSlots is findSlots with a read past the end that a check missed
// turned into "not our template". Only parsing is covered: the write loop in
// SetIcons has no recover, so a panic there is never reported as "no byte
// changed".
func safeFindSlots(exe []byte) (slots []slot) {
	defer func() {
		if recover() != nil {
			slots = nil
		}
	}()
	return findSlots(exe)
}

// SetIcons returns how many icons were written; 0 means none (and no byte
// changed). render returns size*size RGBA pixels, top-down.
func SetIcons(exe []byte, render func(size int) []byte) (n int) {
	slots := safeFindSlots(exe)
	if len(slots) == 0 {
		return 0
	}
	images := make([][]byte, len(slots))
	for i, s := range slots {
		images[i] = render(s.size)
		if len(images[i]) != s.size*s.size*4 {
			return 0
		}
	}
	// In bounds without a recover: findSlots accepted a slot only when
	// ok(offset, n) held and n >= 40 + w*w*4 + mask size, so the pixels
	// (w*w*4 bytes at offset+40) and the mask after them lie inside exe, and
	// each image was checked above to hold w*w*4 bytes.
	for k, s := range slots {
		rgba := images[k]
		px := s.offset + 40
		for y := 0; y < s.size; y++ {
			row := px + (s.size-1-y)*s.size*4 // DIB rows are bottom-up
			for x := 0; x < s.size; x++ {
				src := (y*s.size + x) * 4
				d := row + x*4
				exe[d], exe[d+1], exe[d+2], exe[d+3] = rgba[src+2], rgba[src+1], rgba[src], rgba[src+3]
			}
		}
		mask := px + s.size*s.size*4
		clear(exe[mask : mask+(s.size+31)/32*4*s.size])
	}
	return len(slots)
}
