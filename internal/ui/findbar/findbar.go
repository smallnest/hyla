// Package findbar holds the find bar that floats over a pane: a search of
// its output, its scrollback included, as Ghostty's find, or of a
// browser's page.
package findbar

import (
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/panes"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// State is the find bar's: a search of a pane's output, its scrollback
// included, as Ghostty's find.
type State struct {
	Pane  string // the pane searched; "" while the bar is closed
	Query string
	// Focus has the field take the keyboard, its text selected, as the
	// bar opens.
	Focus bool
	Input ui.Handle
}

// Open opens the find bar over the pane the user is in, or gives the
// keyboard back to it, with the last query to type over.
func Open(a App) {
	s := a.SelectedSession()
	if s == nil {
		return
	}
	id := a.FocusOf(s)
	if id == "" {
		return
	}
	f := a.FindState()
	if f.Pane != id {
		if t := term(a); t != nil {
			t.EndSearch()
		}
	}
	f.Pane, f.Focus = id, true
}

// Step goes to the next match, toward older output, or the previous one;
// with the bar closed it opens, the last query searched again.
func Step(a App, back bool) {
	f := a.FindState()
	if f.Pane == "" {
		Open(a)
		return
	}
	if e := a.WebsNow()[f.Pane]; e != nil {
		a.FindInPage(e, true, back)
		return
	}
	t := term(a)
	if t == nil {
		return
	}
	if back {
		t.SearchPrevious()
	} else {
		t.SearchNext()
	}
}

// Close closes the find bar, its highlights going, and gives the keyboard
// back to the pane.
func Close(a App) {
	if t := term(a); t != nil {
		t.EndSearch()
	}
	f := a.FindState()
	pane := f.Pane
	if e := a.WebsNow()[pane]; e != nil && e.View != nil {
		e.FindText, e.Found = "", mygo.FindResult{}
		e.FindSeq++
		go e.View.Page().StopFindInPage()
	}
	f.Pane = ""
	if pane != "" {
		a.PaneRef(pane).Focus()
	}
}

// term returns the terminal of the pane searched.
func term(a App) *terminal.Terminal {
	return a.TermOf(a.FindState().Pane)
}

// Bar floats over the top right of the pane searched: the query, how many
// matches it has, and the buttons going through them.
func Bar(a App, c *ui.Context, pal *theme.Palette, p *proto.Pane, t *terminal.Terminal) {
	f := a.FindState()
	// A terminal attached anew, as settings change, searches again.
	if f.Query != "" && t.SearchStatus().Text != f.Query {
		t.Search(f.Query)
	}
	bar := ui.Row(c.Key("find")).Absolute().Top(8).Right(12).Height(36).Padding(0, 4, 0, 12).Gap(4).
		AlignItems(ui.Center).Radius(10).Background(pal.Popover).Label(locales.FindLabel())
	style.Floating(bar, pal).Children(func() {
		ui.Icon(c, glyph.Search).Size(13, 13).TextColor(pal.TextMuted).Shrink(0)
		in := ui.TextInputBase(c.Key("q"), &f.Query).Width(180).FontSize(13).Placeholder(locales.FindPlaceholder()).Bind(&f.Input)
		if f.Focus {
			f.Focus = false
			in.Focus().SetTextSelection(0, utf8.RuneCountInString(f.Query))
		}
		if in.Changed() {
			t.Search(f.Query)
		}
		switch {
		case in.Submitted() && c.Modifiers()&ui.Shift != 0:
			t.SearchPrevious()
		case in.Submitted():
			t.SearchNext()
		}
		st := t.SearchStatus()
		count := ""
		switch {
		case f.Query == "":
		case st.Total == 0 && !st.Searching:
			count = locales.FindNoMatches()
		case st.Total > 0:
			count = locales.FindCount(panes.Itoa(st.Selected), panes.Itoa(st.Total))
		}
		ui.Text(c, count).FontSize(12).FontFeatures("tnum").TextColor(pal.TextFaint).SingleLine().MinWidth(56).TextAlign(ui.End).Margin(0, 4, 0, 0)
		// Up goes back through the output, as Enter does, as in Ghostty.
		paints.IconButton(c, "next", glyph.ChevronUp, a.Tip(locales.ActionFindNext(), "find.next"), pal, t.SearchNext)
		paints.IconButton(c, "previous", glyph.ChevronDown, a.Tip(locales.ActionFindPrevious(), "find.previous"), pal, t.SearchPrevious)
		paints.IconButton(c, "close", glyph.Close, locales.FindClose(), pal, func() { a.RunLater(func() { Close(a) }) })
	})
	// Escape closes it from its field and its buttons alike.
	if bar.Shortcut(0, ui.KeyEscape) {
		a.RunLater(func() { Close(a) })
	}
}
