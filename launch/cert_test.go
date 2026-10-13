package launch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/4aykas/fleet-rdp/payload"
)

type fakeStore struct {
	data   map[string][]byte
	getErr error
	setErr error
	sets   int
}

func newStore() *fakeStore { return &fakeStore{data: map[string][]byte{}} }

func (s *fakeStore) Get(host string) ([]byte, bool, error) {
	v, ok := s.data[host]
	return v, ok, s.getErr
}

func (s *fakeStore) Set(host string, sum []byte) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.sets++
	s.data[host] = sum
	return nil
}

const pinnedHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func pinnedBytes() []byte {
	b, err := hex.DecodeString(pinnedHex)
	if err != nil {
		panic(err)
	}
	return b
}

func TestEnsureCert(t *testing.T) {
	der := []byte("der of the leaf")
	derSum := sha256.Sum256(der)
	existing := bytes.Repeat([]byte{7}, 32)
	boom := errors.New("unreachable")

	t.Run("pinned is written even over an existing value", func(t *testing.T) {
		s := newStore()
		s.data["10.0.0.5"] = existing
		fetched := false
		p := payload.Payload{Host: "10.0.0.5", Port: 2157, CertSha256: pinnedHex}
		err := EnsureCert(p, s, func(string, int) ([]byte, error) { fetched = true; return der, nil })
		if err != nil {
			t.Fatal(err)
		}
		if fetched {
			t.Error("a pinned certificate must not trigger a network fetch")
		}
		if want := pinnedBytes(); !bytes.Equal(s.data["10.0.0.5"], want) {
			t.Errorf("stored %x, want %x", s.data["10.0.0.5"], want)
		}
	})

	t.Run("absent: fetch and store, with the bare host and the port", func(t *testing.T) {
		s := newStore()
		var gotHost string
		var gotPort int
		p := payload.Payload{Host: "[fd00::1]", Port: 2157}
		err := EnsureCert(p, s, func(h string, port int) ([]byte, error) { gotHost, gotPort = h, port; return der, nil })
		if err != nil {
			t.Fatal(err)
		}
		if gotHost != "fd00::1" || gotPort != 2157 {
			t.Errorf("fetched %s:%d", gotHost, gotPort)
		}
		if !bytes.Equal(s.data["fd00::1"], derSum[:]) {
			t.Errorf("stored %x, want %x", s.data["fd00::1"], derSum)
		}
	})

	t.Run("absent and the fetch fails: not fatal, nothing stored", func(t *testing.T) {
		s := newStore()
		p := payload.Payload{Host: "10.0.0.5", Port: 2157}
		if err := EnsureCert(p, s, func(string, int) ([]byte, error) { return nil, boom }); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
		if s.sets != 0 {
			t.Error("nothing should have been written")
		}
	})

	t.Run("present: left alone, no fetch", func(t *testing.T) {
		s := newStore()
		s.data["10.0.0.5"] = existing
		fetched := false
		p := payload.Payload{Host: "10.0.0.5", Port: 2157}
		if err := EnsureCert(p, s, func(string, int) ([]byte, error) { fetched = true; return der, nil }); err != nil {
			t.Fatal(err)
		}
		if fetched || s.sets != 0 || !bytes.Equal(s.data["10.0.0.5"], existing) {
			t.Error("an existing CertHash must be left alone")
		}
	})

	t.Run("present but not 32 bytes: unusable, refetched and rewritten", func(t *testing.T) {
		s := newStore()
		s.data["10.0.0.5"] = []byte{1, 2, 3}
		p := payload.Payload{Host: "10.0.0.5", Port: 2157}
		if err := EnsureCert(p, s, func(string, int) ([]byte, error) { return der, nil }); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(s.data["10.0.0.5"], derSum[:]) {
			t.Errorf("stored %x, want %x", s.data["10.0.0.5"], derSum)
		}
	})

	t.Run("a pinned certSha256 that is not valid hex is refused, nothing stored", func(t *testing.T) {
		for _, bad := range []string{strings.Repeat("zz", 32), "abcd"} {
			s := newStore()
			p := payload.Payload{Host: "10.0.0.5", Port: 2157, CertSha256: bad}
			if err := EnsureCert(p, s, nil); err == nil {
				t.Errorf("%q: expected an error", bad)
			}
			if s.sets != 0 {
				t.Errorf("%q: nothing should have been written", bad)
			}
		}
	})

	t.Run("a store that cannot be read is an error", func(t *testing.T) {
		s := newStore()
		s.getErr = boom
		p := payload.Payload{Host: "10.0.0.5", Port: 2157}
		if err := EnsureCert(p, s, func(string, int) ([]byte, error) { return der, nil }); !errors.Is(err, boom) {
			t.Fatalf("got %v, want the store error", err)
		}
	})

	t.Run("a store that cannot be written is an error", func(t *testing.T) {
		s := newStore()
		s.setErr = boom
		p := payload.Payload{Host: "10.0.0.5", Port: 2157, CertSha256: pinnedHex}
		if err := EnsureCert(p, s, nil); !errors.Is(err, boom) {
			t.Fatalf("got %v, want the store error", err)
		}
	})
}
