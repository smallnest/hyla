package hosts

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/theme"
)

// App is what the machines' pages and the sidebar ask of the app.
//
// The first three answers of the prefs and the hosts are awkward names for
// fields: the app's own methods are taken, or shadow them.
type App interface {
	// RemoteHosts are the other machines the app reaches, and savePrefs
	// keeps them; MachinesNow are the hosts the app holds, this machine's
	// first.
	RemoteHosts() []Remote
	SetRemoteHosts(rs []Remote)
	MachinesNow() []*host.Host
	SetMachines(ms []*host.Host)
	SavePrefs()
	// Local, HostNamed and SessionsOf find a host, and its sessions.
	PaletteLocal() *host.Host
	HostNamed(name string) *host.Host
	SessionsOf(h *host.Host) []proto.Session
	ProjectsOf(h *host.Host) []proto.Project
	// Connect, DropTerms, MergeState, Reselect and UpdateTitle keep the
	// hosts, as they come and go.
	Connect(h *host.Host)
	DropHostTerms(h *host.Host)
	MergeState(h *host.Host, st proto.State)
	Reselect()
	UpdateTitle()
	// SelectSession and NewSessionOn act on one, and HostState draws how
	// the app stands with it.
	PaletteSelectSession(id string)
	NewSessionOn(h *host.Host, projectID string)
	HostState(c *ui.Context, pal *theme.Palette, h *host.Host)
	// SetConfirm asks the app's confirmation.
	SetConfirm(a *confirm.Ask)
	// HostDraft, PairCodeDraft and P2PDraft are the forms' drafts, and
	// TCPListen is this machine's listener.
	HostDraft() *draft.HostDraft
	PairCodeDraft() *draft.TCHostDraft
	P2PDraft() *draft.TailcatDraft
	TCPListen(c *ui.Context, pal *theme.Palette, d *draft.TailcatDraft)
}
