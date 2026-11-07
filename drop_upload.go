package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/pkg/sftp"

	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/remote"

	"hyla/locales"
)

// Files dragged from this machine onto a pane of another machine go to
// the directory its program runs in, as scp puts them: where the shell is,
// or where the agent in it went, as into a worktree. The host tells its
// pane's directory (pane.dir), which the daemon reads from the processes
// of the terminal, as it does for the sidebar. The files go over an SFTP
// connection of the connection to the host, as scp's do, and their paths
// there are pasted into the program, as a drop on it there's would be.

// maxDropFiles and maxDropLevel bound what a dropped folder may hold, and
// how deep it goes, so that a large tree is told of before it copies.
const (
	maxDropFiles = 2000
	maxDropLevel = 8
)

// paneUpload is a pane that files are dropping on now: dir is the
// directory of its program as its host told it, which the paths pasted
// into it go from.
type paneUpload struct{ id, dir string }

// dropUpload puts what was dropped on the pane id on its host, in the
// directory of the pane's program, and pastes its paths there into the
// program.
func (a *App) dropUpload(id string, dropped []string) {
	h := a.hostOf(id)
	if !h.remote() {
		return
	}
	if h.cl == nil {
		a.toast(locales.HostNotConnected(h.label()))
		return
	}
	// The pane's slot shows the upload on the next frame (droppedNow),
	// and comes off with its end.
	up := paneUpload{id: id}
	a.uploading = append(a.uploading, up)
	dialer := h.dialer()
	go func() {
		pd, err := uploadable(dialer, id)
		var dir string
		if err == nil {
			dir = pd.Dir
		}
		a.main(func() {
			up.dir = dir
			a.uploadDir(up)
		})
		if err == nil {
			err = a.uploadDrop(dialer, id, dir, dropped)
		}
		// The end comes with its own frame, which takes the slot up, if
		// none was built while the upload was under way (uploadDir).
		a.main(func() { a.dropDone(up, err) })
	}()
}

// uploadDir sets the directory of an upload, once its host told it.
func (a *App) uploadDir(up paneUpload) {
	a.uploading = append(slices.DeleteFunc(a.uploading, func(u paneUpload) bool { return u.id == up.id }), up)
}

// dropDone ends an upload, and says what went wrong, if anything; a pane
// that shows first, once its upload is done, shows with the paths
// pasted.
func (a *App) dropDone(up paneUpload, err error) {
	a.uploading = slices.DeleteFunc(a.uploading, func(u paneUpload) bool { return u == up })
	if err != nil {
		a.toast(err.Error())
	}
}

// uploadable returns the directory of the pane's program, as its host
// tells it (pane.dir).
func uploadable(dialer client.Dialer, id string) (proto.PaneDir, error) {
	cl, err := client.ConnectVia(dialer)
	if err != nil {
		return proto.PaneDir{}, err
	}
	defer cl.Close()
	var r proto.PaneDir
	if err := cl.Call("pane.dir", proto.PaneDirParams{Pane: id}, &r); err != nil {
		return proto.PaneDir{}, err
	}
	if !validDir(r.Dir) {
		return proto.PaneDir{}, errors.New("the pane's directory is unknown")
	}
	return r, nil
}

// uploadDrop copies what was dropped to dir of the host, the pane's
// program's directory, and pastes the paths there into the pane.
func (a *App) uploadDrop(dialer client.Dialer, id, dir string, dropped []string) error {
	list, err := droppedFiles(dropped)
	if err != nil {
		return err
	}
	s, err := newSFTP(dialer)
	if err != nil {
		return err
	}
	defer s.Close()
	if list.total > 1<<20 {
		a.main(func() { a.toast(locales.PasteUploading(a.hostOf(id).label())) })
	}
	names, err := list.put(s, dir)
	if err != nil {
		return err
	}
	h := a.hostOf(id)
	if h.cl == nil {
		return errors.New(locales.HostNotConnected(h.label()))
	}
	h.cl.Go("paste.text", proto.PasteTextParams{Pane: id, Text: pastedLine(h, dir, names), Dir: dir}, nil)
	return nil
}

// pasteDrop puts what was dropped on the local pane id in the directory
// of its program—where the drop came from—and pastes its paths there into
// the program, as a drop on it in Finder does. The host tells the
// directory off the main thread; what it says comes back through a.main.
func (a *App) pasteDrop(id string, dropped []string) {
	h := a.hostOf(id)
	cl := h.cl
	if cl == nil {
		a.toast(locales.HostNotConnected(h.label()))
		return
	}
	go func() {
		var pd proto.PaneDir
		if err := cl.Call("pane.dir", proto.PaneDirParams{Pane: id}, &pd); err != nil {
			a.main(func() { a.toast(err.Error()) })
			return
		}
		if !h.windows() && !filepath.IsAbs(pd.Dir) {
			a.main(func() { a.toast(locales.DropDirUnknown()) })
			return
		}
		names := make([]string, len(dropped))
		for i, f := range dropped {
			names[i] = filepath.Base(f)
		}
		if err := cl.Call("paste.text", proto.PasteTextParams{Pane: id, Text: pastedLine(h, pd.Dir, names), Dir: pd.Dir}, nil); err != nil {
			a.main(func() { a.toast(err.Error()) })
		}
	}()
}

// droppedFiles lists what was dropped, as its host takes it, bounded.
func droppedFiles(dropped []string) (droppedList, error) {
	files, dirs, total, err := dropFiles(dropped)
	if err != nil {
		return droppedList{}, err
	}
	if total > maxUpload {
		return droppedList{}, errors.New(locales.PasteTooLarge(maxUpload >> 20))
	}
	return droppedList{files: files, dirs: dirs, total: total}, nil
}

// droppedList is what the user dropped: the files, the folders, and their
// size all.
type droppedList struct {
	files, dirs []string
	total       int64
}

// put puts a dropped list in dir of the files s, and returns the names
// they take there: free ones, so that a drop does not write over what is
// there.
func (l droppedList) put(s sftpFS, dir string) ([]string, error) {
	var names []string
	for _, f := range l.files {
		name, err := putFile(s, dir, filepath.Base(f), f)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", filepath.Base(f), err)
		}
		names = append(names, name)
	}
	for _, d := range l.dirs {
		name, err := putDir(s, dir, d)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", filepath.Base(d), err)
		}
		names = append(names, name)
	}
	return names, nil
}

// pastedLine writes the names of what was dropped in dir of the host as
// the words a program takes: their paths there, escaped for its shell,
// a space apart.
func pastedLine(h *host, dir string, names []string) string {
	words := make([]string, len(names))
	for i, n := range names {
		words[i] = pastedPath(h, path.Join(dir, n))
	}
	return strings.Join(words, " ")
}

// newSFTP opens an SFTP connection to a host, over a remote connection of
// its own: as scp's, which puts files on the host from the user's
// namespaces there. A var for tests.
var newSFTP = func(dialer client.Dialer) (sftpFS, error) {
	r, ok := dialer.(remote.Opener)
	if !ok {
		return nil, errors.New("the host is this machine's")
	}
	conn, err := remote.OpenSFTP(r)
	if err != nil {
		return nil, err
	}
	s, err := sftp.NewClientPipe(conn, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return sftpFiles{s}, nil
}

// sftpFiles is the files of an SFTP client.
type sftpFiles struct{ c *sftp.Client }

func (f sftpFiles) ReadDir(dir string) ([]os.FileInfo, error) { return f.c.ReadDir(dir) }
func (f sftpFiles) MkdirAll(dir string) error                 { return f.c.MkdirAll(dir) }
func (f sftpFiles) Close() error                              { return f.c.Close() }
func (f sftpFiles) Create(path string) (writeCloser, error) {
	w, err := f.c.Create(path)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// sftpFS is what the files dropped need of an SFTP connection: the
// client to put them, or one that keeps them here, for tests.
type sftpFS interface {
	ReadDir(string) ([]os.FileInfo, error)
	Create(string) (writeCloser, error)
	MkdirAll(string) error
	Close() error
}

// writeCloser is a file being written on the host.
type writeCloser interface {
	ReadFrom(io.Reader) (int64, error)
	Close() error
}

// droppedNow reports whether files are dropping on the pane id now, which
// its paths then go to when they do (dropped).
func (a *App) droppedNow(id string) bool {
	for _, u := range a.uploading {
		if u.id == id {
			return true
		}
	}
	return false
}

// dropFiles lists the files dropped, and the folders, with what they hold
// up to maxDropFiles: the total size of all.
func dropFiles(dropped []string) (files, dirs []string, total int64, err error) {
	for _, p := range dropped {
		st, err := os.Stat(p)
		if err != nil {
			return nil, nil, 0, err
		}
		if !st.IsDir() {
			files = append(files, p)
			total += st.Size()
			continue
		}
		dirs = append(dirs, p)
		n := 0
		err = filepath.WalkDir(p, func(q string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if n++; n > maxDropFiles {
				return fmt.Errorf("more than %d files", maxDropFiles)
			}
			if rel, err := filepath.Rel(p, q); err == nil && strings.Count(rel, string(filepath.Separator)) > maxDropLevel {
				return fmt.Errorf("deeper than %d levels", maxDropLevel)
			}
			if d.Type().IsRegular() {
				if info, err := d.Info(); err == nil {
					total += info.Size()
				}
			}
			return nil
		})
		if err != nil {
			return nil, nil, 0, err
		}
	}
	return files, dirs, total, nil
}

// validDir reports whether dir is an absolute path of a host, as one its
// shell reads as it is.
func validDir(dir string) bool {
	if dir == "" || strings.ContainsRune(dir, 0) {
		return false
	}
	if len(dir) >= 2 && dir[1] == ':' { // C:\ or C:/
		return len(dir) > 2 && (dir[2] == '\\' || dir[2] == '/')
	}
	return strings.HasPrefix(dir, "/") || strings.HasPrefix(dir, "~")
}

// putFile puts a file of this machine in dir of the host, under its name
// or a free one (freeName), and returns the name.
func putFile(s sftpFS, dir, base, file string) (string, error) {
	name, err := freeName(s, dir, base)
	if err != nil {
		return "", err
	}
	in, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := s.Create(path.Join(dir, name))
	if err != nil {
		return "", err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return "", err
	}
	return name, out.Close()
}

// putDir puts a folder of this machine, with what it holds, in dir of the
// host, and returns the name it takes there.
func putDir(s sftpFS, dir, folder string) (string, error) {
	name, err := freeName(s, dir, filepath.Base(folder))
	if err != nil {
		return "", err
	}
	root := path.Join(dir, name)
	err = filepath.WalkDir(folder, func(q string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(folder, q)
		if err != nil {
			return err
		}
		remote := root
		if rel != "." {
			remote = path.Join(root, filepath.ToSlash(rel))
		}
		switch {
		case d.IsDir():
			return s.MkdirAll(remote)
		case d.Type().IsRegular():
			_, err := putFile(s, path.Dir(remote), d.Name(), q)
			return err
		}
		return nil // a socket, a link: not what scp copies
	})
	return name, err
}

// freeName returns base, or base with a suffix as "name 2.ext", when a
// name of that one is in dir, as a Finder's save takes.
func freeName(s sftpFS, dir, base string) (string, error) {
	names := map[string]bool{}
	entries, err := s.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !names[base] {
		return base, nil
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; ; i++ {
		name := stem + " " + strconv.Itoa(i) + ext
		if !names[name] {
			return name, nil
		}
	}
}
