//go:build windows

package hylaplugin

import (
	"net"
	"os"
	"time"
)

type pipeConn struct {
	*os.File
}

func (pipeConn) LocalAddr() net.Addr                { return pipeAddr{} }
func (pipeConn) RemoteAddr() net.Addr               { return pipeAddr{} }
func (pipeConn) SetDeadline(time.Time) error        { return nil }
func (pipeConn) SetReadDeadline(time.Time) error    { return nil }
func (pipeConn) SetWriteDeadline(time.Time) error   { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "named-pipe" }

func dialPluginSocket(addr string) (net.Conn, error) {
	f, err := os.OpenFile(addr, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return pipeConn{File: f}, nil
}
