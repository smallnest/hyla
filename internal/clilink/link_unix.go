//go:build !windows

package clilink

import (
	"os"
	"path/filepath"
)

func target(bin string) string {
	t, _ := os.Readlink(filepath.Join(bin, "hyla"))
	return t
}

func link(exe, bin string) error {
	os.MkdirAll(bin, 0o755)
	dropLegacy(bin)
	link := filepath.Join(bin, "hyla")
	if cur, err := os.Readlink(link); err == nil && cur == exe {
		return nil
	}
	os.Remove(link)
	return os.Symlink(exe, link)
}

// dropLegacy removes the kero command older Hylas left in bin: a link
// to an executable that is no longer made.
func dropLegacy(bin string) {
	p := filepath.Join(bin, "kero")
	if t, err := os.Readlink(p); err == nil && filepath.Base(t) == "kero" {
		os.Remove(p)
	}
}
