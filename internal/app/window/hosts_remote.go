package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/app/link"
	"hyla/internal/cli"
	"hyla/internal/proto"
	"hyla/internal/ui/hostlink"
	"hyla/internal/ui/hosts"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/theme"
)

// The app, as the machines' pages and the sidebar ask it. The names
// Local and SelectSession are the palette's already: PaletteLocal and
// PaletteSelectSession.
var _ hosts.App = (*App)(nil)

// RemoteHosts are the other machines the app reaches.
func (a *App) RemoteHosts() []hosts.Remote { return a.Prefs.RemoteHosts }

func (a *App) SetRemoteHosts(rs []hosts.Remote) { a.Prefs.RemoteHosts = rs }

func (a *App) SetMachines(ms []*host.Host) { a.Machines = ms }

// PaletteLocal and PaletteSelectSession are the palette's names, which
// this file does not take.
func (a *App) PaletteLocal() *host.Host { return a.Local() }

func (a *App) PaletteSelectSession(id string) { a.selectSession(id) }

func (a *App) HostNamed(name string) *host.Host { return a.hostNamed(name) }

func (a *App) SessionsOf(h *host.Host) []proto.Session { return a.sessionsOn(h) }

func (a *App) ProjectsOf(h *host.Host) []proto.Project { return a.projectsOn(h) }

func (a *App) Connect(h *host.Host) { link.ConnectHost(a, h) }

func (a *App) MergeState(h *host.Host, st proto.State) {
	a.St.Projects, a.St.Sessions, a.Owner = hostlink.Merge(a, h, st)
}

// StopDaemon stops this machine's hylad.
func (a *App) StopDaemon() error { return cli.StopDaemon() }

func (a *App) HostDraft() *draft.HostDraft { return &a.Ss.Remote }

func (a *App) PairCodeDraft() *draft.TCHostDraft { return &a.Ss.TCHost }

func (a *App) TailcatDraft() *draft.TailcatDraft { return &a.Ss.Tailcat }

func (a *App) P2PDraft() *draft.P2PDraft { return &a.Ss.P2P }

func (a *App) TCPListen(c *ui.Context, pal *theme.Palette, d *draft.P2PDraft) {
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

// SettingsConfirm asks q in the Settings window, where the machines'
// pages are: the question shows over the page, not behind it.
func (a *App) SettingsConfirm(q *confirm.Ask) { a.Ss.Confirm = q }

// SettingsToast tells msg in the Settings window.
func (a *App) SettingsToast(msg string) { a.Ss.Toast = msg }
