//go:build !windows

package credstore

import "errors"

// Write is supported only on Windows.
func Write(target, user, password string) error {
	return errors.ErrUnsupported
}
