package main

import (
	"embed"
	"regexp"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"hyla/internal/proto"
)

// The icons are Framework7's, drawn after SF Symbols so that they feel at
// home on macOS, a few of other sets where it has none, and programs'
// logos from Simple Icons, as Iconify serves them: see icons/README.md.
//
//go:embed icons/*.svg
var iconFiles embed.FS

// glyph returns an icon of the set, as "f7-gear-alt".
func glyph(name string) *ui.SVG {
	b, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic("hyla: no icon " + name)
	}
	return ui.MustParseSVG(b)
}

// icon draws an icon of hyla's own, on 16, as a status glyph.
func icon(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}

var (
	sidebarIcon     = glyph("f7-sidebar-left")
	plusIcon        = glyph("f7-plus")
	chevronIcon     = glyph("f7-chevron-right")
	chevronUpIcon   = glyph("f7-chevron-up")
	chevronDownIcon = glyph("f7-chevron-down")
	folderIcon      = glyph("f7-folder")
	docIcon         = glyph("f7-doc-text")
	searchIcon      = glyph("f7-search")
	terminalIcon    = glyph("meteor-terminal")
	returnIcon      = glyph("f7-return")
	gearIcon        = glyph("f7-gear-alt")
	hostIcon        = glyph("f7-desktopcomputer")
	splitRightIcon  = glyph("f7-square-split-2x1")
	splitDownIcon   = glyph("f7-square-split-1x2")
	zoomIcon        = glyph("f7-arrow-up-left-arrow-down-right")
	unzoomIcon      = glyph("f7-arrow-down-right-arrow-up-left")
	closeIcon       = glyph("f7-xmark")
	swapIcon        = glyph("f7-arrow-right-arrow-left")
	branchIcon      = glyph("f7-arrow-branch")
	diffIcon        = glyph("octicon-file-diff-16")
	revertIcon      = glyph("f7-arrow-uturn-left")
	openFileIcon    = glyph("f7-arrow-up-right-square")
	globeIcon       = glyph("f7-globe")
	chevronLeftIcon = glyph("f7-chevron-left")
	reloadIcon      = glyph("f7-arrow-clockwise")
	copyIcon        = glyph("f7-doc-on-doc")
	infoIcon        = glyph("lucide-info")
	linkIcon        = glyph("f7-arrow-up-right")
	chatIcon        = icon(`<path d="M2.5 3.5a2 2 0 0 1 2-2h7a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2H6.5L3 13.5v-3a2 2 0 0 1-.5-1.3z"/><path d="M5.5 5.5h5M5.5 8h3"/>`)
	zedIcon         = glyph("si-zed")
	editIcon        = glyph("f7-pencil")
	infoCopyIcon    = icon(`<rect x="5.5" y="5.5" width="8" height="8" rx="2"/><path d="M10.5 5.5v-1a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v4a2 2 0 0 0 2 2h1"/>`)
	cpuIcon         = icon(`<rect x="2.5" y="2.5" width="11" height="11" rx="2.5"/><rect x="5.75" y="5.75" width="4.5" height="4.5" rx="1"/><path d="M6 1v1.5M10 1v1.5M6 13.5V15M10 13.5V15M1 6h1.5M1 10h1.5M13.5 6H15M13.5 10H15"/>`)
	archiveIcon     = icon(`<rect x="2" y="3" width="12" height="3" rx="1"/><path d="M3 6v6.5a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V6M6.5 9h3"/>`)
	sortIcon        = icon(`<path d="M3 4.5h9M3 8h6M3 11.5h3"/><path d="M12 4.5v7M9.5 9 12 11.5 14.5 9"/>`)
	// The sidebar's Collapse All and Expand All: each group's chevron, on
	// the lines they fold.
	foldAllIcon   = icon(`<path d="M2.5 4h11M2.5 8h5M2.5 12h5"/><path d="M11 6.5 13.5 9 11 11.5"/>`)
	unfoldAllIcon = icon(`<path d="M2.5 4h11M2.5 8h5M2.5 12h5"/><path d="M10 11.5 12.5 9 15 11.5"/>`)
	// k8sIcon is the Kubernetes wheel: a heptagon around a ship's wheel,
	// as the logo draws it, in Hyla's stroke.
	k8sIcon = icon(`<path d="M8 1.7 13.2 4.2 14.4 9.8 10.9 14.3 5.1 14.3 1.6 9.8 2.8 4.2Z"/><circle cx="8" cy="8.3" r="1.9"/><path d="M8 6.4V3.3M9.5 7.2l2.7-2M9.8 8.6l3.4.7M8.9 9.9l1.5 3.1M7.1 9.9l-1.5 3.1M6.2 8.6l-3.4.7M6.5 7.2l-2.7-2"/>`)
)

// menuIcon is an icon as a menu item's image, made once: the items that
// show it share what the backend makes of it (MenuItem.Icon).
func menuIcon(s *ui.SVG) *mygo.MenuIcon {
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
	newSessionMenuIcon  = menuIcon(terminalIcon)
	openProjectMenuIcon = menuIcon(folderIcon)
	addSSHMenuIcon      = menuIcon(hostIcon)
	addK8sMenuIcon      = menuIcon(k8sIcon)
)

// programIcons are the logos of programs a pane may run, by the name hylad
// gives the foreground program, its agent or its script's runtime.
var programIcons = map[string]*ui.SVG{}

func init() {
	for name, logo := range map[string]string{
		"claude": "claude", "codex": "openai", "gemini": "googlegemini", "opencode": "opencode",
		"amp": "amp", "cursor-agent": "cursor", "copilot": "githubcopilot", "qwen": "qwen",
		"kimi": "kimi", "cline": "cline", "zed": "zed",
		"node": "nodedotjs", "bun": "bun", "deno": "deno", "python": "python", "python3": "python",
		"vim": "vim", "nvim": "neovim", "git": "git", "docker": "docker",
	} {
		programIcons[name] = glyph("si-" + logo)
	}
}

// programIcon is the logo of the pane's foreground program, as Claude's
// for Claude Code and Node.js's for a script node runs; the terminal's
// while the shell is in the foreground or the program has none; a globe
// for a browser, a page for a file.
func programIcon(p *proto.Pane) *ui.SVG {
	if p.FilePane() {
		return docIcon
	}
	if p.Browser() {
		return globeIcon
	}
	for _, name := range []string{p.Agent, p.Process, p.Runtime} {
		if i := programIcons[name]; i != nil {
			return i
		}
	}
	return terminalIcon
}

// The Files tab's icons.
var (
	collapseIcon = glyph("f7-rectangle-compress-vertical")
)

// The agents' marks, which the sidebar puts before a session's name: the
// logos their makers drew, as resources/icons/agents/README.md tells which
// one is whose. They are Simple Icons' SVGs (CC0, the brands their owners'
// trademarks), as Iconify serves them, and each file's fill is its brand's
// colour.
//
//go:embed resources/icons/agents/*.svg
var agentMarkFS embed.FS

// agentMarks tells, for an agent as hylad names it, the logo it wears. An
// agent whose makers drew none has no mark in the sidebar, rather than
// another agent's, which would name the wrong one.
var agentMarks = map[string]string{
	"claude":         "claude",
	"codex":          "openai",
	"openai":         "openai",
	"opencode":       "opencode",
	"pi":             "pi",
	"dsh":            "deepseek",
	"deepseek":       "deepseek",
	"gemini":         "googlegemini",
	"google":         "google",
	"amp":            "amp",
	"cursor-agent":   "cursor",
	"cursor":         "cursor",
	"copilot":        "githubcopilot",
	"github-copilot": "githubcopilot",
	"cline":          "cline",
	"qwen":           "qwen",
	"kimi":           "moonshotai",
	"moonshot":       "moonshotai",
	"windsurf":       "windsurf",
	"zed":            "zedindustries",
	"jetbrains":      "jetbrains",
	"trae":           "trae",
	"mistral":        "mistralai",
	"ollama":         "ollama",
	"llama":          "ollama",
	"huggingface":    "huggingface",
	"minimax":        "minimax",
	"doubao":         "bytedance",
	"bytedance":      "bytedance",
	"alibabacloud":   "alibabacloud",
	"anthropic":      "anthropic",
	"vercel":         "vercel",
	"warp":           "warp",
	"replit":         "replit",
	"perplexity":     "perplexity",
	"nvidia":         "nvidia",
	"meta":           "meta",
	"ray":            "ray",
	"langchain":      "langchain",
	"langgraph":      "langgraph",
	"crewai":         "crewai",
	"n8n":            "n8n",
	"dify":           "dify",
	"vllm":           "vllm",
	"lmstudio":       "lmstudio",
	"modal":          "modal",
}

// A mark, drawn once: the logo in the colour of the text, and the colour
// its makers draw it in, for the sidebar to show.
type mark struct {
	logo  *ui.SVG
	brand ui.Color
}

var (
	markMu    sync.Mutex
	markCache = map[string]mark{}

	// markTitle is a logo's own name, which nothing draws.
	markTitle = regexp.MustCompile(`<title>[^<]*</title>`)
	// markFill is a logo's own colour, in its opening tag.
	markFill = regexp.MustCompile(`(<svg[^>]*?)fill="([^"]*)"`)
)

// agentMark returns the mark of the agent named, and the colour its makers
// draw it in: false for an agent with none, and for none at all.
func agentMark(agent string) (*ui.SVG, ui.Color, bool) {
	file, ok := agentMarks[strings.ToLower(strings.TrimSpace(agent))]
	if !ok {
		return nil, ui.Color{}, false
	}
	m, ok := markOf(file)
	if !ok {
		return nil, ui.Color{}, false
	}
	return m.logo, m.brand, true
}

// markOf reads a logo of the set, once.
func markOf(file string) (mark, bool) {
	markMu.Lock()
	defer markMu.Unlock()
	if m, ok := markCache[file]; ok {
		return m, m.logo != nil
	}
	b, err := agentMarkFS.ReadFile("resources/icons/agents/" + file + ".svg")
	if err != nil {
		// A mark that is not in the set is no mark, which a test catches
		// before anything ships.
		markCache[file] = mark{}
		return mark{}, false
	}
	brand := ui.Hex("#000000")
	if m := markFill.FindSubmatch(b); m != nil {
		brand = markColor(string(m[2]), brand)
	}
	s := string(markTitle.ReplaceAll(b, nil))
	// The logo's own colour gives way to the text's, which the window
	// decides: the mark is the row's, in the brand's colour.
	if i := strings.Index(s, "<svg"); i >= 0 {
		if j := strings.IndexByte(s[i:], '>'); j > 0 {
			tag := s[i : i+j]
			if t := markFill.ReplaceAllString(tag, `${1}fill="currentColor"`); t != tag {
				tag = t
			} else {
				tag = strings.TrimSuffix(tag, "/") + ` fill="currentColor"`
			}
			s = s[:i] + tag + s[i+j:]
		}
	}
	logo, err := ui.ParseSVG([]byte(s))
	if err != nil {
		markCache[file] = mark{}
		return mark{}, false
	}
	m := mark{logo: logo, brand: brand}
	markCache[file] = m
	return m, true
}

// markColor parses a fill as a colour, black when it is not one: a logo
// drawn in currentColor, as some are, is the text's colour.
func markColor(fill string, fallback ui.Color) ui.Color {
	if len(fill) != 4 && len(fill) != 7 || !strings.HasPrefix(fill, "#") {
		return fallback
	}
	for _, r := range fill[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return fallback
		}
	}
	return ui.Hex(fill)
}

// markTint is a brand's colour as a window shows it: a near-black mark, as
// Codex's or OpenCode's, is the colour of the row's text, so that it reads
// on a dark window, and the palest brands are pushed off a white one.
func markTint(brand ui.Color, dark bool, text ui.Color) ui.Color {
	l := (0.299*float64(brand.R) + 0.587*float64(brand.G) + 0.114*float64(brand.B)) / 255
	switch {
	case l < 0.28:
		return text
	case dark && l < 0.45:
		return brand.Mix(ui.Hex("#ffffff"), 0.35)
	case !dark && l > 0.66:
		return brand.Mix(ui.Hex("#000000"), 0.3)
	}
	return brand
}
