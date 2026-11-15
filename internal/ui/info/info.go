// Package info is the side panel's Info view: where the pane in the
// session works, the processes of its terminal and the ports they listen
// on, each a card that folds.
package info

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/app/paneinfo"
	"hyla/internal/platform"
	"hyla/internal/proto"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// App is what the Info view reads and runs, as the app gives it.
type App interface {
	// Update runs fn on the app's own goroutine.
	Update(fn func())
	// InfoViews is what the panes' Info views read.
	InfoViews() *paneinfo.Panes
	// HostOf is the host a session or a pane is of.
	HostOf(id string) *host.Host
	// WorkDir is a pane's working directory.
	WorkDir(id string) string
	// FocusOf is the pane of a session the user is in.
	FocusOf(s *proto.Session) string
	// Scroll is the Info view's scroll.
	Scroll() *ui.ScrollState
	// SideMessage shows a word in the side panel.
	SideMessage(c *ui.Context, pal *theme.Palette, title, detail string)
	// Folded are the cards the user folded, by their key, and sets one.
	Folded() map[string]bool
	// SavePrefs saves what the window remembers.
	SavePrefs()
	// AfterFrame runs fn after the frame.
	AfterFrame(fn func())
	// Session is a session the app knows, nil while none.
	Session(id string) *proto.Session
	// SetFocus focuses a pane of a session.
	SetFocus(s *proto.Session, pane string)
	// Copied is the text the window copies next, and sets.
	Copied() *string
	// Split splits a pane, with the session's params.
	Split(s *proto.Session, p proto.CreateParams)
	// InfoMagpie draws Magpie's card.
	InfoMagpie(c *ui.Context, pal *theme.Palette)
	// InfoFeishu draws the Feishu bot's card.
	InfoFeishu(c *ui.Context, pal *theme.Palette, p *proto.Pane)
}

// paneInfoOf returns a pane's Info view, made for the pane as it is now.
func paneInfoOf(x App, p *proto.Pane) *paneinfo.State {
	return x.InfoViews().Of(p)
}

// infoTick starts a pane's reads as they fall due.
func infoTick(c *ui.Context, x App, p *proto.Pane) {
	st := paneInfoOf(x, p)
	now := time.Now()
	if now.Sub(st.ReadAt) >= paneinfo.Tick {
		paneinfo.Read(st, x.HostOf(p.ID).Client, x.WorkDir(p.ID), x.Update)
	}
	if !st.ProcsAt || now.Sub(st.ListedAt) >= paneinfo.ProcsTick {
		paneinfo.ReadProcs(st, x.HostOf(p.ID).Client, p, x.Update)
	}
	c.After(paneinfo.ProcsTick / 2)
}

// --- the views ---

// infoView is the side panel's Info view: of the pane the user is in.
func View(c *ui.Context, x App, pal *theme.Palette, s *proto.Session) {
	p := s.PaneByID(x.FocusOf(s))
	if p == nil {
		p = s.FocusPane()
	}
	if p == nil {
		x.SideMessage(c, pal, locales.SideInfo(), "")
		return
	}
	infoTick(c, x, p)
	ui.Scroll(c.Key("info")).Grow(1).MinHeight(0).TrackScroll(x.Scroll()).Children(func() {
		Body(c, x, pal, s, p)
	})
}

// infoBody is an Info view: the working directory, the processes and the
// ports, each a card of the side panel's own look, under its header's tabs
// and a margin of the panel's on either side.
func Body(c *ui.Context, x App, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	st := paneInfoOf(x, p)
	ui.Column(c).Padding(8, 12, 12, 12).Gap(8).AlignItems(ui.Stretch).Children(func() {
		infoDir(c, x, pal, s, x.HostOf(p.ID), p, st)
		infoProcs(c, x, pal, p, st)
		infoPorts(c, x, pal, p, st)
		x.InfoMagpie(c, pal)
		x.InfoFeishu(c, pal, p)
	})
}

// Card is a card of the Info view: a row of a header, a tile's icon, a
// title and an aside, with a hairline under it. fold is the card's key in
// prefs.InfoFolded, or "" for a card that does not fold; a card folded
// keeps its header, which a click unfolds.
func Card(c *ui.Context, x App, pal *theme.Palette, fold string, svg *ui.SVG, title, aside string, body func()) {
	collapsed := fold != "" && x.Folded()[fold]
	ui.Column(c).Radius(12).Background(pal.Chip).Clip().Children(func() {
		infoHead(c, x, pal, fold, collapsed, title, func() {
			Tile(c, pal, svg)
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			ui.Spacer(c)
			if aside != "" {
				if strings.ContainsRune(aside, '/') {
					Value(c, pal, aside)
				} else {
					ui.Text(c, aside).FontSize(11.5).TextColor(pal.TextFaint).FontFeatures("tnum").SingleLine().Shrink(0).TextAlign(ui.End)
				}
			}
		})
		if !collapsed {
			body()
		}
	})
}

// infoHead is a card's header: its row, then its hairline, which a folded
// card leaves out, its row closing the card. With fold, the header takes
// the whole row as a button, and wears a chevron that points down while
// the card is open and right while it is folded.
func infoHead(c *ui.Context, x App, pal *theme.Palette, fold string, collapsed bool, title string, fn func()) {
	if fold == "" {
		ui.Column(c).Children(func() {
			ui.Row(c).Height(40).Padding(0, 12).Gap(9).AlignItems(ui.Center).Children(fn)
			Rule(c, pal)
		})
		return
	}
	head := ui.ButtonBase(c.Key("info-head-"+fold)).Height(40).Padding(0, 12, 0, 12).Gap(9).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	label := locales.InfoCollapse(title)
	if collapsed {
		label = locales.InfoExpand(title)
	}
	turn := head.AnimateTo("fold", 90*(1-b2f(collapsed)), ui.Snappy)
	head.Label(label).Tooltip(label).Children(func() {
		fn()
		ui.Icon(c, glyph.Chevron).Size(11, 11).TextColor(pal.TextFaint).Shrink(0).Rotate(turn)
	})
	// The click lands in prefs, which reshape the cards after the frame.
	if head.Clicked() {
		x.AfterFrame(func() { toggleInfoCard(x, fold) })
	}
	if !collapsed {
		Rule(c, pal)
	}
}

// toggleInfoCard folds a card of the Info view, or unfolds it.
func toggleInfoCard(x App, fold string) {
	folded := x.Folded()
	if folded[fold] {
		delete(folded, fold)
	} else {
		if folded == nil {
			folded = map[string]bool{}
		}
		folded[fold] = true
	}
	x.SavePrefs()
}

// The cards of the Info view that fold, as prefs.InfoFolded remembers them.
const (
	FoldDir        = "dir"
	FoldProcs      = "procs"
	FoldPorts      = "ports"
	FoldMagpie     = "magpie"
	FoldFeishu     = "feishu"
	infoFoldFeishu = "feishu"
)

// infoRows lists a card's rows, each divided from the one above.
func Rows(c *ui.Context, fn func()) {
	ui.Column(c).Padding(0, 0, 3, 0).AlignItems(ui.Stretch).Children(fn)
}

// infoRule divides a card's rows.
func Rule(c *ui.Context, pal *theme.Palette) {
	ui.Box(c).Height(1).Margin(0, 1, 0, 1).Background(pal.Text.Alpha(0.07))
}

// infoTile is a card's icon, on a small rounded square.
func Tile(c *ui.Context, pal *theme.Palette, svg *ui.SVG) {
	t := ui.Box(c).Size(24, 24).Radius(7).Background(pal.Text.Alpha(0.07)).Center().Shrink(0)
	t.Children(func() { ui.Icon(c, svg).Size(13, 13).TextColor(pal.TextMuted) })
}

// infoLabel is a row's label, on the left.
func Label(c *ui.Context, pal *theme.Palette, label string) {
	ui.Text(c, label).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
}

// infoValue is a card's aside, on the right of its title: what the card
// holds, in short words, cut short if need be.
func Value(c *ui.Context, pal *theme.Palette, value string) {
	ui.Text(c, value).FontSize(11.5).TextColor(pal.TextFaint).FontFeatures("tnum").SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).Grow(1).TextAlign(ui.End)
}

// infoDir is the Info view's working directory, with what opens it: its
// own name as the card's title, its parent after, and the ways to it,
// each a button.
func infoDir(c *ui.Context, x App, pal *theme.Palette, s *proto.Session, h *host.Host, p *proto.Pane, st *paneinfo.State) {
	dir := st.Dir
	if dir == "" {
		dir = p.WorkDir()
	}
	pid, sid := p.ID, s.ID
	// Copying carries the pane the user is in: here, no press of the card's
	// focuses the pane.
	copyDir := func() {
		if cur := x.Session(sid); cur != nil {
			x.SetFocus(cur, pid)
		}
		*x.Copied() = dir
	}
	_, name := splitDir(dir)
	Card(c, x, pal, FoldDir, glyph.Folder, name, dir, func() {
		ui.Row(c).Padding(8, 12, 12, 12).Gap(6).Wrap().Children(func() {
			// This machine's folder alone opens here: another machine's
			// files are its own, its Zed's to open.
			if dir != "" {
				infoAction(c, x, pal, "zed", locales.InfoOpenEditor("Zed"), glyph.Zed, func() {
					if cur := x.Session(sid); cur != nil {
						x.SetFocus(cur, pid)
					}
					openInZed(dir)
				})
			}
			editor := editorName(h)
			if editor == "vi" {
				editor = "Vim"
			}
			infoAction(c, x, pal, "editor", locales.InfoOpenEditor(editor), editorGlyph(editor), func() {
				x.Split(s, proto.CreateParams{Split: p.ID, Direction: "right", Command: host.EditorCommand(h, dir)})
			})
			infoAction(c, x, pal, "copy-dir", locales.InfoCopyPath(), glyph.InfoCopy, copyDir)
			infoAction(c, x, pal, "reveal", locales.InfoReveal(), glyph.Folder, func() { platform.RevealPath(dir) })
		})
	})
}

// infoAction is a button of a card: its icon and its label, on a light pill
// that deepens as the pointer is on it.
func infoAction(c *ui.Context, x App, pal *theme.Palette, key, label string, svg *ui.SVG, fn func()) {
	b := ui.ButtonBase(c.Key(key)).Height(26).Padding(0, 9).Gap(5).Radius(7).AlignItems(ui.Center).Shrink(0).Cursor(ui.CursorPointer).Label(label)
	bg := pal.Text.Alpha(0.05)
	if b.Pressed() {
		bg = pal.Text.Alpha(0.13)
	} else if b.Hovered() {
		bg = pal.Text.Alpha(0.09)
	}
	b.Background(bg).TextColor(pal.Text)
	b.Children(func() {
		ui.Icon(c, svg).Size(11, 11).Opacity(0.65)
		ui.Text(c, label).FontSize(12).FontWeight(500).SingleLine()
	})
	b.Tooltip(label)
	if b.Clicked() {
		fn()
	}
}

// splitDir tells a directory's parent and its own name, as /Users/hyla
// and hyla; of the root, the whole path is the name.
func splitDir(dir string) (parent, name string) {
	dir = strings.TrimSuffix(dir, "/")
	if i := strings.LastIndexByte(dir, '/'); i > 0 {
		return dir[:i], dir[i+1:]
	}
	if dir == "" {
		return "", ""
	}
	return dir, ""
}

// editorGlyph is the logo of the editor the editor button opens, as vim's
// for vi and nvim.
func editorGlyph(name string) *ui.SVG {
	if g := glyph.ProgramIcons[name]; g != nil {
		return g
	}
	if name == "vi" {
		return glyph.ProgramIcons["vim"]
	}
	return glyph.Link
}

// openInZed opens a folder on this machine in the Zed outside Hyla: Zed's
// CLI, in a PATH, or else macOS's launch of the app by its bundle id, for
// a Zed dragged into Applications from a disk image, which installs none.
// The app starting may take a moment, so its wait runs off the frame.
func openInZed(dir string) {
	go func() {
		cmd := exec.Command("zed", dir)
		// The editor's output goes nowhere, and its input from nowhere,
		// for a program that would wait on them of a window-less process.
		cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
		if err := cmd.Run(); err == nil {
			return
		}
		if runtime.GOOS == "darwin" {
			exec.Command("/usr/bin/open", "-b", "dev.zed.Zed", dir).Run()
		}
	}()
}

// editorName names the editor the editor button opens the directory in:
// the shell's, $VISUAL or $EDITOR, by its command's name.
func editorName(h *host.Host) string {
	if runtime.GOOS != "darwin" {
		return "vi"
	}
	for _, v := range []string{os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if f := strings.Fields(v); len(f) > 0 && f[0] != "" {
			return filepath.Base(f[0])
		}
	}
	return "vi"
}

// infoProcs is the Info view's processes: those of the pane's terminal,
// the parents first, each with how long it ran, its CPU and the ports it
// listens on.
func infoProcs(c *ui.Context, x App, pal *theme.Palette, p *proto.Pane, st *paneinfo.State) {
	if st.Shell <= 0 {
		// The shell has not started: the view looks again for it.
		c.After(paneinfo.Idle)
	}
	aside := ""
	if len(st.Procs) > 0 {
		aside = itoa(len(st.Procs))
	}
	Card(c, x, pal, FoldProcs, glyph.CPU, locales.InfoProcess(), aside, func() {
		switch {
		case st.ListErr != "":
			ui.Text(c, st.ListErr).FontSize(12).TextColor(pal.Danger).LineHeight(1.35).Padding(0, 12, 8, 12).Wrap()
		case p.PID <= 0:
			Empty(c, pal, locales.InfoNoShell())
		case !st.ProcsAt:
			ui.Box(c).Height(34).Padding(0, 12).Center().Children(func() { ui.Spinner(c).Size(12, 12) })
		default:
			Rows(c, func() {
				for i, pr := range st.Procs {
					if i > 0 {
						Rule(c, pal)
					}
					processRow(c, pal, pr)
				}
			})
		}
	})
}

// infoEmpty is a card's word that it has nothing to list.
func Empty(c *ui.Context, pal *theme.Palette, text string) {
	ui.Text(c, text).FontSize(12).TextColor(pal.TextFaint).Padding(0, 12, 10, 12)
}

// processRow is a process of the list: its name over its PID and how long
// it ran, and on the right its ports, its CPU and its logo.
func processRow(c *ui.Context, pal *theme.Palette, pr proto.Process) {
	row := ui.Row(c.Key("proc-"+itoa(pr.PID))).MinHeight(42).Padding(0, 12).Gap(10).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(pal.RowHover)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, pr.Name).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…")
			sub := "PID " + itoa(pr.PID)
			if age := duration(pr.Age); age != "" {
				sub += " · " + age
			}
			ui.Text(c, sub).FontSize(11).TextColor(pal.TextFaint).FontFeatures("tnum").SingleLine().Ellipsis("…")
		})
		if pr.CPU >= 0.5 {
			ui.Text(c, fmt.Sprintf("%.0f%%", pr.CPU)).FontSize(11.5).TextColor(pal.TextMuted).FontFeatures("tnum").Shrink(0)
		}
		if g := glyph.ProgramIcons[pr.Name]; g != nil {
			ui.Icon(c, g).Size(12, 12).TextColor(pal.TextFaint).Opacity(0.8)
		}
	})
}

// infoPorts is the Info view's ports: those the pane's processes listen on,
// each opening in the browser.
func infoPorts(c *ui.Context, x App, pal *theme.Palette, p *proto.Pane, st *paneinfo.State) {
	if st.Shell <= 0 {
		// The shell has not started: the ports come with its processes.
		c.After(paneinfo.Idle)
	}
	aside := ""
	if st.Ports > 0 {
		aside = itoa(st.Ports)
	}
	Card(c, x, pal, FoldPorts, glyph.Globe, locales.InfoPorts(), aside, func() {
		switch {
		case !st.ProcsAt && p.PID <= 0:
			Empty(c, pal, locales.InfoNoShell())
		case !st.ProcsAt:
			ui.Box(c).Height(34).Padding(0, 12).Center().Children(func() { ui.Spinner(c).Size(12, 12) })
		case st.Ports == 0:
			Empty(c, pal, locales.InfoNoPorts())
		default:
			Rows(c, func() {
				first := true
				for _, pr := range st.Procs {
					for _, port := range pr.Ports {
						if !first {
							Rule(c, pal)
						}
						first = false
						portRow(c, pal, pr.Name, port)
					}
				}
			})
		}
	})
}

// portRow is a port a process listens on, which opens in the browser.
func portRow(c *ui.Context, pal *theme.Palette, name string, port int) {
	row := ui.Row(c.Key("port-"+itoa(port))).MinHeight(36).Padding(0, 12).Gap(10).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(pal.RowHover)
	}
	if row.Clicked() {
		platform.OpenPath("http://localhost:" + itoa(port))
	}
	row.Children(func() {
		ui.Text(c, "localhost:"+itoa(port)).FontSize(12.5).FontWeight(600).TextColor(pal.Accent).FontFeatures("tnum").SingleLine().Shrink(0)
		ui.Text(c, name).FontSize(11.5).TextColor(pal.TextFaint).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		ui.Icon(c, glyph.Link).Size(11, 11).TextColor(pal.TextFaint).Opacity(0.8)
	})
}

// duration returns how long a process ran as ps tells it, in words: 62h
// 17m, 2m 14s or 51s, "" for a time not read.
func duration(age string) string {
	if age == "" {
		return ""
	}
	days := 0
	if d, rest, ok := strings.Cut(age, "-"); ok {
		days, age = atoi(d), rest
	}
	// ps tells a time's fields with colons, as 62:17:03 for hours,
	// minutes and seconds, and a day's in hours and minutes, as 2-17:03;
	// and else with their units, as 62h17m03s.
	var f []int
	if strings.Contains(age, ":") {
		for _, v := range strings.Split(age, ":") {
			f = append(f, atoi(v))
		}
	} else {
		// ps leaves the fields it has none of out, as 5m for five minutes.
		units, num := map[string]int{}, ""
		for _, r := range age {
			switch r {
			case 'h', 'm', 's':
				units[string(r)] = atoi(num)
				num = ""
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				num += string(r)
			}
		}
		if num != "" {
			units["s"] = atoi(num)
		}
		f = []int{units["h"], units["m"], units["s"]}
	}
	var hours, minutes, seconds int
	switch len(f) {
	case 3:
		hours, minutes, seconds = f[0], f[1], f[2]
	case 2:
		minutes, seconds = f[0], f[1]
	case 1:
		seconds = f[0]
	default:
		return ""
	}
	if days > 0 || hours > 0 {
		return itoa(days*24+hours) + "h " + itoa(minutes) + "m"
	}
	if minutes > 0 {
		return itoa(minutes) + "m " + itoa(seconds) + "s"
	}
	return itoa(seconds) + "s"
}

// atoi reads a string of digits as a number.
func atoi(s string) int {
	num := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		num = num*10 + int(r-'0')
	}
	return num
}

// b2f is b as 0, or 1 for true.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// itoa writes n as a decimal.
func itoa(n int) string {
	return strconv.Itoa(n)
}
