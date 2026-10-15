package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/4aykas/fleet-rdp/cert"
	"github.com/4aykas/fleet-rdp/maker"
	"github.com/4aykas/fleet-rdp/payload"
)

// executable and fetchCert are replaced in tests.
var (
	executable = os.Executable
	fetchCert  = cert.Fetch
)

const usage = `usage:
  fleet-rdp build --host H [--port P] --user U [--name N] [--cert-sha256 X] [--template PATH] -o OUT.exe
  fleet-rdp inspect FILE.exe
  fleet-rdp probe HOST PORT

The password is read from FLEET_RDP_PASSWORD or, failing that, from stdin
(a hidden prompt on a terminal). It is never an argument.
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, env func(string) string) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "build":
		return runBuild(args[1:], stdin, stdout, stderr, env)
	case "inspect":
		return runInspect(args[1:], stdout, stderr)
	case "probe":
		return runProbe(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "fleet-rdp: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runBuild(args []string, stdin io.Reader, stdout, stderr io.Writer, env func(string) string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	host := fs.String("host", "", "server address (IPv4, IPv6 or DNS name)")
	port := fs.Int("port", payload.DefaultPort, "RDP port")
	user := fs.String("user", "", `login, optionally DOMAIN\login`)
	name := fs.String("name", "", "label for dialog titles (default: the host)")
	pin := fs.String("cert-sha256", "", "pin the server certificate (64 hex characters) instead of trusting it on first use")
	monitors := fs.String("monitors", payload.MonitorsAuto, "which screens mstsc uses: auto (all when there is more than one), one or all")
	tmpl := fs.String("template", "", "launcher template (default: fleet-rdp-launcher.exe beside this program)")
	out := fs.String("o", "", "output file")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "fleet-rdp build: unexpected argument %q\n\n%s", fs.Arg(0), usage)
		return 2
	}
	if *host == "" || *user == "" || *out == "" {
		fmt.Fprintln(stderr, "fleet-rdp build: --host, --user and -o are required")
		return 2
	}

	tmplPath := *tmpl
	if tmplPath == "" {
		exe, err := executable()
		if err != nil {
			fmt.Fprintln(stderr, "fleet-rdp build: cannot locate the default template:", err)
			return 1
		}
		tmplPath = filepath.Join(filepath.Dir(exe), "fleet-rdp-launcher.exe")
	}
	// Writing the output over the template would destroy it.
	if oi, err := os.Stat(*out); err == nil {
		if ti, err := os.Stat(tmplPath); err == nil && os.SameFile(oi, ti) {
			fmt.Fprintln(stderr, "fleet-rdp build: the output file is the template")
			return 1
		}
	}
	template, err := os.ReadFile(tmplPath)
	if err != nil {
		fmt.Fprintln(stderr, "fleet-rdp build: cannot read the template:", err)
		return 1
	}

	if fi, err := os.Stat(filepath.Dir(*out)); err != nil || !fi.IsDir() {
		fmt.Fprintln(stderr, "fleet-rdp build: the output directory does not exist")
		return 1
	}

	label := *name
	if label == "" {
		label = *host
	}
	mon := *monitors
	if mon == payload.MonitorsAuto {
		mon = "" // auto is the default; keep it off the wire
	}
	p := payload.Payload{
		V: payload.Version, Name: label, Host: *host, Port: *port,
		User: *user, Password: "x", CertSha256: strings.ToLower(*pin),
		Monitors: mon,
	}
	// Everything but the password is checked before the prompt, so a typo in
	// a flag does not cost a typed password. (A placeholder stands in for it.)
	if _, err := payload.Append(template, p); err != nil {
		fmt.Fprintln(stderr, "fleet-rdp build:", err)
		return 1
	}

	password, err := readPassword(stdin, stderr, env)
	if err != nil {
		fmt.Fprintln(stderr, "fleet-rdp build:", err)
		return 1
	}
	if password == "" {
		fmt.Fprintln(stderr, "fleet-rdp build: the password is empty")
		return 1
	}
	p.Password = password
	built, err := payload.Append(template, p)
	if err != nil {
		fmt.Fprintln(stderr, "fleet-rdp build:", err)
		return 1
	}
	if err := maker.WriteAtomic(*out, built); err != nil {
		fmt.Fprintln(stderr, "fleet-rdp build: cannot write the output:", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", *out, len(built))
	return 0
}

// runProbe prints the SHA-256 of the certificate a server shows on its RDP
// port, and how long the exchange took.
func runProbe(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) != 2 || args[0] == "" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	port, err := strconv.Atoi(args[1])
	if err != nil || port < 1 || port > 65535 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	start := time.Now()
	der, err := fetchCert(context.Background(), args[0], port)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintln(stderr, "fleet-rdp probe:", err)
		return 1
	}
	sum := sha256.Sum256(der)
	fmt.Fprintf(stdout, "%s  %s\n", hex.EncodeToString(sum[:]), elapsed.Round(time.Millisecond))
	return 0
}

func runInspect(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	exe, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "fleet-rdp inspect:", err)
		return 1
	}
	p, err := payload.Read(exe)
	if err != nil {
		fmt.Fprintln(stderr, "fleet-rdp inspect:", err)
		return 1
	}
	p.Password = "***"
	js, _ := json.MarshalIndent(p, "", "  ")
	fmt.Fprintln(stdout, string(js))
	return 0
}

// readPassword: FLEET_RDP_PASSWORD, else a hidden prompt on a terminal, else
// one line of stdin.
func readPassword(stdin io.Reader, stderr io.Writer, env func(string) string) (string, error) {
	if v := env("FLEET_RDP_PASSWORD"); v != "" {
		return v, nil
	}
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(stderr, "Password: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", errors.New("no password on stdin and FLEET_RDP_PASSWORD is not set")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
