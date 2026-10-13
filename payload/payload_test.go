package payload

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/*.tail from testdata/*.json")

func good() Payload {
	return Payload{V: 1, Name: "SGCLOUD", Host: "10.0.0.5", Port: 2157, User: "vlad", Password: "Dummy-Pass-1!"}
}

// rawTail builds a tail by hand so the tests do not depend on Append.
func rawTail(js string, declaredLen uint32) []byte {
	out := []byte(js)
	out = binary.LittleEndian.AppendUint32(out, declaredLen)
	return append(out, Magic...)
}

func TestRoundTrip(t *testing.T) {
	template := []byte("MZ fake template bytes")
	p := good()
	p.CertSha256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	exe, err := Append(template, p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(exe, template) {
		t.Fatal("template bytes were changed")
	}
	got, err := Read(exe)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("got %+v, want %+v", got, p)
	}
}

func TestAppendRefusesATemplateThatHasAPayload(t *testing.T) {
	exe, err := Append([]byte("MZ"), good())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Append(exe, good()); !errors.Is(err, ErrHasPayload) {
		t.Fatalf("got %v, want ErrHasPayload", err)
	}
}

func TestReadWithoutPayload(t *testing.T) {
	for name, exe := range map[string][]byte{
		"empty":     nil,
		"short":     []byte("FLTRDP01"),
		"bad magic": append(bytes.Repeat([]byte{0}, 32), []byte("FLTRDP02")...),
		"plain exe": []byte("MZ this is just a template"),
	} {
		if _, err := Read(exe); !errors.Is(err, ErrNoPayload) {
			t.Errorf("%s: got %v, want ErrNoPayload", name, err)
		}
	}
}

func TestReadCorruptLength(t *testing.T) {
	js := `{"v":1,"host":"h","user":"u","password":"p"}`
	for name, declared := range map[string]uint32{
		"over 64 KiB":   MaxJSON + 1,
		"over the file": uint32(len(js)) + 1,
	} {
		if _, err := Read(rawTail(js, declared)); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: got %v, want ErrCorrupt", name, err)
		}
	}
	if _, err := Read(rawTail("not json at all", 15)); !errors.Is(err, ErrCorrupt) {
		t.Errorf("bad JSON: got %v, want ErrCorrupt", err)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]func(*Payload){
		"host empty":     func(p *Payload) { p.Host = "  " },
		"user empty":     func(p *Payload) { p.User = "" },
		"port too big":   func(p *Payload) { p.Port = 70000 },
		"port negative":  func(p *Payload) { p.Port = -1 },
		"cert too short": func(p *Payload) { p.CertSha256 = "abcd" },
		"cert not hex":   func(p *Payload) { p.CertSha256 = string(bytes.Repeat([]byte("z"), 64)) },
	}
	for name, mutate := range cases {
		p := good()
		mutate(&p)
		if err := p.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
		if _, err := Append(nil, p); err == nil {
			t.Errorf("%s: Append accepted it", name)
		}
	}
}

func TestVersionMustBeOne(t *testing.T) {
	p := good()
	p.V = 2
	if _, err := Append(nil, p); !errors.Is(err, ErrVersion) {
		t.Fatalf("Append: got %v, want ErrVersion", err)
	}
	js := `{"v":2,"host":"h","user":"u","password":"p"}`
	if _, err := Read(rawTail(js, uint32(len(js)))); !errors.Is(err, ErrVersion) {
		t.Fatalf("Read: got %v, want ErrVersion", err)
	}
}

func TestReadDefaultsThePortAndIgnoresUnknownFields(t *testing.T) {
	js := `{"v":1,"host":"h","user":"u","password":"p","icon":"later"}`
	p, err := Read(rawTail(js, uint32(len(js))))
	if err != nil {
		t.Fatal(err)
	}
	if p.Port != DefaultPort {
		t.Fatalf("port = %d, want %d", p.Port, DefaultPort)
	}
}

// The vector is shared with the dashboard's TypeScript writer. The expected
// tail is built here from the JSON file alone, not with Append, so the test
// also proves that Append produces the JSON the file holds (field order,
// no HTML escaping).
func TestSharedVector(t *testing.T) {
	for _, name := range []string{"payload-v1", "payload-v1-nocert", "payload-v1-monitors"} {
		t.Run(name, func(t *testing.T) {
			js, err := os.ReadFile(filepath.Join("..", "testdata", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			want := rawTail(string(js), uint32(len(js)))
			tailPath := filepath.Join("..", "testdata", name+".tail")
			if *update {
				if err := os.WriteFile(tailPath, want, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			tail, err := os.ReadFile(tailPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(tail, want) {
				t.Fatalf("%s.tail does not match %s.json; run: go test ./payload -run TestSharedVector -update", name, name)
			}

			var p Payload
			if err := json.Unmarshal(js, &p); err != nil {
				t.Fatal(err)
			}
			// Go -> bytes
			got, err := Append(nil, p)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tail) {
				t.Fatalf("Append produced\n%q\nwant\n%q", got, tail)
			}
			// bytes -> Go
			back, err := Read(tail)
			if err != nil {
				t.Fatal(err)
			}
			if back != p {
				t.Fatalf("Read gave %+v, want %+v", back, p)
			}
		})
	}
}

func TestValidateHost(t *testing.T) {
	valid := []string{"10.0.0.5", "::1", "2001:db8::1", "[2001:db8::1]", "[::1]", "rdp.example.com", "WIN-UF5UMK2KNVD", "a", "a-b.c1"}
	invalid := []string{"10.0.0.5:2157", "host name", " 10.0.0.5", `a\b`, "a\x08b", "a/b", "[10.0.0.5]", "-bad.example", "bad-.example", "a..b", "", "   ", "[::1", "::1]", "host:3389", strings.Repeat("a", 254)}
	for _, h := range valid {
		p := good()
		p.Host = h
		if err := p.Validate(); err != nil {
			t.Errorf("host %q: %v", h, err)
		}
	}
	for _, h := range invalid {
		p := good()
		p.Host = h
		err := p.Validate()
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("host %q: got %v, want ErrInvalid", h, err)
			continue
		}
		if strings.Contains(err.Error(), p.Password) {
			t.Errorf("host %q: error holds the password", h)
		}
	}
	p := good()
	p.Host = "10.0.0.5:2157"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "port field") {
		t.Errorf("host with port: error %v should point to the port field", err)
	}
}

func TestValidatePassword(t *testing.T) {
	p := good()
	p.Password = ""
	if err := p.Validate(); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "password is empty") {
		t.Errorf("empty: got %v", err)
	}
	p.Password = strings.Repeat("a", 1280)
	if err := p.Validate(); err != nil {
		t.Errorf("1280 units: %v", err)
	}
	p.Password = strings.Repeat("a", 1281)
	err := p.Validate()
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "1280") {
		t.Errorf("1281 units: got %v", err)
	}
	// An astral rune is two UTF-16 code units.
	p.Password = strings.Repeat("a", 1279) + "\U0001F600"
	if err := p.Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("surrogate pair over the limit: got %v", err)
	}
}

func TestAppendPortZeroWritesDefault(t *testing.T) {
	p := good()
	p.Port = 0
	exe, err := Append([]byte("MZ"), p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(exe, []byte(`"port":3389`)) || bytes.Contains(exe, []byte(`"port":0`)) {
		t.Errorf("JSON does not hold port 3389: %s", exe)
	}
	got, err := Read(exe)
	if err != nil || got.Port != DefaultPort {
		t.Errorf("Read: %v, port %d", err, got.Port)
	}
}

func TestValidateMonitors(t *testing.T) {
	base := Payload{V: Version, Name: "n", Host: "10.0.0.5", Port: 3389, User: "u", Password: "p"}
	for _, m := range []string{"", MonitorsAuto, MonitorsOne, MonitorsAll} {
		p := base
		p.Monitors = m
		if err := p.Validate(); err != nil {
			t.Errorf("monitors %q: %v", m, err)
		}
	}
	for _, m := range []string{"two", "ALL", " one"} {
		p := base
		p.Monitors = m
		if err := p.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("monitors %q: err = %v, want ErrInvalid", m, err)
		}
	}
}

func TestOldVectorHasNoMonitors(t *testing.T) {
	js, err := os.ReadFile(filepath.Join("..", "testdata", "payload-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p Payload
	if err := json.Unmarshal(js, &p); err != nil {
		t.Fatal(err)
	}
	if p.Monitors != "" {
		t.Fatalf("Monitors = %q, want empty (auto)", p.Monitors)
	}
}
