package marks

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
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
		"copilot", "cursor-agent", "cline", "amp", "kimi", "windsurf", "zed", "antigravity", "agy",
	} {
		logo, _, ok := Agent(agent)
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
		if _, _, ok := Agent(agent); ok {
			t.Errorf("%q: want no mark", agent)
		}
	}
	// Every file of the set parses, and every mark the map names is one.
	entries, err := agentFS.ReadDir("agents")
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

// A mark of several colours is kept as its makers drew it, for the row to
// draw in them; a mark of one colour has none, and the row draws it in the
// text's.
func TestMarkFull(t *testing.T) {
	for _, agent := range []string{"antigravity", "agy", "ANTIGRAVITY"} {
		logo, known := MarkFull(agent)
		if !known || logo == nil {
			t.Fatalf("%s: no mark drawn in its colours", agent)
		}
		img := logo.Image(28, 28, color.RGBA{A: 255})
		colours := map[color.RGBA]bool{}
		for y := 0; y < 28; y++ {
			for x := 0; x < 28; x++ {
				if c := img.RGBAAt(x, y); c.A > 200 {
					colours[c] = true
				}
			}
		}
		if len(colours) < 3 {
			t.Errorf("%s: %d colours, want several", agent, len(colours))
		}
	}
	if logo, known := MarkFull("claude"); logo != nil || !known {
		t.Errorf("Claude's one-colour mark: %v, %v", logo, known)
	}
	if logo, known := MarkFull("nonesuch"); logo != nil || known {
		t.Errorf("an agent with no mark: %v, %v", logo, known)
	}
}

// A mark is drawn in the colour of the row's text, whatever colour its
// makers drew it in: the window decides how bright it is.
func TestMarkDrawsInTheTextColour(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	for _, agent := range []string{"claude", "codex", "dsh", "gemini", "kimi"} {
		logo, _, ok := Agent(agent)
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
	if got := Tint(black, true, white); !same(got, white) {
		t.Errorf("a black mark on a dark window = %v, want the text's colour", got)
	}
	if got := Tint(ui.Hex("#18181b"), false, black); !same(got, black) {
		t.Errorf("a black mark on a light window = %v, want the text's colour", got)
	}
	if got := Tint(ui.Hex("#d97757"), false, black); !same(got, ui.Hex("#d97757")) {
		t.Errorf("Claude's orange = %v, want its brand's colour", got)
	}
	if got := Tint(ui.Hex("#ffd21e"), false, black); got.R > 210 || got.G > 210 {
		t.Errorf("Hugging Face's yellow = %v, want it darker on a white window", got)
	}
}

// aPNG is a small image of one colour.
func aPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// TestLogoOf checks which agents have a logo: the set's, the user's where
// the set has none, and none otherwise.
func TestLogoOf(t *testing.T) {
	dir := t.TempDir()
	LoadUserMarks(dir)
	if _, ok := LogoOf("claude"); !ok {
		t.Error("Claude has a logo of the set")
	}
	if _, ok := LogoOf("no-such-agent"); ok {
		t.Error("an agent Hyla has no logo for has one")
	}

	file := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(file, aPNG(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetUserMark(dir, "Acme", file); err != nil {
		t.Fatal(err)
	}
	if _, ok := LogoOf("acme"); !ok {
		t.Error("the agent the user gave a logo has none")
	}
	if _, ok := LogoOf("ACME"); !ok {
		t.Error("an agent's name is not case-sensitive")
	}
	if _, err := os.Stat(filepath.Join(dir, "acme.png")); err != nil {
		t.Errorf("the logo is not kept under the agent's name: %v", err)
	}

	ClearUserMark(dir, "acme")
	if _, ok := LogoOf("acme"); ok {
		t.Error("the logo outlives its file")
	}
}

// TestSetUserMarkRefuses checks that SetUserMark keeps no file that is no
// image, and that a logo reads back as it was given.
func TestSetUserMarkRefuses(t *testing.T) {
	dir := t.TempDir()
	notImage := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(notImage, []byte("not a picture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetUserMark(dir, "acme", notImage); err == nil {
		t.Error("a file that is no image became a logo")
	}
	if err := SetUserMark(dir, "", notImage); err == nil {
		t.Error("a logo without an agent became one")
	}
}

// TestLoadUserMarks checks that the files of a directory load, by their
// names, and only the images.
func TestLoadUserMarks(t *testing.T) {
	dir := t.TempDir()
	png := aPNG()
	for name, data := range map[string][]byte{"acme.png": png, "zeta.JPG": png, "notes.txt": png, "bad.png": []byte("x")} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	LoadUserMarks(dir)
	for _, agent := range []string{"acme", "zeta"} {
		if _, ok := LogoOf(agent); !ok {
			t.Errorf("%s has no logo", agent)
		}
	}
	for _, agent := range []string{"notes", "bad"} {
		if _, ok := LogoOf(agent); ok {
			t.Errorf("%s has a logo of a file that is no image", agent)
		}
	}
}

// TestLogoOfTheUserOverTheSet checks that the set's mark wins for an agent
// Hyla knows, and the user's for one it does not.
func TestLogoOfTheUserOverTheSet(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(file, aPNG(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetUserMark(dir, "claude", file); err != nil {
		t.Fatal(err)
	}
	src, ok := LogoOf("claude")
	if !ok || src != nil {
		t.Errorf("Claude's logo = %v, want its maker's, not the user's", src)
	}
	ClearUserMark(dir, "claude")
}
