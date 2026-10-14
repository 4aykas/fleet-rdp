// Package maker builds launchers from the template without any UI: the logic
// behind fleet-rdp-maker.exe. Its errors are full sentences the window shows
// as they are, and they never contain a password.
package maker

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/4aykas/fleet-rdp/invader"
	"github.com/4aykas/fleet-rdp/payload"
	"github.com/4aykas/fleet-rdp/peicon"
)

type Person struct{ User, Password string }

// Spec is one server and the people who get a launcher for it.
type Spec struct {
	Host       string
	Port       int // 0 means 3389
	Name       string
	Monitors   string // "", "auto", "one" or "all"
	CertSha256 string // empty: the launcher trusts the certificate on first use
	People     []Person
}

// Label is what the launcher's dialogs and file names call the server.
func (s Spec) Label() string {
	if n := strings.TrimSpace(s.Name); n != "" {
		return n
	}
	return s.Host
}

type File struct {
	Name string
	Data []byte
}

type Result struct {
	Files []File
	// DefaultIcon: the template took no icon, so the launchers keep the
	// template's own.
	DefaultIcon bool
}

const maxPasswordUnits = 1280

// Build returns one launcher per person: the template with this server's
// invader icon (the dashboard's icon for a server with no organization) and
// that person's payload appended. The template is not modified.
func Build(s Spec, template []byte) (Result, error) {
	if s.CertSha256 != "" {
		if sum, err := hex.DecodeString(s.CertSha256); err != nil || len(sum) != 32 {
			return Result{}, errors.New("Certificate SHA-256 must be 64 hex characters.")
		}
	}
	switch s.Monitors {
	case "", payload.MonitorsAuto, payload.MonitorsOne, payload.MonitorsAll:
	default:
		return Result{}, errors.New("Monitors must be Auto, One or All.")
	}
	probe := payload.Payload{V: payload.Version, Name: "x", Host: s.Host, Port: s.Port, User: "x", Password: "x"}
	if err := probe.Validate(); err != nil {
		return Result{}, errors.New("Server: enter an IP address or a DNS name, optionally with :port.")
	}
	if len(s.People) == 0 {
		return Result{}, errors.New("Add at least one person.")
	}
	for i, p := range s.People {
		if strings.TrimSpace(bare(strings.TrimSpace(p.User))) == "" {
			return Result{}, errors.New("Login is empty.")
		}
		if p.Password == "" {
			return Result{}, fmt.Errorf("Password for %s is empty.", p.User)
		}
		if len(utf16.Encode([]rune(p.Password))) > maxPasswordUnits {
			return Result{}, fmt.Errorf("Password for %s is longer than 1280 characters.", p.User)
		}
		for _, q := range s.People[:i] {
			if SameLogin(p.User, q.User) {
				return Result{}, fmt.Errorf("Two people have the login %s.", strings.TrimSpace(bare(p.User)))
			}
		}
	}

	tmpl := bytes.Clone(template)
	g := invader.Grid(s.Host)
	colour := invader.ColourFor(s.Host)
	n := peicon.SetIcons(tmpl, func(size int) []byte {
		return invader.Render(g, colour, invader.NoOrgTile, size)
	})

	label := s.Label()
	res := Result{DefaultIcon: n == 0}
	for _, p := range s.People {
		user := strings.TrimSpace(p.User)
		data, err := payload.Append(tmpl, payload.Payload{
			V: payload.Version, Name: label, Host: s.Host, Port: s.Port,
			User: user, Password: p.Password, CertSha256: s.CertSha256, Monitors: s.Monitors,
		})
		if err != nil {
			// payload errors never quote the password.
			return Result{}, fmt.Errorf("Launcher for %s: %w", user, err)
		}
		name := FileBase(user, label) + ".exe"
		for _, f := range res.Files {
			if strings.EqualFold(f.Name, name) {
				return Result{}, fmt.Errorf("Two people would get the same file name %s.", f.Name)
			}
		}
		res.Files = append(res.Files, File{Name: name, Data: data})
	}
	return res, nil
}

// Zip puts every file into one ZIP archive.
func Zip(files []File) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
