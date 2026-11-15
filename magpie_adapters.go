package main

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

// infoMagpie draws Magpie's card.
func (a *App) infoMagpie(c *ui.Context, pal *theme.Palette) { magpieview.Card(a, c, pal) }
