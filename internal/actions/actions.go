// Package actions names what hyla's app does, for its menus, its command
// palette, the shortcuts the user rebinds in Settings, and `hyla do`,
// which runs them from a shell or an agent (Remote Control).
package actions

import (
	"runtime"
	"strings"
	"sync"
)

// Action is something the app does.
type Action struct {
	ID    string
	Title string
	Menu  string // the menu it shows in
	Keys  string // the default accelerator, as "CmdOrCtrl+Shift+D"; "" none
	// Remote tells that `hyla do` may run it: what needs no dialog or
	// focus of the user's.
	Remote bool
	// Global tells that its shortcut works in every app, as the system's
	// global shortcut, rather than as a key of the app's menus.
	Global bool
}

// All are the app's actions, in their menus' order.
var All = []Action{
	{ID: "settings.open", Title: "Settings…", Menu: "Hyla", Keys: "CmdOrCtrl+,", Remote: true},
	{ID: "cli.install", Title: "Install Command Line Tool…", Menu: "Hyla"},
	{ID: "app.quitAll", Title: "Quit and End All Sessions…", Menu: "Hyla", Keys: "CmdOrCtrl+Alt+Q"},

	{ID: "session.new", Title: "New Session", Menu: "Session", Keys: "CmdOrCtrl+T", Remote: true},
	{ID: "session.newHome", Title: "New Session in Home", Menu: "Session", Keys: "CmdOrCtrl+Shift+T", Remote: true},
	{ID: "project.open", Title: "Open Project…", Menu: "Session", Keys: "CmdOrCtrl+O"},
	{ID: "palette.open", Title: "Go to Session…", Menu: "Session", Keys: "CmdOrCtrl+P", Remote: true},
	{ID: "session.attention", Title: "Jump to Session Needing You", Menu: "Session", Keys: "CmdOrCtrl+J", Remote: true},
	{ID: "session.next", Title: "Next Session", Menu: "Session", Keys: "CmdOrCtrl+Shift+]", Remote: true},
	{ID: "session.previous", Title: "Previous Session", Menu: "Session", Keys: "CmdOrCtrl+Shift+[", Remote: true},
	{ID: "session.rename", Title: "Rename Session", Menu: "Session", Keys: "CmdOrCtrl+R"},
	{ID: "pane.restart", Title: "Restart Pane", Menu: "Session", Keys: "CmdOrCtrl+Shift+R", Remote: true},
	{ID: "pane.close", Title: "Close", Menu: "Session", Keys: "CmdOrCtrl+W"},
	{ID: "session.close", Title: "Close Session", Menu: "Session", Keys: "CmdOrCtrl+Shift+W"},

	{ID: "find.open", Title: "Find…", Menu: "Edit", Keys: "CmdOrCtrl+F"},
	{ID: "find.next", Title: "Find Next", Menu: "Edit", Keys: "CmdOrCtrl+G"},
	{ID: "find.previous", Title: "Find Previous", Menu: "Edit", Keys: "CmdOrCtrl+Shift+G"},

	{ID: "pane.split.right", Title: "Split Right", Menu: "Pane", Keys: "CmdOrCtrl+D", Remote: true},
	{ID: "pane.split.down", Title: "Split Down", Menu: "Pane", Keys: "CmdOrCtrl+Shift+D", Remote: true},
	{ID: "pane.browser", Title: "New Browser Pane", Menu: "Pane", Keys: "CmdOrCtrl+Shift+B", Remote: true},
	{ID: "browser.address", Title: "Open Location…", Menu: "Pane", Keys: "CmdOrCtrl+L"},
	{ID: "pane.zoom", Title: "Zoom Pane", Menu: "Pane", Keys: "CmdOrCtrl+Shift+Enter", Remote: true},
	{ID: "pane.next", Title: "Next Pane", Menu: "Pane", Keys: "CmdOrCtrl+]", Remote: true},
	{ID: "pane.previous", Title: "Previous Pane", Menu: "Pane", Keys: "CmdOrCtrl+[", Remote: true},
	{ID: "pane.left", Title: "Pane Left", Menu: "Pane", Keys: "CmdOrCtrl+Alt+Left", Remote: true},
	{ID: "pane.right", Title: "Pane Right", Menu: "Pane", Keys: "CmdOrCtrl+Alt+Right", Remote: true},
	{ID: "pane.up", Title: "Pane Above", Menu: "Pane", Keys: "CmdOrCtrl+Alt+Up", Remote: true},
	{ID: "pane.down", Title: "Pane Below", Menu: "Pane", Keys: "CmdOrCtrl+Alt+Down", Remote: true},

	{ID: "sidebar.toggle", Title: "Toggle Sidebar", Menu: "View", Keys: "CmdOrCtrl+B", Remote: true},
	{ID: "side.toggle", Title: "Toggle Side Panel", Menu: "View", Keys: "CmdOrCtrl+Shift+E", Remote: true},
	{ID: "changes.toggle", Title: "Toggle Changes", Menu: "View", Keys: "CmdOrCtrl+Alt+G", Remote: true},
	{ID: "files.toggle", Title: "Toggle Files", Menu: "View", Keys: "CmdOrCtrl+Alt+F", Remote: true},
	{ID: "info.toggle", Title: "Toggle Info", Menu: "View", Keys: "CmdOrCtrl+Alt+I", Remote: true},
	{ID: "pet.toggle", Title: "Toggle Hyla", Menu: "View", Keys: "CmdOrCtrl+Shift+K"},
	{ID: "text.bigger", Title: "Bigger", Menu: "View", Keys: "CmdOrCtrl+=", Remote: true},
	{ID: "text.smaller", Title: "Smaller", Menu: "View", Keys: "CmdOrCtrl+-", Remote: true},
	{ID: "text.reset", Title: "Actual Size", Menu: "View", Keys: "CmdOrCtrl+0", Remote: true},
	// Control+`, which Ghostty's documentation gives and works in every
	// app: the user may give it another, or none, in Settings › Keyboard.
	{ID: "quickTerminal.toggle", Title: "Quick Terminal", Menu: "View", Keys: "Ctrl+`", Remote: true, Global: true},
}

// pcKeys are the defaults on Windows and Linux, where Control is the
// terminal's: Control+T, D, W or R belong to the shell, so the shortcuts add
// Shift, as Windows Terminal's do, or Alt; "" leaves an action without one.
var pcKeys = map[string]string{
	"settings.open":     "Ctrl+,",
	"app.quitAll":       "",
	"session.new":       "Ctrl+Shift+T",
	"session.newHome":   "",
	"project.open":      "Ctrl+Shift+O",
	"palette.open":      "Ctrl+Shift+P",
	"session.attention": "Ctrl+Shift+J",
	"session.next":      "Ctrl+PageDown",
	"session.previous":  "Ctrl+PageUp",
	"session.rename":    "",
	"pane.restart":      "Ctrl+Shift+R",
	"pane.close":        "Ctrl+Shift+W",
	"session.close":     "",
	"find.open":         "Ctrl+Shift+F",
	"find.next":         "",
	"find.previous":     "",
	"pane.split.right":  "Alt+Shift+=",
	"pane.split.down":   "Alt+Shift+-",
	"pane.browser":      "Alt+Shift+B",
	"browser.address":   "",
	"pane.zoom":         "Ctrl+Shift+Enter",
	"pane.next":         "Ctrl+Shift+]",
	"pane.previous":     "Ctrl+Shift+[",
	"pane.left":         "Alt+Left",
	"pane.right":        "Alt+Right",
	"pane.up":           "Alt+Up",
	"pane.down":         "Alt+Down",
	"sidebar.toggle":    "Ctrl+Shift+B",
	"side.toggle":       "Ctrl+Shift+E",
	"changes.toggle":    "Ctrl+Shift+G",
	"files.toggle":      "Ctrl+Alt+F",
	"info.toggle":       "Ctrl+Alt+I",
	"pet.toggle":        "Ctrl+Shift+K",
	"text.bigger":       "Ctrl+=",
	"text.smaller":      "Ctrl+-",
	"text.reset":        "Ctrl+0",
}

// linuxKeys are the defaults on Linux alone, where the quick terminal's
// Control+Shift+T is GTK's paste, which the system takes before Hyla
// when something is selected in another app.
var linuxKeys = map[string]string{
	"quickTerminal.toggle": "Ctrl+`",
}

func init() {
	if runtime.GOOS == "darwin" {
		return
	}
	for i, a := range All {
		if k, ok := pcKeys[a.ID]; ok {
			All[i].Keys = k
		}
		if k, ok := linuxKeys[a.ID]; ok && runtime.GOOS == "linux" {
			All[i].Keys = k
		}
	}
}

// Find returns an action by its id.
func Find(id string) (Action, bool) {
	for _, a := range All {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}

// Keys returns an action's accelerator: the user's, over the default.
func Keys(a Action, overrides map[string]string) string {
	if k, ok := overrides[a.ID]; ok {
		return k
	}
	return a.Keys
}

// glyphs are the accelerators written already: a view draws a shortcut
// every frame, and writing one splits it and bends it to the system's
// symbols each time.
var glyphs sync.Map // accelerator → its glyphs

// Glyphs writes an accelerator as the system's menus show it: "⇧⌘D" on
// macOS, "Ctrl+Shift+D" elsewhere.
func Glyphs(accel string) string {
	if accel == "" {
		return ""
	}
	if g, ok := glyphs.Load(accel); ok {
		return g.(string)
	}
	g := glyphsOf(accel)
	glyphs.Store(accel, g)
	return g
}

// glyphsOf is Glyphs' own writing, once an accelerator.
func glyphsOf(accel string) string {
	parts := strings.Split(accel, "+")
	key := parts[len(parts)-1]
	if key == "" && len(parts) > 1 { // "CmdOrCtrl++"
		key = "+"
	}
	if runtime.GOOS != "darwin" {
		return words(parts[:len(parts)-1], key)
	}
	var ctrl, alt, shift, cmd bool
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "cmdorctrl", "commandorcontrol", "cmd", "command", "super", "meta":
			cmd = true
		case "ctrl", "control":
			ctrl = true
		case "alt", "option":
			alt = true
		case "shift":
			shift = true
		}
	}
	var b strings.Builder
	if ctrl {
		b.WriteString("⌃")
	}
	if alt {
		b.WriteString("⌥")
	}
	if shift {
		b.WriteString("⇧")
	}
	if cmd {
		b.WriteString("⌘")
	}
	names := map[string]string{
		"enter": "↩", "return": "↩", "tab": "⇥", "space": "Space", "backspace": "⌫", "delete": "⌦",
		"escape": "⎋", "esc": "⎋", "up": "↑", "down": "↓", "left": "←", "right": "→",
		"home": "↖", "end": "↘", "pageup": "⇞", "pagedown": "⇟", "plus": "+",
	}
	if n, ok := names[strings.ToLower(key)]; ok {
		b.WriteString(n)
	} else {
		b.WriteString(strings.ToUpper(key))
	}
	return b.String()
}

// words writes an accelerator as Windows and Linux menus show it:
// "Ctrl+Shift+D".
func words(mods []string, key string) string {
	var ctrl, alt, shift, super bool
	for _, m := range mods {
		switch strings.ToLower(m) {
		case "cmdorctrl", "commandorcontrol", "ctrl", "control":
			ctrl = true
		case "cmd", "command", "super", "meta":
			super = true
		case "alt", "option":
			alt = true
		case "shift":
			shift = true
		}
	}
	var out []string
	if super && runtime.GOOS == "windows" {
		out = append(out, "Win")
	} else if super {
		out = append(out, "Super")
	}
	if ctrl {
		out = append(out, "Ctrl")
	}
	if alt {
		out = append(out, "Alt")
	}
	if shift {
		out = append(out, "Shift")
	}
	names := map[string]string{
		"enter": "Enter", "return": "Enter", "tab": "Tab", "space": "Space", "backspace": "Backspace", "delete": "Del",
		"escape": "Esc", "esc": "Esc", "up": "↑", "down": "↓", "left": "←", "right": "→",
		"home": "Home", "end": "End", "pageup": "PgUp", "pagedown": "PgDn", "plus": "+",
	}
	if n, ok := names[strings.ToLower(key)]; ok {
		key = n
	} else {
		key = strings.ToUpper(key)
	}
	return strings.Join(append(out, key), "+")
}
