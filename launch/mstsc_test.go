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
		count int
		mode  string
		flags []string
		want  bool
	}{
		{2, "", nil, true},
		{1, "", nil, false},
		{0, "", nil, false},
		{2, "auto", nil, true},
		{2, "one", nil, false},
		{1, "all", nil, true},
		{2, "one", []string{"--multi"}, true},
		{1, "all", []string{"--single"}, false},
		{2, "all", []string{"--multi", "--single"}, false},
		{2, "one", []string{"--bogus"}, false},
		{2, "", []string{"--bogus"}, true},
		{1, "all", []string{"--bogus", "--single"}, false},
	}
	for _, c := range cases {
		if got := ChooseMulti(c.count, c.mode, c.flags); got != c.want {
			t.Errorf("ChooseMulti(%d, %q, %v) = %v, want %v", c.count, c.mode, c.flags, got, c.want)
		}
	}
}
