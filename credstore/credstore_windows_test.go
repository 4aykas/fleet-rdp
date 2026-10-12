//go:build windows

package credstore

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/danieljoos/wincred"
)

func testTarget(t *testing.T) string {
	t.Helper()
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	target := "fleet-rdp-test/" + hex.EncodeToString(random)
	t.Cleanup(func() {
		err := wincred.NewGenericCredential(target).Delete()
		if err != nil && !errors.Is(err, wincred.ErrElementNotFound) {
			t.Errorf("cleanup credential: %v", err)
		}
	})
	return target
}

func assertCredential(t *testing.T, target, user, password string) {
	t.Helper()
	credential, err := wincred.GetGenericCredential(target)
	if err != nil {
		t.Fatal(err)
	}
	if credential.UserName != user || credential.Persist != wincred.PersistLocalMachine {
		t.Fatal("credential username or persistence mismatch")
	}
	blob := credential.CredentialBlob
	if len(blob)%2 != 0 {
		t.Fatal("password blob has odd byte length")
	}
	units := make([]uint16, len(blob)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(blob[i*2:])
	}
	if string(utf16.Decode(units)) != password || len(units) != len(utf16.Encode([]rune(password))) {
		t.Fatal("password blob does not match unterminated UTF-16LE")
	}
	// Generic reads above check type; enumeration also proves overwrites create no duplicate.
	credentials, err := wincred.FilteredList(target)
	if err != nil || len(credentials) != 1 {
		t.Fatalf("expected one credential, got %d (error: %v)", len(credentials), err)
	}
	if !bytes.Equal(credentials[0].CredentialBlob, blob) {
		t.Fatal("enumerated blob differs")
	}
}

func TestWriteRoundTrip(t *testing.T) {
	target := testTarget(t)
	if err := Write(target, "login", "p#ss wörd Ж"); err != nil {
		t.Fatal(err)
	}
	assertCredential(t, target, "login", "p#ss wörd Ж")
	if os.Getenv("FLEET_RDP_TEST_CMDKEY") == "1" {
		t.Logf("temporary credential available for cmdkey: %s", target)
		time.Sleep(20 * time.Second)
	}
}

func TestWriteOverwrite(t *testing.T) {
	target := testTarget(t)
	for _, password := range []string{"first", "second 😀"} {
		if err := Write(target, "login", password); err != nil {
			t.Fatal(err)
		}
	}
	assertCredential(t, target, "login", "second 😀")
}

func TestWriteDomainUser(t *testing.T) {
	target := testTarget(t)
	if err := Write(target, `DOMAIN\login`, "example"); err != nil {
		t.Fatal(err)
	}
	assertCredential(t, target, `DOMAIN\login`, "example")
}

func TestWriteInvalid(t *testing.T) {
	for _, kind := range []string{"target", "user", "password", "null target", "null user", "surrogate limit"} {
		t.Run(kind, func(t *testing.T) {
			target := testTarget(t)
			inputTarget, user, password := target, "login", "example"
			switch kind {
			case "target":
				inputTarget = ""
			case "user":
				user = ""
			case "password":
				password = strings.Repeat("x", 1281)
			case "null target":
				inputTarget += "\x00suffix"
			case "null user":
				user += "\x00suffix"
			case "surrogate limit":
				password = strings.Repeat("😀", 641)
			}
			err := Write(inputTarget, user, password)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if strings.Contains(err.Error(), password) {
				t.Fatal("error contains password")
			}
			if _, err := wincred.GetGenericCredential(target); !errors.Is(err, wincred.ErrElementNotFound) {
				t.Fatal("invalid input must not create a credential")
			}
		})
	}
}

func TestWritePasswordBoundaries(t *testing.T) {
	for _, password := range []string{"", strings.Repeat("x", 1280), strings.Repeat("😀", 640)} {
		target := testTarget(t)
		if err := Write(target, "login", password); err != nil {
			t.Fatal(err)
		}
		assertCredential(t, target, "login", password)
	}
}
