package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/timelineview"
)

// The app, as the timeline's card asks it.
var _ timelineview.App = (*App)(nil)

func (a *App) AgentPane(s *proto.Session) *proto.Pane { return s.PaneByID(a.focusOf(s)) }

func (a *App) TimelineNow(pane string) *timelineview.Info {
	if a.Timelines == nil {
		return nil
	}
	return a.Timelines[pane]
}

func (a *App) SetTimeline(pane string, st *timelineview.Info) {
	if a.Timelines == nil {
		a.Timelines = map[string]*timelineview.Info{}
	}
	a.Timelines[pane] = st
}

// InfoTimeline draws the timeline's card.
func (a *App) InfoTimeline(c *ui.Context, pal *theme.Palette) { timelineview.Card(a, c, pal) }

// TimelineNone tells whether the Info view shows no timeline of the pane:
// the pane runs no agent whose records the timeline reads.
func (a *App) TimelineNone(p *proto.Pane) bool { return !timelineview.Agents(p) }
