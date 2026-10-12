// Package payload is the tail format of a launcher exe: the template's own
// bytes, then the connection as JSON, a little-endian u32 with the JSON
// length, and the 8-byte magic "FLTRDP01". An unsigned PE tolerates the
// overlay; a signed one would not, which is fine because nothing is signed.
//
// Rules of v1 (the TypeScript writer copies them; Validate is the reference):
//
//   - v is 1.
//   - host is an IP address or a DNS name, nothing else. An IPv4 address is
//     dotted decimal. An IPv6 address is bare ("::1") or in brackets ("[::1]");
//     brackets are allowed only around an IPv6 address. A DNS name matches
//     ^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$
//     and is at most 253 characters. No whitespace, backslash, slash or port:
//     a colon is accepted only inside an IPv6 address; the port goes in the
//     port field.
//   - port is 1-65535. Append writes 3389 for 0; Read reads 0 or a missing
//     port as 3389.
//   - user is not blank.
//   - password is not empty and at most 1280 UTF-16 code units (the Credential
//     Manager limit).
//   - certSha256 is empty or 64 hex characters.
//   - Fields are written in struct order, compact, without HTML escaping.
package payload

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode/utf16"
)

const (
	Version     = 1
	Magic       = "FLTRDP01"
	MaxJSON     = 64 << 10
	DefaultPort = 3389

	trailerLen = 4 + len(Magic)

	maxPasswordUnits = 1280
	maxHostLen       = 253
)

var dnsName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$`)

// validHost reports whether h is an IP address (IPv6 optionally in brackets)
// or a DNS name.
func validHost(h string) bool {
	if len(h) > maxHostLen {
		return false
	}
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		ip := net.ParseIP(h[1 : len(h)-1])
		return ip != nil && ip.To4() == nil && strings.Contains(h, ":")
	}
	if net.ParseIP(h) != nil {
		return true
	}
	return dnsName.MatchString(h)
}

var (
	ErrNoPayload  = errors.New("payload: file carries no payload")
	ErrCorrupt    = errors.New("payload: damaged payload")
	ErrHasPayload = errors.New("payload: template already carries a payload")
	ErrInvalid    = errors.New("payload: invalid")
	// ErrVersion is shown to the user as is.
	ErrVersion = errors.New("This launcher is newer than its template.")
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
	if !validHost(p.Host) {
		return fmt.Errorf("%w: host must be an IP address or a DNS name; put the port in the port field (--port for the CLI)", ErrInvalid)
	}
	if p.Port < 0 || p.Port > 65535 {
		return fmt.Errorf("%w: port must be 1-65535", ErrInvalid)
	}
	if strings.TrimSpace(p.User) == "" {
		return fmt.Errorf("%w: user is empty", ErrInvalid)
	}
	if p.Password == "" {
		return fmt.Errorf("%w: password is empty", ErrInvalid)
	}
	if len(utf16.Encode([]rune(p.Password))) > maxPasswordUnits {
		return fmt.Errorf("%w: password is longer than the 1280-character limit of Credential Manager", ErrInvalid)
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
	if p.Port == 0 {
		p.Port = DefaultPort
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
