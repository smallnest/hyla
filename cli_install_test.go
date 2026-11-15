//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A link is what an install made, and installing again over it keeps the
// new target: a command that already runs this executable is left as it is.
func TestLinkTheCommand(t *testing.T) {
	dir := t.TempDir()
	// Names apart by more than their case: macOS's volumes are case
	// insensitive, and "Hyla" would be the file "hyla" looks for.
	target := filepath.Join(dir, "hyla-app")
	other := filepath.Join(dir, "other-app")
	for _, p := range []string{target, other} {
		if err := os.WriteFile(p, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, "hyla")

	if sameLink(link, target) {
		t.Error("a link that is not there counts as one")
	}
	if err := replaceLink(link, target); err != nil {
		t.Fatal(err)
	}
	if !sameLink(link, target) {
		t.Error("the link does not lead to what it was made for")
	}
	if err := replaceLink(link, other); err != nil {
		t.Fatal(err)
	}
	if !sameLink(link, other) || sameLink(link, target) {
		t.Error("replacing the link kept the old target")
	}
	// The executable itself counts as installed: an install that ran
	// before left the file where the link would go.
	if !sameLink(target, target) {
		t.Error("the executable itself does not count as linked to it")
	}
}
