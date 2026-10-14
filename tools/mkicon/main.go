package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	host := flag.String("host", "", "draw this name's invader in its hash colour instead of the default icon")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, "usage: go run ./tools/mkicon [-host NAME] OUT.ico") }
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := os.WriteFile(flag.Arg(0), buildICO(renderFor(*host), Sizes), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "mkicon:", err)
		os.Exit(1)
	}
}
