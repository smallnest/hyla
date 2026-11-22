package hosts

import (
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/settings/kit"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Remote is another machine Hyla reaches: over its tcp:// listener, by
// the address and the password of that listener, or by a tailcat pair
// code, which is the address and the secret both. The sidebar shows its
// sessions beside this machine's.
type Remote struct {
	Name         string `json:"name"`
	Addr         string `json:"addr"`            // host:port, or tailcat://tc…
	Token        string `json:"token,omitempty"` // a tcp listener's password
	Port         string `json:"port,omitempty"`  // a tailcat listener's
	Disconnected bool   `json:"disconnected,omitempty"`
}

// Tailcat reports whether the machine is reached by a pair code.
func (r Remote) Tailcat() bool { return strings.HasPrefix(r.Addr, "tailcat://") }

// BeaconTower reports whether the machine is reached through a BeaconTower
// encrypted relay pair code.
func (r Remote) BeaconTower() bool { return strings.HasPrefix(r.Addr, "beacontower://") }

// RowInset is how far a row starts from the sidebar's edge.
const RowInset = 12

// Sync follows the machines the app reaches: it connects to those added,
// and lets go of those removed or changed.
func Sync(a App) {
	keep := []*host.Host{a.PaletteLocal()}
	toConnect := []*host.Host{}
	for _, r := range a.RemoteHosts() {
		addr := strings.TrimSpace(r.Addr)
		if addr == "" || r.Name == "" {
			continue
		}
		bt := r.BeaconTower()
		tc := r.Tailcat()
		port := r.Port
		if tc && port == "" {
			port = remote.DefaultPort
		}
		code := ""
		switch {
		case bt:
			code = remote.BeaconCode(addr)
		case tc:
			code = remote.TailcatCode(addr)
		}
		h := a.HostNamed(r.Name)
		if h != nil && (h.Addr != addr || h.Token != r.Token || h.TC != tc || h.BT != bt || h.Port != port) {
			Close(a, h)
			h = nil
		}
		if h == nil {
			st := host.StateConnecting
			if r.Disconnected {
				st = host.StateDisconnected
			}
			h = &host.Host{Name: r.Name, Addr: addr, Token: r.Token, TC: tc, BT: bt, Code: code, Port: port, Conn: st}
			if !r.Disconnected {
				toConnect = append(toConnect, h)
			}
		} else {
			if r.Disconnected && h.Conn != host.StateDisconnected {
				Close(a, h)
				h.Conn, h.Retries, h.Err = host.StateDisconnected, 0, ""
			} else if !r.Disconnected && h.Conn == host.StateDisconnected {
				h.Conn, h.Retries, h.Err = host.StateConnecting, 0, ""
				toConnect = append(toConnect, h)
			}
		}
		keep = append(keep, h)
	}
	for _, h := range a.MachinesNow()[1:] {
		if !slices.Contains(keep, h) {
			Close(a, h)
		}
	}
	a.SetMachines(keep)
	a.MergeState(a.PaletteLocal(), proto.State{Projects: a.ProjectsOf(a.PaletteLocal()), Sessions: a.SessionsOf(a.PaletteLocal())})
	for _, h := range toConnect {
		if h.Conn != host.StateDisconnected {
			a.Connect(h)
		}
	}
	a.Reselect()
	a.UpdateTitle()
	a.InvalidateHosts()
}

// Close lets go of a remote host: its connection, its terminals, and its
// sessions in the sidebar.
func Close(a App, h *host.Host) {
	h.Gen++
	h.Dialing = false
	a.DropHostTerms(h)
	a.StatusesForget(h)
	if h.BT {
		remote.CloseBeaconTower(h.Code)
	}
	if cl := h.Client; cl != nil {
		h.Client = nil
		go cl.Close()
	}
	a.MergeState(h, proto.State{})
	h.Loaded = false
	h.Convs = nil
	h.ConvGen = 0
	h.ConvKnown = false
	h.ConvAsked = false
	h.ViewSent = nil
}

// Add saves a machine and connects to it: by the address and password of
// its tcp listener, or by a tailcat pair code, whose port is the
// listener's.
func Add(a App, name, addr, token, port string) error {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(addr)
	token = strings.TrimSpace(token)
	port = strings.TrimSpace(port)
	tc := false
	bt := false
	// A code a phone scans, pasted here: hyla://pair?code=tc…&name=…
	if code, label, ok := pairOf(addr); ok {
		if name == "" {
			name = label
		}
		addr = code
		tc = true
	} else if code, label, ok := beaconPairOf(addr); ok {
		if name == "" {
			name = label
		}
		addr = code
		bt = true
	} else if strings.HasPrefix(addr, "beacontower://") || remote.ValidBeaconCode(addr) {
		addr = remote.BeaconCode(addr)
		bt = true
	} else if strings.HasPrefix(addr, "tailcat://") || remote.ValidPairCode(addr) {
		addr = remote.TailcatCode(addr)
		tc = true
	}
	if bt {
		if !remote.ValidBeaconCode(addr) {
			return errNoBeaconCode
		}
		if name == "" {
			name = remote.BeaconRelayDisplay(addr)
			if i := strings.IndexByte(name, ':'); i > 0 {
				name = name[:i]
			}
			if name == "" {
				name = "beacontower"
			}
		}
		addr = "beacontower://" + addr
		token = ""
		port = ""
	} else if tc {
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
	for _, r := range a.RemoteHosts() {
		if strings.EqualFold(r.Name, name) {
			return errHostNamed(name)
		}
	}
	rs := append(slices.Clone(a.RemoteHosts()), Remote{Name: name, Addr: addr, Token: token, Port: port})
	a.SetRemoteHosts(rs)
	a.SavePrefs()
	Sync(a)
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

func beaconPairOf(s string) (code, name string, ok bool) {
	if !strings.Contains(s, "://") {
		return "", "", false
	}
	u, err := url.Parse(s)
	if err != nil || !strings.EqualFold(u.Scheme, "hyla") ||
		(!strings.EqualFold(u.Host, "beacon") && !strings.EqualFold(u.Host, "pair")) {
		return "", "", false
	}
	code = remote.BeaconCode(u.Query().Get("code"))
	if !remote.ValidBeaconCode(code) {
		return "", "", false
	}
	return code, strings.TrimSpace(u.Query().Get("name")), true
}

// Remove forgets a machine, and its sessions.
func Remove(a App, name string) {
	a.SetRemoteHosts(slices.DeleteFunc(slices.Clone(a.RemoteHosts()), func(r Remote) bool {
		return strings.EqualFold(r.Name, name)
	}))
	a.SavePrefs()
	Sync(a)
}

// hostErrors are why a machine cannot be added as it is given.
var (
	errNoHostToken   = errString(locales.SetHostNeedsToken())
	errNoHostAddress = errString(locales.SetHostBadAddress())
	errHostNamed     = func(name string) error { return errString(locales.SetHostNameTaken(name)) }
	errNoHostCode    = errString(locales.SetHostNeedsCode())
	errNoBeaconCode  = errString(locales.SetHostNeedsBeaconCode())
)

type errString string

func (e errString) Error() string { return string(e) }

// P2PGroup is Settings › Remote Connection › P2P: the machines this app
// reaches by an address and a password, and this machine's own listener,
// which a client of them reaches.
func P2PGroup(a App, c *ui.Context, pal *theme.Palette) {
	d := a.P2PDraft()
	kit.Group(c, pal, locales.SetTCPTitle(), "", func() {
		Remotes(a, c, pal)
		if kit.Rows++; kit.Rows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
		}
		a.TCPListen(c, pal, d)
	})
}

// PairCodeHosts are the machines reached by a pair code: the head of the
// Tailcat group.
func PairCodeHosts(a App, c *ui.Context, pal *theme.Palette) {
	d := a.PairCodeDraft()
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Text(c, locales.SetTailcatHostsTitle()).FontSize(13.5).TextColor(pal.Text)
		for _, r := range a.RemoteHosts() {
			if !r.Tailcat() {
				continue
			}
			summary := strings.TrimSpace(shortPairCode(r.Addr) + " " + r.Port)
			remoteHostRow(a, c, pal, "tc-", r, summary)
		}
		add := func() {
			port := d.Port
			if strings.TrimSpace(port) == "" {
				port = remote.DefaultPort
			}
			if err := Add(a, d.Name, d.Code, "", port); err != nil {
				d.Err = err.Error()
				return
			}
			*d = draft.TCHostDraft{}
		}
		ui.Row(c).Padding(2, 0, 10, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			if in := controls.TextField(c, "tc-code", &d.Code, locales.SetTailcatHostCode(), 190, pal).Font("monospace"); in.Submitted() {
				add()
			}
			if in := controls.TextField(c, "tc-name", &d.Name, locales.SetTailcatHostName(), 100, pal); in.Submitted() {
				add()
			}
			if in := controls.TextField(c, "tc-port", &d.Port, remote.DefaultPort, 70, pal); in.Submitted() {
				add()
			}
			if style.FieldButton(c, "tc-add", locales.CommonAdd(), true, pal) {
				add()
			}
		})
		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
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

// Remotes is the machines the sidebar lists beside this one's: the head
// of the P2P group.
func Remotes(a App, c *ui.Context, pal *theme.Palette) {
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, locales.SetRemoteHostsTitle()).FontSize(13.5).TextColor(pal.Text)
			if d := locales.SetRemoteHostsDetail(); d != "" {
				ui.Text(c, d).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
			}
		})
		if !slices.ContainsFunc(a.RemoteHosts(), func(r Remote) bool { return !r.Tailcat() && !r.BeaconTower() }) {
			ui.Text(c, locales.SetRemoteHostsNone()).FontSize(12.5).TextColor(pal.TextFaint).Padding(2, 4)
		}
		for _, r := range a.RemoteHosts() {
			if r.Tailcat() || r.BeaconTower() {
				continue
			}
			remoteHostRow(a, c, pal, "p2p-", r, r.Addr)
		}
		d := a.HostDraft()
		add := func() {
			if err := Add(a, d.Name, d.Addr, d.Token, ""); err != nil {
				d.Err = err.Error()
				return
			}
			*d = draft.HostDraft{}
		}
		ui.Row(c).Padding(2, 0, 10, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			controls.TextField(c, "remote-addr", &d.Addr, locales.SetRemoteHostAddress(), 170, pal).Font("monospace").OnSubmit(add)
			if in := controls.TextField(c, "remote-name", &d.Name, locales.SetHostNamePlaceholder(""), 110, pal); in.Submitted() {
				add()
			}
			if in := controls.TextField(c, "remote-token", &d.Token, locales.SetRemoteHostPassword(), 150, pal); in.Submitted() {
				add()
			}
			if style.FieldButton(c, "remote-add", locales.CommonAdd(), true, pal) {
				add()
			}
		})
		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}
	})
}

func remoteStatusStyle(pal *theme.Palette, st host.State) (dot, text, bg ui.Color, label string) {
	switch st {
	case host.StateReady:
		return pal.Online, pal.Online, pal.Online.Alpha(0.14), locales.HostStatusConnected()
	case host.StateConnecting:
		return pal.Attention, pal.Attention, pal.Attention.Alpha(0.14), locales.StatusConnecting()
	case host.StateFailed:
		return pal.Danger, pal.Danger, pal.Danger.Alpha(0.14), locales.StatusFailed()
	case host.StateVersion:
		return pal.Attention, pal.Attention, pal.Attention.Alpha(0.14), locales.HostStatusVersion()
	default:
		return pal.TextFaint, pal.TextMuted, pal.Text.Alpha(0.08), locales.StatusDisconnected()
	}
}

func remoteHostRow(a App, c *ui.Context, pal *theme.Palette, prefix string, r Remote, subtitle string) {
	st := host.StateDisconnected
	errMsg := ""
	if h := a.HostNamed(r.Name); h != nil {
		st = h.Conn
		errMsg = h.Err
	}
	dotCol, textCol, bgCol, statusLabel := remoteStatusStyle(pal, st)
	nameCol := pal.Text
	if st == host.StateDisconnected {
		nameCol = pal.TextMuted
	}
	ui.Column(c.Key(prefix+"host-"+r.Name)).Padding(4, 0).Gap(4).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, glyph.Host).Size(14, 14).TextColor(pal.TextMuted).Shrink(0)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
					ui.Text(c, r.Name).FontSize(13).TextColor(nameCol).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
					ui.Row(c.Key("status")).Height(19).Padding(0, 7).Gap(5).Radius(9.5).AlignItems(ui.Center).Shrink(0).Background(bgCol).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(dotCol).Shrink(0)
						ui.Text(c, statusLabel).FontSize(11.5).FontWeight(500).TextColor(textCol).SingleLine()
					})
				})
				ui.Text(c, subtitle).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
			})
			ui.Row(c).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
				if st == host.StateReady || st == host.StateConnecting {
					if style.FieldButton(c, prefix+"disconnect-"+r.Name, locales.HostDisconnect(), false, pal) {
						DisconnectName(a, r.Name)
					}
				} else {
					if style.FieldButton(c, prefix+"connect-"+r.Name, locales.CommonConnect(), false, pal) {
						ConnectName(a, r.Name)
					}
				}
				paints.IconButton(c, prefix+"rm-"+r.Name, glyph.Close, locales.HostRemove(), pal, func() {
					Remove(a, r.Name)
				})
			})
		})
		switch {
		case st == host.StateFailed && errMsg != "":
			ui.Text(c, errMsg).FontSize(12).TextColor(pal.Danger).LineHeight(1.35).Wrap().Padding(0, 0, 0, 24)
		case st == host.StateVersion:
			ui.Text(c, locales.HostVersionMismatchDetail()).FontSize(12).TextColor(pal.Attention).LineHeight(1.35).Wrap().Padding(0, 0, 0, 24)
		}
	})
}

func hostDisplayAddr(h *host.Host) string {
	switch {
	case h.BT:
		if hostLabel := remote.BeaconRelayDisplay(h.Addr); hostLabel != "" {
			return hostLabel + " · " + shortBeaconCode(h.Addr)
		}
		return shortBeaconCode(h.Addr)
	case h.TC:
		if h.Port != "" {
			return shortPairCode(h.Addr) + " " + h.Port
		}
		return shortPairCode(h.Addr)
	default:
		return h.Addr
	}
}

// HostHeader is another machine's name in the sidebar, above its
// projects: how the app stands with it, and what the user may do about
// it.
func HostHeader(a App, c *ui.Context, pal *theme.Palette, h *host.Host, top float32) {
	hdr := ui.Row(c.Key("remote-"+h.Name)).Height(28).Margin(top, 0, 3, 0).Padding(0, 6, 0, RowInset).Gap(6).AlignItems(ui.Center).Radius(6)
	hovered := hdr.Hovered()
	if hovered {
		hdr.Background(pal.RowHover)
	}
	_, statusCol, _, statusText := remoteStatusStyle(pal, h.Conn)
	tip := h.Name + " · " + statusText
	if addr := hostDisplayAddr(h); addr != "" {
		tip += "\n" + addr
	}
	if h.Conn == host.StateFailed && h.Err != "" {
		tip += "\n" + h.Err
	}
	paints.PointerTooltip(c, hdr, "tip-remote-"+h.Name, tip)
	hdr.ContextMenu(func(m *ui.Menu) {
		switch h.Conn {
		case host.StateReady, host.StateConnecting:
			if m.Item(locales.HostReconnect()).Chosen() {
				Reconnect(a, h)
			}
			if m.Item(locales.HostDisconnect()).Chosen() {
				Disconnect(a, h)
			}
		case host.StateDisconnected:
			if m.Item(locales.CommonConnect()).Chosen() {
				Connect(a, h)
			}
		default:
			if m.Item(locales.CommonConnect()).Chosen() {
				Connect(a, h)
			}
			if m.Item(locales.HostDisconnect()).Chosen() {
				Disconnect(a, h)
			}
		}
		m.Separator()
		if m.Item(locales.HostRemove()).Chosen() {
			name := h.Name
			a.SetConfirm(&confirm.Ask{
				Open:    true,
				Title:   locales.HostRemoveTitle(name),
				Detail:  locales.HostRemoveDetail(name),
				Action:  locales.CommonRemove(),
				Destroy: true,
				Run:     func() { Remove(a, name) },
			})
		}
	})
	if hdr.Clicked() {
		if s := a.SessionsOf(h); len(s) > 0 {
			a.PaletteSelectSession(s[0].ID)
		}
	}
	hdr.Children(func() {
		ui.Icon(c, glyph.Host).Size(13, 13).TextColor(pal.TextMuted).Shrink(0)
		nameCol := pal.Text
		if h.Conn == host.StateDisconnected {
			nameCol = pal.TextMuted
		}
		ui.Text(c, h.Name).FontSize(13).FontWeight(600).TextColor(nameCol).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
		ui.Spacer(c)
		a.HostState(c, pal, h)
		switch h.Conn {
		case host.StateReady:
			ui.Textf(c, "%d", len(a.SessionsOf(h))).FontSize(12.5).TextColor(pal.TextFaint).FontFeatures("tnum")
			if hovered {
				b := ui.ButtonBase(c.Key("add")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).
					Label(locales.NewSessionIn(h.Name)).Tooltip(locales.ActionNewSession())
				if b.Hovered() {
					b.Background(pal.RowHover).TextColor(pal.Text)
				}
				b.Children(func() { ui.Icon(c, glyph.Plus).Size(13, 13) })
				if b.Clicked() {
					a.NewSessionOn(h, "")
				}
				db := ui.ButtonBase(c.Key("disconnect")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).
					Label(locales.HostDisconnect()).Tooltip(locales.HostDisconnect())
				if db.Hovered() {
					db.Background(pal.RowHover).TextColor(pal.Text)
				}
				db.Children(func() { ui.Icon(c, glyph.Disconnect).Size(12, 12) })
				if db.Clicked() {
					Disconnect(a, h)
				}
			}
		case host.StateConnecting:
			if !hovered {
				ui.Text(c, statusText).FontSize(11.5).TextColor(pal.TextMuted).SingleLine().Shrink(0)
			} else {
				db := ui.ButtonBase(c.Key("disconnect")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).
					Label(locales.HostDisconnect()).Tooltip(locales.HostDisconnect())
				if db.Hovered() {
					db.Background(pal.RowHover).TextColor(pal.Text)
				}
				db.Children(func() { ui.Icon(c, glyph.Disconnect).Size(12, 12) })
				if db.Clicked() {
					Disconnect(a, h)
				}
			}
		default:
			if !hovered {
				ui.Text(c, statusText).FontSize(11.5).TextColor(statusCol).SingleLine().Shrink(0)
			} else {
				cb := ui.ButtonBase(c.Key("connect")).Size(20, 20).Radius(5).Center().TextColor(pal.TextMuted).
					Label(locales.CommonConnect()).Tooltip(locales.CommonConnect())
				if cb.Hovered() {
					cb.Background(pal.RowHover).TextColor(pal.Text)
				}
				cb.Children(func() { ui.Icon(c, glyph.Connect).Size(12, 12) })
				if cb.Clicked() {
					Connect(a, h)
				}
			}
		}
	})
	if h.Conn != host.StateReady && h.Conn != host.StateDisconnected && h.Err != "" {
		ui.Text(c, h.Err).FontSize(12).TextColor(pal.TextMuted).Padding(2, 10).LineHeight(1.35)
	}
}

func setRemoteDisconnected(a App, name string, disconnected bool) {
	rs := slices.Clone(a.RemoteHosts())
	changed := false
	for i := range rs {
		if strings.EqualFold(rs[i].Name, name) && rs[i].Disconnected != disconnected {
			rs[i].Disconnected = disconnected
			changed = true
		}
	}
	if !changed {
		return
	}
	a.SetRemoteHosts(rs)
	a.SavePrefs()
}

// Disconnect disconnects a remote host and stops automatic reconnects until
// Connect or Reconnect is invoked.
func Disconnect(a App, h *host.Host) {
	if h == nil || !h.Remote() {
		return
	}
	setRemoteDisconnected(a, h.Name, true)
	Close(a, h)
	h.Conn, h.Retries, h.Err = host.StateDisconnected, 0, ""
	a.Reselect()
	a.UpdateTitle()
	a.InvalidateHosts()
}

// DisconnectName disconnects the named remote host, persisting its
// disconnected preference even if it is not currently in MachinesNow.
func DisconnectName(a App, name string) {
	setRemoteDisconnected(a, name, true)
	if h := a.HostNamed(name); h != nil {
		Disconnect(a, h)
		return
	}
	a.InvalidateHosts()
}

// Connect connects a remote host now and clears its disconnected preference.
func Connect(a App, h *host.Host) {
	if h == nil || !h.Remote() {
		return
	}
	Reconnect(a, h)
}

// ConnectName clears the disconnected preference of the named remote host and
// connects it now.
func ConnectName(a App, name string) {
	setRemoteDisconnected(a, name, false)
	if h := a.HostNamed(name); h != nil {
		Reconnect(a, h)
		return
	}
	Sync(a)
}

// Reconnect connects to a remote host again, now.
func Reconnect(a App, h *host.Host) {
	if h == nil || !h.Remote() {
		return
	}
	setRemoteDisconnected(a, h.Name, false)
	a.DropHostTerms(h)
	a.StatusesForget(h)
	if h.BT {
		remote.CloseBeaconTower(h.Code)
	}
	if cl := h.Client; cl != nil {
		h.Client = nil
		go cl.Close()
	}
	h.Retries, h.Err = 0, ""
	h.Conn = host.StateConnecting
	a.Connect(h)
	a.InvalidateHosts()
}
