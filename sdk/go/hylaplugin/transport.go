package hylaplugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

type methodHandler func(ctx context.Context, params json.RawMessage) (any, error)

type peer struct {
	r      io.Reader
	w      io.Writer
	closer io.Closer

	writeMu  sync.Mutex
	pendMu   sync.Mutex
	pending  map[string]chan Envelope
	nextID   atomic.Int64
	handlers sync.Map
	doneOnce sync.Once
	done     chan struct{}
	closeErr error
}

func newPeer(r io.Reader, w io.Writer, closer io.Closer) *peer {
	return &peer{
		r:       r,
		w:       w,
		closer:  closer,
		pending: make(map[string]chan Envelope),
		done:    make(chan struct{}),
	}
}

func (p *peer) handle(method string, h methodHandler) {
	p.handlers.Store(method, h)
}

func (p *peer) start(ctx context.Context) {
	go p.readLoop(ctx)
}

func (p *peer) Done() <-chan struct{} { return p.done }

func (p *peer) Close() error {
	var err error
	p.doneOnce.Do(func() {
		if p.closer != nil {
			err = p.closer.Close()
		}
		p.failPending(errors.New("connection closed"))
		close(p.done)
	})
	return err
}

func (p *peer) Call(ctx context.Context, method string, params any, result any) error {
	id := strconv.FormatInt(p.nextID.Add(1), 10)
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		rawParams = b
	}
	ch := make(chan Envelope, 1)
	p.pendMu.Lock()
	select {
	case <-p.done:
		p.pendMu.Unlock()
		return errors.New("connection closed")
	default:
		p.pending[id] = ch
	}
	p.pendMu.Unlock()

	if err := p.writeEnvelope(Envelope{
		JSONRPC: JSONRPCVersion,
		ID:      json.RawMessage(id),
		Method:  method,
		Params:  rawParams,
	}); err != nil {
		p.pendMu.Lock()
		delete(p.pending, id)
		p.pendMu.Unlock()
		return err
	}

	select {
	case <-ctx.Done():
		p.pendMu.Lock()
		delete(p.pending, id)
		p.pendMu.Unlock()
		return ctx.Err()
	case <-p.done:
		return errors.New("connection closed")
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && len(resp.Result) > 0 && string(bytes.TrimSpace(resp.Result)) != "null" {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}

func (p *peer) Notify(method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		rawParams = b
	}
	return p.writeEnvelope(Envelope{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  rawParams,
	})
}

func (p *peer) writeEnvelope(env Envelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if len(b)+1 > MaxFrameBytes {
		return NewError(ErrCodePayloadTooLarge, "payload exceeds 16 MiB frame limit")
	}
	b = append(b, '\n')
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err = p.w.Write(b)
	return err
}

func (p *peer) readLoop(ctx context.Context) {
	defer func() { _ = p.Close() }()
	sc := bufio.NewScanner(p.r)
	sc.Buffer(make([]byte, 64*1024), MaxFrameBytes)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var env Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			_ = p.writeEnvelope(Envelope{
				JSONRPC: JSONRPCVersion,
				Error:   NewError(ErrCodeParse, "invalid JSON"),
			})
			continue
		}
		if env.Method == "" && len(env.ID) > 0 {
			key := string(bytes.TrimSpace(env.ID))
			p.pendMu.Lock()
			ch := p.pending[key]
			delete(p.pending, key)
			p.pendMu.Unlock()
			if ch != nil {
				ch <- env
			}
			continue
		}
		if env.Method == "" {
			continue
		}
		idCopy := append(json.RawMessage(nil), env.ID...)
		paramsCopy := append(json.RawMessage(nil), env.Params...)
		go p.dispatch(ctx, idCopy, env.Method, paramsCopy)
	}
}

func (p *peer) dispatch(ctx context.Context, id json.RawMessage, method string, params json.RawMessage) {
	isReq := len(bytes.TrimSpace(id)) > 0
	val, ok := p.handlers.Load(method)
	if !ok {
		if isReq {
			_ = p.writeEnvelope(Envelope{
				JSONRPC: JSONRPCVersion,
				ID:      id,
				Error:   NewError(ErrCodeMethodNotFound, fmt.Sprintf("method %q not found", method)),
			})
		}
		return
	}
	fn := val.(methodHandler)
	var res any
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = NewError(ErrCodeInternal, fmt.Sprintf("plugin panic: %v", r))
			}
		}()
		res, err = fn(ctx, params)
	}()
	if !isReq {
		return
	}
	if err != nil {
		var re *RPCError
		if !errors.As(err, &re) {
			re = NewError(ErrCodeInternal, err.Error())
		}
		_ = p.writeEnvelope(Envelope{JSONRPC: JSONRPCVersion, ID: id, Error: re})
		return
	}
	var resRaw json.RawMessage
	if res == nil {
		resRaw = json.RawMessage("null")
	} else {
		b, merr := json.Marshal(res)
		if merr != nil {
			_ = p.writeEnvelope(Envelope{JSONRPC: JSONRPCVersion, ID: id, Error: NewError(ErrCodeInternal, merr.Error())})
			return
		}
		resRaw = b
	}
	_ = p.writeEnvelope(Envelope{JSONRPC: JSONRPCVersion, ID: id, Result: resRaw})
}

func (p *peer) failPending(err error) {
	p.pendMu.Lock()
	defer p.pendMu.Unlock()
	for id, ch := range p.pending {
		ch <- Envelope{JSONRPC: JSONRPCVersion, Error: NewError(ErrCodeInternal, err.Error())}
		delete(p.pending, id)
	}
}

func writeAuthFrame(w io.Writer, token string) error {
	b, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: token})
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func openTransport() (io.ReadCloser, io.WriteCloser, bool, error) {
	if sock := os.Getenv("HYLA_PLUGIN_SOCKET"); sock != "" {
		token := os.Getenv("HYLA_PLUGIN_TOKEN")
		conn, err := dialPluginSocket(sock)
		if err != nil {
			return nil, nil, false, err
		}
		if err := writeAuthFrame(conn, token); err != nil {
			_ = conn.Close()
			return nil, nil, false, err
		}
		return conn, conn, false, nil
	}
	return os.Stdin, os.Stdout, true, nil
}
