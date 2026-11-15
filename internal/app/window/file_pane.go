package window

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"hyla/internal/app/filepane"
	"hyla/internal/proto"
	"hyla/internal/ui/files"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// A file pane shows a file of its host in MyGo's editor, colored by its
// language, as a terminal shows a program's cells: the daemon keeps the
// path, and the app reads the file through fs.read as the Files view does.

// filePaneFor returns a pane's file state, made as the pane first shows.
func (a *App) filePaneFor(p *proto.Pane) *filepane.State {
	return a.Files.Of(p.ID, p.File)
}

// FilePane shows the file of a file pane, and reads it as it first shows
// or changes on disk.
func (a *App) FilePane(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	f := a.filePaneFor(p)
	filepane.Read(f, a.hostOf(p.ID).Client, a.Update, files.RemoteError)
	ui.Box(c.Key("body")).Grow(1).MinHeight(0).Children(func() {
		switch {
		case f.Err != "":
			a.PaneMessage(c, pal, locales.FilesCantRead(), f.Err, "", nil)
		case f.Binary:
			a.PaneMessage(c, pal, locales.FilesBinary(), f.Path, "", nil)
		case f.Large:
			a.PaneMessage(c, pal, locales.FilesLarge(), f.Path, "", nil)
		case f.Busy || f.St == nil && f.BM == nil:
			ui.Column(c).Fill().Center().Children(func() { ui.Spinner(c) })
		case f.BM != nil:
			ui.Column(c.Key("image")).Fill().Padding(8).Center().Children(func() {
				ui.Image(c, f.BM).Shrink(1)
			})
		default:
			st := f.St
			st.ReadOnly = true
			st.SoftWrap = true
			st.LineNumbers = editor.LineNumbersOn
			st.Font = ui.Font{Family: a.termFamily(), Size: max(a.settingsIn.FontSize-1, 10)}
			st.Theme = a.codeTheme(c, pal)
			ed := editor.View(c.Key("file-"+p.ID), st).Fill()
			if ed.FocusWithin() && a.focusOf(s) != p.ID {
				a.setFocus(s, p.ID)
			}
		}
	})
}
