//go:build !windows

package daemon

import (
	"net"
	"os"
	"syscall"
	"time"
)

// listenSocket listens on a Unix socket only its user can connect to: the
// umask makes it so as it is made, rather than a chmod after, which would
// leave a moment where it is anyone's.
func listenSocket(path string) (net.Listener, error) {
	old := syscall.Umask(0o177)
	defer syscall.Umask(old)
	return net.Listen("unix", path)
}

// answered reports whether a hylad listens on the socket: one that does is
// another daemon's, which removing the path would leave running with
// nothing to connect to, its sessions gone from every client that dials.
// A path nothing listens on, as one a killed hylad left, is answered by
// no one, and is taken.
func answered(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	c, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// ownedByUser reports whether a file is this process's user's.
func ownedByUser(st os.FileInfo) bool {
	s, ok := st.Sys().(*syscall.Stat_t)
	return ok && int(s.Uid) == os.Getuid()
}
