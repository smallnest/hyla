package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/shell"
	"hyla/internal/update"

	"hyla/locales"
)

// host is this machine's hylad, as the app has it: its connection, and
// what it holds.
type host struct {
	name string // "" for this machine

	cl        *client.Client
	conn      connState
	err       string // why the last connection failed
	daemonApp string // the version of Hyla hylad runs
	daemonFP  string // what its code is made of (proto.Welcome.Build)
	info      proto.HostInfo
	retries   int
	gen       int  // connection attempts: the results of one another followed are dropped
	dialing   bool // a connection attempt is under way
	hylad     string
	loaded    bool // a state came
	// convs are the conversations hylad remembers, for the sidebar, as of
	// its count of their changes convGen; convAsked tells that they're
	// asked for.
	convs     []proto.Conversation
	convGen   uint64
	convKnown bool
	convAsked bool
	viewSent  *string
}

// relaunchedEnv is the environment a relaunched app finds, which says it
// went into that Hyla already: a loop of relaunches stops there.
const relaunchedEnv = "HYLA_RELAUNCHED"

// relaunchForHylad relaunches the app into the Hyla hylad runs, when it
// is newer, which a remote machine's update leaves it in.
func relaunchForHylad(v string) {
	app := mygo.App.Version()
	if !released(v) || !released(app) || update.Compare(v, app) <= 0 ||
		!mygo.Updater.Enabled() || os.Getenv(relaunchedEnv) == v {
		return
	}
	os.Setenv(relaunchedEnv, v)
	mygo.App.Relaunch()
}

// released tells a version of Hyla that has a release to install: not a
// development build's.
func released(v string) bool { return v != "" && !strings.Contains(v, "dev") && !mygo.IsDev() }

func (a *App) local() *host { return a.hosts[0] }

// hostOf returns the host of a session, a pane or a project, by id: this
// machine, which is the only one the app has.
func (a *App) hostOf(id string) *host { return a.local() }

// label names the host in words: this machine, which has no name of its
// own.
func (h *host) label() string {
	if h.name == "" {
		return "local"
	}
	return h.name
}

// shortDir is a directory as the sidebar and the tooltips show it, from
// this machine's home.
func (h *host) shortDir(dir string) string { return proto.ShortDir(dir) }

// joinPath makes a path of this machine, whose separator is its own.
func (h *host) joinPath(parts ...string) string { return filepath.Join(parts...) }

// windows reports whether this machine is one of Windows: a path is
// absolute there only with a drive, and its programs are PowerShell's.
func (h *host) windows() bool { return runtime.GOOS == "windows" }

// editorCommand edits a file in a pane, with the editor this machine's
// shell names, $VISUAL or $EDITOR, else vi, or on Windows edit.
func editorCommand(h *host, file string) []string {
	if !h.windows() {
		return []string{"sh", "-c", `exec ${VISUAL:-${EDITOR:-vi}} "$1"`, "sh", file}
	}
	return []string{"powershell.exe", "-NoProfile", "-Command",
		`$e = if ($env:VISUAL) { $env:VISUAL } elseif ($env:EDITOR) { $env:EDITOR } else { 'edit' }; Invoke-Expression ($e + ' ' + ` + shell.PSQuote(shell.PSQuote(file)) + `)`}
}

// dialer opens connections to this machine's hylad.
func (h *host) dialer() client.Dialer { return client.Local{NoStart: true} }

// selectedHost is the host of the session shown: this machine.
func (a *App) selectedHost() *host { return a.local() }

// connect connects to this machine's hylad, starting it.
func (a *App) connect() { a.connectHost(a.local()) }

// connectHost connects to this machine's hylad.
func (a *App) connectHost(h *host) {
	h.gen++
	gen := h.gen
	if h.conn != connReady {
		h.conn = connConnecting
	}
	h.dialing = true
	go func() {
		cl, err := connectLocal()
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
func (a *App) connected(h *host, gen int, cl *client.Client, err error) {
	if h.gen != gen || !slices.Contains(a.hosts, h) {
		if cl != nil {
			go cl.Close()
		}
		return
	}
	h.dialing = false
	if err != nil {
		h.cl = nil
		if errors.Is(err, client.ErrVersion) {
			h.conn, h.err = connVersion, ""
			return
		}
		h.conn, h.err = connFailed, err.Error()
		h.retries++
		a.retryLater(h)
		// The session shown last may be of the hylad that failed.
		if !h.loaded {
			a.reselect()
		}
		return
	}
	h.cl, h.conn, h.err, h.retries = cl, connReady, "", 0
	h.hylad = hyladID(h, cl)
	h.daemonApp, h.daemonFP = cl.Welcome.App, cl.Welcome.Build
	relaunchForHylad(h.daemonApp)
	cl.OnEvent(func(m proto.Message) { a.update(func() { a.event(h, cl, m) }) })
	cl.Go("subscribe", nil, nil)
	h.viewSent = nil
	a.reportView()
	go func() {
		<-cl.Done()
		a.update(func() { a.disconnected(h, cl) })
	}()
}

// retryLater connects again after a failure, a few times, as hylad may be
// starting.
func (a *App) retryLater(h *host) {
	gen := h.gen
	if h.retries >= 4 {
		return
	}
	wait := time.Duration(h.retries) * 400 * time.Millisecond
	time.AfterFunc(wait, func() {
		a.update(func() {
			if h.gen == gen && h.conn == connFailed && slices.Contains(a.hosts, h) {
				a.connectHost(h)
			}
		})
	})
}

// disconnected follows hylad going away, as when it was restarted: the
// terminals attached to it are done, and the app connects again.
func (a *App) disconnected(h *host, cl *client.Client) {
	if h.cl != cl {
		return
	}
	h.cl = nil
	h.conn = connConnecting
	a.dropTerms(h)
	// What changed while the app was away tells nothing new.
	for _, s := range a.sessionsOf(h) {
		delete(a.statuses, s.ID)
	}
	time.AfterFunc(300*time.Millisecond, func() { a.update(func() { a.reconnect(h) }) })
}

// reconnect connects again, unless it is.
func (a *App) reconnect(h *host) {
	if slices.Contains(a.hosts, h) && h.cl == nil && !h.dialing {
		a.connectHost(h)
	}
}

// dropTerms lets go of the terminals, which attach again as they show.
func (a *App) dropTerms(h *host) {
	for id := range a.terms {
		a.dropTerm(id)
	}
}

// sessionsOf returns the sessions, in the state the app shows.
func (a *App) sessionsOf(h *host) []proto.Session { return a.st.Sessions }

func (a *App) projectsOf(h *host) []proto.Project { return a.st.Projects }

// mergeState makes st the state the app shows.
func (a *App) mergeState(h *host, st proto.State) {
	a.st.Projects, a.st.Sessions = st.Projects, st.Sessions
}

// remoteControl reports whether programs may drive the app (hyla focus,
// hyla do), as Settings › Agents says, or a Feishu bot, which takes
// prompts as the agent runs them.
func (a *App) remoteControl(h *host) bool {
	if settings.Load().Feishu != nil && settings.Load().Feishu.App != "" {
		return true
	}
	return a.set.RemoteControl
}

// hostedName is a session's name, as a notification and the window's
// title say it.
func (a *App) hostedName(s *proto.Session) string {
	return s.DisplayName(a.set.SessionName)
}

// allLoaded reports whether the state came, or failed to: the session the
// app showed last may be of a hylad still connecting.
func (a *App) allLoaded() bool {
	return a.local().loaded || a.local().conn != connConnecting
}

// actionHost is the host an action runs for: this machine.
func (a *App) actionHost() *host { return a.local() }

// hyladID tells a hylad apart from any other: its machine, its pid and
// when it started.
func hyladID(h *host, cl *client.Client) string {
	name, _ := os.Hostname()
	return fmt.Sprintf("%s/%d/%d", name, cl.Welcome.PID, cl.Welcome.Started)
}

// reselect shows another session when the one shown is gone: the first,
// once the state came or failed to.
func (a *App) reselect() {
	if a.session(a.selected) != nil {
		return
	}
	ids := a.visibleOrder()
	switch {
	case len(ids) == 0 && a.allLoaded():
		a.selected = ""
	case len(ids) > 0 && (a.selected == "" || a.allLoaded()):
		a.selectSession(ids[0])
	}
}

// hostDot says how the app stands with this machine's hylad: a dot, and
// its words as the tooltip.
func (a *App) hostDot(c *ui.Context, pal *palette, col ui.Color, label string) {
	ui.Box(c).Size(7, 7).Radius(3.5).Background(col).Shrink(0).Margin(0, 4, 0, 0).Label(label).Tooltip(label)
}

// hostState says how the app stands with this machine's hylad: a dot
// while it connects or failed to, a chip when it wants restarting.
func (a *App) hostState(c *ui.Context, pal *palette, h *host) {
	switch {
	case h.conn == connConnecting && h.loaded:
		a.hostDot(c, pal, pal.attention, locales.HostReconnecting())
	case h.conn == connConnecting:
		a.hostDot(c, pal, pal.textFaint, locales.StatusConnecting())
	case h.conn == connFailed:
		a.hostDot(c, pal, pal.danger, locales.HostCantConnect(h.name))
	case h.conn == connVersion || h.stale():
		b := ui.ButtonBase(c.Key("restart")).Height(22).Padding(0, 7).Gap(5).Radius(11).AlignItems(ui.Center).
			Label(locales.CommonRestart()).Tooltip(locales.HostStaleTip(h.name, h.daemonApp, mygo.App.Version()))
		if b.Hovered() {
			b.Background(pal.rowHover)
		}
		b.Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(pal.attention)
			ui.Text(c, locales.CommonRestart()).FontSize(12).FontWeight(500).TextColor(pal.attention).SingleLine()
		})
		if b.Clicked() {
			a.offerHostRestart(h)
		}
	}
}

// offerHostRestart restarts hylad, asking first while programs run in its
// sessions.
func (a *App) offerHostRestart(h *host) {
	a.confirm = &confirmation{
		open:    true,
		title:   locales.HostRestartTitle(h.label()),
		detail:  locales.HostRestartDetail(h.label()),
		action:  locales.CommonRestart(),
		destroy: true,
		run:     func() { a.restartDaemon() },
	}
	if a.win != nil {
		a.win.Focus()
	}
}

// stale reports whether hylad runs an older Hyla than the app: what this
// machine's hylad is, before it is restarted into the app's.
func (h *host) stale() bool {
	if h.conn != connReady || h.daemonApp == "" {
		return false
	}
	return hyladStale(h.daemonApp, h.daemonFP)
}
