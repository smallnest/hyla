package main

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/hosts"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/theme"
)

// The app, as the machines' pages and the sidebar ask it.
// The names Local and SelectSession are the palette's already: PaletteLocal
// and PaletteSelectSession.
var _ hosts.App = (*App)(nil)

func (a *App) DropHostTerms(h *host.Host) { a.dropTerms(h) }

// RemoteHosts are the other machines the app reaches.
func (a *App) RemoteHosts() []hosts.Remote { return a.Prefs.RemoteHosts }

func (a *App) SetRemoteHosts(rs []hosts.Remote) { a.Prefs.RemoteHosts = rs }

// MachinesNow are the hosts the app holds, this machine's first.
func (a *App) MachinesNow() []*host.Host { return a.Machines }

func (a *App) SetMachines(ms []*host.Host) { a.Machines = ms }

// PaletteLocal and PaletteSelectSession are the palette's names, which
// this file does not take.
func (a *App) PaletteLocal() *host.Host { return a.local() }

func (a *App) PaletteSelectSession(id string) { a.selectSession(id) }

func (a *App) HostNamed(name string) *host.Host { return a.hostNamed(name) }

func (a *App) SessionsOf(h *host.Host) []proto.Session { return a.sessionsOn(h) }

func (a *App) ProjectsOf(h *host.Host) []proto.Project { return a.projectsOn(h) }

func (a *App) Connect(h *host.Host) { a.connectHost(h) }

func (a *App) MergeState(h *host.Host, st proto.State) { a.mergeState(h, st) }

func (a *App) Reselect() { a.reselect() }

func (a *App) UpdateTitle() { a.updateTitle() }

func (a *App) NewSessionOn(h *host.Host, projectID string) { a.newSessionOn(h, projectID) }

func (a *App) HostState(c *ui.Context, pal *theme.Palette, h *host.Host) { a.hostStateNow(c, pal, h) }

func (a *App) HostDraft() *draft.HostDraft { return &a.Ss.Remote }

func (a *App) PairCodeDraft() *draft.TCHostDraft { return &a.Ss.TCHost }

func (a *App) P2PDraft() *draft.TailcatDraft { return &a.Ss.Tailcat }

func (a *App) TCPListen(c *ui.Context, pal *theme.Palette, d *draft.TailcatDraft) {
	a.tcpListen(c, pal, d)
}

// syncRemoteHosts follows the machines the app reaches.
func (a *App) syncRemoteHosts() { hosts.Sync(a) }

func (a *App) p2pGroup(c *ui.Context, pal *theme.Palette) { hosts.P2PGroup(a, c, pal) }

func (a *App) tailcatHosts(c *ui.Context, pal *theme.Palette) { hosts.PairCodeHosts(a, c, pal) }

func (a *App) remoteHosts(c *ui.Context, pal *theme.Palette) { hosts.Remotes(a, c, pal) }

func (a *App) RemoteHostHeader(c *ui.Context, pal *theme.Palette, h *host.Host, top float32) {
	hosts.HostHeader(a, c, pal, h, top)
}

func (a *App) reconnectHost(h *host.Host) { hosts.Reconnect(a, h) }
