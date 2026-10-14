package invader

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/invader-v1.json")

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Computed with dashboard/src/utils/invader.ts (the reference).
func TestMatchesDashboardGolden(t *testing.T) {
	cases := []struct {
		host        string
		rows        [8]string
		colour      string
		sha16, s256 string
	}{
		{"SGCLOUD",
			[8]string{".##..##.", "...##...", "########", "##.##.##", ".######.", "...##...", "#..##..#", "..#..#.."},
			"#38bdf8",
			"f5aa7e3f5385bc6a9ae355b09d1bd30baf8163f36a609a688cab409c74809ed8",
			"b966998f22c2091790c7c95c3dcd816482a4b6f4329a3573ad517e00a6f5f76e"},
		{"rdp.example",
			[8]string{".##..##.", "..####..", "#.####.#", "##.##.##", "..####..", "########", ".######.", "##....##"},
			"#ff6ec7",
			"8a0604428d926347eea187cb1e53194cf92608760d32059c2322f3d28de51c75",
			"7ed78b05b6bd82dc760b372474146ef47e2b072a6431b6de3240a72a0cf4edb4"},
	}
	for _, c := range cases {
		g := Grid(c.host)
		if Rows(g) != c.rows {
			t.Errorf("%s: grid %v, want %v", c.host, Rows(g), c.rows)
		}
		if got := ColourFor(c.host); got != c.colour {
			t.Errorf("%s: colour %s, want %s", c.host, got, c.colour)
		}
		if got := sha(Render(g, c.colour, NoOrgTile, 16)); got != c.sha16 {
			t.Errorf("%s: 16 px sha %s", c.host, got)
		}
		if got := sha(Render(g, c.colour, NoOrgTile, 256)); got != c.s256 {
			t.Errorf("%s: 256 px sha %s", c.host, got)
		}
	}
}

func TestGridIsSymmetricAndBounded(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 1000; i++ {
		host := fmt.Sprintf("host-%d-%d", i, r.Uint32())
		g := Grid(host)
		filled, emptyRows := 0, 0
		for y := 0; y < 8; y++ {
			rowEmpty := true
			for x := 0; x < 8; x++ {
				if g[y][x] != g[y][7-x] {
					t.Fatalf("%s: row %d not symmetric", host, y)
				}
				if g[y][x] {
					filled++
					rowEmpty = false
				}
			}
			if rowEmpty {
				emptyRows++
			}
		}
		if filled < 26 || filled > 46 || emptyRows > 1 {
			t.Fatalf("%s: filled %d, empty rows %d", host, filled, emptyRows)
		}
	}
}

func TestRenderShape(t *testing.T) {
	for _, size := range []int{16, 32, 48, 256} {
		px := Render(Grid("SGCLOUD"), "#38bdf8", NoOrgTile, size)
		if len(px) != size*size*4 {
			t.Fatalf("%d: len %d", size, len(px))
		}
		if px[3] != 0 {
			t.Fatalf("%d: top-left corner is not transparent", size)
		}
	}
}

// vectorHosts: rivers' hostnames, an IP, an IPv6 literal and a long DNS name.
var vectorHosts = []string{
	"SGCLOUD", "SGS", "DBS", "REMOTE02", "WIN-UF5UMK2KNVD", "rdp.example",
	"168.119.80.235", "2a01:4f8:c17:1::1",
	"very-long-subdomain-name.example-company-domain.tebin.pro",
}

type vectorHost struct {
	Host   string            `json:"host"`
	Grid   [8]string         `json:"grid"`
	Colour string            `json:"colour"`
	Sha256 map[string]string `json:"sha256"`
}

type vector struct {
	Tile  string       `json:"tile"`
	Hosts []vectorHost `json:"hosts"`
}

func buildVector() vector {
	v := vector{Tile: NoOrgTile}
	for _, h := range vectorHosts {
		g := Grid(h)
		vh := vectorHost{Host: h, Grid: Rows(g), Colour: ColourFor(h), Sha256: map[string]string{}}
		for _, size := range []int{16, 32, 48, 256} {
			vh.Sha256[fmt.Sprint(size)] = sha(Render(g, vh.Colour, NoOrgTile, size))
		}
		v.Hosts = append(v.Hosts, vh)
	}
	return v
}

// The dashboard checks the same file (dashboard/test/invader.vector.test.ts).
func TestSharedVector(t *testing.T) {
	path := filepath.Join("..", "testdata", "invader-v1.json")
	want, err := json.MarshalIndent(buildVector(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if *update {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./invader -update)", err)
	}
	if string(got) != string(want) {
		t.Fatal("testdata/invader-v1.json is stale: run go test ./invader -update and copy it to FleetView")
	}
}

func TestRenderRejectsBadColours(t *testing.T) {
	g := Grid("rdp.example.test")
	for _, bad := range []string{"", "red", "#12", "#gggggg", "#1234567", "123456", " #123456"} {
		if Render(g, bad, NoOrgTile, 16) != nil {
			t.Errorf("colour %q: want nil", bad)
		}
		if Render(g, "#ff5a5f", bad, 16) != nil {
			t.Errorf("tile %q: want nil", bad)
		}
	}
	if got := Render(g, "#ABCDEF", "#abcdef", 16); len(got) != 16*16*4 {
		t.Errorf("valid mixed-case colours: length %d", len(got))
	}
}
