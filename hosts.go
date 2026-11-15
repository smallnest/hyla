package main

import (
	"errors"
	"hyla/internal/app/host"
	"slices"
	"strings"
	"time"

	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/hostlink"
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

// connect connects to this machine's hylad, starting it.
func (a *App) connect() { a.connectHost(a.local()) }

// connectHost connects to hylad: this machine's, or another machine's,
// over its tcp:// listener.
func (a *App) connectHost(h *host.Host) {
	h.Gen++
	gen := h.Gen
	if h.Conn != host.StateReady {
		h.Conn = host.StateConnecting
	}
	h.Dialing = true
	go func() {
		var cl *client.Client
		var err error
		if h.Remote() {
			cl, err = client.ConnectVia(h.Dialer())
		} else {
			cl, err = connectLocal()
		}
		a.update(func() { a.connected(h, gen, cl, err) })
	}()
}

// connectLocal connects to this machine's hylad, as client.Connect: one of
// an earlier Hyla, which runs too, is first stopped, as restarting hylad
// stops it, so that the two do not stop each other. The app stops it
// without asking: another Hyla's hylad answers this app's clients none,
// and its sessions would not show.
func connectLocal() (*client.Client, error) {
	for i := 0; i < 3; i++ {
		cl, err := client.Connect(false)
		if err != nil || !client.Settle(cl.Welcome) {
			return cl, err
		}
		// The next hylad starts as this connection goes.
		pid := cl.Welcome.PID
		cl.Close()
		client.SettleStop(pid)
	}
	return client.Connect(false)
}

// connected takes the result of connectHost.
func (a *App) connected(h *host.Host, gen int, cl *client.Client, err error) {
	if h.Gen != gen || !slices.Contains(a.Machines, h) {
		if cl != nil {
			go cl.Close()
		}
		return
	}
	h.Dialing = false
	if err != nil {
		h.Client = nil
		if errors.Is(err, client.ErrVersion) {
			h.Conn, h.Err = host.StateVersion, ""
			return
		}
		h.Conn, h.Err = host.StateFailed, err.Error()
		h.Retries++
		a.retryLater(h)
		// The session shown last may be of the hylad that failed.
		if !h.Loaded {
			a.reselect()
		}
		return
	}
	h.Client, h.Conn, h.Err, h.Retries = cl, host.StateReady, "", 0
	h.Hylad = host.ID(cl)
	h.DaemonApp, h.DaemonFP = cl.Welcome.App, cl.Welcome.Build
	if !h.Remote() {
		host.RelaunchFor(h.DaemonApp)
	}
	cl.OnEvent(func(m proto.Message) { a.update(func() { a.event(h, cl, m) }) })
	cl.Go("subscribe", nil, nil)
	h.ViewSent = nil
	a.reportView()
	go func() {
		<-cl.Done()
		a.update(func() { a.disconnected(h, cl) })
	}()
}

// retryLater connects again after a failure, a few times, as hylad may be
// starting.
func (a *App) retryLater(h *host.Host) {
	gen := h.Gen
	if h.Retries >= 4 {
		return
	}
	wait := time.Duration(h.Retries) * 400 * time.Millisecond
	time.AfterFunc(wait, func() {
		a.update(func() {
			if h.Gen == gen && h.Conn == host.StateFailed && slices.Contains(a.Machines, h) {
				a.connectHost(h)
			}
		})
	})
}

// disconnected follows hylad going away, as when it was restarted: the
// terminals attached to it are done, and the app connects again.
func (a *App) disconnected(h *host.Host, cl *client.Client) {
	if h.Client != cl {
		return
	}
	h.Client = nil
	h.Conn = host.StateConnecting
	a.dropTerms(h)
	// What changed while the app was away tells nothing new.
	for _, s := range a.sessionsOn(h) {
		delete(a.Statuses, s.ID)
	}
	time.AfterFunc(300*time.Millisecond, func() { a.update(func() { a.reconnect(h) }) })
}

// reconnect connects again, unless it is.
func (a *App) reconnect(h *host.Host) {
	if slices.Contains(a.Machines, h) && h.Client == nil && !h.Dialing {
		a.connectHost(h)
	}
}

// dropTerms lets go of a host's terminals, which attach again as they
// show.
func (a *App) dropTerms(h *host.Host) {
	for id := range a.Terms {
		if a.hostOf(id) == h {
			a.dropTerm(id)
		}
	}
}

// mergeState makes st a host's part of the state the app shows.
func (a *App) mergeState(h *host.Host, st proto.State) {
	a.St.Projects, a.St.Sessions, a.Owner = hostlink.Merge(a, h, st)
}

// own records a session the app just made as a host's.
func (a *App) own(h *host.Host, s proto.Session) { hostlink.Own(a, h, s) }

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
	ids := a.visibleOrder()
	switch {
	case len(ids) == 0 && a.allLoaded():
		a.Selected = ""
	case len(ids) > 0 && (a.Selected == "" || a.allLoaded()):
		a.selectSession(ids[0])
	}
}
