package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/platform"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

var (
	minimizeIcon = glyph.Drawn(`<path d="M3.5 8h9"/>`)
	maximizeIcon = glyph.Drawn(`<rect x="3.5" y="3.5" width="9" height="9" rx="1.5"/>`)
	restoreIcon  = glyph.Drawn(`<rect x="3.5" y="5.5" width="7" height="7" rx="1.2"/><path d="M5.5 3.5h5.5a1.5 1.5 0 0 1 1.5 1.5v5.5"/>`)
)

// windowControls are a window's minimize, maximize and close buttons, as
// Hyla draws them (ownControls), at the right of a header.
func windowControls(c *ui.Context, pal *theme.Palette, w *mygo.Window) {
	if !platform.OwnControls || w == nil {
		return
	}
	ui.Row(c.Key("window-controls")).Gap(2).AlignItems(ui.Center).Shrink(0).Margin(0, 0, 0, 6).Children(func() {
		iconButton(c, "minimize", minimizeIcon, locales.WindowMinimize(), pal, w.Minimize)
		// The system's maximize button for the pointer, which brings
		// Windows 11's snap layouts as it rests there: no tooltip over them.
		maximized := w.IsMaximized()
		label, svg := locales.WindowMaximize(), maximizeIcon
		if maximized {
			label, svg = locales.WindowRestore(), restoreIcon
		}
		m := ui.ButtonBase(c.Key("maximize")).Size(28, 26).Radius(6).Center().TextColor(pal.TextMuted).Label(label).MaximizeButton()
		if m.Pressed() {
			m.Background(pal.RowHover.Alpha(0.12))
		} else if m.Hovered() {
			m.Background(pal.RowHover).TextColor(pal.Text)
		}
		m.Children(func() { ui.Icon(c, svg).Size(16, 16) })
		if m.Clicked() {
			if maximized {
				w.Unmaximize()
			} else {
				w.Maximize()
			}
		}
		b := ui.ButtonBase(c.Key("close-window")).Size(28, 26).Radius(6).Center().TextColor(pal.TextMuted).Label(locales.CommonClose()).Tooltip(locales.CommonClose())
		if b.Hovered() || b.Pressed() {
			b.Background(pal.Danger).TextColor(ui.RGB(255, 255, 255))
		}
		b.Children(func() { ui.Icon(c, glyph.Close).Size(16, 16) })
		if b.Clicked() {
			w.Close()
		}
	})
}
