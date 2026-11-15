package main

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/worktree"

	"hyla/locales"
)

// newWorktreeSession asks a branch's name for a session in a worktree of
// its own, of the repository of the session the user is in, else of its
// host's first project.
func (a *App) newWorktreeSession() {
	h := a.selectedHost()
	if h.Client == nil {
		a.toast(locales.HostNotConnected(h.Label()))
		return
	}
	var p proto.CreateParams
	if cur := a.session(a.Selected); cur != nil && a.hostOf(cur.ID) == h {
		p.Project, p.After, p.Dir = cur.Project, cur.ID, cur.FocusPane().WorkDir()
		p.Cols, p.Rows = cur.Cols, cur.Rows
	}
	if a.settingsIn.WorktreeBase == "current" {
		p.Base = "HEAD"
	}
	a.WtAsk = &worktree.Ask{Open: true, Host: h, P: p}
}

var _ worktree.App = (*App)(nil)

// Create starts a session, as the worktree dialog's.
func (a *App) Create(h *host.Host, p proto.CreateParams, project string) { a.create(h, p, project) }

func (a *App) worktreeView(c *ui.Context, pal *theme.Palette) {
	w := a.WtAsk
	if branch, made := worktree.View(a, c, pal, w); made {
		w.P.Worktree = branch
		a.create(w.Host, w.P, w.P.Project)
	}
	if w != nil && !w.Open {
		a.WtAsk = nil
		a.focusTerminal()
	}
}

// askCloseWorktree closes a session in a worktree of its own once the
// user says whether its worktree goes with it, as what hylad finds the
// removal would lose decides: Remove at hand when nothing, Keep when
// changes. A worktree another session works in stays, unasked.
func (a *App) askCloseWorktree(id string) {
	s, h := a.session(id), a.hostOf(id)
	cl := h.Client
	if s == nil || cl == nil {
		return
	}
	name := s.DisplayName(a.settingsIn.SessionName)
	go func() {
		var wc proto.WorktreeCheck
		err := cl.Call("worktree.check", proto.IDParams{ID: id}, &wc)
		a.update(func() {
			if a.session(id) == nil {
				return
			}
			keep := func() { a.endSession(id, proto.CloseParams{ID: id}) }
			if err != nil || wc.Shared {
				keep()
				return
			}
			remove := func(force bool) func() {
				return func() { a.endSession(id, proto.CloseParams{ID: id, RemoveWorktree: true, Force: force}) }
			}
			path := h.ShortDir(wc.Path)
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
			a.Confirm = cf
		})
	}()
}
