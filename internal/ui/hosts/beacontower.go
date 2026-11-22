package hosts

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/xingkaixin/mygo-native-ui/ui/qrcode"

	"hyla/internal/pocket"
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

// BeaconTowerGroup draws Settings › Remote Connection › BeaconTower: the
// relay URL & optional token, remote BeaconTower machines added by pair code,
// and this machine's own encrypted BeaconTower listener & QR pair code.
func BeaconTowerGroup(a App, c *ui.Context, pal *theme.Palette) {
	kit.Group(c, pal, locales.SetBeaconTowerTitle(), "", func() {
		d := a.BeaconTowerDraft()
		s := a.SettingsNow()
		listen := hasBeaconTowerListener(s)
		if !d.Loaded || (!d.RelayFocused && !d.TokenFocused &&
			(s.BeaconTowerRelay != d.SyncedRelay ||
				s.BeaconTowerToken != d.SyncedToken ||
				s.BeaconTowerRotate != d.SyncedRotate ||
				listen != d.SyncedListen)) {
			ResetBeaconTower(a)
		}
		if kit.Rows++; kit.Rows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
		}
		ui.Column(c).Padding(11, 0).Gap(6).Children(func() {
			ui.Text(c, locales.SetBeaconTowerRelay()).FontSize(13.5).TextColor(pal.Text)
			if detail := locales.SetBeaconTowerRelayDetail(); detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
			}
			inRelay := controls.TextField(c, "bt-relay", &d.Relay, locales.SetBeaconTowerRelayPlaceholder(), 0, pal).Font("monospace")
			relayFocused := inRelay.Focused()
			if inRelay.Submitted() || (d.RelayFocused && !relayFocused && strings.TrimSpace(d.Relay) != a.SettingsNow().BeaconTowerRelay) {
				saveBeaconTower(a, d)
			}
			d.RelayFocused = relayFocused
			tailcatFieldErr(c, pal, d.Err, "bt-relay")

			ui.Text(c, locales.SetBeaconTowerToken()).FontSize(13.5).TextColor(pal.Text).Padding(6, 0, 0, 0)
			inToken := controls.TextField(c, "bt-token", &d.Token, locales.SetBeaconTowerTokenPlaceholder(), 0, pal).Font("monospace")
			tokenFocused := inToken.Focused()
			if inToken.Submitted() || (d.TokenFocused && !tokenFocused && strings.TrimSpace(d.Token) != a.SettingsNow().BeaconTowerToken) {
				saveBeaconTower(a, d)
			}
			d.TokenFocused = tokenFocused
		})
		if kit.Rows++; kit.Rows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
		}
		BeaconTowerHosts(a, c, pal)
		if kit.Rows++; kit.Rows > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
		}
		BeaconTowerListen(a, c, pal, d)
	})
}

// BeaconTowerHosts lists machines added via BeaconTower pair codes and provides
// an inline form to add one.
func BeaconTowerHosts(a App, c *ui.Context, pal *theme.Palette) {
	d := a.BTHostDraft()
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Text(c, locales.SetBeaconTowerHostsTitle()).FontSize(13.5).TextColor(pal.Text)
		for _, r := range a.RemoteHosts() {
			if !r.BeaconTower() {
				continue
			}
			r := r
			summary := shortBeaconCode(r.Addr)
			if hostLabel := remote.BeaconRelayDisplay(r.Addr); hostLabel != "" {
				summary = hostLabel + " · " + summary
			}
			ui.Row(c).Padding(2, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, glyph.Host).Size(14, 14).TextColor(pal.TextMuted).Shrink(0)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, r.Name).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…")
					ui.Text(c, summary).Font("monospace").FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
				})
				paints.IconButton(c, "rm-bt-"+r.Name, glyph.Close, locales.HostRemove(), pal, func() {
					Remove(a, r.Name)
				})
			})
		}
		add := func() {
			if err := Add(a, d.Name, d.Code, "", ""); err != nil {
				d.Err = err.Error()
				return
			}
			*d = draft.BTHostDraft{}
		}
		ui.Row(c).Padding(2, 0, 10, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			if in := controls.TextField(c, "bt-code", &d.Code, locales.SetBeaconTowerHostCode(), 240, pal).Font("monospace"); in.Submitted() {
				add()
			}
			if in := controls.TextField(c, "bt-name", &d.Name, locales.SetTailcatHostName(), 110, pal); in.Submitted() {
				add()
			}
			if style.FieldButton(c, "bt-add", locales.CommonAdd(), true, pal) {
				add()
			}
		})
		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}
	})
}

// BeaconTowerListen draws the host switch and self-contained pair code/QR section.
func BeaconTowerListen(a App, c *ui.Context, pal *theme.Palette, d *draft.BeaconTowerDraft) {
	on := hasBeaconTowerListener(a.SettingsNow())
	ui.Column(c).Padding(11, 0).Gap(8).Children(func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, locales.SetBeaconTowerListen()).FontSize(13.5).TextColor(pal.Text)
				if detail := locales.SetBeaconTowerListenDetail(); detail != "" {
					ui.Text(c, detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
				style.Toggle(c.Key("bt-listen"), &on, pal).OnClick(func() {
					s := a.SettingsNow().Clone()
					if on {
						if !saveDraftRelayAndToken(&s, d) {
							return
						}
					}
					s.SetBeaconTowerListen(on)
					a.SetSettingsNow(s)
					ResetBeaconTower(a)
				})
			})
		})
		if !on {
			return
		}
		if strings.TrimSpace(a.SettingsNow().BeaconTowerRelay) == "" {
			ui.Text(c, locales.SetBeaconTowerNeedsRelay()).FontSize(12.5).TextColor(pal.TextMuted).LineHeight(1.35)
			return
		}
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, locales.SetBeaconTowerCode()).FontSize(13.5).TextColor(pal.Text)
			if detail := locales.SetBeaconTowerCodeDetail(); detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
			}
		})
		if d.QR != nil {
			qrcode.View(c, d.QR).Size(160, 160).Shrink(0)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			code, col := d.Code, pal.Text
			if d.CodeErr != "" {
				code, col = d.CodeErr, pal.Danger
			} else if code == "" {
				code, col = locales.SetTailcatStarting(), pal.TextMuted
			}
			ui.Text(c, code).Font("monospace").FontSize(12.5).TextColor(col).LineHeight(1.35).Selectable().MinWidth(0)
			ui.Row(c).Grow(1)
			if d.Code == "" || d.CodeErr != "" {
				return
			}
			icon, label := glyph.Copy, locales.HostCopyAddress()
			if time.Since(d.Copied) < 1500*time.Millisecond {
				icon, label = glyph.Of("lucide-check"), locales.ToastCopied()
			}
			paints.IconButton(c, "copy-beacontower", icon, label, pal, func() {
				c.WriteClipboard(d.Code)
				d.Copied = time.Now()
				go func() {
					time.Sleep(1500 * time.Millisecond)
					a.Update(func() {})
				}()
			})
			paints.IconButton(c, "change-beacontower", glyph.Reload, locales.SetBeaconTowerChange(), pal, func() {
				a.SettingsConfirm(&confirm.Ask{
					Open:    true,
					Title:   locales.SetBeaconTowerChange(),
					Action:  locales.SetBeaconTowerChange(),
					Destroy: true,
					Run:     func() { RotateBeaconTowerCode(a) },
				})
			})
		})
	})
}

// ResetBeaconTower reloads the BeaconTower draft fields and pair code from settings.
func ResetBeaconTower(a App) {
	s := *a.SettingsNow()
	d := a.BeaconTowerDraft()
	*d = draft.BeaconTowerDraft{
		Relay:        s.BeaconTowerRelay,
		Token:        s.BeaconTowerToken,
		Loaded:       true,
		SyncedRelay:  s.BeaconTowerRelay,
		SyncedToken:  s.BeaconTowerToken,
		SyncedListen: hasBeaconTowerListener(&s),
		SyncedRotate: s.BeaconTowerRotate,
	}
	RefreshBeaconTowerCode(a)
}

// RefreshBeaconTowerCode recomputes the displayed pair code and QR image from current settings.
func RefreshBeaconTowerCode(a App) {
	s := a.SettingsNow()
	d := a.BeaconTowerDraft()
	if strings.TrimSpace(s.BeaconTowerRelay) == "" || !hasBeaconTowerListener(s) {
		d.Code, d.CodeErr, d.QR = "", "", nil
		return
	}
	rawCode, err := remote.LocalBeaconTowerAddress(s.BeaconTowerRelay, s.BeaconTowerToken)
	if err != nil {
		d.Code, d.CodeErr, d.QR = "", err.Error(), nil
		return
	}
	d.Code = "beacontower://" + rawCode
	d.CodeErr = ""
	// HylaPocket scans hyla://pair?code=bt1_...&name=... to add the host and auto-fill its name.
	pairURL := pocket.Code{Pair: rawCode, Name: remote.LocalName()}.CodeURL()
	d.QR, _ = qrcode.Encode(pairURL, qrcode.L)
}

// RotateBeaconTowerCode generates a new keypair & PSK and prompts hylad to reload.
func RotateBeaconTowerCode(a App) {
	s := a.SettingsNow().Clone()
	if _, err := remote.RotateBeaconTowerCode(remote.BeaconTowerKeyPath(), s.BeaconTowerRelay, s.BeaconTowerToken); err != nil {
		a.SettingsToast(err.Error())
		return
	}
	s.BeaconTowerRotate = !s.BeaconTowerRotate
	d := a.BeaconTowerDraft()
	d.SyncedRotate = s.BeaconTowerRotate
	a.SetSettingsNow(s)
	RefreshBeaconTowerCode(a)
	a.SettingsToast(locales.SetBeaconTowerChanged())
}

func saveBeaconTower(a App, d *draft.BeaconTowerDraft) {
	v := strings.TrimSpace(d.Relay)
	if v != "" && !settings.ValidBeaconRelay(v) {
		d.Err = draft.TailcatField{Key: "bt-relay", Err: v + ": use host:port, tls://, ws://, or wss://"}
		return
	}
	s := a.SettingsNow().Clone()
	s.BeaconTowerRelay = v
	s.BeaconTowerToken = strings.TrimSpace(d.Token)
	d.SyncedRelay = s.BeaconTowerRelay
	d.SyncedToken = s.BeaconTowerToken
	d.SyncedListen = hasBeaconTowerListener(&s)
	d.SyncedRotate = s.BeaconTowerRotate
	d.Err = draft.TailcatField{}
	a.SetSettingsNow(s)
	RefreshBeaconTowerCode(a)
}

func saveDraftRelayAndToken(s *settings.Settings, d *draft.BeaconTowerDraft) bool {
	v := strings.TrimSpace(d.Relay)
	if v != "" && !settings.ValidBeaconRelay(v) {
		d.Err = draft.TailcatField{Key: "bt-relay", Err: v + ": use host:port, tls://, ws://, or wss://"}
		return false
	}
	s.BeaconTowerRelay = v
	s.BeaconTowerToken = strings.TrimSpace(d.Token)
	d.Err = draft.TailcatField{}
	return true
}

func hasBeaconTowerListener(s *settings.Settings) bool {
	_, ok := s.BeaconTowerListen()
	return ok
}

func shortBeaconCode(addr string) string {
	code := strings.TrimPrefix(addr, "beacontower://")
	if len(code) > 14 {
		return code[:14] + "…"
	}
	return code
}
