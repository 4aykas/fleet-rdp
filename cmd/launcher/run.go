package main

import (
	"errors"
	"fmt"

	"github.com/4aykas/fleet-rdp/launch"
	"github.com/4aykas/fleet-rdp/payload"
)

const templateMessage = "This is a template; build a launcher with fleet-rdp build"

// deps is everything run touches outside memory, so the flow is tested on any
// OS with fakes. main_windows.go wires the real ones.
type deps struct {
	readSelf     func() ([]byte, error)
	flags        []string
	ensureCert   func(payload.Payload) error
	writeCred    func(target, user, password string) error
	monitorCount func() int
	start        func(args []string) error
}

// run does the launcher's whole job. It returns the payload's name (empty when
// the payload could not be read) for the error dialog's title. Error texts
// never contain the password.
func run(d deps) (string, error) {
	exe, err := d.readSelf()
	if err != nil {
		return "", fmt.Errorf("could not read the launcher file: %w", err)
	}
	p, err := payload.Read(exe)
	if errors.Is(err, payload.ErrNoPayload) {
		return "", errors.New(templateMessage)
	}
	if err != nil {
		return "", err
	}

	// Not fatal (spec, step 2): without a CertHash mstsc shows its own
	// certificate dialog, which is the honest fallback, not a dead end.
	_ = d.ensureCert(p)
	// Target without the port: verified live, mstsc ignores a credential saved
	// under TERMSRV/host:port.
	if err := d.writeCred("TERMSRV/"+launch.BareHost(p.Host), p.User, p.Password); err != nil {
		return p.Name, fmt.Errorf("could not save the login: %w", err)
	}
	multi := launch.ChooseMulti(d.monitorCount(), d.flags)
	if err := d.start(launch.MstscArgs(p.Host, p.Port, multi)); err != nil {
		return p.Name, err
	}
	return p.Name, nil
}
