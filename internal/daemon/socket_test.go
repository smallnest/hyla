//go:build !windows

package daemon

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// A socket a hylad listens on is answered, and one a killed hylad left, or
// none at all, is not: hylad takes only the second, leaving a daemon that
// runs with its path.
func TestAnswered(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "hylad.sock")

	if answered(sock) {
		t.Error("a socket that was never made is answered")
	}

	stale := filepath.Join(dir, "stale.sock")
	if err := os.WriteFile(stale, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if answered(stale) {
		t.Error("a socket file nothing listens on is answered")
	}

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if !answered(sock) {
		t.Error("a socket a listener waits on is not answered")
	}
}
