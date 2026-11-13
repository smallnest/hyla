// Package remote reads the addresses of the machines Hyla's SSH project
// adds, and makes the tailcat key of this machine's listener, which
// HylaPocket reaches it by.
//
// An address is ssh://[user@]host[:port], or [user@]host[:port] as
// ssh(1) takes it, an alias of ~/.ssh/config too, or wsl://[distro] for a
// WSL distribution of this Windows machine.
package remote

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// DefaultPort is the port hylad's tailcat listener takes when it names
// none, as a development build's does not (ListenPort).
const DefaultPort = "7473"

// Addr is how to reach a host.
type Addr struct {
	Scheme string // "ssh" or "wsl"
	User   string // ssh's
	Host   string // the host, or for wsl the distribution, "" its default
	Port   string
	// Hyla is the hyla executable on the host, for ssh and wsl, when it is
	// not where the bridge looks (BridgeLocations).
	Hyla string
}

// ParseAddr reads an address: a URL of a scheme above, or an SSH
// destination, [user@]host[:port], as ssh(1) takes it, an alias of
// ~/.ssh/config too.
func ParseAddr(s string) (Addr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Addr{}, errors.New("no address")
	}
	if strings.ContainsFunc(s, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
		return Addr{}, fmt.Errorf("%q: an address has no spaces", s)
	}
	raw := s
	if !strings.Contains(s, "://") {
		s = "ssh://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return Addr{}, fmt.Errorf("%q is not an address: %v", raw, err)
	}
	a := Addr{Scheme: strings.ToLower(u.Scheme), Host: u.Hostname(), Port: u.Port()}
	if u.User != nil {
		a.User = u.User.Username()
		if _, has := u.User.Password(); has {
			return Addr{}, fmt.Errorf("%q: Hyla takes no password in an address", raw)
		}
	}
	a.Hyla = u.Query().Get("hyla")
	switch a.Scheme {
	case "ssh":
		if a.Host == "" {
			return Addr{}, fmt.Errorf("%q names no host", raw)
		}
	case "wsl":
		a.Port, a.User = "", ""
	default:
		return Addr{}, fmt.Errorf("%q: Hyla reaches hosts over ssh://, wsl:// or tailcat://", raw)
	}
	if u.Path != "" && u.Path != "/" {
		return Addr{}, fmt.Errorf("%q: an address has no path", raw)
	}
	// Names of hosts and users as DNS and accounts have them, which no
	// shell, and no ssh_config's %h or %r in a ProxyCommand, reads as
	// anything else; nothing ssh(1) would take for one of its options.
	if a.Host != "" && net.ParseIP(a.Host) == nil && !plainName(a.Host, "._-") || strings.HasPrefix(a.Host, "-") {
		return Addr{}, fmt.Errorf("%q: a host is a name of letters, digits, dots and dashes, or an IP address", raw)
	}
	if a.User != "" && (!plainName(a.User, "._+-") || strings.HasPrefix(a.User, "-")) {
		return Addr{}, fmt.Errorf("%q: a user is a name of letters, digits, dots and dashes", raw)
	}
	if a.Hyla != "" && !absPath(a.Hyla) {
		return Addr{}, fmt.Errorf("%q: hyla= is an absolute path of letters, digits, dots, dashes and slashes, as /opt/hyla/hyla or C:/hyla/hyla.exe", raw)
	}
	if a.Port != "" {
		for _, r := range a.Port {
			if r < '0' || r > '9' {
				return Addr{}, fmt.Errorf("%q: bad port", raw)
			}
		}
	}
	return a, nil
}

// String writes the address as ParseAddr reads it: an SSH destination
// without its scheme.
func (a Addr) String() string {
	var b strings.Builder
	if a.Scheme != "ssh" {
		b.WriteString(a.Scheme + "://")
	}
	if a.User != "" {
		b.WriteString(url.User(a.User).String() + "@")
	}
	host := a.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	b.WriteString(host)
	if a.Port != "" {
		b.WriteString(":" + a.Port)
	}
	if a.Hyla != "" {
		b.WriteString("?hyla=" + url.QueryEscape(a.Hyla))
	}
	return b.String()
}

// Bridged tells an address whose host runs `hyla __bridge` for the
// client: ssh and wsl.
func (a Addr) Bridged() bool { return a.Scheme == "ssh" || a.Scheme == "wsl" }

// Label is a short name for the host, as a new host takes by default: the
// host's first label, the distribution for WSL.
func (a Addr) Label() string {
	h := a.Host
	if a.Scheme == "wsl" && h == "" {
		return "wsl"
	}
	if net.ParseIP(h) == nil {
		h, _, _ = strings.Cut(h, ".")
	}
	return h
}

// ValidName reports whether a name may name a saved host: letters,
// digits, dots, dashes and underscores, from a letter or digit; "local"
// stands for this machine.
func ValidName(name string) bool {
	if name == "" || len(name) > 64 || strings.EqualFold(name, "local") {
		return false
	}
	for i, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if i > 0 {
			ok = ok || r == '.' || r == '-' || r == '_'
		}
		if !ok {
			return false
		}
	}
	return true
}

// plainName reports whether s holds only ASCII letters, digits and the
// characters of extra.
func plainName(s, extra string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(extra, r)) {
			return false
		}
	}
	return true
}

// absPath reports whether p is a path hyla= takes: absolute, of Unix, or
// of Windows with forward slashes, as C:/hyla/hyla.exe, of characters no
// shell, sh, cmd or PowerShell, reads as anything else.
func absPath(p string) bool {
	rest := p
	if len(p) > 3 && p[1] == ':' && p[2] == '/' && ('A' <= p[0] && p[0] <= 'Z' || 'a' <= p[0] && p[0] <= 'z') {
		rest = p[2:]
	}
	return strings.HasPrefix(rest, "/") && plainName(rest, "._+-/")
}
