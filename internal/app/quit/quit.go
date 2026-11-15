// Package quit decides what a quit does: it ends this machine's sessions
// when the user chose that, asking first while a program runs in one, and
// lets the app go when it saves nothing the user could lose.
package quit

import (
	"time"

	"github.com/egoist/mygo"

	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/locales"
)

// Grace is how long a quit may take before the app leaves without it: a
// window that will not close, or a loop that will not stop, must not keep
// the user waiting on it.
const Grace = 2 * time.Second

// App is what a quit asks of the app.
type App interface {
	// SavePrefs keeps the prefs, and SettingsNow are the settings.
	SavePrefs()
	SettingsNow() *settings.Settings
	// Asking tells that the quit asks, and AskingNow whether it does: a
	// quit taken back before, or the one its answer makes, asks no more.
	Asking(on bool)
	AskingNow() bool
	// Confirm asks the user, and Local is this machine's host: its
	// sessions are the ones a quit ends.
	ConfirmAsk(cf *confirm.Ask)
	Local() *host.Host
	// Hosts are the hosts the app holds, and Seen whether a host has
	// shown its sessions.
	Hosts() []*host.Host
	Seen(h *host.Host) bool
	// Busy counts a host's sessions whose programs a restart of its hylad
	// would end.
	Busy(h *host.Host) int
	// BringToFront shows the window, and Leaving tells that the app goes,
	// which closes the quick terminal's window rather than hiding it.
	BringToFront()
	Leaving()
	// ArmQuitTimer leaves the app after Grace, whatever holds the quit.
	ArmQuitTimer()
	// EndSessionsAndQuit ends this machine's sessions, its hylad, and the
	// app.
	EndSessionsAndQuit()
	// SessionsOn are a host's sessions, as EndSessions closes each.
	SessionsOn(h *host.Host) []proto.Session
}

// Requested answers a quit: true lets it go, false takes it back. It ends
// this machine's sessions when the user chose that, asking first while a
// program runs in one.
func Requested(a App) bool {
	a.SavePrefs()
	if !a.SettingsNow().QuitEndsSessions || a.AskingNow() {
		// A quit taken back before, or the one its answer makes: either
		// way it is over.
		a.Asking(false)
		return allow(a)
	}
	busy := a.Busy(a.Local())
	switch {
	case busy > 0 && a.SettingsNow().ConfirmClose:
		a.Asking(true)
		a.ConfirmAsk(&confirm.Ask{
			Open:    true,
			Title:   locales.ConfirmQuitAllTitle(),
			Detail:  Detail(busy, len(a.Hosts()) > 1),
			Action:  locales.ConfirmQuitAllAction(),
			Destroy: true,
			Ends:    true, // Return may be a program's to read
			Run:     a.EndSessionsAndQuit,
		})
		return false
	case busy == 0 && loaded(a):
		// Nothing runs that the user could lose: the quit ends the
		// sessions itself, as it goes.
		return allow(a)
	case !loaded(a):
		// The sessions have not come yet, and with them what runs: take
		// the quit back while the app shows them, and ask again when the
		// user quits after.
		a.Asking(true)
		a.BringToFront()
		return false
	default:
		// Nothing runs, and the sessions are not all in sight, as when
		// the app quits from the Dock while hidden: let it end them.
		return allow(a)
	}
}

// Detail says what a quit ends: this machine's sessions, as other
// machines' keep running.
func Detail(n int, hosts bool) string {
	d := locales.ConfirmQuitAllDetail(n)
	if hosts {
		d += " " + locales.ConfirmQuitAllHosts()
	}
	return d
}

// allow lets a quit go, and makes sure it goes: the quick terminal's
// window closes with it rather than hiding itself, and if the app is
// still here after Grace it exits anyway.
func allow(a App) bool {
	a.Leaving()
	a.ArmQuitTimer()
	return true
}

// loaded reports whether every host has shown its sessions, as the app
// asks before it ends this machine's.
func loaded(a App) bool {
	for _, h := range a.Hosts() {
		if h.Conn == host.StateConnecting || (h.Client != nil && !a.Seen(h)) {
			return false
		}
	}
	return true
}

// EndsSession ends this machine's sessions without hylad to ask, as when
// the app quits before it connects: the quit-ends-sessions the user chose
// then takes the sessions' state, so that none starts again.
func EndsSession(a App) {
	h := a.Local()
	var cl *client.Client
	for _, s := range a.SessionsOn(h) {
		if cl == nil {
			var err error
			if cl, err = client.Connect(true); err != nil {
				return
			}
		}
		cl.Call("close", proto.IDParams{ID: s.ID}, nil)
	}
	if cl != nil {
		cl.Close()
	}
}

// QuitNow quits: the daemon may take its time ending the sessions, so the
// app leaves within Grace whatever it answers.
func QuitNow() { mygo.App.Quit() }
