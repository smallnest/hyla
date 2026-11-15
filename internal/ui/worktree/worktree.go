// Package worktree is a session's own git worktree: the dialog that asks
// the branch it checks out.
package worktree

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// Ask is the question of a new session's branch, which is made in a
// worktree of the host with p.
type Ask struct {
	Open   bool
	Branch string
	Host   *host.Host
	P      proto.CreateParams
}

// App is what the dialog asks of the app.
type App interface {
	// Create starts a session: on host, with p, in the project named.
	Create(h *host.Host, p proto.CreateParams, project string)
	// FocusTerminal gives the focus back to the terminals.
	FocusTerminal()
}

// View draws the dialog, and makes the session as the user says. It
// returns the branch the session is made with, when there is one.
func View(x App, c *ui.Context, pal *theme.Palette, w *Ask) (branch string, made bool) {
	if w == nil {
		return "", false
	}
	ui.DialogBase(c.Key("worktree"), &w.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		style.Floating(panel.Width(420).Padding(22).Gap(8).Radius(14).Background(pal.Popover), pal).Label(locales.WorktreeTitle())
		ui.Text(c, locales.WorktreeTitle()).FontSize(15).FontWeight(650).TextColor(pal.Text)
		detail := locales.WorktreeDetailDefault()
		if w.P.Base == "HEAD" {
			detail = locales.WorktreeDetailCurrent()
		}
		ui.Text(c, detail).FontSize(13).TextColor(pal.TextMuted).LineHeight(1.4)
		ui.Row(c).Margin(8, 0, 0, 0).Children(func() {
			in := controls.TextField(c, "branch", &w.Branch, locales.WorktreeBranch(), 0, pal)
			in.AutoFocus()
			if in.Submitted() {
				made = true
			}
		})
		ui.Row(c).Gap(8).Justify(ui.End).Margin(14, 0, 0, 0).Children(func() {
			if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				w.Open = false
			}
			if style.Button(c, "ok", locales.WorktreeCreate(), "↩", true, pal) {
				made = true
			}
		})
	})
	if b := strings.TrimSpace(w.Branch); made && b != "" {
		w.Open = false
		return b, true
	}
	if !w.Open {
		x.FocusTerminal()
	}
	return "", false
}
