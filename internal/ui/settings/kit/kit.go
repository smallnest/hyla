// Package kit draws the Settings window's parts: a group of settings and
// its rows, a segmented control, and the search's marks on them.
package kit

import (
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/theme"
)

// groupTitle is a group's title, which the search highlights as it does
// a setting.
func GroupTitle(c *ui.Context, pal *theme.Palette, title string) {
	row := ui.Row(c.Key("title-"+title)).Padding(0, 4)
	row.Children(func() {
		Highlight(c, row, title, pal, 2, 3)
		ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(pal.TextMuted)
	})
}

// group is a titled set of settings, on a soft panel.
func Group(c *ui.Context, pal *theme.Palette, title, note string, fn func()) {
	ui.Column(c).Gap(8).Children(func() {
		if title != "" {
			GroupTitle(c, pal, title)
		}
		if fn != nil { // a group of a note alone has no panel
			Rows = 0
			ui.Column(c).Radius(12).Background(pal.Chip).Padding(4, 16).Children(fn)
		}
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(pal.TextFaint).LineHeight(1.4).Padding(0, 4)
		}
	})
}

// Reset clears what a frame sets for the pages: the search's marks, and
// the rows counted.
func Reset() {
	Hits, Reveal, Rows = nil, "", 0
}

// Rows counts the rows of the group being built, so that a divider
// goes between them, as in macOS's Settings.
var Rows int

// setting is a row of a group: a label, what it does, and its control.
func Row(c *ui.Context, pal *theme.Palette, label, detail string, control func()) {
	if Rows++; Rows > 1 {
		ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
	}
	row := ui.Row(c.Key("row-"+label)).Padding(11, 0).Gap(16).AlignItems(ui.Center)
	row.Children(func() {
		Highlight(c, row, label, pal, 10, -2)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, label).FontSize(13.5).TextColor(pal.Text)
			if detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
			}
		})
		ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(control)
	})
}

// segmented chooses one of ids into value, as macOS's segmented control
// shows it: a soft track, the choice a lighter fill raised a hair, thin
// separators between the others, and labels of one weight.
func Segmented(c *ui.Context, key string, value *string, ids, labels []string, onChange ...func()) {
	pal := theme.For(c)
	i := max(slices.Index(ids, *value), 0)
	track, thumb := ui.RGBA(0, 0, 0, 0.06), ui.Hex("#ffffff")
	if pal.Dark {
		track, thumb = ui.RGBA(255, 255, 255, 0.07), ui.RGBA(255, 255, 255, 0.2)
	}
	seg := ui.SegmentedBase(c.Key(key), &i, len(labels))
	seg.Track.Height(24).Padding(2).Radius(7).Background(track).AlignItems(ui.Center).Children(func() {
		for n, label := range labels {
			if n > 0 {
				// A separator, where neither segment beside it is chosen.
				sep := ui.Box(c).Width(1).Height(12).Shrink(0)
				if n != i && n-1 != i {
					sep.Background(pal.Text.Alpha(0.14))
				}
			}
			s := seg.Segment(n).FillHeight().Padding(0, 12).Radius(5).Center()
			if n == i {
				s.Background(thumb)
				if pal.Dark {
					s.Shadow(0, 1, 1, 0, ui.RGBA(0, 0, 0, 0.25))
				} else {
					s.Shadow(0, 0.5, 1.5, 0, ui.RGBA(0, 0, 0, 0.16)).Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.04))
				}
			}
			s.Children(func() {
				ui.Text(c, label).FontSize(12.5).SingleLine().TextColor(pal.Text)
			})
		}
	})
	// The choice lands after this runs: take it then.
	seg.Track.OnChange(func() {
		*value = ids[i]
		for _, fn := range onChange {
			if fn != nil {
				fn()
			}
		}
	})
}

// The search's finds on the page shown, by label, which Row and Group
// highlight as they build; nil while not searching. The Settings window
// is the only view that builds them, and sets these before it does.
var (
	Hits   map[string]bool
	Reveal string // the find to scroll to, in this build only
)

// Highlight marks a find of the search behind el, a row or a title, as a
// tint wider than it by dx on each side, and taller by dy.
func Highlight(c *ui.Context, el ui.Element, label string, pal *theme.Palette, dx, dy float32) {
	hit := Hits[label]
	t := el.Animate("hit", b2f(hit), 180*time.Millisecond)
	if label == Reveal {
		el.ScrollIntoView()
	}
	if t > 0 {
		ui.Box(c.Key("hit")).Absolute().Top(-dy).Bottom(-dy).Left(-dx).Right(-dx).Radius(8).PassThrough().
			Background(pal.Accent.Alpha(0.13*t)).InsetShadow(0, 0, 0, 1, pal.Accent.Alpha(0.28*t))
	}
}

// b2f is b as 0, or 1 for true.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
