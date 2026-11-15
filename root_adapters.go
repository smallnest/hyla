package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/root"
	"hyla/internal/ui/side"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/worktree"
	"hyla/locales"
)

// The app, as the window asks it.
var _ root.App = (*App)(nil)

func (a *App) AppSettings() *settings.Settings { return &a.settingsIn }

func (a *App) BorderlessTheme(c *ui.Context, pal *theme.Palette) *ui.Theme { return borderless(c, pal) }

func (a *App) Shortcuts(c *ui.Context) { a.shortcuts(c) }

func (a *App) Tick() { a.State.Tick(a) }

func (a *App) TitleBar(bar ui.TitleBar) { a.Bar = bar }

func (a *App) ChromeNow() layout.Chrome { return a.ChromeState }

func (a *App) SetChrome(ch layout.Chrome) { a.ChromeState = ch }

func (a *App) MotionProgress(c *ui.Context, hidden bool) float32 {
	return ui.Column(c.Key("sidebar")).AnimateTo("shown", layout.B2F(!hidden), layout.SidebarMotion)
}

func (a *App) SidebarHidden() bool { return a.Prefs.SidebarHidden }

func (a *App) SidebarW() float32 { return a.Prefs.SidebarWidth }

func (a *App) SideRowW(winW float32) float32 { return sideRowW(winW, a.ChromeState) }

func (a *App) CrowdSide(winW float32) {
	a.Side.Crowded = winW > 0 && a.SideRowW(winW) < side.MinCards
	a.Side.RowW = a.SideRowW(winW)
}

func (a *App) SideCrowded() bool { return a.Side.Crowded }

func (a *App) UnforcePanel() { a.Side.Forced = false }

func (a *App) SidebarTakesFiles() bool {
	h := a.local()
	return h.Conn == host.StateReady && h.Client != nil && !a.Prefs.SidebarHidden && !a.Pal.Open && !a.Picker.Open &&
		a.SshAsk == nil && a.K8sAsk == nil && (a.Confirm == nil || !a.Confirm.Open)
}

func (a *App) SidebarDropped(files []string) {
	h := a.local()
	go a.openProjectDirs(h, h.Client, slices.Clone(files), true)
}

func (a *App) DrawSidebarArt(c *ui.Context) { a.sidebarArtView(c) }

func (a *App) ResizeHandle(c *ui.Context, pal *theme.Palette) { a.resizeHandle(c, pal) }

func (a *App) Sidebar(c *ui.Context, pal *theme.Palette, bar ui.TitleBar) { a.sidebarView(c, pal, bar) }

func (a *App) CardFor(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, id string) {
	layout.Card(a, c, pal, bar, shown, id)
}

func (a *App) StagedPages(c *ui.Context) {
	for id, e := range a.Webs {
		if e.Staged && e.View != nil {
			ui.Box(c.Key("staged-" + id)).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Stretch).Opacity(0).Children(func() {
				ui.WebView(c.Key("web"), e.View).Grow(1)
			})
		}
	}
}

func (a *App) Palette(c *ui.Context, pal *theme.Palette)    { a.paletteView(c, pal) }
func (a *App) PickerView(c *ui.Context, pal *theme.Palette) { a.pickerView(c, pal) }
func (a *App) ConfirmDialog(c *ui.Context, pal *theme.Palette) {
	a.confirmView(c, pal)
}

// Worktree draws the ask of a new session's branch.
func (a *App) Worktree(c *ui.Context, pal *theme.Palette) { worktree.ViewOf(a, a.WtAsk, c, pal) }

// The ask of a new session's branch, as worktree asks it.
func (a *App) AskNew(w *worktree.Ask) { a.WtAsk = w }

func (a *App) AskClose(cf *confirm.Ask) { a.Confirm = cf }

func (a *App) SelectedHost() *host.Host { return a.selectedHost() }

func (a *App) EndSession(id string, p proto.CloseParams) { a.endSession(id, p) }
func (a *App) SSH(c *ui.Context, pal *theme.Palette)     { a.sshView(c, pal) }
func (a *App) K8s(c *ui.Context, pal *theme.Palette)     { a.k8sView(c, pal) }

// Undo is a removal's words and what puts back what it took away.
func (a *App) Undo() (msg, undo string, act func(), ok bool) {
	if a.UndoAction == nil {
		return "", "", nil, false
	}
	msg, undo, act = a.UndoMessage, locales.CommonUndo(), a.UndoAction
	a.PendingToast, a.UndoAction, a.UndoMessage = "", nil, ""
	return msg, undo, act, true
}

// PendingToast is a line to show, and clears it.
func (a *App) TakeToast() (string, bool) {
	if a.PendingToast == "" {
		return "", false
	}
	msg := a.PendingToast
	a.PendingToast = ""
	return msg, true
}

func (a *App) EndFrame(c *ui.Context) {
	// A preview that showed no more fades in anew next time.
	if !a.PreviewDrawn {
		a.PreviewPane = ""
	}
	a.PreviewDrawn = false
	// An image previewed no more closes, and lets go of its bitmap.
	a.endPeek()
	if later := a.Later; len(later) > 0 {
		a.Later = nil
		for _, fn := range later {
			fn()
		}
		c.Invalidate()
	}
}

func (a *App) view(c *ui.Context) { root.View(a, c) }

// Create starts a session, as the worktree dialog's.
func (a *App) Create(h *host.Host, p proto.CreateParams, project string) { a.create(h, p, project) }

// The quit's answers, as internal/app/quit asks them.
func (a *App) Asking(on bool) { a.quitting = on }

func (a *App) AskingNow() bool { return a.quitting }

func (a *App) ConfirmAsk(cf *confirm.Ask) { a.Confirm = cf }

func (a *App) Seen(h *host.Host) bool { return h.Loaded }

func (a *App) Busy(h *host.Host) int { return a.busyCount(h) }

func (a *App) Hosts() []*host.Host { return a.Machines }

func (a *App) BringToFront() { a.bringToFront() }

func (a *App) Leaving() { a.quittingApp = true }

func (a *App) ArmQuitTimer() { a.armQuitTimer() }

func (a *App) EndSessionsAndQuit() { a.endSessionsAndQuit() }

func (a *App) SessionsOn(h *host.Host) []proto.Session { return a.sessionsOn(h) }
