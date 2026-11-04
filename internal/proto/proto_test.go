package proto

import (
	"os"
	"path/filepath"
	"testing"
)

// A console's default title, its program's path, names nothing: the pane
// goes by its program.
func TestConsoleTitle(t *testing.T) {
	for title, want := range map[string]bool{
		`管理员: C:\Program Files\WindowsApps\Microsoft.PowerShell_7.6.7.0_x64__8wekyb3d8bbwe\pwsh.exe`: true,
		`Administrator: C:\Windows\system32\cmd.exe - ping  -n 6 127.0.0.1`:                          true,
		`C:\Program Files\PowerShell\7\pwsh.exe`:                                                     true,
		`C:\src\hyla`:                                                                                true,
		`Fix the build`:                                                                              false,
		`Deploying: C:\deploy.ps1 running`:                                                           false,
	} {
		if got := looksLikePath(title); got != want {
			t.Errorf("looksLikePath(%q) = %v", title, got)
		}
	}
	p := Pane{Process: "node", Title: `管理员: C:\Program Files\PowerShell\7\pwsh.exe`}
	if got := p.PaneName(); got != "node" {
		t.Errorf("pane named %q", got)
	}
}

func TestWithin(t *testing.T) {
	sep := string(filepath.Separator)
	home := sep + "home" + sep + "me"
	for path, want := range map[string]bool{home: true, home + sep + "src": true, home + "2": false, sep + "home": false} {
		if got := Within(path, home); got != want {
			t.Errorf("Within(%q, %q) = %v", path, home, got)
		}
	}
}

// A Windows host's directory names its pane by its last element on any
// system the app runs on.
func TestDirName(t *testing.T) {
	for dir, want := range map[string]string{
		`C:\Users\egoist\dev\mygo`: "mygo",
		`C:\src\hyla\`:             "hyla",
		`C:\`:                      `C:\`,
		"/home/me/src":             "src",
		"/":                        "/",
	} {
		if got := DirName(dir); got != want {
			t.Errorf("DirName(%q) = %q", dir, got)
		}
	}
	p := Pane{Dir: `C:\Users\egoist\dev\mygo`}
	if got := p.PaneName(); got != "mygo" {
		t.Errorf("pane named %q", got)
	}
}

// Kero's variables are Hyla's, unless Hyla's are set.
func TestAdoptLegacyEnv(t *testing.T) {
	t.Setenv("HYLA_PANE", "")
	t.Setenv("HYLA_SESSION", "new")
	t.Setenv("KERO_PANE", "old-pane")
	t.Setenv("KERO_SESSION", "old-session")
	AdoptLegacyEnv()
	if got := os.Getenv("HYLA_PANE"); got != "old-pane" {
		t.Errorf("HYLA_PANE = %q, want old-pane", got)
	}
	if got := os.Getenv("HYLA_SESSION"); got != "new" {
		t.Errorf("HYLA_SESSION = %q, want new", got)
	}
}

// The directory a 2.x install left as Kero's is the one this Hyla uses.
func TestAdoptDir(t *testing.T) {
	base := t.TempDir()
	old, dir := filepath.Join(base, "kero"), filepath.Join(base, "hyla")
	if got := adoptDir(dir, old); got != dir {
		t.Errorf("no directory at all: got %q, want %q", got, dir)
	}
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := adoptDir(dir, old); got != dir {
		t.Errorf("2.x's directory: got %q, want %q", got, dir)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err != nil || string(b) != "{}" {
		t.Errorf("the settings of 2.x did not come along: %v", err)
	}
	// A directory of this Hyla's stays as it is, and the old one stays where it is.
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"new":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := adoptDir(dir, old); got != dir {
		t.Errorf("both directories: got %q, want %q", got, dir)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) != `{"new":1}` {
		t.Errorf("this Hyla's settings were replaced: %s", b)
	}
}
