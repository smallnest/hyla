package window

import (
	"github.com/egoist/mygo/ui"

	"github.com/egoist/mygo"
	"hyla/internal/ui/theme"

	"hyla/internal/app/host"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/hostlink"

	"hyla/locales"
)

// The app, as the machines' state asks it.
var _ hostlink.App = (*App)(nil)

func (a *App) OwnerOf() map[string]*host.Host { return a.Owner }

func (a *App) SetOwner(o map[string]*host.Host) { a.Owner = o }

func (a *App) ConfirmRestart(h *host.Host) {
	a.Confirm = &confirm.Ask{
		Open:    true,
		Title:   locales.HostRestartTitle(h.Label()),
		Detail:  locales.HostRestartDetail(h.Label()),
		Action:  locales.CommonRestart(),
		Destroy: true,
		Run:     func() { a.restartDaemon() },
	}
	if a.Win != nil {
		a.Win.Focus()
	}
}

func (a *App) DaemonVersion() string { return mygo.App.Version() }

// hostStateNow says how the app stands with a machine's hylad.
func (a *App) HostState(c *ui.Context, pal *theme.Palette, h *host.Host) {
	hostlink.State(a, c, pal, h)
}
