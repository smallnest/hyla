package main

import (
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/theme"
)

// searchEntry is a setting the Settings window's search finds: its page,
// its label as its row shows it, and words that find it besides, as its
// key in the configuration file.
type searchEntry struct {
	page, label, words string
}

// settingsPagesOf are the Settings' pages, as the search knows them.
func settingsPagesOf() []search.Page {
	pages := make([]search.Page, 0, len(settingsPages))
	for _, p := range settingsPages {
		pages = append(pages, search.Page{ID: p.id, Title: p.title()})
	}
	return pages
}

// searchResults returns what a query finds, by page.
func searchResults(query string, shortcuts map[string]string) map[string][]string {
	return search.Results(query, shortcuts, settingsPagesOf(), actionTitle)
}

// The search's finds on the page shown, by label, which settingRow and
// settingsGroup highlight as they build; nil while not searching. The
// Settings window is the only view that builds them, and sets these
// before it does.
var (
	searchHits   map[string]bool
	searchReveal string // the find to scroll to, in this build only
)

// searchHighlight marks a find of the search behind el, a row or a
// title, as a tint wider than it by dx on each side, and taller by dy.
func searchHighlight(c *ui.Context, el ui.Element, label string, pal *theme.Palette, dx, dy float32) {
	hit := searchHits[label]
	t := el.Animate("hit", b2f(hit), 180*time.Millisecond)
	if label == searchReveal {
		el.ScrollIntoView()
	}
	if t > 0 {
		ui.Box(c.Key("hit")).Absolute().Top(-dy).Bottom(-dy).Left(-dx).Right(-dx).Radius(8).PassThrough().
			Background(pal.Accent.Alpha(0.13*t)).InsetShadow(0, 0, 0, 1, pal.Accent.Alpha(0.28*t))
	}
}
