// Package k8sdialog is the dialog that adds a kubectl terminal to the
// sidebar: the command that enters a pod, as kubectl exec takes it.
package k8sdialog

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/controls"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// Ask is the dialog: Open while it shows, the command the user typed,
// and what went wrong as the pod was added.
type Ask struct {
	Open bool
	Cmd  string
	Err  string
}

// View draws the dialog while it is open, and reports whether the user
// said to connect.
func View(c *ui.Context, pal *theme.Palette, d *Ask) (connect bool) {
	if d == nil {
		return false
	}
	ui.DialogBase(c.Key("k8s"), &d.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		style.Floating(panel.Width(520).Padding(22).Gap(8).Radius(14).Background(pal.Popover), pal).Label(locales.ActionAddK8sPod())
		ui.Text(c, locales.ActionAddK8sPod()).FontSize(15).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, locales.K8sPodDetail()).FontSize(13).TextColor(pal.TextMuted).LineHeight(1.4)
		ui.Row(c).Margin(8, 0, 1, 0).Children(func() {
			in := controls.TextField(c, "k8s-cmd", &d.Cmd, locales.K8sPodCommandPlaceholder(), 0, pal).Font("monospace")
			in.AutoFocus()
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
