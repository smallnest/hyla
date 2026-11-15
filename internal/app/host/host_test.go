package host

import (
	"runtime"
	"testing"
)

// A path pasted into a POSIX shell keeps its words apart: what a shell
// would read as its own is escaped.
func TestPastedPath(t *testing.T) {
	h := &Host{}
	for c, want := range map[string]string{
		"plain/path":       "plain/path",
		"with space.md":    `with\ space.md`,
		"quote's file.txt": `quote\'s\ file.txt`,
		"π/日本語":            "π/日本語",
	} {
		if got := PastedPath(h, c); got != want {
			t.Errorf("PastedPath(%q) = %q, want %q", c, got, want)
		}
	}
}

// A host names itself: this machine has no name of its own, and another's
// is its name. Only a remote host's is what a session's title adds.
func TestLabelAndRemote(t *testing.T) {
	local, remote := &Host{}, &Host{Name: "devbox"}
	if local.Label() != "local" {
		t.Errorf("this machine's label is %q, want local", local.Label())
	}
	if remote.Label() != "devbox" || !remote.Remote() || local.Remote() {
		t.Errorf("labels: local %q remote %q, remote %v local %v",
			local.Label(), remote.Label(), remote.Remote(), local.Remote())
	}
}

// Only Windows' hosts quote paths as PowerShell does.
func TestWindows(t *testing.T) {
	if got := (&Host{}).Windows(); got != (runtime.GOOS == "windows") {
		t.Errorf("Windows() = %v on %s", got, runtime.GOOS)
	}
}
