//go:build windows

package launch

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// StartMstsc starts mstsc.exe from the system directory (not from PATH, so a
// planted mstsc.exe in the download folder is never picked up) and does not
// wait for it.
func StartMstsc(args []string) error {
	dir, err := windows.GetSystemDirectory()
	if err != nil {
		return fmt.Errorf("could not locate the system directory: %w", err)
	}
	cmd := exec.Command(filepath.Join(dir, "mstsc.exe"), args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start mstsc: %w", err)
	}
	return cmd.Process.Release()
}
