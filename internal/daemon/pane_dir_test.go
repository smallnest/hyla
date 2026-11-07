package daemon

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"hyla/internal/proto"
)

// TestPaneDir checks that a pane tells the directory of its program, for
// the files dropped on it: its shell's when the shell is in the
// terminal's foreground, and the program's own when one is.
func TestPaneDir(t *testing.T) {
	t.Setenv("HYLA_CONFIG", filepath.Join(t.TempDir(), "config"))
	d := &Daemon{changeCh: make(chan struct{}, 1), saveCh: make(chan struct{}, 1), panes: map[string]*pane{}}
	d.viewed.Store(new(string))
	info := proto.Pane{ID: "a", Dir: "/repo", Status: proto.Idle, Cols: 80, Rows: 24}
	p, err := newPane(d, "s", info)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.term.Close() })
	d.panes["a"] = p

	if _, err := d.paneDir(proto.PaneDirParams{Pane: "none"}); err == nil {
		t.Error("a pane that is not there answered")
	}
	got, err := d.paneDir(proto.PaneDirParams{Pane: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Dir != "/repo" || got.Shell != "/repo" {
		t.Errorf("shell in front: %+v", got)
	}

	if testing.Short() {
		return
	}
	// A program in front that went elsewhere, as an agent into a worktree.
	dir := t.TempDir()
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	sh := startInDir(t, dir)
	p.mu.Lock()
	p.fgPGID = sh
	p.mu.Unlock()
	got, err = d.paneDir(proto.PaneDirParams{Pane: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Dir != dir || got.Shell != "/repo" {
		t.Errorf("program in front: %+v, want %q in front of %q", got, dir, "/repo")
	}
}

// startInDir starts a shell that stays in dir, and returns its pid, as a
// process group in the terminal's foreground would be.
func startInDir(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("sh", "-c", "cd \"$1\" && exec sleep 30", "sh", dir)
	if err := cmd.Start(); err != nil {
		t.Skipf("no shell: %v", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	// The system calls of procCwd find it once it has gone there.
	for i := 0; ; i++ {
		if procCwd(cmd.Process.Pid) == dir {
			return cmd.Process.Pid
		}
		if i == 2000 {
			t.Fatalf("the shell stays in %q", procCwd(cmd.Process.Pid))
		}
		time.Sleep(time.Millisecond)
	}
}

// TestCutPrefix checks that the shell goes to a directory the program was
// in, by whole components: "/repo" of "/repo/work", and never of
// "/repo2/work".
func TestCutPrefix(t *testing.T) {
	for _, c := range []struct {
		s, p, want string
		ok         bool
	}{
		{"/repo/work", "/repo", "/work", true},
		{"/repo/work/sub", "/repo/", "/work/sub", true},
		{"/repo", "/repo", "", true},
		{"/repo2", "/repo", "", false},
		{"/elsewhere", "/repo", "", false},
	} {
		got, ok := cutPrefix(c.s, c.p)
		if got != c.want || ok != c.ok {
			t.Errorf("cutPrefix(%q, %q) = %q, %v, want %q, %v", c.s, c.p, got, ok, c.want, c.ok)
		}
	}
}
