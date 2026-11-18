//go:build unix

package daemonhost

import (
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"path/filepath"
)

func listenPluginSocket(id, dataDir, hostKind string) (net.Listener, string, func(), error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, "", nil, err
	}
	sockPath := filepath.Join(dataDir, fmt.Sprintf("%s.sock", hostKind))
	if len(sockPath) > 96 {
		h := fnv.New32a()
		_, _ = h.Write([]byte(dataDir + "/" + hostKind))
		sockPath = filepath.Join(os.TempDir(), fmt.Sprintf("hyla-plg-%d-%08x.sock", os.Getuid(), h.Sum32()))
	}
	_ = os.Remove(sockPath)
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, "", nil, err
	}
	_ = os.Chmod(sockPath, 0o600)
	cleanup := func() {
		_ = ln.Close()
		_ = os.Remove(sockPath)
	}
	return ln, sockPath, cleanup, nil
}
