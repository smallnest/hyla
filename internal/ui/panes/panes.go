// Package panes holds what a session's panes are to the app: the actions
// that split them, close them, zoom one and move the user between them.
package panes

import (
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/locales"
)

// App is what the actions ask of the app they change.
type App interface {
	// HostOf is the machine a session runs on.
	HostOf(id string) *host.Host
	// Session is a session the app knows, nil for none, and
	// SelectedSession the one the window shows.
	Session(id string) *proto.Session
	SelectedSession() *proto.Session
	// FocusOf is the pane of a session the user is in, and SetFocus puts
	// the user in another.
	FocusOf(s *proto.Session) string
	SetFocus(s *proto.Session, pane string)
	// FocusMap is the panes the user is in, by session.
	FocusMap() map[string]string
	// Settings are the settings the app follows.
	Settings() *settings.Settings
	// SetConfirm asks the app's confirmation.
	SetConfirm(a *confirm.Ask)
	// CloseSession closes a session, by its id, asking first when asked
	// to.
	CloseSession(id string, ask bool)
	// DropTerms and DropWeb drop a pane's terminal and its page.
	DropTerms(pane string)
	DropWeb(pane string)
	// FocusTerminal gives the keyboard to the pane the user is in.
	FocusTerminal()
	// PaneRef is the handle a pane's terminal binds.
	PaneRef(id string) *ui.Handle
	// ReplaceSession puts a session as hylad answered into the app.
	ReplaceSession(h *host.Host, s proto.Session)
	// EnsureTerm and Touch keep a pane's terminal.
	EnsureTerm(pane string)
	Touch(pane string)
	// Shows tells that a pane shows a page or a file.
	Shows(pane string) bool
	// CallPane asks hylad, on the host a session or a pane is of.
	CallPane(id, method string, params any, done func(err error))
	// Update runs fn on the main thread, and Toast tells a failure.
	Update(fn func())
	Toast(msg string)
}

// SplitPane splits the pane the user is in, or at, along direction.
func SplitPane(x App, s *proto.Session, at, direction string) {
	if s == nil {
		return
	}
	if at == "" {
		at = x.FocusOf(s)
	}
	Split(x, s, proto.CreateParams{Split: at, Direction: direction})
}

// Split makes a pane beside another, as p asks, on the session's host,
// and puts the user in it.
func Split(x App, s *proto.Session, p proto.CreateParams) {
	if s == nil {
		return
	}
	h := x.HostOf(s.ID)
	if pane := s.PaneByID(p.Split); pane != nil && !pane.Shown() {
		p.Cols, p.Rows = pane.Cols, pane.Rows
		if p.Direction == "down" || p.Direction == "up" {
			p.Rows = max(p.Rows/2, 4)
		} else {
			p.Cols = max(p.Cols/2, 10)
		}
	}
	cl := h.Client
	if cl == nil {
		return
	}
	go func() {
		var ns proto.Session
		err := cl.Call("split", p, &ns)
		x.Update(func() {
			if err != nil {
				x.Toast(err.Error())
				return
			}
			x.ReplaceSession(h, ns)
			if cur := x.Session(ns.ID); cur != nil {
				x.FocusMap()[ns.ID] = ns.Focus
				x.EnsureTerm(ns.Focus)
				x.Touch(ns.Focus)
				if !x.Shows(ns.Focus) {
					// A browser takes the keyboard in its address bar, or
					// its page once it has one; a file's in its text.
					x.PaneRef(ns.Focus).Focus()
				}
			}
		})
	}()
}

// ClosePane closes a pane, asking first when asked to: the session with
// its last.
func ClosePane(x App, s *proto.Session, pane string, ask bool) {
	if s == nil {
		return
	}
	if len(s.Panes) <= 1 {
		x.CloseSession(s.ID, ask)
		return
	}
	p := s.PaneByID(pane)
	if p == nil {
		return
	}
	if ask && x.Settings().ConfirmClose && p.Status != proto.Exited && p.Process != "" {
		sid := s.ID
		x.SetConfirm(&confirm.Ask{
			Open:    true,
			Title:   locales.ConfirmClosePaneTitle(),
			Detail:  locales.ConfirmClosePaneDetail(p.Process),
			Action:  locales.PaneClose(),
			Destroy: true,
			Run: func() {
				if s := x.Session(sid); s != nil {
					ClosePane(x, s, pane, false)
				}
			},
		})
		return
	}
	// The neighbor taking the room takes the focus, as hylad does.
	if x.FocusOf(s) == pane && s.Layout != nil {
		for _, dir := range []string{"left", "up", "right", "down"} {
			if n := s.Layout.Neighbor(pane, dir); n != "" {
				x.FocusMap()[s.ID] = n
				break
			}
		}
	}
	s.Layout = s.Layout.Remove(pane)
	if s.Layout.Leaf() {
		s.Layout = nil
	}
	if s.Zoomed == pane {
		s.Zoomed = ""
	}
	for i := range s.Panes {
		if s.Panes[i].ID == pane {
			s.Panes = append(s.Panes[:i], s.Panes[i+1:]...)
			break
		}
	}
	x.DropTerms(pane)
	x.DropWeb(pane)
	x.CallPane(pane, "pane.close", proto.IDParams{ID: pane}, nil)
	x.FocusTerminal()
}

// ZoomPane shows a pane alone, or the layout again.
func ZoomPane(x App, s *proto.Session, pane string) {
	if s == nil || len(s.Panes) < 2 {
		return
	}
	if pane == "" {
		pane = x.FocusOf(s)
	}
	if s.Zoomed == pane {
		s.Zoomed = ""
	} else {
		s.Zoomed = pane
		x.SetFocus(s, pane)
	}
	x.CallPane(s.ID, "zoom", proto.FocusParams{Session: s.ID, Pane: pane}, nil)
	x.PaneRef(pane).Focus()
}

// FocusDirection moves into the pane next to the focused one.
func FocusDirection(x App, dir string) {
	s := x.SelectedSession()
	if s == nil || s.Layout == nil || s.Zoomed != "" {
		return
	}
	if n := s.Layout.Neighbor(x.FocusOf(s), dir); n != "" {
		x.SetFocus(s, n)
		x.PaneRef(n).Focus()
	}
}

// CyclePane moves to the next or previous pane, in the layout's order.
func CyclePane(x App, d int) {
	s := x.SelectedSession()
	if s == nil || len(s.Panes) < 2 {
		return
	}
	order := s.Layout.Panes()
	i := 0
	for j, id := range order {
		if id == x.FocusOf(s) {
			i = j
		}
	}
	n := order[(i+d+len(order))%len(order)]
	x.SetFocus(s, n)
	x.PaneRef(n).Focus()
}
