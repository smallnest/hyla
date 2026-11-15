package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/theme"
)

// The app, as the card asks it.
var _ layout.Carder = (*App)(nil)

// CacheShown reports whether the app's state came: a session shows once it
// did.
func (a *App) CacheShown() bool { return a.Loaded }

// SessionOr is a session the app knows, nil for none.
func (a *App) SessionOr(id string) *proto.Session { return a.session(id) }

// Glass is a color of the window, as the settings make it.
func (a *App) Glass(c ui.Color, pal *theme.Palette) ui.Color {
	return theme.TranslucentGlass(c, pal, a.settingsIn)
}

func (a *App) PanesIn(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, right bool) {
	a.panes(c, pal, bar, shown, s, right)
}

func (a *App) CardDrop(s *proto.Session, files []string) { a.cardDrop(s, files) }

func (a *App) SideEdge(c *ui.Context, pal *theme.Palette, row float32) { a.sideEdge(c, pal, row) }

func (a *App) SidePanel(c *ui.Context, pal *theme.Palette, s *proto.Session, width float32, right bool) {
	a.sidePanel(c, pal, s, width, right)
}

// RowW is the width the side panel leaves the panes, as the last frame
// had it.
func (a *App) RowW() float32 { return a.Side.RowW }

func (a *App) HeaderPad() float32 { return a.ChromeState.HeaderPad }

func (a *App) ButtonsLeft(bar ui.TitleBar) float32 { return a.ChromeState.ButtonsLeft(bar) }

// Retry connects to a machine's hylad again.
func (a *App) Retry(h *host.Host) {
	h.Retries = 0
	h.Conn = host.StateConnecting
	a.connect()
}

// SwitchMode is how a session comes in place of another, as the settings
// say.
func (a *App) SwitchMode() panes.SessionSwitch { return panes.Switch(a.settingsIn.SessionSwitch) }
