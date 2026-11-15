package main

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
)

// resizeHandle is the sidebar's edge, which a drag moves.
func (a *App) resizeHandle(c *ui.Context, pal *theme.Palette) {
	// A drag leaves the keyboard in the pane the user is in.
	h := ui.Box(c.Key("resize")).Absolute().Left(-4).Top(0).Bottom(0).Width(9).Cursor(ui.CursorResizeColumn).KeepFocus()
	if dx, _, ok := h.Dragged(); ok {
		if a.SideDrag == 0 {
			a.SideDrag = a.Prefs.SidebarWidth
		}
		a.SideDrag += dx
		a.Prefs.SidebarWidth = min(max(a.SideDrag, minSidebar), maxSidebar)
	} else if a.SideDrag != 0 && !h.Pressed() {
		a.SideDrag = 0
		a.savePrefs()
	}
	if h.DoubleClicked() {
		a.Prefs.SidebarWidth = defaultSidebar
		a.savePrefs()
	}
}

func (a *App) message(c *ui.Context, pal *theme.Palette, title, detail, action string, fn func()) {
	ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Text(c, title).FontSize(17).FontWeight(600).TextColor(pal.Text)
		ui.Text(c, detail).FontSize(13).TextColor(pal.TextMuted).MaxWidth(420).TextAlign(ui.Center).LineHeight(1.4)
		if action != "" {
			ui.Box(c).Height(8)
			if style.Button(c, "msg-action", action, "", true, pal) {
				fn()
			}
		}
	})
}
