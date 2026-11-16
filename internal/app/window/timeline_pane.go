package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/timelineview"
)

// openTimeline splits a timeline pane beside the pane the user is in: the
// turns of that pane's agent, next to the agent itself, which the
// session's layout, zoom and closing treat as any other pane. The pane's
// timeline is open once: asking again goes to it.
func (a *App) openTimeline() {
	s := a.session(a.Selected)
	if s == nil {
		return
	}
	at := a.timelineAnchor(s)
	if at == "" {
		return
	}
	if q := a.timelineOf(s, at); q != nil {
		a.setFocus(s, q.ID)
		return
	}
	a.split(s, proto.CreateParams{Split: at, Direction: "right", Timeline: at})
}

// timelineAnchor is the pane a timeline the user opens now shows: the one
// the user is in, or, in a timeline, the pane that one shows.
func (a *App) timelineAnchor(s *proto.Session) string {
	at := a.focusOf(s)
	if p := s.PaneByID(at); p != nil && p.TimelinePane() {
		if q := a.timelineShown(s, p); q != nil {
			return q.ID
		}
		return ""
	}
	return at
}

// timelineOf is the pane of a session that shows a pane's timeline, nil
// when none does.
func (a *App) timelineOf(s *proto.Session, pane string) *proto.Pane {
	for i := range s.Panes {
		if p := &s.Panes[i]; p.TimelinePane() && p.Timeline == pane {
			return p
		}
	}
	return nil
}

// timelineShown is the pane a timeline pane shows, which the session
// knows. A pane of another machine's is still the pane's: the app reads
// this machine's records alone, which the timeline says.
func (a *App) timelineShown(s *proto.Session, p *proto.Pane) *proto.Pane {
	return s.TimelineShown(p)
}

// TimelineBody draws a timeline pane: the turns of the agent in the pane
// it shows, whole, and the word where that pane runs no agent whose
// records the timeline reads.
func (a *App) TimelineBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	timelineview.Pane(a, c, pal, a.timelineShown(s, p), a.timelineNote)
}
