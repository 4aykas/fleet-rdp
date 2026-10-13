//go:build windows

package launch

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"golang.org/x/sys/windows/registry"

	"github.com/4aykas/fleet-rdp/payload"
)

const testBase = `Software\fleet-rdp-test`

func cleanRegistry(t *testing.T, hosts ...string) {
	t.Helper()
	remove := func() {
		for _, h := range hosts {
			_ = registry.DeleteKey(registry.CURRENT_USER, testBase+`\`+h)
		}
		_ = registry.DeleteKey(registry.CURRENT_USER, testBase)
	}
	remove()
	t.Cleanup(remove)
}

func TestRegistryStoreRoundTrip(t *testing.T) {
	cleanRegistry(t, "10.0.0.5", "fd00::1")
	s := &RegistryStore{root: registry.CURRENT_USER, path: testBase}

	if _, present, err := s.Get("10.0.0.5"); err != nil || present {
		t.Fatalf("empty store: present=%v err=%v", present, err)
	}

	sum := bytes.Repeat([]byte{0xAB}, 32)
	if err := s.Set("10.0.0.5", sum); err != nil {
		t.Fatal(err)
	}
	got, present, err := s.Get("10.0.0.5")
	if err != nil || !present || !bytes.Equal(got, sum) {
		t.Fatalf("got %x present=%v err=%v", got, present, err)
	}

	// Set overwrites.
	other := bytes.Repeat([]byte{0xCD}, 32)
	if err := s.Set("10.0.0.5", other); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Get("10.0.0.5"); !bytes.Equal(got, other) {
		t.Fatalf("overwrite failed: %x", got)
	}

	// A bare IPv6 address is a legal key name.
	if err := s.Set("fd00::1", sum); err != nil {
		t.Fatal(err)
	}
	if _, present, _ := s.Get("fd00::1"); !present {
		t.Fatal("ipv6 key not found")
	}
}

func TestRegistryStoreUnusableCertHashIsAbsent(t *testing.T) {
	cleanRegistry(t, "10.0.0.5")
	s := &RegistryStore{root: registry.CURRENT_USER, path: testBase}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, testBase+`\10.0.0.5`, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	if err := k.SetDWordValue("CertHash", 7); err != nil {
		t.Fatal(err)
	}
	if _, present, err := s.Get("10.0.0.5"); err != nil || present {
		t.Fatalf("wrong type: present=%v err=%v", present, err)
	}
	if err := k.SetBinaryValue("CertHash", []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if _, present, err := s.Get("10.0.0.5"); err != nil || present {
		t.Fatalf("wrong length: present=%v err=%v", present, err)
	}

	// EnsureCert replaces it with a fresh fetch.
	der := []byte("der")
	p := payload.Payload{Host: "10.0.0.5", Port: 3389}
	if err := EnsureCert(p, s, func(string, int) ([]byte, error) { return der, nil }); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(der)
	if got, present, _ := s.Get("10.0.0.5"); !present || !bytes.Equal(got, want[:]) {
		t.Fatalf("not rewritten: %x present=%v", got, present)
	}
}
