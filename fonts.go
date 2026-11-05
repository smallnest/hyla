package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/settings"
)

// hyla carries JetBrains Mono, with Nerd Fonts' icons (resources/fonts,
// SIL Open Font License, see OFL.txt there): the terminals' font unless
// the user picks another, and the icons of prompts and of agents' TUIs.
// Its weights from thin to extra bold, each file one, are all it draws.

// bundledFont is the family the fonts register as.
const bundledFont = "JetBrainsMono Nerd Font"

// fontsLoaded tells that the fonts registered, so that a terminal asks
// for them only where they are.
var fontsLoaded bool

// loadFonts registers the fonts hyla carries: from the app's resources,
// where `mygo build` and `mygo dev` put them, else from the source tree,
// as under `go run`.
func loadFonts() {
	var dirs []string
	if res, err := mygo.App.Path(mygo.PathResources); err == nil {
		dirs = append(dirs, filepath.Join(res, "fonts"))
	}
	dirs = append(dirs, filepath.Join("resources", "fonts"))
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.ttf"))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			if ui.RegisterFont(data, bundledFont) == nil {
				fontsLoaded = true
			}
		}
		if fontsLoaded {
			return
		}
	}
}

// termFamily returns the family terminals draw with: the user's, else
// the font hyla carries, else the system's monospaced one, then the
// fonts of font-fallback, which draw what it lacks.
func (a *App) termFamily() string {
	f := "monospace" // the system's
	switch {
	case a.set.FontFamily != "":
		f = a.set.FontFamily
	case fontsLoaded:
		f = bundledFont
	}
	return a.set.FallbackFamilies(f)
}

// fallbackFonts are the families of font-fallback, less those the
// terminal's font has already.
func (a *App) fallbackFonts() []string {
	var out []string
	for _, f := range settings.ParseFontFallback(a.set.FontFallback) {
		// The terminal's own font is not a fallback, whichever field names it.
		if strings.EqualFold(f, a.termFamily()) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// addFallback adds family to font-fallback: in place, as the i'th of
// them, else at the end.
func (a *App) addFallback(family string, i int) {
	f := a.fallbackFonts()
	if i < 0 || i > len(f) {
		f = append(f, family)
	} else {
		f = slices.Insert(f, i, family)
	}
	a.set.FontFallback = strings.Join(f, ", ")
}
