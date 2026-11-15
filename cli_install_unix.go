//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"hyla/internal/proto"

	"hyla/locales"
)

// cliLinks are where installCLI puts the hyla command, and where Settings
// looks for it.
func cliLinks() []string {
	return []string{"/usr/local/bin/hyla", "/opt/homebrew/bin/hyla", filepath.Join(proto.Home(), ".local/bin/hyla")}
}

// installCLI links the hyla command to this executable: /usr/local/bin's
// on macOS, asking for an administrator's password when the directory
// needs it; ~/.local/bin's on Linux, which needs none.
func (a *App) installCLI() {
	exe, err := os.Executable()
	if err != nil {
		a.toast(err.Error())
		return
	}
	go func() {
		link := "/usr/local/bin/hyla"
		if runtime.GOOS != "darwin" {
			link = filepath.Join(proto.Home(), ".local/bin/hyla")
			os.MkdirAll(filepath.Dir(link), 0o755)
		}
		// A command that already runs this executable needs nothing, and
		// asks for no password: /usr/local/bin's belongs to root, and a
		// link left there by an earlier install cannot be replaced by the
		// user alone.
		err := error(nil)
		if !sameLink(link, exe) {
			err = replaceLink(link, exe)
		}
		if err != nil && runtime.GOOS == "darwin" {
			// The paths go in as arguments, quoted by AppleScript itself,
			// so no app location can break the script.
			err = exec.Command("/usr/bin/osascript",
				"-e", "on run argv",
				"-e", `do shell script "mkdir -p /usr/local/bin && ln -sf " & quoted form of item 1 of argv & " " & quoted form of item 2 of argv & " with administrator privileges`,
				"-e", "end run",
				exe, link).Run()
		}
		a.update(func() {
			if err != nil {
				// Why it failed, not only that it did: a password put off,
				// a directory of another owner, a dialog that never showed.
				a.toast(locales.CliInstallFailed(err.Error()))
				return
			}
			a.toast(locales.CliInstalledIn(proto.ShortDir(filepath.Dir(link))))
		})
	}()
}

// sameLink tells whether path leads to target: a link to it, or the file
// itself, which an install that ran before left there.
func sameLink(path, target string) bool {
	if t, err := os.Readlink(path); err == nil {
		return t == target
	}
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	ts, err := os.Stat(target)
	return err == nil && os.SameFile(st, ts)
}

// replaceLink points path at target, replacing what is there.
func replaceLink(path, target string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(target, path)
}

// cliInstalled returns where the hyla command is installed, and what it
// runs, "" when it is nowhere.
func cliInstalled() string {
	for _, p := range cliLinks() {
		if t, err := os.Readlink(p); err == nil {
			return p + " → " + proto.ShortDir(t)
		}
	}
	return ""
}
