package maker

import (
	"errors"
	"strconv"
	"strings"

	"github.com/4aykas/fleet-rdp/payload"
)

var (
	errServerEmpty   = errors.New("Server is empty.")
	errServerBracket = errors.New("Server: write an IPv6 address with a port as [address]:port.")
	errServerPort    = errors.New("Server: port must be 1–65535.")
)

// ParseServer reads "host", "host:port", a bare IPv6 address or
// "[v6]:port". The port defaults to 3389. Whether host is a valid address is
// Build's check.
func ParseServer(s string) (host string, port int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, errServerEmpty
	}
	host, portText, hasPort := s, "", false
	switch {
	case strings.HasPrefix(s, "["):
		end := strings.Index(s, "]")
		if end < 0 {
			return "", 0, errServerBracket
		}
		host = s[1:end]
		rest := s[end+1:]
		if rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return "", 0, errServerBracket
			}
			portText, hasPort = rest[1:], true
		}
	case strings.Count(s, ":") == 1:
		i := strings.Index(s, ":")
		host, portText, hasPort = s[:i], s[i+1:], true
	}
	port = payload.DefaultPort
	if hasPort {
		p, err := strconv.Atoi(portText)
		if err != nil || p < 1 || p > 65535 {
			return "", 0, errServerPort
		}
		port = p
	}
	return host, port, nil
}

// NormalizeCert accepts a pasted fingerprint with colons, spaces or capitals.
func NormalizeCert(s string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if r == ':' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s))
}
