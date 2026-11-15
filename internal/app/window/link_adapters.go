package window

import (
	"hyla/internal/app/host"
	"hyla/internal/app/link"
	"hyla/internal/client"
	"hyla/internal/proto"
)

// The app, as its connections ask it.
var _ link.App = (*App)(nil)

// The names Local, Update, ReportView, MergeState, Reselect and
// DropTerms are others' already: the app's own take the "Now" suffix.
func (a *App) MachinesNow() []*host.Host { return a.Machines }

func (a *App) UpdateNow(fn func()) { a.update(fn) }

// ReportViewNow tells each hylad which session the user looks at.
func (a *App) ReportViewNow() { a.reportView() }

func (a *App) ReselectNow() { a.reselect() }

// TakeStateOf takes a host's state from its hylad.
func (a *App) TakeStateOf(h *host.Host, st proto.State) { a.setState(h, st) }

func (a *App) DropHostTerms(h *host.Host) { a.dropTerms(h) }

// StatusesForget forgets the statuses the app last saw of a host's
// sessions.
func (a *App) StatusesForget(h *host.Host) { a.forgetStatuses(h) }

// EventOf takes a message from a host's hylad.
func (a *App) EventOf(h *host.Host, cl *client.Client, m proto.Message) { a.event(h, cl, m) }
