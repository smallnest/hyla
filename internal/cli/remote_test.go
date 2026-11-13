package cli

import (
	"os"
	"path/filepath"
	"testing"

	"hyla/internal/remote"
	"hyla/internal/settings"
)

func TestRemoteRepoPath(t *testing.T) {
	for in, want := range map[string]string{
		"":                  "",
		"src/a.go":          "src/a.go",
		`src\a.go`:          "src/a.go",
		"/srv/app/src/a.go": "src/a.go",
		"../../etc/passwd":  "etc/passwd",
	} {
		got, err := repoPath("/srv/app", in, true)
		if err != nil || got != want {
			t.Errorf("repoPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := repoPath("/srv/app", "/etc/passwd", true); err == nil {
		t.Error("a path outside the worktree passes")
	}
}

// `hyla tailcat` turns this machine's tailcat listener on and off,
// and rotates its pair code, in the settings file it reads.
func TestHostsTailcat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_CONFIG", filepath.Join(home, "config"))
	// A file that turns Tailcat off keeps its listener off, and says so.
	set := settings.Load()
	set.Tailcat = false
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
	if err := tailcatCmd([]string{"enable"}); err == nil {
		t.Error("a listener with Tailcat off")
	}
	set.Tailcat = true
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
	if err := tailcatCmd([]string{"enable", "7400"}); err != nil {
		t.Fatal(err)
	}
	set = settings.Load()
	if l, port, ok := set.TailcatListen(); !ok || l != "tailcat://:7400" || port != "7400" {
		t.Fatalf("listen %q", settings.Load().Listen)
	}
	if err := tailcatCmd([]string{"port", "http"}); err == nil {
		t.Error("a bad port")
	}
	if err := tailcatCmd([]string{"disable"}); err != nil {
		t.Fatal(err)
	}
	set = settings.Load()
	if _, _, ok := set.TailcatListen(); ok {
		t.Errorf("listen %q", settings.Load().Listen)
	}
	rot := settings.Load().TailcatRotate
	if err := tailcatCmd([]string{"rotate"}); err != nil {
		t.Fatal(err)
	}
	if settings.Load().TailcatRotate == rot {
		t.Error("hylad told not to start its listener again")
	}
	if st, err := os.Stat(remote.TailcatKeyPath()); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file %v, %v", st, err)
	}
}
