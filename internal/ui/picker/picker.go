// Package picker lists the folders of a host, to open a project in one,
// as the system's dialog does this machine's: hylad there lists them
// (fs.list).
package picker

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/fuzzy"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// App is what the picker lists and opens, as the app gives it.
type App interface {
	// Update runs fn on the app's own goroutine.
	Update(fn func())
	// Toast shows a message.
	Toast(msg string)
	// OpenProjectAt starts a session in a folder of a host, in its
	// project.
	OpenProjectAt(h *host.Host, dir string)
	// FocusTerminal focuses the terminal the user works in.
	FocusTerminal()
}

// State is the picker's state.
type State struct {
	Open bool
	// Host is the host whose folders it lists.
	Host *host.Host
	// List is the folder listed, nil while it comes, and Err why it did
	// not; Gen tells the listing asked for last.
	List *proto.FSList
	Err  string
	Gen  int
	// Query filters the folders, or, as ~/… or /…, names one to go to.
	Query  string
	Index  int
	Hidden bool
	Scroll ui.ScrollState
	Reveal bool // the chosen folder scrolls into view, once
}

// item is a folder the picker offers.
type item struct {
	name, path string
	repo, up   bool
}

// Open lists a host's folders, from its home, to open a project in one.
func Open(st *State, app App, h *host.Host) {
	if h.Client == nil {
		app.Toast(locales.HostNotConnected(h.Name))
		return
	}
	*st = State{Open: true, Host: h}
	GoTo(st, app, "~")
}

// GoTo lists a folder of the picker's host.
func GoTo(st *State, app App, dir string) {
	st.Gen++
	gen, hidden := st.Gen, st.Hidden
	st.Err, st.Query, st.Index, st.Reveal = "", "", 0, true
	h := st.Host
	cl := h.Client
	if cl == nil {
		st.Err = locales.HostNotConnected(h.Name)
		return
	}
	go func() {
		var l proto.FSList
		err := cl.Call("fs.list", proto.FSListParams{Dir: dir, Hidden: hidden}, &l)
		app.Update(func() {
			if st.Gen != gen {
				return
			}
			if err != nil {
				st.Err = err.Error()
				return
			}
			// It shows from its first folder, whatever the last one's scroll.
			st.List, st.Reveal = &l, true
		})
	}()
}

// Items are the folders the picker shows: the parent, then those of the
// folder listed that the query matches.
func Items(st *State) []item {
	if st.List == nil {
		return nil
	}
	var items []item
	q := strings.TrimSpace(st.Query)
	if q == "" && st.List.Parent != "" {
		items = append(items, item{name: "..", path: st.List.Parent, up: true})
	}
	type scored struct {
		item
		score int
	}
	var found []scored
	for _, e := range st.List.Entries {
		if !e.Dir {
			continue
		}
		sc := fuzzy.Score(q, e.Name)
		if sc < 0 {
			continue
		}
		found = append(found, scored{item{name: e.Name, path: st.Host.JoinPath(st.List.Dir, e.Name), repo: e.Repo}, sc})
	}
	if q != "" {
		slices.SortStableFunc(found, func(x, y scored) int { return y.score - x.score })
	}
	for _, f := range found {
		items = append(items, f.item)
	}
	return items
}

// isPathQuery tells a query that names a folder rather than filters: on
// Windows, ~\…, a share's \\server\… and a drive's C:\… too.
func isPathQuery(q string, windows bool) bool {
	if strings.HasPrefix(q, "/") || q == "~" || strings.HasPrefix(q, "~/") {
		return true
	}
	drive := len(q) >= 3 && q[1] == ':' && (q[2] == '\\' || q[2] == '/') && ('A' <= q[0] && q[0] <= 'Z' || 'a' <= q[0] && q[0] <= 'z')
	return windows && (strings.HasPrefix(q, `~\`) || strings.HasPrefix(q, `\\`) || drive)
}

// View draws the picker, and opens the folder the user chose.
func View(c *ui.Context, st *State, app App, pal *theme.Palette) {
	p := st
	if !p.Open {
		return
	}
	h := p.Host
	items := Items(p)
	p.Index = min(max(p.Index, 0), max(len(items)-1, 0))
	var goTo, openDir string
	ui.DialogBase(c.Key("picker"), &p.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay).Justify(ui.Start).Padding(84, 0, 0, 0)
		style.Floating(panel.Width(600).MaxHeight(500).Radius(14).Background(pal.Popover), pal).Clip().
			Label(locales.PickerTitle(h.Label()))
		ui.Row(c).Height(42).Shrink(0).Padding(0, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, glyph.Host).Size(14, 14).TextColor(pal.TextMuted)
			ui.Text(c, locales.PickerTitle(h.Label())).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Shrink(0)
			if p.List != nil {
				ui.Text(c, h.ShortDir(p.List.Dir)).FontSize(12.5).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
			}
			ui.Spacer(c)
			label := locales.PickerShowHidden()
			if p.Hidden {
				label = locales.PickerHideHidden()
			}
			b := ui.ButtonBase(c.Key("hidden")).Height(22).Padding(0, 8).Radius(6).Label(label)
			if b.Hovered() {
				b.Background(pal.RowHover)
			}
			b.Children(func() { ui.Text(c, label).FontSize(12).TextColor(pal.TextMuted).SingleLine() })
			if b.Clicked() && p.List != nil {
				p.Hidden = !p.Hidden
				goTo = p.List.Dir
			}
		})
		ui.Row(c).Height(44).Shrink(0).Padding(0, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, glyph.Folder).Size(16, 16).TextColor(pal.TextMuted)
			in := ui.TextInputBase(c.Key("q"), &p.Query).Grow(1).FontSize(15).Placeholder(locales.PickerPlaceholder()).AutoFocus()
			if in.Changed() {
				p.Index, p.Reveal = 0, true
			}
			if c.Shortcut(0, ui.KeyDown) || c.Shortcut(ui.Ctrl, ui.KeyN) {
				p.Index, p.Reveal = min(p.Index+1, len(items)-1), true
			}
			if c.Shortcut(0, ui.KeyUp) || c.Shortcut(ui.Ctrl, ui.KeyP) {
				p.Index, p.Reveal = max(p.Index-1, 0), true
			}
			q := strings.TrimSpace(p.Query)
			switch {
			case c.Shortcut(ui.Cmd, ui.KeyEnter):
				// Opens the folder chosen, without going in.
				openDir = chosen(p, items)
			case in.Submitted() || c.Shortcut(0, ui.KeyEnter) || c.Shortcut(0, ui.KeyTab):
				switch {
				case isPathQuery(q, h.Windows()):
					goTo = q
				case len(items) > 0:
					goTo = items[p.Index].path
				}
			}
		})
		list := ui.Scroll(c.Key("folders")).Grow(1).MinHeight(0).Padding(6).TrackScroll(&p.Scroll)
		reveal := p.Reveal
		p.Reveal = false
		list.Children(func() {
			switch {
			case p.Err != "":
				ui.Text(c, p.Err).FontSize(13).TextColor(pal.Danger).Padding(12, 12).Selectable()
			case p.List == nil:
				ui.Row(c).Padding(12).Children(func() { ui.Spinner(c) })
			case len(items) == 0:
				ui.Text(c, locales.PickerNoFolders()).FontSize(13).TextColor(pal.TextFaint).Padding(12, 12)
			}
			for i, it := range items {
				row := ui.Row(c.Key(it.path)).Height(34).Padding(0, 10).Gap(10).AlignItems(ui.Center).Radius(8)
				if i == p.Index {
					row.Background(pal.Accent.Alpha(0.12))
					if reveal {
						row.ScrollIntoView()
					}
				} else if row.Hovered() {
					row.Background(pal.RowHover)
				}
				if row.Clicked() {
					p.Index = i
				}
				if row.DoubleClicked() {
					goTo = it.path
				}
				row.Children(func() {
					ic := glyph.Folder
					if it.repo {
						ic = glyph.Branch
					}
					ui.Icon(c, ic).Size(14, 14).TextColor(pal.TextFaint)
					ui.Text(c, it.name).FontSize(13.5).TextColor(pal.Text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
				})
			}
		})
		ui.Row(c).Height(52).Shrink(0).Padding(0, 16).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.PickerHint(actions.Glyphs("CmdOrCtrl+Enter"))).FontSize(12).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				p.Open = false
			}
			if style.Button(c, "open", locales.CommonOpen(), "", true, pal) {
				openDir = chosen(p, items)
			}
		})
	})
	switch {
	case openDir != "":
		p.Open = false
		app.OpenProjectAt(h, openDir)
	case goTo != "":
		GoTo(p, app, goTo)
	}
	if !p.Open {
		*st = State{}
		if openDir == "" {
			app.FocusTerminal()
		}
	}
}

// chosen is the folder Open opens: the one chosen in the list, else, as
// with the parent chosen, the folder listed.
func chosen(p *State, items []item) string {
	if len(items) > 0 && !items[p.Index].up {
		return items[p.Index].path
	}
	if p.List != nil {
		return p.List.Dir
	}
	return ""
}

// Hosts is the picker's switch of machines: the one it lists, and each
// other, which a click lists instead.
func Hosts(c *ui.Context, st *State, app App, hosts []*host.Host, pal *theme.Palette) {
	shown := st.Host
	ui.Row(c.Key("hosts")).Gap(2).AlignItems(ui.Center).MinWidth(0).Shrink(1).Children(func() {
		for _, h := range hosts {
			label := h.Label()
			b := ui.ButtonBase(c.Key("host-"+h.Name)).Height(24).Padding(0, 8).Radius(6).AlignItems(ui.Center).
				Label(locales.PickerTitle(label))
			fg := pal.TextMuted
			switch {
			case h == shown:
				b.Background(pal.RowSelected).InsetShadow(0, 0, 0, 1, pal.RowSelBorder)
				fg = pal.Text
			case b.Hovered():
				b.Background(pal.RowHover)
			}
			b.Children(func() {
				ui.Text(c, label).FontSize(12.5).FontWeight(600).TextColor(fg).SingleLine()
			})
			if b.Clicked() && h != shown {
				Open(st, app, h)
			}
		}
	})
}
