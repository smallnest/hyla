// Package beacontower implements the BeaconTower encrypted relay server,
// end-to-end encrypted tunnel handshake (X25519 + PSK + ChaCha20-Poly1305),
// credit-based stream multiplexer, and Hyla host/client endpoints.
//
// The relay server is zero-trust by design: it only pairs authenticated hosts
// and clients using HostID / SessionID tokens and forwards opaque AEAD frames
// without ever seeing plaintext traffic or PSKs.
package beacontower

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	// Scheme is the URI prefix used by Hyla for BeaconTower addresses.
	Scheme = "beacontower://"
	// PairCodePrefix versions compact encoded BeaconTower pair codes.
	PairCodePrefix = "bt1_"
	// DefaultPort is the default listening port of the BeaconTower relay.
	DefaultPort = "7474"
	// DefaultWSPath is the default WebSocket upgrade path on the relay.
	DefaultWSPath = "/ws"

	pairCodeVersion byte = 1
	ctrlMaxPayload       = 4096
	maxPairFieldLen      = 512
)

// NativeMagic starts every native TCP/TLS BeaconTower connection so the
// relay can multiplex raw socket and HTTP/WebSocket traffic on one port.
var NativeMagic = [4]byte{'B', 'T', '0', '1'}

// Control message types exchanged between peers and the relay server.
const (
	msgServerHello byte = 0x01
	msgRegister    byte = 0x02
	msgConnect     byte = 0x03
	msgInvite      byte = 0x04
	msgJoin        byte = 0x05
	msgOK          byte = 0x06
	msgErr         byte = 0x07
	msgPing        byte = 0x08
	msgPong        byte = 0x09

	flagTokenRequired byte = 0x01
)

// HostIDFromPub derives the 128-bit relay routing identifier from a host's
// X25519 public key. Knowing a HostID alone is insufficient to impersonate a
// host or decrypt end-to-end traffic.
func HostIDFromPub(pub [32]byte) [16]byte {
	var in [13 + 32]byte
	copy(in[:13], "bt-host-id-v1")
	copy(in[13:], pub[:])
	sum := sha256.Sum256(in[:])
	var id [16]byte
	copy(id[:], sum[:16])
	return id
}

func writeCtrlMsg(w io.Writer, typ byte, payload []byte) error {
	if len(payload) > ctrlMaxPayload {
		return fmt.Errorf("beacontower: control payload too large (%d)", len(payload))
	}
	buf := make([]byte, 3+len(payload))
	buf[0] = typ
	binary.BigEndian.PutUint16(buf[1:3], uint16(len(payload)))
	copy(buf[3:], payload)
	_, err := w.Write(buf)
	return err
}

func readCtrlMsg(r io.Reader) (byte, []byte, error) {
	var hdr [3]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	n := int(binary.BigEndian.Uint16(hdr[1:3]))
	if n > ctrlMaxPayload {
		return 0, nil, fmt.Errorf("beacontower: control payload exceeds limit (%d)", n)
	}
	if n == 0 {
		return hdr[0], nil, nil
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return hdr[0], payload, nil
}

// RelayAddr describes a parsed BeaconTower relay server endpoint.
type RelayAddr struct {
	Scheme string // "tcp", "tls", "ws", or "wss"
	Host   string
	Port   string
	Path   string // for ws/wss
	Raw    string
}

// ParseRelayAddr validates and normalizes a relay endpoint string.
// Accepted forms: host:port, host, tcp://host[:port], tls://host[:port],
// ws://host[:port][/path], wss://host[:port][/path].
func ParseRelayAddr(raw string) (RelayAddr, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return RelayAddr{}, errors.New("empty relay address")
	}
	if strings.ContainsFunc(s, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
		return RelayAddr{}, fmt.Errorf("%q: relay address must not contain spaces or control characters", raw)
	}
	explicitScheme := strings.Contains(s, "://")
	parseInput := s
	if !explicitScheme {
		parseInput = "tcp://" + s
	}
	u, err := url.Parse(parseInput)
	if err != nil {
		return RelayAddr{}, fmt.Errorf("%q: invalid relay address: %w", raw, err)
	}
	if u.User != nil {
		return RelayAddr{}, fmt.Errorf("%q: relay address must not contain credentials", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return RelayAddr{}, fmt.Errorf("%q: relay address must not contain query or fragment", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.TrimSpace(u.Hostname())
	if host == "" || strings.HasPrefix(host, "-") {
		return RelayAddr{}, fmt.Errorf("%q: relay address missing host", raw)
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return RelayAddr{}, fmt.Errorf("%q: invalid port %q", raw, port)
		}
	}
	addr := RelayAddr{Scheme: scheme, Host: host, Port: port}
	switch scheme {
	case "tcp", "tls":
		if u.Path != "" && u.Path != "/" {
			return RelayAddr{}, fmt.Errorf("%q: %s relay address takes no path", raw, scheme)
		}
		if addr.Port == "" {
			addr.Port = DefaultPort
		}
		if explicitScheme {
			addr.Raw = scheme + "://" + net.JoinHostPort(addr.Host, addr.Port)
		} else {
			addr.Raw = net.JoinHostPort(addr.Host, addr.Port)
		}
	case "ws", "wss":
		p := u.EscapedPath()
		if p == "" || p == "/" {
			p = DefaultWSPath
		}
		addr.Path = p
		if addr.Port == "" {
			if scheme == "wss" {
				addr.Port = "443"
			} else {
				addr.Port = "80"
			}
			addr.Raw = scheme + "://" + formatURLHost(addr.Host) + addr.Path
		} else {
			defaultPort := (scheme == "wss" && addr.Port == "443") || (scheme == "ws" && addr.Port == "80")
			if defaultPort {
				addr.Raw = scheme + "://" + formatURLHost(addr.Host) + addr.Path
			} else {
				addr.Raw = scheme + "://" + net.JoinHostPort(addr.Host, addr.Port) + addr.Path
			}
		}
	default:
		return RelayAddr{}, fmt.Errorf("%q: unsupported relay scheme %q (use host:port, tcp://, tls://, ws://, or wss://)", raw, scheme)
	}
	return addr, nil
}

func formatURLHost(host string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

// AddrPort returns host:port for dialers.
func (a RelayAddr) AddrPort() string {
	return net.JoinHostPort(a.Host, a.Port)
}

// WSURL returns the full ws:// or wss:// URL.
func (a RelayAddr) WSURL() string {
	p := a.Path
	if p == "" {
		p = DefaultWSPath
	}
	if (a.Scheme == "wss" && a.Port == "443") || (a.Scheme == "ws" && a.Port == "80") || a.Port == "" {
		return a.Scheme + "://" + formatURLHost(a.Host) + p
	}
	return a.Scheme + "://" + net.JoinHostPort(a.Host, a.Port) + p
}

// Display returns a concise host:port label for UI display.
func (a RelayAddr) Display() string {
	if (a.Scheme == "wss" && a.Port == "443") || (a.Scheme == "ws" && a.Port == "80") {
		return a.Host
	}
	return net.JoinHostPort(a.Host, a.Port)
}

// PairCode contains everything a client needs to locate a host via a relay,
// authenticate with the relay (when required), and establish an E2EE tunnel.
type PairCode struct {
	Version    byte
	Relay      string
	HostPub    [32]byte
	PSK        [32]byte
	RelayToken string
}

// EncodePairCode serializes p as a compact bt1_... code.
func EncodePairCode(p PairCode) (string, error) {
	ra, err := ParseRelayAddr(p.Relay)
	if err != nil {
		return "", fmt.Errorf("pair code relay: %w", err)
	}
	relayBytes := []byte(ra.Raw)
	tokenBytes := []byte(strings.TrimSpace(p.RelayToken))
	if len(relayBytes) > maxPairFieldLen {
		return "", errors.New("pair code relay address too long")
	}
	if len(tokenBytes) > maxPairFieldLen {
		return "", errors.New("pair code relay token too long")
	}
	var zero [32]byte
	if p.HostPub == zero || p.PSK == zero {
		return "", errors.New("pair code missing host key or preshared key")
	}
	raw := make([]byte, 0, 1+32+32+2+len(relayBytes)+2+len(tokenBytes))
	raw = append(raw, pairCodeVersion)
	raw = append(raw, p.HostPub[:]...)
	raw = append(raw, p.PSK[:]...)
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(relayBytes)))
	raw = append(raw, relayBytes...)
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(tokenBytes)))
	raw = append(raw, tokenBytes...)
	return PairCodePrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// ParsePairCode decodes a bt1_... or beacontower://bt1_... pair code.
func ParsePairCode(s string) (PairCode, error) {
	code := StripScheme(s)
	if !strings.HasPrefix(code, PairCodePrefix) {
		return PairCode{}, errors.New("not a beacontower pair code")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(code, PairCodePrefix))
	if err != nil {
		return PairCode{}, fmt.Errorf("bad beacontower pair code: %w", err)
	}
	// Minimum size: 1B version + 32B hostPub + 32B PSK + 2B relayLen + 1B relay + 2B tokenLen = 70
	if len(raw) < 70 {
		return PairCode{}, errors.New("truncated beacontower pair code")
	}
	if raw[0] != pairCodeVersion {
		return PairCode{}, fmt.Errorf("unsupported beacontower pair code version %d", raw[0])
	}
	var pc PairCode
	pc.Version = raw[0]
	copy(pc.HostPub[:], raw[1:33])
	copy(pc.PSK[:], raw[33:65])
	pos := 65
	relayLen := int(binary.BigEndian.Uint16(raw[pos : pos+2]))
	pos += 2
	if relayLen == 0 || relayLen > maxPairFieldLen || pos+relayLen+2 > len(raw) {
		return PairCode{}, errors.New("invalid relay field in beacontower pair code")
	}
	relayRaw := string(raw[pos : pos+relayLen])
	pos += relayLen
	tokenLen := int(binary.BigEndian.Uint16(raw[pos : pos+2]))
	pos += 2
	if tokenLen > maxPairFieldLen || pos+tokenLen != len(raw) {
		return PairCode{}, errors.New("invalid token field in beacontower pair code")
	}
	pc.RelayToken = string(raw[pos : pos+tokenLen])
	ra, err := ParseRelayAddr(relayRaw)
	if err != nil {
		return PairCode{}, fmt.Errorf("invalid relay in pair code: %w", err)
	}
	pc.Relay = ra.Raw
	if _, err := PublicFromPrivateOrCheckPub(pc.HostPub); err != nil {
		return PairCode{}, err
	}
	var zero [32]byte
	if pc.PSK == zero {
		return PairCode{}, errors.New("empty preshared key in pair code")
	}
	return pc, nil
}

// StripScheme returns the bare bt1_... token, stripping whitespace,
// beacontower://, hyla://beacon?code=..., or hyla://pair?code=... wrappers.
func StripScheme(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= len(Scheme) && strings.EqualFold(s[:len(Scheme)], Scheme) {
		s = strings.TrimSpace(s[len(Scheme):])
	}
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil && strings.EqualFold(u.Scheme, "hyla") &&
			(strings.EqualFold(u.Host, "beacon") || strings.EqualFold(u.Host, "pair")) {
			if c := strings.TrimSpace(u.Query().Get("code")); c != "" {
				if len(c) >= len(Scheme) && strings.EqualFold(c[:len(Scheme)], Scheme) {
					c = strings.TrimSpace(c[len(Scheme):])
				}
				return c
			}
		}
	}
	return s
}

// LooksLikePairCode performs a quick syntactic check for bt1_... strings.
func LooksLikePairCode(s string) bool {
	code := StripScheme(s)
	if len(code) < 96 || !strings.HasPrefix(code, PairCodePrefix) {
		return false
	}
	for _, r := range code {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// ValidPairCode reports whether s decodes as a complete, valid BeaconTower pair code.
func ValidPairCode(s string) bool {
	_, err := ParsePairCode(s)
	return err == nil
}
