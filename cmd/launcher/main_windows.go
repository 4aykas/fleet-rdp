//go:build windows

// The launcher template. Built as a GUI program (-H windowsgui) so a
// double-click opens no console window; errors go to a message box instead.
package main

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"time"

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
		reach: func(host string, port int) error {
			c, err := net.DialTimeout("tcp", net.JoinHostPort(launch.BareHost(host), strconv.Itoa(port)), 5*time.Second)
			if err != nil {
				return err
			}
			return c.Close()
		},
		askRetry:     askRetry,
		writeCred:    credstore.Write,
		monitorCount: launch.MonitorCount,
		start:        launch.StartMstsc,
	}
	name, err := run(d)
	if err != nil {
		if errors.Is(err, errCancelled) {
			os.Exit(0)
		}
		showError(dialogTitle(name), err.Error())
		os.Exit(1)
	}
}

// idRetry is the MessageBox result for the Retry button.
const idRetry = 4

func askRetry(title, text string) bool {
	t, err1 := windows.UTF16PtrFromString(title)
	m, err2 := windows.UTF16PtrFromString(text)
	if err1 != nil || err2 != nil {
		return false
	}
	r, _ := windows.MessageBox(0, m, t, windows.MB_RETRYCANCEL|windows.MB_ICONWARNING)
	return r == idRetry
}

func showError(title, text string) {
	t, err1 := windows.UTF16PtrFromString(title)
	m, err2 := windows.UTF16PtrFromString(text)
	if err1 != nil || err2 != nil {
		return
	}
	_, _ = windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}
