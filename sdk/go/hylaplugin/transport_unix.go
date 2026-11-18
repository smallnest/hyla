//go:build !windows

package hylaplugin

import "net"

func dialPluginSocket(addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
}
