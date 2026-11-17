package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/subagentsview"
	"hyla/internal/ui/theme"
)

// The app, as the side panel's Subagents view asks it.
var _ subagentsview.App = (*App)(nil)

// SubagentsNow is what the Subagents view shows.
func (a *App) SubagentsNow() *subagentsview.Info { return a.Subagents }

// SetSubagents holds what the Subagents view shows.
func (a *App) SetSubagents(st *subagentsview.Info) { a.Subagents = st }

// SubagentsSide draws the Subagents view of a session's panes.
func (a *App) SubagentsSide(c *ui.Context, pal *theme.Palette, s *proto.Session) {
	subagentsview.Side(a, c, pal, s)
}
