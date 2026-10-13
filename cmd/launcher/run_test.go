package main

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/4aykas/fleet-rdp/payload"
)

type rec struct {
	calls   []string
	credArg [3]string
	started []string
}

func template(t *testing.T, p payload.Payload) []byte {
	t.Helper()
	exe, err := payload.Append([]byte("MZ template"), p)
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func sample() payload.Payload {
	return payload.Payload{V: 1, Name: "SGCLOUD", Host: "10.0.0.5", Port: 2157, User: "vlad", Password: "Dummy-Pass-1!"}
}

func newDeps(exe []byte, r *rec) deps {
	return deps{
		readSelf: func() ([]byte, error) { return exe, nil },
		ensureCert: func(payload.Payload) error {
			r.calls = append(r.calls, "cert")
			return nil
		},
		writeCred: func(target, user, password string) error {
			r.calls = append(r.calls, "cred")
			r.credArg = [3]string{target, user, password}
			return nil
		},
		reach: func(string, int) error {
			r.calls = append(r.calls, "reach")
			return nil
		},
		askRetry: func(string, string) bool {
			r.calls = append(r.calls, "ask")
			return false
		},
		monitorCount: func() int { return 1 },
		start: func(args []string) error {
			r.calls = append(r.calls, "start")
			r.started = args
			return nil
		},
	}
}

func TestRunHappyPath(t *testing.T) {
	r := &rec{}
	name, err := run(newDeps(template(t, sample()), r))
	if err != nil {
		t.Fatal(err)
	}
	if name != "SGCLOUD" {
		t.Errorf("name = %q", name)
	}
	if want := []string{"reach", "cert", "cred", "start"}; !reflect.DeepEqual(r.calls, want) {
		t.Errorf("order = %v, want %v", r.calls, want)
	}
	if r.credArg != [3]string{"TERMSRV/10.0.0.5", "vlad", "Dummy-Pass-1!"} {
		t.Errorf("credential = %v (the target must have no port)", r.credArg)
	}
	if want := []string{"/v:10.0.0.5:2157", "/f"}; !reflect.DeepEqual(r.started, want) {
		t.Errorf("mstsc args = %v, want %v", r.started, want)
	}
}

func TestRunIPv6UsesTheBareAddressForTheTargetAndBracketsTheArgument(t *testing.T) {
	r := &rec{}
	p := sample()
	p.Host = "fd00::1"
	if _, err := run(newDeps(template(t, p), r)); err != nil {
		t.Fatal(err)
	}
	if r.credArg[0] != "TERMSRV/fd00::1" {
		t.Errorf("target = %q", r.credArg[0])
	}
	if r.started[0] != "/v:[fd00::1]:2157" {
		t.Errorf("arg = %q", r.started[0])
	}
}

func TestRunMonitors(t *testing.T) {
	cases := []struct {
		monitors int
		mode     string
		flags    []string
		multi    bool
	}{
		{2, "", nil, true}, {1, "", nil, false},
		{2, "", []string{"--single"}, false}, {1, "", []string{"--multi"}, true},
		{2, payload.MonitorsOne, nil, false}, {1, payload.MonitorsAll, nil, true},
		{2, payload.MonitorsOne, []string{"--multi"}, true},
	}
	for _, c := range cases {
		r := &rec{}
		p := sample()
		p.Monitors = c.mode
		d := newDeps(template(t, p), r)
		d.monitorCount = func() int { return c.monitors }
		d.flags = c.flags
		if _, err := run(d); err != nil {
			t.Fatal(err)
		}
		got := r.started[len(r.started)-1] == "/multimon"
		if got != c.multi {
			t.Errorf("monitors=%d mode=%q flags=%v: multimon=%v, want %v", c.monitors, c.mode, c.flags, got, c.multi)
		}
	}
}

func TestRunOnABareTemplate(t *testing.T) {
	r := &rec{}
	_, err := run(newDeps([]byte("MZ no payload here"), r))
	if err == nil || err.Error() != "This is a template; build a launcher with fleet-rdp build" {
		t.Fatalf("got %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("nothing may run on a template: %v", r.calls)
	}
}

func TestRunNewerPayload(t *testing.T) {
	js := `{"v":2,"host":"h","user":"u","password":"p"}`
	exe := append([]byte("MZ"), js...)
	exe = append(exe, byte(len(js)), 0, 0, 0)
	exe = append(exe, payload.Magic...)
	_, err := run(newDeps(exe, &rec{}))
	if !errors.Is(err, payload.ErrVersion) {
		t.Fatalf("got %v, want ErrVersion", err)
	}
}

func TestRunFailures(t *testing.T) {
	boom := errors.New("boom")
	t.Run("certificate failure is not fatal", func(t *testing.T) {
		r := &rec{}
		d := newDeps(template(t, sample()), r)
		d.ensureCert = func(payload.Payload) error { r.calls = append(r.calls, "cert"); return boom }
		if _, err := run(d); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
		if want := []string{"reach", "cert", "cred", "start"}; !reflect.DeepEqual(r.calls, want) {
			t.Errorf("calls = %v, want %v", r.calls, want)
		}
	})
	t.Run("credential", func(t *testing.T) {
		r := &rec{}
		d := newDeps(template(t, sample()), r)
		d.writeCred = func(string, string, string) error { return boom }
		if _, err := run(d); !errors.Is(err, boom) {
			t.Fatalf("got %v", err)
		}
		if len(r.started) != 0 {
			t.Error("mstsc must not start without a saved credential")
		}
	})
	t.Run("the password is not in the message", func(t *testing.T) {
		r := &rec{}
		d := newDeps(template(t, sample()), r)
		d.start = func([]string) error { return boom }
		_, err := run(d)
		if err == nil || strings.Contains(err.Error(), "Dummy-Pass-1!") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestRunUnreachableCancel(t *testing.T) {
	r := &rec{}
	d := newDeps(template(t, sample()), r)
	d.reach = func(string, int) error { r.calls = append(r.calls, "reach"); return errors.New("timeout") }
	var shown string
	d.askRetry = func(title, text string) bool {
		r.calls = append(r.calls, "ask")
		shown = title + "|" + text
		return false
	}
	_, err := run(d)
	if !errors.Is(err, errCancelled) {
		t.Fatalf("err = %v, want errCancelled", err)
	}
	for _, c := range r.calls {
		if c == "cert" || c == "cred" || c == "start" {
			t.Fatalf("%s ran after Cancel: %v", c, r.calls)
		}
	}
	if !strings.Contains(shown, "Cannot reach") || !strings.Contains(shown, "Cloudflare WARP") {
		t.Fatalf("dialog = %q", shown)
	}
	if strings.Contains(shown, sample().Password) {
		t.Fatal("dialog shows the password")
	}
}

func TestRunUnreachableRetryThenOK(t *testing.T) {
	r := &rec{}
	d := newDeps(template(t, sample()), r)
	fails := 2
	d.reach = func(string, int) error {
		r.calls = append(r.calls, "reach")
		if fails > 0 {
			fails--
			return errors.New("refused")
		}
		return nil
	}
	d.askRetry = func(string, string) bool { r.calls = append(r.calls, "ask"); return true }
	if _, err := run(d); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.calls[:5], ","); got != "reach,ask,reach,ask,reach" {
		t.Fatalf("calls start %q", got)
	}
}

func TestRunReachBeforeCert(t *testing.T) {
	r := &rec{}
	if _, err := run(newDeps(template(t, sample()), r)); err != nil {
		t.Fatal(err)
	}
	ri, ci := slices.Index(r.calls, "reach"), slices.Index(r.calls, "cert")
	if ri < 0 || ci < 0 || ri > ci {
		t.Fatalf("reach must come before cert: %v", r.calls)
	}
}

func TestRunCannotReadItself(t *testing.T) {
	d := newDeps(nil, &rec{})
	d.readSelf = func() ([]byte, error) { return nil, errors.New("denied") }
	if _, err := run(d); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRunDamagedPayloadIsFriendly(t *testing.T) {
	const want = "This launcher file is damaged. Ask for a new one."
	corrupt := append([]byte("MZ"), 0xff, 0xff, 0xff, 0x7f)
	corrupt = append(corrupt, payload.Magic...)
	_, err := run(newDeps(corrupt, &rec{}))
	if err == nil || err.Error() != want || !errors.Is(err, payload.ErrCorrupt) {
		t.Errorf("corrupt: got %v", err)
	}
	js := `{"v":1,"host":"a b","user":"u","password":"p"}`
	bad := append([]byte("MZ"), js...)
	bad = append(bad, byte(len(js)), 0, 0, 0)
	bad = append(bad, payload.Magic...)
	_, err = run(newDeps(bad, &rec{}))
	if err == nil || err.Error() != want || !errors.Is(err, payload.ErrInvalid) {
		t.Errorf("invalid: got %v", err)
	}
}

func TestRunNewerPayloadText(t *testing.T) {
	js := `{"v":2,"host":"h","user":"u","password":"p"}`
	exe := append([]byte("MZ"), js...)
	exe = append(exe, byte(len(js)), 0, 0, 0)
	exe = append(exe, payload.Magic...)
	_, err := run(newDeps(exe, &rec{}))
	if err == nil || err.Error() != "This launcher is newer than its template." {
		t.Errorf("got %v", err)
	}
}
