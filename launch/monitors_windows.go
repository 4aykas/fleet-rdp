//go:build windows

package launch

import "golang.org/x/sys/windows"

const smCMonitors = 80 // SM_CMONITORS: displays that are part of the desktop

var getSystemMetrics = windows.NewLazySystemDLL("user32.dll").NewProc("GetSystemMetrics")

// MonitorCount returns 0 when the call fails, which ChooseMulti treats as one
// monitor. Verified live in Task 6, not by a unit test (CI has no display).
func MonitorCount() int {
	n, _, _ := getSystemMetrics.Call(smCMonitors)
	return int(n)
}
