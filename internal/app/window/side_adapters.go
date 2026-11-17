package window

import (
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/settings"
	"hyla/internal/ui/layout"
	"hyla/internal/ui/side"
	"hyla/internal/ui/theme"
)

// The app, as the side panel asks it.
var _ side.App = (*App)(nil)

// The views' ids, as prefs.SideTab holds one.
const (
	sideChanges   = side.Changes
	sideFiles     = side.Files
	sideInfo      = side.Info
	sideSubagents = side.Subagents
)

// Panel is the side panel's state.
func (a *App) Panel() *side.State { return &a.Side }

// SideVisible is the view the panel shows.
func (a *App) SideVisible() *string { return &a.Prefs.Side }

// SideTabPref is the view the panel showed last.
func (a *App) SideTabPref() *string { return &a.Prefs.SideTab }

// SideWidthPref is the panel's width.
func (a *App) SideWidthPref() *float32 { return &a.Prefs.SideWidth }

// SettingsOf is the settings the panel draws with.
func (a *App) SettingsOf() *settings.Settings { return &a.settingsIn }

func (a *App) PaneRef(id string) *ui.Handle { return a.paneRef(id) }
func (a *App) Motion() ui.Animation         { return layout.SidebarMotion }

func (a *App) CardGapIn() float32 { return layout.CardGap }

func (a *App) HeaderHeightIn() float32 { return layout.HeaderH }

func (a *App) Gap() float32    { return sideGap }
func (a *App) sideTab() string { return side.Tab(a) }

func (a *App) toggleSide(view string) { side.Toggle(a, view) }

func (a *App) rightShown() bool { return side.RightShown(a) }

func (a *App) sideView() string { return side.View(a) }

func (a *App) showSide(view string) { side.Show(a, view) }

func (a *App) sideWidth(w float32) float32 { return side.Width(a.Prefs.SideWidth, sideGap, w) }

func (a *App) SideEdge(c *ui.Context, pal *theme.Palette, row float32) { side.Edge(a, c, pal, row) }

func sideRowW(winW float32, ch layout.Chrome) float32 {
	return side.RowW(winW, ch.SideW, ch.MainPad, layout.CardGap)
}
