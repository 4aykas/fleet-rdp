//go:build windows

// The launcher template. Built as a GUI program (-H windowsgui) so a
// double-click opens no console window; errors go to a message box instead.
package main

import (
	"context"
	"os"

	"golang.org/x/sys/windows"

	"github.com/4aykas/fleet-rdp/cert"
	"github.com/4aykas/fleet-rdp/credstore"
	"github.com/4aykas/fleet-rdp/launch"
	"github.com/4aykas/fleet-rdp/payload"
)

func main() {
	d := deps{
		readSelf: func() ([]byte, error) {
			exe, err := os.Executable()
			if err != nil {
				return nil, err
			}
			return os.ReadFile(exe)
		},
		flags: os.Args[1:],
		ensureCert: func(p payload.Payload) error {
			return launch.EnsureCert(p, launch.NewRegistryStore(), func(host string, port int) ([]byte, error) {
				return cert.Fetch(context.Background(), host, port)
			})
		},
		writeCred:    credstore.Write,
		monitorCount: launch.MonitorCount,
		start:        launch.StartMstsc,
	}
	name, err := run(d)
	if err != nil {
		title := "Fleet RDP"
		if name != "" {
			title = name + " - Fleet RDP"
		}
		showError(title, err.Error())
		os.Exit(1)
	}
}

func showError(title, text string) {
	t, err1 := windows.UTF16PtrFromString(title)
	m, err2 := windows.UTF16PtrFromString(text)
	if err1 != nil || err2 != nil {
		return
	}
	_, _ = windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}
