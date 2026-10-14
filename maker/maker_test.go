package maker

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4aykas/fleet-rdp/payload"
)

const secret = "S3cret-Pass-word!"

var fakeTemplate = []byte("MZ not a real PE, only bytes to append to")

func spec(people ...Person) Spec {
	return Spec{Host: "168.119.80.235", Port: 2157, Name: "Krasna", Monitors: "auto", People: people}
}

func TestBuildOnePerson(t *testing.T) {
	r, err := Build(spec(Person{`CORP\anna`, secret}), fakeTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Files) != 1 || r.Files[0].Name != "anna_Krasna.exe" {
		t.Fatalf("files %+v", r.Files)
	}
	if !r.DefaultIcon {
		t.Fatal("a fake template cannot take an icon: DefaultIcon must be true")
	}
	p, err := payload.Read(r.Files[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	want := payload.Payload{V: 1, Name: "Krasna", Host: "168.119.80.235", Port: 2157, User: `CORP\anna`, Password: secret, Monitors: "auto"}
	if p != want {
		t.Fatalf("payload %+v", p)
	}
	if !bytes.HasPrefix(r.Files[0].Data, fakeTemplate) {
		t.Fatal("the template bytes must come first")
	}
}

func TestBuildSeveralPeopleAndLabelFallback(t *testing.T) {
	s := spec(Person{"anna", "a1"}, Person{"oleh", "o1"})
	s.Name = ""
	s.Port = 0
	s.CertSha256 = strings.Repeat("ab", 32)
	r, err := Build(s, fakeTemplate)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{r.Files[0].Name, r.Files[1].Name}
	if names[0] != "anna_168.119.80.235.exe" || names[1] != "oleh_168.119.80.235.exe" {
		t.Fatalf("names %v", names)
	}
	p, _ := payload.Read(r.Files[1].Data)
	if p.Name != "168.119.80.235" || p.Port != 3389 || p.User != "oleh" || p.Password != "o1" || p.CertSha256 != s.CertSha256 {
		t.Fatalf("payload %+v", p)
	}
}

func TestBuildErrors(t *testing.T) {
	long := strings.Repeat("x", 1281)
	cases := []struct {
		name string
		s    Spec
		want string
	}{
		{"no people", spec(), "Add at least one person."},
		{"bad host", Spec{Host: "bad host", People: []Person{{"anna", secret}}}, "Server: enter an IP address or a DNS name, optionally with :port."},
		{"blank login", spec(Person{"  ", secret}), "Login is empty."},
		{"domain only login", spec(Person{`CORP\`, secret}), "Login is empty."},
		{"same file name", spec(Person{"a:b", secret}, Person{"a?B", secret}), "Two people would get the same file name a_b_Krasna.exe."},
		{"empty password", spec(Person{"anna", ""}), "Password for anna is empty."},
		{"long password", spec(Person{"anna", long}), "Password for anna is longer than 1280 characters."},
		{"duplicate", spec(Person{`CORP\Anna`, secret}, Person{"anna", secret}), "Two people have the login anna."},
		{"cert", func() Spec { s := spec(Person{"anna", secret}); s.CertSha256 = "abc"; return s }(), "Certificate SHA-256 must be 64 hex characters."},
		{"monitors", func() Spec { s := spec(Person{"anna", secret}); s.Monitors = "two"; return s }(), "Monitors must be Auto, One or All."},
	}
	for _, c := range cases {
		_, err := Build(c.s, fakeTemplate)
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: error %v, want %q", c.name, err, c.want)
		}
		if err != nil && (strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), long)) {
			t.Errorf("%s: the error contains the password", c.name)
		}
	}
}

func TestBuildRefusesTemplateWithPayloadWithoutPassword(t *testing.T) {
	r, err := Build(spec(Person{"anna", secret}), fakeTemplate)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Build(spec(Person{"anna", secret}), r.Files[0].Data)
	if err == nil || !strings.HasPrefix(err.Error(), "Launcher for anna: ") || strings.Contains(err.Error(), secret) {
		t.Fatalf("error %v", err)
	}
}

func TestBuildPatchesTheRealTemplateIcon(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join("..", "dist", "fleet-rdp-launcher.exe"))
	if err != nil {
		t.Skip("no dist/fleet-rdp-launcher.exe; run build.ps1")
	}
	r, err := Build(spec(Person{"anna", secret}), tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if r.DefaultIcon {
		t.Fatal("the real template must take the icon")
	}
	if bytes.Equal(r.Files[0].Data[:len(tmpl)], tmpl) {
		t.Fatal("the icon bytes did not change")
	}
	if len(r.Files[0].Data) <= len(tmpl) {
		t.Fatal("no payload appended")
	}
}

func TestBuildDoesNotModifyTheTemplate(t *testing.T) {
	tmpl := bytes.Clone(fakeTemplate)
	if _, err := Build(spec(Person{"anna", secret}), tmpl); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tmpl, fakeTemplate) {
		t.Fatal("Build changed the caller's template")
	}
}

func TestZip(t *testing.T) {
	files := []File{{"anna_Krasna.exe", []byte("one")}, {"oleh_Krasna.exe", []byte("two")}}
	b, err := Zip(files)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("%d entries", len(zr.File))
	}
	for i, f := range zr.File {
		rc, _ := f.Open()
		got, _ := io.ReadAll(rc)
		rc.Close()
		if f.Name != files[i].Name || string(got) != string(files[i].Data) {
			t.Fatalf("entry %d: %s %q", i, f.Name, got)
		}
	}
}
