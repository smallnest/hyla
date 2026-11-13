package main

import (
	"os"
	"path/filepath"
	"strings"

	"hyla/internal/proto"

	"hyla/locales"
)

// maxDropFiles and maxDropLevel bound what a drop's listing takes: files
// a folder holds, and how deep it goes.
const (
	maxDropFiles = 2000
	maxDropLevel = 8
)

// pasteDrop pastes the paths of what was dropped into the program of the
// pane, as its shell takes them: they are of this machine, where the
// program runs too.
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

func pastedLine(h *host, dir string, names []string) string {
	words := make([]string, len(names))
	for i, n := range names {
		words[i] = pastedPath(h, filepath.Join(dir, n))
	}
	return strings.Join(words, " ")
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
				return os.ErrInvalid
			}
			if rel, err := filepath.Rel(p, q); err == nil && strings.Count(rel, string(filepath.Separator)) > maxDropLevel {
				return os.ErrInvalid
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
