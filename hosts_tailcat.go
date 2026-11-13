package main

import (
	"context"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/xingkaixin/mygo-native-ui/ui/qrcode"

	"hyla/internal/remote"
	"hyla/internal/settings"

	"hyla/locales"
)

// Tailcat is this machine as a host for HylaPocket: a relay with a pair
// code (internal/remote/tailcat.go). The tailcat switch in Settings ›
// Hosts turns it on and off: off, hylad starts no listener of it, and the
// phone reaches nothing.

// tailcatField is a field of the Tailcat group that failed: which, as its
// key, and why.
type tailcatField struct {
	key, err string
}

// tailcatDraft is the Tailcat group's form, and the fields it edits: the
// relay, and the port of this machine's listener.
type tailcatDraft struct {
	listenPort string // this machine's listener's, while a host
	relay      string
	tcpAddr    string // the network listener's address, "" for every
	tcpToken   string // its password, as listen-token keeps it
	err        tailcatField
	code       string       // this machine's pair code, as hylad's key makes it
	codeErr    string       // why there is none, as a failure
	qr         *qrcode.Code // code as a QR code, for a phone to scan
	at         time.Time    // when the key was last read, or rotated
	copied     time.Time
	looking    bool
}

// tailcatGroup is Settings › Hosts › Tailcat: the switch, and with it on,
// the hosts Hyla adds and this machine as a host.
func (a *App) tailcatGroup(c *ui.Context, pal *palette) {
	on := a.set.Tailcat
	settingsGroup(c, pal, "", "", func() {
		groupTitle(c, pal, locales.SetTailcatTitle())
		settingRow(c, pal, locales.SetTailcat(), locales.SetTailcatDetail(), func() {
			toggle(c.Key("tailcat"), &on, pal).OnClick(func() { a.setTailcat(on) })
		})
		if !on {
			return
		}
		d := &a.ss.tailcat
		if d.listenPort == "" {
			a.resetTailcat()
		}
		if groupRows++; groupRows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.text.Alpha(0.07))
		}
		a.tailcatListen(c, pal, d)
	})
	if !on && locales.SetTailcatOffNote() != "" {
		settingsGroup(c, pal, "", locales.SetTailcatOffNote(), nil)
	}
}

// setTailcat turns Tailcat on or off, as the switch of Settings › Hosts
// does: off, this machine listens through no relay either, since the
// daemon follows the listener and not the switch, and the line left
// behind would be a configuration problem.
func (a *App) setTailcat(on bool) {
	s := a.set.Clone()
	s.Tailcat = on
	if !on {
		s.SetTailcatListen("")
	}
	a.set = s
	a.resetTailcat()
}

// resetTailcat reads the Tailcat group's fields again from the settings,
// and this machine's pair code with them.
func (a *App) resetTailcat() {
	s := a.set
	d := &a.ss.tailcat
	*d = tailcatDraft{
		relay:      s.TailcatRelay,
		listenPort: settings.TailcatListenPortDefault(),
		tcpAddr:    tcpListenAddr(&s),
		tcpToken:   s.ListenToken,
	}
	if _, port, ok := s.TailcatListen(); ok {
		d.listenPort = port
	}
	if s.Tailcat {
		a.tailcatCode()
	}
}

// tailcatSettingRow is settingRow, which is no row of its group's, here.
func tailcatSettingRow(c *ui.Context, pal *palette, label, detail string, control func()) {
	n := groupRows
	settingRow(c, pal, label, detail, control)
	groupRows = n
}

// tailcatListen is this machine as a host over Tailcat: the switch of
// hylad's tailcat listener here, its port, and the pair code Hyla on
// other machines adds with.
func (a *App) tailcatListen(c *ui.Context, pal *palette, d *tailcatDraft) {
	on := hasTailcatListener(&a.set)
	detail := locales.SetTailcatListenDetail()
	if on {
		detail = locales.SetTailcatOn()
	}
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTailcatListen()).FontSize(13.5).TextColor(pal.text)
				if detail != "" {
					ui.Text(c, detail).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
				toggle(c.Key("tailcat-listen"), &on, pal).OnClick(func() {
					s := a.set.Clone()
					if on {
						port := strings.TrimSpace(d.listenPort)
						if !settings.ValidListenPort(port) {
							port = settings.TailcatListenPortDefault()
						}
						s.SetTailcatListen(port)
					} else {
						s.SetTailcatListen("")
					}
					a.set = s
					a.resetTailcat()
				})
			})
		})
		if !on {
			return
		}
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTailcatListenPort()).FontSize(13.5).TextColor(pal.text)
				if d := locales.SetTailcatListenPortDetail(); d != "" {
					ui.Text(c, d).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
				}
			})
			if in := textField(c, "tailcat-listen-port", &d.listenPort, settings.TailcatListenPortDefault(), 70, pal); in.Submitted() {
				d.save(a, "tailcat-listen-port")
			}
		})
		tailcatFieldErr(c, pal, d, "tailcat-listen-port")
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.SetTailcatCode()).FontSize(13.5).TextColor(pal.text)
			if d := locales.SetTailcatCodeDetail(); d != "" {
				ui.Text(c, d).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
			}
		})
		// What other machines add with the code, as a phone scans it:
		// nothing where there is none.
		if d.qr != nil {
			qrcode.View(c, d.qr).Size(160, 160).Shrink(0)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			code, col := d.code, pal.text
			switch {
			case d.codeErr != "":
				code, col = d.codeErr, pal.danger
			case code == "":
				code, col = locales.SetTailcatStarting(), pal.textMuted
			}
			ui.Text(c, code).Font("monospace").FontSize(12.5).TextColor(col).LineHeight(1.35).Selectable().MinWidth(0)
			ui.Row(c).Grow(1)
			if code == "" || d.codeErr != "" {
				return
			}
			icon, label := copyIcon, locales.HostCopyAddress()
			if time.Since(d.copied) < 1500*time.Millisecond {
				icon, label = checkIcon, locales.ToastCopied()
			}
			iconButton(c, "copy-tailcat", icon, label, pal, func() {
				c.WriteClipboard(code)
				d.copied = time.Now()
				go func() {
					time.Sleep(1500 * time.Millisecond)
					a.update(func() {})
				}()
			})
			iconButton(c, "change-tailcat", reloadIcon, locales.SetTailcatChange(), pal, func() {
				a.ss.confirm = &confirmation{
					open:    true,
					title:   locales.SetTailcatChange(),
					action:  locales.SetTailcatChange(),
					destroy: true,
					run:     func() { a.rotateTailcatCode() },
				}
			})
		})
	})
}

// tailcatFieldErr is the reason a field of the group failed, under it.
func tailcatFieldErr(c *ui.Context, pal *palette, d *tailcatDraft, key string) {
	if d.err.key == key {
		ui.Text(c, d.err.err).FontSize(12).TextColor(pal.danger)
	}
}

func (a *App) tailcatCode() {
	d := &a.ss.tailcat
	if d.looking || !d.at.IsZero() {
		return
	}
	d.looking = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		code, err := remote.LocalTailcatAddress(ctx)
		cancel()
		// The code drawn as a phone's camera reads it: the key alone here,
		// since nothing else adds the scheme a pair code needs.
		var qr *qrcode.Code
		if err == nil {
			qr, _ = qrcode.Encode("tailcat://"+code, qrcode.L)
		}
		a.update(func() {
			d.code, d.codeErr, d.qr, d.looking, d.at = code, errorText(err), qr, false, time.Now()
		})
	}()
}

// rotateTailcatCode gives this machine a new pair code, which the old
// one's no longer opens.
func (a *App) rotateTailcatCode() {
	if _, err := remote.RotateTailcatKey(remote.TailcatKeyPath()); err != nil {
		a.toast(err.Error())
		return
	}
	d := &a.ss.tailcat
	*d = tailcatDraft{relay: d.relay, listenPort: d.listenPort}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		k, err := remote.TailcatKeyAt(ctx, remote.TailcatKeyPath())
		var code string
		if err == nil {
			code, err = remote.TailcatAddress(ctx, k)
		}
		cancel()
		a.update(func() {
			d.code, d.codeErr, d.at = errorText(err), errorText(err), time.Now()
			if err == nil {
				d.code = "tailcat://" + code
			}
		})
	}()
	a.toast(locales.SetTailcatChanged())
	// hylad starts its listener with the key again, which follow finds.
	s := a.set.Clone()
	s.TailcatRotate = !s.TailcatRotate
	a.set = s
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

// tcpListen is this machine on its own network: a tcp:// listener, whose
// address and password a client of Hyla, or of HylaPocket, takes.
func (a *App) tcpListen(c *ui.Context, pal *palette, d *tailcatDraft) {
	on := hasTCPListener(&a.set)
	detail := locales.SetTCPListenDetail()
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTCP()).FontSize(13.5).TextColor(pal.text)
				if detail != "" {
					ui.Text(c, detail).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
				toggle(c.Key("tcp-listen"), &on, pal).OnClick(func() {
					s := a.set.Clone()
					if on {
						addr := strings.TrimSpace(d.tcpAddr)
						if addr == "" {
							addr = ":" + remote.PocketPort
						}
						if spec, err := remote.ParseListen("tcp://" + addr); err == nil {
							s.Listen = slices.DeleteFunc(s.Listen, func(l string) bool {
								o, err := remote.ParseListen(l)
								return err == nil && o.Kind == "tcp"
							})
							s.Listen = append(s.Listen, spec.String())
						}
						if s.ListenToken == "" {
							if token, err := remote.RandomToken(); err == nil {
								s.ListenToken = token
							}
						}
					} else {
						s.Listen = slices.DeleteFunc(s.Listen, func(l string) bool {
							o, err := remote.ParseListen(l)
							return err == nil && o.Kind == "tcp"
						})
					}
					a.set = s
					a.resetTailcat()
				})
			})
		})
		if !on {
			return
		}
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetTCPAddress()).FontSize(13.5).TextColor(pal.text)
				if d := locales.SetTCPAddressDetail(); d != "" {
					ui.Text(c, d).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
				}
			})
			if in := textField(c, "tcp-addr", &d.tcpAddr, ":"+remote.PocketPort, 140, pal).Font("monospace"); in.Submitted() {
				d.save(a, "tcp-addr")
			}
		})
		tailcatFieldErr(c, pal, d, "tcp-addr")
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.SetTCPPassword()).FontSize(13.5).TextColor(pal.text)
			if d := locales.SetTCPPasswordDetail(); d != "" {
				ui.Text(c, d).FontSize(12).TextColor(pal.textMuted).LineHeight(1.35)
			}
		})
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, d.tcpToken).Font("monospace").FontSize(12.5).TextColor(pal.text).Selectable().MinWidth(0)
			ui.Row(c).Grow(1)
			icon, label := copyIcon, locales.HostCopyAddress()
			if time.Since(d.copied) < 1500*time.Millisecond {
				icon, label = checkIcon, locales.ToastCopied()
			}
			iconButton(c, "copy-token", icon, label, pal, func() {
				c.WriteClipboard(d.tcpToken)
				d.copied = time.Now()
				go func() {
					time.Sleep(1500 * time.Millisecond)
					a.update(func() {})
				}()
			})
			iconButton(c, "new-token", reloadIcon, locales.SetTCPNew(), pal, func() {
				token, err := remote.RandomToken()
				if err != nil {
					a.toast(err.Error())
					return
				}
				s := a.set.Clone()
				s.ListenToken = token
				a.set = s
				a.resetTailcat()
			})
		})
		// Where a client of another machine reaches this one: its own
		// addresses, which the listener's port follows, and the code a
		// phone scans to take the first of them.
		addrs := tcpAddrs(&a.set)
		if len(addrs) == 0 {
			return
		}
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.SetTCPQR()).FontSize(13.5).TextColor(pal.text)
		})
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			if q, err := qrcode.Encode(addrs[0], qrcode.L); err == nil {
				qrcode.View(c, q).Size(140, 140).Shrink(0)
			}
			ui.Column(c).Gap(4).Children(func() {
				for _, addr := range addrs {
					ui.Text(c, addr).Font("monospace").FontSize(12.5).TextColor(pal.text).Selectable().SingleLine().Ellipsis("…")
				}
			})
		})
	})
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

// tcpAddrs are the addresses a client of another machine reaches the
// network listener at: the one it names, or this machine's own, with the
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

// save checks and keeps the group's field keyed key, which is one of its
// fields set.
func (d *tailcatDraft) save(a *App, key string) {
	var v string
	switch key {
	case "tailcat-relay":
		v = strings.TrimSpace(d.relay)
		if v != "" && !settings.ValidDERPMapURL(v) {
			d.err = tailcatField{key, v + ": a relay is the https URL of a DERP map"}
			return
		}
	case "tcp-addr":
		addr := strings.TrimSpace(d.tcpAddr)
		if addr == "" {
			addr = ":" + remote.PocketPort
		}
		spec, err := remote.ParseListen("tcp://" + addr)
		if err != nil {
			d.err = tailcatField{key, err.Error()}
			return
		}
		s := a.set.Clone()
		s.Listen = slices.DeleteFunc(s.Listen, func(l string) bool {
			o, err := remote.ParseListen(l)
			return err == nil && o.Kind == "tcp"
		})
		s.Listen = append(s.Listen, spec.String())
		a.set, d.err = s, tailcatField{}
		return
	case "tailcat-listen-port":
		if v = strings.TrimSpace(d.listenPort); !settings.ValidListenPort(v) {
			d.err = tailcatField{key, v + ": a port is 1 to 65535"}
			return
		}
		s := a.set.Clone()
		s.SetTailcatListen(v)
		a.set, d.err = s, tailcatField{}
		return
	}
	s := a.set.Clone()
	if key == "tailcat-relay" {
		s.TailcatRelay = v
	}
	a.set, d.err = s, tailcatField{}
}
