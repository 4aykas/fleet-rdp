// Package cert fetches the certificate an RDP server presents, for trust on
// first use. mstsc keeps its own pin (the CertHash registry value); this
// package only produces the bytes to put there.
package cert

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// x224Request is a TPKT + X.224 Connection Request carrying an RDP_NEG_REQ
// that asks for TLS | CredSSP (requested protocols 0x00000003).
var x224Request = []byte{
	0x03, 0x00, 0x00, 0x13, 0x0E, 0xE0, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x08, 0x00, 0x03, 0x00, 0x00, 0x00,
}

const (
	timeout     = 5 * time.Second
	responseLen = 19
)

// Fetch dials host:port, negotiates TLS the way an RDP client does and returns
// the DER of the server's leaf certificate. The whole exchange is bounded by 5
// seconds and by ctx.
func Fetch(ctx context.Context, host string, port int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("cert: connect: %w", err)
	}
	defer conn.Close()
	// The deadline covers the plain reads; AfterFunc covers a cancelled ctx.
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	if _, err := conn.Write(x224Request); err != nil {
		return nil, fmt.Errorf("cert: send connection request: %w", err)
	}
	resp := make([]byte, responseLen)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return nil, fmt.Errorf("cert: read connection confirm: %w", err)
	}
	if resp[0] != 0x03 || resp[5] != 0xD0 {
		return nil, errors.New("cert: not an RDP server")
	}
	switch resp[11] {
	case 0x02: // RDP_NEG_RSP
		if binary.LittleEndian.Uint32(resp[15:19]) == 0 {
			return nil, errors.New("cert: server did not select TLS")
		}
	case 0x03: // RDP_NEG_FAILURE
		return nil, errors.New("cert: server refused the negotiation (RDP_NEG_FAILURE)")
	default:
		return nil, fmt.Errorf("cert: unexpected negotiation type 0x%02x", resp[11])
	}

	// Verification is off on purpose: we are not authenticating the chain, we
	// are recording the key the server shows now and pinning it (trust on
	// first use). Anything stronger is the payload's certSha256.
	tc := tls.Client(conn, tlsConfig())
	if err := tc.HandshakeContext(ctx); err != nil {
		return nil, fmt.Errorf("cert: TLS handshake: %w", err)
	}
	certs := tc.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, errors.New("cert: server presented no certificate")
	}
	return certs[0].Raw, nil
}

// tlsConfig accepts every protocol version and cipher suite Go can speak:
// nothing is authenticated here (TOFU), and old RDP servers must not fail.
func tlsConfig() *tls.Config {
	var suites []uint16
	for _, s := range tls.CipherSuites() {
		suites = append(suites, s.ID)
	}
	for _, s := range tls.InsecureCipherSuites() {
		suites = append(suites, s.ID)
	}
	return &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10, CipherSuites: suites} // #nosec G402
}

// Sha256 is the CertHash mstsc stores: SHA-256 of the DER certificate.
func Sha256(der []byte) [32]byte { return sha256.Sum256(der) }
