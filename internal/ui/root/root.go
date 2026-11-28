// Package root draws the app's window: the row of the sidebar and the
// card, the pages a screenshot stages, the dialogs over them, and the
// toasts and previews that frame's end clears.
package root

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/theme"
)

// App is what the window asks of the app it draws.
type App interface {
	// AppSettings are the settings, and BorderlessTheme the theme a window's
	// edge-less content draws with.
	AppSettings() *settings.Settings
	BorderlessTheme(c *ui.Context, pal *theme.Palette) *ui.Theme
	// Shortcuts takes the frame's keyboard shortcuts, and Tick syncs this
	// machine's configuration.
	Shortcuts(c *ui.Context)
	Tick()
	// TitleBar keeps the window's title bar, as the frame drew it, and
	// Chrome is where the sidebar's slide leaves things, which SetChrome
	// keeps.
	TitleBar(bar ui.TitleBar)
	ChromeNow() layout.Chrome
	SetChrome(ch layout.Chrome)
	// MotionProgress is the sidebar's slide, as the sidebar's animation
	// brings it, hidden telling that the user hid the sidebar; the
	// "shown" key of the sidebar's column is what it animates.
	MotionProgress(c *ui.Context, hidden bool) float32
	// SidebarHidden reports that the sidebar is hidden, and SidebarWidth is
	// its width; SideRowW is the room the side panel leaves the panes, in a
	// window winW wide.
	SidebarHidden() bool
	SidebarW() float32
	// Crowd tells the side panel how crowded the window is, and Crowded
	// reports whether it is too narrow for both; UnforceSide lets the
	// panel close again.
	CrowdSide(winW float32)
	SideCrowded() bool
	UnforcePanel()
	// SidebarTakesFiles reports that the sidebar takes the files dropped
	// on it; SidebarDropped opens them as projects.
	SidebarTakesFiles() bool
	SidebarDropped(files []string)
	// SidebarArt draws the picture behind the sidebar, and Sidebar the
	// sidebar.
	DrawSidebarArt(c *ui.Context)
	Sidebar(c *ui.Context, pal *theme.Palette, bar ui.TitleBar)
	ResizeHandle(c *ui.Context, pal *theme.Palette)
	// CardFor draws the card of the session id, and SelectedID is it.
	CardFor(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, id string)
	SelectedID() string
	// Staged draws the pages a screenshot stages under the window's
	// content, and the dialogs their own methods.
	StagedPages(c *ui.Context)
	Palette(c *ui.Context, pal *theme.Palette)
	PickerView(c *ui.Context, pal *theme.Palette)
	ConfirmDialog(c *ui.Context, pal *theme.Palette)
	Worktree(c *ui.Context, pal *theme.Palette)
	SSH(c *ui.Context, pal *theme.Palette)
	K8s(c *ui.Context, pal *theme.Palette)
	Arena(c *ui.Context, pal *theme.Palette)
	MergeQueue(c *ui.Context, pal *theme.Palette)
	Checkpoint(c *ui.Context, pal *theme.Palette)
	DenyTeach(c *ui.Context, pal *theme.Palette)
	BrowserReplay(c *ui.Context, pal *theme.Palette)
	// Undo is a removal's words and what puts back what it took away, and
	// TakeToast a line to show: each clears what it returns.
	Undo() (msg, undo string, act func(), ok bool)
	TakeToast() (msg string, ok bool)
	// EndFrame ends the frame: a preview no pane showed goes, the image's
	// peek closes, and the actions the view asked for after it run.
	EndFrame(c *ui.Context)
}

// View draws the app's window.
func View(x App, c *ui.Context) {
	pal := theme.ForBackground(c, x.AppSettings().Background)
	c.SetTheme(x.BorderlessTheme(c, pal))
	if msg, undo, act, ok := x.Undo(); ok {
		// The removal's own words, even as another toast came first.
		c.ToastAction(msg, undo, act)
	} else if msg, ok := x.TakeToast(); ok {
		c.Toast(msg)
	}

	x.Shortcuts(c)
	x.Tick()

	bar := c.TitleBar()
	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	theme.Backdrop(c, pal, row, *x.AppSettings())
	// A curve, not a spring: a spring's long tail crawls under a pixel a
	// frame, which whole pixels turn into hops and holds, a shake as it
	// settles. This one lands in about 13 frames, moving in every one.
	var progress float32
	shown := x.ChromeNow().Shown
	// The panel is not crowded by a window too narrow for it beside the
	// panes, which it then leaves to them while it is open, unless the
	// user opens it anyway (sideState.forced). The row's width is this
	// frame's, as its bounds have it: the last frame's would lag a frame
	// as the window resizes.
	winW, _ := c.Size()
	x.TitleBar(bar)
	x.CrowdSide(winW)
	if !x.SideCrowded() {
		x.UnforcePanel()
	}
	row.Children(func() {
		progress = x.MotionProgress(c, x.SidebarHidden())
		x.SetChrome(layout.For(progress, x.SidebarW(), bar))
		w := x.ChromeNow().SideW
		side := ui.Row(c).Width(w).Shrink(0).AlignItems(ui.Stretch).ClipX()
		if x.SidebarTakesFiles() {
			if files := side.DroppedFiles(); len(files) > 0 {
				x.SidebarDropped(files)
			}
			if side.FileDragOver() {
				side.Background(pal.Accent.Alpha(0.08)).Draw(func(p *ui.Painter, r ui.Rect) {
					p.Stroke(ui.Rect{X: r.X + 4, Y: r.Y + 4, W: r.W - 8, H: r.H - 8}, pal.Accent.Alpha(0.55), 8, 1)
				})
			}
		}
		side.Children(func() {
			ui.Column(c).Width(x.SidebarW()).Shrink(0).Margin(0, 0, 0, w-x.SidebarW()).
				Opacity(0.4 + 0.6*shown).Children(func() {
				x.DrawSidebarArt(c)
				x.Sidebar(c, pal, bar)
			})
		})
		// The gaps around the cards move the window too.
		main := ui.Column(c).Grow(1).MinWidth(0).Padding(layout.CardGap, layout.CardGap, layout.CardGap, x.ChromeNow().MainPad).DragWindow()
		main.Children(func() {
			if shown > 0.01 {
				x.ResizeHandle(c, pal)
			}
			x.CardFor(c, pal, bar, shown, x.SelectedID())
		})
	})
	// A page staged for a screenshot takes the window's size, under its
	// content, which hides it: at no opacity, it paints no hole there.
	x.StagedPages(c)
	x.Palette(c, pal)
	x.PickerView(c, pal)
	x.ConfirmDialog(c, pal)
	x.Worktree(c, pal)
	x.SSH(c, pal) // the dialog that adds a machine
	x.K8s(c, pal)
	x.Arena(c, pal)
	x.MergeQueue(c, pal)
	x.Checkpoint(c, pal)
	x.DenyTeach(c, pal)
	x.BrowserReplay(c, pal)
	x.EndFrame(c)
}
