// Package marks holds the agents' marks, which the sidebar puts before a
// session's name: the logos their makers drew, as agents/README.md tells
// which one is whose. They are Simple Icons' SVGs (CC0, the brands their
// owners' trademarks), as Iconify serves them, and each file's fill is its
// brand's colour.
package marks

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/manifest"
	"hyla/internal/proto"
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
// its makers draw it in, for the sidebar to show. full is the logo as its
// makers drew it, for a mark of several colours.
type mark struct {
	logo  *ui.SVG
	brand ui.Color
	full  *ui.SVG
}

// The marks drawn in the colours their makers chose, not the row's:
// Antigravity's is a picture of several colours under a white shape, which
// the text's colour would leave blank. Every other mark is one shape,
// drawn in the row's text colour and tinted to the window.
var fullColor = map[string]bool{
	"antigravity": true,
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
	key := strings.ToLower(strings.TrimSpace(agent))
	if file, ok := agentMarks[key]; ok {
		if m, ok := markOf(file); ok {
			return m.logo, m.brand, true
		}
	}
	pluginMu.RLock()
	m, ok := pluginSVG[key]
	pluginMu.RUnlock()
	if ok && m.logo != nil {
		return m.logo, m.brand, true
	}
	return nil, ui.Color{}, false
}

// MarkFull returns the agent's mark as its makers drew it, where it has
// several colours, which the row draws in them: nil for a mark drawn in one
// colour, which the row draws in its text's. known is false for an agent
// Hyla has no mark for.
func MarkFull(agent string) (full *ui.SVG, known bool) {
	key := strings.ToLower(strings.TrimSpace(agent))
	if file, ok := agentMarks[key]; ok {
		m, ok := markOf(file)
		if !ok {
			return nil, false
		}
		if fullColor[file] {
			return m.full, true
		}
		return nil, true
	}
	pluginMu.RLock()
	m, ok := pluginSVG[key]
	isFull := pluginSVGFull[key]
	pluginMu.RUnlock()
	if !ok {
		return nil, false
	}
	if isFull {
		return m.full, true
	}
	return nil, true
}

func parseMarkSVG(b []byte, isFull bool) (mark, bool) {
	// A mark of several colours is kept as drawn: its shapes take their own
	// colours, and a shape drawn in a colour is not the text's.
	var full *ui.SVG
	if isFull {
		if s, err := ui.ParseSVG([]byte(markTitle.ReplaceAll(b, nil))); err == nil {
			full = s
		}
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
		return mark{}, false
	}
	return mark{logo: logo, brand: brand, full: full}, true
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
	m, ok := parseMarkSVG(b, fullColor[file])
	if !ok {
		markCache[file] = mark{}
		return mark{}, false
	}
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

// --- the user's own marks ---

// User marks are the logos the user gives the agents Hyla has none for,
// as an image of any size, kept as a file named after the agent, as
// acme.png, in MarksDir: the set's logos win for an agent the set has
// one for.
var (
	pluginMu      sync.RWMutex
	pluginSVG     = map[string]mark{}
	pluginSVGFull = map[string]bool{}
	pluginBitmaps = map[string]*ui.Bitmap{}

	userMu    sync.RWMutex
	userMarks = map[string]*ui.Bitmap{}
)

// LoadPluginMarks refreshes plugin-contributed agent marks (SVG or bitmap),
// positioned in precedence between built-in marks and user marks.
func LoadPluginMarks(assets []manifest.MarkAsset) {
	nextSVG := map[string]mark{}
	nextFull := map[string]bool{}
	nextBmp := map[string]*ui.Bitmap{}
	for _, a := range assets {
		key := strings.ToLower(strings.TrimSpace(a.Agent))
		if key == "" || a.Path == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(a.Path))
		if ext == ".svg" {
			data, err := os.ReadFile(a.Path)
			if err != nil {
				continue
			}
			if m, ok := parseMarkSVG(data, a.FullColor); ok {
				nextSVG[key] = m
				if a.FullColor {
					nextFull[key] = true
				}
			}
			continue
		}
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
			if bmp := readBitmap(a.Path); bmp != nil {
				nextBmp[key] = bmp
			}
		}
	}
	pluginMu.Lock()
	pluginSVG = nextSVG
	pluginSVGFull = nextFull
	pluginBitmaps = nextBmp
	pluginMu.Unlock()
}

// LogoOf is an agent's logo, as the sidebar and a card draw it: its own
// mark where the set or a plugin has it, else the user's where they gave it.
// known is false for an agent whose logo is neither.
func LogoOf(agent string) (src ui.ImageSource, known bool) {
	if _, _, ok := Agent(agent); ok {
		return nil, true
	}
	key := strings.ToLower(strings.TrimSpace(agent))
	pluginMu.RLock()
	pb := pluginBitmaps[key]
	pluginMu.RUnlock()
	if pb != nil {
		return pb, true
	}
	if b := userMark(agent); b != nil {
		return b, true
	}
	return nil, false
}

// MarksDir is where the user's marks are: "marks" in the app's directory.
func MarksDir() string { return filepath.Join(proto.Dir(), "marks") }

// LoadUserMarks reads the user's marks in dir into the cache, replacing
// those it read before: the window calls it as it starts, and a mark the
// user gives.
func LoadUserMarks(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		userMu.Lock()
		userMarks = map[string]*ui.Bitmap{}
		userMu.Unlock()
		return
	}
	next := map[string]*ui.Bitmap{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		default:
			continue
		}
		agent := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		if agent == "" {
			continue
		}
		if b := readBitmap(filepath.Join(dir, name)); b != nil {
			next[agent] = b
		}
	}
	userMu.Lock()
	userMarks = next
	userMu.Unlock()
}

// SetUserMark gives an agent the logo in a file, which it keeps in dir
// under the agent's name: a name the set has no mark for keeps its file's
// extension. The file must be an image DecodeImage reads.
func SetUserMark(dir, agent, file string) error {
	agent = strings.ToLower(strings.TrimSpace(agent))
	if agent == "" {
		return errors.New("no agent")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if _, err := ui.DecodeImage(data); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
	default:
		ext = ".png"
	}
	dst := filepath.Join(dir, agent+ext)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return err
	}
	LoadUserMarks(dir)
	return nil
}

// RemoveUserMark forgets an agent's own logo, as ClearUserMark does with
// the directory's files, and the agent's mark when the file is gone.
func RemoveUserMark(agent string) { ClearUserMark(MarksDir(), agent) }

// ClearUserMark forgets an agent's own logo, if it has one.
func ClearUserMark(dir, agent string) {
	agent = strings.ToLower(strings.TrimSpace(agent))
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp"} {
		_ = os.Remove(filepath.Join(dir, agent+ext))
	}
	LoadUserMarks(dir)
}

// userMark is the user's logo of an agent, if any.
func userMark(agent string) *ui.Bitmap {
	agent = strings.ToLower(strings.TrimSpace(agent))
	if agent == "" {
		return nil
	}
	userMu.RLock()
	defer userMu.RUnlock()
	return userMarks[agent]
}

// readBitmap decodes a file as a logo, if it is one DecodeImage reads.
func readBitmap(path string) *ui.Bitmap {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src, err := ui.DecodeImage(data)
	if err != nil {
		return nil
	}
	b, ok := src.(*ui.Bitmap)
	if !ok {
		return nil
	}
	return b
}
