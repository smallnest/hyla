package pocket

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"hyla/internal/daemon"
)

// server serves the daemon under the prefix hylad's tailcat listener
// hands HylaPocket's paths to, and returns its address.
func server(t *testing.T) (*daemon.Daemon, string) {
	t.Helper()
	d := daemon.PocketForTest(log.New(io.Discard, "", 0))
	s := New(d, log.New(io.Discard, "", 0))
	root := http.NewServeMux()
	root.Handle(s.PocketPath(), s)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: root}
	t.Cleanup(func() {
		s.Close()
		srv.Close()
	})
	go srv.Serve(ln)
	return d, ln.Addr().String()
}

func dial(t *testing.T, addr, proto string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws://"+addr+PocketPath+"api/control", &websocket.DialOptions{
		Subprotocols: []string{proto},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// rpc is one request's answer, as HylaPocket reads it.
type rpc struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Error   *struct {
		Code string `json:"code"`
	} `json:"error"`
}

func call(t *testing.T, c *websocket.Conn, id, method string, payload any) (r rpc) {
	t.Helper()
	defer func() {
		if r.Error != nil {
			t.Logf("%s: %+v", method, *r.Error)
		}
	}()
	msg := map[string]any{"type": "request", "id": id, "method": method}
	if payload != nil {
		msg["payload"] = payload
	}
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
	for {
		_, got, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var r rpc
		_ = json.Unmarshal(got, &r)
		if r.Type != "response" {
			continue // an event
		}
		var head struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(got, &head)
		if head.ID == id {
			return r
		}
	}
}

// A page's request is refused, as hylad's listeners refuse it.
func TestNoWebPages(t *testing.T) {
	_, addr := server(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, "ws://"+addr+PocketPath+"api/control", &websocket.DialOptions{
		Subprotocols: []string{ProtoControl},
		HTTPHeader:   http.Header{"Origin": {"https://evil.example"}},
	})
	if err == nil {
		t.Fatal("a page's request was admitted")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("web page: %v %v", resp, err)
	}
}

// A session lists, views as a window of its panes, and takes what is
// typed in it, which comes back in its terminal's stream.
func TestSessionAndStream(t *testing.T) {
	d, addr := server(t)
	c := dial(t, addr, ProtoControl)
	defer c.CloseNow()

	list := call(t, c, "1", "session.list", nil)
	var l struct {
		Sessions []struct {
			ID    string `json:"session_id"`
			Label string `json:"label"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(list.Payload, &l); err != nil || len(l.Sessions) != 0 {
		t.Fatalf("list: %s %v", list.Payload, err)
	}

	created := call(t, c, "2", "session.create", map[string]any{"label": "pocket"})
	var cr struct {
		ID string `json:"session_id"`
	}
	if err := json.Unmarshal(created.Payload, &cr); err != nil || cr.ID == "" {
		t.Fatalf("create: %s %v", created.Payload, err)
	}

	attached := call(t, c, "3", "session.attach", map[string]any{"session_id": cr.ID})
	var at struct {
		View struct {
			Label   string `json:"label"`
			Windows []struct {
				Layers []struct {
					Blocks []struct {
						ID string `json:"block_id"`
					} `json:"blocks"`
				} `json:"layers"`
			} `json:"windows"`
		} `json:"view"`
	}
	if err := json.Unmarshal(attached.Payload, &at); err != nil {
		t.Fatal(err)
	}
	if at.View.Label != "pocket" || len(at.View.Windows) != 1 || len(at.View.Windows[0].Layers) != 1 {
		t.Fatalf("view: %s", attached.Payload)
	}
	block := at.View.Windows[0].Layers[0].Blocks[0].ID

	// A stream: the pane's snapshot first, whose scrollback and screen
	// come as VT.
	ticket := call(t, c, "4", "data.connect", map[string]any{"session_id": cr.ID, "block_id": block})
	var tk struct {
		Ticket string `json:"ticket"`
	}
	if err := json.Unmarshal(ticket.Payload, &tk); err != nil || tk.Ticket == "" {
		t.Fatalf("ticket: %s", ticket.Payload)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stream, _, err := websocket.Dial(ctx, "ws://"+addr+PocketPath+"api/data?ticket="+tk.Ticket+"&session="+cr.ID+"&block="+block, &websocket.DialOptions{
		Subprotocols: []string{ProtoData},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.CloseNow()
	typ, first, err := stream.Read(ctx)
	if err != nil || typ != websocket.MessageBinary || len(first) == 0 {
		t.Fatalf("first frame: %v %v (%d bytes)", typ, err, len(first))
	}

	// Typing reaches the pane, and its screen comes back.
	if n := call(t, c, "5", "com.superlogical.terminal.write", map[string]any{
		"session_id": cr.ID, "block_id": block, "args": map[string]any{"data": "echo pocket-marker\r"},
	}); n.Error != nil {
		t.Fatalf("write: %+v", n.Error)
	}
	// Typing comes back in the stream: a frame changes only what it
	// changes, so the screen it draws over is the snapshot's.
	var shown strings.Builder
	shown.Write(first)
	frames := make(chan []byte, 64)
	go func() {
		for {
			_, frame, err := stream.Read(context.Background())
			if err != nil {
				close(frames)
				return
			}
			frames <- frame
		}
	}()
	deadline := time.After(20 * time.Second)
loop:
	for !strings.Contains(shown.String(), "pocket-marker") {
		select {
		case frame, ok := <-frames:
			if !ok {
				t.Fatal("the stream ended")
			}
			shown.Write(frame)
		case <-deadline:
			break loop
		}
	}
	if !strings.Contains(shown.String(), "pocket-marker") {
		t.Fatalf("the pane's stream never showed what was typed:\n%s", shown.String())
	}

	// Closing the pane closes the session, as its last one.
	if n := call(t, c, "6", "block.close", map[string]any{"session_id": cr.ID, "block_id": block}); n.Error != nil {
		t.Fatalf("close: %+v", n.Error)
	}
	if _, ok := d.PocketSessionOf(cr.ID); ok {
		t.Fatal("the session outlived its only pane")
	}
}

// The methods HylaPocket's screens call, as they call them: the splits,
// the labels, a view, and a window that takes the session's other panes.
func TestScreensMethods(t *testing.T) {
	d, addr := server(t)
	c := dial(t, addr, ProtoControl)
	defer c.CloseNow()

	var created struct {
		ID string `json:"session_id"`
	}
	_ = json.Unmarshal(call(t, c, "1", "session.create", map[string]any{"label": "screens"}).Payload, &created)
	if created.ID == "" {
		t.Fatal("no session was made")
	}

	type block struct {
		ID string `json:"block_id"`
	}
	type layer struct {
		Blocks []block `json:"blocks"`
	}
	type window struct {
		ID      string  `json:"window_id"`
		Focused string  `json:"focused_block_id"`
		Layers  []layer `json:"layers"`
	}
	// The session's tab and panes; the pane the app would show is its
	// tab's focused one. A new session starts its shell a moment later.
	shown := func(id string) (tabs []window, focused string) {
		var v struct {
			SessionID string   `json:"session_id"`
			Windows   []window `json:"windows"`
		}
		r := call(t, c, "v", "session.view", map[string]any{"session_id": id})
		if r.Error != nil {
			t.Fatalf("view: %+v", r.Error)
		}
		if err := json.Unmarshal(r.Payload, &v); err != nil {
			t.Fatal(err)
		}
		if len(v.Windows) == 0 {
			return nil, ""
		}
		return v.Windows, v.Windows[0].Focused
	}
	var (
		tabs []window
		only string
	)
	for i := 0; i < 100; i++ {
		tabs, only = shown(created.ID)
		if len(tabs) > 0 && len(tabs[0].Layers) > 0 && len(tabs[0].Layers[0].Blocks) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(tabs) != 1 || len(tabs[0].Layers) != 1 || len(tabs[0].Layers[0].Blocks) != 1 || only == "" {
		t.Fatalf("a new session: tabs=%+v only=%q", tabs, only)
	}
	panes := func() []block {
		var all []block
		for _, w := range tabs {
			for _, l := range w.Layers {
				all = append(all, l.Blocks...)
			}
		}
		return all
	}
	if n := len(panes()); n != 1 {
		t.Fatalf("a new session has %d panes, want 1", n)
	}

	for _, dir := range []string{"horizontal", "vertical"} {
		r := call(t, c, "2", "session.new_split", map[string]any{
			"session_id": created.ID, "anchor_id": only, "direction": dir, "side": "after", "ratio": 0.5,
		})
		if r.Error != nil {
			t.Fatalf("split %s: %+v", dir, r.Error)
		}
	}
	// The pane split sideways has the first above it in its column, though
	// the desktop would show it beside the first: the app shows one row.
	tabs, _ = shown(created.ID)
	if len(panes()) != 3 {
		t.Fatalf("two splits made %d panes: %+v", len(panes()), tabs)
	}
	if r := call(t, c, "3", "com.superlogical.terminal.resize", map[string]any{
		"session_id": created.ID, "block_id": only,
		"args": map[string]any{"columns": 90, "rows": 30, "cell_width_px": 8, "cell_height_px": 16},
	}); r.Error != nil {
		t.Fatalf("resize: %+v", r.Error)
	}
	if r := call(t, c, "4", "session.set_block_label", map[string]any{
		"session_id": created.ID, "block_id": only, "label": "api",
	}); r.Error != nil {
		t.Fatalf("rename pane: %+v", r.Error)
	}
	if r := call(t, c, "5", "session.set_label", map[string]any{"session_id": created.ID, "label": "screens 2"}); r.Error != nil {
		t.Fatalf("rename session: %+v", r.Error)
	}

	// Closing the session closes it on the daemon too.
	if r := call(t, c, "6", "session.destroy", map[string]any{"session_id": created.ID}); r.Error != nil {
		t.Fatalf("destroy: %+v", r.Error)
	}
	if _, ok := d.PocketSessionOf(created.ID); ok {
		t.Fatal("the session outlived its destroy")
	}
}
