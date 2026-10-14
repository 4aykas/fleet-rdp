package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/4aykas/fleet-rdp/maker"
)

// form is the window's state with no Win32 in it, so it can be tested. The
// window copies its controls into a form, calls these methods and shows the
// results; it holds no other logic.
type form struct {
	Server, Login, Password, Name, Monitors, Cert string
	People                                        []maker.Person
}

func (f *form) addPerson() error {
	login := strings.TrimSpace(f.Login)
	if login == "" {
		return errors.New("Login is empty.")
	}
	if f.Password == "" {
		return fmt.Errorf("Password for %s is empty.", login)
	}
	for _, p := range f.People {
		if maker.SameLogin(p.User, login) {
			return fmt.Errorf("%s is already in the list.", login)
		}
	}
	f.People = append(f.People, maker.Person{User: login, Password: f.Password})
	f.Login, f.Password = "", ""
	return nil
}

func (f *form) removePerson(i int) {
	if i >= 0 && i < len(f.People) {
		f.People = slices.Delete(f.People, i, i+1)
	}
}

// clearSecrets drops every password the form holds, after a save.
func (f *form) clearSecrets() {
	f.Password = ""
	f.People = nil
}

// spec is the list plus whatever person is still typed in the fields.
func (f *form) spec() (maker.Spec, error) {
	host, port, err := maker.ParseServer(f.Server)
	if err != nil {
		return maker.Spec{}, err
	}
	people := slices.Clone(f.People)
	if strings.TrimSpace(f.Login) != "" || f.Password != "" {
		people = append(people, maker.Person{User: strings.TrimSpace(f.Login), Password: f.Password})
	}
	return maker.Spec{
		Host: host, Port: port, Name: strings.TrimSpace(f.Name), Monitors: f.Monitors,
		CertSha256: maker.NormalizeCert(f.Cert), People: people,
	}, nil
}

// suggestedName: login_Label.zip for one person, Label.zip for several.
func suggestedName(s maker.Spec) string {
	if len(s.People) == 1 {
		return maker.FileBase(s.People[0].User, s.Label()) + ".zip"
	}
	return maker.SafeName(s.Label()) + ".zip"
}

// outputPath decides the format by the extension the Save As dialog returned:
// .exe writes one bare launcher (one person only), .zip a ZIP. Any other
// extension is refused, as appending ".zip" would overwrite a file the Save As
// dialog never asked about; a name with no extension gets ".zip" unless that
// file already exists.
func outputPath(path string, people int) (string, bool, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe":
		if people != 1 {
			return "", false, errors.New("Several people go into one ZIP: choose .zip.")
		}
		return path, true, nil
	case ".zip":
		return path, false, nil
	case "":
		if _, err := os.Stat(path + ".zip"); err == nil {
			return "", false, errors.New("Save as .zip or .exe.")
		}
		return path + ".zip", false, nil
	default:
		return "", false, errors.New("Save as .zip or .exe.")
	}
}

func checkTarget(server string) (string, int, error) { return maker.ParseServer(server) }

func checkStale() string { return "Server changed while checking; press Check again." }

func checkOK() string { return "Reachable, certificate pinned." }

func checkFailed(host string, port int) string {
	return fmt.Sprintf("Cannot reach %s. If this computer uses Cloudflare WARP, turn it on.", net.JoinHostPort(host, strconv.Itoa(port)))
}

func savedMessage(path string, defaultIcon bool) string {
	if defaultIcon {
		return "Saved " + path + " with the default icon."
	}
	return "Saved " + path + "."
}
