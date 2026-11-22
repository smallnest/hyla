package beacontower

import (
	"context"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	defaultMaxHosts         = 1024
	defaultMaxSessions      = 4096
	defaultMaxPendingInvite = 64
	relayHelloTimeout       = 15 * time.Second
	relayInviteTTL          = 15 * time.Second
	relayPingInterval       = 20 * time.Second
	relayIdleTimeout        = 60 * time.Second
)

// ServerConfig configures a BeaconTower relay server.
type ServerConfig struct {
	ListenAddr  string
	Token       string
	TLSCert     string
	TLSKey      string
	TLSConfig   *tls.Config
	WSPath      string
	MaxHosts    int
	MaxSessions int
	Logger      *log.Logger
}

type registeredHost struct {
	id           [16]byte
	conn         net.Conn
	wmu          sync.Mutex
	closed       atomic.Bool
	lastActivity atomic.Int64
	pendingCount atomic.Int32
}

func (h *registeredHost) send(typ byte, payload []byte) error {
	if h.closed.Load() {
		return net.ErrClosed
	}
	h.wmu.Lock()
	defer h.wmu.Unlock()
	if h.closed.Load() {
		return net.ErrClosed
	}
	return writeCtrlMsg(h.conn, typ, payload)
}

func (h *registeredHost) close() {
	if h.closed.Swap(true) {
		return
	}
	_ = h.conn.Close()
}

type pendingSession struct {
	readyCh  chan net.Conn // unbuffered: ownership passes synchronously on pair
	cancelCh chan struct{}
}

// Server is a zero-trust encrypted-traffic relay for BeaconTower.
// It multiplexes native binary TCP/TLS connections and HTTP/WebSocket traffic
// on a single port, authenticates peers without seeing plaintext tokens or E2EE keys,
// and bridges paired sessions via buffered io.CopyBuffer splices.
type Server struct {
	cfg           ServerConfig
	tokenRequired bool
	tokenDigest   [32]byte
	wsPath        string
	maxHosts      int
	maxSessions   int
	log           *log.Logger
	httpHandler   http.Handler

	mu               sync.Mutex
	ln               net.Listener
	hosts            map[[16]byte]*registeredHost
	pending          map[[16]byte]*pendingSession
	conns            map[net.Conn]struct{}
	reservedSessions int
	closed           bool

	activeSessions atomic.Int64
	copyBufPool    sync.Pool
	doneCh         chan struct{}
	closeOnce      sync.Once
}

// NewServer constructs a BeaconTower relay server from cfg.
func NewServer(cfg ServerConfig) *Server {
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		cfg.ListenAddr = ":" + DefaultPort
	}
	wsPath := strings.TrimSpace(cfg.WSPath)
	if wsPath == "" {
		wsPath = DefaultWSPath
	}
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}
	maxHosts := cfg.MaxHosts
	if maxHosts <= 0 {
		maxHosts = defaultMaxHosts
	}
	maxSessions := cfg.MaxSessions
	if maxSessions <= 0 {
		maxSessions = defaultMaxSessions
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	tok := strings.TrimSpace(cfg.Token)
	s := &Server{
		cfg:           cfg,
		tokenRequired: tok != "",
		tokenDigest:   TokenDigest(tok),
		wsPath:        wsPath,
		maxHosts:      maxHosts,
		maxSessions:   maxSessions,
		log:           logger,
		hosts:         make(map[[16]byte]*registeredHost),
		pending:       make(map[[16]byte]*pendingSession),
		conns:         make(map[net.Conn]struct{}),
		doneCh:        make(chan struct{}),
		copyBufPool: sync.Pool{
			New: func() any {
				b := make([]byte, 32<<10)
				return &b
			},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		hosts, sessions := s.Stats()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"hosts":    hosts,
			"sessions": sessions,
		})
	})
	mux.HandleFunc(s.wsPath, func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
			CompressionMode:    websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		ws.SetReadLimit(128 << 10)
		netConn := websocket.NetConn(context.Background(), ws, websocket.MessageBinary)
		s.handlePeer(netConn)
	})
	s.httpHandler = mux
	return s
}

// ListenAndServe binds cfg.ListenAddr (with optional TLS) and serves until Close or ctx cancels.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}
	tlsCfg, err := s.resolveTLSConfig()
	if err != nil {
		_ = ln.Close()
		return err
	}
	if tlsCfg != nil {
		ln = tls.NewListener(ln, tlsCfg)
	}
	if ctx != nil {
		stop := context.AfterFunc(ctx, func() { _ = s.Close() })
		defer stop()
	}
	return s.Serve(ln)
}

func (s *Server) resolveTLSConfig() (*tls.Config, error) {
	if s.cfg.TLSConfig != nil {
		cfg := s.cfg.TLSConfig.Clone()
		if cfg.MinVersion == 0 {
			cfg.MinVersion = tls.VersionTLS13
		}
		return cfg, nil
	}
	if s.cfg.TLSCert != "" || s.cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(s.cfg.TLSCert, s.cfg.TLSKey)
		if err != nil {
			return nil, err
		}
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}, nil
	}
	return nil, nil
}

// Serve accepts connections from ln until Close is called.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = ln.Close()
		return net.ErrClosed
	}
	s.ln = ln
	s.mu.Unlock()

	for {
		c, err := ln.Accept()
		if err != nil {
			select {
			case <-s.doneCh:
				return nil
			default:
				return err
			}
		}
		if tc, ok := c.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
		}
		go s.handleConn(c)
	}
}

// Close shuts down the relay server, registered host control links, and active sessions.
func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.doneCh)
		s.mu.Lock()
		s.closed = true
		ln := s.ln
		hosts := make([]*registeredHost, 0, len(s.hosts))
		for id, h := range s.hosts {
			hosts = append(hosts, h)
			delete(s.hosts, id)
		}
		conns := make([]net.Conn, 0, len(s.conns))
		for c := range s.conns {
			conns = append(conns, c)
			delete(s.conns, c)
		}
		s.mu.Unlock()
		for _, h := range hosts {
			h.close()
		}
		for _, c := range conns {
			_ = c.Close()
		}
		if ln != nil {
			err = ln.Close()
		}
	})
	return err
}

func (s *Server) trackConn(c net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[c] = struct{}{}
	return true
}

func (s *Server) untrackConn(c net.Conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

// Stats returns the current number of registered hosts and active bridged sessions.
func (s *Server) Stats() (hosts, sessions int) {
	s.mu.Lock()
	hosts = len(s.hosts)
	s.mu.Unlock()
	sessions = int(s.activeSessions.Load())
	return hosts, sessions
}

func (s *Server) handleConn(raw net.Conn) {
	var head [4]byte
	_ = raw.SetReadDeadline(time.Now().Add(relayHelloTimeout))
	if _, err := io.ReadFull(raw, head[:]); err != nil {
		_ = raw.Close()
		return
	}
	_ = raw.SetReadDeadline(time.Time{})

	if head == NativeMagic {
		s.handlePeer(raw)
		return
	}

	if looksLikeHTTPMethod(head) {
		s.serveHTTPConn(&prefixConn{Conn: raw, prefix: head[:]})
		return
	}

	_ = raw.Close()
}

func looksLikeHTTPMethod(head [4]byte) bool {
	switch string(head[:]) {
	case "GET ", "HEAD", "POST", "PUT ", "OPTI":
		return true
	}
	return false
}

func (s *Server) serveHTTPConn(c net.Conn) {
	if !s.trackConn(c) {
		_ = c.Close()
		return
	}
	srv := &http.Server{
		Handler:           s.httpHandler,
		ReadHeaderTimeout: relayHelloTimeout,
		ErrorLog:          log.New(io.Discard, "", 0),
		ConnState: func(conn net.Conn, st http.ConnState) {
			if st == http.StateClosed || st == http.StateHijacked {
				s.untrackConn(conn)
			}
		},
	}
	srv.SetKeepAlivesEnabled(false)
	_ = srv.Serve(&singleConnListener{conn: c, addr: c.LocalAddr()})
}

func (s *Server) handlePeer(conn net.Conn) {
	// Guard the initial hello + request against slowloris without using
	// SetReadDeadline (which closes websocket.NetConn irreversibly if hit).
	timer := time.AfterFunc(relayHelloTimeout, func() { _ = conn.Close() })

	ephPriv, ephPub, nonce, err := newHelloChallenge()
	if err != nil {
		timer.Stop()
		_ = conn.Close()
		return
	}
	var hello [65]byte
	copy(hello[:32], ephPub[:])
	copy(hello[32:64], nonce[:])
	if s.tokenRequired {
		hello[64] = flagTokenRequired
	}
	if err := writeCtrlMsg(conn, msgServerHello, hello[:]); err != nil {
		timer.Stop()
		_ = conn.Close()
		return
	}

	typ, payload, err := readCtrlMsg(conn)
	timer.Stop()
	if err != nil {
		_ = conn.Close()
		return
	}

	switch typ {
	case msgRegister:
		s.handleRegister(conn, ephPriv, nonce, payload)
	case msgConnect:
		s.handleConnect(conn, nonce, payload)
	case msgJoin:
		s.handleJoin(conn, nonce, payload)
	default:
		_ = writeCtrlMsg(conn, msgErr, []byte("unexpected initial message"))
		_ = conn.Close()
	}
}

func newHelloChallenge() (priv, pub, nonce [32]byte, err error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return priv, pub, nonce, err
	}
	copy(priv[:], k.Bytes())
	copy(pub[:], k.PublicKey().Bytes())
	if _, err := rand.Read(nonce[:]); err != nil {
		return priv, pub, nonce, err
	}
	return priv, pub, nonce, nil
}

func (s *Server) handleRegister(conn net.Conn, relayEphPriv, nonce [32]byte, payload []byte) {
	if len(payload) != 64 {
		_ = writeCtrlMsg(conn, msgErr, []byte("invalid REGISTER payload"))
		_ = conn.Close()
		return
	}
	var hostPub, proof [32]byte
	copy(hostPub[:], payload[:32])
	copy(proof[:], payload[32:64])
	if !VerifyRegisterProof(relayEphPriv, hostPub, nonce, s.tokenDigest, proof) {
		_ = writeCtrlMsg(conn, msgErr, []byte("host authentication failed"))
		_ = conn.Close()
		return
	}

	hostID := HostIDFromPub(hostPub)
	entry := &registeredHost{id: hostID, conn: conn}
	entry.lastActivity.Store(time.Now().UnixNano())

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	prev := s.hosts[hostID]
	if prev == nil && len(s.hosts) >= s.maxHosts {
		s.mu.Unlock()
		_ = writeCtrlMsg(conn, msgErr, []byte("relay host capacity reached"))
		_ = conn.Close()
		return
	}
	s.hosts[hostID] = entry
	s.mu.Unlock()

	if prev != nil {
		prev.close()
	}

	if err := entry.send(msgOK, nil); err != nil {
		s.unregisterHost(entry)
		entry.close()
		return
	}

	s.runHostControl(entry)
}

func (s *Server) unregisterHost(entry *registeredHost) {
	s.mu.Lock()
	if cur, ok := s.hosts[entry.id]; ok && cur == entry {
		delete(s.hosts, entry.id)
	}
	s.mu.Unlock()
}

func (s *Server) runHostControl(entry *registeredHost) {
	defer func() {
		s.unregisterHost(entry)
		entry.close()
	}()

	stopKeepalive := make(chan struct{})
	defer close(stopKeepalive)

	// Separate idle watchdog from ping writes so a wedged send buffer cannot
	// prevent idle disconnection.
	go func() {
		t := time.NewTicker(relayPingInterval)
		defer t.Stop()
		for {
			select {
			case <-stopKeepalive:
				return
			case <-s.doneCh:
				entry.close()
				return
			case <-t.C:
				last := time.Unix(0, entry.lastActivity.Load())
				if time.Since(last) > relayIdleTimeout {
					entry.close()
					return
				}
			}
		}
	}()

	go func() {
		t := time.NewTicker(relayPingInterval)
		defer t.Stop()
		for {
			select {
			case <-stopKeepalive:
				return
			case <-s.doneCh:
				return
			case <-t.C:
				var p [8]byte
				binary.BigEndian.PutUint64(p[:], uint64(time.Now().UnixNano()))
				if err := entry.send(msgPing, p[:]); err != nil {
					entry.close()
					return
				}
			}
		}
	}()

	for {
		typ, payload, err := readCtrlMsg(entry.conn)
		if err != nil {
			return
		}
		entry.lastActivity.Store(time.Now().UnixNano())
		switch typ {
		case msgPing:
			if err := entry.send(msgPong, payload); err != nil {
				return
			}
		case msgPong:
		default:
			return
		}
	}
}

func (s *Server) handleConnect(clientConn net.Conn, nonce [32]byte, payload []byte) {
	if len(payload) != 48 {
		_ = writeCtrlMsg(clientConn, msgErr, []byte("invalid CONNECT payload"))
		_ = clientConn.Close()
		return
	}
	var hostID [16]byte
	var tokenProof [32]byte
	copy(hostID[:], payload[:16])
	copy(tokenProof[:], payload[16:48])

	if s.tokenRequired {
		want := ClientTokenProof(s.tokenDigest, nonce, hostID)
		if !hmac.Equal(want[:], tokenProof[:]) {
			_ = writeCtrlMsg(clientConn, msgErr, []byte("invalid relay token"))
			_ = clientConn.Close()
			return
		}
	}

	if !s.trackConn(clientConn) {
		_ = clientConn.Close()
		return
	}
	defer s.untrackConn(clientConn)

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = clientConn.Close()
		return
	}
	if s.reservedSessions >= s.maxSessions {
		s.mu.Unlock()
		_ = writeCtrlMsg(clientConn, msgErr, []byte("relay session capacity reached"))
		_ = clientConn.Close()
		return
	}
	hostEntry := s.hosts[hostID]
	if hostEntry == nil || hostEntry.closed.Load() {
		s.mu.Unlock()
		_ = writeCtrlMsg(clientConn, msgErr, []byte("target host offline"))
		_ = clientConn.Close()
		return
	}
	s.reservedSessions++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.reservedSessions--
		s.mu.Unlock()
	}()

	if hostEntry.pendingCount.Add(1) > defaultMaxPendingInvite {
		hostEntry.pendingCount.Add(-1)
		_ = writeCtrlMsg(clientConn, msgErr, []byte("too many pending invites for host"))
		_ = clientConn.Close()
		return
	}
	defer hostEntry.pendingCount.Add(-1)

	var sessionID [16]byte
	if _, err := rand.Read(sessionID[:]); err != nil {
		_ = clientConn.Close()
		return
	}

	pend := &pendingSession{
		readyCh:  make(chan net.Conn),
		cancelCh: make(chan struct{}),
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = clientConn.Close()
		return
	}
	s.pending[sessionID] = pend
	s.mu.Unlock()

	cleanupPending := func() {
		s.mu.Lock()
		delete(s.pending, sessionID)
		s.mu.Unlock()
	}

	if err := hostEntry.send(msgInvite, sessionID[:]); err != nil {
		cleanupPending()
		close(pend.cancelCh)
		_ = writeCtrlMsg(clientConn, msgErr, []byte("host unreachable"))
		_ = clientConn.Close()
		return
	}

	var hostConn net.Conn
	timer := time.NewTimer(relayInviteTTL)
	select {
	case hostConn = <-pend.readyCh:
		timer.Stop()
		cleanupPending()
	case <-timer.C:
		cleanupPending()
		close(pend.cancelCh)
		_ = writeCtrlMsg(clientConn, msgErr, []byte("timed out waiting for host"))
		_ = clientConn.Close()
		return
	case <-s.doneCh:
		timer.Stop()
		cleanupPending()
		close(pend.cancelCh)
		_ = clientConn.Close()
		return
	}

	if !s.trackConn(hostConn) {
		_ = hostConn.Close()
		_ = clientConn.Close()
		return
	}
	defer s.untrackConn(hostConn)

	if err := writeCtrlMsg(hostConn, msgOK, nil); err != nil {
		_ = hostConn.Close()
		_ = writeCtrlMsg(clientConn, msgErr, []byte("host stream failed"))
		_ = clientConn.Close()
		return
	}
	if err := writeCtrlMsg(clientConn, msgOK, nil); err != nil {
		_ = hostConn.Close()
		_ = clientConn.Close()
		return
	}

	s.activeSessions.Add(1)
	s.splice(clientConn, hostConn)
	s.activeSessions.Add(-1)
}

func (s *Server) handleJoin(hostConn net.Conn, nonce [32]byte, payload []byte) {
	if len(payload) != 48 {
		_ = writeCtrlMsg(hostConn, msgErr, []byte("invalid JOIN payload"))
		_ = hostConn.Close()
		return
	}
	var sessionID [16]byte
	var tokenProof [32]byte
	copy(sessionID[:], payload[:16])
	copy(tokenProof[:], payload[16:48])

	if s.tokenRequired {
		want := JoinTokenProof(s.tokenDigest, nonce, sessionID)
		if !hmac.Equal(want[:], tokenProof[:]) {
			_ = writeCtrlMsg(hostConn, msgErr, []byte("invalid relay token"))
			_ = hostConn.Close()
			return
		}
	}

	s.mu.Lock()
	pend, ok := s.pending[sessionID]
	if ok {
		delete(s.pending, sessionID)
	}
	s.mu.Unlock()

	if !ok {
		_ = writeCtrlMsg(hostConn, msgErr, []byte("unknown or expired session"))
		_ = hostConn.Close()
		return
	}

	select {
	case pend.readyCh <- hostConn:
		// Ownership of hostConn passed synchronously to handleConnect.
	case <-pend.cancelCh:
		_ = writeCtrlMsg(hostConn, msgErr, []byte("session expired"))
		_ = hostConn.Close()
	case <-s.doneCh:
		_ = hostConn.Close()
	}
}

func (s *Server) splice(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	copySide := func(dst, src net.Conn) {
		defer wg.Done()
		bufPtr := s.copyBufPool.Get().(*[]byte)
		_, _ = io.CopyBuffer(dst, src, *bufPtr)
		s.copyBufPool.Put(bufPtr)
		_ = dst.Close()
		_ = src.Close()
	}
	go copySide(a, b)
	go copySide(b, a)
	wg.Wait()
}

type singleConnListener struct {
	conn net.Conn
	addr net.Addr
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	if l.conn == nil {
		return nil, io.EOF
	}
	c := l.conn
	l.conn = nil
	return c, nil
}

func (l *singleConnListener) Close() error   { return nil }
func (l *singleConnListener) Addr() net.Addr { return l.addr }
