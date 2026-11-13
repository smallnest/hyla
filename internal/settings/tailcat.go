package settings

import (
	"net/url"
	"slices"
	"strconv"

	"hyla/internal/remote"
)

// ValidListenPort reports whether s is a port a listener takes.
func ValidListenPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n > 0 && n <= 65535
}

// ValidDERPMapURL reports whether s may name a DERP map: an http or https
// URL, with no password in it for a setting to show.
func ValidDERPMapURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil
}

// TailcatListen is a tailcat:// listen of the settings', and its port:
// the one line, as the Tailcat settings of Settings › Hosts keep it.
func (s *Settings) TailcatListen() (listen, port string, ok bool) {
	for _, l := range s.Listen {
		spec, err := remote.ParseListen(l)
		if err == nil && spec.Kind == "tailcat" {
			return l, spec.Port, true
		}
	}
	return "", "", false
}

// SetTailcatListen makes this machine a tailcat host with a listener on
// port, in place of the one it had, or none for "".
func (s *Settings) SetTailcatListen(port string) {
	s.Listen = slices.DeleteFunc(s.Listen, func(l string) bool {
		spec, err := remote.ParseListen(l)
		return err == nil && spec.Kind == "tailcat"
	})
	if port != "" {
		s.Listen = append(s.Listen, "tailcat://:"+port)
	}
}

// TailcatListenPortDefault is a listener of the settings' port when it
// names none.
func TailcatListenPortDefault() string { return remote.ListenPort() }
