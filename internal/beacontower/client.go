package beacontower

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

const clientConnectTimeout = 20 * time.Second

type dialCall struct {
	done chan struct{}
	mux  *Mux
	err  error
}

// ClientPool maintains persistent E2EE multiplexed tunnels keyed by BeaconTower
// pair code so control RPCs and multiple terminal panes share one encrypted relay tunnel.
type ClientPool struct {
	tlsConfig *tls.Config

	mu       sync.Mutex
	conns    map[string]*Mux
	inflight map[string]*dialCall
}

// DefaultPool is the process-wide shared BeaconTower tunnel pool.
var DefaultPool = NewClientPool()

// NewClientPool creates an empty tunnel pool.
func NewClientPool() *ClientPool {
	return &ClientPool{
		conns:    make(map[string]*Mux),
		inflight: make(map[string]*dialCall),
	}
}

// NewClientPoolWithTLS creates a tunnel pool applying custom TLS settings (e.g. custom CAs).
func NewClientPoolWithTLS(tlsCfg *tls.Config) *ClientPool {
	p := NewClientPool()
	if tlsCfg != nil {
		p.tlsConfig = tlsCfg.Clone()
	}
	return p
}

// Open opens a logical stream to target ("hyla" or "pocket") on the remote host
// identified by pairCode, reusing a cached E2EE multiplexed tunnel when healthy.
func (p *ClientPool) Open(ctx context.Context, pairCode, target string) (net.Conn, error) {
	pc, err := ParsePairCode(pairCode)
	if err != nil {
		return nil, err
	}
	key := canonicalCode(pairCode)

	if mux := p.getCached(key); mux != nil {
		st, err := mux.OpenStream(ctx, target)
		if err == nil {
			return st, nil
		}
		// Only drop the pooled tunnel if the session itself died; caller ctx
		// cancellation should not tear down streams used by other panes.
		if mux.IsClosed() {
			p.dropMux(key, mux)
			_ = mux.Close()
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	mux, err := p.dialShared(ctx, key, pc)
	if err != nil {
		return nil, err
	}
	st, err := mux.OpenStream(ctx, target)
	if err != nil {
		if mux.IsClosed() {
			p.dropMux(key, mux)
			_ = mux.Close()
		}
		return nil, err
	}
	return st, nil
}

// CloseCode closes and removes any pooled tunnel associated with pairCode.
func (p *ClientPool) CloseCode(pairCode string) {
	key := canonicalCode(pairCode)
	p.mu.Lock()
	mux := p.conns[key]
	delete(p.conns, key)
	p.mu.Unlock()
	if mux != nil {
		_ = mux.Close()
	}
}

// CloseAll closes every cached tunnel in the pool.
func (p *ClientPool) CloseAll() {
	p.mu.Lock()
	muxes := make([]*Mux, 0, len(p.conns))
	for k, m := range p.conns {
		muxes = append(muxes, m)
		delete(p.conns, k)
	}
	p.mu.Unlock()
	for _, m := range muxes {
		_ = m.Close()
	}
}

func (p *ClientPool) getCached(key string) *Mux {
	p.mu.Lock()
	mux := p.conns[key]
	if mux != nil && mux.IsClosed() {
		delete(p.conns, key)
		mux = nil
	}
	p.mu.Unlock()
	return mux
}

func (p *ClientPool) dropMux(key string, mux *Mux) {
	p.mu.Lock()
	if cur, ok := p.conns[key]; ok && cur == mux {
		delete(p.conns, key)
	}
	p.mu.Unlock()
}

func (p *ClientPool) dialShared(ctx context.Context, key string, pc PairCode) (*Mux, error) {
	p.mu.Lock()
	if mux := p.conns[key]; mux != nil && !mux.IsClosed() {
		p.mu.Unlock()
		return mux, nil
	}
	delete(p.conns, key)

	if call, ok := p.inflight[key]; ok {
		p.mu.Unlock()
		select {
		case <-call.done:
			return call.mux, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	call := &dialCall{done: make(chan struct{})}
	p.inflight[key] = call
	p.mu.Unlock()

	go func() {
		dialCtx, cancel := context.WithTimeout(context.Background(), clientConnectTimeout)
		mux, err := p.dialTunnel(dialCtx, pc)
		cancel()

		p.mu.Lock()
		delete(p.inflight, key)
		if err == nil && mux != nil {
			// Ensure stale closed tunnels are evicted asynchronously.
			p.conns[key] = mux
			go func(k string, m *Mux) {
				<-m.Closed()
				p.dropMux(k, m)
			}(key, mux)
		}
		call.mux, call.err = mux, err
		p.mu.Unlock()
		close(call.done)
	}()

	select {
	case <-call.done:
		return call.mux, call.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *ClientPool) dialTunnel(ctx context.Context, pc PairCode) (*Mux, error) {
	raw, err := DialRelayWithTLS(ctx, pc.Relay, p.tlsConfig)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })

	_, nonce, flags, err := readServerHello(raw)
	if err != nil {
		stop()
		_ = raw.Close()
		return nil, err
	}
	if flags&flagTokenRequired != 0 && strings.TrimSpace(pc.RelayToken) == "" {
		stop()
		_ = raw.Close()
		return nil, errors.New("beacontower: relay requires an access token missing from this pair code")
	}

	hostID := HostIDFromPub(pc.HostPub)
	proof := ClientTokenProof(TokenDigest(pc.RelayToken), nonce, hostID)
	var req [48]byte
	copy(req[:16], hostID[:])
	copy(req[16:48], proof[:])
	if err := writeCtrlMsg(raw, msgConnect, req[:]); err != nil {
		stop()
		_ = raw.Close()
		return nil, err
	}
	if err := readOKOrErr(raw); err != nil {
		stop()
		_ = raw.Close()
		return nil, err
	}
	if !stop() {
		_ = raw.Close()
		return nil, context.DeadlineExceeded
	}

	e2eConn, err := ClientE2EHandshake(ctx, raw, pc.HostPub, pc.PSK)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	return NewMux(e2eConn, true), nil
}

func canonicalCode(s string) string {
	return StripScheme(s)
}
