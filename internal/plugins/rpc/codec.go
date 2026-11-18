package rpc

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

// Handler handles incoming JSON-RPC requests or notifications.
type Handler func(ctx context.Context, method string, params json.RawMessage) (any, error)

// Peer is a bidirectional JSON-RPC 2.0 NDJSON transport over a stream pair
// (such as stdio pipes, net.Pipe, Unix domain sockets, or Windows named pipes).
type Peer struct {
	r      io.Reader
	w      io.Writer
	closer io.Closer

	writeMu  sync.Mutex
	pendMu   sync.Mutex
	pending  map[string]chan Envelope
	nextID   atomic.Int64
	handlers sync.Map // method -> Handler
	fallback Handler

	doneOnce sync.Once
	done     chan struct{}
	closeErr error
}

// NewPeer wraps r/w (and optional closer) and starts reading frames until EOF or Close.
func NewPeer(r io.Reader, w io.Writer, closer io.Closer) *Peer {
	p := &Peer{
		r:       r,
		w:       w,
		closer:  closer,
		pending: make(map[string]chan Envelope),
		done:    make(chan struct{}),
	}
	return p
}

// Start launches the read loop in a background goroutine.
func (p *Peer) Start(ctx context.Context) {
	go p.readLoop(ctx)
}

// Serve runs the read loop synchronously on the caller goroutine.
func (p *Peer) Serve(ctx context.Context) error {
	p.readLoop(ctx)
	return p.closeErr
}

// Handle registers a handler for a specific RPC method.
func (p *Peer) Handle(method string, h Handler) {
	p.handlers.Store(method, h)
}

// HandleDefault registers a fallback handler invoked when no exact method handler exists.
func (p *Peer) HandleDefault(h Handler) {
	p.fallback = h
}

// Done returns a channel closed when the read loop stops.
func (p *Peer) Done() <-chan struct{} {
	return p.done
}

// Err returns the read/close error after Done() closes.
func (p *Peer) Err() error {
	<-p.done
	return p.closeErr
}

// Close closes the underlying stream and cancels all pending calls.
func (p *Peer) Close() error {
	var err error
	p.doneOnce.Do(func() {
		if p.closer != nil {
			err = p.closer.Close()
		}
		p.failPending(errors.New("rpc peer closed"))
		close(p.done)
	})
	return err
}

func (p *Peer) finish(err error) {
	p.doneOnce.Do(func() {
		p.closeErr = err
		if p.closer != nil {
			_ = p.closer.Close()
		}
		if err == nil {
			err = io.EOF
		}
		p.failPending(err)
		close(p.done)
	})
}

func (p *Peer) failPending(err error) {
	p.pendMu.Lock()
	defer p.pendMu.Unlock()
	for id, ch := range p.pending {
		delete(p.pending, id)
		ch <- Envelope{Error: &Error{Code: ErrCodeTimeout, Message: err.Error()}}
	}
}

// Call invokes method with params, waits for the response or ctx cancellation,
// and unmarshals result into out (if out != nil).
func (p *Peer) Call(ctx context.Context, method string, params any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	idNum := p.nextID.Add(1)
	idRaw := json.RawMessage(strconv.AppendInt(nil, idNum, 10))
	idKey := string(idRaw)

	var paramsRaw json.RawMessage
	if params != nil {
		if raw, ok := params.(json.RawMessage); ok {
			if len(raw) > 0 {
				paramsRaw = raw
			}
		} else {
			b, err := json.Marshal(params)
			if err != nil {
				return fmt.Errorf("marshal params for %s: %w", method, err)
			}
			paramsRaw = b
		}
	}

	ch := make(chan Envelope, 1)
	p.pendMu.Lock()
	select {
	case <-p.done:
		p.pendMu.Unlock()
		return NewError(ErrCodeTargetNotFound, "rpc peer is closed")
	default:
		p.pending[idKey] = ch
	}
	p.pendMu.Unlock()

	req := Envelope{
		JSONRPC: JSONRPCVersion,
		ID:      idRaw,
		Method:  method,
		Params:  paramsRaw,
	}
	if err := p.writeEnvelope(req); err != nil {
		p.pendMu.Lock()
		delete(p.pending, idKey)
		p.pendMu.Unlock()
		return err
	}

	select {
	case <-ctx.Done():
		p.pendMu.Lock()
		delete(p.pending, idKey)
		p.pendMu.Unlock()
		return NewError(ErrCodeTimeout, ctx.Err().Error())
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if out != nil && len(resp.Result) > 0 && !bytes.Equal(resp.Result, []byte("null")) {
			if rawPtr, ok := out.(*json.RawMessage); ok {
				*rawPtr = append([]byte(nil), resp.Result...)
				return nil
			}
			if err := json.Unmarshal(resp.Result, out); err != nil {
				return fmt.Errorf("unmarshal result for %s: %w", method, err)
			}
		}
		return nil
	}
}

// Notify sends a JSON-RPC 2.0 notification (no response expected).
func (p *Peer) Notify(method string, params any) error {
	var paramsRaw json.RawMessage
	if params != nil {
		if raw, ok := params.(json.RawMessage); ok {
			paramsRaw = raw
		} else {
			b, err := json.Marshal(params)
			if err != nil {
				return err
			}
			paramsRaw = b
		}
	}
	return p.writeEnvelope(Envelope{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  paramsRaw,
	})
}

func (p *Peer) writeEnvelope(env Envelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if len(b) > MaxFrameBytes {
		return NewError(ErrCodePayloadTooLarge, fmt.Sprintf("frame exceeds %d bytes", MaxFrameBytes))
	}
	b = append(b, '\n')
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err = p.w.Write(b)
	return err
}

func (p *Peer) readLoop(ctx context.Context) {
	sc := bufio.NewScanner(p.r)
	sc.Buffer(make([]byte, 64*1024), MaxFrameBytes)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var env Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		if env.Method != "" {
			// Incoming request or notification; run concurrently so bi-directional callbacks do not deadlock.
			idCopy := append([]byte(nil), env.ID...)
			paramsCopy := append([]byte(nil), env.Params...)
			method := env.Method
			go p.dispatch(ctx, idCopy, method, paramsCopy)
			continue
		}
		if len(env.ID) > 0 {
			idKey := normalizeID(env.ID)
			p.pendMu.Lock()
			ch := p.pending[idKey]
			if ch != nil {
				delete(p.pending, idKey)
			}
			p.pendMu.Unlock()
			if ch != nil {
				ch <- env
			}
		}
	}
	err := sc.Err()
	if errors.Is(err, bufio.ErrTooLong) {
		err = NewError(ErrCodePayloadTooLarge, fmt.Sprintf("incoming line exceeds %d bytes", MaxFrameBytes))
	}
	p.finish(err)
}

func (p *Peer) dispatch(ctx context.Context, id json.RawMessage, method string, params json.RawMessage) {
	isReq := len(id) > 0 && !bytes.Equal(id, []byte("null"))
	var h Handler
	if v, ok := p.handlers.Load(method); ok {
		h = v.(Handler)
	} else {
		h = p.fallback
	}
	if h == nil {
		if isReq {
			_ = p.writeEnvelope(Envelope{
				JSONRPC: JSONRPCVersion,
				ID:      id,
				Error:   NewError(ErrCodeMethodNotFound, "method not found: "+method),
			})
		}
		return
	}
	res, err := h(ctx, method, params)
	if !isReq {
		return
	}
	if err != nil {
		var rpcErr *Error
		if !errors.As(err, &rpcErr) {
			rpcErr = NewError(ErrCodeInternal, err.Error())
		}
		_ = p.writeEnvelope(Envelope{
			JSONRPC: JSONRPCVersion,
			ID:      id,
			Error:   rpcErr,
		})
		return
	}
	var resRaw json.RawMessage
	if res == nil {
		resRaw = json.RawMessage("null")
	} else if raw, ok := res.(json.RawMessage); ok {
		resRaw = raw
	} else {
		b, merr := json.Marshal(res)
		if merr != nil {
			_ = p.writeEnvelope(Envelope{
				JSONRPC: JSONRPCVersion,
				ID:      id,
				Error:   NewError(ErrCodeInternal, merr.Error()),
			})
			return
		}
		resRaw = b
	}
	if err := p.writeEnvelope(Envelope{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  resRaw,
	}); err != nil {
		_ = p.writeEnvelope(Envelope{
			JSONRPC: JSONRPCVersion,
			ID:      id,
			Error:   NewError(ErrCodePayloadTooLarge, err.Error()),
		})
	}
}

func normalizeID(raw json.RawMessage) string {
	return string(bytes.TrimSpace(raw))
}

// authFrame is exchanged as the first NDJSON line over local socket/pipe transports.
type authFrame struct {
	Token string `json:"token"`
}

// WriteAuthFrame writes the authentication token as the first line on a socket connection.
func WriteAuthFrame(w io.Writer, token string) error {
	b, err := json.Marshal(authFrame{Token: token})
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

// VerifyAuthFrame reads and constant-time-compares the opening auth line on a local socket.
func VerifyAuthFrame(r io.Reader, expectedToken string) (io.Reader, error) {
	br := bufio.NewReaderSize(r, 4096)
	line, err := br.ReadSlice('\n')
	if err != nil {
		return nil, fmt.Errorf("read plugin auth frame: %w", err)
	}
	var f authFrame
	if err := json.Unmarshal(bytes.TrimSpace(line), &f); err != nil {
		return nil, fmt.Errorf("invalid plugin auth frame: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(f.Token), []byte(expectedToken)) != 1 {
		return nil, NewError(ErrCodePermissionDenied, "invalid plugin socket auth token")
	}
	return br, nil
}
