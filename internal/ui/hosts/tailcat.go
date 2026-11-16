package hosts

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/xingkaixin/mygo-native-ui/ui/qrcode"

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

// Tailcat is this machine as a host for HylaPocket: a relay with a pair
// code (internal/remote/tailcat.go). The tailcat switch in Settings ›
// Hosts turns it on and off: off, hylad starts no listener of it, and the
// phone reaches nothing. The tcp:// listener, which a client of Hyla or
// HylaPocket reaches by an address and a password, is the P2P group's:
// each switch acts on its own listener, never the other's.

// TailcatGroup is Settings › Hosts › Tailcat: the switch, and with it on,
// the hosts Hyla adds and this machine as a host.
func TailcatGroup(a App, c *ui.Context, pal *theme.Palette) {
	kit.Group(c, pal, locales.SetTailcatTitle(), "", func() {
		d := a.TailcatDraft()
		if d.ListenPort == "" {
			ResetTailcat(a)
		}
		if kit.Rows++; kit.Rows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
		}
		// The relay both ways: the machines added are found through it, and
		// so is this machine's listener.
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, locales.SetTailcatRelay()).FontSize(13.5).TextColor(pal.Text)
			if in := controls.TextField(c, "tailcat-relay", &d.Relay, remote.DefaultDERPMapURL, 0, pal); in.Submitted() {
				saveTailcat(a, d, "tailcat-relay")
			}
		})
		tailcatFieldErr(c, pal, d.Err, "tailcat-relay")
		if kit.Rows++; kit.Rows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
		}
		PairCodeHosts(a, c, pal)
		if kit.Rows++; kit.Rows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
		}
		TailcatListen(a, c, pal, d)
	})
}

// ResetTailcat reads the Tailcat form's fields again from the settings,
// and this machine's pair code with them. The P2P form is ResetP2P's.
func ResetTailcat(a App) {
	orig := a.SettingsNow()
	s := *orig
	d := a.TailcatDraft()
	*d = draft.TailcatDraft{
		Relay:      s.TailcatRelay,
		ListenPort: settings.TailcatListenPortDefault(),
	}
	if _, port, ok := s.TailcatListen(); ok {
		d.ListenPort = port
	}
	if s.Tailcat {
		TailcatCode(a)
	}
}

// tailcatSettingRow is settingRow, which is no row of its group's, here.
func tailcatSettingRow(c *ui.Context, pal *theme.Palette, label, detail string, control func()) {
	n := kit.Rows
	kit.Row(c, pal, label, detail, control)
	kit.Rows = n
}

// TailcatListen is this machine as a host over Tailcat: the switch of
// hylad's tailcat listener here, its port, and the pair code Hyla on
// other machines adds with.
func TailcatListen(a App, c *ui.Context, pal *theme.Palette, d *draft.TailcatDraft) {
	on := hasTailcatListener(a.SettingsNow())
	detail := locales.SetTailcatListenDetail()
	if on {
		detail = locales.SetTailcatOn()
	}
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTailcatListen()).FontSize(13.5).TextColor(pal.Text)
				if detail != "" {
					ui.Text(c, detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
				style.Toggle(c.Key("tailcat-listen"), &on, pal).OnClick(func() {
					s := a.SettingsNow().Clone()
					if on {
						port := strings.TrimSpace(d.ListenPort)
						if !settings.ValidListenPort(port) {
							port = settings.TailcatListenPortDefault()
						}
						s.SetTailcatListen(port)
					} else {
						s.SetTailcatListen("")
					}
					a.SetSettingsNow(s)
					ResetTailcat(a)
				})
			})
		})
		if !on {
			return
		}
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTailcatListenPort()).FontSize(13.5).TextColor(pal.Text)
				if d := locales.SetTailcatListenPortDetail(); d != "" {
					ui.Text(c, d).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
				}
			})
			if in := controls.TextField(c, "tailcat-listen-port", &d.ListenPort, settings.TailcatListenPortDefault(), 70, pal); in.Submitted() {
				saveTailcat(a, d, "tailcat-listen-port")
			}
		})
		tailcatFieldErr(c, pal, d.Err, "tailcat-listen-port")
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.SetTailcatCode()).FontSize(13.5).TextColor(pal.Text)
			if d := locales.SetTailcatCodeDetail(); d != "" {
				ui.Text(c, d).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
			}
		})
		// What other machines add with the code, as a phone scans it:
		// nothing where there is none.
		if d.QR != nil {
			qrcode.View(c, d.QR).Size(160, 160).Shrink(0)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			code, col := d.Code, pal.Text
			switch {
			case d.CodeErr != "":
				code, col = d.CodeErr, pal.Danger
			case code == "":
				code, col = locales.SetTailcatStarting(), pal.TextMuted
			}
			ui.Text(c, code).Font("monospace").FontSize(12.5).TextColor(col).LineHeight(1.35).Selectable().MinWidth(0)
			ui.Row(c).Grow(1)
			if code == "" || d.CodeErr != "" {
				return
			}
			icon, label := glyph.Copy, locales.HostCopyAddress()
			if time.Since(d.Copied) < 1500*time.Millisecond {
				icon, label = glyph.Of("lucide-check"), locales.ToastCopied()
			}
			paints.IconButton(c, "copy-tailcat", icon, label, pal, func() {
				c.WriteClipboard(code)
				d.Copied = time.Now()
				go func() {
					time.Sleep(1500 * time.Millisecond)
					a.Update(func() {})
				}()
			})
			paints.IconButton(c, "change-tailcat", glyph.Reload, locales.SetTailcatChange(), pal, func() {
				a.SettingsConfirm(&confirm.Ask{
					Open:    true,
					Title:   locales.SetTailcatChange(),
					Action:  locales.SetTailcatChange(),
					Destroy: true,
					Run:     func() { RotateTailcatCode(a) },
				})
			})
		})
	})
}

// tailcatFieldErr is the reason a field of a form failed, under it.
func tailcatFieldErr(c *ui.Context, pal *theme.Palette, f draft.TailcatField, key string) {
	if f.Key == key {
		ui.Text(c, f.Err).FontSize(12).TextColor(pal.Danger)
	}
}

// TailcatCode reads this machine's pair code, once.
func TailcatCode(a App) {
	d := a.TailcatDraft()
	if d.Looking || !d.At.IsZero() {
		return
	}
	d.Looking = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		code, err := remote.LocalTailcatAddress(ctx)
		cancel()
		// The code drawn as a phone's camera reads it: its scheme, which
		// the code alone does not carry.
		var qr *qrcode.Code
		if err == nil {
			qr, _ = qrcode.Encode("tailcat://"+code, qrcode.L)
		}
		a.Update(func() {
			d.Code, d.CodeErr, d.QR, d.Looking, d.At = code, errorText(err), qr, false, time.Now()
		})
	}()
}

// RotateTailcatCode gives this machine a new pair code, which the old
// one's no longer opens.
func RotateTailcatCode(a App) {
	if _, err := remote.RotateTailcatKey(remote.TailcatKeyPath()); err != nil {
		a.SettingsToast(err.Error())
		return
	}
	d := a.TailcatDraft()
	*d = draft.TailcatDraft{Relay: d.Relay, ListenPort: d.ListenPort}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		k, err := remote.TailcatKeyAt(ctx, remote.TailcatKeyPath())
		var code string
		if err == nil {
			code, err = remote.TailcatAddress(ctx, k)
		}
		cancel()
		a.Update(func() {
			d.Code, d.CodeErr, d.At = code, errorText(err), time.Now()
			if err == nil {
				// The code a phone scans carries the scheme the code
				// alone does not.
				d.QR, _ = qrcode.Encode("tailcat://"+code, qrcode.L)
			}
		})
	}()
	a.SettingsToast(locales.SetTailcatChanged())
	// hylad starts its listener with the key again, which follow finds.
	s := a.SettingsNow().Clone()
	s.TailcatRotate = !s.TailcatRotate
	a.SetSettingsNow(s)
}

// hasTailcatListener reports whether s makes this machine a tailcat host.
func hasTailcatListener(s *settings.Settings) bool {
	_, _, ok := s.TailcatListen()
	return ok
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TCPListen is this machine on its own network: a tcp:// listener, whose
// address and password a client of Hyla, or of HylaPocket, takes.
func TCPListen(a App, c *ui.Context, pal *theme.Palette, d *draft.P2PDraft) {
	on := hasTCPListener(a.SettingsNow())
	detail := locales.SetTCPListenDetail()
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTCP()).FontSize(13.5).TextColor(pal.Text)
				if detail != "" {
					ui.Text(c, detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
				style.Toggle(c.Key("tcp-listen"), &on, pal).OnClick(func() {
					s := a.SettingsNow().Clone()
					if on {
						addr := strings.TrimSpace(d.Addr)
						if addr == "" {
							addr = ":" + settings.TailcatListenPortDefault()
						}
						if spec, err := remote.ParseListen("tcp://" + addr); err == nil {
							s.SetTCPListen(spec.Host, spec.Port)
						}
						if s.ListenToken == "" {
							if token, err := remote.RandomToken(); err == nil {
								s.ListenToken = token
							}
						}
					} else {
						s.SetTCPListen("", "")
					}
					a.SetSettingsNow(s)
					ResetP2P(a)
				})
			})
		})
		if !on {
			return
		}
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTCPAddress()).FontSize(13.5).TextColor(pal.Text)
				if d := locales.SetTCPAddressDetail(); d != "" {
					ui.Text(c, d).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
				}
			})
			if in := controls.TextField(c, "tcp-addr", &d.Addr, ":"+settings.TailcatListenPortDefault(), 140, pal).Font("monospace"); in.Submitted() {
				saveP2P(a, d, "tcp-addr")
			}
		})
		tailcatFieldErr(c, pal, d.Err, "tcp-addr")
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.SetTCPPassword()).FontSize(13.5).TextColor(pal.Text)
			if d := locales.SetTCPPasswordDetail(); d != "" {
				ui.Text(c, d).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, d.Token).Font("monospace").FontSize(12.5).TextColor(pal.Text).Selectable().MinWidth(0)
			ui.Row(c).Grow(1)
			icon, label := glyph.Copy, locales.HostCopyAddress()
			if time.Since(d.Copied) < 1500*time.Millisecond {
				icon, label = glyph.Of("lucide-check"), locales.ToastCopied()
			}
			paints.IconButton(c, "copy-token", icon, label, pal, func() {
				c.WriteClipboard(d.Token)
				d.Copied = time.Now()
				go func() {
					time.Sleep(1500 * time.Millisecond)
					a.Update(func() {})
				}()
			})
			paints.IconButton(c, "new-token", glyph.Reload, locales.SetTCPNew(), pal, func() {
				token, err := remote.RandomToken()
				if err != nil {
					a.SettingsToast(err.Error())
					return
				}
				s := a.SettingsNow().Clone()
				s.ListenToken = token
				a.SetSettingsNow(s)
				ResetP2P(a)
			})
		})
		// Where a client of another machine reaches this one: its own
		// addresses, which the listener's port follows, and the code a
		// phone scans to take the first of them.
		addrs := tcpAddrs(a.SettingsNow())
		if len(addrs) == 0 {
			return
		}
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.SetTCPQR()).FontSize(13.5).TextColor(pal.Text)
		})
		// The code carries the address and the password together, as a
		// client of the listener takes them: tcp://host:port?token=….
		code := tcpCode(addrs[0], a.SettingsNow().ListenToken)
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			if q, err := qrcode.Encode(code, qrcode.L); err == nil {
				qrcode.View(c, q).Size(140, 140).Shrink(0)
			}
			ui.Column(c).Gap(4).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, code).Font("monospace").FontSize(12.5).TextColor(pal.Text).Selectable().SingleLine().Ellipsis("…")
				for _, addr := range addrs {
					ui.Text(c, addr).Font("monospace").FontSize(12).TextColor(pal.TextFaint).Selectable().SingleLine().Ellipsis("…")
				}
			})
		})
	})
}

// ResetP2P reads the P2P form's fields again from the settings.
func ResetP2P(a App) {
	s := *a.SettingsNow()
	*a.P2PDraft() = draft.P2PDraft{Addr: tcpListenAddr(&s), Token: s.ListenToken}
}

// hasTCPListener reports whether s listens on this machine's network.
func hasTCPListener(s *settings.Settings) bool {
	for _, l := range s.Listen {
		if spec, err := remote.ParseListen(l); err == nil && spec.Kind == "tcp" {
			return true
		}
	}
	return false
}

// tcpListenAddr is the address of the network listener, "" for every one
// of this machine's.
func tcpListenAddr(s *settings.Settings) string {
	for _, l := range s.Listen {
		if spec, err := remote.ParseListen(l); err == nil && spec.Kind == "tcp" {
			return net.JoinHostPort(spec.Host, spec.Port)
		}
	}
	return ""
}

// tcpCode is what the code a phone scans carries: the listener's address
// and, with it, the password that admits it, as a client takes them.
func tcpCode(addr, token string) string {
	if token == "" {
		return "tcp://" + addr
	}
	return "tcp://" + addr + "?token=" + url.QueryEscape(token)
}

// tcpAddrs are the addresses a client of another machine reaches the
// network listener At: the one it names, or this machine's own, with the
// listener's port.
func tcpAddrs(s *settings.Settings) []string {
	port := tcpListenPort(s)
	if port == "" {
		return nil
	}
	for _, l := range s.Listen {
		if spec, err := remote.ParseListen(l); err == nil && spec.Kind == "tcp" && spec.Host != "" {
			if ip := net.ParseIP(spec.Host); ip == nil || !ip.IsUnspecified() {
				return []string{net.JoinHostPort(spec.Host, port)}
			}
		}
	}
	var out []string
	for _, addr := range remote.LocalAddrs() {
		out = append(out, net.JoinHostPort(addr, port))
	}
	return out
}

// tcpListenPort is the port of the network listener, "" for none.
func tcpListenPort(s *settings.Settings) string {
	for _, l := range s.Listen {
		if spec, err := remote.ParseListen(l); err == nil && spec.Kind == "tcp" {
			return spec.Port
		}
	}
	return ""
}

// save checks and keeps the Tailcat form's field keyed key, which is one
// of its fields set.
func saveTailcat(a App, d *draft.TailcatDraft, key string) {
	switch key {
	case "tailcat-relay":
		v := strings.TrimSpace(d.Relay)
		if v != "" && !settings.ValidDERPMapURL(v) {
			d.Err = draft.TailcatField{Key: key, Err: v + ": a relay is the https URL of a DERP map"}
			return
		}
		s := a.SettingsNow().Clone()
		s.TailcatRelay = v
		a.SetSettingsNow(s)
	case "tailcat-listen-port":
		v := strings.TrimSpace(d.ListenPort)
		if !settings.ValidListenPort(v) {
			d.Err = draft.TailcatField{Key: key, Err: v + ": a port is 1 to 65535"}
			return
		}
		s := a.SettingsNow().Clone()
		s.SetTailcatListen(v)
		a.SetSettingsNow(s)
	}
	d.Err = draft.TailcatField{}
}

// saveP2P checks and keeps the P2P form's field keyed key, which is one
// of its fields set.
func saveP2P(a App, d *draft.P2PDraft, key string) {
	if key == "tcp-addr" {
		addr := strings.TrimSpace(d.Addr)
		if addr == "" {
			addr = ":" + settings.TailcatListenPortDefault()
		}
		spec, err := remote.ParseListen("tcp://" + addr)
		if err != nil {
			d.Err = draft.TailcatField{Key: key, Err: err.Error()}
			return
		}
		s := a.SettingsNow().Clone()
		s.SetTCPListen(spec.Host, spec.Port)
		a.SetSettingsNow(s)
	}
	d.Err = draft.TailcatField{}
}
