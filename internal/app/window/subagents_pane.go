package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/subagentsview"
	"hyla/internal/ui/theme"
)

// openSubagents splits a subagents pane beside the pane the user is in: the
// subagents of the session's agents, next to the agents themselves, which
// the session's layout, zoom and closing treat as any other pane. The pane
// is open once: asking again goes to it.
func (a *App) openSubagents() {
	s := a.session(a.Selected)
	if s == nil {
		return
	}
	if p := subagentsPaneOf(s); p != nil {
		a.setFocus(s, p.ID)
		return
	}
	at := a.focusOf(s)
	if at == "" {
		return
	}
	a.split(s, proto.CreateParams{Split: at, Direction: "right", Subagents: true})
}

// subagentsPaneOf is the pane of a session that shows its subagents, nil
// when none does.
func subagentsPaneOf(s *proto.Session) *proto.Pane {
	for i := range s.Panes {
		if s.Panes[i].SubagentsPane() {
			return &s.Panes[i]
		}
	}
	return nil
}

// SubagentsBody draws a subagents pane: the subagents of the session's
// agents, whole.
func (a *App) SubagentsBody(c *ui.Context, pal *theme.Palette, s *proto.Session, _ *proto.Pane) {
	subagentsview.Pane(a, c, pal, s)
}
