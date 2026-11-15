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

// `hyla p2p` turns this machine's network listener on and off, and makes
// its password, in the settings file it reads: it is not hyla tailcat's
// pair code.
func TestHostsP2P(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_CONFIG", filepath.Join(home, "config"))
	if err := settings.Save(settings.Load()); err != nil {
		t.Fatal(err)
	}
	if err := p2pCmd([]string{"enable", "7401"}); err != nil {
		t.Fatal(err)
	}
	set := settings.Load()
	if l, port, ok := set.TCPListen(); !ok || l != "tcp://:7401" || port != "7401" {
		t.Fatalf("listen %q", set.Listen)
	}
	if _, _, ok := set.TailcatListen(); ok {
		t.Errorf("hyla p2p made a tailcat listener: %q", set.Listen)
	}
	if token := set.ListenToken; token == "" {
		t.Error("no password")
	} else if err := p2pCmd([]string{"password"}); err != nil {
		t.Fatal(err)
	} else if next := settings.Load().ListenToken; next == "" || next == token {
		t.Errorf("a new password is %q, the old was %q", next, token)
	}
	if err := p2pCmd([]string{"port", "http"}); err == nil {
		t.Error("a bad port")
	}
	if err := p2pCmd([]string{"disable"}); err != nil {
		t.Fatal(err)
	}
	set = settings.Load()
	if _, _, ok := set.TCPListen(); ok {
		t.Errorf("listen %q", set.Listen)
	}
}
