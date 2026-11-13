package daemon

import (
	"bufio"
	"context"
	"crypto/subtle"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"
)

// listeners are hylad's network listeners (Options.Listen, else the
// configuration's listen key, which they follow as it changes), which
// take remote connections over tailcat, admitted by a pair code. They are
// off unless asked for.
type listeners struct {
	d *Daemon

	mu sync.Mutex
	// on are the listeners asked for, by what the configuration calls
	// them.
	on     map[string]*listener
	closed bool
	// pocket is HylaPocket's server, which a tailcat listener answers
	// beside hylad's, under HylaPocket's own paths.
	pocket PocketServer
}

// listener is one of the listeners, which stops, and drops the
// connections it took, as its context ends.
type listener struct {
	spec    remote.Listen
	ctx     context.Context
	cancel  context.CancelFunc
	servers []*http.Server // listeners.mu's
	socket  string
	tailcat *remote.TailcatRelay // tailcat's
	tcp     net.Listener         // a tcp::// listener's
}

// listen starts the network listeners.
func (d *Daemon) listen() *listeners {
	l := &listeners{d: d, on: map[string]*listener{}, pocket: NewPocket(d, d.log)}
	set := settings.Load()
	remote.SetRelay(set.TailcatRelay)
	specs := d.opts.Listen
	if specs != nil {
		l.set(specs)
		return l
	}
	// The others, after hylad answers: a relay takes a while to join, and
	// the socket a client waits on answers at once.
	l.set(nil)
	go func() {
		l.set(set.Listen)
		l.follow(set)
	}()
	return l
}

// follow applies the configuration's listen as it changes, as Settings ›
// Hosts turns this machine's listener on and off.
func (l *listeners) follow(last settings.Settings) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-l.d.quit:
			return
		case <-t.C:
		}
		set := settings.Load()
		if slices.Equal(set.Listen, last.Listen) && set.TailcatRelay == last.TailcatRelay && set.TailcatRotate == last.TailcatRotate {
			continue
		}
		remote.SetRelay(set.TailcatRelay)
		last = set
		// A new key's listener, for its new pair code.
		for _, s := range set.Listen {
			if spec, err := remote.ParseListen(s); err == nil && spec.Kind == "tailcat" {
				l.drop(spec.String())
			}
		}
		l.set(set.Listen)
	}
}

// set makes the listeners those of specs, stopping the others.
func (l *listeners) set(specs []string) {
	want := map[string]remote.Listen{}
	for _, s := range specs {
		spec, err := remote.ParseListen(s)
		if err != nil {
			l.d.log.Printf("listen: %v", err)
			continue
		}
		want[spec.String()] = spec
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	for name, ln := range l.on {
		if _, ok := want[name]; !ok {
			l.d.log.Printf("no longer listening on %s", name)
			ln.stop()
			delete(l.on, name)
		}
	}
	for name, spec := range want {
		if l.on[name] != nil {
			continue
		}
		ln := &listener{spec: spec}
		ln.ctx, ln.cancel = context.WithCancel(context.Background())
		l.on[name] = ln
		if spec.Kind == "tcp" {
			go l.tcpListener(ln)
			continue
		}
		go l.tailcatListener(ln)
	}
}

// drop stops the listener named name, which set starts again as it
// starts one a configuration adds.
func (l *listeners) drop(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ln := l.on[name]; ln != nil {
		ln.stop()
		delete(l.on, name)
	}
}

// active reports whether hylad listens on the network, for remote clients
// to find it there: it then never exits for being idle.
func (l *listeners) active() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.on) > 0
}

func (l *listeners) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	for _, ln := range l.on {
		ln.stop()
	}
	if l.pocket != nil {
		l.pocket.Close()
	}
}

// stop stops a listener, and the connections it took; listeners.mu is
// held.
func (ln *listener) stop() {
	ln.cancel()
	for _, s := range ln.servers {
		s.Close()
	}

	if ln.tailcat != nil {
		ln.tailcat.Close()
	}
	if ln.socket != "" {
		os.Remove(ln.socket)
	}
	if ln.tcp != nil {
		ln.tcp.Close()
	}
}

// tailcatListener listens through a tailcat relay, whose pair code is the
// listener's address: the key that makes it stays in hyla's directory, as
// the code is. A relay that cannot be joined, as one the network had not
// reached when hylad started, is tried again, with a longer wait each
// time, so that a listener the user turned on does not stay off until the
// settings change again.
func (l *listeners) tailcatListener(lis *listener) {
	wait := time.Second
	for {
		err := l.joinTailcat(lis)
		if err == nil || lis.ctx.Err() != nil {
			return
		}
		l.d.log.Printf("listen %s: %v: trying again in %s", lis.spec, err, wait)
		select {
		case <-lis.ctx.Done():
			return
		case <-time.After(wait):
		}
		if wait < time.Minute {
			wait *= 2
		}
	}
}

// joinTailcat starts the listener's relay, once, and keeps it.
func (l *listeners) joinTailcat(lis *listener) error {
	ctx, cancel := context.WithTimeout(lis.ctx, time.Minute)
	defer cancel()
	// HylaPocket is the server's other port: a phone opens one beside
	// Hyla's own connection, and the pair code admitted it (pocket.go).
	if l.pocket != nil {
		mux := http.NewServeMux()
		mux.Handle(l.pocket.PocketPath(), l.pocket)
		remote.ServePocket(lis.spec.PocketPort, mux)
	}
	r, err := remote.ListenTailcat(ctx, filepath.Join(proto.Dir(), "tailcat.json"), lis.spec.Port)
	if err != nil {
		return err
	}
	l.mu.Lock()
	if lis.ctx.Err() != nil {
		l.mu.Unlock()
		r.Close()
		return nil
	}
	lis.tailcat = r
	lis.spec.Tailcat = strings.TrimPrefix(r.PairCode, "tailcat://")
	l.mu.Unlock()
	l.d.log.Printf("listening on %s (tailcat)", r.PairCode)
	return nil
}

// tcpListener listens on this machine's network, at the address the
// configuration names: the password of its listeners admits every
// connection, which Hyla and HylaPocket ask for by the address alone.
func (l *listeners) tcpListener(lis *listener) {
	addr := net.JoinHostPort(lis.spec.Host, lis.spec.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		l.d.log.Printf("listen %s: %v", lis.spec, err)
		return
	}
	l.mu.Lock()
	if lis.ctx.Err() != nil {
		l.mu.Unlock()
		ln.Close()
		return
	}
	lis.tcp = ln
	l.mu.Unlock()
	l.d.log.Printf("listening on %s (tcp)", addr)
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go l.serveTCP(c)
	}
}

// serveTCP takes a connection of a tcp:// listener: the password first,
// then Hyla's own protocol, or, as a client of HylaPocket asks, HTTP.
func (l *listeners) serveTCP(c net.Conn) {
	r := bufio.NewReaderSize(c, 64<<10)
	c.SetReadDeadline(time.Now().Add(15 * time.Second))
	line, err := r.ReadString('\n')
	if err != nil {
		c.Close()
		return
	}
	c.SetReadDeadline(time.Time{})
	if looksHTTP(line) {
		l.serveHTTP(c, r, line)
		return
	}
	if !l.password(strings.TrimSpace(line)) {
		l.d.log.Printf("refused %s: no password", c.RemoteAddr())
		c.Close()
		return
	}
	hello, err := proto.ReadLine(r, 64<<10)
	if err != nil {
		c.Close()
		return
	}
	l.d.serveHello(c, r, hello, identity{peer: "the password", via: "tcp"})
}

// password reports whether s is the password of the configuration's
// listeners, as listen-token sets it, in constant time.
func (l *listeners) password(s string) bool {
	want := settings.Load().ListenToken
	if want == "" || s == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(s), []byte(want)) == 1
}

// looksHTTP tells an HTTP request line, as HylaPocket's connections begin,
// from the password of Hyla's own protocol.
func looksHTTP(line string) bool {
	for _, m := range []string{"GET ", "POST ", "PUT ", "HEAD ", "OPTIONS ", "DELETE ", "PATCH "} {
		if strings.HasPrefix(line, m) {
			return true
		}
	}
	return false
}

// serveHTTP answers a connection of HylaPocket's, whose request line came
// first, line, with the password it carries.
func (l *listeners) serveHTTP(c net.Conn, r *bufio.Reader, line string) {
	conn := &readConn{Conn: c, r: io.MultiReader(strings.NewReader(line), r)}
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !l.token(req) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="hyla"`)
			http.Error(w, "this is Hyla's: the password admits you", http.StatusUnauthorized)
			return
		}
		if l.pocket == nil || !strings.HasPrefix(req.URL.Path, l.pocket.PocketPath()) {
			http.NotFound(w, req)
			return
		}
		l.pocket.ServeHTTP(w, req)
	})
	one := &oneConn{conn: conn, addr: c.LocalAddr()}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 15 * time.Second, ErrorLog: l.d.log}
	srv.Serve(one)
}

// token reports whether a request carries the password, as HylaPocket
// sends it: Authorization: Bearer.
func (l *listeners) token(r *http.Request) bool {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return false
	}
	return l.password(strings.TrimSpace(h[len(prefix):]))
}

// readConn is a connection whose first bytes were read already.
type readConn struct {
	net.Conn
	r io.Reader
}

func (c *readConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// oneConn is a listener of one connection, for the HTTP server of a
// HylaPocket connection.
type oneConn struct {
	conn net.Conn
	addr net.Addr
}

func (o *oneConn) Accept() (net.Conn, error) {
	if o.conn == nil {
		return nil, io.EOF
	}
	c := o.conn
	o.conn = nil
	return c, nil
}

func (o *oneConn) Close() error   { return nil }
func (o *oneConn) Addr() net.Addr { return o.addr }
