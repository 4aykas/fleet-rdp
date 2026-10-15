package peicon

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/peicon-shortraw-v1.json")

type peOpts struct {
	bpp uint16
	png bool
}

// makePE builds a minimal PE32+ whose .rsrc holds one RT_ICON per size, each a
// 32-bit DIB filled with 0x11.
func makePE(sizes []int, o peOpts) []byte {
	le := binary.LittleEndian
	if o.bpp == 0 {
		o.bpp = 32
	}
	dib := func(w int) []byte {
		b := make([]byte, 40+w*w*4+(w+31)/32*4*w)
		if o.png {
			copy(b, []byte{0x89, 'P', 'N', 'G'})
		} else {
			le.PutUint32(b[0:], 40)
			le.PutUint32(b[4:], uint32(w))
			le.PutUint32(b[8:], uint32(2*w))
			le.PutUint16(b[12:], 1)
			le.PutUint16(b[14:], o.bpp)
		}
		for i := 40; i < len(b); i++ {
			b[i] = 0x11
		}
		return b
	}
	n := len(sizes)
	l2 := 16 + 8
	l3 := l2 + 16 + 8*n
	dataEnt := l3 + n*(16+8)
	imgsOff := dataEnt + n*16
	imgs := make([][]byte, n)
	total := 0
	for i, w := range sizes {
		imgs[i] = dib(w)
		total += len(imgs[i])
	}
	rsrcSize := imgsOff + total
	const rsrcFile, rsrcRVA = 0x200, 0x1000
	pe := make([]byte, rsrcFile+rsrcSize)
	pe[0], pe[1] = 'M', 'Z'
	le.PutUint32(pe[0x3c:], 0x40)
	copy(pe[0x40:], "PE\x00\x00")
	le.PutUint16(pe[0x44:], 0x8664)
	le.PutUint16(pe[0x46:], 1)   // sections
	le.PutUint16(pe[0x54:], 240) // optional header size (PE32+)
	opt := 0x58
	le.PutUint16(pe[opt:], 0x20b)
	le.PutUint32(pe[opt+108:], 16)         // NumberOfRvaAndSizes
	le.PutUint32(pe[opt+112+16:], rsrcRVA) // resource directory RVA
	le.PutUint32(pe[opt+112+20:], uint32(rsrcSize))
	sec := opt + 240
	copy(pe[sec:], ".rsrc")
	le.PutUint32(pe[sec+8:], uint32(rsrcSize))
	le.PutUint32(pe[sec+12:], rsrcRVA)
	le.PutUint32(pe[sec+16:], uint32(rsrcSize))
	le.PutUint32(pe[sec+20:], rsrcFile)
	R := rsrcFile
	le.PutUint16(pe[R+14:], 1) // one id entry
	le.PutUint32(pe[R+16:], 3) // RT_ICON
	le.PutUint32(pe[R+20:], 0x80000000|uint32(l2))
	le.PutUint16(pe[R+l2+14:], uint16(n))
	img := imgsOff
	for i := range sizes {
		le.PutUint32(pe[R+l2+16+8*i:], uint32(i+1))
		e3 := l3 + i*24
		le.PutUint32(pe[R+l2+20+8*i:], 0x80000000|uint32(e3))
		le.PutUint16(pe[R+e3+14:], 1)
		le.PutUint32(pe[R+e3+16:], 0x409)
		de := dataEnt + i*16
		le.PutUint32(pe[R+e3+20:], uint32(de))
		le.PutUint32(pe[R+de:], uint32(rsrcRVA+img))
		le.PutUint32(pe[R+de+4:], uint32(len(imgs[i])))
		copy(pe[R+img:], imgs[i])
		img += len(imgs[i])
	}
	return pe
}

// solid: every pixel (10,20,30,255) except the top-left one, (1,2,3,4).
func solid(size int) []byte {
	p := make([]byte, size*size*4)
	for i := 0; i < size*size; i++ {
		copy(p[i*4:], []byte{10, 20, 30, 255})
	}
	copy(p, []byte{1, 2, 3, 4})
	return p
}

// dibAt returns the image data of the i-th icon in makePE's layout.
func dibAt(pe []byte, sizes []int, i int) []byte {
	n := len(sizes)
	off := 0x200 + 16 + 8 + 16 + 8*n + n*24 + n*16
	for k := 0; k < i; k++ {
		w := sizes[k]
		off += 40 + w*w*4 + (w+31)/32*4*w
	}
	w := sizes[i]
	return pe[off : off+40+w*w*4+(w+31)/32*4*w]
}

func TestWritesBGRABottomUpAndZeroesMask(t *testing.T) {
	sizes := []int{16, 32}
	pe := makePE(sizes, peOpts{})
	if n := SetIcons(pe, solid); n != 2 {
		t.Fatalf("SetIcons = %d, want 2", n)
	}
	for i, w := range sizes {
		d := dibAt(pe, sizes, i)
		px := d[40:]
		// The top-left source pixel lands at the start of the last DIB row.
		last := (w - 1) * w * 4
		if !bytes.Equal(px[last:last+4], []byte{3, 2, 1, 4}) {
			t.Fatalf("%d: marker pixel %v, want BGRA 3,2,1,4", w, px[last:last+4])
		}
		if !bytes.Equal(px[0:4], []byte{30, 20, 10, 255}) {
			t.Fatalf("%d: body pixel %v", w, px[0:4])
		}
		for _, b := range px[w*w*4:] {
			if b != 0 {
				t.Fatalf("%d: mask not zeroed", w)
			}
		}
	}
}

func TestRefusesAnythingElseUnchanged(t *testing.T) {
	cases := map[string][]byte{
		"empty":     {},
		"not MZ":    bytes.Repeat([]byte{0}, 0x400),
		"png entry": makePE([]int{16}, peOpts{png: true}),
		"24 bpp":    makePE([]int{16}, peOpts{bpp: 24}),
		"odd size":  makePE([]int{20}, peOpts{}),
	}
	noRsrc := makePE([]int{16}, peOpts{})
	binary.LittleEndian.PutUint32(noRsrc[0x58+112+16:], 0)
	cases["no .rsrc"] = noRsrc
	badRVA := makePE([]int{16}, peOpts{})
	// The data entry of the only icon points far past the section.
	de := 0x200 + 16 + 8 + 16 + 8 + 24
	binary.LittleEndian.PutUint32(badRVA[de:], 0x7fff0000)
	cases["RVA out of bounds"] = badRVA
	full := makePE([]int{16, 32}, peOpts{})
	cases["truncated"] = full[:len(full)-100]
	for name, pe := range cases {
		before := bytes.Clone(pe)
		if n := SetIcons(pe, solid); n != 0 {
			t.Errorf("%s: SetIcons = %d, want 0", name, n)
		}
		if !bytes.Equal(pe, before) {
			t.Errorf("%s: bytes changed", name)
		}
	}
}

func TestWrongRenderLengthWritesNothing(t *testing.T) {
	pe := makePE([]int{16, 32}, peOpts{})
	before := bytes.Clone(pe)
	short := func(size int) []byte { return make([]byte, 4) }
	if n := SetIcons(pe, short); n != 0 || !bytes.Equal(pe, before) {
		t.Fatalf("SetIcons = %d or bytes changed", n)
	}
}

// The real template has four icons (16, 32, 48, 256). Skips without a build.
func TestRealTemplate(t *testing.T) {
	exe, err := os.ReadFile(filepath.Join("..", "dist", "fleet-rdp-launcher.exe"))
	if err != nil {
		t.Skip("no dist/fleet-rdp-launcher.exe; run build.ps1")
	}
	if n := SetIcons(exe, solid); n != 4 {
		t.Fatalf("SetIcons = %d, want 4", n)
	}
}

func TestGarbageInputReturnsZeroUnchanged(t *testing.T) {
	good := makePE([]int{16, 32}, peOpts{})
	for _, cut := range []int{0x3c, 0x40, 0x80, 0x100, 0x1f0, 0x210, 0x240} {
		pe := bytes.Clone(good[:cut])
		before := bytes.Clone(pe)
		if n := SetIcons(pe, solid); n != 0 || !bytes.Equal(pe, before) {
			t.Errorf("cut %#x: SetIcons = %d or bytes changed", cut, n)
		}
	}
	garbage := bytes.Repeat([]byte{0xff}, 0x600)
	garbage[0], garbage[1] = 'M', 'Z'
	before := bytes.Clone(garbage)
	if n := SetIcons(garbage, solid); n != 0 || !bytes.Equal(garbage, before) {
		t.Errorf("garbage: SetIcons = %d or bytes changed", n)
	}
}

// vectorCase is one PE and what SetIcons must do with it. The dashboard reads
// the same file (dashboard/test/peIcon.vector.test.ts).
type vectorCase struct {
	Name      string `json:"name"`
	PEBase64  string `json:"pe_base64"`
	Written   int    `json:"written"`
	Unchanged bool   `json:"unchanged"`
}

func buildPEVector() []vectorCase {
	const section = 0x58 + 240 // the one section header
	le := binary.LittleEndian
	with := func(virtual, raw uint32) []byte {
		pe := makePE([]int{16, 32}, peOpts{})
		le.PutUint32(pe[section+8:], virtual)
		if raw != 0 {
			le.PutUint32(pe[section+16:], raw)
		}
		return pe
	}
	enc := base64.StdEncoding.EncodeToString
	return []vectorCase{
		{Name: "virtual size larger than raw data, icons inside the raw data", PEBase64: enc(with(0x10000, 0)), Written: 2},
		{Name: "icons past SizeOfRawData, though VirtualSize covers them", PEBase64: enc(with(0x10000, 0x80)), Written: 0, Unchanged: true},
	}
}

func TestSharedVector(t *testing.T) {
	path := filepath.Join("..", "testdata", "peicon-shortraw-v1.json")
	want, err := json.MarshalIndent(buildPEVector(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if *update {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./peicon -update)", err)
	}
	if string(raw) != string(want) {
		t.Fatal("testdata/peicon-shortraw-v1.json is stale: run go test ./peicon -update and copy it to FleetView")
	}
	var cases []vectorCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		pe, err := base64.StdEncoding.DecodeString(c.PEBase64)
		if err != nil {
			t.Fatal(err)
		}
		before := bytes.Clone(pe)
		if n := SetIcons(pe, solid); n != c.Written {
			t.Errorf("%s: SetIcons = %d, want %d", c.Name, n, c.Written)
		}
		if c.Unchanged && !bytes.Equal(pe, before) {
			t.Errorf("%s: bytes changed", c.Name)
		}
	}
}
