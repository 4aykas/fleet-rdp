//go:build windows

package credstore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/danieljoos/wincred"
)

// Write saves user and password for target in the current user's Windows
// Credential Manager and replaces any credential with the same target.
func Write(target, user, password string) error {
	if target == "" {
		return errors.New("credential target must not be empty")
	}
	if user == "" {
		return errors.New("credential user must not be empty")
	}
	// Windows names are null-terminated; reject truncation before entering the API.
	if strings.ContainsRune(target, 0) || strings.ContainsRune(user, 0) {
		return errors.New("credential target and user must not contain null characters")
	}
	units := utf16.Encode([]rune(password))
	if len(units) > 1280 {
		return errors.New("credential password exceeds the 2560-byte UTF-16 limit")
	}
	blob := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(blob[i*2:], unit)
	}
	credential := wincred.NewGenericCredential(target)
	credential.UserName = user
	credential.Persist = wincred.PersistLocalMachine
	// mstsc expects UTF-16LE without a trailing null, including for generic credentials.
	credential.CredentialBlob = blob
	if err := credential.Write(); err != nil {
		return fmt.Errorf("write Windows credential: %w", err)
	}
	return nil
}
