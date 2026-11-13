package remote

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
)

// Listen is hylad's listener, as the configuration's listen key and
// `hyla daemon serve --listen` name it:
//
//   - tailcat://:7473, or tailcat:// for ListenPort: a relay for Hyla
//     anywhere, over the tailcat library (tailcat.go): WireGuard end to
//     end, with a public DERP relay only to reach the peer. The pair code
//     hylad shows is the secret to connect, and the address of the host.
//   - tcp://:7473, or tcp://0.0.0.0:7473: plain TCP on this machine's
//     network, which the password of the configuration admits (the
//     listener's token). Hyla and HylaPocket reach it by its address.
type Listen struct {
	Kind string // "tailcat" or "tcp"
	Host string // tcp's bind address, "" for every address
	Port string
	// Tailcat is the pair code of a tailcat listener, as hylad makes
	// it: "" of a setting's, which hylad's key's is.
	Tailcat string
	// PocketPort is the listener's port for HylaPocket, whose pair code
	// is the only address a phone needs: PocketPort, which hylad served
	// on a tcp listener's port before.
	PocketPort string
}

// ParseListen reads a listener.
func ParseListen(s string) (Listen, error) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return Listen{}, fmt.Errorf("%q is not a listener: %v", s, err)
	}
	if u.Scheme == "tcp" {
		if u.Path != "" && u.Path != "/" {
			return Listen{}, fmt.Errorf("%q: a tcp listener takes an address and a port, no path", s)
		}
		l := Listen{Kind: "tcp", Host: u.Hostname(), Port: u.Port()}
		if l.Port == "" {
			l.Port = ListenPort()
		}
		if n, err := strconv.Atoi(l.Port); err != nil || n < 1 || n > 65535 {
			return Listen{}, fmt.Errorf("%q: a port is 1 to 65535", s)
		}
		return l, nil
	}
	if u.Scheme != "tailcat" {
		return Listen{}, fmt.Errorf("%q: hylad listens on tailcat://:port or tcp://:port", s)
	}
	if u.Hostname() != "" {
		return Listen{}, fmt.Errorf("%q: a tailcat listener takes any port: write tailcat://:port", s)
	}
	code := strings.TrimPrefix(u.Path, "/")
	if code != "" && !ValidPairCode(code) {
		return Listen{}, fmt.Errorf("%q: not a tailcat pair code", s)
	}
	l := Listen{Kind: "tailcat", Port: u.Port(), PocketPort: PocketPort}
	if l.Port == "" {
		l.Port = ListenPort()
	}
	if n, err := strconv.Atoi(l.Port); err != nil || n < 1 || n > 65535 {
		return Listen{}, fmt.Errorf("%q: a port is 1 to 65535", s)
	}
	return l, nil
}

// ListenPort is the port a tailcat listener takes when it names none:
// DefaultPort, or the next for a development build, whose hylad then
// listens beside the installed Hyla's on the same machine.
func ListenPort() string {
	if mygo.IsDev() {
		n, _ := strconv.Atoi(DefaultPort)
		return strconv.Itoa(n + 1)
	}
	return DefaultPort
}

// PocketPort is the port of a tailcat listener that serves HylaPocket:
// the one hylad served it on a tcp listener, so a phone that had it there
// keeps working.
const PocketPort = "7297"

func (l Listen) String() string {
	if l.Kind == "tcp" {
		return "tcp://" + net.JoinHostPort(l.Host, l.Port)
	}
	return l.Kind + "://:" + l.Port
}
