package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/4aykas/fleet-rdp/launch"
	"github.com/4aykas/fleet-rdp/payload"
)

const damagedMessage = "This launcher file is damaged. Ask for a new one."

// damagedError shows the friendly sentence and still unwraps to the cause.
type damagedError struct{ cause error }

func (e damagedError) Error() string { return damagedMessage }
func (e damagedError) Unwrap() error { return e.cause }

const templateMessage = "This is a template; build a launcher with fleet-rdp build"

// deps is everything run touches outside memory, so the flow is tested on any
// OS with fakes. main_windows.go wires the real ones.
type deps struct {
	readSelf     func() ([]byte, error)
	flags        []string
	ensureCert   func(payload.Payload) error
	writeCred    func(target, user, password string) error
	reach        func(host string, port int) error
	askRetry     func(title, text string) bool
	monitorCount func() int
	start        func(args []string) error
}

// errCancelled means the person pressed Cancel on the unreachable dialog:
// nothing was written, and there is nothing more to say.
var errCancelled = errors.New("cancelled")

// dialogTitle names the server in every dialog the launcher shows.
func dialogTitle(name string) string {
	if name == "" {
		return "Fleet RDP"
	}
	return name + " - Fleet RDP"
}

func unreachableMessage(p payload.Payload) string {
	return fmt.Sprintf("Cannot reach %s (%s).\n\nIf this computer uses Cloudflare WARP, turn it on, then press Retry.",
		p.Name, net.JoinHostPort(launch.BareHost(p.Host), strconv.Itoa(p.Port)))
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
	if errors.Is(err, payload.ErrCorrupt) || errors.Is(err, payload.ErrInvalid) {
		return "", damagedError{err}
	}
	if err != nil {
		return "", err
	}

	// mstsc's own error for an unreachable server says nothing useful; the
	// usual cause behind Zero Trust is WARP being off.
	for d.reach(p.Host, p.Port) != nil {
		if !d.askRetry(dialogTitle(p.Name), unreachableMessage(p)) {
			return p.Name, errCancelled
		}
	}

	// Not fatal (spec, step 2): without a CertHash mstsc shows its own
	// certificate dialog, which is the honest fallback, not a dead end.
	_ = d.ensureCert(p)
	// Target without the port: verified live, mstsc ignores a credential saved
	// under TERMSRV/host:port.
	if err := d.writeCred("TERMSRV/"+launch.BareHost(p.Host), p.User, p.Password); err != nil {
		return p.Name, fmt.Errorf("could not save the login: %w", err)
	}
	multi := launch.ChooseMulti(d.monitorCount(), p.Monitors, d.flags)
	if err := d.start(launch.MstscArgs(p.Host, p.Port, multi)); err != nil {
		return p.Name, err
	}
	return p.Name, nil
}
