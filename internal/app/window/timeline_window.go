package window

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/marks"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/timelineview"
	"hyla/locales"
)

// openTimeline shows the timeline's window, as the menu opens it: the
// turns of the agent in the pane the user is in, which it follows from
// pane to pane and session to session while it is open.
func (a *App) openTimeline() {
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

// timelineWindow draws the window: its title, the agent's mark and name in
// the pane the user is in, and the timeline's card, whole. A pane of
// another machine's has the timeline of this machine's records alone: the
// window says so, as its own does.
func (a *App) timelineWindow(c *ui.Context) {
	pal := theme.For(c)
	p := a.timelinePane()
	ui.Column(c).MinWidth(0).Grow(1).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Height(44).Padding(0, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			if a.TimelineMark != "" {
				if full, _ := marks.MarkFull(a.TimelineMark); full != nil {
					ui.Image(c, full).Size(marks.Size, marks.Size).Shrink(0)
				} else if logo, brand, ok := marks.Agent(a.TimelineMark); ok {
					ui.Icon(c, logo).Size(16, 16).TextColor(marks.Tint(brand, pal.Dark, pal.Text)).Shrink(0)
				}
			}
			title := locales.InfoTimeline()
			if a.TimelineMark != "" {
				title = a.TimelineMark
			}
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine()
			ui.Spacer(c)
			if p != nil {
				if dir := a.workDir(p.ID); dir != "" {
					ui.Text(c, dir).FontSize(11.5).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
				}
			}
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

// timelinePane is the pane the window shows: the one the user is in.
func (a *App) timelinePane() *proto.Pane {
	if s := a.session(a.Selected); s != nil {
		return s.PaneByID(a.focusOf(s))
	}
	return nil
}
