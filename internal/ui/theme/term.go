package theme

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/plugins/manifest"
)

var lightTerm = &terminal.Theme{
	Foreground:    ui.Hex("#1d1d1f"),
	Background:    Light.Card,
	Cursor:        ui.Hex("#1d1d1f"),
	CursorText:    ui.Hex("#ffffff"),
	Selection:     ui.Hex("#cfdcfb"),
	SelectionText: ui.Hex("#1d1d1f"),
	Palette: [16]ui.Color{
		ui.Hex("#1d1d1f"), ui.Hex("#d3302f"), ui.Hex("#22863a"), ui.Hex("#b08800"),
		ui.Hex("#2f5bd8"), ui.Hex("#a033c4"), ui.Hex("#0f8199"), ui.Hex("#8e8e93"),
		ui.Hex("#5f5f66"), ui.Hex("#ef4444"), ui.Hex("#2fa84f"), ui.Hex("#d4a017"),
		ui.Hex("#3b82f6"), ui.Hex("#c04ee0"), ui.Hex("#14a3bf"), ui.Hex("#b4b4bb"),
	},
}

var darkTerm = &terminal.Theme{
	Foreground:    ui.Hex("#e6e6ea"),
	Background:    Dark.Card,
	Cursor:        ui.Hex("#e6e6ea"),
	CursorText:    ui.Hex("#161618"),
	Selection:     ui.Hex("#2f4170"),
	SelectionText: ui.Hex("#ffffff"),
	Palette: [16]ui.Color{
		ui.Hex("#3a3a40"), ui.Hex("#ff6b64"), ui.Hex("#4fd17a"), ui.Hex("#f2c14e"),
		ui.Hex("#6b9bff"), ui.Hex("#d07cf0"), ui.Hex("#4cc9e0"), ui.Hex("#c9c9cf"),
		ui.Hex("#6c6c78"), ui.Hex("#ff8a84"), ui.Hex("#74e397"), ui.Hex("#ffd479"),
		ui.Hex("#8fb3ff"), ui.Hex("#e09cf7"), ui.Hex("#7fdcec"), ui.Hex("#f5f5f7"),
	},
}

var inkstoneLightTerm = &terminal.Theme{
	Foreground:    InkstoneLight.Text,
	Background:    InkstoneLight.Card,
	Cursor:        InkstoneLight.Text,
	CursorText:    InkstoneLight.Card,
	Selection:     ui.Hex("#d5e2d5"),
	SelectionText: InkstoneLight.Text,
	Palette:       lightTerm.Palette,
}

var inkstoneDarkTerm = &terminal.Theme{
	Foreground:    InkstoneDark.Text,
	Background:    InkstoneDark.Card,
	Cursor:        InkstoneDark.Text,
	CursorText:    InkstoneDark.Sidebar,
	Selection:     ui.Hex("#2b4232"),
	SelectionText: ui.Hex("#ffffff"),
	Palette:       darkTerm.Palette,
}

// LightTerm is the terminal's colors on the light palette's card, and
// DarkTerm on the dark one's: Apple's system colors, a little deeper in
// the light.
func LightTerm() *terminal.Theme { return lightTerm }

// DarkTerm is the dark appearance's terminal theme, as LightTerm is the
// light one's.
func DarkTerm() *terminal.Theme { return darkTerm }

// LightTermFor returns the built-in light terminal theme for background.
func LightTermFor(background string) *terminal.Theme {
	if background == "inkstone" {
		return inkstoneLightTerm
	}
	return lightTerm
}

// DarkTermFor returns the built-in dark terminal theme for background.
func DarkTermFor(background string) *terminal.Theme {
	if background == "inkstone" {
		return inkstoneDarkTerm
	}
	return darkTerm
}

var (
	pluginThemeMu sync.RWMutex
	pluginThemes  = map[string]*terminal.Theme{}
	pluginNames   []string
)

// ReloadPluginThemes parses plugin-supplied .ghostty theme files and replaces
// active plugin terminal themes atomically.
func ReloadPluginThemes(assets []manifest.ThemeAsset) error {
	next := map[string]*terminal.Theme{}
	var errs []error
	for _, a := range assets {
		if a.Name == "" || a.Path == "" {
			continue
		}
		if a.Name == "hyla" {
			errs = append(errs, fmt.Errorf("plugin %s theme cannot override reserved name %q", a.PluginID, a.Name))
			continue
		}
		data, err := os.ReadFile(a.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("theme %s/%s: %w", a.PluginID, a.Name, err))
			continue
		}
		t, err := terminal.ParseGhosttyTheme(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("theme %s/%s: %w", a.PluginID, a.Name, err))
			continue
		}
		next[a.Name] = t
	}
	names := make([]string, 0, len(next))
	for k := range next {
		names = append(names, k)
	}
	sort.Strings(names)
	pluginThemeMu.Lock()
	pluginThemes = next
	pluginNames = names
	pluginThemeMu.Unlock()
	return errors.Join(errs...)
}

// TerminalThemes returns all selectable terminal theme names: "hyla", built-in
// Ghostty themes, and enabled plugin Ghostty themes.
func TerminalThemes() []string {
	base := terminal.GhosttyThemes()
	seen := make(map[string]bool, len(base)+1)
	seen["hyla"] = true
	for _, n := range base {
		seen[n] = true
	}
	out := make([]string, 0, 1+len(base))
	out = append(out, "hyla")
	out = append(out, base...)
	pluginThemeMu.RLock()
	for _, n := range pluginNames {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	pluginThemeMu.RUnlock()
	return out
}

// TerminalTheme resolves a named Ghostty theme, checking plugin-supplied
// themes when built-in Ghostty themes do not contain name.
func TerminalTheme(name string) (*terminal.Theme, error) {
	if name == "" || name == "hyla" {
		return nil, fmt.Errorf("built-in theme %q uses LightTerm/DarkTerm", name)
	}
	pluginThemeMu.RLock()
	pt := pluginThemes[name]
	pluginThemeMu.RUnlock()
	if t, err := terminal.GhosttyTheme(name); err == nil {
		return t, nil
	} else if pt != nil {
		return pt, nil
	} else {
		return nil, err
	}
}
