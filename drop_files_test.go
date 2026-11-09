package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hashicorp/yamux"

	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"
)

// TestDropOnRemotePane drops a file on the pane of another machine, and
// checks that it goes where the pane's program runs, as the host tells,
// and that its path there is pasted into the pane.
func TestDropOnRemotePane(t *testing.T) {
	a := testApp(t)
	a.local().cl = nil
	h := &host{name: "devbox", cfg: settings.Host{Name: "devbox"}, conn: connReady}
	hostDir := t.TempDir()
	hostDir, _ = filepath.EvalSymlinks(hostDir)
	var mu sync.Mutex
	var pasted string
	handle := func(method string, params json.RawMessage) (any, error) {
		switch method {
		case "pane.dir":
			return proto.PaneDir{Dir: hostDir}, nil
		case "paste.text":
			var p proto.PasteTextParams
			json.Unmarshal(params, &p)
			mu.Lock()
			pasted = p.Text
			mu.Unlock()
			return nil, nil
		}
		return nil, fmt.Errorf("test daemon rejected %s", method)
	}
	// The app keeps hylad's control connection to the host (h.cl), and
	// the connection its streams open on (h.rc), which serves SFTP and
	// hylad's calls.
	conn := newTestConn(t, newTestHost(t, handle))
	h.rc = conn
	h.cl = testClient(t, h.rc)
	a.hosts = append(a.hosts, h)
	if a.owner == nil {
		a.owner = map[string]*host{}
	}
	for _, p := range a.st.Sessions[0].Panes {
		a.owner[p.ID] = h
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(file, []byte("dropped"), 0o600); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	tt.Frame()
	if !tt.DropFiles(600, 400, file) {
		t.Fatal("the content took no drop")
	}
	// The drop starts the upload as a frame is built (App.later): the
	// files go to the host's SFTP, and its hylad, off the main thread.
	// The end comes as it comes, and shows as a frame takes it up.
	// Frames follow the drop, as a window draws them, until the upload
	// ended and the frame that carried the drop was built: a drop takes
	// its target in one frame, and the upload starts in the next.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		tt.Frame()
		if len(a.uploading) == 0 {
			break
		}
	}
	if len(a.uploading) > 0 {
		t.Fatalf("the upload did not end: %+v; the toast says %q", a.uploading, a.pendingToast)
	}
	if _, err := os.Stat(filepath.Join(hostDir, "notes.txt")); err != nil {
		t.Errorf("the file did not go to the host's directory: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if pasted == "" || pasted != filepath.Join(hostDir, "notes.txt") {
		t.Errorf("pasted %q, want %q", pasted, filepath.Join(hostDir, "notes.txt"))
	}
}

// testClient opens a control connection to a test host, as one of its
// own streams.
func testClient(t *testing.T, d client.Dialer) *client.Client {
	t.Helper()
	cl, err := client.ConnectVia(d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	return cl
}

// buffered is a connection whose reads go through the reader that read
// its welcome: the session starts behind the handshake.
type buffered struct {
	r *bufio.Reader
	net.Conn
}

func (c buffered) Read(p []byte) (int, error) { return c.r.Read(p) }

// socketDialer dials a socket of this machine.
type socketDialer string

func (d socketDialer) Dial() (net.Conn, error) { return net.Dial("unix", string(d)) }

// newTestHost serves a test host at the path of a socket, as remote.Dial
// would reach it: its hylad, which answers handle's calls on the streams
// it opens, and SFTP, on a stream of its own. The files it gets go to
// this machine's file system, the test's.
func newTestHost(t *testing.T, handle func(method string, params json.RawMessage) (any, error)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "m")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveTestConnection(conn, handle)
		}
	}()
	return path
}

// serveTestConnection serves a connection a client of the test host
// opened, as hylad's mux does (remote.Server): the welcome, then each
// stream the client opens.
func serveTestConnection(conn net.Conn, handle func(method string, params json.RawMessage) (any, error)) {
	s := &remote.Server{
		Info:   func() proto.HostInfo { return proto.HostInfo{OS: "linux", Version: proto.Version} },
		Stream: func(conn net.Conn, hello []byte, _ proto.MuxHello) { serveControl(conn, handle) },
		SFTP:   remote.ServeSFTP,
	}
	s.Serve(conn) // it closes the connection as it ends
}

// serveControl serves hylad's control connection, which opens with a
// Hello: its welcome, and the calls the app makes on it.
func serveControl(conn net.Conn, handle func(method string, params json.RawMessage) (any, error)) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	enc := json.NewEncoder(conn)
	enc.Encode(proto.Welcome{Version: proto.Version})
	dec := json.NewDecoder(br)
	for {
		var req struct {
			ID     int64           `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := dec.Decode(&req); err != nil {
			return
		}
		var v any
		var err error
		if handle != nil {
			v, err = handle(req.Method, req.Params)
		}
		resp := map[string]any{"id": req.ID, "result": v}
		if err != nil {
			resp = map[string]any{"id": req.ID, "error": err.Error()}
		}
		if enc.Encode(resp) != nil {
			return
		}
	}
}

// newTestConn opens a remote connection to the test host at path, as a
// host's connection would, which serves SFTP on its streams.
func newTestConn(t *testing.T, path string) *testConn {
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	if _, err := c.Write([]byte(`{"kind":"mux","mux":1,"version":4}` + "\n")); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(c)
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var w proto.MuxWelcome
	if err := json.Unmarshal([]byte(line), &w); err != nil {
		t.Fatal(err)
	}
	sess, err := yamux.Client(buffered{br, c}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sess.Close() })
	return &testConn{sess: sess, w: w}
}

// testConn is a connection to a test host's streams.
type testConn struct {
	sess *yamux.Session
	w    proto.MuxWelcome
}

func (c *testConn) Dial() (net.Conn, error) { return c.sess.Open() }
func (c *testConn) Open() (net.Conn, error) { return c.sess.Open() }
func (c *testConn) Close() error            { return c.sess.Close() }
func (c *testConn) Done() <-chan struct{}   { return c.sess.CloseChan() }
func (c *testConn) Host(context.Context, proto.HostRequest) error {
	return errors.New("the test host takes no host requests")
}
func (c *testConn) Welcome() proto.MuxWelcome { return c.w }
