package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
)

// TestDropFiles lists what a drop holds: files, and folders with their
// size, up to the limit of what drops.
func TestDropFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world!"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, dirs, total, err := dropFiles([]string{filepath.Join(dir, "a.txt"), filepath.Join(dir, "sub")})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || len(dirs) != 1 || total != 11 {
		t.Errorf("files %v, dirs %v, total %d", files, dirs, total)
	}
}

// TestValidDir checks the directories that files dropped are put in: the
// host's absolute ones, as its shell reads them, and no others.
func TestValidDir(t *testing.T) {
	for dir, want := range map[string]bool{
		"/home/me":      true,
		"/":             true,
		"~/repo":        true,
		`C:\repo`:       true,
		"C:/repo":       true,
		"":              false,
		":":             false,
		"repo":          false,
		"C:":            false,
		"/home/me\nrm":  true, // quoted as it is pasted, which no shell runs
		"/home/\x00me":  false,
		"relative/path": false,
	} {
		if got := validDir(dir); got != want {
			t.Errorf("validDir(%q) = %v, want %v", dir, got, want)
		}
	}
}

// TestFreeName takes a name, or one beside it as a Finder's save does, so
// that a drop does not write over what is there.
func TestFreeName(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "a 2.txt", "noext"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := localFS{}
	for _, c := range []struct{ base, want string }{
		{"a.txt", "a 3.txt"},
		{"noext", "noext 2"},
		{"b.txt", "b.txt"},
	} {
		got, err := freeName(s, dir, c.base)
		if err != nil || got != c.want {
			t.Errorf("freeName(%q) = %q, %v, want %q", c.base, got, err, c.want)
		}
	}
}

// localFS serves files of this machine, as an SFTP client of a host does
// whose directories are this machine's.
type localFS struct{}

func (f localFS) dir(dir string) string { return dir }

func (f localFS) ReadDir(dir string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(f.dir(dir))
	if err != nil {
		return nil, err
	}
	info := make([]os.FileInfo, len(entries))
	for i, e := range entries {
		if info[i], err = e.Info(); err != nil {
			return nil, err
		}
	}
	return info, nil
}

func (f localFS) MkdirAll(dir string) error { return os.MkdirAll(f.dir(dir), 0o700) }

func (f localFS) Create(p string) (writeCloser, error) { return os.Create(f.dir(p)) }

func (f localFS) Close() error { return nil }

// TestPutFileAndDir checks that a drop's file, and folder, go where its
// paths say, and that a second of a name goes beside the first.
func TestPutFileAndDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "repo", "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "repo", "sub", "b.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := localFS{}
	for range 2 {
		name, err := putFile(s, dst, "a.txt", filepath.Join(src, "a.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if b, err := os.ReadFile(filepath.Join(dst, name)); err != nil || string(b) != "one" {
			t.Errorf("%s: %q, %v", name, b, err)
		}
	}
	name, err := putDir(s, dst, filepath.Join(src, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dst, name, "sub", "b.txt")); err != nil || string(b) != "two" {
		t.Errorf("%s/sub/b.txt: %q, %v", name, b, err)
	}
	names, _ := os.ReadDir(dst)
	if len(names) != 3 {
		t.Errorf("dst holds %v", names)
	}
}

// Dropping a file on a pane of this machine pastes its path into the
// program, as it is, escaped for the shell, with no copy made.
func TestPasteDropLocal(t *testing.T) {
	a := testDropApp(t)
	pasted := make(chan proto.PasteTextParams, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "pane.dir":
			return proto.PaneDir{Dir: "/work"}, nil
		case "paste.text":
			var p proto.PasteTextParams
			json.Unmarshal(raw, &p)
			pasted <- p
			return nil, nil
		}
		return nil, fmt.Errorf("test daemon rejected %s", method)
	})
	defer stop()
	a.local().cl = cl
	dir := t.TempDir()
	file := filepath.Join(dir, "my notes.txt")
	if err := os.WriteFile(file, []byte("dropped"), 0o600); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1100, 640)
	tt.Frame()
	if !tt.DropFiles(400, 340, file) {
		t.Fatal("the pane took no drop")
	}
	select {
	case p := <-pasted:
		if want := "/work/my\\ notes.txt"; p.Text != want || p.Dir != "/work" {
			t.Errorf("pasted %q from %q, want %q", p.Text, p.Dir, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no paste")
	}
}
