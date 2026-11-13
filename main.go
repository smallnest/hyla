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
	"github.com/egoist/mygo/ui"
	"golang.org/x/term"

	"hyla/internal/cli"
	"hyla/internal/clilink"
	"hyla/internal/daemon"
	_ "hyla/internal/pocket"
	"hyla/internal/proto"

	"hyla/locales"
)

func main() {
	proto.AdoptLegacyEnv()
	if len(os.Args) > 1 {
		switch {
		case os.Args[1] == "__hylad":
			if err := daemon.Run(daemon.Options{}); err != nil {
				fmt.Fprintln(os.Stderr, "hylad:", err)
				os.Exit(1)
			}
			return
		case cli.IsCommand(os.Args[1]):
			os.Exit(cli.Run(os.Args[1:]))
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
	startAppProfiling()
	cli.SyncSkill()
	// Updates from Hyla's GitHub releases, in a window of native UI.
	mygo.Use(native.Plugin)
	a := newApp()
	loadFonts()
	mygo.Theme.SetSource(mygo.ThemeSource(a.set.Appearance))
	mygo.App.SetLocale(localeTag(a.set.Language))
	mygo.App.SetMenu(a.menu())
	// Windows draw anew in another language; the menu and the titles,
	// set once, are made again in it.
	mygo.App.OnLocaleChanged(func(string) {
		a.update(func() {
			mygo.App.SetMenu(a.menu())
			if a.setWin != nil {
				a.setWin.SetTitle(locales.SettingsTitle())
			}
			if a.pet.win != nil {
				a.pet.win.SetTitle(locales.PetName())
			}
			if a.tray != nil {
				a.tray.SetMenu(a.trayMenu())
			}
		})
	})
	mygo.App.OnSecondInstance(func([]string, string) { a.showWindow() })
	mygo.App.OnDidBecomeActive(func() {
		mygo.ClearNotifications()
		a.update(func() { a.active = true; a.reportView() })
	})
	mygo.App.OnDidResignActive(func() {
		a.update(func() { a.active = false; a.reportView() })
	})
	mygo.App.OnActivate(func(hasVisible bool) {
		if !hasVisible {
			a.showWindow()
		}
	})
	mygo.App.OnWindowAllClosed(a.lastWindowClosed)
	// The question quit-ends-sessions may ask comes before the windows
	// close, and takes the quit back.
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		// quitRequested saves the prefs too.
		if !a.quitRequested() {
			e.PreventDefault()
		}
	})
	// Windows leaves an icon in the notification area after its app until
	// the pointer passes over it.
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if a.tray != nil {
			a.tray.Destroy()
		}
	})
	// No Dock icon from the start for the menu bar alone.
	if runtime.GOOS == "darwin" {
		mygo.App.SetActivationPolicy(activationPolicy(a.set))
	}
	mygo.App.WhenReady(func() {
		a.openWindow()
		a.applyTray()
		a.applyGlobalShortcuts()
		a.idleTime = mygo.Power.IdleTime
		a.syncPet()
		go a.watchReturn()
		a.connect()
		a.syncHosts()
		go a.watchConfig()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// openWindow makes the main window: as Hyla starts, and again after
// closing it left Hyla running in the menu bar or the notification area.
func (a *App) openWindow() {
	a.win = mygo.NewWindow(framed(mygo.WindowOptions{
		Title:     "Hyla",
		Width:     1180,
		Height:    760,
		MinWidth:  560,
		MinHeight: 360,
		StateKey:  "main",
		Vibrancy:  vibrancyFor(a.set),
		Content:   ui.View(a.view),
	}))
	a.winOpen = true
	a.updateTitle()
	a.win.OnFocus(func() { a.update(func() { a.focused = true; a.claimShown(); a.reportView() }) })
	a.win.OnBlur(func() { a.update(func() { a.focused = false; a.reportView() }) })
	a.win.OnClosed(a.windowClosed)
}

// newSessionIn starts a session in a directory of a host, in its project.
func (a *App) newSessionIn(h *host, dir string) { a.create(h, proto.CreateParams{Dir: dir}, "") }

func stopDaemon() error { return cli.StopDaemon() }
