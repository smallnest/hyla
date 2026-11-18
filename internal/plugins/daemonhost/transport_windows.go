//go:build windows

package daemonhost

import (
	"fmt"
	"hash/fnv"
	"net"

	"github.com/tailscale/go-winio"
)

func listenPluginSocket(id, dataDir, hostKind string) (net.Listener, string, func(), error) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(dataDir + "/" + id + "/" + hostKind))
	pipeName := fmt.Sprintf(`\\.\pipe\hyla-plugin-%s-%08x`, hostKind, h.Sum32())
	ln, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		return nil, "", nil, err
	}
	cleanup := func() {
		_ = ln.Close()
	}
	return ln, pipeName, cleanup, nil
}
