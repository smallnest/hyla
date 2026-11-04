//go:build !windows

package clilink

import (
	"os"
	"path/filepath"
	"testing"
)

// The command of 2.x, which ran an executable no longer made, goes as the
// command of this Hyla comes.
func TestLinkDropsLegacyCommand(t *testing.T) {
	bin := t.TempDir()
	old := filepath.Join(bin, "kero")
	if err := os.Symlink(filepath.Join("/gone", "kero"), old); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "hyla")
	if err := Link(exe, bin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(old); !os.IsNotExist(err) {
		t.Errorf("the kero command of 2.x is still there: %v", err)
	}
	if got, err := os.Readlink(filepath.Join(bin, "hyla")); err != nil || got != exe {
		t.Errorf("hyla links to %q, %v; want %q", got, err, exe)
	}
}
