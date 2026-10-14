package invader

import "testing"

func TestNonASCIIHost(t *testing.T) {
	// dashboard/src/utils/invader.ts hashes TextEncoder (UTF-8) bytes too, as
	// this package does with the bytes of a Go string. Changing the hashing
	// here breaks parity with the dashboard.
	const host = "сервер-1.example.test"
	wantRows := [8]string{
		"........", "..#..#..", ".#.##.#.", ".######.",
		".######.", ".######.", "..####..", ".######.",
	}
	if got := Rows(Grid(host)); got != wantRows {
		t.Errorf("rows %q", got)
	}
	if got := ColourFor(host); got != "#ff6ec7" {
		t.Errorf("colour %s", got)
	}
}
