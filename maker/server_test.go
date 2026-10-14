package maker

import "testing"

func TestParseServer(t *testing.T) {
	ok := []struct {
		in   string
		host string
		port int
	}{
		{"168.119.80.235", "168.119.80.235", 3389},
		{" 168.119.80.235:2157 ", "168.119.80.235", 2157},
		{"rdp.example", "rdp.example", 3389},
		{"rdp.example:443", "rdp.example", 443},
		{"::1", "::1", 3389},
		{"2a01:4f8::1", "2a01:4f8::1", 3389},
		{"[::1]", "::1", 3389},
		{"[2a01:4f8::1]:3390", "2a01:4f8::1", 3390},
	}
	for _, c := range ok {
		h, p, err := ParseServer(c.in)
		if err != nil || h != c.host || p != c.port {
			t.Errorf("ParseServer(%q) = %q, %d, %v", c.in, h, p, err)
		}
	}
	bad := map[string]string{
		"":           "Server is empty.",
		"   ":        "Server is empty.",
		"[::1":       "Server: write an IPv6 address with a port as [address]:port.",
		"[::1]3389":  "Server: write an IPv6 address with a port as [address]:port.",
		"host:0":     "Server: port must be 1–65535.",
		"host:65536": "Server: port must be 1–65535.",
		"host:abc":   "Server: port must be 1–65535.",
		"host:":      "Server: port must be 1–65535.",
	}
	for in, want := range bad {
		if _, _, err := ParseServer(in); err == nil || err.Error() != want {
			t.Errorf("ParseServer(%q) error %v, want %q", in, err, want)
		}
	}
}

func TestNormalizeCert(t *testing.T) {
	if got := NormalizeCert(" 4B:B7 94\t69 "); got != "4bb79469" {
		t.Fatalf("got %q", got)
	}
}
