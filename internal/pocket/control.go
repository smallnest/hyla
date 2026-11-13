package pocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// maxMessage bounds what a client sends in a request.
const maxMessage = 1 << 20

func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// control is one control connection of HylaPocket: it reads requests,
// runs them beside each other, and writes the answers and the events in
// one goroutine, so that nothing interleaves.
type control struct {
	s *Server
	c *websocket.Conn
	// out is written by eventsLoop alone.
	out  chan any
	stop chan struct{}
	once sync.Once

	mu sync.Mutex
	// viewed is the session the connection attached to, by pane, for the
	// panes it owns.
	viewed   map[string]string
	lastSess string
	// sizes are the sizes the connection asked for, by pane.
	sizes map[string][4]int
}

func newControl(s *Server, c *websocket.Conn) *control {
	return &control{
		s: s, c: c, out: make(chan any, 1024), stop: make(chan struct{}),
		viewed: map[string]string{}, sizes: map[string][4]int{},
	}
}

func (c *control) close(code websocket.StatusCode, reason string) {
	c.end()
	c.c.Close(code, reason)
}

func (c *control) end() {
	c.once.Do(func() { close(c.stop) })
}

func (c *control) lastViewed() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastSess, c.lastSess != ""
}

// keepAlive pings the client, and ends a connection whose client is gone: a phone that was killed
// leaves its tailcat stream up until the relay times it out, and the panes it owned stay its own
// until this connection drops.
func (c *control) keepAlive() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := c.c.Ping(ctx)
			cancel()
			if err != nil {
				c.end()
				c.c.CloseNow()
				return
			}
		}
	}
}

// run reads requests until the connection ends.
func (c *control) run() {
	go c.eventsLoop()
	go c.keepAlive()
	for {
		c.c.SetReadLimit(maxMessage)
		typ, data, err := c.c.Read(context.Background())
		if err != nil {
			break
		}
		if typ != websocket.MessageText {
			continue
		}
		var msg struct {
			Type    string          `json:"type"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		// A client's hello names it and asks for a revision; nothing in the
		// answers depends on it, so it is taken and not kept.
		if msg.Type == "hello" {
			continue
		}
		if msg.Type != "request" {
			continue
		}
		// Calls run side by side, as rexpocket's do; typing keeps its
		// order in the order the requests arrive.
		go c.answer(msg.ID, msg.Method, msg.Payload)
	}
	c.end()
	c.c.CloseNow()
}

// answer runs one request and queues its answer.
func (c *control) answer(id json.RawMessage, method string, payload json.RawMessage) {
	res, code, err := c.call(method, payload)
	m := map[string]any{"type": "response", "id": id}
	if err != nil {
		status := 400
		switch code {
		case "not_found":
			status = 404
		case "conflict":
			status = 409
		case "internal":
			status = 500
		}
		m["error"] = map[string]any{"code": code, "status": status, "message": err.Error()}
	} else if res != nil {
		m["payload"] = res
	} else {
		m["payload"] = map[string]any{}
	}
	c.send(m)
}

func (c *control) send(m any) {
	select {
	case c.out <- m:
	case <-c.stop:
	}
}

// event queues an event to send; a client far behind loses it, and the
// next view it reads has what it missed.
func (c *control) event(payload any) {
	select {
	case c.out <- map[string]any{"type": "event", "payload": payload}:
	default:
	}
}

func (c *control) eventsLoop() {
	for {
		select {
		case <-c.stop:
			return
		case m := <-c.out:
			if err := writeText(c.c, m); err != nil {
				c.end()
				c.c.CloseNow()
				return
			}
		}
	}
}

// params decodes a request's payload into v.
func params(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return nil
}

// call runs a method of the wire. Its error's code says what failed, as
// rexpocket reads it.
func (c *control) call(method string, payload json.RawMessage) (any, string, error) {
	d := c.s.d
	switch method {
	case "session.list":
		return map[string]any{"sessions": d.PocketSessions()}, "", nil

	case "session.bootstrap":
		var p struct {
			Preferred string `json:"preferred_session_id"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		sessions := d.PocketSessions()
		out := map[string]any{"attach_all": map[string]any{"sessions": sessions}}
		// The session to show: the one asked for, else the first, as
		// rexpocket opens what the list gives first.
		sid := p.Preferred
		if sid == "" && len(sessions) > 0 {
			sid = sessions[0].ID
		}
		if sid != "" {
			if view, err := d.PocketView(sid); err == nil {
				out["selected_session"] = map[string]any{"session": view.SessionID, "view": view}
				c.look(view.SessionID)
			}
		}
		return out, "", nil

	case "session.view":
		var p paneRef
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		view, err := d.PocketView(p.SessionID)
		if err != nil {
			return nil, "not_found", err
		}
		return view, "", nil

	case "session.attach":
		var p paneRef
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		view, err := d.PocketView(p.SessionID)
		if err != nil {
			return nil, "not_found", err
		}
		c.look(p.SessionID)
		return map[string]any{"view": view}, "", nil

	case "session.create":
		var p struct {
			Label   string `json:"label"`
			Project string `json:"project"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		sid, err := d.PocketCreate(p.Label, p.Project)
		if err != nil {
			return nil, "internal", err
		}
		return map[string]any{"session_id": sid}, "", nil

	case "session.destroy":
		var p paneRef
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		if err := d.PocketDestroy(p.SessionID); err != nil {
			return nil, "not_found", err
		}
		return nil, "", nil

	case "session.set_label":
		var p struct {
			SessionID string `json:"session_id"`
			Label     string `json:"label"`
			ID        string `json:"id"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		if p.SessionID == "" {
			p.SessionID = p.ID
		}
		if err := d.PocketRename(p.SessionID, p.Label); err != nil {
			return nil, "not_found", err
		}
		return nil, "", nil

	case "session.new_split":
		var p struct {
			SessionID string `json:"session_id"`
			Anchor    string `json:"anchor_id"`
			Direction string `json:"direction"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		if p.Anchor == "" {
			return nil, "invalid_request", errors.New("a split needs an anchor")
		}
		block, err := d.PocketSplit(p.SessionID, p.Anchor, p.Direction)
		if err != nil {
			return nil, "not_found", err
		}
		return map[string]any{"block_ids": []string{block}}, "", nil

	case "session.new_window":
		// Hyla has no tabs: a window is a session here, and the app opens
		// a new one for it.
		var p struct {
			SessionID string `json:"session_id"`
		}
		_ = params(payload, &p)
		label := ""
		project := ""
		if p.SessionID != "" {
			if ps, ok := d.PocketSessionOf(p.SessionID); ok {
				label = ps.Label
				project = ps.Project
			}
		}
		sid, err := d.PocketCreate(label, project)
		if err != nil {
			return nil, "internal", err
		}
		out := map[string]any{"session_id": sid, "block_ids": []string{}}
		if panes, err := d.PocketPanes(sid); err == nil && len(panes) > 0 {
			out["block_ids"] = panes
		}
		return out, "", nil

	case "session.close_window":
		var p struct {
			SessionID string `json:"session_id"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		if err := d.PocketDestroy(p.SessionID); err != nil {
			return nil, "not_found", err
		}
		return nil, "", nil

	case "block.close":
		var p paneRef
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		if err := d.PocketClosePane(p.SessionID, p.BlockID); err != nil {
			return nil, "not_found", err
		}
		return nil, "", nil

	case "session.set_block_label":
		var p struct {
			SessionID string `json:"session_id"`
			BlockID   string `json:"block_id"`
			Label     string `json:"label"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		if err := d.PocketRenamePane(p.SessionID, p.BlockID, p.Label); err != nil {
			return nil, "not_found", err
		}
		return nil, "", nil

	case "com.superlogical.terminal.write":
		var p struct {
			SessionID string `json:"session_id"`
			BlockID   string `json:"block_id"`
			Args      struct {
				Data string `json:"data"`
			} `json:"args"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		if err := d.PocketWrite(p.SessionID, p.BlockID, p.Args.Data); err != nil {
			return nil, "not_found", err
		}
		c.lookFor(p.SessionID, p.BlockID)
		c.mu.Lock()
		// Typing in a pane no client owns makes the phone its terminal:
		// its size becomes the pane's, as a terminal that is typed in.
		if _, owned := c.s.owners[p.BlockID]; !owned {
			if size, ok := c.sizes[p.BlockID]; ok {
				_ = d.PocketResize(p.SessionID, p.BlockID, size[0], size[1], size[2], size[3])
			}
		}
		c.mu.Unlock()
		return nil, "", nil

	case "com.superlogical.terminal.resize":
		var p struct {
			SessionID string `json:"session_id"`
			BlockID   string `json:"block_id"`
			Args      struct {
				Columns    int `json:"columns"`
				Rows       int `json:"rows"`
				CellWidth  int `json:"cell_width_px"`
				CellHeight int `json:"cell_height_px"`
			} `json:"args"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		c.lookFor(p.SessionID, p.BlockID)
		size := [4]int{p.Args.Columns, p.Args.Rows, p.Args.CellWidth, p.Args.CellHeight}
		c.mu.Lock()
		c.sizes[p.BlockID] = size
		c.mu.Unlock()
		c.s.mu.Lock()
		c.s.sized[p.BlockID] = size
		owner := c.s.owners[p.BlockID]
		c.s.mu.Unlock()
		// An owner's size is the pane's at once; another client's waits
		// until it owns the pane.
		if owner == c {
			if err := d.PocketResize(p.SessionID, p.BlockID, size[0], size[1], size[2], size[3]); err != nil {
				return nil, "not_found", err
			}
		}
		return nil, "", nil

	case "session.own_block":
		var p struct {
			SessionID string   `json:"session_id"`
			BlockIDs  []string `json:"block_ids"`
			Mode      string   `json:"mode"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		for _, b := range p.BlockIDs {
			c.lookFor(p.SessionID, b)
			c.s.mu.Lock()
			o, owned := c.s.owners[b]
			if owned && o != c && p.Mode != "force" {
				c.s.mu.Unlock()
				return nil, "conflict", fmt.Errorf("%s is another client's terminal", b)
			}
			c.s.owners[b] = c
			size, has := c.s.sized[b]
			c.s.mu.Unlock()
			if has {
				_ = d.PocketResize(p.SessionID, b, size[0], size[1], size[2], size[3])
			}
		}
		return true, "", nil

	case "session.disown_block":
		var p struct {
			SessionID string   `json:"session_id"`
			BlockIDs  []string `json:"block_ids"`
		}
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		c.s.mu.Lock()
		for _, b := range p.BlockIDs {
			if c.s.owners[b] == c {
				delete(c.s.owners, b)
			}
		}
		c.s.mu.Unlock()
		return true, "", nil

	case "data.connect":
		var p paneRef
		if err := params(payload, &p); err != nil {
			return nil, "invalid_request", err
		}
		// A ticket goes only for a pane that exists.
		if _, err := d.PocketView(p.SessionID); err != nil {
			return nil, "not_found", err
		}
		c.lookFor(p.SessionID, p.BlockID)
		id := newTicketID()
		c.s.mu.Lock()
		c.s.tickets[id] = &ticket{client: c, session: p.SessionID, pane: p.BlockID, expires: time.Now().Add(ticketLife)}
		c.s.mu.Unlock()
		return map[string]any{"ticket": id}, "", nil
	}
	return nil, "not_found", fmt.Errorf("no method %q", method)
}

// look tells the daemon that the connection shows a session, as hyla
// focus does: its unread marks clear.
func (c *control) look(sid string) {
	c.lookFor(sid, "")
}

func (c *control) lookFor(sid, pane string) {
	c.mu.Lock()
	c.lastSess = sid
	if pane != "" {
		c.viewed[pane] = sid
	}
	c.mu.Unlock()
	c.s.d.PocketLooks(sid)
}

type paneRef struct {
	SessionID string `json:"session_id"`
	BlockID   string `json:"block_id"`
}

// follow starts the follower of the connection's panes.
func (c *control) follow() {
	(&follower{c: c}).start()
}

// follower reads the daemon's state as it changes, and sends what differs
// in the panes the connection shows, as rexpocket's events say it.
type follower struct {
	c    *control
	last map[string]string
}

func (f *follower) start() {
	f.last = map[string]string{}
	go func() {
		sub := f.c.s.d.PocketWatch()
		defer sub.Close()
		for {
			select {
			case <-f.c.stop:
				return
			case <-sub.C:
			}
			f.tick()
		}
	}()
}

// tick sends the events of what changed since the last state.
func (f *follower) tick() {
	state := f.c.s.d.PocketStatuses()
	for pane, st := range state {
		prev, ok := f.last[pane]
		if ok && prev == st.Status {
			continue
		}
		f.last[pane] = st.Status
		if !ok && st.Status == "" {
			continue
		}
		f.c.event(map[string]any{
			"type": "block_status", "session_id": st.Session, "block_id": pane,
			"state": mapStatus(st.Status), "message": st.Message, "kind": st.Kind, "progress": st.Progress,
		})
	}
	for pane := range f.last {
		if _, ok := state[pane]; !ok {
			delete(f.last, pane)
			f.c.mu.Lock()
			sid := f.c.viewed[pane]
			f.c.mu.Unlock()
			f.c.event(map[string]any{"type": "pane_closed", "session_id": sid, "block_id": pane})
		}
	}
}

// mapStatus says a status in rexpocket's words: it has no waiting.
func mapStatus(s string) string {
	if s == "waiting" {
		return "working"
	}
	return s
}
