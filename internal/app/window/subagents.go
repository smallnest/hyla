package window

import (
	"hyla/internal/ui/subagentsview"
)

// The app, as a subagents pane asks it.
var _ subagentsview.App = (*App)(nil)

// SubagentsNow is what a subagents pane shows.
func (a *App) SubagentsNow() *subagentsview.Info { return a.Subagents }

// SetSubagents holds what a subagents pane shows.
func (a *App) SetSubagents(st *subagentsview.Info) { a.Subagents = st }
