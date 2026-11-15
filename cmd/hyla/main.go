// hyla is a terminal for coding agents: sessions grouped by project in a
// sidebar that shows which work, which need you and which are done. The
// sessions run in hylad, a daemon of their own, so quitting hyla leaves
// them running, and opening it again shows everything as it was.
//
// The same executable is the hyla CLI (hyla help), which drives sessions
// from a shell or an agent, and hylad itself (hyla __hylad).
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/native"
	"golang.org/x/term"

	"hyla/internal/app/window"
	"hyla/internal/cli"
	"hyla/internal/clilink"
	"hyla/internal/daemon"
	_ "hyla/internal/pocket"
	"hyla/internal/profiling"
	"hyla/internal/proto"
)

func main() {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		switch {
		case arg == "__hylad":
			if err := daemon.Run(daemon.Options{}); err != nil {
				fmt.Fprintln(os.Stderr, "hylad:", err)
				os.Exit(1)
			}
			return
		case cli.IsCommand(arg):
			os.Exit(cli.Run(os.Args[1:]))
		}
		if what, ok := cli.Removed(arg); ok {
			fmt.Fprintf(os.Stderr, "hyla: %s is gone: %s\n", arg, what)
			os.Exit(2)
		}
		// Not a command, and not the process serial number a launch from
		// the Finder passes: say so, rather than open the app and ignore
		// the word.
		if !strings.HasPrefix(arg, "-psn") {
			fmt.Fprintf(os.Stderr, "hyla: no command %q: hyla help lists them\n", arg)
			os.Exit(2)
		}
	}
	// `hyla` alone in a terminal, through the CLI's link to the app's
	// executable: open the app as the Finder would.
	if len(os.Args) == 1 && term.IsTerminal(int(os.Stdin.Fd())) {
		if exe, err := os.Executable(); err == nil {
			if real, err := filepath.EvalSymlinks(exe); err == nil {
				if i := strings.Index(real, ".app/Contents/MacOS/"); i >= 0 {
					if exec.Command("/usr/bin/open", real[:i+4]).Run() == nil {
						return
					}
				}
				// On Windows the command is a copy of the app's executable
				// (package clilink), which knows what it is a copy of.
				if bin := filepath.Dir(real); runtime.GOOS == "windows" && strings.EqualFold(bin, filepath.Join(proto.Dir(), "bin")) {
					if app := clilink.Target(bin); app != "" && exec.Command(app).Start() == nil {
						return
					}
				}
			}
		}
	}
	runApp()
}

func runApp() {
	// Started from a pane, as Hyla Dev by `mygo dev` in the installed Hyla:
	// the pane's variables name its hylad, not this build's.
	if os.Getenv("HYLA_PANE") != "" {
		for _, k := range []string{"HYLA_SOCKET", "HYLA_SESSION", "HYLA_PANE"} {
			os.Unsetenv(k)
		}
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	profiling.Start()
	cli.SyncSkill()
	// Updates from Hyla's GitHub releases, in a window of native UI.
	mygo.Use(native.Plugin)
	a := window.New()
	window.LoadFonts()
	mygo.Theme.SetSource(a.Appearance())
	mygo.App.SetLocale(a.Language())
	mygo.App.SetMenu(a.Menu())
	// Windows draw anew in another language; the menu and the titles,
	// set once, are made again in it.
	mygo.App.OnLocaleChanged(func(string) { a.LanguageChanged() })
	mygo.App.OnSecondInstance(func([]string, string) { a.ShowWindow() })
	mygo.App.OnDidBecomeActive(a.BecomesActive)
	mygo.App.OnDidResignActive(a.Resigns)
	mygo.App.OnActivate(a.Activated)
	mygo.App.OnWindowAllClosed(a.EveryWindowClosed)
	// The question quit-ends-sessions may ask comes before the windows
	// close, and takes the quit back.
	mygo.App.OnBeforeQuit(a.BeforeQuit)
	// Windows leaves an icon in the notification area after its app until
	// the pointer passes over it.
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.WillQuit() })
	// No Dock icon from the start for the menu bar alone.
	if runtime.GOOS == "darwin" {
		a.MenuBarOnly()
	}
	mygo.App.WhenReady(a.Start)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func stopDaemon() error { return cli.StopDaemon() }
