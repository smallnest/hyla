package main

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/ui/picker"
	"hyla/internal/ui/theme"
)

// The app, as the picker lists and opens what it offers.
var _ picker.App = (*App)(nil)

func (a *App) OpenProjectAt(h *host.Host, dir string) { a.openProjectAt(h, dir) }

// openProjectOn lists a host's folders, from its home, to open a project
// in one.
func (a *App) openProjectOn(h *host.Host) { picker.Open(&a.Picker, a, h) }

// pickerGo lists a folder of the picker's host.
func (a *App) pickerGo(dir string) { picker.GoTo(&a.Picker, a, dir) }

func (a *App) pickerView(c *ui.Context, pal *theme.Palette) { picker.View(c, &a.Picker, a, pal) }

// pickerHosts is the picker's switch of machines: the one it lists, and
// each other, which a click lists instead.
func (a *App) pickerHosts(c *ui.Context, pal *theme.Palette, shown *host.Host) {
	picker.Hosts(c, &a.Picker, a, a.Hosts, pal)
}
