// Package marks holds the agents' marks, which the sidebar puts before a
// session's name: the logos their makers drew, as agents/README.md tells
// which one is whose. They are Simple Icons' SVGs (CC0, the brands their
// owners' trademarks), as Iconify serves them, and each file's fill is its
// brand's colour.
package marks

import (
	"embed"
	"regexp"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Size is how large a mark is, and the slot every session row keeps for
// one: names line up even where a row has no agent.
const Size = 14

// The agents' marks, which the sidebar puts before a session's name: the
// logos their makers drew, as agents/README.md tells which
// one is whose. They are Simple Icons' SVGs (CC0, the brands their owners'
// trademarks), as Iconify serves them, and each file's fill is its brand's
// colour.
//
//go:embed agents/*.svg
var agentFS embed.FS

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
	"antigravity":    "antigravity",
	"agy":            "antigravity",
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

// Agent returns the mark of the agent named, and the colour its makers
// draw it in: false for an agent with none, and for none at all.
func Agent(agent string) (*ui.SVG, ui.Color, bool) {
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
	b, err := agentFS.ReadFile("agents/" + file + ".svg")
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

// Tint is a brand's colour as a window shows it: a near-black mark, as
// Codex's or OpenCode's, is the colour of the row's text, so that it reads
// on a dark window, and the palest brands are pushed off a white one.
func Tint(brand ui.Color, dark bool, text ui.Color) ui.Color {
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
