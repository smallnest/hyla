package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
	"hyla/internal/ui/hosts"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/theme"
)

// The Tailcat and P2P groups are internal/ui/hosts' (tailcat.go), which
// the app answers here.

func (a *App) SettingsNow() *settings.Settings { return &a.settingsIn }

func (a *App) SetSettingsNow(s settings.Settings) { a.settingsIn = s }

func (a *App) tailcatGroup(c *ui.Context, pal *theme.Palette) { hosts.TailcatGroup(a, c, pal) }

func (a *App) beaconTowerGroup(c *ui.Context, pal *theme.Palette) {
	hosts.BeaconTowerGroup(a, c, pal)
}

func (a *App) resetTailcat() { hosts.ResetTailcat(a) }

func (a *App) resetBeaconTower() { hosts.ResetBeaconTower(a) }

func (a *App) resetP2P() { hosts.ResetP2P(a) }

func (a *App) tcpListen(c *ui.Context, pal *theme.Palette, d *draft.P2PDraft) {
	hosts.TCPListen(a, c, pal, d)
}
