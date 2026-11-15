package window

import (
	"hyla/internal/app/host"
	"hyla/internal/app/link"
)

// The app, as its connections ask it.
var _ link.App = (*App)(nil)

// The names Local, Update, ReportView, MergeState, Reselect and
// DropTerms are others' already: the app's own take the "Now" suffix.
func (a *App) MachinesNow() []*host.Host { return a.Machines }

// StatusesForget forgets the statuses the app last saw of a host's
// sessions.
func (a *App) StatusesForget(h *host.Host) { a.forgetStatuses(h) }
