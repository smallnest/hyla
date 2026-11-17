package window

import (
	"hyla/internal/ui/timelineview"
)

// The app, as a timeline asks it.
var _ timelineview.App = (*App)(nil)

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
