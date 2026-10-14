package maker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.zip")
	if err := WriteAtomic(p, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(p, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "new" {
		t.Fatalf("got %q", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("%d files left in the directory", len(entries))
	}
}

func TestWriteAtomicMissingDirectory(t *testing.T) {
	if err := WriteAtomic(filepath.Join(t.TempDir(), "no", "out.zip"), []byte("x")); err == nil {
		t.Fatal("want an error")
	}
}
