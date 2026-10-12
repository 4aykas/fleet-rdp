package launch

import (
	"reflect"
	"testing"
)

func TestMstscArgs(t *testing.T) {
	cases := []struct {
		name  string
		host  string
		port  int
		multi bool
		want  []string
	}{
		{"ipv4", "10.0.0.5", 2157, false, []string{"/v:10.0.0.5:2157", "/f"}},
		{"dns name", "srv.example.com", 3389, false, []string{"/v:srv.example.com:3389", "/f"}},
		{"multi", "10.0.0.5", 3389, true, []string{"/v:10.0.0.5:3389", "/f", "/multimon"}},
		{"ipv6 bare", "fd00::1", 2157, false, []string{"/v:[fd00::1]:2157", "/f"}},
		{"ipv6 already bracketed", "[fd00::1]", 2157, false, []string{"/v:[fd00::1]:2157", "/f"}},
	}
	for _, c := range cases {
		if got := MstscArgs(c.host, c.port, c.multi); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBareHost(t *testing.T) {
	for in, want := range map[string]string{"10.0.0.5": "10.0.0.5", "[fd00::1]": "fd00::1", "fd00::1": "fd00::1"} {
		if got := BareHost(in); got != want {
			t.Errorf("BareHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChooseMulti(t *testing.T) {
	cases := []struct {
		name     string
		monitors int
		flags    []string
		want     bool
	}{
		{"one monitor", 1, nil, false},
		{"two monitors", 2, nil, true},
		{"detection failed", 0, nil, false},
		{"--single wins over two monitors", 2, []string{"--single"}, false},
		{"--multi wins over one monitor", 1, []string{"--multi"}, true},
		{"last flag wins", 1, []string{"--multi", "--single"}, false},
		{"unknown flags are ignored", 2, []string{"--whatever"}, true},
	}
	for _, c := range cases {
		if got := ChooseMulti(c.monitors, c.flags); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
