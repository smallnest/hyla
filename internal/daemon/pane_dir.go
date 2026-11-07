package daemon

import (
	"os"
	"path/filepath"

	"hyla/internal/proto"
)

// paneDir tells where a pane's program runs, for the files that the user
// drops on it, as scp puts them: in the directory of the program in the
// terminal's foreground, or of the shell when that is the terminal's, and
// the shell's beside it.
func (d *Daemon) paneDir(p proto.PaneDirParams) (proto.PaneDir, error) {
	d.mu.Lock()
	pane := d.findPaneLocked(p.Pane)
	d.mu.Unlock()
	if pane == nil {
		return proto.PaneDir{}, os.ErrNotExist
	}
	return paneDirOf(pane), nil
}

// paneDirOf tells where a pane's program runs: in the directory of the
// program in the terminal's foreground, or of the shell when that is the
// terminal's, and the shell's beside it. An agent that went into a
// worktree is made for it: its commands run in the one it was given, as a
// cd of the program alone does not lead to.
func paneDirOf(pane *pane) proto.PaneDir {
	pane.mu.Lock()
	dir, shell, pg := pane.info.Dir, pane.info.Dir, pane.fgPGID
	shellPID := pane.shellPID
	pane.mu.Unlock()
	if pg > 0 && pg != shellPID {
		// The program's own, as an agent's that went into a worktree:
		// the shell under it stays where it was.
		if pdir := procCwd(pg); pdir != "" && filepath.IsAbs(pdir) {
			dir = pdir
		}
	}
	return proto.PaneDir{Dir: dir, Shell: shell}
}
