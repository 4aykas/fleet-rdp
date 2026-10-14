package main

import (
	"embed"
	"errors"
)

// The launcher template, copied here by build.ps1 from the same build. The
// README keeps the directory (and so the go:embed pattern) valid in a plain
// checkout.
//
//go:embed template
var templateFS embed.FS

var errNoTemplate = errors.New("This maker was built without the launcher template. Build it with build.ps1.")

func embeddedTemplate() ([]byte, error) {
	b, err := templateFS.ReadFile("template/fleet-rdp-launcher.exe")
	if err != nil {
		return nil, errNoTemplate
	}
	return b, nil
}
