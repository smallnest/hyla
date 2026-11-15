package main

import (
	"fmt"
	"hyla/internal/platform"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"hyla/internal/ui/theme"

	"hyla/internal/app/paneinfo"
	"hyla/internal/proto"

	"hyla/locales"

	"hyla/internal/ui/glyph"
)

// The Info view is the side panel's first: where the pane in the session
// works, the processes of its terminal and the ports they listen on.

// paneInfoOf returns a pane's Info view, made for the pane as it is now.
func (a *App) paneInfoOf(p *proto.Pane) *paneinfo.State {
	return a.Info.Of(p)
}

// infoTick starts a pane's reads as they fall due.
func (a *App) infoTick(c *ui.Context, p *proto.Pane) {
	st := a.paneInfoOf(p)
	now := time.Now()
	if now.Sub(st.ReadAt) >= paneinfo.Tick {
		paneinfo.Read(st, a.hostOf(p.ID).cl, a.workDir(p.ID), a.update)
	}
	if !st.ProcsAt || now.Sub(st.ListedAt) >= paneinfo.ProcsTick {
		paneinfo.ReadProcs(st, a.hostOf(p.ID).cl, p, a.update)
	}
	c.After(paneinfo.ProcsTick / 2)
}

// --- the views ---

// infoView is the side panel's Info view: of the pane the user is in.
func (a *App) infoView(c *ui.Context, pal *theme.Palette, s *proto.Session) {
	p := s.PaneByID(a.focusOf(s))
	if p == nil {
		p = s.FocusPane()
	}
	if p == nil {
		a.sideMessage(c, pal, locales.SideInfo(), "")
		return
	}
	a.infoTick(c, p)
	ui.Scroll(c.Key("info")).Grow(1).MinHeight(0).TrackScroll(&a.Side.info.scroll).Children(func() {
		a.infoBody(c, pal, s, p)
	})
}

// infoBody is an Info view: the working directory, the processes and the
// ports, each a card of the side panel's own look, under its header's tabs
// and a margin of the panel's on either side.
func (a *App) infoBody(c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	st := a.paneInfoOf(p)
	ui.Column(c).Padding(8, 12, 12, 12).Gap(8).AlignItems(ui.Stretch).Children(func() {
		a.infoDir(c, pal, s, a.hostOf(p.ID), p, st)
		a.infoProcs(c, pal, p, st)
		a.infoPorts(c, pal, p, st)
		a.infoMagpie(c, pal)
		a.infoFeishu(c, pal, p)
	})
}

// infoCard is a card of the Info view: a row of a header, a tile's icon, a
// title and an aside, with a hairline under it. fold is the card's key in
// prefs.InfoFolded, or "" for a card that does not fold; a card folded
// keeps its header, which a click unfolds.
func (a *App) infoCard(c *ui.Context, pal *theme.Palette, fold string, svg *ui.SVG, title, aside string, body func()) {
	collapsed := fold != "" && a.Prefs.InfoFolded[fold]
	ui.Column(c).Radius(12).Background(pal.Chip).Clip().Children(func() {
		a.infoHead(c, pal, fold, collapsed, title, func() {
			infoTile(c, pal, svg)
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			ui.Spacer(c)
			if aside != "" {
				if strings.ContainsRune(aside, '/') {
					infoValue(c, pal, aside)
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
func (a *App) infoHead(c *ui.Context, pal *theme.Palette, fold string, collapsed bool, title string, fn func()) {
	if fold == "" {
		ui.Column(c).Children(func() {
			ui.Row(c).Height(40).Padding(0, 12).Gap(9).AlignItems(ui.Center).Children(fn)
			infoRule(c, pal)
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
		a.defer_(func() { a.toggleInfoCard(fold) })
	}
	if !collapsed {
		infoRule(c, pal)
	}
}

// toggleInfoCard folds a card of the Info view, or unfolds it.
func (a *App) toggleInfoCard(fold string) {
	if a.Prefs.InfoFolded[fold] {
		delete(a.Prefs.InfoFolded, fold)
	} else {
		if a.Prefs.InfoFolded == nil {
			a.Prefs.InfoFolded = map[string]bool{}
		}
		a.Prefs.InfoFolded[fold] = true
	}
	a.savePrefs()
}

// The cards of the Info view that fold, as prefs.InfoFolded remembers them.
const (
	infoFoldDir    = "dir"
	infoFoldProcs  = "procs"
	infoFoldPorts  = "ports"
	infoFoldMagpie = "magpie"
	infoFoldFeishu = "feishu"
)

// infoRows lists a card's rows, each divided from the one above.
func infoRows(c *ui.Context, fn func()) {
	ui.Column(c).Padding(0, 0, 3, 0).AlignItems(ui.Stretch).Children(fn)
}

// infoRule divides a card's rows.
func infoRule(c *ui.Context, pal *theme.Palette) {
	ui.Box(c).Height(1).Margin(0, 1, 0, 1).Background(pal.Text.Alpha(0.07))
}

// infoTile is a card's icon, on a small rounded square.
func infoTile(c *ui.Context, pal *theme.Palette, svg *ui.SVG) {
	t := ui.Box(c).Size(24, 24).Radius(7).Background(pal.Text.Alpha(0.07)).Center().Shrink(0)
	t.Children(func() { ui.Icon(c, svg).Size(13, 13).TextColor(pal.TextMuted) })
}

// infoLabel is a row's label, on the left.
func infoLabel(c *ui.Context, pal *theme.Palette, label string) {
	ui.Text(c, label).FontSize(12.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
}

// infoValue is a card's aside, on the right of its title: what the card
// holds, in short words, cut short if need be.
func infoValue(c *ui.Context, pal *theme.Palette, value string) {
	ui.Text(c, value).FontSize(11.5).TextColor(pal.TextFaint).FontFeatures("tnum").SingleLine().Ellipsis("…").Shrink(1).MinWidth(0).Grow(1).TextAlign(ui.End)
}

// infoDir is the Info view's working directory, with what opens it: its
// own name as the card's title, its parent after, and the ways to it,
// each a button.
func (a *App) infoDir(c *ui.Context, pal *theme.Palette, s *proto.Session, h *host, p *proto.Pane, st *paneinfo.State) {
	dir := st.Dir
	if dir == "" {
		dir = p.WorkDir()
	}
	pid, sid := p.ID, s.ID
	// Copying carries the pane the user is in: here, no press of the card's
	// focuses the pane.
	copyDir := func() {
		if cur := a.session(sid); cur != nil {
			a.setFocus(cur, pid)
		}
		a.CopyText = dir
	}
	_, name := splitDir(dir)
	a.infoCard(c, pal, infoFoldDir, glyph.Folder, name, dir, func() {
		ui.Row(c).Padding(8, 12, 12, 12).Gap(6).Wrap().Children(func() {
			// This machine's folder alone opens here: another machine's
			// files are its own, its Zed's to open.
			if dir != "" {
				a.infoAction(c, pal, "zed", locales.InfoOpenEditor("Zed"), glyph.Zed, func() {
					if cur := a.session(sid); cur != nil {
						a.setFocus(cur, pid)
					}
					openInZed(dir)
				})
			}
			editor := editorName(h)
			if editor == "vi" {
				editor = "Vim"
			}
			a.infoAction(c, pal, "editor", locales.InfoOpenEditor(editor), editorGlyph(editor), func() {
				a.split(s, proto.CreateParams{Split: p.ID, Direction: "right", Command: editorCommand(h, dir)})
			})
			a.infoAction(c, pal, "copy-dir", locales.InfoCopyPath(), glyph.InfoCopy, copyDir)
			a.infoAction(c, pal, "reveal", locales.InfoReveal(), glyph.Folder, func() { platform.RevealPath(dir) })
		})
	})
}

// infoAction is a button of a card: its icon and its label, on a light pill
// that deepens as the pointer is on it.
func (a *App) infoAction(c *ui.Context, pal *theme.Palette, key, label string, svg *ui.SVG, fn func()) {
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
func editorName(h *host) string {
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
func (a *App) infoProcs(c *ui.Context, pal *theme.Palette, p *proto.Pane, st *paneinfo.State) {
	if st.Shell <= 0 {
		// The shell has not started: the view looks again for it.
		c.After(paneinfo.Idle)
	}
	aside := ""
	if len(st.Procs) > 0 {
		aside = itoa(len(st.Procs))
	}
	a.infoCard(c, pal, infoFoldProcs, glyph.CPU, locales.InfoProcess(), aside, func() {
		switch {
		case st.ListErr != "":
			ui.Text(c, st.ListErr).FontSize(12).TextColor(pal.Danger).LineHeight(1.35).Padding(0, 12, 8, 12).Wrap()
		case p.PID <= 0:
			infoEmpty(c, pal, locales.InfoNoShell())
		case !st.ProcsAt:
			ui.Box(c).Height(34).Padding(0, 12).Center().Children(func() { ui.Spinner(c).Size(12, 12) })
		default:
			infoRows(c, func() {
				for i, pr := range st.Procs {
					if i > 0 {
						infoRule(c, pal)
					}
					processRow(c, pal, pr)
				}
			})
		}
	})
}

// infoEmpty is a card's word that it has nothing to list.
func infoEmpty(c *ui.Context, pal *theme.Palette, text string) {
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
func (a *App) infoPorts(c *ui.Context, pal *theme.Palette, p *proto.Pane, st *paneinfo.State) {
	if st.Shell <= 0 {
		// The shell has not started: the ports come with its processes.
		c.After(paneinfo.Idle)
	}
	aside := ""
	if st.Ports > 0 {
		aside = itoa(st.Ports)
	}
	a.infoCard(c, pal, infoFoldPorts, glyph.Globe, locales.InfoPorts(), aside, func() {
		switch {
		case !st.ProcsAt && p.PID <= 0:
			infoEmpty(c, pal, locales.InfoNoShell())
		case !st.ProcsAt:
			ui.Box(c).Height(34).Padding(0, 12).Center().Children(func() { ui.Spinner(c).Size(12, 12) })
		case st.Ports == 0:
			infoEmpty(c, pal, locales.InfoNoPorts())
		default:
			infoRows(c, func() {
				first := true
				for _, pr := range st.Procs {
					for _, port := range pr.Ports {
						if !first {
							infoRule(c, pal)
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
