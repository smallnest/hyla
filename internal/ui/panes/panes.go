// Package panes holds what a session's panes are to the app: the actions
// that split them, close them, zoom one and move the user between them.
package panes

import (
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// App is what the actions ask of the app they change.
type App interface {
	// HostOf is the machine a session runs on.
	HostOf(id string) *host.Host
	// Session is a session the app knows, nil for none, and
	// SelectedSession the one the window shows: the app's Selected.
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
	// DropTerms, DropWeb and DropHongguo drop a pane's terminal, page and drama player.
	DropTerms(pane string)
	DropWeb(pane string)
	DropHongguo(pane string)
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
	// Update runs fn on the main thread, RunLater runs fn after the
	// frame, and Toast tells a failure.
	Update(fn func())
	RunLater(fn func())
	Toast(msg string)

	// Chrome is where the sidebar's animation puts things, and
	// HeaderHeight is the height of a card's header.
	ChromeForPanel() Chrome
	HeaderHeightIn() float32

	// PaneChat are the panes the palette gave their session's chat in
	// their terminal's place.
	PaneChat() map[string]bool

	// FilePane, PaneBrowser, SubagentsBody, HongguoBody, ChatBody and PluginPaneBody are
	// the card's body, by kind.
	FilePane(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane)
	SubagentsBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane)
	HongguoBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane)
	PaneBrowser(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, top, left, right, multi, focused, hovered bool) (dragging bool)
	ChatBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane)
	PluginPaneBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane)
	PaneContextMenu(m *ui.Menu, s *proto.Session, p *proto.Pane)

	// Term is a pane's terminal, as it attaches; ImagePeek peeks at a
	// path under the pointer, and FindBar shows its find.
	Term(pane string) *TermEntry
	ImagePeek(pane, link string, at, term ui.Rect)
	FindPane() string
	FindBar(c *ui.Context, pal *theme.Palette, p *proto.Pane, t *terminal.Terminal)
	// PaneMessage says what a pane cannot show.
	PaneMessage(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func())

	// LoadPaneArt, PaneArt and Settings' PaneImage give a terminal's
	// picture.
	LoadPaneArt(path string)
	PaneArt() Art
	// StartRename and RestartSession act on a session, and PasteDrop
	// takes files dropped on a pane.
	StartRename(id string)
	RestartSession(id string)
	PasteDrop(pane string, files []string)
	// GitChip draws a pane's branch and its changes, after its title.
	GitChip(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane)
	// WindowControls draws the window's buttons on the right of its title.
	WindowCtl(c *ui.Context, pal *theme.Palette)
	// ToggleSidebar and NewSession are the sidebar's buttons.
	ToggleSidebar()
	NewSessionIn(projectID string)

	// HoverPane, HoverAt, Arrived and LastZone follow a pane dragged.
	HoverPane() string
	SetHoverPane(id string)
	HoverAt() [2]float32
	SetHoverAt(at [2]float32)
	Arrived(pane string) bool
	TakeArrived(pane string)
	SetArrived(pane string)
	LastZone() Zone
	SetLastZone(z Zone)
	// Preview is the room the dragged pane would take, and the pane it
	// was of, which DrawnPreview notes as drawn.
	Preview() Preview
	SetPreview(pane string, since time.Time)
	DrawnPreview()

	// LayoutDrag, SetLayoutDrag and ClearLayoutDrag keep a divider's
	// drag, and ClearLayoutDrag' key tells the split it is of.
	LayoutDrag(split string) (held bool, ratio float64)
	LayoutDragKey() string
	SetLayoutDrag(session, split string, ratio float64)
	ClearLayoutDrag()

	// Tip adds an action's shortcut, in words, to a title.
	Tip(title, id string) string
}

// Chrome is where the sidebar's animation puts things.
type Chrome struct {
	SideW     float32 // the sidebar's width as shown
	MainPad   float32 // the left padding of the cards' column
	HeaderPad float32 // the left padding of the header at the left edge
	// Lead is where the sidebar's buttons start in the window, after the
	// traffic lights on macOS; elsewhere inside the card's edge.
	Lead float32
}

// ButtonsLeft places the sidebar's buttons that the header holds while
// the sidebar hides, in the header's coordinates.
func (ch Chrome) ButtonsLeft() float32 { return ch.Lead - (ch.SideW + ch.MainPad) }

// FeishuOn tells that an app of Feishu is configured, whose chats the
// panes show (chat.go).
// TermEntry is a pane's terminal as it attaches: the terminal, or why it
// did not.
type TermEntry struct {
	Term    *terminal.Terminal
	Err     string
	Started time.Time
}

// Art is a picture behind the terminals.
type Art struct {
	Path string
	BM   *ui.Bitmap
	Card *ui.Color
}

// Preview is the pane a drop preview is of, and since when it shows.
type Preview struct {
	Key   string
	Since time.Time
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
	if pane := s.PaneByID(p.Split); pane != nil && !pane.Shown() {
		p.Cols, p.Rows = splitDimensions(pane, p.Direction)
	}
	callSessionSplit(x, s.ID, "split", p)
}

// ForkPane forks a pane's active coding agent session into a new split pane.
func ForkPane(x App, s *proto.Session, at, direction string) {
	if s == nil {
		return
	}
	if at == "" {
		at = x.FocusOf(s)
	}
	pane := s.PaneByID(at)
	if pane == nil || !pane.CanForkSession() {
		return
	}
	if direction == "" {
		direction = "right"
	}
	cols, rows := splitDimensions(pane, direction)
	callSessionSplit(x, s.ID, "pane.fork", proto.ForkParams{
		Pane:      at,
		Direction: direction,
		Cols:      cols,
		Rows:      rows,
	})
}

func splitDimensions(pane *proto.Pane, direction string) (cols, rows int) {
	cols, rows = pane.Cols, pane.Rows
	if direction == "down" || direction == "up" {
		rows = max(rows/2, 4)
	} else {
		cols = max(cols/2, 10)
	}
	return cols, rows
}

func callSessionSplit(x App, sessionID, method string, params any) {
	h := x.HostOf(sessionID)
	cl := h.Client
	if cl == nil {
		return
	}
	go func() {
		var ns proto.Session
		err := cl.Call(method, params, &ns)
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
	x.DropHongguo(pane)
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
