package launch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/4aykas/fleet-rdp/cert"
	"github.com/4aykas/fleet-rdp/payload"
)

// CertStore is mstsc's per-server certificate pin.
type CertStore interface {
	Get(host string) (sum []byte, present bool, err error)
	Set(host string, sum []byte) error
}

// EnsureCert applies the trust rules before mstsc starts:
//
//	pinned certSha256 in the payload -> write it, always
//	CertHash absent                  -> fetch the certificate, write its SHA-256
//	CertHash present                 -> leave it alone
//	CertHash of the wrong type or
//	length (not 32 bytes)            -> unusable, counts as absent: fetch and rewrite
//
// A failed fetch (unreachable, WARP off) is not fatal: mstsc will then fail or
// show its own dialog, which is the honest fallback.
func EnsureCert(p payload.Payload, store CertStore, fetch func(host string, port int) ([]byte, error)) error {
	host := BareHost(p.Host)
	if p.CertSha256 != "" {
		sum, err := hex.DecodeString(p.CertSha256)
		if err != nil || len(sum) != sha256.Size {
			return errors.New("pinned certSha256 is not 64 hex characters")
		}
		return store.Set(host, sum)
	}
	cur, present, err := store.Get(host)
	if err != nil {
		return err
	}
	if present && len(cur) == sha256.Size {
		return nil
	}
	der, err := fetch(host, p.Port)
	if err != nil {
		return nil
	}
	sum := cert.Sha256(der)
	return store.Set(host, sum[:])
}
