// Package hostlink holds what the app makes of the machines it reaches:
// the state it shows, merged from their hylads', and how the app stands
// with each of them.
package hostlink

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// App is what the machines' state asks of the app.
type App interface {
	// MachinesNow are the hosts the app holds, this machine's first, and
	// SessionsOf and ProjectsOf those of one host.
	MachinesNow() []*host.Host
	SessionsOf(h *host.Host) []proto.Session
	ProjectsOf(h *host.Host) []proto.Project
	// OwnerOf is which host each session, project and pane is of, and
	// SetOwner holds it.
	OwnerOf() map[string]*host.Host
	SetOwner(o map[string]*host.Host)
	// ConfirmRestart asks to restart a machine's hylad.
	ConfirmRestart(h *host.Host)
	// DaemonVersion is hylad's version, as this app knows it.
	DaemonVersion() string
}

// Merge makes st a host's part of the state the app shows, the hosts'
// parts in their order: what the app changed in the others before their
// hylads' states come stays.
func Merge(a App, h *host.Host, st proto.State) (projects []proto.Project, sessions []proto.Session, owner map[string]*host.Host) {
	owner = map[string]*host.Host{}
	for _, o := range a.MachinesNow() {
		ps, ss := st.Projects, st.Sessions
		if o != h {
			ps, ss = a.ProjectsOf(o), a.SessionsOf(o)
		}
		for _, p := range ps {
			owner[p.ID] = o
		}
		for _, s := range ss {
			owner[s.ID] = o
			for _, p := range s.Panes {
				owner[p.ID] = o
			}
		}
		projects = append(projects, ps...)
		sessions = append(sessions, ss...)
	}
	return projects, sessions, owner
}

// Own records a session the app just made as a host's, before its hylad's
// state, which has it, comes.
func Own(a App, h *host.Host, s proto.Session) {
	owner := a.OwnerOf()
	if owner == nil {
		owner = map[string]*host.Host{}
	}
	owner[s.ID] = h
	owner[s.Project] = h
	for _, p := range s.Panes {
		owner[p.ID] = h
	}
	a.SetOwner(owner)
}

// State says how the app stands with this machine's hylad: a dot while it
// connects or failed to, a chip when it wants restarting.
func State(a App, c *ui.Context, pal *theme.Palette, h *host.Host) {
	if h.Remote() {
		// Another machine's hylad is not this app's to restart: it says
		// where it stands, and the user reconnects to it.
		switch {
		case h.Conn == host.StateConnecting:
			dot(c, pal, pal.TextFaint, locales.StatusConnecting())
		case h.Conn == host.StateFailed:
			dot(c, pal, pal.Danger, h.Err)
		}
		return
	}
	switch {
	case h.Conn == host.StateConnecting && h.Loaded:
		dot(c, pal, pal.Attention, locales.HostReconnecting())
	case h.Conn == host.StateConnecting:
		dot(c, pal, pal.TextFaint, locales.StatusConnecting())
	case h.Conn == host.StateFailed:
		dot(c, pal, pal.Danger, locales.HostCantConnect(h.Name))
	case h.Conn == host.StateVersion || h.Stale():
		b := ui.ButtonBase(c.Key("restart")).Height(22).Padding(0, 7).Gap(5).Radius(11).AlignItems(ui.Center).
			Label(locales.CommonRestart()).Tooltip(locales.HostStaleTip(h.Name, h.DaemonApp, a.DaemonVersion()))
		if b.Hovered() {
			b.Background(pal.RowHover)
		}
		b.Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(pal.Attention)
			ui.Text(c, locales.CommonRestart()).FontSize(12).FontWeight(500).TextColor(pal.Attention).SingleLine()
		})
		if b.Clicked() {
			a.ConfirmRestart(h)
		}
	}
}

// dot says how the app stands with this machine's hylad: a dot, and its
// words as the tooltip.
func dot(c *ui.Context, pal *theme.Palette, col ui.Color, label string) {
	ui.Box(c).Size(7, 7).Radius(3.5).Background(col).Shrink(0).Margin(0, 4, 0, 0).Label(label).Tooltip(label)
}
