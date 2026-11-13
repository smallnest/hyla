package daemon

// What HylaPocket's server (package pocket) asks of the daemon: the model
// in rexpocket's words, and a pane's terminal as a stream of VT. They stay
// here rather than in package pocket because they read and change the
// daemon's panes; the wire they feed is pocket's.

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"hyla/internal/proto"
)

// PocketServer is what a HylaPocket server does: serve the requests
// under pocket.PocketPath, which hylad's socket and its tailcat relay
// serve beside Hyla's, and close.
type PocketServer interface {
	http.Handler
	Close()
	// PocketPath is the prefix of the paths it serves.
	PocketPath() string
}

// newPocket builds the HylaPocket server, which package pocket sets as
// hylad starts: a cycle of imports keeps package pocket from daemon, so
// its server comes in as a function.
var newPocket func(d *Daemon, logger *log.Logger) PocketServer

// SetPocket makes the HylaPocket server a listener's.
func SetPocket(fn func(d *Daemon, logger *log.Logger) PocketServer) {
	newPocket = fn
}

// NewPocket returns the HylaPocket server of d, or nil when this build has
// none.
func NewPocket(d *Daemon, logger *log.Logger) PocketServer {
	if newPocket == nil {
		return nil
	}
	return newPocket(d, logger)
}

// PocketForTest makes a daemon that no hylad owns, for package pocket's
// tests: no socket, and the shells a session starts run here, so that
// its server can be given one.
func PocketForTest(logger *log.Logger) *Daemon {
	d := &Daemon{
		log: logger, started: time.Now(), panes: map[string]*pane{},
		subs: map[*controlConn]struct{}{}, watchers: map[*controlConn]struct{}{},
		pocketWatch: map[*PocketWatch]struct{}{},
		changeCh:    make(chan struct{}, 1), saveCh: make(chan struct{}, 1), quit: make(chan struct{}),
	}
	d.viewed.Store(new(string))
	d.net = &listeners{d: d, on: map[string]*listener{}}
	dir, err := os.MkdirTemp("", "hylad-pocket")
	if err == nil {
		d.dir = dir
	}
	return d
}

// PocketSession is a row of HylaPocket's list.
type PocketSession struct {
	ID      string `json:"session_id"`
	Label   string `json:"label"`
	Project string `json:"project,omitempty"`
	Panes   int    `json:"panes"`
	Dir     string `json:"dir,omitempty"`
	Status  string `json:"status"`
}

// PocketView is a session as HylaPocket shows it.
type PocketView struct {
	SessionID string         `json:"session_id"`
	Label     string         `json:"label"`
	Revision  int64          `json:"revision"`
	Active    string         `json:"active_window_id"`
	Windows   []PocketWindow `json:"windows"`
}

// PocketWindow is a tab of a view. Hyla has no tabs, and a row of a
// session's panes makes one.
type PocketWindow struct {
	ID      string        `json:"window_id"`
	Label   string        `json:"label"`
	Focused string        `json:"focused_block_id,omitempty"`
	Layers  []PocketLayer `json:"layers"`
}

// PocketLayer holds the blocks of a window's layer: one pane each, so
// that a pane's column order in the layout reads left to right.
type PocketLayer struct {
	Blocks []PocketBlock `json:"blocks"`
}

// PocketBlock is a pane as a view lists it.
type PocketBlock struct {
	ID     string `json:"block_id"`
	Label  string `json:"label"`
	Dir    string `json:"dir,omitempty"`
	Status string `json:"status,omitempty"`
}

// PocketPanes returns a session's panes in the layout's order, left to
// right and top to bottom.
func (d *Daemon) PocketPanes(sid string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.findSessionLocked(sid)
	if s == nil {
		return nil, fmt.Errorf("no session %q", sid)
	}
	out := make([]string, 0, len(s.panes))
	for _, id := range s.layout.Panes() {
		out = append(out, id)
	}
	if len(out) == 0 {
		for _, p := range s.panes {
			out = append(out, p.info.ID)
		}
	}
	return out, nil
}

// PocketWatch returns a subscription that has a value whenever the
// daemon's state changes, as hyla events hears of it.
func (d *Daemon) PocketWatch() *PocketWatch {
	w := &PocketWatch{
		d:    d,
		C:    make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	d.mu.Lock()
	d.pocketWatch[w] = struct{}{}
	d.mu.Unlock()
	return w
}

// PocketWatch has a value when the daemon's state changed.
type PocketWatch struct {
	d    *Daemon
	C    chan struct{}
	done chan struct{}
	once sync.Once
}

// Close ends the subscription.
func (w *PocketWatch) Close() {
	w.once.Do(func() {
		if d := w.d; d != nil {
			d.mu.Lock()
			delete(d.pocketWatch, w)
			d.mu.Unlock()
		}
		close(w.done)
	})
}

// PocketStatus is a pane's status, as HylaPocket shows it.
type PocketStatus struct {
	Session  string
	Status   string
	Message  string
	Kind     string
	Progress int
}

// PocketStatuses returns every pane's status.
func (d *Daemon) PocketStatuses() map[string]PocketStatus {
	d.mu.Lock()
	all := make([]*pane, 0, len(d.panes))
	for _, p := range d.panes {
		all = append(all, p)
	}
	d.mu.Unlock()
	out := make(map[string]PocketStatus, len(all))
	for _, p := range all {
		p.mu.Lock()
		out[p.info.ID] = PocketStatus{
			Session:  p.sid,
			Status:   string(p.info.Status),
			Message:  p.info.Message,
			Kind:     p.info.Kind,
			Progress: p.info.Progress,
		}
		p.mu.Unlock()
	}
	return out
}

// PocketViews returns every session's view.
func (d *Daemon) PocketViews() map[string]PocketView {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]PocketView, len(d.sessions))
	for _, s := range d.sessions {
		out[s.id] = d.pocketViewLocked(s)
	}
	return out
}

// PocketView returns a session's view without moving anything: what the
// app shows when it opens the session.
func (d *Daemon) PocketView(sid string) (PocketView, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.findSessionLocked(sid)
	if s == nil {
		return PocketView{}, fmt.Errorf("no session %q", sid)
	}
	return d.pocketViewLocked(s), nil
}

// PocketSessions returns the sessions, as the app lists them.
func (d *Daemon) PocketSessions() []PocketSession {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]PocketSession, 0, len(d.sessions))
	for _, s := range d.sessions {
		out = append(out, d.pocketSessionLocked(s))
	}
	return out
}

// PocketSessionOf returns one session's list row, and whether it exists.
func (d *Daemon) PocketSessionOf(sid string) (PocketSession, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.findSessionLocked(sid)
	if s == nil {
		return PocketSession{}, false
	}
	return d.pocketSessionLocked(s), true
}

// PocketLooks marks a session as the one this machine's clients look at,
// as hyla focus does: its unread marks clear, and what the hub makes of
// its panes goes on. "" marks none. It is the daemon's one viewed
// session, shared with the app, so the app and the phone move it in turn.
func (d *Daemon) PocketLooks(sid string) {
	d.viewed.Store(&sid)
	var panes []*pane
	if sid != "" {
		d.mu.Lock()
		if s := d.findSessionLocked(sid); s != nil {
			panes = slices.Clone(s.panes)
		}
		d.mu.Unlock()
	}
	for _, p := range panes {
		p.mu.Lock()
		p.info.Unread = false
		dir := p.info.WorkDir()
		p.mu.Unlock()
		if d.git != nil {
			d.git.poke(dir)
		}
	}
	d.changed()
}

// PocketViewed is the session clients look at, "" for none.
func (d *Daemon) PocketViewed() string {
	if p := d.viewed.Load(); p != nil {
		return *p
	}
	return ""
}

func (d *Daemon) pocketSessionLocked(s *session) PocketSession {
	info := d.sessionInfo(s)
	ps := PocketSession{ID: s.id, Label: info.Name, Project: d.projectNameLocked(s.project), Panes: len(s.panes), Status: string(info.Status)}
	if ps.Label == "" && len(s.panes) > 0 {
		ps.Label = s.panes[0].info.PaneName()
	}
	if ps.Label == "" {
		ps.Label = s.id
	}
	if s.focus != "" {
		if p := d.findPaneLocked(s.focus); p != nil {
			ps.Dir = p.info.WorkDir()
			ps.Status = string(p.info.Status)
		}
	}
	return ps
}

// projectNameLocked is the name of project id, or "" for a session of none. d.mu is held.
func (d *Daemon) projectNameLocked(id string) string {
	if id == "" {
		return ""
	}
	for _, p := range d.projects {
		if p.ID == id {
			if p.Name != "" {
				return p.Name
			}
			return p.Dir
		}
	}
	return id
}

// pocketViewLocked builds a view: a row of the layout is a window, and a
// pane of the row is a block. d.mu is held.
func (d *Daemon) pocketViewLocked(s *session) PocketView {
	info := d.sessionInfo(s)
	v := PocketView{SessionID: s.id, Revision: s.created, Active: s.id}
	if info.Name != "" {
		v.Label = info.Name
	} else if len(s.panes) > 0 {
		v.Label = s.panes[0].info.PaneName()
	} else {
		v.Label = s.id
	}
	byID := map[string]*pane{}
	for _, p := range s.panes {
		byID[p.info.ID] = p
		p.mu.Lock()
		v.Revision = max(v.Revision, p.info.Created, p.info.Since)
		p.mu.Unlock()
	}
	block := func(p *pane) PocketBlock {
		p.mu.Lock()
		defer p.mu.Unlock()
		return PocketBlock{ID: p.info.ID, Label: p.info.PaneName(), Dir: p.info.WorkDir(), Status: string(p.info.Status)}
	}
	row := func(ids []string) PocketWindow {
		w := PocketWindow{Label: v.Label}
		for _, id := range ids {
			p := byID[id]
			if p == nil {
				continue
			}
			w.Layers = append(w.Layers, PocketLayer{Blocks: []PocketBlock{block(p)}})
		}
		if len(w.Layers) > 0 && len(w.Layers[0].Blocks) > 0 {
			w.ID = "w" + w.Layers[0].Blocks[0].ID
		} else {
			w.ID = "w"
		}
		if _, ok := byID[s.focus]; ok {
			w.Focused = s.focus
		}
		return w
	}
	var rows [][]string
	if s.layout == nil || s.layout.Leaf() {
		ids := make([]string, 0, len(s.panes))
		for _, p := range s.panes {
			ids = append(ids, p.info.ID)
		}
		rows = [][]string{ids}
	} else {
		rows = layoutRows(s.layout)
	}
	for _, r := range rows {
		if len(r) > 0 {
			v.Windows = append(v.Windows, row(r))
		}
	}
	if len(v.Windows) == 0 {
		v.Windows = []PocketWindow{{ID: "w", Label: v.Label, Layers: []PocketLayer{{}}}}
	}
	v.Active = v.Windows[0].ID
	return v
}

// layoutRows folds a layout into rows: a row of panes is one, a column
// of them is not, and a row split of a pane and a column makes two, the
// pane above or below the column's rows.
func layoutRows(l *proto.Layout) [][]string {
	rows, _ := alignedRows(l, "")
	return rows
}

// alignedRows lines up a layout's panes row by row, top to bottom: panes
// side by side share a row; what is below the lowest of the upper part
// starts beside it. A split kept the anchor's pane as the top of its side
// with no leaf of its own, so carried says which pane that is: what is
// beside that side shares the row the pane alone takes above it.
func alignedRows(l *proto.Layout, carried string) (rows [][]string, taken bool) {
	if l == nil {
		return nil, false
	}
	if l.Leaf() {
		if l.Pane == "" {
			return nil, false
		}
		return [][]string{{l.Pane}}, false
	}
	if l.Row {
		var upper []string
		if carried != "" {
			upper = []string{carried}
			taken = true
		}

		for _, side := range []*proto.Layout{l.A, l.B} {
			sideRows, sideTook := alignedRows(side, "")
			if sideTook {
				// That side took the carried pane's row as its top.
				taken = true
			}
			for i, row := range sideRows {
				if i < len(rows) {
					rows[i] = append(rows[i], row...)
				} else {
					rows = append(rows, row)
				}
			}
		}
		if len(upper) > 0 {
			rows = append([][]string{upper}, rows...)
		}

		return rows, taken
	}
	a, taken := alignedRows(l.A, carried)
	b, tookB := alignedRows(l.B, "")
	taken = taken || tookB
	if len(a) == 0 {
		if carried == "" {
			return b, taken
		}
		return append([][]string{{carried}}, b...), true
	}
	if len(b) == 0 {
		return a, taken
	}
	out := append([][]string(nil), a...)
	out[len(out)-1] = append(out[len(out)-1], b[0]...)
	return append(out, b[1:]...), taken
}

// PocketCreate makes a session with a shell, as a new session does, in project: a project id or
// name, or "" for the machine's own.
func (d *Daemon) PocketCreate(name, project string) (string, error) {
	s, err := d.create(proto.CreateParams{Name: strings.TrimSpace(name), Project: strings.TrimSpace(project)})
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// PocketDestroy closes a session and its panes.
func (d *Daemon) PocketDestroy(sid string) error { return d.closeSession(proto.CloseParams{ID: sid}) }

// PocketRename names a session.
func (d *Daemon) PocketRename(sid, name string) error {
	d.mu.Lock()
	s := d.findSessionLocked(sid)
	if s == nil {
		d.mu.Unlock()
		return fmt.Errorf("no session %q", sid)
	}
	s.name = strings.TrimSpace(name)
	s.agentNamed = false
	d.mu.Unlock()
	d.changed()
	return nil
}

// PocketSplit adds a pane beside anchor, down or to the right of it.
// Hyla's split ends focused on the new pane, as the app does, so the pane
// the user was in goes back to the view's pane first.
func (d *Daemon) PocketSplit(sid, anchor, direction string) (string, error) {
	// The app names a side the new pane goes on; the layout names the axis.
	dir := "right"
	if direction == "down" || direction == "vertical" {
		dir = "down"
	}
	before := make(map[string]bool)
	if panes, err := d.PocketPanes(sid); err == nil {
		for _, id := range panes {
			before[id] = true
		}
	}
	focus := anchor
	if look, err := d.PocketStream(true, sid, anchor); err == nil {
		look.Close()
		focus = look.Focus()
	}
	info, err := d.create(proto.CreateParams{Split: focus, Direction: dir, Ratio: 0.5})
	if err != nil {
		return "", err
	}
	if info.ID != sid {
		return "", errors.New("the split opened a pane of another session")
	}
	// A session started a moment ago may not have listed its anchor yet, so
	// the new pane is the one that was not there before, and not the anchor.
	before[focus] = true
	for _, p := range info.Panes {
		if !before[p.ID] {
			return p.ID, nil
		}
	}
	return "", errors.New("the split made no pane")
}

// PocketClosePane closes a pane; the session goes with its last pane.
func (d *Daemon) PocketClosePane(sid, pane string) error {
	panes, err := d.PocketPanes(sid)
	if err != nil {
		return err
	}
	if len(panes) <= 1 {
		return d.PocketDestroy(sid)
	}
	return d.removePane(pane)
}

// PocketRenamePane names a pane, which its block shows.
func (d *Daemon) PocketRenamePane(sid, pane, name string) error {
	p := d.pocketPane(sid, pane)
	if p == nil {
		return fmt.Errorf("no pane %q in session %q", pane, sid)
	}
	p.mu.Lock()
	p.info.Title = strings.TrimSpace(name)
	p.mu.Unlock()
	d.changed()
	return nil
}

// PocketWrite writes what the user typed into a pane, as it is: keys,
// pastes and the terminal's answers alike, in the order of the calls.
func (d *Daemon) PocketWrite(sid, pane, data string) error {
	if data == "" {
		return nil
	}
	p := d.pocketPane(sid, pane)
	if p == nil {
		return fmt.Errorf("no pane %q in session %q", pane, sid)
	}
	p.input([]byte(data))
	return nil
}

// PocketResize takes the size a client's terminal fits.
func (d *Daemon) PocketResize(sid, pane string, cols, rows, cellW, cellH int) error {
	if cols <= 0 || rows <= 0 {
		return errors.New("a size is positive")
	}
	p := d.pocketPane(sid, pane)
	if p == nil {
		return fmt.Errorf("no pane %q in session %q", pane, sid)
	}
	p.resize(cols, rows, max(cellW, 0), max(cellH, 0))
	return nil
}

func (d *Daemon) pocketPane(sid, id string) *pane {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s := d.findSessionLocked(sid); s == nil {
		return nil
	}
	if p := d.findPaneLocked(id); p != nil && p.sid == sid {
		return p
	}
	return nil
}

// PocketStream is a pane's terminal over time: a mirror of it that one
// client reads frames from, and a wake for every change of its screen.
type PocketStream struct {
	// snap is what the pane showed when the stream began, as VT, and the
	// first Next returns it.
	snap []byte
	m    *terminal.Mirror
	// Focus is the pane the session's view focuses.
	focus string
	// Wake has a value when the screen changed.
	Wake chan struct{}
	// doneAway is closed when the client takes the pane: the stream goes
	// with it, and the client opens another, a small one.
	doneAway chan struct{}
	// done is closed when Close ends the stream.
	done chan struct{}

	once sync.Once
}

// Next returns what brings a terminal to what the pane shows now: nil
// while it shows it already.
func (s *PocketStream) Next() []byte {
	if s.snap != nil {
		f := s.snap
		s.snap = nil
		return f
	}
	f, _ := s.m.Frame()
	return f
}

// Focus is the pane the session's view focuses.
func (s *PocketStream) Focus() string { return s.focus }

// Done is closed when the stream must end: the client took the pane, or
// hylad ends it.
func (s *PocketStream) Done() <-chan struct{} { return s.done }

// Close ends the stream; the pane's next stream starts from its snapshot.
func (s *PocketStream) Close() {
	s.once.Do(func() {
		close(s.done)
		s.m.Close()
	})
}

// PocketStream starts a pane's stream. A plain stream is the pane as the
// client that owns it sees it: one at a time, and a client that takes the
// pane ends it (endPocketStream). A small stream is a look, for reading a
// view, and it changes nothing.
func (d *Daemon) PocketStream(small bool, sid, id string) (*PocketStream, error) {
	d.mu.Lock()
	s := d.findSessionLocked(sid)
	var p *pane
	if s != nil {
		p = d.findPaneLocked(id)
	}
	focus := ""
	if s != nil {
		focus = s.focus
	}
	d.mu.Unlock()
	if s == nil {
		return nil, fmt.Errorf("no session %q", sid)
	}
	if p == nil || p.sid != sid {
		return nil, fmt.Errorf("no pane %q in session %q", id, sid)
	}

	p.mu.Lock()
	if p.closed || p.term == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("pane %q closed", id)
	}
	if !small {
		// A stream the pane had is to a client that went: its mirror saw
		// output this stream's snapshot also shows.
		if p.pocket != nil {
			p.pocket.Close()
			p.pocket = nil
		}
		if p.pocketAway != nil {
			close(p.pocketAway)
			p.pocketAway = nil
		}
		p.pocketAway = make(chan struct{})
	}
	// The snapshot first: a mirror started after it would show the
	// changes since the snapshot as the whole screen, and lose what the
	// screen held before them.
	snap := p.term.Snapshot()
	m, err := p.term.NewMirror()
	if err != nil {
		p.mu.Unlock()
		return nil, err
	}
	st := &PocketStream{snap: snap, m: m, focus: focus, Wake: make(chan struct{}, 1), doneAway: p.pocketAway, done: make(chan struct{})}
	if small {
		st.doneAway = make(chan struct{})
	} else {
		p.pocket = st
	}
	p.mu.Unlock()
	st.Wake <- struct{}{}
	return st, nil
}

// pocketPush wakes the pane's stream, as its screen changed; s.mu is
// held.
func (s *pane) pocketPush() {
	if s.pocket == nil {
		return
	}
	select {
	case s.pocket.Wake <- struct{}{}:
	default:
	}
}

// endPocketStream ends the stream as another client takes the terminal:
// the stream's output is gone to that client, which reads the pane from
// its own stream, and this one stops. s.mu is held.
func (s *pane) endPocketStream() {
	if s.pocket != nil && s.pocketAway != nil {
		close(s.pocketAway)
		s.pocketAway = nil
	}
}
