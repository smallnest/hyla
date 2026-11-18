package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/paneinfo"
	"hyla/internal/proto"
	"hyla/internal/ui/info"
	"hyla/internal/ui/theme"
)

// The app, as the Info view reads and runs what it shows.
var _ info.App = (*App)(nil)

func (a *App) InfoViews() *paneinfo.Panes { return &a.Info }

func (a *App) WorkDir(id string) string { return a.workDir(id) }

func (a *App) FocusOf(s *proto.Session) string { return a.focusOf(s) }

func (a *App) InfoScroll() *ui.ScrollState { return &a.Side.Info }

func (a *App) Folded() map[string]bool {
	if a.Prefs.InfoFolded == nil {
		a.Prefs.InfoFolded = map[string]bool{}
	}
	return a.Prefs.InfoFolded
}

func (a *App) AfterFrame(fn func()) { a.RunLater(fn) }

func (a *App) SetFocus(s *proto.Session, pane string) { a.setFocus(s, pane) }

func (a *App) Split(s *proto.Session, p proto.CreateParams) { a.split(s, p) }

func (a *App) InfoMagpie(c *ui.Context, pal *theme.Palette) { a.infoMagpie(c, pal) }

func (a *App) InfoPluginCards(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	a.infoPluginCards(c, pal, s, p)
}

func (a *App) InfoFeishu(c *ui.Context, pal *theme.Palette, p *proto.Pane) { a.infoFeishu(c, pal, p) }

// infoView is the side panel's Info view: of the pane the user is in.
func (a *App) InfoSide(c *ui.Context, pal *theme.Palette, s *proto.Session) { info.View(c, a, pal, s) }
