package main

import (
	"context"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"

	"hyla/locales"
)

// remoteHost is another machine Hyla reaches: over its tcp:// listener, by
// the address and the password of that listener, or by a tailcat pair
// code, which is the address and the secret both. The sidebar shows its
// sessions beside this machine's.
type remoteHost struct {
	Name  string `json:"name"`
	Addr  string `json:"addr"`            // host:port, or tailcat://tc…
	Token string `json:"token,omitempty"` // a tcp listener's password
	Port  string `json:"port,omitempty"`  // a tailcat listener's
}

// tailcat reports whether the machine is reached by a pair code.
func (r remoteHost) tailcat() bool { return strings.HasPrefix(r.Addr, "tailcat://") }

// tcpDialer opens connections to a machine's tcp:// listener: the
// password first, which the listener admits, then Hyla's own protocol.
type tcpDialer struct {
	addr, token string
}

// tailcatDialer reaches a machine by its pair code, through the relay the
// code names: the port is its listener's, which carries Hyla's protocol.
type tailcatDialer struct{ code, port string }

func (d tailcatDialer) Dial() (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return remote.OpenTailcat(ctx, d.code, d.port)
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
		// Tailcat off reaches none of its machines: the app lists none.
		if addr == "" || r.Name == "" || r.tailcat() && !a.set.Tailcat {
			continue
		}
		tc := r.tailcat()
		port := r.Port
		if tc && port == "" {
			port = remote.DefaultPort
		}
		code := remote.TailcatCode(addr)
		h := a.hostNamed(r.Name)
		if h != nil && (h.addr != addr || h.token != r.Token || h.tc != tc || h.port != port) {
			a.closeHost(h)
			h = nil
		}
		if h == nil {
			h = &host{name: r.Name, addr: addr, token: r.Token, tc: tc, code: code, port: port, conn: connConnecting}
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

// addRemoteHost saves a machine and connects to it: by the address and
// password of its tcp listener, or by a tailcat pair code, whose port is
// the listener's.
func (a *App) addRemoteHost(name, addr, token, port string) error {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(addr)
	token = strings.TrimSpace(token)
	port = strings.TrimSpace(port)
	tc := false
	// A code a phone scans, pasted here: hyla://pair?code=tc…&name=…
	if code, label, ok := pairOf(addr); ok {
		if name == "" {
			name = label
		}
		addr = code
		tc = true
	} else if strings.HasPrefix(addr, "tailcat://") || remote.ValidPairCode(addr) {
		addr = remote.TailcatCode(addr)
		tc = true
	}
	if tc {
		if !remote.ValidPairCode(addr) {
			return errNoHostCode
		}
		if port == "" {
			port = remote.DefaultPort
		}
		if !settings.ValidListenPort(port) {
			return errNoHostAddress
		}
		addr = "tailcat://" + addr
	} else {
		addr = strings.TrimPrefix(addr, "tcp://")
		// A code a phone scans, pasted here: tcp://host:port?token=…, whose
		// token is the password.
		if i := strings.Index(addr, "?"); i >= 0 {
			if q, err := url.ParseQuery(addr[i+1:]); err == nil && token == "" {
				token = strings.TrimSpace(q.Get("token"))
			}
			addr = addr[:i]
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return errNoHostAddress
		}
		if name == "" {
			name = host
		}
		if token == "" {
			return errNoHostToken
		}
	}
	for _, r := range a.prefs.RemoteHosts {
		if strings.EqualFold(r.Name, name) {
			return errHostNamed(name)
		}
	}
	a.prefs.RemoteHosts = append(a.prefs.RemoteHosts, remoteHost{Name: name, Addr: addr, Token: token, Port: port})
	a.savePrefs()
	a.syncRemoteHosts()
	return nil
}

// pairOf reads a code a phone scans, hyla://pair?code=tc…&name=…, as its
// pair code and name.
func pairOf(s string) (code, name string, ok bool) {
	if !strings.HasPrefix(s, "hyla://pair?") {
		return "", "", false
	}
	q, err := url.ParseQuery(strings.TrimPrefix(s, "hyla://pair?"))
	if err != nil {
		return "", "", false
	}
	code = q.Get("code")
	if !remote.ValidPairCode(code) {
		return "", "", false
	}
	return code, strings.TrimSpace(q.Get("name")), true
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
	errNoHostCode    = errString(locales.SetHostNeedsCode())
)

type errString string

func (e errString) Error() string { return string(e) }

// p2pGroup is Settings › Remote Connection › P2P: the machines this app
// reaches by an address and a password, and this machine's own listener,
// which a client of them reaches.
func (a *App) p2pGroup(c *ui.Context, pal *palette) {
	d := &a.ss.tailcat
	settingsGroup(c, pal, locales.SetTCPTitle(), "", func() {
		a.remoteHosts(c, pal)
		if groupRows++; groupRows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.text.Alpha(0.07))
		}
		a.tcpListen(c, pal, d)
	})
}

// tcHostDraft is the form that adds a machine by its tailcat pair code.
type tcHostDraft struct {
	name, code, port string
	err              string
}

// tailcatHosts are the machines reached by a pair code: the head of the
// Tailcat group.
func (a *App) tailcatHosts(c *ui.Context, pal *palette) {
	d := &a.ss.tcHost
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Text(c, locales.SetTailcatHostsTitle()).FontSize(13.5).TextColor(pal.text)
		n := 0
		for _, r := range a.prefs.RemoteHosts {
			if !r.tailcat() {
				continue
			}
			n++
			r := r
			ui.Row(c).Padding(2, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, hostIcon).Size(14, 14).TextColor(pal.textMuted).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, r.Name).FontSize(13).TextColor(pal.text).SingleLine().Ellipsis("…")
					ui.Text(c, strings.TrimPrefix(r.Addr, "tailcat://")[:min(12, len(strings.TrimPrefix(r.Addr, "tailcat://")))]+"… "+r.Port).
						Font("monospace").FontSize(12).TextColor(pal.textFaint).SingleLine().Ellipsis("…")
				})
				iconButton(c, "rm-tc-"+r.Name, closeIcon, locales.HostRemove(), pal, func() {
					a.removeRemoteHost(r.Name)
				})
			})
		}
		add := func() {
			port := d.port
			if strings.TrimSpace(port) == "" {
				port = remote.DefaultPort
			}
			if err := a.addRemoteHost(d.name, d.code, "", port); err != nil {
				d.err = err.Error()
				return
			}
			*d = tcHostDraft{}
		}
		ui.Row(c).Padding(2, 0, 10, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			if in := textField(c, "tc-code", &d.code, locales.SetTailcatHostCode(), 190, pal).Font("monospace"); in.Submitted() {
				add()
			}
			if in := textField(c, "tc-name", &d.name, locales.SetTailcatHostName(), 100, pal); in.Submitted() {
				add()
			}
			if in := textField(c, "tc-port", &d.port, remote.DefaultPort, 70, pal); in.Submitted() {
				add()
			}
			if fieldButton(c, "tc-add", locales.CommonAdd(), true, pal) {
				add()
			}
		})
		if d.err != "" {
			ui.Text(c, d.err).FontSize(12).TextColor(pal.danger)
		}
	})
}

// shortPairCode is a pair code as a list shows it: its first characters.
func shortPairCode(addr string) string {
	code := strings.TrimPrefix(addr, "tailcat://")
	if len(code) > 12 {
		return code[:12] + "…"
	}
	return code
}

// hostDraft is the form that adds another machine: its name, its
// listener's address and its password, and what went wrong.
type hostDraft struct {
	name, addr, token string
	err               string
}

// remoteHosts is the machines the sidebar lists beside this one's: the
// head of the P2P group.
func (a *App) remoteHosts(c *ui.Context, pal *palette) {
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, locales.SetRemoteHostsTitle()).FontSize(13.5).TextColor(pal.text)
			if d := locales.SetRemoteHostsDetail(); d != "" {
				ui.Text(c, d).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
			}
		})
		if len(a.prefs.RemoteHosts) == 0 {
			ui.Text(c, locales.SetRemoteHostsNone()).FontSize(12.5).TextColor(pal.textFaint).Padding(2, 4)
		}
		for _, r := range a.prefs.RemoteHosts {
			if r.tailcat() {
				continue
			}
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
			if err := a.addRemoteHost(d.name, d.addr, d.token, ""); err != nil {
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
