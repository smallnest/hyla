package beacontower

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// DialRelay connects to a BeaconTower relay using the transport specified by
// rawAddr (host:port, tcp://, tls://, ws://, or wss://).
func DialRelay(ctx context.Context, rawAddr string) (net.Conn, error) {
	return DialRelayWithTLS(ctx, rawAddr, nil)
}

// DialRelayWithTLS is like DialRelay but applies tlsCfg when dialing tls:// or wss://.
func DialRelayWithTLS(ctx context.Context, rawAddr string, tlsCfg *tls.Config) (net.Conn, error) {
	addr, err := ParseRelayAddr(rawAddr)
	if err != nil {
		return nil, err
	}
	switch addr.Scheme {
	case "tcp":
		return dialNativeTCP(ctx, addr)
	case "tls":
		return dialNativeTLS(ctx, addr, tlsCfg)
	case "ws", "wss":
		return dialWebSocket(ctx, addr, tlsCfg)
	default:
		return nil, fmt.Errorf("beacontower: unsupported scheme %q", addr.Scheme)
	}
}

func dialNativeTCP(ctx context.Context, addr RelayAddr) (net.Conn, error) {
	d := net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	c, err := d.DialContext(ctx, "tcp", addr.AddrPort())
	if err != nil {
		return nil, err
	}
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	if err := writeMagic(ctx, c); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func dialNativeTLS(ctx context.Context, addr RelayAddr, tlsCfg *tls.Config) (net.Conn, error) {
	d := net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	raw, err := d.DialContext(ctx, "tcp", addr.AddrPort())
	if err != nil {
		return nil, err
	}
	if tc, ok := raw.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	var cfg *tls.Config
	if tlsCfg != nil {
		cfg = tlsCfg.Clone()
	} else {
		cfg = &tls.Config{}
	}
	if cfg.ServerName == "" {
		cfg.ServerName = addr.Host
	}
	if cfg.MinVersion == 0 {
		cfg.MinVersion = tls.VersionTLS13
	}
	tc := tls.Client(raw, cfg)
	if err := tc.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := writeMagic(ctx, tc); err != nil {
		_ = tc.Close()
		return nil, err
	}
	return tc, nil
}

func dialWebSocket(ctx context.Context, addr RelayAddr, tlsCfg *tls.Config) (net.Conn, error) {
	opts := &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	}
	if tlsCfg != nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = tlsCfg.Clone()
		opts.HTTPClient = &http.Client{Transport: tr}
	}
	ws, resp, err := websocket.Dial(ctx, addr.WSURL(), opts)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	// AEAD records reach 4 + 32 KiB + 16 = 32,788 bytes, exceeding coder/websocket's 32 KiB default.
	ws.SetReadLimit(128 << 10)
	// Pass context.Background() because websocket.NetConn binds the lifetime
	// of the connection to the supplied context, whereas dial ctx ends soon.
	return websocket.NetConn(context.Background(), ws, websocket.MessageBinary), nil
}

func writeMagic(ctx context.Context, c net.Conn) error {
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer func() { _ = stop() }()
	_, err := c.Write(NativeMagic[:])
	if err != nil && !stop() && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// prefixConn replays bytes already peeked from Conn before delegating to Conn.
type prefixConn struct {
	net.Conn
	prefix []byte
}

func (c *prefixConn) Read(p []byte) (int, error) {
	if len(c.prefix) > 0 {
		n := copy(p, c.prefix)
		c.prefix = c.prefix[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}
