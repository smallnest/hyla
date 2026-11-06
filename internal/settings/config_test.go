package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"hyla/internal/actions"
)

func TestFormatKeepsTheUsersFile(t *testing.T) {
	file := `# my hyla
font-size = 14

# colors
theme = light:Catppuccin Latte,dark:Catppuccin Mocha
some-future-key = yes
env = A=1
env = B=2
`
	s, probs := parse(file)
	if s.FontSize != 14 || s.LightTheme != "Catppuccin Latte" || s.DarkTheme != "Catppuccin Mocha" {
		t.Fatalf("parsed %+v", s)
	}
	if !reflect.DeepEqual(s.Env, []string{"A=1", "B=2"}) {
		t.Errorf("env %q", s.Env)
	}
	if len(probs) != 1 || !strings.Contains(probs[0], "some-future-key") {
		t.Errorf("problems %q", probs)
	}
	s.FontSize = 15
	s.Env = []string{"C=3"}
	s.CursorBlink = false
	out := format(s, file)
	want := `# my hyla
font-size = 15

# colors
theme = light:Catppuccin Latte,dark:Catppuccin Mocha
some-future-key = yes
env = C=3
cursor-style-blink = false
`
	if out != want {
		t.Errorf("formatted:\n%s\nwant:\n%s", out, want)
	}
	back, _ := parse(out)
	if !reflect.DeepEqual(back, s) {
		t.Errorf("read back %+v, want %+v", back, s)
	}
}

func TestFormatNewFileHasOnlyChanges(t *testing.T) {
	s := Default()
	settingLines := func(out string) int {
		n := 0
		for _, l := range strings.Split(out, "\n") {
			if l != "" && !strings.HasPrefix(l, "#") {
				n++
			}
		}
		return n
	}
	if out := format(s, ""); settingLines(out) != 0 {
		t.Errorf("defaults wrote settings:\n%s", out)
	}
	s.FontFamily, s.Appearance = "Iosevka Term", "dark"
	out := format(s, "")
	if !strings.Contains(out, "window-theme = dark\n") || !strings.Contains(out, "font-family = Iosevka Term\n") {
		t.Errorf("formatted:\n%s", out)
	}
	if n := settingLines(out); n != 2 {
		t.Errorf("%d settings written, want 2:\n%s", n, out)
	}
}

func TestKeybinds(t *testing.T) {
	s, probs := parse("keybind = cmd+shift+x=pane.split.right\nkeybind = cmd+d=unbind\nkeybind = cmd+==text.bigger\nkeybind = ctrl+q=nope\n")
	if s.Shortcuts["pane.split.right"] != "CmdOrCtrl+Shift+X" {
		t.Errorf("split right %q", s.Shortcuts["pane.split.right"])
	}
	if v, ok := s.Shortcuts["pane.split.right"]; !ok || v == "" {
		t.Errorf("split right lost to the unbind: %q", v)
	}
	if s.Shortcuts["text.bigger"] != "CmdOrCtrl+=" {
		t.Errorf("bigger %q", s.Shortcuts["text.bigger"])
	}
	if len(probs) != 1 || !strings.Contains(probs[0], "nope") {
		t.Errorf("problems %q", probs)
	}
	// Unbinding keys frees the action that has them.
	split, _ := actions.Find("pane.split.right")
	keys := configKeys(split.Keys) // cmd+d on macOS
	s2, _ := parse("keybind = " + keys + "=unbind\n")
	if v, ok := s2.Shortcuts["pane.split.right"]; !ok || v != "" {
		t.Errorf("%s unbound, split right %q %v", keys, v, ok)
	}
	if out := format(s2, ""); !strings.Contains(out, "keybind = "+keys+"=unbind\n") {
		t.Errorf("formatted:\n%s", out)
	}
}

// Ghostty's global: prefix binds the quick terminal as Ghostty's
// configuration writes it.
func TestGlobalKeybind(t *testing.T) {
	s, probs := parse("keybind = global:ctrl+`=quickTerminal.toggle\n")
	if len(probs) != 0 || s.Shortcuts["quickTerminal.toggle"] != "Ctrl+`" {
		t.Errorf("quick terminal %q, problems %q", s.Shortcuts["quickTerminal.toggle"], probs)
	}
	if out := format(s, ""); !strings.Contains(out, "keybind = ctrl+`=quickTerminal.toggle\n") {
		t.Errorf("formatted:\n%s", out)
	}
}

// font-fallback holds the fonts after font-family, in order.
func TestFontFallback(t *testing.T) {
	s, probs := parse("font-fallback = PingFang SC,  Apple Color Emoji\n")
	if len(probs) != 0 || strings.Join(ParseFontFallback(s.FontFallback), ", ") != "PingFang SC, Apple Color Emoji" {
		t.Errorf("fallback %q, problems %q", s.FontFallback, probs)
	}
	if got, want := s.FallbackFamilies("Menlo"), "Menlo, PingFang SC,  Apple Color Emoji"; got != want {
		t.Errorf("families %q, want %q", got, want)
	}
	s, _ = parse("font-fallback = \n")
	if s.FontFallback != "" || s.FallbackFamilies("Menlo") != "Menlo" {
		t.Errorf("an empty fallback %q", s.FontFallback)
	}
}

func TestBadValuesKeepDefaults(t *testing.T) {
	s, probs := parse("cursor-style = triangle\nfont-size = big\nconfirm-close = maybe\n")
	d := Default()
	if s.Cursor != d.Cursor || s.FontSize != d.FontSize || s.ConfirmClose != d.ConfirmClose {
		t.Errorf("bad values changed settings: %+v", s)
	}
	if len(probs) != 3 {
		t.Errorf("problems %q", probs)
	}
}

// The terminals' translucency reads its switch and number, and keeps the
// default for the number out of its range.
func TestPaneTranslucency(t *testing.T) {
	s, probs := parse("pane-opacity = 0.4\npane-blur = true\npane-material = glass-clear\n")
	if len(probs) != 0 || s.PaneOpacity != 0.4 || !s.PaneBlur || s.PaneMaterial != "glass-clear" {
		t.Errorf("pane-opacity %v, pane-blur %v, pane-material %q, problems %q",
			s.PaneOpacity, s.PaneBlur, s.PaneMaterial, probs)
	}
	d := Default()
	if d.PaneOpacity != DefaultPaneOpacity || d.PaneBlur || d.PaneMaterial != "glass" {
		t.Errorf("defaults %v, %v, %q", d.PaneOpacity, d.PaneBlur, d.PaneMaterial)
	}
	s, _ = parse("pane-opacity = 2\n")
	if s.PaneOpacity != d.PaneOpacity {
		t.Errorf("out of range: pane-opacity %v, pane-blur %v", s.PaneOpacity, s.PaneBlur)
	}
	if _, probs := parse("pane-material = nope"); len(probs) == 0 {
		t.Error("a terminal material Hyla has not was taken")
	}
}

// Hyla elsewhere updates the app here only once the user lets it.
// The window's material is a choice, the sidebar's by default, and one
// Hyla has not is refused.
func TestWindowMaterial(t *testing.T) {
	s, probs := parse("window-material = glass")
	if len(probs) != 0 || s.WindowMaterial != "glass" {
		t.Errorf("window-material glass: %q, %v", s.WindowMaterial, probs)
	}
	if d := Default(); d.WindowMaterial != "sidebar" {
		t.Errorf("default window-material %q", d.WindowMaterial)
	}
	if _, probs := parse("window-material = nope"); len(probs) == 0 {
		t.Error("a material Hyla has not was taken")
	}
}

// The Dock's is a wash of its own, as the Dock's background is.
func TestDockBackground(t *testing.T) {
	s, probs := parse("background = dock")
	if len(probs) != 0 || s.Background != "dock" {
		t.Errorf("background dock: %q, %v", s.Background, probs)
	}
	if !slices.ContainsFunc(Backgrounds, func(b struct{ ID, Name string }) bool { return b.ID == "dock" }) {
		t.Error("the Dock's background is not one Settings offers")
	}
}

// Dusk is gone: a configuration that still names it gets Petal.
func TestDuskBackgroundMigrates(t *testing.T) {
	s, probs := parse("background = dusk\n")
	if len(probs) != 0 || s.Background != "rex" {
		t.Errorf("background dusk: %q, %v", s.Background, probs)
	}
}

func TestRemoteUpdatesOptIn(t *testing.T) {
	if Default().RemoteUpdates {
		t.Error("remote updates are on by default")
	}
	s, probs := parse("remote-updates = true\n")
	if !s.RemoteUpdates || len(probs) != 0 {
		t.Errorf("remote-updates = true: %v, problems %q", s.RemoteUpdates, probs)
	}
}

func TestMigratesSettingsJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HYLA_HOME", dir)
	t.Setenv("HYLA_CONFIG", filepath.Join(dir, "config", "hyla", "config"))
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"fontSize": 16, "appearance": "dark", "background": "rex"}`), 0o600)
	mu.Lock()
	cache = nil
	mu.Unlock()
	s := Load()
	if s.FontSize != 16 || s.Appearance != "dark" {
		t.Fatalf("migrated %+v", s)
	}
	b, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "font-size = 16") || strings.Contains(string(b), "background") {
		t.Errorf("config:\n%s", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err == nil {
		t.Error("settings.json stayed")
	}
}

func TestAdLists(t *testing.T) {
	tests := []struct {
		file     string
		want     []string
		problems int
	}{
		{"", []string{"easylist", "easyprivacy"}, 0},
		{"browser-filter-list = easylist\nbrowser-filter-list = peter-lowe\nbrowser-filter-list = easylist", []string{"easylist", "peter-lowe"}, 0},
		{"browser-filter-list =", []string{}, 0},
		{"browser-filter-list = easylist\nbrowser-filter-list =\nbrowser-filter-list = fanboy-cookie", []string{"fanboy-cookie"}, 0},
		{"browser-filter-list = https://lists.example/ads.txt", []string{"https://lists.example/ads.txt"}, 0},
		{"browser-filter-list = easylist\nbrowser-filter-list = no-such-list\nbrowser-filter-list = ftp://lists.example/x", []string{"easylist"}, 1},
	}
	for _, tt := range tests {
		s, probs := parse(tt.file)
		if !reflect.DeepEqual(s.AdLists, tt.want) || len(probs) != tt.problems {
			t.Errorf("%q: lists %q, problems %q", tt.file, s.AdLists, probs)
		}
		// What is saved reads back the same: no lists as none, not the
		// defaults.
		if back, _ := parse(format(s, "")); !reflect.DeepEqual(back.AdLists, s.AdLists) {
			t.Errorf("%q: saved and read back %q, want %q", tt.file, back.AdLists, s.AdLists)
		}
	}
	if out := format(Default(), ""); strings.Contains(out, "browser-filter-list") {
		t.Errorf("the default lists were written:\n%s", out)
	}
}

func TestSaveKeepsASymlinkedConfig(t *testing.T) {
	dir := t.TempDir()
	dotfiles := filepath.Join(dir, "dotfiles", "hyla", "config")
	os.MkdirAll(filepath.Dir(dotfiles), 0o755)
	os.WriteFile(dotfiles, []byte("# mine\nfont-size = 14\n"), 0o600)
	link := filepath.Join(dir, "config", "hyla", "config")
	os.MkdirAll(filepath.Dir(link), 0o755)
	if err := os.Symlink("../../dotfiles/hyla/config", link); err != nil {
		t.Skip(err) // Windows without the privilege
	}
	t.Setenv("HYLA_HOME", dir)
	t.Setenv("HYLA_CONFIG", link)
	mu.Lock()
	cache = nil
	mu.Unlock()
	s := Load()
	s.FontSize = 16
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink went: %v %v", st, err)
	}
	b, _ := os.ReadFile(dotfiles)
	if string(b) != "# mine\nfont-size = 16\n" {
		t.Errorf("dotfiles' config:\n%s", b)
	}
	if st, _ := os.Stat(dotfiles); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode().Perm())
	}
}

func TestTargetFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "dotfiles", "config") // not there yet, as before stow's first save
	mid := filepath.Join(dir, "mid")
	link := filepath.Join(dir, "config")
	if err := os.Symlink("dotfiles/config", mid); err != nil {
		t.Skip(err)
	}
	os.Symlink(mid, link)
	if got := target(link); got != file {
		t.Errorf("target(link) = %s, want %s", got, file)
	}
	if got := target(file); got != file {
		t.Errorf("target(file) = %s", got)
	}
}

// TestMouseScrollMultiplier: Ghostty's mouse-scroll-multiplier, with its
// precision: and discrete: prefixes or one number for both, kept in
// Ghostty's bounds.
func TestMouseScrollMultiplier(t *testing.T) {
	for _, c := range []struct {
		line                string
		precision, discrete float32
	}{
		{"", 1, 3},
		{"mouse-scroll-multiplier = 0.5", 0.5, 0.5},
		{"mouse-scroll-multiplier = precision:0.5,discrete:3", 0.5, 3},
		{"mouse-scroll-multiplier = discrete:1", 1, 1},
		{"mouse-scroll-multiplier = precision: 2 , discrete: 6", 2, 6},
		{"mouse-scroll-multiplier = precision:0,discrete:20000", 0.01, 10000},
	} {
		s, probs := parse(c.line + "\n")
		if len(probs) != 0 || s.ScrollPrecision != c.precision || s.ScrollDiscrete != c.discrete {
			t.Errorf("%q: precision %v, discrete %v, problems %q; want %v, %v", c.line, s.ScrollPrecision, s.ScrollDiscrete, probs, c.precision, c.discrete)
		}
		back, _ := parse(format(s, ""))
		if back.ScrollPrecision != s.ScrollPrecision || back.ScrollDiscrete != s.ScrollDiscrete {
			t.Errorf("%q: read back %v, %v", c.line, back.ScrollPrecision, back.ScrollDiscrete)
		}
	}
	for _, bad := range []string{"mouse-scroll-multiplier = fast", "mouse-scroll-multiplier = wheel:2"} {
		if _, probs := parse(bad + "\n"); len(probs) != 1 {
			t.Errorf("%q: problems %q, want one", bad, probs)
		}
	}
	if out := format(Default(), ""); strings.Contains(out, "mouse-scroll-multiplier =") {
		t.Errorf("a new file sets the default:\n%s", out)
	}
}

func TestFontFeature(t *testing.T) {
	for _, c := range []struct {
		file     string
		want     []string
		problems int
	}{
		{"", nil, 0},
		{"font-feature = ss01", []string{"ss01"}, 0},
		{"font-feature = ss01, -calt , cv05=2", []string{"ss01", "-calt", "cv05=2"}, 0},
		{"font-feature = +ss01\nfont-feature = calt=0", []string{"+ss01", "calt=0"}, 0},
		{"font-feature = ss01\nfont-feature =\nfont-feature = ss02", []string{"ss02"}, 0},
		{"font-feature = ss01, ligatures, -calt=1, cv05=x, ss2", []string{"ss01"}, 1},
	} {
		s, probs := parse(c.file + "\n")
		if !reflect.DeepEqual(s.FontFeatures, c.want) || len(probs) != c.problems {
			t.Errorf("%q: features %q, problems %q; want %q", c.file, s.FontFeatures, probs, c.want)
		}
		if back, _ := parse(format(s, "")); !reflect.DeepEqual(back.FontFeatures, s.FontFeatures) {
			t.Errorf("%q: read back %q", c.file, back.FontFeatures)
		}
	}
	if out := format(Default(), ""); strings.Contains(out, "font-feature") {
		t.Errorf("a new file sets font-feature:\n%s", out)
	}
}

// font-fallback gives the terminals the user's fallback fonts, after their
// font, which the platform draws the glyphs missing from.
func TestFallbackFamilies(t *testing.T) {
	s := Settings{FontFallback: " PingFang SC ,Apple Color Emoji"}
	if got, want := s.FallbackFamilies("Menlo"), "Menlo,  PingFang SC ,Apple Color Emoji"; got != want {
		t.Errorf("families %q, want %q", got, want)
	}
	s = Settings{FontFallback: "  "}
	if got := s.FallbackFamilies("Menlo"); got != "Menlo" {
		t.Errorf("an empty fallback gave %q", got)
	}
}
