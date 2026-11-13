package main

import (
	"net"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"

	"hyla/locales"
)

// remoteHost is another machine Hyla reaches over its tcp:// listener, by
// the address and the password of that listener: the sidebar shows its
// sessions beside this machine's.
type remoteHost struct {
	Name  string `json:"name"`
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

// tcpDialer opens connections to a machine's tcp:// listener: the
// password first, which the listener admits, then Hyla's own protocol.
type tcpDialer struct {
	addr, token string
}

func (d tcpDialer) Dial() (net.Conn, error) {
	c, err := net.DialTimeout("tcp", d.addr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if _, err := c.Write([]byte(d.token + "\n")); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// syncRemoteHosts follows the machines the app reaches: it connects to
// those added, and lets go of those removed or changed.
func (a *App) syncRemoteHosts() {
	keep := []*host{a.local()}
	for _, r := range a.prefs.RemoteHosts {
		addr := strings.TrimSpace(r.Addr)
		if addr == "" || r.Name == "" {
			continue
		}
		h := a.hostNamed(r.Name)
		if h != nil && (h.addr != addr || h.token != r.Token) {
			a.closeHost(h)
			h = nil
		}
		if h == nil {
			h = &host{name: r.Name, addr: addr, token: r.Token, conn: connConnecting}
		}
		keep = append(keep, h)
	}
	added := []*host{}
	for _, h := range keep[1:] {
		if !slices.Contains(a.hosts, h) {
			added = append(added, h)
		}
	}
	for _, h := range a.hosts[1:] {
		if !slices.Contains(keep, h) {
			a.closeHost(h)
		}
	}
	a.hosts = keep
	a.mergeState(a.local(), proto.State{Projects: a.projectsOf(a.local()), Sessions: a.sessionsOf(a.local())})
	for _, h := range added {
		a.connectHost(h)
	}
	a.reselect()
	a.updateTitle()
}

// closeHost lets go of a remote host: its connection, its terminals, and
// its sessions in the sidebar.
func (a *App) closeHost(h *host) {
	h.gen++
	a.dropTerms(h)
	if h.cl != nil {
		go h.cl.Close()
		h.cl = nil
	}
	a.mergeState(h, proto.State{})
	h.loaded = false
}

// addRemoteHost saves a machine and connects to it.
func (a *App) addRemoteHost(name, addr, token string) error {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(addr), "tcp://"))
	token = strings.TrimSpace(token)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return errNoHostAddress
	}
	if name == "" {
		name = host
	}
	for _, r := range a.prefs.RemoteHosts {
		if strings.EqualFold(r.Name, name) {
			return errHostNamed(name)
		}
	}
	if token == "" {
		return errNoHostToken
	}
	a.prefs.RemoteHosts = append(a.prefs.RemoteHosts, remoteHost{Name: name, Addr: addr, Token: token})
	a.savePrefs()
	a.syncRemoteHosts()
	return nil
}

// removeRemoteHost forgets a machine, and its sessions.
func (a *App) removeRemoteHost(name string) {
	a.prefs.RemoteHosts = slices.DeleteFunc(a.prefs.RemoteHosts, func(r remoteHost) bool {
		return strings.EqualFold(r.Name, name)
	})
	a.savePrefs()
	a.syncRemoteHosts()
}

// hostErrors are why a machine cannot be added as it is given.
var (
	errNoHostToken   = errString(locales.SetHostNeedsToken())
	errNoHostAddress = errString(locales.SetHostBadAddress())
	errHostNamed     = func(name string) error { return errString(locales.SetHostNameTaken(name)) }
)

type errString string

func (e errString) Error() string { return string(e) }

// hostDraft is the form that adds another machine: its name, its
// listener's address and its password, and what went wrong.
type hostDraft struct {
	name, addr, token string
	err               string
}

// remoteHostsGroup is Settings › Tailcat's machines: those the sidebar
// lists beside this one's.
func (a *App) remoteHostsGroup(c *ui.Context, pal *palette) {
	settingsGroup(c, pal, locales.SetRemoteHostsTitle(), locales.SetRemoteHostsDetail(), func() {
		if len(a.prefs.RemoteHosts) == 0 {
			ui.Text(c, locales.SetRemoteHostsNone()).FontSize(12.5).TextColor(pal.textFaint).Padding(2, 4)
		}
		for _, r := range a.prefs.RemoteHosts {
			r := r
			ui.Row(c).Padding(2, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, hostIcon).Size(14, 14).TextColor(pal.textMuted).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, r.Name).FontSize(13).TextColor(pal.text).SingleLine().Ellipsis("…")
					ui.Text(c, r.Addr).Font("monospace").FontSize(12).TextColor(pal.textFaint).SingleLine().Ellipsis("…")
				})
				iconButton(c, "rm-"+r.Name, closeIcon, locales.HostRemove(), pal, func() {
					a.removeRemoteHost(r.Name)
				})
			})
		}
		d := &a.ss.remote
		add := func() {
			if err := a.addRemoteHost(d.name, d.addr, d.token); err != nil {
				d.err = err.Error()
				return
			}
			*d = hostDraft{}
		}
		ui.Row(c).Padding(2, 0, 10, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			textField(c, "remote-addr", &d.addr, locales.SetRemoteHostAddress(), 170, pal).Font("monospace").OnSubmit(add)
			if in := textField(c, "remote-name", &d.name, locales.SetHostNamePlaceholder(""), 110, pal); in.Submitted() {
				add()
			}
			if in := textField(c, "remote-token", &d.token, locales.SetRemoteHostPassword(), 150, pal); in.Submitted() {
				add()
			}
			if fieldButton(c, "remote-add", locales.CommonAdd(), true, pal) {
				add()
			}
		})
		if d.err != "" {
			ui.Text(c, d.err).FontSize(12).TextColor(pal.danger)
		}
	})
}

// remoteHostHeader is another machine's name in the sidebar, above its
// projects: how the app stands with it, and what the user may do about it.
func (a *App) remoteHostHeader(c *ui.Context, pal *palette, h *host, top float32) {
	hdr := ui.Row(c.Key("remote-"+h.name)).Height(28).Margin(top, 0, 3, 0).Padding(0, 6, 0, rowInset).Gap(6).AlignItems(ui.Center).Radius(6)
	hovered := hdr.Hovered()
	if hovered {
		hdr.Background(pal.rowHover)
	}
	pointerTooltip(c, hdr, "tip-remote-"+h.name, h.name+" "+h.addr)
	hdr.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.HostReconnect()).Chosen() {
			a.reconnectHost(h)
		}
		m.Separator()
		if m.Item(locales.HostRemove()).Chosen() {
			name := h.name
			a.confirm = &confirmation{
				open:    true,
				title:   locales.HostRemoveTitle(name),
				detail:  locales.HostRemoveDetail(name),
				action:  locales.CommonRemove(),
				destroy: true,
				run:     func() { a.removeRemoteHost(name) },
			}
		}
	})
	if hdr.Clicked() {
		if s := a.sessionsOf(h); len(s) > 0 {
			a.selectSession(s[0].ID)
		}
	}
	hdr.Children(func() {
		ui.Icon(c, hostIcon).Size(13, 13).TextColor(pal.textMuted).Shrink(0)
		ui.Text(c, h.name).FontSize(13).FontWeight(600).TextColor(pal.text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
		ui.Spacer(c)
		a.hostState(c, pal, h)
		if h.conn == connReady {
			ui.Textf(c, "%d", len(a.sessionsOf(h))).FontSize(12.5).TextColor(pal.textFaint).FontFeatures("tnum")
		}
		if hovered {
			b := ui.ButtonBase(c.Key("add")).Size(20, 20).Radius(5).Center().TextColor(pal.textMuted).
				Label(locales.NewSessionIn(h.name)).Tooltip(locales.ActionNewSession())
			if b.Hovered() {
				b.Background(pal.rowHover).TextColor(pal.text)
			}
			b.Children(func() { ui.Icon(c, plusIcon).Size(13, 13) })
			if b.Clicked() {
				a.newSessionOn(h, "")
			}
		}
	})
	if h.conn != connReady && h.err != "" {
		ui.Text(c, h.err).FontSize(12).TextColor(pal.textMuted).Padding(2, 10).LineHeight(1.35)
	}
}

// reconnectHost connects to a remote host again, now.
func (a *App) reconnectHost(h *host) {
	a.dropTerms(h)
	if h.cl != nil {
		go h.cl.Close()
		h.cl = nil
	}
	h.retries, h.err = 0, ""
	h.conn = connConnecting
	a.connectHost(h)
}
