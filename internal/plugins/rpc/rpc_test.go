package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestBidirectionalPeerAndAuthFrame(t *testing.T) {
	aConn, bConn := net.Pipe()
	defer aConn.Close()
	defer bConn.Close()

	// Test auth handshake + large post-auth stream (>4KiB).
	errCh := make(chan error, 1)
	go func() {
		errCh <- WriteAuthFrame(bConn, "secret-tok-123")
	}()
	authedReader, err := VerifyAuthFrame(aConn, "secret-tok-123")
	if err != nil {
		t.Fatalf("VerifyAuthFrame: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("WriteAuthFrame: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hostPeer := NewPeer(authedReader, aConn, aConn)
	pluginPeer := NewPeer(bConn, bConn, bConn)

	hostPeer.Handle("host.getState", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return HostGetStateResult{ActiveSession: "s1", ControlAllowed: true}, nil
	})
	pluginPeer.Handle("ui.render", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		// Bi-directional re-entrant callback inside handler!
		var st HostGetStateResult
		if err := pluginPeer.Call(ctx, "host.getState", HostGetStateParams{}, &st); err != nil {
			return nil, err
		}
		var req UIRenderParams
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		return UIRenderResult{
			Title: "Rendered:" + st.ActiveSession + ":" + strings.Repeat("x", len(req.SlotID)),
			Root:  &UINode{Type: "text", Key: "t1", Props: map[string]any{"text": "ok"}},
		}, nil
	})

	hostPeer.Start(ctx)
	pluginPeer.Start(ctx)

	var res UIRenderResult
	bigSlot := strings.Repeat("s", 8192) // verify >4096 auth reader cap does not clip subsequent frames
	if err := hostPeer.Call(ctx, "ui.render", UIRenderParams{SlotID: bigSlot}, &res); err != nil {
		t.Fatalf("hostPeer.Call: %v", err)
	}
	if res.Root == nil || !strings.HasPrefix(res.Title, "Rendered:s1:") {
		t.Fatalf("unexpected result: %+v", res)
	}

	var dummy any
	err = hostPeer.Call(ctx, "unknown.method", nil, &dummy)
	var rpcErr *Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != ErrCodeMethodNotFound {
		t.Fatalf("expected ErrCodeMethodNotFound, got %v", err)
	}

	// Wrong auth token rejected
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	go func() { _ = WriteAuthFrame(c2, "wrong") }()
	if _, err := VerifyAuthFrame(c1, "secret-tok-123"); err == nil {
		t.Fatal("expected auth failure for mismatched token")
	}
}
