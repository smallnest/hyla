package main

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/files"
	"hyla/internal/ui/theme"
)

// The app, as the Files tab reads and runs what it shows.
var _ files.App = (*App)(nil)

func (a *App) FilesState() *files.State { return &a.Side.files }

// AskDelete asks to delete a file or folder, before the app does it.
func (a *App) AskDelete(title, detail, action string, run func()) {
	a.Confirm = &confirm.Ask{Open: true, Title: title, Detail: detail, Action: action, Destroy: true, Run: run}
}

func (a *App) SessionOfPane(id string) *proto.Session { return a.sessionOfPane(id) }

func (a *App) StartCreate(s *proto.Session, pane, dir, rel string, folder bool) {
	files.StartCreate(a, s, pane, dir, rel, folder)
}

func (a *App) EntryMenu(m *ui.Menu, s *proto.Session, pane string, it files.Item) {
	files.EntryMenu(a, m, s, pane, it)
}

func (a *App) OpenFileInPane(pane, file string) { files.OpenFileInPane(a, pane, file) }

func (a *App) OpenFileInEditor(pane, file string) { files.OpenFileInEditor(a, pane, file) }

func (a *App) TermFamily() string { return a.termFamily() }

func (a *App) FontSize() float32 { return a.settingsIn.FontSize }

// filesRoot is the folder the Files tab shows, and the pane it is of.
func (a *App) filesRoot(s *proto.Session) (*proto.Pane, string) { return files.Root(a, s) }

// filesSide is the Files tab.
func (a *App) filesSide(c *ui.Context, pal *theme.Palette, s *proto.Session) {
	files.Side(c, a, pal, s)
}

// fileViewer is the viewer of a file of Files.
func (a *App) fileViewer(c *ui.Context, pal *theme.Palette, s *proto.Session, fv *files.View) {
	files.Viewer(c, a, pal, s, fv)
}

func (a *App) filesItems(m *ui.Menu, s *proto.Session) { files.Items(a, m, s) }

// showText shows text in a file's viewer, as the viewer does when it reads
// the file, for the tests of it.
func showText(fv *files.View, text string) { files.ShowText(fv, text) }

// filesKeyShim keys a path of a host, as the tab does.
func filesKeyShim(h *host.Host, p string) string { return files.Key(h, p) }
