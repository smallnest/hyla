package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/tailscale/tailcat"
	"tailscale.com/wgengine/filter"
)

// A tailcat listener (tailcat.go) for hylad: a Tailcat server that hylad's
// remote server answers the clients of, as it answers an http(s)://
// listener's, at the listener's pair code as its address.

// TailcatRelay is hylad's tailcat listener.
type TailcatRelay struct {
	*TailcatServer
	ln net.Listener
}

// TailcatServer is a Tailcat server with hylad's remote server on one of
// its ports.
type TailcatServer struct {
	*tailcat.Server
	PairCode string // hylad's address, tailcat:// and the pair code
}

// ServePocket makes pocket the handler a tailcat server's other port
// serves, at port: a phone opens that port beside Hyla, and has no Hyla
// connection to open it over. Every other port, and port itself when
// pocket is nil, answers nothing.
func ServePocket(port string, pocket http.Handler) {
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		n = 7297
	}
	pocketPort, pocketHandler = uint16(n), pocket
}

var (
	pocketPort    uint16
	pocketHandler http.Handler
)

// servePocket answers a connection of the server's other port: one HTTP
// connection of HylaPocket's, which ends as it does.
func servePocket(conn net.Conn) {
	srv := &http.Server{
		Handler:           pocketHandler,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
		ConnState: func(_ net.Conn, st http.ConnState) {
			if st == http.StateHijacked || st == http.StateClosed {
				conn.Close()
			}
		},
	}
	_ = srv.Serve(&oneConnListener{conn: conn})
}

// oneConnListener answers one connection, and then none: http.Server
// takes it as it takes a listener's.
type oneConnListener struct {
	conn net.Conn
	done bool
}

func (l *oneConnListener) Accept() (net.Conn, error) {
	if l.done {
		return nil, io.EOF
	}
	l.done = true
	return l.conn, nil
}

func (l *oneConnListener) Close() error   { return nil }
func (l *oneConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// ListenTailcat starts hylad's tailcat listener on port for the key at
// path (TailcatKeyAt), whose pair code it makes, and answers the
// connections it takes with srv, within ctx. Hylad is "hyla", for
// whomever lists the clients.
func ListenTailcat(ctx context.Context, path, port string, srv *Server) (*TailcatRelay, error) {
	k, err := TailcatKeyAt(ctx, path)
	if err != nil {
		return nil, err
	}
	code, err := TailcatAddress(ctx, k)
	if err != nil {
		return nil, err
	}
	// The region's map, which the code's region came from: the server
	// starts at that region.
	dm, err := tailcat.FetchDERPMap(ctx, tailcat.DERPMapURL(relayURL()))
	if err != nil {
		return nil, fmt.Errorf("the tailcat relay: %w", err)
	}
	ci, err := tailcat.ParseAddr(tailcat.Addr(code))
	if err != nil {
		return nil, fmt.Errorf("tailcat: %w", err)
	}
	if len(ci.Region) == 0 {
		return nil, errors.New("tailcat: the pair code names no relay")
	}
	region := relayRegion(dm, k.Region)
	if region == nil {
		return nil, fmt.Errorf("tailcat: region %s is not in %s", k.Region, relayURL())
	}
	priv := &tailcat.PrivateKey{Public: ci}
	if err := priv.Private.UnmarshalText([]byte(k.Key)); err != nil {
		return nil, fmt.Errorf("tailcat key: %w", err)
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return nil, fmt.Errorf("tailcat: %q is no port", port)
	}
	s := &tailcat.Server{
		Key:          priv.Private,
		PresharedKey: priv.Public.PresharedKey,
		Region:       region,
		// The relay map, which names the clients' region.
		DERPMapURL: relayURL(),
		// Every other port is HylaPocket's, which a phone opens beside
		// the mux: hylad says so with ServePocketPort (pocket.go).
		OnTCP: func(port uint16) func(net.Conn) {
			if port == pocketPort && pocketHandler != nil {
				return servePocket
			}
			return func(c net.Conn) { c.Close() }
		},
		// The port among them, as the filter admits the ports a server
		// serves and not ONTCP's every one.
		ServedTCPPorts: []filter.PortRange{{First: pocketPort, Last: pocketPort}},
	}
	ln, err := s.Listen(ctx, "tcp", ":"+strconv.FormatUint(n, 10))
	if err != nil {
		s.Close()
		return nil, err
	}
	// Hylad says who connects: the pair code tells a client's key.
	srv.Peer = func(c net.Conn, hello []byte) string { return peerName(hello) }
	r := &TailcatRelay{TailcatServer: &TailcatServer{Server: s, PairCode: "tailcat://" + code}, ln: ln}
	go r.serve(srv)
	return r, nil
}

func (r *TailcatRelay) serve(srv *Server) {
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		go srv.Serve(c)
	}
}

// Close stops the listener, and the server with it.
func (r *TailcatRelay) Close() {
	r.ln.Close()
	r.Server.Close()
}

// peerName names a client, from its Hello, hello: "tailcat [vzc7…]",
// once it has said its key.
func peerName(hello []byte) string {
	if name, ok := peerOf(hello); ok {
		return name
	}
	return "tailcat"
}
