// Package link holds the app's connections to the machines it shows: how
// it reaches each machine's hylad, follows it as it goes and comes, and
// what it makes of its events.
package link

import (
	"errors"
	"slices"
	"time"

	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/proto"
)

// App is what a connection asks of the app it serves.
type App interface {
	// Machines are the hosts the app holds, this machine's first, and
	// Update runs fn on the main thread, and draws.
	MachinesNow() []*host.Host
	UpdateNow(fn func())
	// HostOf is the machine a session, a pane or a project is of, and
	// Local this machine.
	HostOf(id string) *host.Host
	Local() *host.Host
	// ReportView tells each hylad which of its sessions the user looks
	// at.
	ReportViewNow()
	// TakeState takes a host's state, Reselect shows another session once
	// the one shown is gone, and SelectSession shows one.
	TakeStateOf(h *host.Host, st proto.State)
	ReselectNow()
	SelectSession(id string)
	// DropTerms lets go of a host's terminals, and StatusesForget forgets
	// the statuses of its sessions.
	DropHostTerms(h *host.Host)
	StatusesForget(h *host.Host)
	InvalidateHosts()
	// Event takes a message from a host's hylad.
	EventOf(h *host.Host, cl *client.Client, m proto.Message)
}

// Connect connects to this machine's hylad, starting it.
func Connect(a App) { ConnectHost(a, a.Local()) }

// ConnectHost connects to hylad: this machine's, or another machine's,
// over its tcp:// listener.
func ConnectHost(a App, h *host.Host) {
	h.Gen++
	gen := h.Gen
	if h.Conn != host.StateReady {
		h.Conn = host.StateConnecting
	}
	h.Dialing = true
	a.InvalidateHosts()
	go func() {
		var cl *client.Client
		var err error
		if h.Remote() {
			cl, err = client.ConnectVia(h.Dialer())
		} else {
			cl, err = connectLocal()
		}
		a.UpdateNow(func() { Connected(a, h, gen, cl, err) })
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

// connected takes the result of ConnectHost.
func Connected(a App, h *host.Host, gen int, cl *client.Client, err error) {
	if h.Gen != gen || h.Conn == host.StateDisconnected || !slices.Contains(a.MachinesNow(), h) {
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
			a.InvalidateHosts()
			return
		}
		h.Conn, h.Err = host.StateFailed, err.Error()
		h.Retries++
		retryLater(a, h)
		// The session shown last may be of the hylad that failed.
		if !h.Loaded {
			a.ReselectNow()
		}
		a.InvalidateHosts()
		return
	}
	h.Client, h.Conn, h.Err, h.Retries = cl, host.StateReady, "", 0
	h.Hylad = host.ID(cl)
	h.DaemonApp, h.DaemonFP = cl.Welcome.App, cl.Welcome.Build
	if !h.Remote() {
		host.RelaunchFor(h.DaemonApp)
	}
	cl.OnEvent(func(m proto.Message) { a.UpdateNow(func() { a.EventOf(h, cl, m) }) })
	cl.Go("subscribe", nil, nil)
	h.ViewSent = nil
	a.ReportViewNow()
	a.InvalidateHosts()
	go func() {
		<-cl.Done()
		a.UpdateNow(func() { disconnected(a, h, cl) })
	}()
}

// retryLater connects again after a failure, a few times, as hylad may be
// starting.
func retryLater(a App, h *host.Host) {
	gen := h.Gen
	if h.Retries >= 4 {
		return
	}
	wait := time.Duration(h.Retries) * 400 * time.Millisecond
	time.AfterFunc(wait, func() {
		a.UpdateNow(func() {
			if h.Gen == gen && h.Conn == host.StateFailed && slices.Contains(a.MachinesNow(), h) {
				ConnectHost(a, h)
			}
		})
	})
}

// disconnected follows hylad going away, as when it was restarted: the
// terminals attached to it are done, and the app connects again.
func disconnected(a App, h *host.Host, cl *client.Client) {
	if h.Client != cl {
		return
	}
	h.Gen++
	gen := h.Gen
	h.Client = nil
	h.Conn = host.StateConnecting
	a.DropHostTerms(h)
	// What changed while the app was away tells nothing new.
	a.StatusesForget(h)
	a.InvalidateHosts()
	time.AfterFunc(300*time.Millisecond, func() { a.UpdateNow(func() { reconnect(a, h, gen) }) })
}

// reconnect connects again, unless it is.
func reconnect(a App, h *host.Host, gen int) {
	if h.Gen == gen && h.Conn != host.StateDisconnected && slices.Contains(a.MachinesNow(), h) && h.Client == nil && !h.Dialing {
		ConnectHost(a, h)
	}
}
