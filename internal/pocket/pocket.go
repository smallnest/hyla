// Package pocket serves Hyla to HylaPocket, an Android client made from
// rexpocket (https://github.com/leizhiyuan/rexpocket). It is an http.Handler
// that hylad's tailcat listener serves on a port of its own, and it speaks
// what rexpocket's core package already reads: a control WebSocket of JSON
// methods, and a data WebSocket of a pane's terminal. Hyla's snapshot and
// frames are VT, so the client, which looks for a GHOSTSNP snapshot first
// and finds none, shows them as output.
//
// The wire is in hyla-pocket's docs/wire.md.
package pocket

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"hyla/internal/build"
	"hyla/internal/daemon"
)

// Version is the revision of the wire this package serves.
const Version = 1

// PocketPath is the prefix of the paths HylaPocket takes on a listener:
// tailcat's, with one connection behind it, and one prefix for it, so that
// the mux, which takes every other path, leaves it alone.
const PocketPath = "/pocket/"

// Paths and subprotocols, as rexpocket names them.
const (
	PathControl  = "/api/control"
	PathData     = "/api/data"
	PathHello    = "/api/hello"
	ProtoControl = "rex.control.v1"
	ProtoData    = "rex.data.terminal.v1"

	// StatusPaneClosed is the close code of a stream whose pane ended, as
	// rexpocket reads it.
	StatusPaneClosed = 4000

	// StatusUnauthorized refuses a data stream.
	StatusUnauthorized = 4001

	// ticketLife bounds a data ticket.
	ticketLife = 30 * time.Second
)

// Server serves HylaPocket's paths of the daemon, under PocketPath.
type Server struct {
	d   *daemon.Daemon
	log *log.Logger

	mu      sync.Mutex
	clients map[*control]struct{}
	tickets map[string]*ticket
	// owners are the clients that own a pane's terminal: their size is
	// the pane's. One client may own a pane, and another's request to
	// take it is refused, as rexpocket expects when it asks after a
	// confirmation (mode "ask").
	owners map[string]*control
	// sized are the sizes the clients asked with terminal.resize, the
	// last per pane; a pane whose owner went takes the last one again.
	sized map[string][4]int
	stop  bool
}

type ticket struct {
	client  *control
	session string
	pane    string
	expires time.Time
}

// init makes this the HylaPocket server of hylad's listeners.
func init() {
	daemon.SetPocket(func(d *daemon.Daemon, logger *log.Logger) daemon.PocketServer {
		return New(d, logger)
	})
}

// New makes a Server of d. A tailcat listener serves it, and its pair
// code is what admits a connection, so a request carries no token.
func New(d *daemon.Daemon, logger *log.Logger) *Server {
	return &Server{
		d: d, log: logger,
		clients: map[*control]struct{}{},
		tickets: map[string]*ticket{},
		owners:  map[string]*control{},
		sized:   map[string][4]int{},
	}
}

// Close ends every client's connection.
func (s *Server) Close() {
	s.mu.Lock()
	s.stop = true
	cs := make([]*control, 0, len(s.clients))
	for c := range s.clients {
		cs = append(cs, c)
	}
	s.mu.Unlock()
	for _, c := range cs {
		c.close(websocket.StatusGoingAway, "hylad stops")
	}
}

// PocketPath is the prefix of the paths this server serves: PocketPath.
func (s *Server) PocketPath() string { return PocketPath }

// ServeHTTP serves a request of hylad's tailcat listener, behind the
// PocketPath prefix: the pair code admitted the connection, so the request
// needs no token, and the paths are the ones rexpocket asks for.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, PocketPath) {
		http.NotFound(w, r)
		return
	}
	r2 := r.Clone(r.Context())
	u := *r.URL
	u.Path = "/" + strings.TrimPrefix(r.URL.Path, PocketPath)
	u.RawPath = ""
	r2.URL = &u
	s.route(w, r2)
}

// route answers a request that may come in.
func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case PathHello:
		h, _ := os.Hostname()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"app": "HylaPocket", "hostname": h, "version": build.Version(), "protocol": Version,
		})
	case PathControl:
		s.control(w, r)
	case PathData:
		s.data(w, r)
	default:
		http.NotFound(w, r)
	}
}

// upgrade accepts a WebSocket, and refuses a request from a web page: as
// hylad's own listeners do, so a page the user visits cannot reach their
// sessions.
func upgrade(w http.ResponseWriter, r *http.Request, subprotocol string) (*websocket.Conn, error) {
	if o := r.Header.Get("Origin"); o != "" {
		return nil, errors.New("no requests from web pages")
	}
	return websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:    []string{subprotocol},
		CompressionMode: websocket.CompressionDisabled,
	})
}

func (s *Server) control(w http.ResponseWriter, r *http.Request) {
	c, err := upgrade(w, r, ProtoControl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	cl := newControl(s, c)
	s.mu.Lock()
	stopped := s.stop
	if !stopped {
		s.clients[cl] = struct{}{}
	}
	s.mu.Unlock()
	defer s.drop(cl)
	if stopped {
		c.Close(websocket.StatusGoingAway, "hylad stops")
		return
	}
	go cl.follow()
	cl.run()
}

// drop forgets a client: the panes it owned go back to another client's
// size, or to their own.
func (s *Server) drop(c *control) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c)
	for pane, o := range s.owners {
		if o == c {
			delete(s.owners, pane)
			if size, ok := s.sized[pane]; ok {
				_ = s.d.PocketResize(c.viewed[pane], pane, size[0], size[1], size[2], size[3])
			}
		}
	}
	for k, t := range s.tickets {
		if t.client == c {
			delete(s.tickets, k)
		}
	}
	if viewed, ok := c.lastViewed(); ok && s.d.PocketViewed() == viewed {
		s.d.PocketLooks("")
	}
}

// data serves one pane's terminal: its VT snapshot, then its frames.
func (s *Server) data(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	t := s.tickets[q.Get("ticket")]
	delete(s.tickets, q.Get("ticket"))
	owner := s.owners[q.Get("block")]
	s.mu.Unlock()
	if t == nil || t.session != q.Get("session") || t.pane != q.Get("block") || time.Now().After(t.expires) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if owner != nil && owner != t.client {
		http.Error(w, "another client owns the terminal", http.StatusForbidden)
		return
	}
	stream, err := s.d.PocketStream(false, t.session, t.pane)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer stream.Close()
	c, err := upgrade(w, r, ProtoData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	defer c.CloseNow()
	ctx := r.Context()
	// The mirror's first frame shows the whole pane, as a snapshot does.
	for {
		if frame := stream.Next(); len(frame) > 0 {

			if err := c.Write(ctx, websocket.MessageBinary, frame); err != nil {

				return
			}
		}
		// A frame asked for while the last one went waits here, which its
		// wake, alone in the channel, holds.
		select {
		case <-ctx.Done():
			return
		case <-stream.Done():
			c.Close(StatusPaneClosed, "the pane closed")
			return
		case <-stream.Wake:
		}
	}
}

func newTicketID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// writeText writes one text message, then returns.
func writeText(c *websocket.Conn, m any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	ctx, cancel := contextWithTimeout(10 * time.Second)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, data)
}
