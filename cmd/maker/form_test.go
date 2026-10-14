package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4aykas/fleet-rdp/maker"
)

func TestAddPersonMovesFieldsIntoList(t *testing.T) {
	f := form{Login: " anna ", Password: "p1"}
	if err := f.addPerson(); err != nil {
		t.Fatal(err)
	}
	if f.Login != "" || f.Password != "" || len(f.People) != 1 || f.People[0] != (maker.Person{User: "anna", Password: "p1"}) {
		t.Fatalf("%+v", f)
	}
	f.Login, f.Password = `CORP\ANNA`, "p2"
	if err := f.addPerson(); err == nil || err.Error() != `CORP\ANNA is already in the list.` {
		t.Fatalf("error %v", err)
	}
	f.Login, f.Password = "oleh", ""
	if err := f.addPerson(); err == nil || err.Error() != "Password for oleh is empty." {
		t.Fatalf("error %v", err)
	}
	f.Login = ""
	if err := f.addPerson(); err == nil || err.Error() != "Login is empty." {
		t.Fatalf("error %v", err)
	}
}

func TestRemoveAndClear(t *testing.T) {
	f := form{People: []maker.Person{{User: "a", Password: "1"}, {User: "b", Password: "2"}, {User: "c", Password: "3"}}, Password: "x"}
	f.removePerson(1)
	if len(f.People) != 2 || f.People[1].User != "c" {
		t.Fatalf("%+v", f.People)
	}
	f.removePerson(7) // out of range: nothing happens
	f.clearSecrets()
	if f.Password != "" || f.People != nil {
		t.Fatalf("%+v", f)
	}
}

func TestSpecTakesListAndTypedPerson(t *testing.T) {
	f := form{Server: "[::1]:3390", Login: "oleh", Password: "o1", Name: " Krasna ", Monitors: "one", Cert: "AB:" + strings.Repeat("ab", 31),
		People: []maker.Person{{User: "anna", Password: "a1"}}}
	s, err := f.spec()
	if err != nil {
		t.Fatal(err)
	}
	if s.Host != "::1" || s.Port != 3390 || s.Name != "Krasna" || s.Monitors != "one" || s.CertSha256 != strings.Repeat("ab", 32) {
		t.Fatalf("%+v", s)
	}
	if len(s.People) != 2 || s.People[1].User != "oleh" {
		t.Fatalf("%+v", s.People)
	}
	if len(f.People) != 1 {
		t.Fatal("spec must not change the list")
	}
	if _, err := (&form{Server: "h:99999"}).spec(); err == nil || err.Error() != "Server: port must be 1–65535." {
		t.Fatalf("error %v", err)
	}
}

func TestSuggestedName(t *testing.T) {
	one := maker.Spec{Host: "1.2.3.4", Name: "Krasna", People: []maker.Person{{User: `CORP\anna`, Password: "x"}}}
	if got := suggestedName(one); got != "anna_Krasna.zip" {
		t.Fatalf("got %q", got)
	}
	many := maker.Spec{Host: "1.2.3.4", People: []maker.Person{{User: "a", Password: "x"}, {User: "b", Password: "y"}}}
	if got := suggestedName(many); got != "1.2.3.4.zip" {
		t.Fatalf("got %q", got)
	}
}

func TestOutputPath(t *testing.T) {
	cases := []struct {
		in     string
		people int
		out    string
		exe    bool
		err    string
	}{
		{`C:\x\anna_K.zip`, 1, `C:\x\anna_K.zip`, false, ""},
		{`C:\x\anna_K.EXE`, 1, `C:\x\anna_K.EXE`, true, ""},
		{`C:\x\anna_K`, 1, `C:\x\anna_K.zip`, false, ""},
		{`C:\x\all.exe`, 2, "", false, "Several people go into one ZIP: choose .zip."},
		{`C:\x\notes.txt`, 1, "", false, "Save as .zip or .exe."},
		{`C:\x\notes.txt`, 2, "", false, "Save as .zip."},
	}
	for _, c := range cases {
		out, exe, err := outputPath(c.in, c.people)
		if c.err != "" {
			if err == nil || err.Error() != c.err {
				t.Errorf("%s: error %v", c.in, err)
			}
			continue
		}
		if err != nil || out != c.out || exe != c.exe {
			t.Errorf("%s: %q %v %v", c.in, out, exe, err)
		}
	}
}

func TestMessages(t *testing.T) {
	if got := checkFailed("168.119.80.235", 2157); got != "Cannot reach 168.119.80.235:2157. If this computer uses Cloudflare WARP, turn it on." {
		t.Fatalf("got %q", got)
	}
	if got := checkFailed("::1", 3389); got != "Cannot reach [::1]:3389. If this computer uses Cloudflare WARP, turn it on." {
		t.Fatalf("got %q", got)
	}
	if got := checkOK(); got != "Reachable, certificate pinned." {
		t.Fatalf("got %q", got)
	}
	if got := savedMessage(`C:\x\a.zip`, false); got != `Saved C:\x\a.zip.` {
		t.Fatalf("got %q", got)
	}
	if got := savedMessage(`C:\x\a.zip`, true); got != `Saved C:\x\a.zip with the default icon.` {
		t.Fatalf("got %q", got)
	}
}

func TestCheckTarget(t *testing.T) {
	h, p, err := checkTarget("rdp.example:2157")
	if err != nil || h != "rdp.example" || p != 2157 {
		t.Fatalf("%q %d %v", h, p, err)
	}
	if _, _, err := checkTarget(""); err == nil {
		t.Fatal("want an error")
	}
}

func TestEmbeddedTemplateMissingInPlainCheckout(t *testing.T) {
	b, err := embeddedTemplate()
	if err == nil {
		if len(b) < 2 || string(b[:2]) != "MZ" {
			t.Fatal("embedded template is not an exe")
		}
		t.Skip("built by build.ps1: the template is embedded")
	}
	if err != errNoTemplate {
		t.Fatalf("error %v", err)
	}
}

func TestCheckStale(t *testing.T) {
	if got := checkStale(); got != "Server changed while checking; press Check again." {
		t.Fatalf("got %q", got)
	}
}

func TestOutputPathNoExtensionNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "anna_K")
	if out, _, err := outputPath(base, 1); err != nil || out != base+".zip" {
		t.Fatalf("free name: %q %v", out, err)
	}
	if err := os.WriteFile(base+".zip", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := outputPath(base, 1); err == nil || err.Error() != "Save as .zip or .exe." {
		t.Fatalf("existing file: %v", err)
	}
}
