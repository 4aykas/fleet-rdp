// Package payload is the tail format of a launcher exe: the template's own
// bytes, then the connection as JSON, a little-endian u32 with the JSON
// length, and the 8-byte magic "FLTRDP01". An unsigned PE tolerates the
// overlay; a signed one would not, which is fine because nothing is signed.
package payload

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	Version     = 1
	Magic       = "FLTRDP01"
	MaxJSON     = 64 << 10
	DefaultPort = 3389

	trailerLen = 4 + len(Magic)
)

var (
	ErrNoPayload  = errors.New("payload: file carries no payload")
	ErrCorrupt    = errors.New("payload: damaged payload")
	ErrHasPayload = errors.New("payload: template already carries a payload")
	ErrInvalid    = errors.New("payload: invalid")
	// ErrVersion is shown to the user as is.
	ErrVersion = errors.New("this launcher is newer than its template")
)

// Payload is JSON v1. Field order is the wire order: the dashboard's
// TypeScript writer must serialize in exactly this order.
type Payload struct {
	V          int    `json:"v"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	CertSha256 string `json:"certSha256,omitempty"`
}

// Validate checks everything a launcher needs. Port 0 means "not given" and is
// accepted here; Read turns it into DefaultPort.
func (p Payload) Validate() error {
	if p.V != Version {
		return ErrVersion
	}
	if strings.TrimSpace(p.Host) == "" {
		return fmt.Errorf("%w: host is empty", ErrInvalid)
	}
	if p.Port < 0 || p.Port > 65535 {
		return fmt.Errorf("%w: port must be 1-65535", ErrInvalid)
	}
	if strings.TrimSpace(p.User) == "" {
		return fmt.Errorf("%w: user is empty", ErrInvalid)
	}
	if p.CertSha256 != "" {
		if sum, err := hex.DecodeString(p.CertSha256); err != nil || len(sum) != 32 {
			return fmt.Errorf("%w: certSha256 must be 64 hex characters", ErrInvalid)
		}
	}
	return nil
}

// marshal writes the compact JSON the shared vector holds. HTML escaping is
// off because JSON.stringify in the dashboard does not do it either.
func marshal(p Payload) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// Append returns template + payload tail. It refuses a template that already
// ends in the magic: a second payload would be silently ignored by Read.
func Append(template []byte, p Payload) ([]byte, error) {
	if bytes.HasSuffix(template, []byte(Magic)) {
		return nil, ErrHasPayload
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	js, err := marshal(p)
	if err != nil {
		return nil, err
	}
	if len(js) > MaxJSON {
		return nil, fmt.Errorf("%w: payload is larger than %d bytes", ErrInvalid, MaxJSON)
	}
	out := make([]byte, 0, len(template)+len(js)+trailerLen)
	out = append(out, template...)
	out = append(out, js...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(js)))
	return append(out, Magic...), nil
}

// Read parses the payload at the end of exe. Error texts never quote the JSON:
// it holds the password.
func Read(exe []byte) (Payload, error) {
	if len(exe) < trailerLen || !bytes.HasSuffix(exe, []byte(Magic)) {
		return Payload{}, ErrNoPayload
	}
	n := binary.LittleEndian.Uint32(exe[len(exe)-trailerLen : len(exe)-len(Magic)])
	if n > MaxJSON || int64(n) > int64(len(exe)-trailerLen) {
		return Payload{}, ErrCorrupt
	}
	body := exe[len(exe)-trailerLen-int(n) : len(exe)-trailerLen]
	var p Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Payload{}, fmt.Errorf("%w: not valid JSON", ErrCorrupt)
	}
	if p.Port == 0 {
		p.Port = DefaultPort
	}
	if err := p.Validate(); err != nil {
		return Payload{}, err
	}
	return p, nil
}
