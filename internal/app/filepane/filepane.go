// Package filepane reads the file a file pane shows. A pane shows what
// the file was as it opened, as a terminal shows what its program
// printed.
package filepane

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"hyla/internal/client"
	"hyla/internal/proto"
)

// The largest a file shown as text, and as an image, may be.
const (
	MaxView  = 4 << 20
	MaxImage = 32 << 20
)

// ErrGone is a file that is no longer there.
var ErrGone = errGone{}

type errGone struct{}

func (errGone) Error() string { return "no such file" }

// State is a file pane's: its file, as read, and when it was, which a
// pane coming back to view reads again once the file is older.
type State struct {
	Path   string
	St     *editor.State
	BM     *ui.Bitmap
	Busy   bool
	Binary bool
	Large  bool
	Err    string
	Gen    uint64
	Read   bool // a read was asked for
	Mod    int64
}

// panes are the files shown in file panes, by pane.
type panes map[string]*State

// Panes keeps the file panes' files, by pane.
type Panes struct {
	byPane panes
}

// Of returns a pane's file state, made as the pane first shows: path is
// the file the pane shows.
func (p *Panes) Of(id, path string) *State {
	if p.byPane == nil {
		p.byPane = panes{}
	}
	f := p.byPane[id]
	if f == nil || f.Path != path {
		f = &State{Path: path}
		p.byPane[id] = f
	}
	return f
}

// DropForgets drops the states of the panes gone.
func (p *Panes) Forget(keep func(id string) bool) {
	for id := range p.byPane {
		if !keep(id) {
			delete(p.byPane, id)
		}
	}
}

// Say says what a failed call came to, as the pane's message names it.
type Say func(error) string

// Read reads a pane's file, once as it opens. Client gives the pane's
// hylad, nil while it is not connected; Update runs fn on the main
// thread; Say says what a failed call came to.
func Read(f *State, cl *client.Client, update func(func()), say Say) {
	if cl == nil || f.Read {
		return
	}
	f.Read = true
	f.Busy, f.Err = true, ""
	f.Gen++
	gen, path := f.Gen, f.Path
	image := imageExt(strings.ToLower(filepath.Ext(path)))
	go func() {
		var st proto.FSStat
		var r proto.FSRead
		var bm *ui.Bitmap
		err := cl.Call("fs.stat", proto.FSStatParams{Path: path}, &st)
		large := err == nil && (image && st.Size > MaxImage || !image && st.Size > MaxView)
		if err == nil && (!st.Exists || st.Dir) {
			err = ErrGone
		}
		if err == nil && !large {
			err = cl.Call("fs.read", proto.FSReadParams{Path: path, Max: max(MaxImage, MaxView)}, &r)
		}
		if err == nil && !large && image {
			bm, err = ui.DecodeBitmap(r.Data)
		}
		if err == nil && !large && image && bm == nil {
			// The host's hylad cannot draw it: its bytes are as good a
			// view as a text of them.
			image = false
		}
		update(func() {
			if f.Gen != gen {
				return
			}
			f.Busy = false
			if err != nil {
				f.Err = say(err)
				return
			}
			f.Err = ""
			f.Large, f.Binary = large, false
			switch {
			case large:
				f.St, f.BM = nil, nil
			case image && bm != nil:
				f.BM, f.St = bm, nil
			case IsBinary(r.Data):
				f.Binary, f.St, f.BM = true, nil, nil
			default:
				f.BM = nil
				b := editor.NewBuffer(string(r.Data))
				b.SetLanguage(editor.LanguageFor(filepath.Base(path)))
				f.St = &editor.State{Buffer: b}
			}
		})
	}()
}

// IsBinary tells a file that isn't text, as git does: a NUL in its first
// 8000 bytes.
func IsBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0
}

// imageExt tells the extensions of the images ui.DecodeBitmap reads.
func imageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}
