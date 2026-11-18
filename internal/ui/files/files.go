// Package files is the side panel's Files tab: the tree of the worktree
// of the pane the user is in, a folder at a time, the viewer of the file
// a click opens, and the making, renaming and deleting of them.
package files

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"hyla/internal/app/filepane"
	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/fileicons"
	"hyla/internal/platform"
	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// App is what the Files tab reads and runs, as the app gives it.
type App interface {
	// FilesState is the tab's state.
	FilesState() *State
	// Update runs fn on the app's own goroutine.
	Update(fn func())
	// Toast shows a message.
	Toast(msg string)
	// AskDelete asks to delete a file or folder.
	AskDelete(title, detail, action string, run func())
	// AfterFrame runs fn after the frame.
	AfterFrame(fn func())
	// HostOf is the host a session or a pane is of.
	HostOf(id string) *host.Host
	// FocusOf is the pane of a session the user is in.
	FocusOf(s *proto.Session) string
	// Session is a session the app knows, nil while none.
	Session(id string) *proto.Session
	// SessionOfPane is the session a pane is of, nil while none.
	SessionOfPane(id string) *proto.Session
	// SideMessage shows a word in the side panel.
	SideMessage(c *ui.Context, pal *theme.Palette, title, detail string)
	// StartCreate puts the field for a new file's name, or a folder's, at
	// the top of a folder.
	StartCreate(s *proto.Session, pane, dir, rel string, folder bool)
	// EntryMenu shows a row's menu.
	EntryMenu(m *ui.Menu, s *proto.Session, pane string, it Item)
	// PluginFileMenu appends enabled plugin actions targeting files.menu.
	PluginFileMenu(m *ui.Menu, s *proto.Session, pane string, it Item)
	// Split splits a pane, with the session's params.
	Split(s *proto.Session, p proto.CreateParams)
	// OpenFileInPane opens a file in a file pane beside a pane.
	OpenFileInPane(pane, file string)
	// OpenFileInEditor opens a file in the host's editor, in a pane.
	OpenFileInEditor(pane, file string)
	// TermFamily is the font family of the terminals.
	TermFamily() string
	// CodeTheme is the editor's theme.
	CodeTheme(c *ui.Context, pal *theme.Palette) *editor.Theme
	// FontSize is the terminals' font size.
	FontSize() float32
}

// The Files tab is the tree of the worktree of the pane the user is in,
// else of its directory, as hylad lists it (fs.list), a folder at a time
// as the user unfolds it: .git not at all, each file and folder with the
// icon of its kind. A click on a file opens it in the viewer over the
// tab, as text, or, an image, as an image; its header opens it in a file
// pane, in the host's editor, or in the OS app. The tab's menu makes
// files and folders, renames and deletes them, and sets the size of their
// text and the viewer's diff layout.

// State is the Files tab's.
type State struct {
	// dirs are the folders listed, open the folders unfolded, by host and
	// path (filesKey); places the trees' places, by host and root.
	Dirs   map[string]*DirList
	Open   map[string]bool
	Places map[string]*ui.ListState
	// gen counts the times the tab came to show, each of which lists its
	// folders anew; shown tells it shows this frame, and showed that it
	// did the frame before.
	Gen           int
	Shown, Showed bool
	// views are what the viewer shows, by session; edit is the name the
	// user types in the tree (files_ops.go).
	Views map[string]*View
	Edit  *NameEdit
}

// dirList is a folder as hylad listed it.
type DirList struct {
	Entries []proto.FSEntry
	Err     string
	Busy    bool
	Stamp   string // the worktree's status it was listed at (paints.Stamp)
	Gen     int
	Fetched time.Time
}

// fileItem is a row of the tree: a file or a folder, depth folders in, or
// why a folder shows nothing, err, or the field the user types a name in,
// edit.
type Item struct {
	proto.FSEntry
	Path      string // on the host
	Rel       string // in the root, with slashes
	Parent    string // the folder it is in, on the host
	ParentRel string
	Depth     int
	Open      bool
	Err       string
	Edit      bool
}

// fileView is a file the viewer shows, as its text, or as an image.
type View struct {
	Pane string // a pane of the host it is on
	Path string // on the host
	Rel  string // in the Files' root, which its header shows
	St   *editor.State
	BM   *ui.Bitmap // of an image
	Err  string
	// gone, large and binary say why it shows no text: there is no file
	// there any more, or one too large, or one that isn't text.
	Gone, Large, Binary bool
	Open                bool // the tab shows it over its tree
	Busy                bool
	Stamp               string
	Fetched             time.Time
}

// ErrGone is a file read that is no longer there.
var ErrGone = filepane.ErrGone

// filesKey keys a path of a host.
func Key(h *host.Host, p string) string { return h.Name + "\x00" + p }

// withGit gives a pane its session's worktree, the one that holds its
// directory, when the pane has none itself.
func WithGit(s *proto.Session, p *proto.Pane) *proto.Pane {
	if p == nil || p.Git != nil {
		return p
	}
	dir := p.WorkDir()
	if dir == "" {
		return p
	}
	for i := range s.Panes {
		if g := s.Panes[i].Git; g != nil && WithinDir(dir, g.Root) {
			cp := *p
			cp.Git = g
			return &cp
		}
	}
	return p
}

// withinDir tells that dir is root or a directory in it, with the paths
// of any host.
func WithinDir(dir, root string) bool {
	root = strings.TrimRight(root, `/\`)
	if dir == root {
		return true
	}
	return strings.HasPrefix(dir, root+"/") || strings.HasPrefix(dir, root+`\`)
}

// sessionWorking tells that an agent of the session works.
func SessionWorking(s *proto.Session) bool {
	for _, p := range s.Panes {
		if p.Status == proto.Working {
			return true
		}
	}
	return false
}

// filesRoot is the folder the Files tab shows, and the pane it is of: the
// worktree of the one the user is in, else its directory.
func Root(x App, s *proto.Session) (*proto.Pane, string) {
	p := WithGit(s, s.PaneByID(x.FocusOf(s)))
	switch {
	case p == nil:
		return nil, ""
	case p.Git != nil:
		return p, p.Git.Root
	case p.ProcessDir != "":
		return p, p.ProcessDir
	}
	return p, p.Dir
}

// filesSide is the Files tab: the root's tree, a folder listed as it
// unfolds, and again as the worktree changes.
func Side(c *ui.Context, x App, pal *theme.Palette, s *proto.Session) {
	fs := x.FilesState()
	if !fs.Showed {
		fs.Gen++
	}
	fs.Shown, fs.Showed = true, true
	p, root := Root(x, s)
	if root == "" {
		x.SideMessage(c, pal, locales.FilesNoFolder(), locales.FilesNoFolderDetail())
		return
	}
	h := x.HostOf(p.ID)
	stamp := paints.Stamp(p.Git)
	if fs.Dirs == nil || len(fs.Dirs) > 2000 {
		fs.Dirs = map[string]*DirList{}
	}
	// A folder with no stamp is one the tab listed before, which a worktree
	// root's, stamped, must not look newer than.
	for _, l := range fs.Dirs {
		if l.Stamp == "" {
			l.Stamp = stamp
		}
	}
	if fs.Open == nil {
		fs.Open = map[string]bool{}
	}
	working := SessionWorking(s)
	if working {
		c.After(5 * time.Second)
	}
	asking := 0
	for _, l := range fs.Dirs {
		if l.Busy {
			asking++
		}
	}
	// The folders shown, listed again as the worktree's status changes,
	// as the tab shows anew, and every few seconds while an agent works,
	// a few at a time.
	list := func(dir string) *DirList {
		l := fs.Dirs[Key(h, dir)]
		stale := l == nil || !l.Busy && (l.Stamp != stamp || l.Gen != fs.Gen || working && time.Since(l.Fetched) > 5*time.Second)
		if stale && asking < 4 && h.Client != nil {
			asking++
			l = listingDir(x, h, dir, stamp)
		}
		return l
	}
	var items []Item
	var walk func(dir, rel string, depth int)
	walk = func(dir, rel string, depth int) {
		l := list(dir)
		// A new file's or folder's name goes at the top of its folder,
		// its host's, which the hosts named alike are.
		if e := fs.Edit; e != nil && e.Op != EditRename && x.HostOf(e.Pane) == h && e.Dir == dir {
			items = append(items, Item{FSEntry: proto.FSEntry{Dir: e.Op == EditNewFolder}, Rel: rel + "/+", Parent: dir, ParentRel: rel, Depth: depth, Edit: true})
		}
		if l == nil || l.Entries == nil && l.Err == "" {
			return
		}
		if l.Err != "" && l.Entries == nil {
			items = append(items, Item{Rel: rel + "/!", Depth: depth, Err: l.Err})
			return
		}
		for _, e := range l.Entries {
			if e.Dir && e.Name == ".git" {
				continue
			}
			it := Item{FSEntry: e, Rel: path.Join(rel, e.Name), Path: h.JoinPath(dir, e.Name), Parent: dir, ParentRel: rel, Depth: depth}
			it.Open = e.Dir && fs.Open[Key(h, it.Path)]
			it.Edit = fs.Edit != nil && fs.Edit.Op == EditRename && fs.Edit.Host == h.Name && fs.Edit.Path == it.Path
			items = append(items, it)
			if it.Open {
				walk(it.Path, it.Rel, depth+1)
			}
		}
	}
	walk(root, "", 0)

	ui.Row(c.Key("root")).Height(28).Shrink(0).Padding(0, 10, 6, 16).Gap(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, h.ShortDir(root)).FontSize(12.5).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").MinWidth(0).Grow(1).Tooltip(root)
		paints.IconButton(c, "new-file", glyph.Doc, locales.FilesNewFile(), pal, func() { x.StartCreate(s, p.ID, root, "", false) })
		paints.IconButton(c, "new-folder", glyph.Folder, locales.FilesNewFolder(), pal, func() { x.StartCreate(s, p.ID, root, "", true) })
		paints.IconButton(c, "fold-folders", glyph.Collapse, locales.FilesCollapseFolders(), pal, func() {
			prefix := Key(h, root)
			for k := range fs.Open {
				if strings.HasPrefix(k, prefix) {
					delete(fs.Open, k)
				}
			}
		})
	})
	// A folder the tab listed before is one it knows, whatever lists it
	// again: it shows its entries until they come.
	rootListed := func(l *DirList) bool { return l != nil && (l.Entries != nil || l.Fetched.IsZero()) }
	if l := fs.Dirs[Key(h, root)]; !rootListed(l) {
		switch {
		case l != nil && l.Err != "" && !l.Busy:
			x.SideMessage(c, pal, locales.FilesCantReadFolder(), l.Err)
		case h.Client != nil:
			ui.Column(c).Grow(1).Center().Children(func() { ui.Spinner(c) })
		}
		return
	}

	if fs.Places == nil {
		fs.Places = map[string]*ui.ListState{}
	}
	ls := fs.Places[Key(h, root)]
	if ls == nil {
		ls = &ui.ListState{}
		fs.Places[Key(h, root)] = ls
	}
	var shown string
	if v := fs.Views[s.ID]; v != nil {
		shown = v.Path
	}
	ls.Key = func(i int) any { return treeKey{items[i].Rel, items[i].Dir, items[i].Path == shown} }
	sid, pid := s.ID, p.ID
	ui.List(c.Key("files"), ls, len(items), func(i int) {
		it := items[i]
		switch {
		case it.Edit:
			nameRow(c, x, pal, it)
		case it.Err != "":
			ui.Text(c, it.Err).FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Padding(4, 8, 4, 6+12*float32(it.Depth)+28).Tooltip(it.Err)
		case it.Dir:
			key := Key(h, it.Path)
			row := treeFolder(c, pal, it.Name, it.Rel, it.Depth, it.Open, false, func() {
				if it.Open {
					delete(fs.Open, key)
				} else {
					fs.Open[key] = true
				}
			})
			row.ContextMenu(func(m *ui.Menu) { x.EntryMenu(m, s, pid, it) })
		default:
			row := treeEntry(c, pal, it, it.Path == shown, func() {
				s := x.Session(sid)
				if s == nil {
					return
				}
				if fs.Views == nil {
					fs.Views = map[string]*View{}
				}
				if fv := fs.Views[sid]; fv != nil && fv.Path == it.Path {
					// The one it shows: open, or hidden as its button
					// hid it.
					openViewer(x, fv)
					return
				}
				fv := &View{Pane: pid, Path: it.Path, Rel: it.Rel, Open: true}
				fs.Views[sid] = fv
				openViewer(x, fv)
			})
			row.ContextMenu(func(m *ui.Menu) { x.EntryMenu(m, s, pid, it) })
		}
	}).Grow(1).MinHeight(0).Padding(0, 10, 16, 10)
}

// treeKey is what a row of a tree is, as its list keeps its state by.
type treeKey struct {
	path    string
	dir     bool
	current bool
}

// treeRow is a row of a tree, depth folders in, the one there chosen.
func treeRow(c *ui.Context, pal *theme.Palette, depth int, current bool) ui.Element {
	row := ui.Row(c.Key("row")).Height(26).Padding(0, 8, 0, 6+12*float32(depth)).Gap(5).AlignItems(ui.Center).Radius(7)
	if pal.Dark {
		// Room for the chosen row's edge in every row.
		row.Border(1, ui.Transparent)
	}
	switch {
	case current:
		row.Background(pal.RowSelected)
		style.Chosen(row, pal)
	case row.Hovered():
		row.Background(pal.RowHover)
	}
	return row
}

// treeFolder is a folder of a tree, which a click folds or unfolds.
func treeFolder(c *ui.Context, pal *theme.Palette, name, tip string, depth int, open, faded bool, toggle func()) ui.Element {
	row := treeRow(c, pal, depth, false).Tooltip(tip)
	if faded {
		row.Opacity(0.55)
	}
	if row.Clicked() {
		toggle()
	}
	row.Children(func() {
		chev := ui.Icon(c, glyph.Chevron).Size(9, 9).TextColor(pal.TextFaint).Shrink(0)
		chev.Rotate(chev.AnimateTo("turn", 90*b2f(open), ui.Snappy))
		fileicons.Image(c, name, true, open, pal.Dark)
		ui.Text(c, name).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").MinWidth(0)
	})
	return row
}

// treeEntry is a file of the Files tab's tree, with its icon, which a
// click opens in the viewer.
func treeEntry(c *ui.Context, pal *theme.Palette, it Item, current bool, open func()) ui.Element {
	row := treeRow(c, pal, it.Depth, current).Tooltip(it.Rel)
	if row.Clicked() {
		open()
	}
	row.Children(func() {
		ui.Box(c).Size(9, 9).Shrink(0)
		fileicons.Image(c, it.Name, false, false, pal.Dark)
		ui.Text(c, it.Name).FontSize(12.5).TextColor(pal.Text).SingleLine().Ellipsis("…").MinWidth(0)
	})
	return row
}

// fetchDir asks hylad for a folder's entries, dotfiles too.
func fetchDir(x App, h *host.Host, dir string, stamp string) *DirList {
	fs := x.FilesState()
	key := Key(h, dir)
	l := fs.Dirs[key]
	if l == nil {
		l = &DirList{}
		fs.Dirs[key] = l
	}
	l.Busy = true
	gen, cl := fs.Gen, h.Client
	go func() {
		var res proto.FSList
		err := cl.Call("fs.list", proto.FSListParams{Dir: dir, Hidden: true}, &res)
		x.Update(func() {
			l.Busy, l.Fetched, l.Stamp, l.Gen = false, time.Now(), stamp, gen
			if err != nil {
				l.Err = err.Error()
				return
			}
			l.Entries, l.Err = res.Entries, ""
		})
	}()
	return l
}

// remoteError says what a call to a host's hylad came to: a method its
// hylad has not is told to update.
func RemoteError(err error) string {
	if strings.Contains(err.Error(), "unknown method") {
		return locales.FilesUpdateHost()
	}
	return err.Error()
}

// openViewer opens a file in the viewer, which reads it.
func openViewer(x App, fv *View) {
	if !fv.Open {
		fv.Open, fv.Fetched = true, time.Time{} // read anew
	}
	if cl := x.HostOf(fv.Pane).Client; cl != nil && !fv.Busy && fv.Fetched.IsZero() {
		loadFile(x, cl, fv, "")
	}
}

// fileViewer is the viewer of a file of Files: an image as an image, else
// its text, read only, which it reads again as the worktree changes, and
// every few seconds while an agent works.
func Viewer(c *ui.Context, x App, pal *theme.Palette, s *proto.Session, fv *View) {
	h := x.HostOf(fv.Pane)
	stamp := ""
	if p := WithGit(s, s.PaneByID(fv.Pane)); p != nil {
		stamp = paints.Stamp(p.Git)
	}
	working := SessionWorking(s)
	if working {
		c.After(3 * time.Second)
	}
	if !fv.Busy && h.Client != nil && (fv.Fetched.IsZero() || fv.Stamp != stamp || working && time.Since(fv.Fetched) > 3*time.Second) {
		loadFile(x, h.Client, fv, stamp)
	}

	dir, name := path.Split(fv.Rel)
	ui.Row(c.Key("hdr")).Height(headerH).Shrink(0).AlignItems(ui.Center).Padding(0, 10, 0, 10).Gap(8).Children(func() {
		fileicons.Image(c, name, false, false, pal.Dark)
		ui.Text(c, name).FontSize(14).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		if dir = strings.TrimSuffix(dir, "/"); dir != "" {
			ui.Text(c, dir).FontSize(13).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(2).MinWidth(0).Tooltip(fv.Path)
		}
		ui.Box(c.Key("drag")).Grow(1).AlignSelf(ui.Stretch).DragWindow()
		if !fv.Gone {
			pid, file := fv.Pane, fv.Path
			paints.IconButton(c, "files-edit", glyph.Doc, locales.FilesOpenInPane(), pal, func() { x.AfterFrame(func() { x.OpenFileInPane(pid, file) }) })
			paints.IconButton(c, "files-editor", glyph.SplitRight, locales.FilesOpenInEditor(), pal, func() { x.AfterFrame(func() { x.OpenFileInEditor(pid, file) }) })
		}
		if !fv.Gone {
			full := fv.Path
			paints.IconButton(c, "open", glyph.OpenFile, locales.ChangesOpenFile(), pal, func() { platform.OpenPath(full) })
		}
		sid := s.ID
		paints.IconButton(c, "files-close", glyph.Close, locales.SideClose(), pal, func() {
			x.AfterFrame(func() {
				if fv := x.FilesState().Views[sid]; fv != nil {
					fv.Open = false
				}
			})
		})
	})
	ui.Column(c.Key("body")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		switch {
		case fv.Gone:
			x.SideMessage(c, pal, locales.FilesGone(), fv.Rel)
		case fv.Large:
			x.SideMessage(c, pal, locales.ChangesTooLarge(), "")
		case fv.Binary:
			x.SideMessage(c, pal, locales.ChangesBinary(), "")
		case fv.St == nil && fv.Err != "":
			x.SideMessage(c, pal, locales.FilesCantReadFile(), fv.Err)
		case fv.BM != nil:
			ui.Column(c.Key("image")).Grow(1).MinHeight(0).Padding(8).Center().Children(func() {
				ui.Image(c, fv.BM).Shrink(1)
			})
		case fv.St == nil:
			ui.Column(c).Grow(1).Center().Children(func() { ui.Spinner(c) })
		default:
			fv.St.Font = ui.Font{Family: x.TermFamily(), Size: max(x.FontSize()-1, 10)}
			fv.St.Theme = x.CodeTheme(c, pal)
			fv.St.ReadOnly = true
			fv.St.NoActiveLine = true
			fv.St.NoScrollPastEnd = true
			editor.View(c.Key("text-"+fv.Path), fv.St).Grow(1).FillWidth().MinHeight(0)
		}
	})
}

// loadFile reads a file of the viewer: an image to show, else its text,
// anew for its first, else as an edit, so that it stays scrolled where it
// was.
func loadFile(x App, cl *client.Client, fv *View, stamp string) {
	fv.Busy = true
	image := ImageExt[strings.ToLower(filepath.Ext(fv.Path))]
	go func() {
		var st proto.FSStat
		var data []byte
		var bm *ui.Bitmap
		limit := int64(filepane.MaxView)
		if image {
			limit = filepane.MaxImage
		}
		err := cl.Call("fs.stat", proto.FSStatParams{Path: fv.Path}, &st)
		if err == nil && st.Exists && !st.Dir && st.Size <= limit {
			var r proto.FSRead
			err = cl.Call("fs.read", proto.FSReadParams{Path: fv.Path, Max: limit}, &r)
			data = r.Data
			if err == nil && image {
				bm, err = ui.DecodeBitmap(data)
				image = bm != nil // else its bytes as text
			}
		}
		x.Update(func() {
			fv.Busy, fv.Fetched, fv.Stamp = false, time.Now(), stamp
			if err != nil {
				fv.Err = err.Error()
				return
			}
			fv.Err = ""
			fv.Gone = !st.Exists || st.Dir
			fv.Large = !fv.Gone && st.Size > limit
			fv.Binary = !fv.Gone && !fv.Large && !image && filepane.IsBinary(data)
			if fv.Gone || fv.Large || fv.Binary {
				fv.BM = nil
				return
			}
			if image {
				fv.BM, fv.St = bm, nil
				return
			}
			fv.BM = nil
			ShowText(fv, string(data))
		})
	}()
}

// showFileText puts a file's text in its viewer's editor, read only.
func ShowText(fv *View, text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if fv.St == nil {
		buf := editor.NewBuffer(text)
		buf.SetLanguage(editor.LanguageFor(path.Base(fv.Rel)))
		fv.St = &editor.State{Buffer: buf, ReadOnly: true}
		fv.St.NoActiveLine = true
		return
	}
	paints.ReplaceText(fv.St.Buffer, text)
}

// openFileInPane opens a file in a file pane beside a pane of its host,
// which shows its text, or its image.
func OpenFileInPane(x App, pane, file string) {
	s := x.SessionOfPane(pane)
	if s == nil {
		return
	}
	x.Split(s, proto.CreateParams{Split: pane, Direction: "right", File: file})
}

// openFileInEditor opens a file in the host's editor, in a pane of its
// shell.
func OpenFileInEditor(x App, pane, file string) {
	s := x.SessionOfPane(pane)
	if s == nil {
		return
	}
	x.Split(s, proto.CreateParams{Split: pane, Direction: "right", Command: host.EditorCommand(x.HostOf(pane), file)})
}

// --- the tab's menu ---

func Items(x App, m *ui.Menu, s *proto.Session) {
	pane, root := Root(x, s)
	if pane == nil {
		return
	}
	if m.Item(locales.FilesNewFile()).Chosen() {
		x.StartCreate(s, pane.ID, root, "", false)
	}
	if m.Item(locales.FilesNewFolder()).Chosen() {
		x.StartCreate(s, pane.ID, root, "", true)
	}
	if m.Item(locales.FilesCollapseFolders()).Chosen() {
		prefix := Key(x.HostOf(pane.ID), root)
		for k := range x.FilesState().Open {
			if strings.HasPrefix(k, prefix) {
				delete(x.FilesState().Open, k)
			}
		}
	}
}

// The Files tab makes files and folders, renames and deletes them, as
// VS Code's explorer does: New File and New Folder, in its menu for the
// root and in a row's menu for its folder, put a field for the name in
// the tree, where the entry will be, which Return makes, as does leaving
// it, and Escape leaves; a name with slashes makes the folders in it.
// Rename puts the field in the entry's place. Delete asks first, and moves
// a file of this machine to the trash, but deletes a remote host's for
// good, as hylad has no trash there to move it to.

const (
	EditNewFile   = "file"
	EditNewFolder = "folder"
	EditRename    = "rename"
)

// NameEdit is the name the user types in the tree: of a file or a folder
// to make in dir, or for an entry to rename.
type NameEdit struct {
	Op      string // EditNewFile, EditNewFolder or EditRename
	Session string
	Pane    string // a pane of the host
	Host    string // its name
	Dir     string // the folder it is in, on the host
	Rel     string // that folder's, in the root
	Path    string // the entry renamed, and its rel
	OldRel  string
	DirEnt  bool // the entry renamed is a folder
	Name    string
	Took    bool // it took the focus
	Miss    int
}

// startCreate puts the field for a new file's name, or a new folder's, at
// the top of a folder, which it unfolds.
func StartCreate(x App, s *proto.Session, pane, dir, rel string, folder bool) {
	h := x.HostOf(pane)
	op := EditNewFile
	if folder {
		op = EditNewFolder
	}
	x.FilesState().Edit = &NameEdit{Op: op, Session: s.ID, Pane: pane, Host: h.Name, Dir: dir, Rel: rel}
	x.FilesState().Open[Key(h, dir)] = true
	// The field shows at once, as its click came in a button, which the
	// tree then shows without; it takes the focus, and the row clicked
	// doesn't.
	x.Update(func() {})
}

// startRenameEntry puts the field for an entry's new name in its place.
func startRenameEntry(x App, s *proto.Session, pane string, it Item) {
	x.FilesState().Edit = &NameEdit{Op: EditRename, Session: s.ID, Pane: pane, Host: x.HostOf(pane).Name, Dir: it.Parent, Rel: it.ParentRel,
		Path: it.Path, OldRel: it.Rel, DirEnt: it.Dir, Name: it.Name}
}

// nameRow is the field the user types a name in, in the tree.
func nameRow(c *ui.Context, x App, pal *theme.Palette, it Item) {
	fs := x.FilesState()
	e := fs.Edit
	row := treeRow(c, pal, it.Depth, true)
	row.Children(func() {
		ui.Box(c).Size(9, 9).Shrink(0)
		// The icon of the name as typed.
		fileicons.Image(c, strings.TrimSpace(e.Name), e.Op == EditNewFolder || e.Op == EditRename && e.DirEnt, false, pal.Dark)
		in := ui.TextInputBase(c.Key("name"), &e.Name).Grow(1).MinWidth(0).FontSize(12.5).Label(it.Name)
		// The field takes the focus in its first frame, which a click
		// left on a button made before this one; it then leaves it only
		// when the user does.
		if in.Focused() {
			e.Took = true
		} else {
			e.Miss++
		}
		in.AutoFocus()
		switch {
		case in.Shortcut(0, ui.KeyEscape):
			x.Toast(fmt.Sprint("FIELDSTATE notfocused=", e.Miss, " took=", e.Took))
			fs.Edit = nil
		case in.Submitted():
			commitName(x)
		case e.Took && !in.Focused() && !in.Pressed() && in.Valid():
			// Leaving it makes the entry.
			x.Toast(fmt.Sprint("FIELDSTATE left notfocused=", e.Miss, " took=", e.Took))
			commitName(x)
		}
	})
}

// commitName makes the file or folder named, or renames the entry.
func commitName(x App) {
	fs := x.FilesState()
	e := fs.Edit
	fs.Edit = nil
	if e == nil {
		return
	}
	name := strings.TrimSpace(e.Name)
	if name == "" || e.Op == EditRename && name == path.Base(e.OldRel) {
		return
	}
	// A new entry's name may name folders too; a renamed one's not.
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' })
	bad := len(parts) == 0 || e.Op == EditRename && len(parts) > 1
	for _, p := range parts {
		bad = bad || p == "." || p == ".."
	}
	if bad {
		x.Toast(locales.FilesBadName(name))
		return
	}
	h := x.HostOf(e.Pane)
	cl := h.Client
	if cl == nil {
		return
	}
	to := e.Dir
	for _, p := range parts {
		to = h.JoinPath(to, p)
	}
	rel := path.Join(append([]string{e.Rel}, parts...)...)
	go func() {
		var err error
		if e.Op == EditRename {
			err = cl.Call("fs.rename", proto.FSRenameParams{From: e.Path, To: to}, nil)
		} else {
			err = cl.Call("fs.create", proto.FSCreateParams{Path: to, Dir: e.Op == EditNewFolder}, nil)
		}
		x.Update(func() {
			if err != nil {
				msg := locales.FilesCreateFailed(name, RemoteError(err))
				if e.Op == EditRename {
					msg = locales.FilesRenameFailed(path.Base(e.OldRel), RemoteError(err))
				}
				x.Toast(msg)
				return
			}
			fs.Gen++ // the folders list anew
			switch e.Op {
			case EditRename:
				renamed(x, h, e.Path, to, e.OldRel, rel)
			case EditNewFolder:
				fs.Open[Key(h, to)] = true
			default:
				// A new file opens, as the user is to write it.
				if s := x.Session(e.Session); s != nil {
					if fs.Views == nil {
						fs.Views = map[string]*View{}
					}
					fv := &View{Pane: e.Pane, Path: to, Rel: rel}
					fs.Views[s.ID] = fv
					openViewer(x, fv)
				}
			}
		})
	}()
}

// renamed follows an entry of a host renamed, from to to: the files the
// viewers show in it, and its folders unfolded.
func renamed(x App, h *host.Host, from, to, fromRel, toRel string) {
	moved := func(p string) (string, bool) {
		switch {
		case p == from:
			return to, true
		case strings.HasPrefix(p, from) && strings.ContainsAny(p[len(from):len(from)+1], `/\`):
			return to + p[len(from):], true
		}
		return p, false
	}
	for _, fv := range x.FilesState().Views {
		if fv == nil || x.HostOf(fv.Pane) != h {
			continue
		}
		if p, ok := moved(fv.Path); ok {
			fv.Path = p
			fv.Rel = toRel + strings.TrimPrefix(fv.Rel, fromRel)
		}
	}
	fs := x.FilesState()
	prefix := Key(h, "")
	for k := range fs.Open {
		if p, ok := moved(strings.TrimPrefix(k, prefix)); ok && strings.HasPrefix(k, prefix) {
			delete(fs.Open, k)
			fs.Open[Key(h, p)] = true
		}
	}
}

// askDelete asks to delete a file or folder: to the trash for one of this
// machine, for good for a remote host's.
func askDelete(x App, pane string, it Item) {
	h := x.HostOf(pane)
	title := locales.FilesDeleteFileTitle(it.Name)
	if it.Dir {
		title = locales.FilesDeleteFolderTitle(it.Name)
	}
	detail, action := locales.FilesDeleteTrashDetail(), locales.FilesMoveToTrash()
	x.AskDelete(title, detail, action, func() { deleteEntry(x, h, it) })
}

// deleteEntry deletes a file or folder, as askDelete says.
func deleteEntry(x App, h *host.Host, it Item) {
	go func() {
		var err error
		if h.Remote() && h.Client != nil {
			// The machine's own hylad deletes it there: another machine's
			// file is not this one's to put in its trash.
			err = h.Client.Call("fs.delete", proto.FSDeleteParams{Path: it.Path}, nil)
		} else {
			err = mygo.Shell.TrashItem(it.Path)
		}
		x.Update(func() {
			if err != nil {
				x.Toast(locales.FilesDeleteFailed(it.Name, RemoteError(err)))
				return
			}
			x.FilesState().Gen++
			// A file a viewer shows is gone: read anew, it says so.
			for _, fv := range x.FilesState().Views {
				if fv != nil && x.HostOf(fv.Pane) == h && (fv.Path == it.Path || WithinDir(fv.Path, it.Path)) {
					fv.Fetched = time.Time{}
				}
			}
		})
	}()
}

// entryMenu is a row's menu: a new file or folder in its folder, the
// folder's own for a folder, and the row's rename and delete.
func EntryMenu(x App, m *ui.Menu, s *proto.Session, pane string, it Item) {
	dir, rel := it.Parent, it.ParentRel
	if it.Dir {
		dir, rel = it.Path, it.Rel
	}
	if m.Item(locales.FilesNewFile()).Chosen() {
		StartCreate(x, s, pane, dir, rel, false)
	}
	if m.Item(locales.FilesNewFolder()).Chosen() {
		StartCreate(x, s, pane, dir, rel, true)
	}
	m.Separator()
	if m.Item(locales.FilesRename()).Chosen() {
		startRenameEntry(x, s, pane, it)
	}
	if m.Item(locales.FilesDeleteItem()).Chosen() {
		askDelete(x, pane, it)
	}
	x.PluginFileMenu(m, s, pane, it)
}

// headerH is the height of the viewer's header.
const headerH = 36

// b2f is b as 0, or 1 for true.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// ImageExt are the extensions of the images the viewer shows as images.
var ImageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true}

// listingDir is a folder's listing, made now as the tab makes it, for
// what opens a folder in a tab that does not show yet.
func listingDir(x App, h *host.Host, dir, stamp string) *DirList {
	st := x.FilesState()
	key := Key(h, dir)
	l := st.Dirs[key]
	if l == nil {
		l = &DirList{}
		st.Dirs[key] = l
	}
	l.Busy = true
	gen := st.Gen
	cl := h.Client
	go func() {
		var res proto.FSList
		err := cl.Call("fs.list", proto.FSListParams{Dir: dir, Hidden: true}, &res)
		x.Update(func() {
			l.Busy, l.Fetched, l.Stamp, l.Gen = false, time.Now(), stamp, gen
			if err != nil {
				l.Err = err.Error()
				return
			}
			l.Entries, l.Err = res.Entries, ""
		})
	}()
	return l
}
