package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/mkicon OUT.ico")
		os.Exit(2)
	}
	if err := os.WriteFile(os.Args[1], buildICO(defaultShape, Sizes), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "mkicon:", err)
		os.Exit(1)
	}
}
