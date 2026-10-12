//go:build !windows

package main

import (
	"fmt"
	"os"
)

// The launcher only does something on Windows; this keeps `go build ./...`
// and `go test ./...` working on other systems.
func main() {
	fmt.Fprintln(os.Stderr, "fleet-rdp launcher runs on Windows only")
	os.Exit(1)
}
