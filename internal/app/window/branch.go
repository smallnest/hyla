package window

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/gitinfo"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"

	"hyla/locales"

	"hyla/internal/ui/glyph"
)

// gitChip is a pane's branch and its changes since HEAD, after its title:
// a click shows the changes panel. In a narrow pane, whose header the
// title and its controls fill, it shows its counts alone, or nothing.
func (a *App) gitChip(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	g := p.Git
	room := a.paneRef("card-" + p.ID).Bounds(c).W
	if room > 0 && room < 320 {
		return
	}
	compact := room > 0 && room < 460
	summary := gitinfo.Summary(g)
	// It gives up its room before the title does: its branch shrinks
	// first, down to its counts.
	b := ui.ButtonBase(c.Key("git")).Height(24).Padding(0, 8).Gap(5).Radius(7).AlignItems(ui.Center).Shrink(8).
		Label(summary).Tooltip(summary + "\n" + a.tip(locales.GitShowChanges(), "changes.toggle"))
	if b.Hovered() || a.sideTab() == sideChanges {
		b.Background(pal.RowHover)
	}
	b.Children(func() {
		ui.Icon(c, glyph.Branch).Size(12, 12).TextColor(pal.TextFaint).Shrink(0)
		if compact && g.Files > 0 {
			a.lineCounts(c, pal, g.Added, g.Removed, 12)
			return
		}
		ui.Text(c, gitinfo.BranchName(g)).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").MinWidth(0).MaxWidth(180)
		if st := gitinfo.State(g.State); st != "" {
			ui.Text(c, st).FontSize(12).TextColor(pal.Attention).SingleLine().Shrink(0)
		}
		a.lineCounts(c, pal, g.Added, g.Removed, 12)
	})
	if b.Clicked() {
		a.defer_(func() { a.toggleSide(sideChanges) })
	}
}
