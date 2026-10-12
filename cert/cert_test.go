package cert

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"testing"
	"time"
)

// confirm is a Connection Confirm selecting TLS (selected protocol = 1).
var confirm = []byte{0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0, 0x00, 0x00, 0x12, 0x34, 0x00, 0x02, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00}

func selfSigned(t *testing.T) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "rdp-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, der
}

// serve accepts one connection, reads the 19-byte request into got, writes
// reply, then (if cert is set) speaks TLS on the same connection. With
// reply == nil it accepts and stays silent until the test ends.
func serve(t *testing.T, reply []byte, cert *tls.Certificate) (port int, got chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got = make(chan []byte, 1)
	done := make(chan struct{})
	t.Cleanup(func() { close(done); ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		req := make([]byte, 19)
		if _, err := io.ReadFull(conn, req); err != nil {
			return
		}
		got <- req
		if reply == nil {
			<-done
			return
		}
		conn.Write(reply)
		if cert != nil {
			tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{*cert}})
			if tc.Handshake() == nil {
				io.Copy(io.Discard, tc)
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestFetchReturnsTheLeafCertificate(t *testing.T) {
	cert, der := selfSigned(t)
	port, got := serve(t, confirm, &cert)

	leaf, err := Fetch(context.Background(), "127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(leaf, der) {
		t.Fatal("returned certificate is not the one the server presented")
	}
	if !bytes.Equal(<-got, x224Request) {
		t.Fatal("the server did not receive the X.224 Connection Request")
	}
}

func TestFetchSurfacesNegotiationFailure(t *testing.T) {
	failure := append([]byte(nil), confirm...)
	failure[11] = 0x03 // RDP_NEG_FAILURE
	port, _ := serve(t, failure, nil)

	if _, err := Fetch(context.Background(), "127.0.0.1", port); err == nil {
		t.Fatal("expected an error for RDP_NEG_FAILURE")
	}
}

func TestFetchRejectsAServerThatDoesNotSelectTLS(t *testing.T) {
	plain := append([]byte(nil), confirm...)
	plain[15] = 0x00 // selected protocol 0 = standard RDP security
	port, _ := serve(t, plain, nil)

	if _, err := Fetch(context.Background(), "127.0.0.1", port); err == nil {
		t.Fatal("expected an error when TLS was not selected")
	}
}

func TestFetchTimesOutOnASilentServer(t *testing.T) {
	port, _ := serve(t, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := Fetch(ctx, "127.0.0.1", port); err == nil {
		t.Fatal("expected a timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %v; the context deadline was ignored", time.Since(start))
	}
}

func TestSha256(t *testing.T) {
	der := []byte("some der bytes")
	if Sha256(der) != sha256.Sum256(der) {
		t.Fatal("Sha256 is not SHA-256")
	}
}
