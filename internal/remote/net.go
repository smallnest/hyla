package remote

import (
	"crypto/rand"
	"encoding/hex"
	"net"
)

// RandomToken is a password of a listener, or of the pair code of a
// keyless one: 24 bytes of randomness, as hex.
func RandomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// LocalAddrs are this machine's addresses on its networks, loopback left
// out: where a client of another machine reaches it.
func LocalAddrs() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ip, _, err := net.ParseCIDR(a.String()); err == nil && ip.To4() != nil {
				out = append(out, ip.String())
			}
		}
	}
	return out
}
