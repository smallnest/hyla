package main

import (
	"image/color"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The sidebar's marks: the agents hylad knows and their makers drew a logo
// for wear it, every file of the set parses, and an agent with no logo
// shows none rather than another agent's.
func TestAgentMarks(t *testing.T) {
	for _, agent := range []string{
		"claude", "codex", "opencode", "pi", "dsh", "gemini", "qwen",
		"copilot", "cursor-agent", "cline", "amp", "kimi", "windsurf", "zed",
	} {
		logo, _, ok := agentMark(agent)
		if !ok || logo == nil {
			t.Errorf("%s: no mark", agent)
			continue
		}
		if w, h := logo.Size(); w <= 0 || h <= 0 {
			t.Errorf("%s: mark is %v×%v", agent, w, h)
		}
	}
	// Aider, Goose, Crush, Kiro and the rest never drew one.
	for _, agent := range []string{"", "aider", "goose", "crush", "kiro-cli", "droid", "auggie", "nonesuch"} {
		if _, _, ok := agentMark(agent); ok {
			t.Errorf("%q: want no mark", agent)
		}
	}
	// Every file of the set parses, and every mark the map names is one.
	entries, err := agentMarkFS.ReadDir("resources/icons/agents")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, e := range entries {
		file, ok := strings.CutSuffix(e.Name(), ".svg")
		if !ok {
			continue
		}
		files[file] = true
		if _, ok := markOf(file); !ok {
			t.Errorf("%s.svg: does not parse", file)
		}
	}
	for agent, file := range agentMarks {
		if !files[file] {
			t.Errorf("%s: mark %s.svg is not in the set", agent, file)
		}
	}
}

// A mark is drawn in the colour of the row's text, whatever colour its
// makers drew it in: the window decides how bright it is.
func TestMarkDrawsInTheTextColour(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	for _, agent := range []string{"claude", "codex", "dsh", "gemini", "kimi"} {
		logo, _, ok := agentMark(agent)
		if !ok {
			t.Fatalf("%s: no mark", agent)
		}
		img := logo.Image(28, 28, red)
		painted := 0
		for y := 0; y < 28; y++ {
			for x := 0; x < 28; x++ {
				if c := img.RGBAAt(x, y); c.R > 200 && c.G < 60 && c.B < 60 && c.A > 200 {
					painted++
				}
			}
		}
		if painted == 0 {
			t.Errorf("%s: the mark is not drawn in the text's colour", agent)
		}
	}
}

// A near-black mark reads as the row's text on both windows, and the
// palest brands are pushed away from the window's background.
func TestMarkTint(t *testing.T) {
	white, black := ui.Hex("#ffffff"), ui.Hex("#000000")
	same := func(a, b ui.Color) bool { return a.R == b.R && a.G == b.G && a.B == b.B }
	if got := markTint(black, true, white); !same(got, white) {
		t.Errorf("a black mark on a dark window = %v, want the text's colour", got)
	}
	if got := markTint(ui.Hex("#18181b"), false, black); !same(got, black) {
		t.Errorf("a black mark on a light window = %v, want the text's colour", got)
	}
	if got := markTint(ui.Hex("#d97757"), false, black); !same(got, ui.Hex("#d97757")) {
		t.Errorf("Claude's orange = %v, want its brand's colour", got)
	}
	if got := markTint(ui.Hex("#ffd21e"), false, black); got.R > 210 || got.G > 210 {
		t.Errorf("Hugging Face's yellow = %v, want it darker on a white window", got)
	}
}
