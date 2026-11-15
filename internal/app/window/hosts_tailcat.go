package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
	"hyla/internal/ui/hosts"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/theme"
)

// The Tailcat group is internal/ui/hosts' (tailcat.go), which the app
// answers here.

func (a *App) SettingsNow() *settings.Settings { return &a.settingsIn }

func (a *App) SetSettingsNow(s settings.Settings) { a.settingsIn = s }

func (a *App) tailcatGroup(c *ui.Context, pal *theme.Palette) { hosts.TailcatGroup(a, c, pal) }

func (a *App) resetTailcat() { hosts.ResetTailcat(a) }

func (a *App) tailcatCode() { hosts.TailcatCode(a) }

func (a *App) rotateTailcatCode() { hosts.RotateTailcatCode(a) }

func (a *App) tailcatListen(c *ui.Context, pal *theme.Palette, d *draft.TailcatDraft) {
	hosts.TailcatListen(a, c, pal, d)
}

func (a *App) tcpListen(c *ui.Context, pal *theme.Palette, d *draft.TailcatDraft) {
	hosts.TCPListen(a, c, pal, d)
}
