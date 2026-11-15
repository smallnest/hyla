package main

import (
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	ui "github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/theme"
)

func (a *App) SwitchIn() bool { return a.SwitchInState }

func (a *App) SwitchFrom() ui.Motion { return a.SwitchFromMotion }

func (a *App) ChromeForPanel() panes.Chrome {
	return panes.Chrome{SideW: a.ChromeState.sideW, MainPad: a.ChromeState.mainPad, HeaderPad: a.ChromeState.headerPad, Lead: lead(a.Bar)}
}

func (a *App) FeishuOn() bool { return panes.FeishuOn(a.settingsIn) }

func (a *App) PaneChat() map[string]bool { return a.PaneChatMap }

func (a *App) ShowPaneChat(id string) {
	if a.PaneChatMap == nil {
		a.PaneChatMap = map[string]bool{}
	}
	a.PaneChatMap[id] = true
}

func (a *App) HidePaneChat(id string) { delete(a.PaneChatMap, id) }

func (a *App) FilePane(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	a.filePane(c, pal, s, p)
}

func (a *App) PaneBrowser(c *ui.Context, pal *theme.Palette, bar ui.TitleBar, shown float32, s *proto.Session, p *proto.Pane, top, left, right, multi, focused, hovered bool) bool {
	return a.paneBrowser(c, pal, bar, shown, s, p, top, left, right, multi, focused, hovered)
}

func (a *App) ChatBody(c *ui.Context, pal *theme.Palette, s *proto.Session) { a.chatBody(c, pal, s) }

func (a *App) Term(pane string) *panes.TermEntry {
	e := a.Terms[pane]
	if e == nil {
		return nil
	}
	return &panes.TermEntry{Term: e.term, Err: e.err, Started: e.started}
}

func (a *App) ImagePeek(pane, link string, at, term ui.Rect) { a.imagePeek(pane, link, at, term) }

func (a *App) FindPane() string { return a.Find.Pane }

func (a *App) FindBar(c *ui.Context, pal *theme.Palette, p *proto.Pane, t *terminal.Terminal) {
	a.findBar(c, pal, p, t)
}

func (a *App) LoadPaneArt(path string) { a.loadPaneArt(path) }

func (a *App) PaneArt() panes.Art { return panes.Art{Path: a.PaneArtimage.Path, BM: a.PaneArtimage.BM} }

func (a *App) StartRename(id string) { a.startRename(id) }

func (a *App) RestartSession(id string) { a.restartSession(id) }

func (a *App) PasteDrop(pane string, files []string) { a.pasteDrop(pane, files) }

func (a *App) GitChip(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	a.gitChip(c, pal, s, p)
}

func (a *App) WindowCtl(c *ui.Context, pal *theme.Palette) { windowControls(c, pal, a.Win) }

func (a *App) ToggleSidebar() { a.toggleSidebar() }

func (a *App) NewSessionIn(projectID string) { a.newSession(projectID) }

func (a *App) HoverPane() string { return a.HoverPaneID }

func (a *App) SetHoverPane(id string) { a.HoverPaneID = id }

func (a *App) HoverAt() [2]float32 { return a.HoverAtPoint }

func (a *App) SetHoverAt(at [2]float32) { a.HoverAtPoint = at }

func (a *App) Arrived(pane string) bool { return a.ArrivedPanes[pane] }

func (a *App) TakeArrived(pane string) { delete(a.ArrivedPanes, pane) }

func (a *App) SetArrived(pane string) { a.ArrivedPanes[pane] = true }

func (a *App) LastZone() panes.Zone { return panes.Zone(a.LastDropZone) }

func (a *App) SetLastZone(z panes.Zone) { a.LastDropZone = z }

func (a *App) Preview() panes.Preview { return panes.Preview{Key: a.PreviewPane, Since: a.PreviewAt} }

func (a *App) SetPreview(pane string, since time.Time) {
	a.PreviewPane, a.PreviewAt = pane, since
}

func (a *App) DrawnPreview() { a.PreviewDrawn = true }

func (a *App) ShowPaneChatIn(s *proto.Session, id string) { panes.ShowPaneChatView(a, s, id) }

func (a *App) LayoutDrag(split string) (bool, float64) {
	d := a.SplitDrag
	if d.split != split {
		return false, 0
	}
	return true, d.ratio
}

func (a *App) LayoutDragKey() string { return a.SplitDrag.split }

func (a *App) SetLayoutDrag(session, split string, ratio float64) {
	d := &a.SplitDrag
	if d.split != split {
		d.session, d.split = session, split
	}
	d.ratio = ratio
}

func (a *App) ClearLayoutDrag() { a.SplitDrag.session, a.SplitDrag.split = "", "" }

// paneBackgroundOf is what a pane's card shows in place of its background.
func (a *App) paneBackgroundOf(pal *theme.Palette, bg ui.Color, p *proto.Pane) ui.Material {
	return panes.PaneBackground(a, pal, bg, p)
}
