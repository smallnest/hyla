// Package glyph holds Hyla's icon set: Framework7's, drawn after SF
// Symbols so that they feel at home on macOS, a few of other sets where it
// has none, and the programs' logos, as Iconify serves them. See
// icons/README.md, at the module's root.
package glyph

import (
	"embed"

	"github.com/egoist/mygo/ui"
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

// Drawn draws an icon of hyla's own, on 16, as a status glyph.
func Drawn(shapes string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round">` + shapes + `</svg>`))
}
