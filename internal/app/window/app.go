package window

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hyla/internal/app/favicon"
	"hyla/internal/app/host"
	"hyla/internal/app/paneinfo"
	"hyla/internal/app/quit"
	"hyla/internal/platform"
	"hyla/internal/ui/layout"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/adblock"
	"hyla/internal/app/filepane"
	configsync "hyla/internal/app/sync"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/hosts"
	"hyla/internal/ui/k8sdialog"
	"hyla/internal/ui/marks"
	"hyla/internal/ui/palette"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/petme"
	"hyla/internal/ui/picker"
	"hyla/internal/ui/quick"
	uisettings "hyla/internal/ui/settings"
	"hyla/internal/ui/side"
	"hyla/internal/ui/sidebar"
	"hyla/internal/ui/sidebarview"
	"hyla/internal/ui/subagentsview"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/timelineview"
	"hyla/internal/ui/webpane"
	"hyla/internal/ui/worktree"

	"hyla/internal/ui/sshdialog"
	"hyla/locales"
)

// connState is how the app stands with hylad.
// prefs are what the app remembers between runs; the sessions themselves
// are hylad's.
type prefs struct {
	Selected      string          `json:"selected,omitempty"`
	SidebarWidth  float32         `json:"sidebarWidth,omitempty"`
	SidebarHidden bool            `json:"sidebarHidden,omitempty"`
	Collapsed     map[string]bool `json:"collapsed,omitempty"`
	// ProjectOrder is the order the user dragged projects into, by id,
	// across every host; unlisted projects follow in the daemon's order.
	ProjectOrder []string `json:"projectOrder,omitempty"`
	// SessionOrder is the order the user dragged sessions into within
	// their projects, by id, across every host; a session it does not
	// name follows the ones it does, in the sidebar's order.
	SessionOrder []string `json:"sessionOrder,omitempty"`
	// SidebarGroup is how the sidebar groups sessions: "" or "project",
	// "none" or "date"; SidebarOrder is "created" or "updated".
	SidebarGroup string  `json:"sidebarGroup,omitempty"`
	SidebarOrder string  `json:"sidebarOrder,omitempty"`
	FontSize     float32 `json:"fontSize,omitempty"`
	// Side is the view the side panel shows, "" while it is hidden, and
	// SideTab the view it shows again; SideWidth is its width, and
	// RightWidth the viewer's.
	Side       string  `json:"side,omitempty"`
	SideTab    string  `json:"sideTab,omitempty"`
	RightWidth float32 `json:"rightWidth,omitempty"`
	SideWidth  float32 `json:"sideWidth,omitempty"`
	// FileOpen is what a click on a file of the Files view does:
	// "preview" or "" shows it in the viewer, "pane" opens it in a file
	// pane, and "editor" in the host's editor.
	FileOpen string `json:"fileOpen,omitempty"`
	// InfoFolded are the Info view's cards the user folded, by their key.
	InfoFolded map[string]bool `json:"infoFolded,omitempty"`
	// EndedOpen and ArchivedOpen tell the sidebar's sections of the
	// agents' ended conversations unfolded (session-history).
	EndedOpen    bool `json:"endedOpen,omitempty"`
	ArchivedOpen bool `json:"archivedOpen,omitempty"`
	// QuickSession is the session the quick terminal shows (quick.go).
	QuickSession string `json:"quickSession,omitempty"`
	// RemoteHosts are the other machines Hyla shows the sessions of, each
	// by the address and the password of its tcp:// listener.
	RemoteHosts []hosts.Remote `json:"remoteHosts,omitempty"`
	// SSHProject is the software project the sidebar's SSH terminals open
	// in, "" until one is made; SSHHosts are the machines it lists.
	SSHProject string    `json:"sshProject,omitempty"`
	SSHHosts   []sshHost `json:"sshHostList,omitempty"`
	// MovedSessions are the sessions the user moved among the ones of a
	// project they moved the session to, as sidebar.Prefs.SessionProjects
	// keeps them: the side of the session there before this move.
	MovedSessions map[string]string `json:"movedSessions,omitempty"`
	// K8sProject is the software project the sidebar's kubectl terminals
	// open in, "" until one is made; K8sPods are the pods it lists.
	K8sProject string   `json:"k8sProject,omitempty"`
	K8sPods    []k8sPod `json:"k8sPodList,omitempty"`
}

const (
	defaultSidebar = 268
	minSidebar     = 200
	maxSidebar     = 440
	defaultFont    = 13
	maxTerms       = 12 // terminals kept attached, most recently shown
)

// termEntry is a session's terminal in the app: attached to hylad, which
// streams the session to it.
type termEntry struct {
	term    *terminal.Terminal
	stream  *client.Stream
	err     string
	started time.Time
}

// App is the app's state. It lives on the main thread: other goroutines
// change it through Window.Update.
type App struct {
	// win is the main window, and winOpen whether it is open: closed, it
	// still takes the updates of Window.Update, which run without it.
	Win     *mygo.Window
	WinOpen bool
	Tray    *mygo.Tray // the icon in the menu bar or notification area, while it shows

	// hosts are the hylads the app shows, this machine's first; st is
	// what they hold, each host's sessions and projects after the one's
	// before, and owner the host of each session, pane and project.
	Machines []*host.Host // this machine and the others
	Owner    map[string]*host.Host
	St       proto.State
	Loaded   bool // a state came

	Prefs       prefs
	settingsIn  settings.Settings // the user's settings: Settings' controls edit it
	Applied     settings.Settings // set, as last applied and saved
	SetWin      *mygo.Window      // the Settings window, while open
	Ss          uisettings.State
	Selected    string
	Terms       map[string]*termEntry
	Webs        map[string]*webpane.Entry // browser panes' pages, by pane
	Files       filepane.Panes            // file panes' files, by pane
	Ads         adblock.State             // what their pages block
	IconsCache  favicon.Cache             // and the icons they show
	Lru         []string
	Refs        map[string]*ui.Handle // panes' terminals, and splits, by id
	Focus       map[string]string     // the pane the user is in, by session
	PaneChatMap map[string]bool       // the panes that show their session's chat, not its terminal
	SplitDrag   struct {              // the split whose divider is dragged
		session, split string
		ratio          float64 // where the pointer puts it, past its bounds
	}

	Active     bool
	focused    bool                    // whether the app is in front, as Focused answers
	Statuses   map[string]proto.Status // as last seen, for notifications
	Pet        petme.State             // Hyla the frog, on the window's top edge
	PetSetView *petme.View             // the Codex pet it shows where it is no frog
	PetList    petme.List              // the pets the settings may show, as last read
	petsGen    int                     // the pets installed or removed, which a list reads again
	// idleTime tells how long the user has not used the machine's keyboard
	// or mouse (mygo.Power.IdleTime), 0 when the system does not tell; nil
	// in tests.
	IdleTime func() time.Duration

	Renaming         string
	RenamingProject  string
	RenameBuf        string
	RenameFramesF    int // the frames a rename has shown, as the sidebar counts them
	configsync.State     // this machine's configuration, kept with the others'

	ChromeState layout.Chrome // where the sidebar's animation puts things
	Bar         ui.TitleBar   // the window's title bar, as the frame drew it

	Pal     palette.State
	Find    findState
	Confirm *confirm.Ask
	WtAsk   *worktree.Ask  // the new session's branch, while asked
	SshAsk  *sshdialog.Ask // the sidebar's add-a-host-over-SSH dialog
	K8sAsk  *k8sdialog.Ask // the sidebar's add-a-kubectl-exec-pod dialog
	// endedAll shows all of the sidebar's Ended; skillMissing tells, as
	// of skillsAt, that no kind of agent has Hyla's skill (ended.go).
	// This fork has no Ended (session history): the fields stay for
	// ended.go's view, which nothing shows.
	EndedAll     bool
	SkillMissing bool
	SkillsAt     time.Time
	SideDrag     float32 // the width as a drag of the sidebar's edge began
	PendingToast string
	// undoMessage and undoAction are the last removal's words, and what puts
	// back what it took away, as the toast's Undo does: a host, an SSH
	// machine or a pod.
	UndoMessage string
	UndoAction  func()
	// later are actions that change what the view is building, run once
	// it is built.
	Later []func()

	// Focus follows the mouse: the pane the pointer went into, and where
	// the pointer was last over a pane, in the window.
	HoverPaneID  string
	HoverAtPoint [2]float32
	// claims are the wheel's rests in each pane, of which the last tells
	// its host the user is there (claimWhenRest). claimMu guards them: the
	// wheel's callback and the frame that drops a pane both reach them, on
	// their own goroutines.
	claimMu sync.Mutex
	Claims  map[string]*claimWait

	// Panes dragged: the zone last under the pointer, the pane whose
	// drop preview shows and since when, and the panes just moved, which
	// spring in.
	LastDropZone panes.Zone
	PreviewPane  string
	// dragAt is the row a drag last hovered, and dragAfter the side of it
	// a drop lands on: a project's, a session's, a machine's or a pod's.
	DragAtAddr    string // the row an insertion line last showed
	DragAfterSide bool   // the side of it the line shows
	PreviewAt     time.Time
	// previewDrawn tells that the frame built a drop preview, which a
	// frame that builds none clears.
	PreviewDrawn bool
	ArrivedPanes map[string]bool
	CopyText     string
	Peek         peek       // the image a path Command hovers shows
	SidebarArt   sidebarArt // the picture behind the sidebar
	PaneArtimage sidebarArt // the picture in the terminals
	// shownSession is the session whose panes the last frame showed: one
	// shown in place of another comes in as sessionSwitch says.
	ShownSessionID string
	// switchIn tells that the frame about to build shows a session that
	// switchFrom posed, on its way in: the one after, posing its place.
	SwitchInState    bool
	SwitchFromMotion ui.Motion
	// switchAt is when a switch came in, which lands switchLand after.
	SwitchAtTime time.Time
	// from is the host whose program asked for the action being run (hyla
	// do), nil for the user's.
	From   *host.Host
	Picker picker.State // the folders of a host, to open a project in
	Side   side.State   // the side panel, beside a session's panes
	// info is what the Info views read, by pane (info.go).
	Info paneinfo.Panes
	// magpie is what Magpie shows of itself (magpie.go), nil until the
	// Info view shows.
	Magpie *magpieInfo
	// Timelines is what the Info view shows of the panes' timelines, by
	// pane (timeline_adapters.go).
	Timelines map[string]*timelineview.Info
	// Subagents is what the side panel's Subagents view shows, of the
	// session it shows (subagents.go), and what a subagents pane shows of
	// its session (subagents_pane.go).
	Subagents *subagentsview.Info

	// quitting is set while the quit quit-ends-sessions makes ends the
	// sessions, either while it asks or once its answer does.
	quitting bool
	// quittingApp tells that the app is leaving, which closes the quick
	// terminal's window rather than hiding it (quick.go); quitTimer
	// leaves even a quit that does not finish.
	quittingApp bool
	quitTimer   *time.Timer
	quick       quickTerm // the terminal the global shortcut drops down
}

// confirmation is a question the app asks before something it cannot undo.

func newApp() *App {
	a := &App{Terms: map[string]*termEntry{}, Webs: map[string]*webpane.Entry{}, Statuses: map[string]proto.Status{}, Active: true, focused: true,
		Refs: map[string]*ui.Handle{}, Focus: map[string]string{}, ArrivedPanes: map[string]bool{}}
	a.loadPrefs()
	// The logos the user gave the agents Hyla has none for, as the
	// sidebar draws them.
	marks.LoadUserMarks(marks.MarksDir())
	a.settingsIn = settings.Load()
	remote.SetRelay(a.settingsIn.TailcatRelay)
	applied.Lock()
	applied.s = a.settingsIn
	applied.Unlock()
	a.Ss.Problems = settings.Problems()
	a.Applied = a.settingsIn.Clone()
	a.Selected = a.Prefs.Selected
	a.Machines = []*host.Host{{Conn: host.StateConnecting}}
	return a
}

func prefsPath() string { return filepath.Join(proto.Dir(), "app.json") }

func (a *App) loadPrefs() {
	if b, err := os.ReadFile(prefsPath()); err == nil {
		json.Unmarshal(b, &a.Prefs)
	}
	if a.Prefs.SidebarWidth == 0 {
		a.Prefs.SidebarWidth = defaultSidebar
	}
	a.Prefs.SidebarWidth = min(max(a.Prefs.SidebarWidth, minSidebar), maxSidebar)
	if a.Prefs.FontSize == 0 {
		a.Prefs.FontSize = defaultFont
	}
	if a.Prefs.Collapsed == nil {
		a.Prefs.Collapsed = map[string]bool{}
	}
}

func (a *App) savePrefs() {
	a.Prefs.Selected = a.Selected
	b, err := json.MarshalIndent(a.Prefs, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(proto.Dir(), 0o700)
	tmp := prefsPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, prefsPath())
	}
}

// Update runs fn on the main thread, and draws.
func (a *App) Update(fn func()) {
	if a.Win != nil {
		a.Win.Update(fn)
	}
}

// UpdateNow runs fn on the main thread, and draws. It is Update, under
// the name the host link asks: its App names a session's Update too.
func (a *App) UpdateNow(fn func()) { a.Update(fn) }

// main runs fn on the main thread, and draws: a test's app, which has no
// window's loop, runs it as it comes.
func (a *App) main(fn func()) {
	if a.Win == nil {
		fn()
		return
	}
	a.UpdateNow(fn)
}

// EventOf takes a message from a host's hylad.
func (a *App) EventOf(h *host.Host, cl *client.Client, m proto.Message) {
	if h.Client != cl {
		return
	}
	switch m.Event {
	case "state":
		if m.State != nil {
			a.TakeStateOf(h, *m.State)
			// The chat windows read their turns as the host's state
			// changes: a chat's turn ends as the session goes idle.
			a.chatChanged(h)
		}
	case "select":
		// A host shows its own sessions, none of another machine.
		if !a.remoteControl(h) || a.hostOf(m.Select) != h {
			return
		}
		// In the pane hylad has it focus, as hyla focus <pane> asks: the
		// state with it came just before.
		if s := a.session(m.Select); s != nil && s.PaneByID(s.Focus) != nil {
			a.Focus[s.ID] = s.Focus
		}
		a.selectSession(m.Select)
		if a.Win != nil {
			a.ShowWindow()
		}
	case "action":
		if !a.remoteControl(h) {
			return
		}
		if act, ok := actions.Find(m.Action); ok && act.Remote {
			a.From = h
			a.run(act.ID, m.Args)
			a.From = nil
		}
	case "browser":
		// hyla browser waits on the answer, which refuses when Remote
		// Control is off, not as an action does, silently.
		if r := m.Browser; r != nil {
			a.browse(h, r, func(res *proto.BrowserResult, err error) {
				ans := proto.BrowserAnswer{Req: r.Req, Result: res}
				if err != nil {
					ans.Result, ans.Error = nil, err.Error()
				}
				cl.Go("browser.answer", ans, nil)
			})
		}
	case "notice":
		// A program's own notification, as a terminal shows it: not
		// while the user looks at its session.
		n := m.Notice
		s := a.session(n.Session)
		if s == nil || a.watching(n.Session) {
			return
		}
		title := n.Title
		if title == "" {
			title = s.DisplayName(a.settingsIn.SessionName)
		}
		a.showNotification(n.Session, title, n.Body)
	}
}

// TakeStateOf takes a host's state.
func (a *App) TakeStateOf(h *host.Host, st proto.State) {
	first, hostFirst := !a.Loaded, !h.Loaded
	h.Loaded = true
	// A divider dragged keeps the ratios it set, which the daemon hears of
	// only once it is let go.
	var dragged *proto.Layout
	if s := a.session(a.SplitDrag.session); s != nil {
		dragged = s.Layout
	}
	a.mergeState(h, st)
	a.Loaded = true
	if s := a.session(a.SplitDrag.session); s != nil && s.Layout.SameShape(dragged) {
		s.Layout = dragged
	}
	// Terminals of panes gone go too.
	live := map[string]bool{}
	for _, s := range a.St.Sessions {
		for _, p := range s.Panes {
			live[p.ID] = true
		}
	}
	for id := range a.Terms {
		if !live[id] {
			a.dropTerm(id)
		}
	}
	for id := range a.Webs {
		if !live[id] {
			a.DropWeb(id)
		}
	}
	a.Files.Forget(func(id string) bool { return live[id] })
	if a.settingsIn.SessionHistory {
		a.SyncEnded()
	} else {
		for _, h := range a.Machines {
			h.Convs, h.ConvKnown, h.ConvAsked = nil, false, false
		}
	}
	a.quickChanged()
	if first {
		// Hyla opens on the default project: its latest session, or one
		// started for it.
		a.openDefaultProject()
	}
	if a.session(a.Selected) == nil {
		a.ReselectNow()
	} else if first || hostFirst && a.hostOf(a.Selected) == h {
		a.selectSession(a.Selected)
	}
	a.notify(first)
	a.UpdateTitle()
}

// openDefaultProject shows the default project's latest session, and starts
// one in the home folder when it has none.
func (a *App) openDefaultProject() {
	for _, p := range a.St.Projects {
		if !p.Default {
			continue
		}
		var best *proto.Session
		for i := range a.St.Sessions {
			s := &a.St.Sessions[i]
			if s.Project != p.ID {
				continue
			}
			if best == nil || sidebar.SessionUpdated(s) > sidebar.SessionUpdated(best) {
				best = s
			}
		}
		if best != nil {
			a.selectSession(best.ID)
			return
		}
		a.SessionIn(proto.Home())
		return
	}
}

// showNotification shows a notification about a session, which a click
// on it selects.
func (a *App) showNotification(id, title, body string) {
	if !mygo.NotificationsSupported() {
		return
	}
	n := mygo.NewNotification(mygo.NotificationOptions{ID: "session/" + id, Group: id, Title: title, Body: body, Silent: !a.settingsIn.NotifySound})
	n.OnClick(func() {
		a.UpdateNow(func() {
			a.selectSession(id)
			a.ShowWindow()
		})
		n.Close()
	})
	go n.Show()
}

// notify tells about sessions that came to need the user, or finished,
// while the user was elsewhere: a notification, and the Dock's badge.
func (a *App) notify(first bool) {
	waiting := 0
	for i := range a.St.Sessions {
		s := &a.St.Sessions[i]
		prev, seen := a.Statuses[s.ID]
		a.Statuses[s.ID] = s.Status
		if s.Status == proto.Waiting {
			waiting++
		}
		if first || !seen || prev == s.Status || a.watching(s.ID) {
			continue
		}
		var title, body string
		switch {
		case s.Status == proto.Waiting && a.settingsIn.NotifyWaiting:
			title, body = locales.NotifyNeedsYouTitle(a.hostedName(s)), s.Message
			if body == "" {
				body = locales.NotifyWaitingBody()
			}
		case s.Status == proto.Failed && a.settingsIn.NotifyFailed:
			title, body = locales.NotifyFailedTitle(a.hostedName(s)), s.Message
			if body == "" {
				body = locales.NotifyFailedBody(s.PaneName())
			}
		case s.Status == proto.Idle && prev == proto.Working && s.Unread && a.settingsIn.NotifyDone && (s.Agent != "" || s.Message != ""):
			title, body = locales.NotifyDoneTitle(a.hostedName(s)), s.Message
			if body == "" {
				body = locales.NotifyDoneBody(s.Agent, a.hostOf(s.ID).ShortDir(s.WorkDir()))
			}
		default:
			continue
		}
		if a.settingsIn.BounceDock && (s.Status == proto.Waiting || s.Status == proto.Failed) {
			mygo.App.Dock.Bounce(false)
		}
		a.showNotification(s.ID, title, body)
	}
	for id := range a.Statuses {
		if a.session(id) == nil {
			delete(a.Statuses, id)
		}
	}
	if !a.settingsIn.DockBadge {
		waiting = 0
	}
	mygo.App.SetBadgeCount(waiting)
}

func (a *App) UpdateTitle() {
	if a.Win == nil {
		return
	}
	title := "Hyla"
	if s := a.session(a.Selected); s != nil {
		title = a.hostedName(s)
	}
	a.Win.SetTitle(title)
}

func (a *App) session(id string) *proto.Session {
	if id == "" {
		return nil
	}
	for i := range a.St.Sessions {
		if a.St.Sessions[i].ID == id {
			return &a.St.Sessions[i]
		}
	}
	return nil
}

func (a *App) project(id string) *proto.Project {
	for i := range a.St.Projects {
		if a.St.Projects[i].ID == id {
			return &a.St.Projects[i]
		}
	}
	return nil
}

func (a *App) selectSession(id string) {
	if a.session(id) == nil {
		return
	}
	changed := a.Selected != id
	a.Selected = id
	s := a.session(id)
	// The session shows: its project unfolds to it, chosen from the
	// palette or as the one that needs the user, with its row in sight.
	if s.Project != "" {
		delete(a.Prefs.Collapsed, s.Project)
	}
	for _, p := range s.Panes {
		a.EnsureTerm(p.ID)
	}
	if pane := a.focusOf(s); pane != "" {
		a.Touch(pane)
	}
	a.focusTerminal()
	a.ReportViewNow()
	a.UpdateTitle()
	if changed {
		a.savePrefs()
	}
}

// watching tells whether the user looks at the session: it's selected,
// and the main window has the keyboard. Only macOS says when the app
// stops being the active one, so the window's focus has to tell
// elsewhere.
func (a *App) watching(id string) bool {
	return id != "" && id == a.Selected && a.Active && a.focused
}

// ReportViewNow tells each host's hylad which of its sessions the user looks
// at, which it then never marks unread.
func (a *App) ReportViewNow() {
	shown := a.selectedHost()
	for _, h := range a.Machines {
		if h.Client == nil {
			continue
		}
		id := ""
		switch {
		case a.quick.Shown && h == a.Local():
			id = a.Prefs.QuickSession
		case a.watching(a.Selected) && h == shown:
			id = a.Selected
		}
		if h.ViewSent != nil && *h.ViewSent == id {
			continue
		}
		h.ViewSent = &id
		h.Client.Go("view", proto.ViewParams{ID: id}, nil)
	}
}

// Touch moves a terminal to the front of those kept attached, dropping
// the one shown longest ago past the limit.
func (a *App) Touch(id string) {
	a.Lru = slices.DeleteFunc(a.Lru, func(x string) bool { return x == id })
	a.Lru = append(a.Lru, id)
	for len(a.Lru) > maxTerms {
		a.dropTerm(a.Lru[0])
	}
}

func (a *App) dropTerm(id string) {
	a.claimMu.Lock()
	cw := a.Claims[id]
	delete(a.Claims, id)
	a.claimMu.Unlock()
	if cw != nil {
		cw.t.Stop()
	}
	if e := a.Terms[id]; e != nil {
		if e.term != nil {
			e.term.Close()
		}
		if e.stream != nil {
			e.stream.Close()
		}
		delete(a.Terms, id)
	}
	if a.Find.Pane == id {
		a.Find.Pane = ""
	}
	a.Lru = slices.DeleteFunc(a.Lru, func(x string) bool { return x == id })
}

// EnsureTerm attaches a terminal to a session, unless one is.
func (a *App) EnsureTerm(id string) {
	h := a.hostOf(id)
	if a.Terms[id] != nil || h.Conn != host.StateReady || a.Shows(id) {
		return
	}
	e := &termEntry{started: time.Now()}
	a.Terms[id] = e
	dialer := h.Dialer()
	go func() {
		stream, err := client.AttachVia(dialer, id)
		if err != nil {
			a.UpdateNow(func() {
				if a.Terms[id] == e {
					e.err = err.Error()
				}
			})
			return
		}
		g := &quick.Gated{Stream: stream, Ready: make(chan struct{})}
		o := a.TermOptions(id, g)
		// The terminal shows hylad's frames: hylad's terminal is the
		// session's, ours shows what it shows, cell for cell, and lays out
		// nothing of the program's own. It takes the sizes hylad's does,
		// between the frames of one size and the next.
		o.ResizeByConn = true
		term, err := terminal.New(o)
		if err != nil {
			stream.Close()
			a.UpdateNow(func() {
				if a.Terms[id] == e {
					e.err = err.Error()
				}
			})
			return
		}
		term.Resize(stream.Welcome.Cols, stream.Welcome.Rows)
		stream.OnResized = term.Resize
		close(g.Ready)
		a.UpdateNow(func() {
			if a.Terms[id] != e {
				term.Close()
				stream.Close()
				return
			}
			e.term, e.stream = term, stream
			// Shown, it takes the window's size (claimShown).
			if s := a.session(a.Selected); s != nil && a.here() && slices.Contains(shownPanes(s), id) {
				go stream.Claim()
			}
			if a.Selected == id {
				a.focusTerminal()
			}
			// The daemon closes the stream when the pane goes, or takes it
			// from another client: here the app attaches one again, rather
			// than leave the terminal at "[Disconnected]", taking no input.
			go a.watchTerm(id, e, termEnded(term, stream))
		})
	}()
}

// termEnded is a terminal's end, or its stream's: either says that the
// pane's stream, which the daemon closed, is over.
func termEnded(term *terminal.Terminal, stream *client.Stream) <-chan struct{} {
	ended := make(chan struct{})
	go func() {
		select {
		case <-term.Done():
		case <-stream.Gone:
		case <-stream.Done():
		}
		close(ended)
	}()
	return ended
}

// reattachFloor is how long a terminal had to last for another to take its
// place when its stream ended.
const reattachFloor = 250 * time.Millisecond

// watchTerm drops a pane's terminal when its stream ended, and attaches one
// again while the pane still runs a terminal: a terminal whose stream the
// daemon closed takes no input, and the screen it shows would stay as it
// was. The scrollback the terminal kept goes with it, and the pane's screen
// comes back from hylad, which keeps it.
func (a *App) watchTerm(id string, e *termEntry, ended <-chan struct{}) {
	<-ended
	a.main(func() {
		if a.Terms[id] != e {
			return
		}
		a.dropTerm(id)
		// A stream that ended at once ended for a reason attaching again
		// would meet again: the pane's going, or the daemon's.
		if a.termPane(id) != nil && time.Since(e.started) >= reattachFloor {
			a.EnsureTerm(id)
		}
	})
}

// termPane is a pane the app still knows that runs a terminal, nil when it
// is gone, or shows a page, a file, a timeline or the subagents.
func (a *App) termPane(id string) *proto.Pane {
	for i := range a.St.Sessions {
		if p := a.St.Sessions[i].PaneByID(id); p != nil && !p.Shown() {
			return p
		}
	}
	return nil
}

// hereIdle is how long after the user's last input, on the keyboard or
// mouse, they are taken to be at this machine still; awayIdle how long
// without it they are taken to have gone, as to another machine's app.
const (
	hereIdle = 2 * time.Second
	awayIdle = 5 * time.Second
)

// here reports whether the user is at this window: it is focused, and
// they used the machine just now. A window stays focused on a machine the
// user left, whose app then moves its panes without them, as when the
// session it showed closed: the idle time the system tells says they
// left. A system that tells none leaves it to the window's focus.
func (a *App) here() bool {
	if !a.focused {
		return false
	}
	return a.IdleTime == nil || a.IdleTime() < hereIdle
}

// claimShown gives the panes the window shows the window's sizes, which
// another machine's app may have taken (client.Stream.Claim), while the
// user is here.
func (a *App) claimShown() {
	s := a.session(a.Selected)
	if s == nil || !a.here() {
		return
	}
	for _, id := range shownPanes(s) {
		if e := a.Terms[id]; e != nil && e.stream != nil {
			go e.stream.Claim()
		}
	}
}

// watchReturn claims the panes the window shows as the user comes back
// to this machine after a while away, as at another machine's app, which
// sized them for its own window: they need not type in them or move into
// them for this window to show them whole.
func (a *App) watchReturn() {
	away := false
	for range time.Tick(time.Second) {
		switch idle := a.IdleTime(); {
		case idle >= awayIdle:
			away = true
		case away && idle < hereIdle:
			away = false
			a.UpdateNow(a.claimShown)
		}
	}
}

func (a *App) termFont() terminal.Font {
	f := terminal.Font{Family: a.termFamily(), Size: a.settingsIn.FontSize, Weight: a.settingsIn.FontWeight,
		CellWidth: a.settingsIn.CellWidth, LineHeight: a.settingsIn.LineHeight, Thicken: a.settingsIn.Thicken}
	f.Features = a.fontFeatures()
	return f
}

// fontFeatures are the terminals' OpenType features, as terminal.Font
// takes them: ligatures off unless they're wanted, then the user's
// font-feature, which win over them for the tags they name.
func (a *App) fontFeatures() []string {
	var fs []string
	if !a.settingsIn.Ligatures {
		for _, tag := range []string{"calt", "liga", "dlig"} {
			if !slices.ContainsFunc(a.settingsIn.FontFeatures, func(f string) bool { return featureTag(f) == tag }) {
				fs = append(fs, "-"+tag)
			}
		}
	}
	return append(fs, a.settingsIn.FontFeatures...)
}

// featureTag is the tag of a feature as terminal.Font takes it.
func featureTag(f string) string {
	tag, _, _ := strings.Cut(strings.TrimLeft(f, "+-"), "=")
	return tag
}

// uiFeatures converts features as terminal.Font takes them, as -calt,
// into the list ui.Span.Features takes, as calt=0.
func uiFeatures(fs []string) string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		if tag, ok := strings.CutPrefix(f, "-"); ok {
			f = tag + "=0"
		} else {
			f = strings.TrimPrefix(f, "+")
		}
		out = append(out, f)
	}
	return strings.Join(out, ",")
}

// termThemes returns the terminal's colors in light and dark windows:
// hyla's, on the card's color, or a Ghostty theme of the user's choice.
func (a *App) termThemes() (light, dark *terminal.Theme) {
	pick := func(name string, def *terminal.Theme) *terminal.Theme {
		if name == "" || name == "hyla" {
			return def
		}
		if t, err := terminal.GhosttyTheme(name); err == nil {
			return t
		}
		return def
	}
	return pick(a.settingsIn.LightTheme, theme.LightTerm()), pick(a.settingsIn.DarkTheme, theme.DarkTerm())
}

// commandKeys are the keys that, with ⌘ alone on a Mac, send the program a
// control byte, as Ghostty's do.
var commandKeys = map[ui.Key]byte{
	ui.KeyBackspace: 0x15, // Control+U, kill-line
	ui.KeyLeft:      0x01, // Control+A, beginning-of-line
	ui.KeyRight:     0x05, // Control+E, end-of-line
}

func (a *App) TermOptions(id string, conn *quick.Gated) terminal.Options {
	light, dark := a.termThemes()
	o := terminal.Options{
		Conn:          conn,
		Font:          a.termFont(),
		Theme:         light,
		DarkTheme:     dark,
		Cursor:        map[string]terminal.CursorStyle{"block": terminal.CursorBlock, "bar": terminal.CursorBar, "underline": terminal.CursorUnderline}[a.settingsIn.Cursor],
		NoBlink:       !a.settingsIn.CursorBlink,
		OptionAsAlt:   a.settingsIn.OptionAsAlt,
		ClipboardRead: map[string]terminal.ClipboardAccess{"ask": terminal.ClipboardAsk, "allow": terminal.ClipboardAllow, "deny": terminal.ClipboardDeny}[a.settingsIn.Clipboard],
		RightClick:    map[string]terminal.RightClickAction{"menu": terminal.RightClickContextMenu, "paste": terminal.RightClickPaste, "copyOrPaste": terminal.RightClickCopyOrPaste}[a.settingsIn.RightClick],
		// The pane's card shows under it, dimmed when the user is in
		// another pane.
		Transparent: true,
		// Text lines up with the pane's title; a full-screen program's
		// colors reach the card's edges.
		Padding:      terminal.Padding{X: 16, Y: 6},
		PaddingColor: terminal.PaddingExtend,
		// Text a program colored for the other appearance, as a dark
		// theme's light grays on a light one, stays legible.
		MinimumContrast:  a.settingsIn.MinContrast,
		ScrollMultiplier: terminal.ScrollMultiplier{Precision: a.settingsIn.ScrollPrecision, Discrete: a.settingsIn.ScrollDiscrete},
		// The scrollback-limit hylad's copy keeps, never the terminal's
		// default, 10 MB, which would keep more here than there.
		Scrollback: a.settingsIn.ScrollbackBytes(),
		// Every terminal zooms together, and the size is remembered.
		OnZoom: func(z terminal.Zoom) bool {
			a.zoom(z.Steps, z.Reset)
			return true
		},
		// ⌘K and the context menu's Clear clear hylad's terminal, the
		// session's: cleared here alone, hylad's next frames, drawn from a
		// screen still full, would put rows back wrong.
		OnClear: func(terminal.Clear) bool {
			conn.Clear(proto.ClearScreen)
			return true
		},
		// ⌘⌫ clears the line the shell is on, and ⌘← and ⌘→ go to its
		// start and end, as in Ghostty: they send Control+U, Control+A and
		// Control+E, which shells take as kill-line, beginning-of-line and
		// end-of-line.
		OnKey: func(mods ui.Modifiers, key ui.Key) bool {
			if runtime.GOOS != "darwin" || mods != ui.Super {
				return false
			}
			b, ok := commandKeys[key]
			if !ok {
				return false
			}
			t := a.TermFor(id)
			if t == nil {
				return false
			}
			t.Send([]byte{b})
			return true
		},
		// The wheel, the user's even over a window not focused, tells they
		// are in this pane, which then takes this window's size back from
		// another machine's app (Stream.Claim); it scrolls as it would. A
		// claim goes once the pointer rests, not once a scroll event, which
		// a trackpad sends by the hundred: each asks the host for the
		// stream's size, and its frames hold the terminal up.
		OnScroll: func(int) bool {
			a.claimWhenRest(id, conn)
			return false
		},
		OpenLink: func(link string) bool { return a.openLink(id, link) },
		// An image alone on the clipboard pastes as the path of a file
		// with it, which agents take as an image they attach.
		OnPaste: func(d transfer.Data) bool { return a.PasteImage(id, d) },
		// A selection copied as it ends shows nothing else: say so.
		OnCopy: func(_ string, by terminal.CopySource) {
			if by == terminal.CopySelect {
				a.toast(locales.ToastCopiedToClipboard())
				if a.Win != nil {
					a.Win.Invalidate()
				}
			}
		},
	}
	if a.settingsIn.CopyOnSelect {
		o.CopyOnSelect = terminal.CopyOnSelectClipboard
	}
	return o
}

// claimWhenRest tells the host the user is in the pane id, as its stream's
// Claim does, once the wheel rests: a wheel's scroll, and every one of a
// trackpad's, comes sooner. Each claim makes the host size the pane to
// this window, and its frames wait for that. A scroll after the last puts
// the claim off, rather than start one of its own to drop.
func (a *App) claimWhenRest(id string, conn claimer) {
	a.claimMu.Lock()
	defer a.claimMu.Unlock()
	if a.Claims == nil {
		a.Claims = map[string]*claimWait{}
	}
	if cw := a.Claims[id]; cw != nil {
		cw.t.Reset(claimRest)
		return
	}
	cw := &claimWait{}
	cw.t = time.AfterFunc(claimRest, func() { a.claimAtRest(id, cw, conn) })
	a.Claims[id] = cw
}

// claimAtRest claims the pane id for the wheel's rest cw, if it is still
// the pane's last, on the main thread.
func (a *App) claimAtRest(id string, cw *claimWait, conn claimer) {
	a.main(func() {
		a.claimMu.Lock()
		last := a.Claims[id] == cw
		if last {
			delete(a.Claims, id)
		}
		a.claimMu.Unlock()
		if !last || a.Terms[id] == nil {
			return
		}
		conn.Claim()
	})
}

// claimWait is a pane's wheel resting, and the timer that ends the rest.
type claimWait struct{ t *time.Timer }

// claimer asks a host for the size of the pane a stream shows.
type claimer interface{ Claim() error }

// claimRest is how long the wheel rests before a pane claims the size: more
// than the least between a trackpad's scroll events, short enough that the
// user, arriving, waits for nothing they see.
const claimRest = 150 * time.Millisecond

// zoom makes every terminal's text bigger or smaller, which Settings
// remembers as the font's size.
func (a *App) zoom(steps int, reset bool) {
	s := a.settingsIn
	s.FontSize = min(max(s.FontSize+float32(steps), 8), 36)
	if reset {
		s.FontSize = settings.Default().FontSize
	}
	if s.FontSize != a.settingsIn.FontSize {
		a.applySettings(s)
	}
}

// applySettings makes settings the app's, saving them: the terminals take
// what changed at once, and are attached again for what only a new
// terminal takes.
func (a *App) applySettings(s settings.Settings) {
	// Where tailcat finds its relays, which a pair code's region names.
	remote.SetRelay(s.TailcatRelay)
	a.apply(s, true)
	a.rememberEnv()
	a.State.Soon()
}

// rememberEnv keeps this window's environment, which hylad runs the
// bot's commands in: launchd starts it with the system's PATH, and none
// of the account's variables, the providers' settings among them. The
// settings' save, as the app starts and as each of them changes, reads
// them again, as a token that ended is read where it is kept.
func (a *App) rememberEnv() {
	if !settings.FeishuSet(settings.Load().Feishu) {
		return
	}
	host, err := os.Hostname()
	if err != nil {
		return
	}
	if err := settings.SetAppEnv(host, uisettings.AppEnv(), time.Now()); err != nil {
		a.toast(err.Error())
	}
}

// apply makes settings the app's, saving them to the configuration when
// they come from the app rather than from the file.
func (a *App) apply(s settings.Settings, save bool) {
	old := a.Applied
	if save {
		if err := settings.Save(s); err != nil {
			a.toast(locales.ToastSaveSettingsFailed(err.Error()))
		}
		// The file's keep the app's: a value it keeps in range, or
		// reads as another, is one the app shows as saved.
		s = settings.Load()
	}
	a.settingsIn, a.Applied = s, s.Clone()
	applied.Lock()
	applied.s = a.settingsIn
	applied.Unlock()
	font := old.FontFamily != s.FontFamily || old.FontFallback != s.FontFallback || old.FontSize != s.FontSize || old.FontWeight != s.FontWeight ||
		old.CellWidth != s.CellWidth || old.LineHeight != s.LineHeight || old.Ligatures != s.Ligatures || old.Thicken != s.Thicken ||
		!slices.Equal(old.FontFeatures, s.FontFeatures)
	themes := old.LightTheme != s.LightTheme || old.DarkTheme != s.DarkTheme
	anew := old.Cursor != s.Cursor || old.CursorBlink != s.CursorBlink || old.OptionAsAlt != s.OptionAsAlt ||
		old.Clipboard != s.Clipboard || old.RightClick != s.RightClick
	light, dark := a.termThemes()
	retune := func(t *terminal.Terminal) {
		if font {
			t.SetFont(a.termFont())
		}
		if themes {
			t.SetTheme(light, dark)
		}
		if old.MinContrast != s.MinContrast {
			t.SetMinimumContrast(s.MinContrast)
		}
		if old.ScrollPrecision != s.ScrollPrecision || old.ScrollDiscrete != s.ScrollDiscrete {
			t.SetScrollMultiplier(terminal.ScrollMultiplier{Precision: s.ScrollPrecision, Discrete: s.ScrollDiscrete})
		}
		if old.CopyOnSelect != s.CopyOnSelect {
			mode := terminal.CopyOnSelectNone
			if s.CopyOnSelect {
				mode = terminal.CopyOnSelectClipboard
			}
			t.SetCopyOnSelect(mode)
		}
	}
	for id, e := range a.Terms {
		if e.term == nil {
			continue
		}
		if anew {
			a.dropTerm(id) // attached again as it shows
			continue
		}
		retune(e.term)
	}
	if t := a.quickTermFor(a.quick.Pane); t != nil {
		if anew {
			a.dropQuick()
			if a.quick.Shown {
				a.attachQuick()
			}
		} else {
			retune(t)
		}
	}
	if old.Appearance != s.Appearance && a.Win != nil {
		// Only in the running app: before it runs, the call waits for it.
		mygo.Theme.SetSource(mygo.ThemeSource(s.Appearance))
	}
	if old.Language != s.Language && a.Win != nil {
		mygo.App.SetLocale(uisettings.LocaleTag(s.Language))
	}
	// The wash changes with the material or its blur, which a change of
	// opacity too, its alpha only, would leave as it was.
	if old.Vibrancy != s.Vibrancy || old.WindowBlur != s.WindowBlur || old.WindowOpacity != s.WindowOpacity || old.WindowMaterial != s.WindowMaterial {
		if a.Win != nil {
			a.Win.SetVibrancy(vibrancyFor(s))
		}
		if a.SetWin != nil {
			a.SetWin.SetVibrancy(vibrancyFor(s))
		}
	}
	if !maps.Equal(old.Shortcuts, s.Shortcuts) && a.Win != nil {
		mygo.App.SetMenu(a.menu())
		a.applyGlobalShortcuts()
	}
	if old.DockBadge != s.DockBadge {
		a.notify(true)
	}
	if old.Pet != s.Pet && a.Win != nil {
		// Not from inside a view's build: the pet's window opens, or
		// goes, once this frame is done.
		a.UpdateNow(a.syncPet)
	}
	if (old.PetOffset != s.PetOffset || old.PetSize != s.PetSize) && a.Pet.Win != nil {
		a.UpdateNow(a.petPlace)
	}
	if old.PetSet != s.PetSet && a.Pet.Win != nil {
		a.UpdateNow(a.petSetReload)
	}
	if old.AppIcon != s.AppIcon && a.Win != nil {
		a.changeAppIcon()
	}
	// Only once a browser pane showed a page: until then, no lists are
	// downloaded.
	if (old.BlockAds != s.BlockAds || !slices.Equal(old.AdLists, s.AdLists)) && (a.Ads.Started() || len(a.Webs) > 0) {
		a.applyAdBlock()
	}
	// The bot's commands run in this window's environment, which a token
	// or a variable read at each login changes: the bot connects again
	// as the window reads settings again.
	if a.Win != nil {
		a.restartFeishu()
	}
	if a.Win != nil {
		a.Win.Invalidate()
	}
	if a.Pet.Win != nil {
		a.Pet.Win.Invalidate()
	}
	if a.SetWin != nil {
		a.SetWin.Invalidate()
	}
	if a.quick.Win != nil {
		a.quick.Win.Invalidate()
	}
}

// watchConfig applies the configuration as the user saves it in an
// editor, and tells what it got wrong. The system hints at changes in the
// file's directory (FSEvents on macOS), where an editor's save, which
// replaces the file, shows too: a hint is all it takes, as Load reads the
// file again only when its modification time changed, so the watch keeps
// no snapshot and stats nothing. The app's own saves come back as changes
// already applied, which change nothing. A configuration that is a
// symlink, as one kept in a dotfiles repository, is watched in the
// directory of the file it points to as well, where the user edits it.
func (a *App) watchConfig() {
	dir := filepath.Dir(settings.Path())
	for {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
		ctx, stop := context.WithCancel(context.Background())
		w, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{HintsOnly: true})
		if err != nil {
			stop()
			return
		}
		linked := filepath.Dir(settings.Target())
		if linked != dir {
			go a.watchLinkedConfig(ctx, linked)
		}
		for {
			if _, err := w.Next(context.Background()); err != nil {
				break // the directory went: watch it again, made anew
			}
			a.reloadConfig()
			if filepath.Dir(settings.Target()) != linked {
				break // the symlink points elsewhere now: watch there
			}
		}
		stop()
		w.Close()
	}
}

// watchLinkedConfig applies the configuration as the user saves the file
// a symlinked configuration points to, watching its directory till ctx
// ends or the directory goes.
func (a *App) watchLinkedConfig(ctx context.Context, dir string) {
	w, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{HintsOnly: true})
	if err != nil {
		return
	}
	defer w.Close()
	for {
		if _, err := w.Next(ctx); err != nil {
			return
		}
		a.reloadConfig()
	}
}

// reloadConfig applies the configuration as the file now has it.
func (a *App) reloadConfig() {
	cur := settings.Load()
	problems := settings.Problems()
	a.UpdateNow(func() {
		changed := strings.Join(problems, "\n") != strings.Join(a.Ss.Problems, "\n")
		a.Ss.Problems = problems
		if !reflect.DeepEqual(cur, a.Applied) {
			a.apply(cur, false)
		}
		if changed && len(problems) > 0 {
			a.toast(locales.ToastConfigProblems(len(problems)))
		}
		if a.SetWin != nil {
			a.SetWin.Invalidate()
		}
	})
}

// vibrancyFor is the window's material, if Background blur is on: off,
// the desktop shows behind the wash without it. The material is the
// setting's: the sidebar's by default, and Liquid Glass, as the Dock's,
// by choice.
func vibrancyFor(s settings.Settings) mygo.Vibrancy {
	if !s.Vibrancy || !s.WindowBlur {
		return mygo.VibrancyNone
	}
	switch s.WindowMaterial {
	case "window":
		return mygo.VibrancyWindow
	case "popover":
		return mygo.VibrancyPopover
	case "plastic":
		return mygo.VibrancyContent
	case "glass":
		return mygo.VibrancyGlass
	case "glass-clear":
		return mygo.VibrancyGlassClear
	}
	return mygo.VibrancySidebar
}

// openLink opens what a Command+click opens: URLs with the system, and
// file paths printed in the session, relative to its directory, in the
// app that opens them.
func (a *App) openLink(id, link string) bool {
	if strings.Contains(link, "://") {
		return false
	}
	path := a.resolvePath(id, link)
	if path == "" {
		return false
	}
	platform.OpenPath(path)
	return true
}

// resolvePath returns the file a path printed in a pane names, relative to
// the directory of the program that printed it, or "" for no such file.
// id is a pane's, or a session's for its pane in front.
func (a *App) resolvePath(id, link string) string {
	path := panes.Path(link)
	if strings.HasPrefix(path, "~/") || runtime.GOOS == "windows" && strings.HasPrefix(path, `~`) {
		path = filepath.Join(proto.Home(), path[2:])
	}
	if !filepath.IsAbs(path) {
		if dir := a.workDir(id); dir != "" {
			path = filepath.Join(dir, path)
		}
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// workDir returns the working directory of the pane, or of the session's
// pane in front, by id.
func (a *App) workDir(id string) string {
	for i := range a.St.Sessions {
		if p := a.St.Sessions[i].PaneByID(id); p != nil {
			return p.WorkDir()
		}
	}
	if s := a.session(id); s != nil {
		return s.WorkDir()
	}
	return ""
}

// --- actions ---

// CallPane calls a method of the hylad that has id: a session's, a pane's
// or a project's, toasting a failure unless done takes it.
func (a *App) CallPane(id, method string, params any, done func(err error)) {
	a.callOn(a.hostOf(id), method, params, done)
}

// Call calls a method of the hylad that has id, toasting a failure unless
// done takes it.
func (a *App) Call(id, method string, params any, done func(error)) {
	a.CallPane(id, method, params, done)
}

// callOn calls a method of a host's hylad.
func (a *App) callOn(h *host.Host, method string, params any, done func(err error)) {
	if h.Client == nil {
		// What the app showed at once comes back as the host does: say so.
		err := errors.New(locales.HostNotConnected(h.Label()))
		if done != nil {
			done(err)
		} else {
			a.toast(err.Error())
		}
		return
	}
	h.Client.Go(method, params, func(err error) {
		if done != nil || err != nil {
			a.UpdateNow(func() {
				if done != nil {
					done(err)
				} else {
					a.toast(err.Error())
				}
			})
		}
	})
}

func (a *App) toast(msg string) { a.PendingToast = msg }

// UndoSays says what a removal took away, and offers undo, which puts it
// back.
func (a *App) UndoSays(msg string, undo func()) {
	a.UndoMessage, a.UndoAction = msg, undo
	a.toast(msg)
}

// newSession starts a session in a project: in the directory of the
// session shown when it is the project's, as a terminal's new tab does,
// else the project's. With no project, it is the shown session's host's.
// SessionIn starts a session in a project, or in none's folder.
func (a *App) SessionIn(projectID string) {
	h := a.selectedHost()
	if projectID != "" {
		h = a.hostOf(projectID)
	}
	// ⌘T in an SSH or K8s terminal adds a machine or a pod, which opens
	// its terminal, as the project's own New Session does; a shell there
	// is no machine's.
	if projectID == "" && h == a.Local() {
		if cur := a.session(a.Selected); cur != nil && cur.Project != "" {
			if cur.Project == a.Prefs.SSHProject {
				a.AskSSHHost()
				return
			}
			if cur.Project == a.Prefs.K8sProject {
				a.AskK8sPod()
				return
			}
		}
	}
	a.NewSessionOn(h, projectID)
}

// newSessionOn starts a session on a host, in a project of its, "" for
// the one of the session shown there, else the first.
func (a *App) NewSessionOn(h *host.Host, projectID string) {
	if h.Client == nil {
		return
	}
	// The SSH and K8s projects' New Session adds a machine or a pod,
	// which opens its terminal.
	if projectID != "" && projectID == a.Prefs.SSHProject {
		a.AskSSHHost()
		return
	}
	if projectID != "" && projectID == a.Prefs.K8sProject {
		a.AskK8sPod()
		return
	}
	p := proto.CreateParams{Project: projectID}
	if cur := a.session(a.Selected); cur != nil && a.hostOf(cur.ID) == h {
		if projectID == "" || cur.Project == projectID {
			p.Project, p.After = cur.Project, cur.ID
			if a.settingsIn.NewSessionDir == "current" {
				p.Dir = cur.FocusPane().WorkDir()
			}
		}
		p.Cols, p.Rows = cur.Cols, cur.Rows
	}
	home := proto.Home()
	if a.settingsIn.NewSessionDir == "home" {
		p.Dir = home
	}
	if p.Project == "" && len(a.ProjectsOf(h)) == 0 {
		p.Dir = home
	}
	a.create(h, p, "")
}

// create starts a session on a host, and shows it, opening the project
// expand in the sidebar.
func (a *App) create(h *host.Host, p proto.CreateParams, expand string) {
	cl := h.Client
	if cl == nil {
		return
	}
	go func() {
		var s proto.Session
		err := cl.Call("create", p, &s)
		a.UpdateNow(func() {
			if err != nil {
				a.toast(err.Error())
				return
			}
			if expand != "" {
				delete(a.Prefs.Collapsed, expand)
			}
			// The state with it may come after the answer.
			a.addPending(h, s)
			a.selectSession(s.ID)
		})
	}()
}

// addPending puts a session just created into the state until its host's
// comes with it.
func (a *App) addPending(h *host.Host, s proto.Session) {
	if a.session(s.ID) != nil {
		return
	}
	a.St.Sessions = append(a.St.Sessions, s)
	if a.project(s.Project) == nil {
		a.St.Projects = append(a.St.Projects, proto.Project{ID: s.Project, Name: proto.DirName(s.Dir), Dir: s.Dir})
	}
}

// openProject asks for a folder, and starts a session there, in its
// project: of this machine, with the system's dialog, else, with hosts,
// of any machine, with the app's picker, from the shown session's.
func (a *App) OpenProjectDialog() {
	h := a.Local()
	if h.Client == nil {
		return
	}
	win := a.Win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win, Title: locales.OpenProjectTitle(), Directory: true, ButtonLabel: locales.CommonOpen(),
			Message: locales.OpenProjectMessage(),
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.UpdateNow(func() { a.openProjectAt(h, paths[0]) })
	}()
}

// openProjectAt starts a session in a folder of a host, in its project.
func (a *App) openProjectAt(h *host.Host, dir string) {
	cl, local := h.Client, true
	if cl == nil {
		return
	}
	go a.openProjectDirs(h, cl, []string{dir}, local)
}

// openProjectDirs runs off the UI thread with the host and client captured.
func (a *App) openProjectDirs(h *host.Host, cl *client.Client, dirs []string, local bool) {
	seen := map[string]bool{}
	var errs []error
	for _, dir := range dirs {
		// Remote paths are validated by their host, not this machine's path rules.
		if local && !filepath.IsAbs(dir) {
			errs = append(errs, fmt.Errorf("not an absolute directory: %s", dir))
			continue
		}
		var p proto.Project
		if err := cl.Call("project.create", proto.ProjectParams{Dir: dir}, &p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", dir, err))
			continue
		}
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		var s proto.Session
		if err := cl.Call("create", proto.CreateParams{Project: p.ID, Dir: dir}, &s); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", dir, err))
			continue
		}
		a.UpdateNow(func() {
			if h.Client != cl {
				return
			}
			delete(a.Prefs.Collapsed, p.ID)
			if a.project(p.ID) == nil {
				a.St.Projects = append(a.St.Projects, p)
			}
			a.addPending(h, s)
			a.selectSession(s.ID)
		})
	}
	if err := errors.Join(errs...); err != nil {
		a.UpdateNow(func() {
			if h.Client == cl {
				a.toast(err.Error())
			}
		})
	}
}

// busy reports whether closing a session would end a program the user may
// care about: anything but the shell at its prompt.
func busy(s *proto.Session) bool {
	return s.Status != proto.Exited && s.Process != ""
}

func (a *App) CloseSession(id string, ask bool) {
	s := a.session(id)
	if s == nil {
		return
	}
	if ask && s.Worktree != nil && a.hostOf(id).Client != nil {
		worktree.AskCloseSession(a, id)
		return
	}
	if ask && a.settingsIn.ConfirmClose && busy(s) {
		name := s.DisplayName(a.settingsIn.SessionName)
		a.Confirm = &confirm.Ask{
			Open:    true,
			Title:   locales.ConfirmCloseTitle(name),
			Detail:  locales.ConfirmCloseSessionDetail(s.Process),
			Action:  locales.ActionCloseSession(),
			Destroy: true,
			Run:     func() { a.CloseSession(id, false) },
		}
		return
	}
	a.EndSession(id, proto.CloseParams{ID: id})
}

// EndSession closes a session, as p says, without asking.
func (a *App) EndSession(id string, p proto.CloseParams) {
	s := a.session(id)
	if s == nil {
		return
	}
	h := a.hostOf(id)
	if id == a.Selected {
		// The next session takes its place, else the one before.
		ids := sidebar.VisibleOrder(a.order())
		i := slices.Index(ids, id)
		next := ""
		if i >= 0 && i+1 < len(ids) {
			next = ids[i+1]
		} else if i > 0 {
			next = ids[i-1]
		}
		a.Selected = next
		if next != "" {
			a.selectSession(next)
		}
	}
	for _, p := range s.Panes {
		a.dropTerm(p.ID)
	}
	a.St.Sessions = slices.DeleteFunc(a.St.Sessions, func(x proto.Session) bool { return x.ID == id })
	a.callOn(h, "close", p, nil)
}

// CloseAllSessions closes every session of a group, leaving its
// project, once the user confirmed it: what runs in them ends with
// them. The ids are gathered first: closing moves the state the
// group's sessions point into.
func (a *App) CloseAllSessions(g sidebar.Group) {
	for _, id := range slices.Clone(g.Sessions) {
		a.CloseSession(id, false)
	}
}

func (a *App) RestartSession(id string) {
	a.Call(id, "restart", proto.IDParams{ID: id}, nil)
}

func (a *App) StartRename(id string) {
	s := a.session(id)
	if s == nil {
		return
	}
	a.Renaming, a.RenamingProject, a.RenameBuf = id, "", s.DisplayName(a.settingsIn.SessionName)
	a.RenameFramesF = 0
}

func (a *App) CommitRename() {
	id, name := a.Renaming, strings.TrimSpace(a.RenameBuf)
	a.Renaming = ""
	s := a.session(id)
	if s == nil || name == s.DisplayName(a.settingsIn.SessionName) {
		a.focusTerminal()
		return
	}
	s.Name = name // at once; the daemon's state follows
	a.Call(id, "rename", proto.RenameParams{ID: id, Name: name}, nil)
	a.focusTerminal()
	a.UpdateTitle()
}

func (a *App) StartProjectRename(id string) {
	for _, p := range a.St.Projects {
		if p.ID == id {
			a.Renaming, a.RenamingProject, a.RenameBuf = "", id, p.Name
			a.RenameFramesF = 0
		}
	}
}

func (a *App) CommitProjectRename() {
	id, name := a.RenamingProject, strings.TrimSpace(a.RenameBuf)
	a.RenamingProject = ""
	for i := range a.St.Projects {
		if p := &a.St.Projects[i]; p.ID == id && name != "" && name != p.Name {
			p.Name = name // at once; the daemon's state follows
			a.Call(id, "project.rename", proto.ProjectParams{ID: id, Name: name}, nil)
		}
	}
	a.focusTerminal()
}

func (a *App) moveSession(id, project string) {
	if s := a.session(id); s != nil {
		s.Project = project
	}
	a.Call(id, "move", proto.MoveParams{ID: id, Project: project, Index: -1}, nil)
}

func (a *App) selectOffset(d int) {
	ids := sidebar.VisibleOrder(a.order())
	if len(ids) == 0 {
		return
	}
	i := slices.Index(ids, a.Selected)
	if i < 0 {
		i = 0
	} else {
		i = (i + d + len(ids)) % len(ids)
	}
	a.selectSession(ids[i])
}

// jumpTo goes to the next session that want takes, after the one shown,
// in the sidebar's order, opening its project, and reports whether there
// was one.
func (a *App) jumpTo(want func(*proto.Session) bool) bool {
	var ids []string
	for _, g := range sidebar.Groups(a.order()) {
		ids = append(ids, g.Sessions...)
	}
	i := slices.Index(ids, a.Selected)
	for k := 1; k <= len(ids); k++ {
		id := ids[(i+k+len(ids))%len(ids)]
		if s := a.session(id); s != nil && id != a.Selected && want(s) {
			delete(a.Prefs.Collapsed, s.Project)
			a.selectSession(id)
			return true
		}
	}
	return false
}

// jumpToAttention goes to the next session that needs the user, after the
// one shown, else to the next with something unseen.
func (a *App) jumpToAttention() {
	if a.jumpTo(func(s *proto.Session) bool { return s.Status == proto.Waiting }) ||
		a.jumpTo(func(s *proto.Session) bool { return s.Unread }) {
		return
	}
	a.toast(locales.ToastNothingNeedsYou())
}

func (a *App) ToggleSidebar() {
	a.Prefs.SidebarHidden = !a.Prefs.SidebarHidden
	a.savePrefs()
}

func (a *App) toggleProject(id string) {
	a.Prefs.Collapsed[id] = !a.Prefs.Collapsed[id]
	if !a.Prefs.Collapsed[id] {
		delete(a.Prefs.Collapsed, id)
	}
	a.savePrefs()
}

// allProjectsCollapsed reports whether every group the sidebar lists by
// project is folded.
func (a *App) allProjectsCollapsed() bool {
	gs := sidebar.Groups(a.order())
	if len(gs) == 0 {
		return false
	}
	for _, g := range gs {
		if !sidebar.Collapsed(a.order(), g) {
			return false
		}
	}
	return true
}

// ToggleAllProjects folds every group the sidebar lists by project, or
// unfolds them all unless all are folded, so that one press always shows
// a change.
func (a *App) ToggleAllProjects() {
	collapse := !a.allProjectsCollapsed()
	for _, g := range sidebar.Groups(a.order()) {
		if collapse {
			a.Prefs.Collapsed[g.Key()] = true
			continue
		}
		delete(a.Prefs.Collapsed, g.Key())
	}
	a.savePrefs()
}

// quitRequested is mygo's quit request, from Cmd+Q, the menu, the window's
// close, or the Dock, and reports whether the quit goes ahead. With
// quit-ends-sessions, a quit ends this machine's sessions as
// endSessionsAndQuit does, so it asks what Quit and End All Sessions asks
// while a program runs, and closes nothing until the answer says so.
func (a *App) quitRequested() bool { return quit.Requested(a) }

// ArmQuitTimer leaves the app after quit.Grace, whatever holds the quit.
func (a *App) ArmQuitTimer() {
	if a.Win == nil || a.quitTimer != nil {
		return
	}
	a.quitTimer = time.AfterFunc(quit.Grace, func() { os.Exit(0) })
}

// BringToFront shows the window, and brings it forward, so that the user
// sees what the app asks, or ends.
func (a *App) BringToFront() {
	if a.Win == nil {
		return
	}
	a.Win.Show() // it has no native window, as in tests
	a.Win.Restore()
	a.Win.Focus()
}

// hostsLoaded reports whether every host has shown its sessions, as the
// app asks before it ends this machine's.
func (a *App) hostsLoaded() bool {
	for _, h := range a.Machines {
		if h.Conn == host.StateConnecting || (h.Client != nil && !h.Loaded) {
			return false
		}
	}
	return true
}

// daemonStale reports whether hylad runs another version of Hyla than
// this one, as after an update: it speaks the protocol, but runs the code
// of before until it restarts.
func (a *App) daemonStale() bool { return a.Local().Stale() }

// OfferRestart restarts hylad into this version of Hyla.
func (a *App) OfferRestart() { quit.Offer(a) }

// busyCount counts a host's sessions whose programs a restart of its
// hylad would end.
func (a *App) busyCount(h *host.Host) int {
	n := 0
	for _, s := range a.SessionsOf(h) {
		if busy(&s) {
			n++
		}
	}
	return n
}

// restartFeishu connects the bot again, with what the configuration says
// now: the environment its agent's commands run in.
func (a *App) restartFeishu() {
	p := proto.FeishuEnv{Env: uisettings.AppEnv()}
	p.Host, _ = os.Hostname()
	a.callOn(a.Local(), "feishu.restart", p, nil)
}

// restartDaemon restarts this machine's hylad.
func (a *App) restartDaemon() { quit.Daemon(a) }

// quitAndEnd ends every session of this machine, and its hylad, then
// quits: other machines' keep running.
func (a *App) quitAndEnd() {
	n := len(a.SessionsOf(a.Local()))
	a.Confirm = &confirm.Ask{
		Open:    true,
		Title:   locales.ConfirmQuitAllTitle(),
		Detail:  quit.Detail(n, len(a.Machines) > 1),
		Action:  locales.ConfirmQuitAllAction(),
		Destroy: true,
		Ends:    true,
		Run:     a.EndSessionsAndQuit,
	}
}

// EndSessionsAndQuit ends every session of this machine, and its hylad,
// then quits: other machines' keep running. A shutdown that takes a
// while is the daemon's to finish, as it is for Quit and End All
// Sessions: the app does not wait on it.
func (a *App) EndSessionsAndQuit() {
	h := a.Local()
	cl := h.Client
	// The daemon may take its time ending the sessions: the app leaves
	// within quit.Grace whatever it answers.
	a.ArmQuitTimer()
	go func() {
		if cl == nil {
			quit.EndsSession(a)
		} else if cl.Call("shutdown", nil, nil) != nil {
			// hylad went before it answered, or speaks another version:
			// stop the one this machine runs.
			a.StopDaemon()
		}
		mygo.App.Quit()
	}()
}

// The app, as the sidebar's order asks it.
var _ sidebar.App = order{}

func (a *App) SessionsNow() []proto.Session { return a.St.Sessions }

func (a *App) ProjectsNow() []proto.Project { return a.St.Projects }

func (a *App) PrefsNowAll() sidebarview.Prefs {
	// The orders its callers set are a copy's: they go back to the app's
	// as they are set, which Save keeps.
	return sidebarview.Prefs{
		Prefs: sidebar.Prefs{
			Collapsed:    a.Prefs.Collapsed,
			ProjectOrder: a.Prefs.ProjectOrder,
			SessionOrder: a.Prefs.SessionOrder,
			// A session's place in the project it moved to lasts while the
			// user keeps the order it moved among, which every drag of the
			// sessions sets anew: the drag order of one session, the one
			// that moved.
			SessionProjects: a.movedSessions(),
			HiddenProjects: map[string]bool{
				a.Prefs.SSHProject: true,
				a.Prefs.K8sProject: true,
			},
			SidebarGroup: a.Prefs.SidebarGroup,
			SidebarOrder: a.Prefs.SidebarOrder,
			SetProjectOrder: func(o []string) {
				a.Prefs.ProjectOrder = o
			},
			SetSessionOrder: func(o []string) {
				a.Prefs.SessionOrder = o
			},
			SetSessionProjects: func(m map[string]string) {
				a.Prefs.MovedSessions = m
			},
			Save: a.savePrefs,
		},
		SSHProject: a.Prefs.SSHProject,
		K8sProject: a.Prefs.K8sProject,
	}
}

func (a *App) SelectedID() string { return a.Selected }

// movedSessions are the sessions a drag moved into their project: the
// last drag's session alone, whose place the drag order it moved among
// keeps, and none for another.
func (a *App) movedSessions() map[string]string {
	if len(a.Prefs.SessionOrder) == 0 {
		return a.Prefs.MovedSessions
	}
	last := a.Prefs.SessionOrder[len(a.Prefs.SessionOrder)-1]
	out := map[string]string{}
	if v, ok := a.Prefs.MovedSessions[last]; ok {
		out[last] = v
	}
	return out
}
