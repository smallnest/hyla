package beacontower

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	hostHandshakeTimeout = 15 * time.Second
	hostPingInterval     = 20 * time.Second
	hostIdleTimeout      = 60 * time.Second
	hostAcceptQueueSize  = 64
)

// HostConfig configures the host-side BeaconTower listener registered on a relay.
type HostConfig struct {
	RelayAddr  string
	RelayToken string
	Keys       KeyPair
	TLSConfig  *tls.Config
	OnPocket   func(net.Conn)
	Logger     *log.Logger
}

// HostListener registers with a BeaconTower relay and accepts end-to-end encrypted,
// multiplexed streams from remote clients. It implements net.Listener semantics
// for the primary "hyla" service while dispatching "pocket" streams to OnPocket.
type HostListener struct {
	cfg         HostConfig
	relay       RelayAddr
	tokenDigest [32]byte
	replay      *ReplayCache
	log         *log.Logger

	ctx    context.Context
	cancel context.CancelFunc

	acceptCh chan net.Conn
	doneCh   chan struct{}
	once     sync.Once

	mu       sync.Mutex
	ctrlConn net.Conn
	muxes    map[*Mux]struct{}
}

// ListenHost validates cfg, establishes the initial registration with the relay,
// and starts background invite handling and auto-reconnect loops.
func ListenHost(ctx context.Context, cfg HostConfig) (*HostListener, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ra, err := ParseRelayAddr(cfg.RelayAddr)
	if err != nil {
		return nil, err
	}
	pub, err := PublicFromPrivate(cfg.Keys.Priv)
	if err != nil {
		return nil, err
	}
	if cfg.Keys.Pub != ([32]byte{}) && cfg.Keys.Pub != pub {
		return nil, errors.New("beacontower: host public key does not match private key")
	}
	cfg.Keys.Pub = pub
	var zero [32]byte
	if cfg.Keys.PSK == zero {
		return nil, errors.New("beacontower: empty preshared key")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	hCtx, hCancel := context.WithCancel(ctx)
	h := &HostListener{
		cfg:         cfg,
		relay:       ra,
		tokenDigest: TokenDigest(strings.TrimSpace(cfg.RelayToken)),
		replay:      NewReplayCache(),
		log:         logger,
		ctx:         hCtx,
		cancel:      hCancel,
		acceptCh:    make(chan net.Conn, hostAcceptQueueSize),
		doneCh:      make(chan struct{}),
		muxes:       make(map[*Mux]struct{}),
	}

	ctrl, err := h.connectAndRegister(h.ctx)
	if err != nil {
		hCancel()
		return nil, err
	}
	h.setCtrlConn(ctrl)

	watchStop := context.AfterFunc(hCtx, func() { _ = h.Close() })
	go func() {
		defer watchStop()
		h.controlLoop(ctrl)
	}()

	return h, nil
}

// Accept waits for the next authenticated "hyla" stream from any remote client.
func (h *HostListener) Accept() (net.Conn, error) {
	select {
	case c := <-h.acceptCh:
		return c, nil
	case <-h.doneCh:
		return nil, net.ErrClosed
	}
}

// Close shuts down the host listener, its relay control link, and all active tunnels.
func (h *HostListener) Close() error {
	h.once.Do(func() {
		h.cancel()
		close(h.doneCh)
		h.mu.Lock()
		ctrl := h.ctrlConn
		h.ctrlConn = nil
		muxes := make([]*Mux, 0, len(h.muxes))
		for m := range h.muxes {
			muxes = append(muxes, m)
			delete(h.muxes, m)
		}
		h.mu.Unlock()
		if ctrl != nil {
			_ = ctrl.Close()
		}
		for _, m := range muxes {
			_ = m.Close()
		}
	})
	return nil
}

// PairCode returns the self-contained beacontower://bt1_... address for this host.
func (h *HostListener) PairCode() (string, error) {
	code, err := EncodePairCode(PairCode{
		Relay:      h.relay.Raw,
		HostPub:    h.cfg.Keys.Pub,
		PSK:        h.cfg.Keys.PSK,
		RelayToken: h.cfg.RelayToken,
	})
	if err != nil {
		return "", err
	}
	return Scheme + code, nil
}

func (h *HostListener) setCtrlConn(c net.Conn) {
	h.mu.Lock()
	prev := h.ctrlConn
	if !h.isClosedLocked() {
		h.ctrlConn = c
	} else {
		h.ctrlConn = nil
	}
	closed := h.ctrlConn == nil && c != nil
	h.mu.Unlock()
	if prev != nil && prev != c {
		_ = prev.Close()
	}
	if closed {
		_ = c.Close()
	}
}

func (h *HostListener) isClosedLocked() bool {
	select {
	case <-h.doneCh:
		return true
	default:
		return false
	}
}

func (h *HostListener) isClosed() bool {
	select {
	case <-h.doneCh:
		return true
	default:
		return false
	}
}

func (h *HostListener) connectAndRegister(ctx context.Context) (net.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, hostHandshakeTimeout)
	defer cancel()

	raw, err := DialRelayWithTLS(dialCtx, h.relay.Raw, h.cfg.TLSConfig)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(dialCtx, func() { _ = raw.Close() })
	defer func() { _ = stop() }()

	relayPub, nonce, _, err := readServerHello(raw)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	proof, err := RegisterProof(h.cfg.Keys.Priv, relayPub, nonce, h.tokenDigest)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	var reg [64]byte
	copy(reg[:32], h.cfg.Keys.Pub[:])
	copy(reg[32:64], proof[:])
	if err := writeCtrlMsg(raw, msgRegister, reg[:]); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := readOKOrErr(raw); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if !stop() && dialCtx.Err() != nil {
		_ = raw.Close()
		return nil, dialCtx.Err()
	}
	return raw, nil
}

func (h *HostListener) controlLoop(initial net.Conn) {
	ctrl := initial
	wait := time.Second
	for {
		if ctrl != nil {
			h.serveControlConn(ctrl)
			ctrl = nil
		}
		if h.isClosed() {
			return
		}
		select {
		case <-h.doneCh:
			return
		case <-time.After(wait):
		}
		dialCtx, cancel := context.WithTimeout(h.ctx, hostHandshakeTimeout)
		next, err := h.connectAndRegister(dialCtx)
		cancel()
		if err != nil {
			h.log.Printf("beacontower relay reconnect failed (%s): %v", h.relay.Raw, err)
			if wait < 30*time.Second {
				wait *= 2
			}
			continue
		}
		wait = time.Second
		h.setCtrlConn(next)
		ctrl = next
	}
}

func (h *HostListener) serveControlConn(ctrl net.Conn) {
	defer ctrl.Close()

	var wmu sync.Mutex
	sendCtrl := func(typ byte, payload []byte) error {
		wmu.Lock()
		defer wmu.Unlock()
		return writeCtrlMsg(ctrl, typ, payload)
	}

	stopPing := make(chan struct{})
	defer close(stopPing)

	lastRecv := time.Now().UnixNano()
	var lastMu sync.Mutex
	touch := func() {
		lastMu.Lock()
		lastRecv = time.Now().UnixNano()
		lastMu.Unlock()
	}
	lastSeen := func() time.Time {
		lastMu.Lock()
		t := time.Unix(0, lastRecv)
		lastMu.Unlock()
		return t
	}

	go func() {
		t := time.NewTicker(hostPingInterval)
		defer t.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-h.doneCh:
				_ = ctrl.Close()
				return
			case <-t.C:
				if time.Since(lastSeen()) > hostIdleTimeout {
					_ = ctrl.Close()
					return
				}
				var p [8]byte
				binary.BigEndian.PutUint64(p[:], uint64(time.Now().UnixNano()))
				if err := sendCtrl(msgPing, p[:]); err != nil {
					_ = ctrl.Close()
					return
				}
			}
		}
	}()

	for {
		typ, payload, err := readCtrlMsg(ctrl)
		if err != nil {
			return
		}
		touch()
		switch typ {
		case msgInvite:
			if len(payload) != 16 {
				continue
			}
			var sessionID [16]byte
			copy(sessionID[:], payload)
			go h.handleInvite(sessionID)
		case msgPing:
			if err := sendCtrl(msgPong, payload); err != nil {
				return
			}
		case msgPong:
		case msgErr:
			h.log.Printf("beacontower relay error: %s", string(payload))
			return
		}
	}
}

func (h *HostListener) handleInvite(sessionID [16]byte) {
	if h.isClosed() {
		return
	}
	ctx, cancel := context.WithTimeout(h.ctx, hostHandshakeTimeout)
	defer cancel()

	raw, err := DialRelayWithTLS(ctx, h.relay.Raw, h.cfg.TLSConfig)
	if err != nil {
		return
	}
	rawGuard := context.AfterFunc(ctx, func() { _ = raw.Close() })

	_, nonce, _, err := readServerHello(raw)
	if err != nil {
		rawGuard()
		_ = raw.Close()
		return
	}
	proof := JoinTokenProof(h.tokenDigest, nonce, sessionID)
	var joinMsg [48]byte
	copy(joinMsg[:16], sessionID[:])
	copy(joinMsg[16:48], proof[:])
	if err := writeCtrlMsg(raw, msgJoin, joinMsg[:]); err != nil {
		rawGuard()
		_ = raw.Close()
		return
	}
	if err := readOKOrErr(raw); err != nil {
		rawGuard()
		_ = raw.Close()
		return
	}
	if !rawGuard() {
		_ = raw.Close()
		return
	}

	e2eConn, err := ServerE2EHandshake(ctx, raw, h.cfg.Keys, h.replay)
	if err != nil {
		h.log.Printf("beacontower e2e server handshake failed: %v", err)
		_ = raw.Close()
		return
	}

	mux := NewMux(e2eConn, false)
	h.mu.Lock()
	if h.isClosedLocked() {
		h.mu.Unlock()
		_ = mux.Close()
		return
	}
	h.muxes[mux] = struct{}{}
	h.mu.Unlock()

	defer func() {
		_ = mux.Close()
		h.mu.Lock()
		delete(h.muxes, mux)
		h.mu.Unlock()
	}()

	for {
		st, target, err := mux.AcceptStream()
		if err != nil {
			return
		}
		switch target {
		case "pocket":
			if h.cfg.OnPocket != nil {
				go h.cfg.OnPocket(st)
			} else {
				_ = st.Close()
			}
		case "", "hyla":
			select {
			case h.acceptCh <- st:
			case <-h.doneCh:
				_ = st.Close()
				return
			}
		default:
			_ = st.Close()
		}
	}
}

func readServerHello(r io.Reader) (relayPub, nonce [32]byte, flags byte, err error) {
	typ, payload, err := readCtrlMsg(r)
	if err != nil {
		return relayPub, nonce, 0, err
	}
	if typ != msgServerHello || len(payload) != 65 {
		return relayPub, nonce, 0, errors.New("beacontower: invalid SERVER_HELLO")
	}
	copy(relayPub[:], payload[:32])
	copy(nonce[:], payload[32:64])
	flags = payload[64]
	return relayPub, nonce, flags, nil
}

func readOKOrErr(r io.Reader) error {
	typ, payload, err := readCtrlMsg(r)
	if err != nil {
		return err
	}
	switch typ {
	case msgOK:
		return nil
	case msgErr:
		msg := strings.TrimSpace(string(payload))
		if msg == "" {
			msg = "relay rejected request"
		}
		return fmt.Errorf("beacontower: %s", msg)
	default:
		return fmt.Errorf("beacontower: unexpected relay reply type %d", typ)
	}
}
