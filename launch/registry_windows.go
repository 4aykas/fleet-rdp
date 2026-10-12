//go:build windows

package launch

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

const (
	serversPath   = `Software\Microsoft\Terminal Server Client\Servers`
	certHashValue = "CertHash"
)

// RegistryStore reads and writes mstsc's CertHash (REG_BINARY, 32 bytes) under
// HKCU. The UsernameHint value that may live in the same key is not touched.
type RegistryStore struct {
	root registry.Key
	path string
}

var _ CertStore = (*RegistryStore)(nil)

func NewRegistryStore() *RegistryStore {
	return &RegistryStore{root: registry.CURRENT_USER, path: serversPath}
}

func (s *RegistryStore) Get(host string) ([]byte, bool, error) {
	k, err := registry.OpenKey(s.root, s.path+`\`+host, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer k.Close()
	v, _, err := k.GetBinaryValue(certHashValue)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

func (s *RegistryStore) Set(host string, sum []byte) error {
	k, _, err := registry.CreateKey(s.root, s.path+`\`+host, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetBinaryValue(certHashValue, sum)
}
