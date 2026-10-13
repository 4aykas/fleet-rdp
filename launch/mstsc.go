// Package launch is what the launcher does around mstsc: the arguments, the
// monitor choice, the pinned certificate. Everything here except the *_windows
// files runs on any OS so it can be tested anywhere.
package launch

import (
	"fmt"
	"strings"
)

// BareHost strips IPv6 brackets. The registry key and the credential target
// use the bare address; only the mstsc argument brackets it.
func BareHost(host string) string {
	return strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
}

// MstscArgs builds `mstsc /v:host:port /f [/multimon]`.
func MstscArgs(host string, port int, multi bool) []string {
	h := BareHost(host)
	if strings.Contains(h, ":") {
		h = "[" + h + "]"
	}
	args := []string{fmt.Sprintf("/v:%s:%d", h, port), "/f"}
	if multi {
		args = append(args, "/multimon")
	}
	return args
}

// ChooseMulti decides /multimon. The payload's mode comes first: "one" never,
// "all" always, anything else by monitor count. --single or --multi on the
// command line override it (the last one wins). A count of 0 means the
// detection failed; that is treated as one monitor.
func ChooseMulti(monitorCount int, mode string, flags []string) bool {
	var multi bool
	switch mode {
	case "one":
		multi = false
	case "all":
		multi = true
	default:
		multi = monitorCount > 1
	}
	for _, f := range flags {
		switch f {
		case "--single":
			multi = false
		case "--multi":
			multi = true
		}
	}
	return multi
}
