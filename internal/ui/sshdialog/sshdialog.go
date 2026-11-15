// Package sshdialog is the dialog that adds an SSH terminal to the
// sidebar: the address the user types, as ssh takes it, and the name the
// SSH project gives the machine.
package sshdialog

import (
	"hyla/internal/remote"

	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/controls"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// Ask is the dialog: Open while it shows, the address and the name the
// user typed, and what went wrong as the machine was added.
type Ask struct {
	Open bool
	Addr string
	Name string
	Err  string
}

// View draws the dialog while it is open, and reports whether the user
// said to connect.
func View(c *ui.Context, pal *theme.Palette, d *Ask) (connect bool) {
	if d == nil {
		return false
	}
	ui.DialogBase(c.Key("ssh"), &d.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		style.Floating(panel.Width(420).Padding(22).Gap(8).Radius(14).Background(pal.Popover), pal).Label(locales.ActionAddSSHHost())
		ui.Text(c, locales.ActionAddSSHHost()).FontSize(15).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, locales.SshHostDetail()).FontSize(13).TextColor(pal.TextMuted).LineHeight(1.4)
		ui.Row(c).Margin(8, 0, 1, 0).Children(func() {
			in := controls.TextField(c, "ssh-addr", &d.Addr, locales.SshHostAddressPlaceholder(), 0, pal).Font("monospace")
			in.AutoFocus()
			if in.Submitted() {
				connect = true
			}
		})
		ui.Row(c).Children(func() {
			in := controls.TextField(c, "ssh-name", &d.Name, locales.SetHostNamePlaceholder(DefaultName(d.Addr)), 0, pal)
			if in.Submitted() {
				connect = true
			}
		})
		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Margin(14, 0, 0, 0).Children(func() {
			if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				d.Open = false
			}
			if style.Button(c, "ok", locales.CommonConnect(), "↩", true, pal) {
				connect = true
			}
		})
	})
	return connect
}

// DefaultName is the name a machine takes while the user types none: the
// address's host, as config names it.
func DefaultName(addr string) string {
	a, err := remote.ParseAddr(addr)
	if err != nil || a.Host == "" {
		return "devbox"
	}
	return a.Host
}
