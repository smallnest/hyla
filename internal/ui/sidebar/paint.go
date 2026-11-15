package sidebar

import (
	"strings"

	ui "github.com/egoist/mygo/ui"

	"hyla/internal/proto"
	"hyla/internal/ui/theme"
)

// ProjectInitial is the first letter of a project's name, for its square.
func ProjectInitial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return ""
}

// DragLine draws the line that tells where a dragged row lands: along its
// top edge, or its bottom one to go after it.
func DragLine(row ui.Element, pal *theme.Palette, after bool) {
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		ly := r.Y
		if after {
			ly = r.Y + r.H - 2
		}
		p.Fill(ui.Rect{X: r.X, Y: ly, W: r.W, H: 2}, pal.Accent, 1)
	})
}

// projectBadges are the squares before project names: muted colors, far
// enough apart to tell, and one per project in the set.
var projectBadges = []ui.Color{
	ui.Hex("#3f6f92"), ui.Hex("#4f8a6a"), ui.Hex("#7b5ea7"), ui.Hex("#a06a2c"),
	ui.Hex("#a34f4f"), ui.Hex("#3f7f8f"), ui.Hex("#6b7d2f"), ui.Hex("#8a4f6a"),
}

// BadgeColor is a project's square: its place among its projects, by id,
// so that renaming a project, or moving it in the sidebar, keeps its
// color, and projects up to the palette's length have their own. The
// "other" project is neutral.
func BadgeColor(projects []proto.Project, p proto.Project) ui.Color {
	if p.ID == "" {
		return ui.Hex("#8b8b95")
	}
	i := 0
	for _, q := range projects {
		if q.ID != "" && q.ID < p.ID {
			i++
		}
	}
	return projectBadges[i%len(projectBadges)]
}
