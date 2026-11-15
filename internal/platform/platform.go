// Package platform is what Hyla asks of the system it runs on: the file
// manager, the app that opens a file, the editor of a text file, and the
// frame of its windows.
package platform

import (
	"os/exec"
	"runtime"

	"github.com/egoist/mygo"

	"hyla/locales"
)

// OwnControls tells that Hyla draws the window controls itself: on
// Windows and Linux the top-right card's header holds minimize, maximize
// and close, beside Hyla's own buttons, where the system's, at the
// window's edge, would straddle the card. macOS keeps its traffic lights.
var OwnControls = runtime.GOOS != "darwin"

// FileManager is the system's file manager, as buttons that show a file
// in it name it.
func FileManager() string {
	switch runtime.GOOS {
	case "darwin":
		return locales.FileManagerFinder()
	case "windows":
		return locales.FileManagerExplorer()
	}
	return locales.FileManagerFiles()
}

// OpenPath opens a file or folder in the app that opens it.
func OpenPath(path string) { mygo.Shell.OpenPath(path) }

// RevealPath shows a file in the file manager.
func RevealPath(path string) { mygo.Shell.ShowItemInFolder(path) }

// OpenTextFile opens a text file, as the configuration, which has no
// extension to tell its app by, in the text editor.
func OpenTextFile(path string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("/usr/bin/open", "-t", path).Start()
	case "windows":
		exec.Command("notepad.exe", path).Start()
	default:
		exec.Command("xdg-open", path).Start()
	}
}

// Frame sets a window's frame: the title bar hidden, under the traffic
// lights on macOS and without its buttons elsewhere (OwnControls), which
// keeps the edges that resize the window. topBar is the height of the
// header the lights sit in, and mid where its middle is.
func Frame(o mygo.WindowOptions, topBar, mid int) mygo.WindowOptions {
	// The wash may be translucent, and with blur off the desktop shows
	// through the window itself, not a material.
	o.Transparent = true
	if OwnControls {
		o.TitleBarStyle = mygo.TitleBarHidden
		o.HideWindowButtons = true
		// No menu bar over the sidebar and the cards: Alt and F10 show the
		// menus, whose shortcuts work all the same.
		o.AutoHideMenuBar = true
		return o
	}
	o.TitleBarStyle = mygo.TitleBarHiddenInset
	o.TitleBarHeight = topBar
	// The lights' buttons are 16 tall, and MyGo puts them a DIP above the
	// position: centered on mid, as the sidebar's buttons and the panes'
	// headers are.
	o.TrafficLightPosition = &mygo.Point{X: 18, Y: mid - 7}
	return o
}
