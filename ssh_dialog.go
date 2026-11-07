package main

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"

	"hyla/locales"
)

// sshAsk is the dialog that adds an SSH terminal to the sidebar: the
// address the user types, as ssh takes it ("root@example.com:2222"), and
// the name the SSH project gives the machine, the address's host while
// the user types none.
type sshAsk struct {
	open       bool
	addr, name string
	err        string
}

// sshHost is a machine the sidebar's SSH project lists: the name the user
// gave it, the address its ssh runs to, and the terminal it has open, if
// any.
type sshHost struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Session string `json:"session,omitempty"`
}

// askSSHHost opens the dialog.
func (a *App) askSSHHost() {
	if a.sshAsk == nil {
		a.sshAsk = &sshAsk{open: true}
	}
}

// sshView draws the dialog while it is open, and adds the machine and
// connects to it once the user says so.
func (a *App) sshView(c *ui.Context, pal *palette) {
	d := a.sshAsk
	if d == nil {
		return
	}
	connect := false
	ui.DialogBase(c.Key("ssh"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.overlay)
		floating(panel.Width(420).Padding(22).Gap(8).Radius(14).Background(pal.popover), pal).Label(locales.ActionAddSSHHost())
		ui.Text(c, locales.ActionAddSSHHost()).FontSize(15).FontWeight(650).TextColor(pal.text)
		ui.Text(c, locales.SshHostDetail()).FontSize(13).TextColor(pal.textMuted).LineHeight(1.4)
		ui.Row(c).Margin(8, 0, 1, 0).Children(func() {
			in := textField(c, "ssh-addr", &d.addr, locales.SshHostAddressPlaceholder(), 0, pal).Font("monospace")
			in.AutoFocus()
			if in.Submitted() {
				connect = true
			}
		})
		ui.Row(c).Children(func() {
			in := textField(c, "ssh-name", &d.name, locales.SetHostNamePlaceholder(sshDefaultName(d.addr)), 0, pal)
			if in.Submitted() {
				connect = true
			}
		})
		if d.err != "" {
			ui.Text(c, d.err).FontSize(12).TextColor(pal.danger)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Margin(14, 0, 0, 0).Children(func() {
			if button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				d.open = false
			}
			if button(c, "ok", locales.CommonConnect(), "↩", true, pal) {
				connect = true
			}
		})
	})
	if connect {
		a.addSSHHost()
	}
	if !d.open {
		a.sshAsk = nil
		a.focusTerminal()
	}
}

// sshDefaultName is the name a machine takes while the user types none:
// the address's host, as config names it.
func sshDefaultName(addr string) string {
	a, err := remote.ParseAddr(addr)
	if err != nil || a.Host == "" {
		return "devbox"
	}
	return a.Host
}

// sshProjectName is what the sidebar's SSH project is called, in the
// language the app shows as it is made.
func sshProjectName() string { return locales.SshHostsProject() }

// sshProjectOf is the sidebar's SSH project, nil while there is none: the
// sidebar may have lost its id, as when the project was closed.
func (a *App) sshProjectOf() *proto.Project {
	if a.prefs.SSHProject == "" {
		return nil
	}
	return a.project(a.prefs.SSHProject)
}

// addSSHHost adds the machine the dialog names to the SSH project, which
// it makes if there is none, and connects to it. The machine it reaches
// needs nothing but an SSH server: ssh runs in a terminal of this one.
func (a *App) addSSHHost() {
	d := a.sshAsk
	if d == nil {
		return
	}
	text := strings.TrimSpace(d.addr)
	if text == "" {
		d.err = locales.SetHostNeedsAddress()
		return
	}
	addr, err := remote.ParseAddr(text)
	if err != nil {
		d.err = err.Error()
		return
	}
	// The dialog is for SSH terminals: the other ways of reaching a
	// machine, Hyla hosts among them, are Settings › Hyla Hosts.
	if addr.Scheme != "ssh" {
		d.err = locales.SshHostOnly()
		return
	}
	if a.local().cl == nil {
		d.err = locales.SshNoService()
		return
	}
	name := strings.TrimSpace(d.name)
	if name == "" {
		name = sshDefaultName(addr.String())
	}
	// The address the project lists already: its terminal comes, rather
	// than a second row for the same machine.
	for i := range a.prefs.SSHHosts {
		if a.prefs.SSHHosts[i].Addr == addr.String() {
			d.open = false
			a.connectSSH(i)
			return
		}
	}
	d.open = false
	a.withSSHProject(func(p proto.Project) { a.addSSHHostTo(p, name, addr) })
}

// withSSHProject runs fn with the sidebar's SSH project, making it first
// if there is none, off the UI thread as the daemon call is.
func (a *App) withSSHProject(fn func(proto.Project)) {
	if p := a.sshProjectOf(); p != nil {
		fn(*p)
		return
	}
	h := a.local()
	// Its state has not come: the project may be one the app cannot see
	// yet, and a second one would follow it.
	if h.cl == nil || !h.loaded {
		a.toast(locales.SshNoService())
		return
	}
	cl := h.cl
	go func() {
		p, err := sshProject(cl, sshProjectName())
		a.main(func() {
			if err != nil {
				a.toast(err.Error())
				return
			}
			a.prefs.SSHProject = p.ID
			a.savePrefs()
			// The project shows as its first terminal comes, before the
			// daemon's state, which has it, does.
			if a.project(p.ID) == nil {
				a.st.Projects = append(a.st.Projects, p)
			}
			fn(p)
		})
	}()
}

// sshProject makes the sidebar's SSH project: one of this machine's,
// named for the sidebar, with no directory of its own, as its sessions
// run wherever ssh does.
func sshProject(cl *client.Client, name string) (proto.Project, error) {
	var p proto.Project
	err := cl.Call("project.create", proto.ProjectParams{Name: name}, &p)
	return p, err
}

// addSSHHostTo adds a machine to the SSH project and connects to it.
func (a *App) addSSHHostTo(p proto.Project, name string, addr remote.Addr) {
	a.prefs.SSHHosts = append(a.prefs.SSHHosts, sshHost{Name: name, Addr: addr.String()})
	a.savePrefs()
	a.connectSSH(len(a.prefs.SSHHosts) - 1)
}

// connectSSH shows a machine's terminal, starting one in the SSH project
// if it has none.
func (a *App) connectSSH(i int) {
	if i < 0 || i >= len(a.prefs.SSHHosts) {
		return
	}
	a.showOrStart(a.prefs.SSHHosts[i])
}

// showOrStart shows the terminal a machine's row has, if it still
// runs, else starts one: a row's click.
func (a *App) showOrStart(host sshHost) {
	if s := a.session(host.Session); s != nil && s.Status != proto.Exited {
		a.selectSession(s.ID)
		return
	}
	a.startSSH(host)
}

// connectSSHTo connects to a machine again, in the row it was read from,
// whatever its terminal does: one still open closes, and the new
// terminal takes its place. The row is the machine's, whatever its
// terminal's end does to the list, and the address finds it once the new
// one comes.
func (a *App) connectSSHTo(host sshHost) {
	if s := a.session(host.Session); s != nil {
		a.closeSession(s.ID, false)
	}
	a.startSSH(host)
}

// startSSH starts a machine's terminal, in the SSH project.
func (a *App) startSSH(host sshHost) {
	addr, err := remote.ParseAddr(host.Addr)
	if err != nil {
		return
	}
	h := a.local()
	if h.cl == nil {
		a.toast(locales.SshNoService())
		return
	}
	a.withSSHProject(func(p proto.Project) {
		cl := h.cl
		go func() {
			var s proto.Session
			err := cl.Call("create", proto.CreateParams{Project: p.ID, Dir: proto.Home(), Name: host.Name, Command: sshArgs(addr)}, &s)
			a.main(func() {
				if err != nil {
					a.toast(err.Error())
					return
				}
				// The row may have gone as the terminal came, and another
				// may have taken its place: the address finds it.
				i := slices.IndexFunc(a.prefs.SSHHosts, func(h sshHost) bool { return h.Addr == host.Addr })
				if i < 0 {
					return
				}
				a.prefs.SSHHosts[i].Session = s.ID
				a.savePrefs()
				delete(a.prefs.Collapsed, p.ID)
				a.addPending(h, s)
				a.selectSession(s.ID)
			})
		}()
	})
}

// sshHostOfSession is the machine a session is the terminal of, nil for
// none.
func (a *App) sshHostOfSession(id string) *sshHost {
	if id == "" {
		return nil
	}
	for i := range a.prefs.SSHHosts {
		if a.prefs.SSHHosts[i].Session == id {
			return &a.prefs.SSHHosts[i]
		}
	}
	return nil
}

// removeSSHHost removes a machine from the SSH project, ending its
// terminal as closing one does.
func (a *App) removeSSHHost(addr string) {
	for i, h := range a.prefs.SSHHosts {
		if h.Addr != addr {
			continue
		}
		a.prefs.SSHHosts = slices.Delete(a.prefs.SSHHosts, i, i+1)
		a.savePrefs()
		if h.Session != "" {
			// The question above said it goes.
			a.closeSession(h.Session, false)
		}
		return
	}
}

// clearSSHProject forgets the SSH project, whose terminals go with it.
func (a *App) clearSSHProject() {
	a.prefs.SSHProject, a.prefs.SSHHosts = "", nil
	a.savePrefs()
}

// sshArgs is ssh's command line for an address as the user typed it: ssh
// -p 2222 root@example.com.
func sshArgs(addr remote.Addr) []string {
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

// sshHostRow is a machine in the SSH project: its name over the address
// ssh runs to, as a session's row has what it is at under its name. A
// click shows its terminal, or starts one.
func (a *App) sshHostRow(c *ui.Context, pal *palette, i int) {
	h := a.prefs.SSHHosts[i]
	s := a.session(h.Session)
	if s != nil && s.Project != a.prefs.SSHProject {
		s = nil
	}
	selected := s != nil && s.ID == a.selected
	inset := float32(rowInset + projectBadge + projectBadgeGap)
	if pal.dark {
		inset++
	}
	row := ui.Row(c.Key("ssh-"+h.Addr)).Height(rowH2).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(10).AlignItems(ui.Center).Radius(8)
	if pal.dark {
		row.Border(1, ui.Transparent)
	}
	bg := ui.Transparent
	switch {
	case selected:
		bg = translucentGlass(pal.rowSelected, pal, a.set)
		chosen(row, pal)
	case row.Hovered():
		bg = translucentGlass(pal.rowHover, pal, a.set)
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	pointerTooltip(c, row, "tip-ssh-"+h.Addr, h.Addr)
	row.ContextMenu(func(m *ui.Menu) {
		// Reconnecting goes on a terminal that ended, as ssh's does when
		// the connection drops, and starts the terminal anew, or replaces
		// one still open: a click on the row shows that one. The row it
		// read is the machine's, whatever its terminal's end does to the
		// list.
		if m.Item(locales.ActionReconnectHost()).Chosen() {
			a.connectSSHTo(h)
		}
		m.Separator()
		if m.Item(locales.HostCopyAddress()).Chosen() {
			a.copyText = h.Addr
		}
		if m.Item(locales.HostRemove()).Chosen() {
			name, addr := h.Name, h.Addr
			a.confirm = &confirmation{
				open:    true,
				title:   locales.SshRemoveTitle(name),
				detail:  locales.SshRemoveDetail(),
				action:  locales.CommonRemove(),
				destroy: true,
				run:     func() { a.removeSSHHost(addr) },
			}
		}
	})
	if row.Clicked() && !selected {
		a.connectSSH(i)
	}
	row.Children(func() {
		ui.Icon(c, terminalIcon).Size(14, 14).TextColor(pal.textMuted).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, h.Name).FontSize(13.5).TextColor(pal.text).SingleLine().Ellipsis("…")
			ui.Text(c, h.Addr).Font("monospace").FontSize(12).TextColor(pal.textFaint).SingleLine().Ellipsis("…").MinWidth(0)
		})
		if s == nil {
			// A machine with no terminal yet keeps the slot the status
			// (or, under the pointer, the close button) takes.
			ui.Box(c).Size(20, 20).Shrink(0)
			return
		}
		// Close ends the terminal, as Remove does, without its question.
		statusSlot(c, pal, row.Hovered(), s.Status, s.Unread, s.Progress, func() {
			addr := h.Addr
			a.defer_(func() { a.removeSSHHost(addr) })
		})
	})
}

// sshEmpty is the SSH project with no machine in it yet: the row that
// adds one, in place of a project's sessions.
func (a *App) sshEmpty(c *ui.Context, pal *palette) {
	inset := float32(rowInset + projectBadge + projectBadgeGap)
	if pal.dark {
		inset++
	}
	row := ui.Row(c.Key("ssh-empty")).Height(rowH).Margin(0, 0, 1, 0).Padding(0, 10, 0, inset).Gap(10).AlignItems(ui.Center).Radius(8)
	if row.Hovered() {
		row.Background(pal.rowHover)
	}
	if row.Clicked() {
		a.askSSHHost()
	}
	row.Children(func() {
		ui.Icon(c, plusIcon).Size(13, 13).TextColor(pal.textMuted).Shrink(0)
		ui.Text(c, locales.ActionAddSSHHost()).FontSize(13).TextColor(pal.textMuted).SingleLine()
	})
}
