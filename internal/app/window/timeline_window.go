package window

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/timelineview"
	"hyla/locales"
)

// openTimelineWindow shows the timeline in a window of its own, as the
// menu opens one for another screen: the turns of the agent in the pane
// the user is in, which it follows from pane to pane and session to
// session while it is open. A pane of the window's own is the usual way
// (timeline_pane.go).
func (a *App) openTimelineWindow() {
	if a.TimelineWin != nil {
		a.TimelineWin.Show()
		a.TimelineWin.Focus()
		return
	}
	a.TimelineWin = mygo.NewWindow(mygo.WindowOptions{
		Title:     locales.ActionTimeline(),
		Width:     680,
		Height:    760,
		MinWidth:  400,
		MinHeight: 300,
		StateKey:  "timeline",
		Vibrancy:  vibrancyFor(a.settingsIn),
		Content:   ui.View(func(c *ui.Context) { a.timelineWindow(c) }),
	})
	win := a.TimelineWin
	win.OnClosed(func() {
		if a.TimelineWin == win {
			a.TimelineWin = nil
		}
	})
}

// timelineWindow draws the window: the name of the agent in the pane the
// user is in, and the timeline's card, whole: no program's mark, as the
// window runs none. A pane of another machine's has the timeline of this
// machine's records alone: the window says so, as its own does.
func (a *App) timelineWindow(c *ui.Context) {
	pal := theme.For(c)
	ui.Column(c).MinWidth(0).Grow(1).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Height(44).Padding(0, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			title := locales.InfoTimeline()
			if a.TimelineMark != "" {
				title = a.TimelineMark
			}
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine()
		})
		ui.Box(c).Height(1).Background(pal.Text.Alpha(0.07))
		timelineview.Window(a, c, pal, a.timelineNote, func(p *proto.Pane) { a.TimelineMark = p.Agent })
	})
}

// timelineNote is the window's word where the pane runs no agent whose
// records the timeline reads, and the card's where it says nothing.
func (a *App) timelineNote(c *ui.Context, pal *theme.Palette, text string) {
	ui.Column(c).Grow(1).Center().Padding(24).Children(func() {
		ui.Text(c, text).FontSize(12.5).TextColor(pal.TextMuted).TextAlign(ui.Center).Wrap()
	})
}
