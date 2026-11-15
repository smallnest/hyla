package worktree

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Sess is what a worktree's sessions ask of the app: more than its
// dialog's own.
type Sess interface {
	// SelectedHost is the host the session the user is in runs on, and
	// HostOf the host of a session.
	SelectedHost() *host.Host
	HostOf(id string) *host.Host
	// SelectedID is the session the user is in, and SessionOf it: nil
	// when there is none.
	SelectedID() string
	SessionOf(id string) *proto.Session
	// SettingsNow are the settings, and Toast says what the app can not
	// do.
	SettingsNow() *settings.Settings
	Toast(msg string)
	// AskNew asks a new session's branch, and AskClose asks whether a
	// closed session's worktree goes with it.
	AskNew(w *Ask)
	AskClose(cf *confirm.Ask)
	// Create starts a session: on host, with p, in the project named.
	Create(h *host.Host, p proto.CreateParams, project string)
	// FocusTerminal gives the focus back to the terminals, and EndSession
	// closes one as p says.
	FocusTerminal()
	EndSession(id string, p proto.CloseParams)
	// Call makes a host call, and UpdateNow runs fn on the main thread.
	Call(id, method string, params any, done func(error))
	UpdateNow(fn func())
}

// NewSession asks a branch's name for a session in a worktree of its own,
// of the repository of the session the user is in, else of its host's
// first project.
func NewSession(a Sess) {
	h := a.SelectedHost()
	if h.Client == nil {
		a.Toast(locales.HostNotConnected(h.Label()))
		return
	}
	var p proto.CreateParams
	if cur := a.SessionOf(a.SelectedID()); cur != nil && a.HostOf(cur.ID) == h {
		p.Project, p.After, p.Dir = cur.Project, cur.ID, cur.FocusPane().WorkDir()
		p.Cols, p.Rows = cur.Cols, cur.Rows
	}
	if a.SettingsNow().WorktreeBase == "current" {
		p.Base = "HEAD"
	}
	a.AskNew(&Ask{Open: true, Host: h, P: p})
}

// ViewOf draws the ask the app holds, and makes the session as the user
// says. It later asks whether a closed session's worktree goes.
func ViewOf(a Sess, w *Ask, c *ui.Context, pal *theme.Palette) {
	if branch, made := View(a, c, pal, w); made {
		w.P.Worktree = branch
		a.Create(w.Host, w.P, w.P.Project)
	}
	if w != nil && !w.Open {
		a.AskNew(nil)
		a.FocusTerminal()
	}
}

// AskCloseSession closes a session in a worktree of its own once the user
// says whether its worktree goes with it, as what hylad finds the removal
// would lose decides.
func AskCloseSession(a Sess, id string) {
	s, h := a.SessionOf(id), a.HostOf(id)
	if s == nil || h.Client == nil {
		return
	}
	// The check waits on the host: the app's call answers off the main
	// thread.
	var wc proto.WorktreeCheck
	name := s.DisplayName(a.SettingsNow().SessionName)
	a.Call(id, "worktree.check", proto.IDParams{ID: id}, func(err error) {
		a.UpdateNow(func() { askClose(a, id, name, &wc, err) })
	})
}

// askClose asks whether a closed session's worktree goes with it, as what
// hylad finds the removal would lose decides: Remove at hand when
// nothing, Keep when changes. A worktree another session works in stays,
// unasked.
func askClose(a Sess, id, name string, wc *proto.WorktreeCheck, err error) {
	if a.SessionOf(id) == nil {
		return
	}
	keep := func() { a.EndSession(id, proto.CloseParams{ID: id}) }
	if err != nil || wc.Shared {
		keep()
		return
	}
	remove := func(force bool) func() {
		return func() { a.EndSession(id, proto.CloseParams{ID: id, RemoveWorktree: true, Force: force}) }
	}
	path := a.HostOf(id).ShortDir(wc.Path)
	cf := &confirm.Ask{Open: true, Title: locales.WorktreeRemoveTitle(name),
		Action: locales.WorktreeRemove(), Run: remove(false), Alt: locales.WorktreeKeep(), AltRun: keep}
	switch {
	case wc.Files > 0:
		cf.Detail = locales.WorktreeRemoveDirty(wc.Files, path, wc.Branch)
		cf.Action, cf.Run = locales.WorktreeKeep(), keep
		cf.Alt, cf.AltRun, cf.AltDestroy = locales.WorktreeRemoveAnyway(), remove(true), true
	case wc.Merged:
		cf.Detail = locales.WorktreeRemoveMerged(path, wc.Branch)
	default:
		cf.Detail = locales.WorktreeRemoveUnmerged(path, wc.Branch)
	}
	a.AskClose(cf)
}
