// Package glyph holds Hyla's icon set: Framework7's, drawn after SF
// Symbols so that they feel at home on macOS, a few of other sets where it
// has none, and the programs' logos, as Iconify serves them. See
// icons/README.md, at the module's root.
package glyph

import (
	"embed"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
)

//go:embed icons/*.svg
var files embed.FS

// Of returns an icon of the set, as "f7-gear-alt".
func Of(name string) *ui.SVG {
	b, err := files.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic("hyla: no icon " + name)
	}
	return ui.MustParseSVG(b)
}

// Try safely resolves either a named shortcut ("terminal", "info", "gear", …)
// or an embedded icon file name ("f7-gear-alt", "lucide-info") without panicking.
func Try(name string) (*ui.SVG, bool) {
	if name == "" || strings.ContainsAny(name, `/\.`) {
		return nil, false
	}
	switch name {
	case "sidebar":
		return Sidebar, true
	case "plus":
		return Plus, true
	case "chevron":
		return Chevron, true
	case "folder":
		return Folder, true
	case "doc":
		return Doc, true
	case "search":
		return Search, true
	case "terminal":
		return Terminal, true
	case "gear":
		return Gear, true
	case "host":
		return Host, true
	case "branch":
		return Branch, true
	case "diff":
		return Diff, true
	case "globe":
		return Globe, true
	case "reload":
		return Reload, true
	case "copy":
		return Copy, true
	case "info":
		return Info, true
	case "link":
		return Link, true
	case "chat":
		return Chat, true
	case "edit":
		return Edit, true
	case "close":
		return Close, true
	case "archive":
		return Archive, true
	case "sort":
		return Sort, true
	case "plugins", "plugin":
		return Plugins, true
	case "drama":
		return Drama, true
	case "connect":
		return Connect, true
	case "disconnect":
		return Disconnect, true
	}
	b, err := files.ReadFile("icons/" + name + ".svg")
	if err != nil {
		return nil, false
	}
	svg, err := ui.ParseSVG(b)
	if err != nil {
		return nil, false
	}
	return svg, true
}

// Drawn draws an icon of hyla's own, on 16, as a status glyph.
func Drawn(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

// The app's icons, by what they draw: every view reads them from here.
var (
	Sidebar     = Of("f7-sidebar-left")
	Plus        = Of("f7-plus")
	Chevron     = Of("f7-chevron-right")
	ChevronUp   = Of("f7-chevron-up")
	ChevronDown = Of("f7-chevron-down")
	Folder      = Of("f7-folder")
	Doc         = Of("f7-doc-text")
	Search      = Of("f7-search")
	Terminal    = Of("meteor-terminal")
	Return      = Of("f7-return")
	Gear        = Of("f7-gear-alt")
	Host        = Of("f7-desktopcomputer")
	SplitRight  = Of("f7-square-split-2x1")
	SplitDown   = Of("f7-square-split-1x2")
	Zoom        = Of("f7-arrow-up-left-arrow-down-right")
	Unzoom      = Of("f7-arrow-down-right-arrow-up-left")
	Close       = Of("f7-xmark")
	Swap        = Of("f7-arrow-right-arrow-left")
	Branch      = Of("f7-arrow-branch")
	Diff        = Of("octicon-file-diff-16")
	Revert      = Of("f7-arrow-uturn-left")
	OpenFile    = Of("f7-arrow-up-right-square")
	Globe       = Of("f7-globe")
	ChevronLeft = Of("f7-chevron-left")
	Reload      = Of("f7-arrow-clockwise")
	Copy        = Of("f7-doc-on-doc")
	Info        = Of("lucide-info")
	Link        = Of("f7-arrow-up-right")
	Connect     = Drawn(`<path d="M6 2.5v3M10 2.5v3M4.2 5.5h7.6v2.2a3.8 3.8 0 0 1-7.6 0zM8 11.5v2"/>`)
	Disconnect  = Drawn(`<path d="M8 2.2v5.3"/><path d="M4.6 4.5a5.2 5.2 0 1 0 6.8 0"/>`)
	Chat        = Drawn(`<path d="M2.5 3.5a2 2 0 0 1 2-2h7a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2H6.5L3 13.5v-3a2 2 0 0 1-.5-1.3z"/><path d="M5.5 5.5h5M5.5 8h3"/>`)
	Zed         = Of("si-zed")
	Edit        = Of("f7-pencil")
	InfoCopy    = Drawn(`<rect x="5.5" y="5.5" width="8" height="8" rx="2"/><path d="M10.5 5.5v-1a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v4a2 2 0 0 0 2 2h1"/>`)
	CPU         = Drawn(`<rect x="2.5" y="2.5" width="11" height="11" rx="2.5"/><rect x="5.75" y="5.75" width="4.5" height="4.5" rx="1"/><path d="M6 1v1.5M10 1v1.5M6 13.5V15M10 13.5V15M1 6h1.5M1 10h1.5M13.5 6H15M13.5 10H15"/>`)
	Archive     = Drawn(`<rect x="2" y="3" width="12" height="3" rx="1"/><path d="M3 6v6.5a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V6M6.5 9h3"/>`)
	Sort        = Drawn(`<path d="M3 4.5h9M3 8h6M3 11.5h3"/><path d="M12 4.5v7M9.5 9 12 11.5 14.5 9"/>`)
	// Subagents is an agent and the two works it gave: a tree of dots, as
	// the side panel's Subagents view shows them.
	Subagents = Drawn(`<circle cx="8" cy="3" r="1.9"/><circle cx="3.6" cy="12.6" r="1.9"/><circle cx="12.4" cy="12.6" r="1.9"/><path d="M7.2 4.7 4.4 10.9M8.8 4.7l2.8 6.2"/>`)
	// The sidebar's Collapse All and Expand All: each group's chevron, on
	// the lines they fold.
	FoldAll   = Drawn(`<path d="M2.5 4h11M2.5 8h5M2.5 12h5"/><path d="M11 6.5 13.5 9 11 11.5"/>`)
	UnfoldAll = Drawn(`<path d="M2.5 4h11M2.5 8h5M2.5 12h5"/><path d="M10 11.5 12.5 9 15 11.5"/>`)
	// K8s is the Kubernetes wheel: a heptagon around a ship's wheel,
	// as the logo draws it, in Hyla's stroke.
	K8s = Drawn(`<path d="M8 1.7 13.2 4.2 14.4 9.8 10.9 14.3 5.1 14.3 1.6 9.8 2.8 4.2Z"/><circle cx="8" cy="8.3" r="1.9"/><path d="M8 6.4V3.3M9.5 7.2l2.7-2M9.8 8.6l3.4.7M8.9 9.9l1.5 3.1M7.1 9.9l-1.5 3.1M6.2 8.6l-3.4.7M6.5 7.2l-2.7-2"/>`)
	// Plugins is a modular grid with one offset piece joining it.
	Plugins = Drawn(`<rect x="2.2" y="2.2" width="5" height="5" rx="1.2"/><rect x="8.8" y="8.8" width="5" height="5" rx="1.2"/><rect x="2.2" y="8.8" width="5" height="5" rx="1.2"/><rect x="9.4" y="1.6" width="5" height="5" rx="1.2"/>`)
	// Drama and player controls for Hongguo short dramas.
	Drama       = Drawn(`<rect x="2.5" y="2" width="11" height="12" rx="2.2"/><path d="M6.5 5.4v5.2l4.2-2.6z"/>`)
	Play        = Drawn(`<path d="M4.5 3.2v9.6l8-4.8z"/>`)
	Pause       = Drawn(`<path d="M5.2 3.5v9M10.8 3.5v9"/>`)
	Volume      = Drawn(`<path d="M2.5 6.2H5l3.5-2.8v9.2L5 9.8H2.5z"/><path d="M11 5.6a3.4 3.4 0 0 1 0 4.8M12.9 3.9a5.8 5.8 0 0 1 0 8.2"/>`)
	VolumeMuted = Drawn(`<path d="M2.5 6.2H5l3.5-2.8v9.2L5 9.8H2.5z"/><path d="M11.2 6l3.3 4M14.5 6l-3.3 4"/>`)
	History     = Drawn(`<path d="M3.2 4.3A5.8 5.8 0 1 1 2.2 8"/><path d="M2.4 2.2v2.7h2.7"/><path d="M8 5.2v3l2.1 1.3"/>`)
	Episodes    = Drawn(`<rect x="2.2" y="2.5" width="4.6" height="4.6" rx="1"/><rect x="9.2" y="2.5" width="4.6" height="4.6" rx="1"/><rect x="2.2" y="9.2" width="4.6" height="4.6" rx="1"/><rect x="9.2" y="9.2" width="4.6" height="4.6" rx="1"/>`)
	Record      = Drawn(`<circle cx="8" cy="8" r="4.8" fill="currentColor" stroke="none"/>`)
	StopSquare  = Drawn(`<rect x="4" y="4" width="8" height="8" rx="1.6" fill="currentColor" stroke="none"/>`)
)

// MenuIcon is an icon as a menu item's image, made once: the items that
// show it share what the backend makes of it (MenuItem.Icon).
func MenuIcon(s *ui.SVG) *mygo.MenuIcon {
	ic, err := mygo.NewMenuIconSVG(s, true)
	if err != nil {
		panic("hyla: menu icon: " + err.Error())
	}
	return ic
}

// The sidebar's + menu, its items' icons: each is the icon the item's
// subject wears elsewhere in the app, a terminal, a folder, a machine and
// the Kubernetes wheel. Starting a session at home has none.
var (
	NewSessionMenuIcon  = MenuIcon(Terminal)
	OpenProjectMenuIcon = MenuIcon(Folder)
	AddSSHMenuIcon      = MenuIcon(Host)
	AddK8sMenuIcon      = MenuIcon(K8s)
)

// programIcons are the logos of programs a pane may run, by the name hylad
// gives the foreground program, its agent or its script's runtime.
var ProgramIcons = map[string]*ui.SVG{}

func init() {
	for name, logo := range map[string]string{
		"claude": "claude", "codex": "openai", "gemini": "googlegemini", "opencode": "opencode",
		"amp": "amp", "cursor-agent": "cursor", "copilot": "githubcopilot", "qwen": "qwen",
		"kimi": "kimi", "cline": "cline", "zed": "zed", "antigravity": "antigravity", "agy": "antigravity",
		"node": "nodedotjs", "bun": "bun", "deno": "deno", "python": "python", "python3": "python",
		"vim": "vim", "nvim": "neovim", "git": "git", "docker": "docker",
	} {
		ProgramIcons[name] = Of("si-" + logo)
	}
}

// ProgramIcon is the logo of the pane's foreground program, as Claude's
// for Claude Code and Node.js's for a script node runs; the terminal's
// while the shell is in the foreground or the program has none; a globe
// for a browser, a page for a file.
func ProgramIcon(p *proto.Pane) *ui.SVG {
	if p.FilePane() {
		return Doc
	}
	if p.Browser() {
		return Globe
	}
	if p.HongguoPane() {
		return Drama
	}
	for _, name := range []string{p.Agent, p.Process, p.Runtime} {
		if i := ProgramIcons[name]; i != nil {
			return i
		}
	}
	return Terminal
}

// The Files tab's icons.
var (
	Collapse = Of("f7-rectangle-compress-vertical")
)

// Sync is the two arrows of a sync: the configuration going up, and the
// others' coming down.
var Sync = Drawn(`<path d="M2.5 8a5.5 5.5 0 0 1 9.4-3.9"/><path d="M13.5 8a5.5 5.5 0 0 1-9.4 3.9"/><path d="M12.5 1.3v3.2H9.3M3.5 14.7v-3.2h3.2"/>`)
