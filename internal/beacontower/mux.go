package beacontower

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	muxOpen byte = 1 + iota
	muxOpenOK
	muxOpenErr
	muxData
	muxWindow
	muxFin
	muxRst
	muxPing
	muxPong
)

const (
	muxHeaderLen             = 9
	muxInitWindow            = 256 << 10
	muxWindowUpdateThreshold = 64 << 10
	muxMaxDataFrame          = 16 << 10
	muxMaxAcceptQueue        = 64
	muxMaxOpenPayload        = 1024
	muxPingInterval          = 20 * time.Second
	muxIdleTimeout           = 60 * time.Second
)

type acceptedStream struct {
	stream *MuxStream
	target string
}

// Mux multiplexes independent bidirectional streams over a single net.Conn
// using per-stream credit-based flow control and automatic keepalives.
type Mux struct {
	conn   net.Conn
	client bool

	wmu  sync.Mutex
	wbuf []byte

	mu      sync.Mutex
	nextID  uint32
	streams map[uint32]*MuxStream
	closed  bool

	acceptCh chan acceptedStream
	pongCh   chan uint32
	doneCh   chan struct{}
	closeOnce sync.Once

	lastRecv atomic.Int64
	pingSeq  atomic.Uint32
}

// NewMux wraps conn in a multiplexed session. isClient must be true on the
// dialing side (odd stream IDs) and false on the listening side (even IDs).
func NewMux(conn net.Conn, isClient bool) *Mux {
	next := uint32(2)
	if isClient {
		next = 1
	}
	m := &Mux{
		conn:     conn,
		client:   isClient,
		wbuf:     make([]byte, muxHeaderLen+muxMaxDataFrame),
		nextID:   next,
		streams:  make(map[uint32]*MuxStream),
		acceptCh: make(chan acceptedStream, muxMaxAcceptQueue),
		pongCh:   make(chan uint32, 8),
		doneCh:   make(chan struct{}),
	}
	m.lastRecv.Store(time.Now().UnixNano())
	go m.readLoop()
	go m.keepaliveLoop()
	return m
}

// Closed returns a channel that is closed when the multiplexer shuts down.
func (m *Mux) Closed() <-chan struct{} { return m.doneCh }

// IsClosed reports whether the multiplexer session has terminated.
func (m *Mux) IsClosed() bool {
	select {
	case <-m.doneCh:
		return true
	default:
		return false
	}
}

// Close closes the underlying connection and aborts all active streams.
func (m *Mux) Close() error {
	var err error
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		streams := make([]*MuxStream, 0, len(m.streams))
		for id, s := range m.streams {
			streams = append(streams, s)
			delete(m.streams, id)
		}
		m.mu.Unlock()

		close(m.doneCh)
		for _, s := range streams {
			s.forceReset(net.ErrClosed)
		}
		err = m.conn.Close()
	})
	return err
}

// OpenStream opens a new multiplexed logical stream to the remote service target.
func (m *Mux) OpenStream(ctx context.Context, target string) (net.Conn, error) {
	if len(target) > muxMaxOpenPayload {
		return nil, fmt.Errorf("beacontower mux: target %q too long", target)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, net.ErrClosed
	}
	id := m.nextID
	m.nextID += 2
	s := newMuxStream(m, id)
	m.streams[id] = s
	m.mu.Unlock()

	if err := m.writeFrame(muxOpen, id, uint32(len(target)), []byte(target)); err != nil {
		m.removeStream(id)
		s.forceReset(err)
		return nil, err
	}

	select {
	case err := <-s.openCh:
		if err != nil {
			m.removeStream(id)
			s.forceReset(err)
			return nil, err
		}
		return s, nil
	case <-m.doneCh:
		m.removeStream(id)
		s.forceReset(net.ErrClosed)
		return nil, net.ErrClosed
	case <-ctx.Done():
		m.removeStream(id)
		s.forceReset(ctx.Err())
		_ = m.writeFrame(muxRst, id, 0, nil)
		return nil, ctx.Err()
	}
}

// AcceptStream waits for and returns the next incoming logical stream and its target service name.
func (m *Mux) AcceptStream() (net.Conn, string, error) {
	select {
	case acc := <-m.acceptCh:
		return acc.stream, acc.target, nil
	case <-m.doneCh:
		return nil, "", io.EOF
	}
}

func (m *Mux) removeStream(id uint32) {
	m.mu.Lock()
	delete(m.streams, id)
	m.mu.Unlock()
}

func (m *Mux) getStream(id uint32) *MuxStream {
	m.mu.Lock()
	s := m.streams[id]
	m.mu.Unlock()
	return s
}

func (m *Mux) writeFrame(typ byte, streamID, val uint32, payload []byte) error {
	m.wmu.Lock()
	defer m.wmu.Unlock()
	if m.IsClosed() {
		return net.ErrClosed
	}
	total := muxHeaderLen + len(payload)
	buf := m.wbuf
	if total > len(buf) {
		buf = make([]byte, total)
	} else {
		buf = buf[:total]
	}
	buf[0] = typ
	binary.BigEndian.PutUint32(buf[1:5], streamID)
	binary.BigEndian.PutUint32(buf[5:9], val)
	copy(buf[9:], payload)
	_, err := m.conn.Write(buf)
	return err
}

func (m *Mux) readLoop() {
	defer m.Close()
	var hdr [muxHeaderLen]byte
	for {
		if _, err := io.ReadFull(m.conn, hdr[:]); err != nil {
			return
		}
		m.lastRecv.Store(time.Now().UnixNano())
		typ := hdr[0]
		streamID := binary.BigEndian.Uint32(hdr[1:5])
		val := binary.BigEndian.Uint32(hdr[5:9])

		var payload []byte
		switch typ {
		case muxOpen, muxOpenErr:
			if val > muxMaxOpenPayload {
				return
			}
			if val > 0 {
				payload = make([]byte, val)
				if _, err := io.ReadFull(m.conn, payload); err != nil {
					return
				}
			}
		case muxData:
			if val > muxMaxDataFrame {
				return
			}
			if val > 0 {
				payload = make([]byte, val)
				if _, err := io.ReadFull(m.conn, payload); err != nil {
					return
				}
			}
		}

		switch typ {
		case muxPing:
			select {
			case m.pongCh <- val:
			default:
			}
		case muxPong:
			// lastRecv updated above.
		case muxOpen:
			m.handleOpen(streamID, string(payload))
		case muxOpenOK:
			if s := m.getStream(streamID); s != nil {
				s.deliverOpen(nil)
			}
		case muxOpenErr:
			msg := string(payload)
			if msg == "" {
				msg = "remote rejected stream"
			}
			if s := m.getStream(streamID); s != nil {
				s.deliverOpen(errors.New(msg))
			}
		case muxData:
			s := m.getStream(streamID)
			if s == nil {
				continue
			}
			if !s.pushData(payload) {
				m.removeStream(streamID)
				s.forceReset(net.ErrClosed)
				_ = m.writeFrame(muxRst, streamID, 0, nil)
			}
		case muxWindow:
			if val > 0 {
				if s := m.getStream(streamID); s != nil {
					s.addSendWindow(val)
				}
			}
		case muxFin:
			if s := m.getStream(streamID); s != nil {
				if s.handleRemoteFin() {
					m.removeStream(streamID)
				}
			}
		case muxRst:
			s := m.getStream(streamID)
			m.removeStream(streamID)
			if s != nil {
				s.handleRemoteRst()
			}
		default:
			return
		}
	}
}

func (m *Mux) handleOpen(streamID uint32, target string) {
	if streamID == 0 || (streamID%2 == 0) != m.client {
		_ = m.writeFrame(muxOpenErr, streamID, 14, []byte("bad stream id"))
		return
	}
	s := newMuxStream(m, streamID)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	if _, exists := m.streams[streamID]; exists {
		m.mu.Unlock()
		_ = m.writeFrame(muxOpenErr, streamID, 15, []byte("duplicate id"))
		return
	}
	m.streams[streamID] = s
	m.mu.Unlock()

	select {
	case m.acceptCh <- acceptedStream{stream: s, target: target}:
		if err := m.writeFrame(muxOpenOK, streamID, 0, nil); err != nil {
			m.removeStream(streamID)
			s.forceReset(err)
		}
	case <-m.doneCh:
		m.removeStream(streamID)
		s.forceReset(net.ErrClosed)
	default:
		m.removeStream(streamID)
		s.forceReset(net.ErrClosed)
		errMsg := []byte("accept queue full")
		_ = m.writeFrame(muxOpenErr, streamID, uint32(len(errMsg)), errMsg)
	}
}

func (m *Mux) keepaliveLoop() {
	t := time.NewTicker(muxPingInterval)
	defer t.Stop()
	for {
		select {
		case <-m.doneCh:
			return
		case seq := <-m.pongCh:
			if err := m.writeFrame(muxPong, 0, seq, nil); err != nil {
				_ = m.Close()
				return
			}
		case <-t.C:
			last := time.Unix(0, m.lastRecv.Load())
			if time.Since(last) > muxIdleTimeout {
				_ = m.Close()
				return
			}
			seq := m.pingSeq.Add(1)
			if err := m.writeFrame(muxPing, 0, seq, nil); err != nil {
				_ = m.Close()
				return
			}
		}
	}
}

// MuxStream is one bidirectional logical stream inside a Mux session.
type MuxStream struct {
	mux *Mux
	id  uint32

	openOnce sync.Once
	openCh   chan error

	recvSig chan struct{}
	sendSig chan struct{}
	doneCh  chan struct{}
	doneOnce sync.Once

	mu        sync.Mutex
	recvBuf   []byte
	consumed  uint32
	remoteFin bool
	reset     bool
	resetErr  error
	closed    bool
	sendWin   int64
	readDL    deadlinePipe
	writeDL   deadlinePipe
}

func newMuxStream(m *Mux, id uint32) *MuxStream {
	return &MuxStream{
		mux:     m,
		id:      id,
		openCh:  make(chan error, 1),
		recvSig: make(chan struct{}, 1),
		sendSig: make(chan struct{}, 1),
		doneCh:  make(chan struct{}),
		sendWin: muxInitWindow,
	}
}

func (s *MuxStream) deliverOpen(err error) {
	s.openOnce.Do(func() {
		s.openCh <- err
	})
}

func (s *MuxStream) signalRecv() {
	select {
	case s.recvSig <- struct{}{}:
	default:
	}
}

func (s *MuxStream) signalSend() {
	select {
	case s.sendSig <- struct{}{}:
	default:
	}
}

func (s *MuxStream) closeDone() {
	s.doneOnce.Do(func() {
		close(s.doneCh)
	})
}

func (s *MuxStream) pushData(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	s.mu.Lock()
	if s.closed || s.reset || s.remoteFin {
		s.mu.Unlock()
		return true
	}
	if len(s.recvBuf)+len(data) > muxInitWindow {
		s.mu.Unlock()
		return false
	}
	s.recvBuf = append(s.recvBuf, data...)
	s.mu.Unlock()
	s.signalRecv()
	return true
}

func (s *MuxStream) addSendWindow(delta uint32) {
	s.mu.Lock()
	s.sendWin += int64(delta)
	s.mu.Unlock()
	s.signalSend()
}

func (s *MuxStream) handleRemoteFin() bool {
	s.mu.Lock()
	s.remoteFin = true
	both := s.closed
	s.mu.Unlock()
	s.signalRecv()
	return both
}

func (s *MuxStream) handleRemoteRst() {
	s.mu.Lock()
	s.reset = true
	s.remoteFin = true
	if s.resetErr == nil {
		s.resetErr = io.EOF
	}
	s.mu.Unlock()
	s.deliverOpen(net.ErrClosed)
	s.signalRecv()
	s.signalSend()
}

func (s *MuxStream) forceReset(err error) {
	if err == nil {
		err = net.ErrClosed
	}
	s.mu.Lock()
	s.closed = true
	s.reset = true
	if s.resetErr == nil {
		s.resetErr = err
	}
	s.readDL.clearLocked()
	s.writeDL.clearLocked()
	s.mu.Unlock()
	s.deliverOpen(err)
	s.closeDone()
	s.signalRecv()
	s.signalSend()
}

func (s *MuxStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return 0, net.ErrClosed
		}
		if len(s.recvBuf) > 0 {
			n := copy(p, s.recvBuf)
			if n == len(s.recvBuf) {
				s.recvBuf = s.recvBuf[:0]
			} else {
				copy(s.recvBuf, s.recvBuf[n:])
				s.recvBuf = s.recvBuf[:len(s.recvBuf)-n]
			}
			s.consumed += uint32(n)
			var credit uint32
			if s.consumed >= muxWindowUpdateThreshold && !s.remoteFin && !s.reset {
				credit = s.consumed
				s.consumed = 0
			}
			s.mu.Unlock()
			if credit > 0 {
				_ = s.mux.writeFrame(muxWindow, s.id, credit, nil)
			}
			return n, nil
		}
		if s.remoteFin {
			err := io.EOF
			if s.reset && s.resetErr != nil {
				err = s.resetErr
			}
			s.mu.Unlock()
			return 0, err
		}
		if s.reset {
			err := s.resetErr
			s.mu.Unlock()
			return 0, err
		}
		dlCh := s.readDL.waitLocked()
		s.mu.Unlock()

		if isClosedChan(dlCh) {
			return 0, os.ErrDeadlineExceeded
		}
		select {
		case <-s.recvSig:
		case <-dlCh:
			return 0, os.ErrDeadlineExceeded
		case <-s.doneCh:
			return 0, net.ErrClosed
		case <-s.mux.doneCh:
			return 0, io.EOF
		}
	}
}

func (s *MuxStream) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		s.mu.Lock()
		if s.closed || s.reset {
			err := s.resetErr
			if err == nil {
				err = net.ErrClosed
			}
			s.mu.Unlock()
			return written, err
		}
		dlCh := s.writeDL.waitLocked()
		avail := s.sendWin
		if avail <= 0 {
			s.mu.Unlock()
			if isClosedChan(dlCh) {
				return written, os.ErrDeadlineExceeded
			}
			select {
			case <-s.sendSig:
				continue
			case <-dlCh:
				return written, os.ErrDeadlineExceeded
			case <-s.doneCh:
				return written, net.ErrClosed
			case <-s.mux.doneCh:
				return written, net.ErrClosed
			}
		}
		n := len(p)
		if int64(n) > avail {
			n = int(avail)
		}
		if n > muxMaxDataFrame {
			n = muxMaxDataFrame
		}
		s.sendWin -= int64(n)
		s.mu.Unlock()

		if err := s.mux.writeFrame(muxData, s.id, uint32(n), p[:n]); err != nil {
			return written, err
		}
		written += n
		p = p[n:]
	}
	return written, nil
}

// Close terminates the stream, sending a graceful FIN or reset to the peer.
func (s *MuxStream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.resetErr == nil {
		s.resetErr = net.ErrClosed
	}
	remoteFin := s.remoteFin
	s.readDL.clearLocked()
	s.writeDL.clearLocked()
	s.mu.Unlock()

	s.closeDone()
	s.signalRecv()
	s.signalSend()

	if remoteFin {
		_ = s.mux.writeFrame(muxFin, s.id, 0, nil)
	} else {
		_ = s.mux.writeFrame(muxRst, s.id, 0, nil)
	}
	s.mux.removeStream(s.id)
	return nil
}

func (s *MuxStream) LocalAddr() net.Addr  { return s.mux.conn.LocalAddr() }
func (s *MuxStream) RemoteAddr() net.Addr { return s.mux.conn.RemoteAddr() }

func (s *MuxStream) SetDeadline(t time.Time) error {
	s.mu.Lock()
	s.readDL.setLocked(t)
	s.writeDL.setLocked(t)
	s.mu.Unlock()
	return nil
}

func (s *MuxStream) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	s.readDL.setLocked(t)
	s.mu.Unlock()
	return nil
}

func (s *MuxStream) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	s.writeDL.setLocked(t)
	s.mu.Unlock()
	return nil
}

type deadlinePipe struct {
	ch    chan struct{}
	timer *time.Timer
}

func (d *deadlinePipe) setLocked(t time.Time) {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.ch = nil
	if t.IsZero() {
		return
	}
	ch := make(chan struct{})
	d.ch = ch
	wait := time.Until(t)
	if wait <= 0 {
		close(ch)
		return
	}
	d.timer = time.AfterFunc(wait, func() {
		close(ch)
	})
}

func (d *deadlinePipe) waitLocked() <-chan struct{} { return d.ch }

func (d *deadlinePipe) clearLocked() {
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}

func isClosedChan(ch <-chan struct{}) bool {
	if ch == nil {
		return false
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
