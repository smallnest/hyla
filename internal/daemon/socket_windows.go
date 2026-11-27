package daemon

import (
	"net"
	"os"
	"time"
)

// listenSocket listens on a Unix socket, in a directory of the user's
// profile, which other users cannot reach.
func listenSocket(path string) (net.Listener, error) { return net.Listen("unix", path) }

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

// peerUID is unknown on Windows: -1.
func peerUID(net.Conn) int { return -1 }

// ownedByUser: the temporary directory on Windows is the user's.
func ownedByUser(os.FileInfo) bool { return true }
