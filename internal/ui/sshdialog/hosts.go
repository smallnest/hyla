package sshdialog

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/marks"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Host is a machine the sidebar's SSH project lists: the name the user
// gave it, the address its ssh runs to, and the terminal it has open, if
// any. The app keeps the machines in its prefs.
type Host struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Session string `json:"session,omitempty"`
}

// Drag is a machine dragged by its row to another place in the SSH
// project.
type Drag struct {
	Addr string
}

// App is what the machines of the SSH project ask of the app, and the
// dialog that adds one.
type App interface {
	// Settings are the settings the rows follow.
	Settings() *settings.Settings
	// PrefsSSHProject is the project the machines' terminals open in, and
	// PrefsSSHHosts the machines; SavePrefs keeps them.
	PrefsSSHProject() string
	SetPrefsSSHProject(id string)
	PrefsSSHHosts() []Host
	SetPrefsSSHHosts(hs []Host)
	SavePrefs()
	// Local is this machine, Session and Project find a session and a
	// project the app knows, and SelectSession shows and CloseSession
	// closes one.
	Local() *host.Host
	Session(id string) *proto.Session
	ProjectNow(id string) *proto.Project
	SelectSession(id string)
	CloseSession(id string, ask bool)
	// AddProject shows a project the app's state does not have yet, and
	// AddPendingSSH holds its first terminal.
	AddProject(p proto.Project)
	AddPendingSSH(s proto.Session)
	// OpenProject unfolds a project, as its first machine's terminal
	// comes.
	OpenProject(id string)
	// Main runs fn on the main thread, RunLater runs fn after the frame,
	// Toast tells a failure, and UndoSays says what a removal took away,
	// with an undo.
	Main(fn func())
	RunLater(fn func())
	Toast(msg string)
	UndoSays(msg string, undo func())
	// Prompt is the dialog that adds a machine, which SetPrompt opens and
	// closes, and FocusTerminal takes the keyboard back as it closes.
	Prompt() *Ask
	SetPrompt(d *Ask)
	FocusTerminal()
	// CopyAddress copies a machine's address, and Confirm asks the app's
	// confirmation.
	CopyAddress(addr string)
	ConfirmNow(q *confirm.Ask)
	// DragAt is the machine an insertion line last showed, and DragAfter
	// the side of it; SetDragAt keeps them.
	DragAt() string
	DragAfter() bool
	SetDragAt(addr string, after bool)
	// SelectedSession is the session the window shows, and SidebarRows
	// the kind of row the sidebar draws.
	SelectedSession() *proto.Session
	SidebarRows() string
}

// Show draws the dialog while it is open, and adds the machine and
// connects to it once the user says so.
func Show(a App, c *ui.Context, pal *theme.Palette) {
	d := a.Prompt()
	if View(c, pal, d) {
		Add(a)
	}
	if d != nil && !d.Open {
		a.SetPrompt(nil)
		a.FocusTerminal()
	}
}

// Project is the sidebar's SSH project, nil while there is none: the
// sidebar may have lost its id, as when the project was closed.
func Project(a App) *proto.Project {
	if a.PrefsSSHProject() == "" {
		return nil
	}
	return a.ProjectNow(a.PrefsSSHProject())
}

// Name is what the sidebar's SSH project is called, in the language the
// app shows as it is made.
func Name() string { return locales.SshHostsProject() }

// Add adds the machine the dialog names to the SSH project, which it makes
// if there is none, and connects to it. The machine it reaches needs
// nothing but an SSH server: ssh runs in a terminal of this one.
func Add(a App) {
	d := a.Prompt()
	if d == nil {
		return
	}
	text := strings.TrimSpace(d.Addr)
	if text == "" {
		d.Err = locales.SetHostNeedsAddress()
		return
	}
	// The dialog is for SSH terminals: the other ways of reaching a
	// machine, Hyla hosts among them, are Settings › Hyla Hosts, and say
	// so for an address of another scheme before it is read.
	if i := strings.Index(text, "://"); i > 0 && !strings.EqualFold(text[:i], "ssh") {
		d.Err = locales.SshHostOnly()
		return
	}
	addr, err := remote.ParseAddr(text)
	if err != nil {
		d.Err = err.Error()
		return
	}
	if addr.Scheme != "ssh" {
		d.Err = locales.SshHostOnly()
		return
	}
	if a.Local().Client == nil {
		d.Err = locales.SshNoService()
		return
	}
	name := strings.TrimSpace(d.Name)
	if name == "" {
		name = DefaultName(addr.String())
	}
	// The address the project lists already: its terminal comes, rather
	// than a second row for the same machine.
	for _, h := range a.PrefsSSHHosts() {
		if h.Addr == addr.String() {
			d.Open = false
			showOrStart(a, h)
			return
		}
	}
	d.Open = false
	withProject(a, func(p proto.Project) { addTo(a, p, name, addr) })
}

// withProject runs fn with the sidebar's SSH project, making it first if
// there is none, off the UI thread as the daemon call is.
func withProject(a App, fn func(proto.Project)) {
	if p := Project(a); p != nil {
		fn(*p)
		return
	}
	h := a.Local()
	// Its state has not come: the project may be one the app cannot see
	// yet, and a second one would follow it.
	if h.Client == nil || !h.Loaded {
		a.Toast(locales.SshNoService())
		return
	}
	cl := h.Client
	go func() {
		p, err := makeProject(cl, Name())
		a.Main(func() {
			if err != nil {
				a.Toast(err.Error())
				return
			}
			a.SetPrefsSSHProject(p.ID)
			a.SavePrefs()
			// The project shows as its first terminal comes, before the
			// daemon's state, which has it, does.
			if a.ProjectNow(p.ID) == nil {
				a.AddProject(p)
			}
			fn(p)
		})
	}()
}

// makeProject makes the sidebar's SSH project: one of this machine's,
// named for the sidebar, with no directory of its own, as its sessions
// run wherever ssh does.
func makeProject(cl *client.Client, name string) (proto.Project, error) {
	var p proto.Project
	err := cl.Call("project.create", proto.ProjectParams{Name: name}, &p)
	return p, err
}

// addTo adds a machine to the SSH project and connects to it.
func addTo(a App, p proto.Project, name string, addr remote.Addr) {
	hs := a.PrefsSSHHosts()
	a.SetPrefsSSHHosts(append(hs, Host{Name: name, Addr: addr.String()}))
	a.SavePrefs()
	showOrStart(a, Host{Name: name, Addr: addr.String()})
}

// showOrStart shows the terminal a machine's row has, if it still runs,
// else starts one: a row's click.
func showOrStart(a App, hst Host) {
	if s := a.Session(hst.Session); s != nil && s.Status != proto.Exited {
		a.SelectSession(s.ID)
		return
	}
	Start(a, hst)
}

// ConnectTo connects to a machine again, in the row it was read from,
// whatever its terminal does: one still open closes, and the new terminal
// takes its place. The row is the machine's, whatever its terminal's end
// does to the list, and the address finds it once the new one comes.
func ConnectTo(a App, hst Host) {
	if s := a.Session(hst.Session); s != nil {
		a.CloseSession(s.ID, false)
	}
	Start(a, hst)
}

// Disconnect ends a machine's terminal, which ends the ssh running in it,
// and leaves the row: a click on it starts a new terminal, and with it a
// new connection.
func Disconnect(a App, addr string) {
	hs := a.PrefsSSHHosts()
	i := slices.IndexFunc(hs, func(h Host) bool { return h.Addr == addr })
	if i < 0 {
		return
	}
	id := hs[i].Session
	if id == "" {
		return
	}
	// The row has no terminal from here, whatever its end does to the
	// list.
	hs[i].Session = ""
	a.SetPrefsSSHHosts(hs)
	a.SavePrefs()
	if s := a.Session(id); s != nil {
		a.CloseSession(id, false)
	}
}

// Start starts a machine's terminal, in the SSH project.
func Start(a App, hst Host) {
	addr, err := remote.ParseAddr(hst.Addr)
	if err != nil {
		return
	}
	h := a.Local()
	if h.Client == nil {
		a.Toast(locales.SshNoService())
		return
	}
	withProject(a, func(p proto.Project) {
		cl := h.Client
		go func() {
			var s proto.Session
			err := cl.Call("create", proto.CreateParams{Project: p.ID, Dir: proto.Home(), Name: hst.Name, Command: Args(addr)}, &s)
			a.Main(func() {
				if err != nil {
					a.Toast(err.Error())
					return
				}
				// The row may have gone as the terminal came, and another
				// may have taken its place: the address finds it.
				hs := a.PrefsSSHHosts()
				i := slices.IndexFunc(hs, func(h Host) bool { return h.Addr == hst.Addr })
				if i < 0 {
					return
				}
				hs[i].Session = s.ID
				a.SetPrefsSSHHosts(hs)
				a.SavePrefs()
				a.OpenProject(p.ID)
				a.AddPendingSSH(s)
				a.SelectSession(s.ID)
			})
		}()
	})
}

// OfSession is the machine a session is the terminal of, nil for none.
func OfSession(a App, id string) *Host {
	if id == "" {
		return nil
	}
	hs := a.PrefsSSHHosts()
	for i := range hs {
		if hs[i].Session == id {
			return &hs[i]
		}
	}
	return nil
}

// Move puts the machine named by from where the machine named by to is,
// before or after it, and keeps the order in the app's preferences.
func Move(a App, from, to string, after bool) {
	at := func(addr string) int {
		return slices.IndexFunc(a.PrefsSSHHosts(), func(h Host) bool { return h.Addr == addr })
	}
	i, j := at(from), at(to)
	if from == "" || from == to || i < 0 || j < 0 {
		return
	}
	hs := a.PrefsSSHHosts()
	m := hs[i]
	hs = slices.Delete(hs, i, i+1)
	j = slices.IndexFunc(hs, func(h Host) bool { return h.Addr == to })
	if j < 0 {
		return
	}
	if after {
		j++
	}
	hs = slices.Insert(hs, j, m)
	a.SetPrefsSSHHosts(hs)
	a.SavePrefs()
}

// Remove removes a machine from the SSH project, ending its terminal as
// closing one does.
func Remove(a App, addr string) {
	hs := a.PrefsSSHHosts()
	for i, h := range hs {
		if h.Addr != addr {
			continue
		}
		hs = slices.Delete(hs, i, i+1)
		a.SetPrefsSSHHosts(hs)
		a.SavePrefs()
		a.UndoSays(locales.SshHostRemoved(h.Addr), func() {
			hs := a.PrefsSSHHosts()
			a.SetPrefsSSHHosts(slices.Insert(hs, min(i, len(hs)), h))
			a.SavePrefs()
		})
		if h.Session != "" {
			// The question above said it goes.
			a.CloseSession(h.Session, false)
		}
		return
	}
}

// Clear forgets the SSH project, whose terminals go with it.
func Clear(a App) {
	a.SetPrefsSSHProject("")
	a.SetPrefsSSHHosts(nil)
	a.SavePrefs()
}

// Args is ssh's command line for an address as the user typed it: ssh -p
// 2222 root@example.com.
func Args(addr remote.Addr) []string {
	args := []string{"ssh"}
	if addr.Port != "" {
		args = append(args, "-p", addr.Port)
	}
	dest := addr.Host
	if addr.User != "" {
		dest = addr.User + "@" + addr.Host
	}
	return append(args, dest)
}

// markSlot is the room a row keeps before its name: as wide as a project's
// badge and the gap after it, less the gap the row itself leaves, so that
// the names of a project's sessions and its machines start at one place.
func markSlot(c *ui.Context, gap float32) ui.Element {
	return ui.Box(c).Size(Badge+BadgeGap-gap, marks.Size).Center().Shrink(0)
}

// statusSlot ends a row that stands for a terminal: its status, or, while
// the pointer is on the row, a close button in the status's fixed slot, so
// that hovering moves no text. close ends what the row's terminal runs.
func statusSlot(c *ui.Context, pal *theme.Palette, hovered bool, st proto.Status, unread bool, progress int, close func()) {
	ui.Box(c.Key("trailing")).Size(20, 20).Shrink(0).Center().Children(func() {
		if !hovered {
			paints.StatusP(c, pal, st, unread, progress, 15)
			return
		}
		b := ui.ButtonBase(c.Key("close")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).
			Label(locales.ActionCloseSession()).Tooltip(locales.ActionCloseSession())
		if b.Hovered() {
			b.Background(pal.RowHover).TextColor(pal.Text)
		}
		b.Children(func() { ui.Icon(c, glyph.Close).Size(13, 13) })
		if b.Clicked() {
			close()
		}
	})
}

// Rows draws the machines of the SSH project, after a project's sessions,
// and the row that adds one where there is none yet.
func Rows(a App, c *ui.Context, pal *theme.Palette) {
	hs := a.PrefsSSHHosts()
	for i := range hs {
		row(a, c, pal, i)
	}
	if len(hs) == 0 {
		empty(a, c, pal)
	}
}

// Row draws a machine in the SSH project: its name over the address ssh
// runs to, as a session's row has what it is at under its name; or on one
// line, as a compact session's row. A click shows its terminal, or starts
// one.
func Row(a App, c *ui.Context, pal *theme.Palette, i int) { row(a, c, pal, i) }

// row draws a machine in the SSH project: its name over the address ssh
// runs to, as a session's row has what it is at under its name; or on one
// line, as a compact session's row. A click shows its terminal, or starts
// one.
func row(a App, c *ui.Context, pal *theme.Palette, index int) {
	h := a.PrefsSSHHosts()[index]
	project := a.PrefsSSHProject()
	s := a.Session(h.Session)
	if s != nil && s.Project != project {
		s = nil
	}
	sel := a.SelectedSession()
	selected := s != nil && sel != nil && s.ID == sel.ID
	inset := float32(RowInset)
	detailed := a.Settings().SidebarRows != "compact"
	height := float32(RowH)
	if detailed {
		height = RowH2
	}
	r := ui.Row(c.Key("ssh-"+h.Addr)).Height(height).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(Gap).AlignItems(ui.Center).Radius(8)
	r.Border(1, ui.Transparent)
	bg := ui.Transparent
	switch {
	case selected:
		bg = theme.TranslucentGlass(pal.RowSelected, pal, *a.Settings())
		style.Chosen(r, pal)
	case r.Hovered():
		bg = theme.TranslucentGlass(pal.RowHover, pal, *a.Settings())
	}
	r.Background(r.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	// The row is its machine's handle: dragged, it moves the machine to
	// another place in the project, its terminal with it.
	r.Drag(Drag{Addr: h.Addr}).Cursor(ui.CursorGrab)
	if r.Dragging() {
		r.Opacity(0.35)
	}
	if d, over := ui.DragOver[Drag](r); over && d.Addr != h.Addr {
		_, y, _ := r.PointerPosition()
		after := y > height/2
		a.SetDragAt(h.Addr, after)
		r.Background(pal.RowHover)
		insertLine(r, pal, after)
	}
	if d, ok := ui.Drop[Drag](r); ok && d.Addr != h.Addr {
		_, y, _ := r.PointerPosition()
		after := y > height/2
		if a.DragAt() == h.Addr {
			// The side the insertion line last showed.
			after = a.DragAfter()
		}
		to := h.Addr
		from := d.Addr
		a.RunLater(func() { Move(a, from, to, after) })
	}
	paints.PointerTooltip(c, r, "tip-ssh-"+h.Addr, h.Addr)
	r.ContextMenu(func(m *ui.Menu) {
		// Reconnecting goes on a terminal that ended, as ssh's does when
		// the connection drops, and starts the terminal anew, or replaces
		// one still open: a click on the row shows that one. The row it
		// read is the machine's, whatever its terminal's end does to the
		// list.
		if m.Item(locales.ActionReconnectHost()).Chosen() {
			ConnectTo(a, h)
		}
		// Disconnecting ends the machine's terminal, and ssh with it, but
		// keeps its row: a click on it starts a new terminal.
		if m.Item(locales.ActionDisconnectHost()).Disabled(s == nil || s.Status == proto.Exited).Chosen() {
			Disconnect(a, h.Addr)
		}
		m.Separator()
		if m.Item(locales.HostCopyAddress()).Chosen() {
			a.CopyAddress(h.Addr)
		}
		if m.Item(locales.HostRemove()).Chosen() {
			name, addr := h.Name, h.Addr
			a.ConfirmNow(&confirm.Ask{
				Open:    true,
				Title:   locales.SshRemoveTitle(name),
				Detail:  locales.SshRemoveDetail(),
				Action:  locales.CommonRemove(),
				Destroy: true,
				Run:     func() { Remove(a, addr) },
			})
		}
	})
	if r.Clicked() && !selected {
		showOrStart(a, h)
	}
	r.Children(func() {
		// The machine's mark stands on the row's first line, beside its
		// name, as an agent's stands beside a session's; the address below
		// starts under the name, past the mark.
		mark := func(gap float32) {
			markSlot(c, gap).Children(func() {
				ui.Icon(c, glyph.Terminal).Size(marks.Size, marks.Size).TextColor(pal.TextMuted)
			})
		}
		top := func() {
			ui.Text(c, h.Name).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		}
		if !detailed {
			mark(Gap)
			top()
			ui.Spacer(c)
			ui.Text(c, h.Addr).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(2).MaxWidth(220)
		} else {
			// The name and the address over the row's width, as a detailed
			// session's row has its name and detail.
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					mark(Gap)
					top()
				})
				addr := ui.Text(c, h.Addr).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").MinWidth(0)
				addr.Padding(0, 0, 0, Badge+BadgeGap)
			})
		}
		if s == nil {
			// A machine with no terminal yet keeps the slot the status
			// (or, under the pointer, the close button) takes.
			ui.Box(c).Size(20, 20).Shrink(0)
			return
		}
		// Close ends the terminal, as Remove does, without its question.
		statusSlot(c, pal, r.Hovered(), s.Status, s.Unread, s.Progress, func() {
			addr := h.Addr
			a.RunLater(func() { Remove(a, addr) })
		})
	})
}

// empty is the SSH project with no machine in it yet: the row that adds
// one, in place of a project's sessions.
func empty(a App, c *ui.Context, pal *theme.Palette) {
	inset := float32(RowInset)
	r := ui.Row(c.Key("ssh-empty")).Height(RowH).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(Gap).AlignItems(ui.Center).Radius(8)
	if r.Hovered() {
		r.Background(pal.RowHover)
	}
	if r.Clicked() {
		a.SetPrompt(&Ask{Open: true})
	}
	r.Children(func() {
		markSlot(c, Gap).Children(func() {
			ui.Icon(c, glyph.Plus).Size(13, 13).TextColor(pal.TextMuted)
		})
		ui.Text(c, locales.ActionAddSSHHost()).FontSize(13).TextColor(pal.TextMuted).SingleLine()
	})
}

// The sidebar's rows, as the SSH project's draw them: a row's height, its
// inset and the gap between its parts, and the square before a project's
// name with the room after it.
const (
	RowH     = 34
	RowH2    = 48 // a detailed row, of two lines
	RowInset = 12
	Gap      = 10
	Badge    = 20
	BadgeGap = 6
)

// insertLine draws the line that tells where a dragged row lands: along
// its top edge, or its bottom one to go after it.
func insertLine(r ui.Element, pal *theme.Palette, after bool) {
	r.Draw(func(p *ui.Painter, box ui.Rect) {
		y := box.Y
		if after {
			y = box.Y + box.H - 2
		}
		p.Fill(ui.Rect{X: box.X, Y: y, W: box.W, H: 2}, pal.Accent, 1)
	})
}
