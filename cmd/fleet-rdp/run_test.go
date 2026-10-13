package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/4aykas/fleet-rdp/payload"
)

const secret = "Dummy-Pass-1!"

func noEnv(string) string { return "" }

func envWith(k, v string) func(string) string {
	return func(key string) string {
		if key == k {
			return v
		}
		return ""
	}
}

func writeTemplate(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "template.exe")
	if err := os.WriteFile(path, []byte("MZ fake template"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func call(args []string, stdin string, env func(string) string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errb, env)
	return code, out.String(), errb.String()
}

func TestBuildThenInspect(t *testing.T) {
	tpl := writeTemplate(t)
	out := filepath.Join(t.TempDir(), "SGCLOUD.exe")

	code, stdout, stderr := call(
		[]string{"build", "--host", "10.0.0.5", "--port", "2157", "--user", `CORP\vlad`, "--name", "SGCLOUD", "--template", tpl, "-o", out},
		"", envWith("FLEET_RDP_PASSWORD", secret))
	if code != 0 {
		t.Fatalf("build exited %d: %s", code, stderr)
	}
	if strings.Contains(stdout+stderr, secret) {
		t.Fatal("build printed the password")
	}

	exe, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	p, err := payload.Read(exe)
	if err != nil {
		t.Fatal(err)
	}
	want := payload.Payload{V: 1, Name: "SGCLOUD", Host: "10.0.0.5", Port: 2157, User: `CORP\vlad`, Password: secret}
	if p != want {
		t.Fatalf("payload = %+v, want %+v", p, want)
	}

	code, stdout, stderr = call([]string{"inspect", out}, "", noEnv)
	if code != 0 {
		t.Fatalf("inspect exited %d: %s", code, stderr)
	}
	if strings.Contains(stdout, secret) || !strings.Contains(stdout, "***") {
		t.Fatalf("inspect must mask the password:\n%s", stdout)
	}
	if !strings.Contains(stdout, "10.0.0.5") || !strings.Contains(stdout, "SGCLOUD") {
		t.Fatalf("inspect lost the connection details:\n%s", stdout)
	}
}

func TestBuildDefaultsPortAndName(t *testing.T) {
	tpl := writeTemplate(t)
	out := filepath.Join(t.TempDir(), "x.exe")
	code, _, stderr := call([]string{"build", "--host", "srv.example.com", "--user", "vlad", "--template", tpl, "-o", out}, "", envWith("FLEET_RDP_PASSWORD", secret))
	if code != 0 {
		t.Fatal(stderr)
	}
	exe, _ := os.ReadFile(out)
	p, err := payload.Read(exe)
	if err != nil {
		t.Fatal(err)
	}
	if p.Port != 3389 || p.Name != "srv.example.com" {
		t.Fatalf("port=%d name=%q", p.Port, p.Name)
	}
}

func TestBuildReadsThePasswordFromStdinWhenThereIsNoEnv(t *testing.T) {
	tpl := writeTemplate(t)
	out := filepath.Join(t.TempDir(), "x.exe")
	code, _, stderr := call([]string{"build", "--host", "h", "--user", "u", "--template", tpl, "-o", out}, secret+"\r\nignored\n", noEnv)
	if code != 0 {
		t.Fatal(stderr)
	}
	exe, _ := os.ReadFile(out)
	p, _ := payload.Read(exe)
	if p.Password != secret {
		t.Fatalf("password = %q", p.Password)
	}
}

func TestBuildPinsTheCertificate(t *testing.T) {
	tpl := writeTemplate(t)
	out := filepath.Join(t.TempDir(), "x.exe")
	pin := "0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef"
	code, _, stderr := call([]string{"build", "--host", "h", "--user", "u", "--cert-sha256", pin, "--template", tpl, "-o", out}, "", envWith("FLEET_RDP_PASSWORD", secret))
	if code != 0 {
		t.Fatal(stderr)
	}
	exe, _ := os.ReadFile(out)
	p, _ := payload.Read(exe)
	if p.CertSha256 != strings.ToLower(pin) {
		t.Fatalf("certSha256 = %q", p.CertSha256)
	}
}

func TestBuildFailures(t *testing.T) {
	tpl := writeTemplate(t)
	dir := t.TempDir()
	withPayload := filepath.Join(dir, "has-payload.exe")
	built, _ := payload.Append([]byte("MZ"), payload.Payload{V: 1, Host: "h", User: "u", Password: "p"})
	os.WriteFile(withPayload, built, 0o644)
	env := envWith("FLEET_RDP_PASSWORD", secret)

	cases := []struct {
		name string
		args []string
		in   string
		env  func(string) string
		code int
	}{
		{"missing host", []string{"build", "--user", "u", "--template", tpl, "-o", filepath.Join(dir, "a.exe")}, "", env, 2},
		{"missing user", []string{"build", "--host", "h", "--template", tpl, "-o", filepath.Join(dir, "a.exe")}, "", env, 2},
		{"missing -o", []string{"build", "--host", "h", "--user", "u", "--template", tpl}, "", env, 2},
		{"empty password", []string{"build", "--host", "h", "--user", "u", "--template", tpl, "-o", filepath.Join(dir, "a.exe")}, "\n", noEnv, 1},
		{"no password at all", []string{"build", "--host", "h", "--user", "u", "--template", tpl, "-o", filepath.Join(dir, "a.exe")}, "", noEnv, 1},
		{"bad port", []string{"build", "--host", "h", "--port", "70000", "--user", "u", "--template", tpl, "-o", filepath.Join(dir, "a.exe")}, "", env, 1},
		{"bad cert", []string{"build", "--host", "h", "--user", "u", "--cert-sha256", "nope", "--template", tpl, "-o", filepath.Join(dir, "a.exe")}, "", env, 1},
		{"template missing", []string{"build", "--host", "h", "--user", "u", "--template", filepath.Join(dir, "none.exe"), "-o", filepath.Join(dir, "a.exe")}, "", env, 1},
		{"template already has a payload", []string{"build", "--host", "h", "--user", "u", "--template", withPayload, "-o", filepath.Join(dir, "a.exe")}, "", env, 1},
		{"output is the template", []string{"build", "--host", "h", "--user", "u", "--template", tpl, "-o", tpl}, "", env, 1},
	}
	for _, c := range cases {
		code, stdout, stderr := call(c.args, c.in, c.env)
		if code != c.code {
			t.Errorf("%s: exit %d, want %d (%s)", c.name, code, c.code, stderr)
		}
		if strings.Contains(stdout+stderr, secret) {
			t.Errorf("%s: the password was printed", c.name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "a.exe")); err == nil {
		t.Error("a failed build left an output file behind")
	}
	if data, _ := os.ReadFile(tpl); string(data) != "MZ fake template" {
		t.Error("the template was modified")
	}
}

func TestBuildDefaultTemplateIsBesideTheCLI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fleet-rdp-launcher.exe"), []byte("MZ beside"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := executable
	executable = func() (string, error) { return filepath.Join(dir, "fleet-rdp.exe"), nil }
	t.Cleanup(func() { executable = old })

	out := filepath.Join(t.TempDir(), "x.exe")
	code, _, stderr := call([]string{"build", "--host", "h", "--user", "u", "-o", out}, "", envWith("FLEET_RDP_PASSWORD", secret))
	if code != 0 {
		t.Fatal(stderr)
	}
	exe, _ := os.ReadFile(out)
	if !bytes.HasPrefix(exe, []byte("MZ beside")) {
		t.Fatal("did not use the template beside the CLI")
	}
}

func TestInspectErrors(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "plain.exe")
	os.WriteFile(plain, []byte("MZ"), 0o644)
	if code, _, _ := call([]string{"inspect", plain}, "", noEnv); code != 1 {
		t.Errorf("inspect of a file without a payload exited %d", code)
	}
	if code, _, _ := call([]string{"inspect"}, "", noEnv); code != 2 {
		t.Errorf("inspect without a file exited %d", code)
	}
}

func TestUsage(t *testing.T) {
	if code, _, _ := call(nil, "", noEnv); code != 2 {
		t.Errorf("no arguments exited %d", code)
	}
	if code, _, _ := call([]string{"frobnicate"}, "", noEnv); code != 2 {
		t.Errorf("unknown command exited %d", code)
	}
	if code, out, _ := call([]string{"help"}, "", noEnv); code != 0 || !strings.Contains(out, "usage") {
		t.Errorf("help: exit %d, output %q", code, out)
	}
}

func TestBuildRejectsStrayArguments(t *testing.T) {
	tpl := writeTemplate(t)
	out := filepath.Join(t.TempDir(), "x.exe")
	code, _, stderr := call([]string{"build", "--host", "h", "--user", "u", "--template", tpl, "-o", out, "stray"}, "", envWith("FLEET_RDP_PASSWORD", secret))
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr, `"stray"`) || !strings.Contains(stderr, "usage") {
		t.Fatalf("want the argument and the usage, got %q", stderr)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("an output file was written")
	}
}

// failingReader fails the test if anything asks it for the password.
type failingReader struct{ t *testing.T }

func (r failingReader) Read([]byte) (int, error) {
	r.t.Error("the password was requested")
	return 0, io.EOF
}

func TestBuildValidatesBeforeAskingForThePassword(t *testing.T) {
	tpl := writeTemplate(t)
	dir := t.TempDir()
	withPayload := filepath.Join(dir, "has-payload.exe")
	built, _ := payload.Append([]byte("MZ"), payload.Payload{V: 1, Host: "h", Port: 3389, User: "u", Password: "p"})
	os.WriteFile(withPayload, built, 0o644)
	out := filepath.Join(dir, "a.exe")

	cases := map[string][]string{
		"bad port":         {"build", "--host", "h", "--port", "70000", "--user", "u", "--template", tpl, "-o", out},
		"bad host":         {"build", "--host", "h:3389", "--user", "u", "--template", tpl, "-o", out},
		"bad cert":         {"build", "--host", "h", "--user", "u", "--cert-sha256", "nope", "--template", tpl, "-o", out},
		"missing template": {"build", "--host", "h", "--user", "u", "--template", filepath.Join(dir, "none.exe"), "-o", out},
		"has a payload":    {"build", "--host", "h", "--user", "u", "--template", withPayload, "-o", out},
		"no output dir":    {"build", "--host", "h", "--user", "u", "--template", tpl, "-o", filepath.Join(dir, "missing", "a.exe")},
	}
	for name, args := range cases {
		var o, e bytes.Buffer
		if code := run(args, failingReader{t}, &o, &e, noEnv); code != 1 {
			t.Errorf("%s: exit %d, want 1 (%s)", name, code, e.String())
		}
	}
}

func TestBuildOutputIsReplacedAtomically(t *testing.T) {
	tpl := writeTemplate(t)
	dir := t.TempDir()
	env := envWith("FLEET_RDP_PASSWORD", secret)

	t.Run("an existing file is replaced, no temp file stays", func(t *testing.T) {
		out := filepath.Join(dir, "old.exe")
		os.WriteFile(out, []byte("old contents"), 0o644)
		if code, _, stderr := call([]string{"build", "--host", "h", "--user", "u", "--template", tpl, "-o", out}, "", env); code != 0 {
			t.Fatal(stderr)
		}
		exe, _ := os.ReadFile(out)
		if _, err := payload.Read(exe); err != nil {
			t.Fatalf("output is not a launcher: %v", err)
		}
		if runtime.GOOS != "windows" {
			if fi, _ := os.Stat(out); fi.Mode().Perm() != 0o600 {
				t.Errorf("mode %v, want 0600", fi.Mode().Perm())
			}
		}
		left, _ := filepath.Glob(filepath.Join(dir, ".fleet-rdp-*"))
		if len(left) != 0 {
			t.Errorf("temp files left behind: %v", left)
		}
	})

	t.Run("a failed rename leaves the target alone and no temp file", func(t *testing.T) {
		// A directory cannot be replaced by a file: the rename fails.
		out := filepath.Join(dir, "adir")
		if err := os.Mkdir(out, 0o755); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := call([]string{"build", "--host", "h", "--user", "u", "--template", tpl, "-o", out}, "", env)
		if code != 1 || !strings.Contains(stderr, "cannot write the output") {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if fi, err := os.Stat(out); err != nil || !fi.IsDir() {
			t.Error("the target was damaged")
		}
		left, _ := filepath.Glob(filepath.Join(dir, ".fleet-rdp-*"))
		if len(left) != 0 {
			t.Errorf("temp files left behind: %v", left)
		}
	})
}

func TestInspectHelpPrintsUsage(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		code, out, errOut := call([]string{"inspect", flag}, "", noEnv)
		if code != 0 || !strings.Contains(out, "usage") || errOut != "" {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", flag, code, out, errOut)
		}
	}
}
