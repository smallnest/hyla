package main

import (
	"sync"

	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
	"hyla/internal/ui/theme"
)

// applied is the settings the window shows: the last Apply's.
var applied struct {
	sync.RWMutex
	s settings.Settings
}

// appliedSettings is the settings the window shows, for what it builds.
func appliedSettings() settings.Settings {
	applied.RLock()
	defer applied.RUnlock()
	return applied.s
}

// borderless returns the theme with hyla's look: its accent, and no lines
// around controls, which stand out by their fill: white fields and
// buttons on soft panels.
func borderless(c *ui.Context, pal *theme.Palette) *ui.Theme {
	t := *c.Theme()
	// Text is the setting's: the font, its size and weight, which the
	// terminal shows too.
	s := appliedSettings()
	if s.FontSize >= 8 && s.FontSize <= 30 {
		t.FontSize = s.FontSize
	}
	if s.FontFamily != "" {
		t.Font = s.FontFamily
	}
	t.Accent = pal.Accent
	t.Selection = pal.Accent.Alpha(0.25)
	t.Border = ui.Transparent
	t.Surface = pal.Control
	t.SurfaceHover = pal.ControlHover
	t.SurfacePressed = pal.ControlHover.Mix(pal.Text, 0.06)
	return &t
}
