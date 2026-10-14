//go:build windows

package main

import (
	"fmt"
	"os"
)

// The maker is a Windows window; this keeps `go build ./...` and
// `go test ./...` working on other systems. Task 5 replaces the Windows stub.
func main() {
	fmt.Fprintln(os.Stderr, "fleet-rdp-maker runs on Windows only")
	os.Exit(1)
}
