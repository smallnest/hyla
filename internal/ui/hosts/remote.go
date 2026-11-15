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
	Name  string `json:"name"`
	Addr  string `json:"addr"`            // host:port, or tailcat://tc…
	Token string `json:"token,omitempty"` // a tcp listener's password
	Port  string `json:"port,omitempty"`  // a tailcat listener's
}

// Tailcat reports whether the machine is reached by a pair code.
func (r Remote) Tailcat() bool { return strings.HasPrefix(r.Addr, "tailcat://") }

// RowInset is how far a row starts from the sidebar's edge.
const RowInset = 12

// Sync follows the machines the app reaches: it connects to those added,
// and lets go of those removed or changed.
func Sync(a App) {
	keep := []*host.Host{a.PaletteLocal()}
	for _, r := range a.RemoteHosts() {
		addr := strings.TrimSpace(r.Addr)
		if addr == "" || r.Name == "" {
			continue
		}
		tc := r.Tailcat()
		port := r.Port
		if tc && port == "" {
			port = remote.DefaultPort
		}
		code := remote.TailcatCode(addr)
		h := a.HostNamed(r.Name)
		if h != nil && (h.Addr != addr || h.Token != r.Token || h.TC != tc || h.Port != port) {
			Close(a, h)
			h = nil
		}
		if h == nil {
			h = &host.Host{Name: r.Name, Addr: addr, Token: r.Token, TC: tc, Code: code, Port: port, Conn: host.StateConnecting}
		}
		keep = append(keep, h)
	}
	added := []*host.Host{}
	for _, h := range keep[1:] {
		if !slices.Contains(a.MachinesNow(), h) {
			added = append(added, h)
		}
	}
	for _, h := range a.MachinesNow()[1:] {
		if !slices.Contains(keep, h) {
			Close(a, h)
		}
	}
	a.SetMachines(keep)
	a.MergeState(a.PaletteLocal(), proto.State{Projects: a.ProjectsOf(a.PaletteLocal()), Sessions: a.SessionsOf(a.PaletteLocal())})
	for _, h := range added {
		a.Connect(h)
	}
	a.Reselect()
	a.UpdateTitle()
}

// Close lets go of a remote host: its connection, its terminals, and its
// sessions in the sidebar.
func Close(a App, h *host.Host) {
	h.Gen++
	a.DropHostTerms(h)
	if h.Client != nil {
		go h.Client.Close()
		h.Client = nil
	}
	a.MergeState(h, proto.State{})
	h.Loaded = false
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
		n := 0
		for _, r := range a.RemoteHosts() {
			if !r.Tailcat() {
				continue
			}
			n++
			r := r
			ui.Row(c).Padding(2, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, glyph.Host).Size(14, 14).TextColor(pal.TextMuted).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, r.Name).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…")
					ui.Text(c, strings.TrimPrefix(r.Addr, "tailcat://")[:min(12, len(strings.TrimPrefix(r.Addr, "tailcat://")))]+"… "+r.Port).
						Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
				})
				paints.IconButton(c, "rm-tc-"+r.Name, glyph.Close, locales.HostRemove(), pal, func() {
					Remove(a, r.Name)
				})
			})
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
		if len(a.RemoteHosts()) == 0 {
			ui.Text(c, locales.SetRemoteHostsNone()).FontSize(12.5).TextColor(pal.TextFaint).Padding(2, 4)
		}
		for _, r := range a.RemoteHosts() {
			if r.Tailcat() {
				continue
			}
			r := r
			ui.Row(c).Padding(2, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, glyph.Host).Size(14, 14).TextColor(pal.TextMuted).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, r.Name).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…")
					ui.Text(c, r.Addr).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
				})
				paints.IconButton(c, "rm-"+r.Name, glyph.Close, locales.HostRemove(), pal, func() {
					Remove(a, r.Name)
				})
			})
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

// HostHeader is another machine's name in the sidebar, above its
// projects: how the app stands with it, and what the user may do about
// it.
func HostHeader(a App, c *ui.Context, pal *theme.Palette, h *host.Host, top float32) {
	hdr := ui.Row(c.Key("remote-"+h.Name)).Height(28).Margin(top, 0, 3, 0).Padding(0, 6, 0, RowInset).Gap(6).AlignItems(ui.Center).Radius(6)
	hovered := hdr.Hovered()
	if hovered {
		hdr.Background(pal.RowHover)
	}
	paints.PointerTooltip(c, hdr, "tip-remote-"+h.Name, h.Name+" "+h.Addr)
	hdr.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.HostReconnect()).Chosen() {
			Reconnect(a, h)
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
		ui.Text(c, h.Name).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
		ui.Spacer(c)
		a.HostState(c, pal, h)
		if h.Conn == host.StateReady {
			ui.Textf(c, "%d", len(a.SessionsOf(h))).FontSize(12.5).TextColor(pal.TextFaint).FontFeatures("tnum")
		}
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
		}
	})
	if h.Conn != host.StateReady && h.Err != "" {
		ui.Text(c, h.Err).FontSize(12).TextColor(pal.TextMuted).Padding(2, 10).LineHeight(1.35)
	}
}

// Reconnect connects to a remote host again, now.
func Reconnect(a App, h *host.Host) {
	a.DropHostTerms(h)
	if h.Client != nil {
		go h.Client.Close()
		h.Client = nil
	}
	h.Retries, h.Err = 0, ""
	h.Conn = host.StateConnecting
	a.Connect(h)
}
