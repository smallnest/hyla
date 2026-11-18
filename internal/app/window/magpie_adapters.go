package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/magpieview"
	"hyla/internal/ui/theme"
)

// The app, as Magpie's card asks it.
var _ magpieview.App = (*App)(nil)

// magpieInfo is what the Info view shows of Magpie.
type magpieInfo = magpieview.Info

func (a *App) MagpieNow() *magpieview.Info { return a.Magpie }

func (a *App) SetMagpie(st *magpieview.Info) { a.Magpie = st }

// hasMagpieFixture reports whether the card should fall back to magpieview
// (only when Plugins is nil, or an offline test pre-seeded a.Magpie with no
// daemon client and the hyla.magpie plugin still enabled).
func (a *App) hasMagpieFixture() bool {
	if a.Magpie == nil || (a.Magpie.Usage == nil && a.Magpie.Err == "") {
		return false
	}
	if a.Plugins == nil {
		return true
	}
	if !a.Plugins.IsPluginEnabled("hyla.magpie") {
		return false
	}
	return a.Local().Client == nil
}

// infoMagpie draws Magpie's card.
func (a *App) infoMagpie(c *ui.Context, pal *theme.Palette) {
	if a.Plugins != nil && !a.hasMagpieFixture() {
		return
	}
	magpieview.Card(a, c, pal)
}
