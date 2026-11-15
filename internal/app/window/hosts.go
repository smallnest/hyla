package window

import (
	"strings"

	"hyla/internal/app/host"
	"hyla/internal/app/link"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/hostlink"
	"hyla/internal/ui/sidebar"
)

func (a *App) local() *host.Host { return a.Machines[0] }

func (a *App) hostOf(id string) *host.Host {
	if h := a.Owner[id]; h != nil {
		return h
	}
	return a.local()
}

// hostNamed returns a remote host, by name, else nil.
func (a *App) hostNamed(name string) *host.Host {
	for _, h := range a.Machines[1:] {
		if strings.EqualFold(h.Name, name) {
			return h
		}
	}
	return nil
}

// selectedHost is the host of the session shown: this machine.
func (a *App) selectedHost() *host.Host { return a.local() }

// sessionsOn returns a host's sessions, in the state the app shows.
func (a *App) sessionsOn(h *host.Host) []proto.Session {
	var out []proto.Session
	for _, s := range a.St.Sessions {
		if a.hostOf(s.ID) == h {
			out = append(out, s)
		}
	}
	return out
}

// projectsOn returns a host's projects, in the state the app shows.
func (a *App) projectsOn(h *host.Host) []proto.Project {
	var out []proto.Project
	for _, p := range a.St.Projects {
		if a.hostOf(p.ID) == h {
			out = append(out, p)
		}
	}
	return out
}

// remoteControl reports whether programs may drive the app (hyla focus,
// hyla do), as Settings › Agents says, or a Feishu bot, which takes
// prompts as the agent runs them.
func (a *App) remoteControl(h *host.Host) bool {
	if h.Remote() {
		// A machine's programs drive the app of the machine they run on.
		return false
	}
	if settings.Load().Feishu != nil && settings.Load().Feishu.App != "" {
		return true
	}
	return a.settingsIn.RemoteControl
}

// hostedName is a session's name, as a notification and the window's
// title say it.
func (a *App) hostedName(s *proto.Session) string {
	name := s.DisplayName(a.settingsIn.SessionName)
	if h := a.hostOf(s.ID); h.Remote() {
		return name + " · " + h.Name
	}
	return name
}

// allLoaded reports whether the state came, or failed to: the session the
// app showed last may be of a hylad still connecting.
func (a *App) allLoaded() bool {
	for _, h := range a.Machines {
		if !h.Loaded && h.Conn == host.StateConnecting {
			return false
		}
	}
	return true
}

// actionHost is the host an action runs for: this machine.
func (a *App) actionHost() *host.Host { return a.local() }

// reselect shows another session when the one shown is gone: the first,
// once the state came or failed to.
func (a *App) reselect() {
	if a.session(a.Selected) != nil {
		return
	}
	ids := sidebar.VisibleOrder(a.order())
	switch {
	case len(ids) == 0 && a.allLoaded():
		a.Selected = ""
	case len(ids) > 0 && (a.Selected == "" || a.allLoaded()):
		a.selectSession(ids[0])
	}
}

// mergeState makes st a host's part of the state the app shows.
func (a *App) mergeState(h *host.Host, st proto.State) {
	a.St.Projects, a.St.Sessions, a.Owner = hostlink.Merge(a, h, st)
}

// own records a session the app just made as a host's.
func (a *App) own(h *host.Host, s proto.Session) { hostlink.Own(a, h, s) }

// dropTerms lets go of a host's terminals, which attach again as they
// show.
func (a *App) dropTerms(h *host.Host) {
	for id := range a.Terms {
		if a.hostOf(id) == h {
			a.dropTerm(id)
		}
	}
}

// forgetStatuses forgets the statuses the app last saw of a host's
// sessions.
func (a *App) forgetStatuses(h *host.Host) {
	for _, s := range a.sessionsOn(h) {
		delete(a.Statuses, s.ID)
	}
}

func (a *App) connect() { link.Connect(a) }
