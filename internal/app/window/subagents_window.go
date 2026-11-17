package window

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/subagentsview"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// openSubagentsWindow shows the subagents in a window of its own, as the
// menu opens one for another screen: the subagents of the session the user
// is in, which it follows from session to session while it is open.
func (a *App) openSubagentsWindow() {
	if a.SubagentsWin != nil {
		a.SubagentsWin.Show()
		a.SubagentsWin.Focus()
		return
	}
	a.SubagentsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:     locales.SideSubagents(),
		Width:     560,
		Height:    720,
		MinWidth:  360,
		MinHeight: 260,
		StateKey:  "subagents",
		Vibrancy:  vibrancyFor(a.settingsIn),
		Content:   ui.View(func(c *ui.Context) { a.subagentsWindow(c) }),
	})
	win := a.SubagentsWin
	win.OnClosed(func() {
		if a.SubagentsWin == win {
			a.SubagentsWin = nil
		}
	})
}

// subagentsWindow draws the window: the name of the session it shows, and
// the view, whole.
func (a *App) subagentsWindow(c *ui.Context) {
	pal := theme.For(c)
	s := a.session(a.Selected)
	ui.Column(c).MinWidth(0).Grow(1).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Height(44).Padding(0, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			title := locales.SideSubagents()
			if s != nil {
				title = s.DisplayName(a.settingsIn.SessionName)
			}
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine()
		})
		ui.Box(c).Height(1).Background(pal.Text.Alpha(0.07))
		if s == nil {
			ui.Column(c).Grow(1).Center().Padding(24).Children(func() {
				ui.Text(c, locales.SubagentsNoSession()).FontSize(12.5).TextColor(pal.TextMuted).TextAlign(ui.Center).Wrap()
			})
			return
		}
		subagentsview.Window(a, c, pal, s)
	})
}
