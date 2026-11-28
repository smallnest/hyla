package arenadialog

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

var agentChoices = []string{"claude", "codex", "agy", "opencode", "pi"}

// Ask holds the Multi-Agent Arena dialog state.
type Ask struct {
	Open      bool
	Tab       string // "new" or "list"
	TabChosen bool
	Host      *host.Host
	Prompt    string
	Agents    map[string]bool
	TestCmd   string
	Dir       string
	Loading   bool
	Loaded    bool
	Busy      bool
	Err       string
	Arenas    []proto.Arena
}

// Open initializes and opens the Multi-Agent Arena dialog, refreshing the list
// of existing arenas from the selected host.
func Open(a App) {
	h := a.SelectedHost()
	if h == nil || h.Client == nil {
		label := "local"
		if h != nil {
			label = h.Label()
		}
		a.Toast(locales.HostNotConnected(label))
		return
	}
	dir := ""
	if s := a.SelectedSession(); s != nil {
		if p := s.FocusPane(); p != nil {
			dir = p.WorkDir()
		}
	}
	d := &Ask{
		Open: true,
		Tab:  "list",
		Host: h,
		Agents: map[string]bool{
			"claude": true,
			"codex":  true,
			"agy":    true,
		},
		Dir: dir,
	}
	a.SetArenaAsk(d)
	refresh(a, d)
}

func refresh(a App, d *Ask) {
	if d == nil || d.Host == nil || d.Host.Client == nil || d.Loading {
		return
	}
	cl := d.Host.Client
	d.Loading = true
	go func() {
		var list []proto.Arena
		err := cl.Call("arena.list", nil, &list)
		a.Main(func() {
			if a.ArenaAskNow() != d {
				return
			}
			d.Loading = false
			d.Loaded = true
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Err = ""
			d.Arenas = list
			if len(list) == 0 && d.Tab == "list" && !d.TabChosen {
				d.Tab = "new"
			}
		})
	}()
}

// View draws the Multi-Agent Arena dialog while d is non-nil.
func View(a App, c *ui.Context, pal *theme.Palette, d *Ask) {
	if d == nil {
		return
	}
	ui.DialogBase(c.Key("arena"), &d.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		style.Floating(panel.Width(640).MaxHeight(580).Padding(22).Gap(10).Radius(14).Background(pal.Popover), pal).
			Label(locales.ArenaTitle())

		ui.Row(c.Key("arena-hdr")).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Column(c.Key("arena-titles")).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, locales.ArenaTitle()).FontSize(15).FontWeight(650).TextColor(pal.Text)
				ui.Text(c, locales.ArenaSubtitle()).FontSize(12.5).TextColor(pal.TextMuted).LineHeight(1.35)
			})
			if style.QuietButton(c, "arena-refresh", locales.ArenaRefresh(), "", pal) {
				refresh(a, d)
			}
		})

		listLabel := locales.ArenaTabList()
		if len(d.Arenas) > 0 {
			listLabel = fmt.Sprintf("%s (%d)", locales.ArenaTabList(), len(d.Arenas))
		}
		ui.Row(c.Key("arena-tabs")).Gap(6).Margin(2, 0, 2, 0).Children(func() {
			if style.SizedButton(c, "tab-new", locales.ArenaTabNew(), "", d.Tab == "new", 26, 7, pal) {
				d.Tab = "new"
				d.TabChosen = true
			}
			if style.SizedButton(c, "tab-list", listLabel, "", d.Tab == "list", 26, 7, pal) {
				d.Tab = "list"
				d.TabChosen = true
			}
		})

		if d.Tab == "new" {
			drawNewTab(a, c, pal, d)
		} else {
			drawListTab(a, c, pal, d)
		}
	})

	if !d.Open {
		a.SetArenaAsk(nil)
		a.FocusTerminal()
	}
}

func drawNewTab(a App, c *ui.Context, pal *theme.Palette, d *Ask) {
	ui.Column(c.Key("arena-new")).Gap(10).Children(func() {
		submit := false
		ui.Row(c.Key("prompt-row")).Children(func() {
			in := controls.TextField(c, "arena-prompt", &d.Prompt, locales.ArenaPromptPlaceholder(), 0, pal)
			in.AutoFocus()
			if in.Submitted() {
				submit = true
			}
		})

		ui.Column(c.Key("agents-col")).Gap(6).Children(func() {
			ui.Text(c, locales.ArenaAgentsLabel()).FontSize(12).FontWeight(550).TextColor(pal.TextMuted)
			ui.Row(c.Key("agents-row")).Gap(6).Wrap().Children(func() {
				for _, ag := range agentChoices {
					selected := d.Agents[ag]
					pill := ui.ButtonBase(c.Key("agent-"+ag)).Height(26).Padding(0, 11).Radius(13).
						AlignItems(ui.Center).Label(ag)
					if selected {
						pill.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
					} else {
						pill.Background(pal.Chip).TextColor(pal.Text)
						if pill.Hovered() {
							pill.Background(pal.RowHover)
						}
					}
					pill.Children(func() {
						ui.Text(c, ag).FontSize(12.5).FontWeight(500).SingleLine()
					})
					if pill.Clicked() {
						d.Agents[ag] = !selected
					}
				}
			})
		})

		ui.Row(c.Key("opts-row")).Gap(8).Children(func() {
			controls.TextField(c, "arena-test", &d.TestCmd, locales.ArenaTestPlaceholder(), 0, pal)
			controls.TextField(c, "arena-dir", &d.Dir, locales.ArenaDirPlaceholder(), 0, pal)
		})

		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}

		canStart := !d.Busy && strings.TrimSpace(d.Prompt) != "" && len(selectedAgents(d)) > 0
		ui.Row(c.Key("new-actions")).Gap(8).Justify(ui.End).Margin(6, 0, 0, 0).Children(func() {
			if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				d.Open = false
			}
			if style.ButtonDisabled(c, "arena-start", locales.ArenaStart(), "↩", true, !canStart, pal) || (submit && canStart) {
				startArena(a, d)
			}
		})
	})
}

func selectedAgents(d *Ask) []string {
	var out []string
	for _, ag := range agentChoices {
		if d.Agents[ag] {
			out = append(out, ag)
		}
	}
	return out
}

func startArena(a App, d *Ask) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	prompt := strings.TrimSpace(d.Prompt)
	agents := selectedAgents(d)
	if prompt == "" || len(agents) == 0 {
		return
	}
	testCmd := strings.TrimSpace(d.TestCmd)
	dir := strings.TrimSpace(d.Dir)
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		var ar proto.Arena
		err := cl.Call("arena.start", proto.ArenaStartParams{
			Prompt:  prompt,
			Agents:  agents,
			TestCmd: testCmd,
			Dir:     dir,
		}, &ar)
		a.Main(func() {
			if a.ArenaAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Prompt = ""
			d.Tab = "list"
			d.TabChosen = true
			d.Arenas = append([]proto.Arena{ar}, d.Arenas...)
			a.Toast(locales.ArenaStartedToast(ar.ID))
			refresh(a, d)
		})
	}()
}

func drawListTab(a App, c *ui.Context, pal *theme.Palette, d *Ask) {
	ui.Column(c.Key("arena-list-col")).Gap(8).Children(func() {
		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}
		if len(d.Arenas) == 0 {
			ui.Box(c.Key("arena-empty")).Padding(22, 12).Radius(10).Background(pal.Chip).Center().Children(func() {
				ui.Text(c, locales.ArenaEmpty()).FontSize(12.5).TextColor(pal.TextMuted)
			})
		} else {
			ui.Scroll(c.Key("arena-scroll")).MaxHeight(380).Children(func() {
				ui.Column(c.Key("arena-items")).Gap(10).Children(func() {
					for i := range d.Arenas {
						drawArenaCard(a, c, pal, d, &d.Arenas[i])
					}
				})
			})
		}
		ui.Row(c.Key("list-footer")).Justify(ui.End).Margin(4, 0, 0, 0).Children(func() {
			if style.Button(c, "close-dialog", locales.ArenaClose(), "esc", false, pal) {
				d.Open = false
			}
		})
	})
}

func drawArenaCard(a App, c *ui.Context, pal *theme.Palette, d *Ask, ar *proto.Arena) {
	card := ui.Column(c.Key("arena-"+ar.ID)).Padding(12).Gap(8).Radius(10).Background(pal.Chip)
	card.Children(func() {
		ui.Row(c.Key("hdr")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c.Key("meta")).Grow(1).MinWidth(0).Gap(2).Children(func() {
				title := ar.Prompt
				if title == "" {
					title = ar.Name
				}
				if title == "" {
					title = ar.ID
				}
				ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…")
				sub := fmt.Sprintf("%s · %s", ar.ID, ar.Base)
				if ar.Winner != "" {
					sub += " · winner: " + ar.Winner
				}
				ui.Text(c, sub).FontSize(11.5).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
			})
			statusBadge(c, pal, "st-"+ar.ID, ar.Status)
			if ar.Status == "running" {
				if style.SizedButton(c, "test-"+ar.ID, locales.ArenaRunTests(), "", false, 24, 6, pal) && !d.Busy {
					runArenaTest(a, d, ar.ID)
				}
			}
			if style.SizedButton(c, "close-"+ar.ID, locales.ArenaClose(), "", false, 24, 6, pal) && !d.Busy {
				closeArena(a, d, ar.ID)
			}
		})

		ui.Column(c.Key("contestants")).Gap(5).Children(func() {
			for _, ct := range ar.Contestants {
				drawContestantRow(a, c, pal, d, ar, ct)
			}
		})
	})
}

func drawContestantRow(a App, c *ui.Context, pal *theme.Palette, d *Ask, ar *proto.Arena, ct proto.ArenaContestant) {
	ui.Row(c.Key("ct-"+ar.ID+"-"+ct.Agent)).Padding(6, 8).Gap(8).Radius(7).Background(pal.Popover).AlignItems(ui.Center).Children(func() {
		ui.Column(c.Key("info")).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Row(c.Key("top")).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, ct.Agent).FontSize(12.5).FontWeight(600).TextColor(pal.Text)
				if ct.Status != "" {
					ui.Text(c, string(ct.Status)).FontSize(11).TextColor(pal.TextMuted)
				}
				if ct.CheckStatus != "" {
					statusBadge(c, pal, "chk-"+ct.Agent, ct.CheckStatus)
				}
			})
			stats := fmt.Sprintf("+%d -%d · %s files · %s commits",
				ct.Added, ct.Removed, strconv.Itoa(ct.Files), strconv.Itoa(ct.Commits))
			ui.Text(c, stats).FontSize(11).FontFeatures("tnum").TextColor(pal.TextFaint).SingleLine()
		})

		if ct.SessionID != "" {
			if style.SizedButton(c, "focus-"+ct.Agent, locales.ArenaFocus(), "", false, 24, 6, pal) {
				sid := ct.SessionID
				d.Open = false
				a.SelectSession(sid)
			}
		}
		if ar.Status == "running" {
			if style.SizedButton(c, "pick-"+ct.Agent, locales.ArenaPickWinner(), "", true, 24, 6, pal) && !d.Busy {
				pickArenaWinner(a, d, ar.ID, ct.Agent)
			}
		}
	})
}

func statusBadge(c *ui.Context, pal *theme.Palette, key, label string) {
	bg, fg := pal.Chip, pal.TextMuted
	switch label {
	case "passed", "picked", "landed":
		bg, fg = pal.Online.Alpha(0.16), pal.Online
	case "failed", "rejected":
		bg, fg = pal.Danger.Alpha(0.16), pal.Danger
	case "running", "queued":
		bg, fg = pal.Accent.Alpha(0.16), pal.Accent
	case "needs_review", "bounced":
		bg, fg = pal.Attention.Alpha(0.18), pal.Attention
	}
	ui.Box(c.Key(key)).Padding(2, 7).Radius(5).Background(bg).Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(550).TextColor(fg).SingleLine()
	})
}

func runArenaTest(a App, d *Ask, arenaID string) {
	if d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	cmd := strings.TrimSpace(d.TestCmd)
	d.Busy = true
	d.Err = ""
	go func() {
		var updated proto.Arena
		err := cl.Call("arena.test", proto.ArenaTestParams{ID: arenaID, Cmd: cmd}, &updated)
		a.Main(func() {
			if a.ArenaAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			replaceArena(d, updated)
			a.Toast(locales.ArenaTestsDoneToast())
		})
	}()
}

func pickArenaWinner(a App, d *Ask, arenaID, agent string) {
	if d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		var updated proto.Arena
		err := cl.Call("arena.pick", proto.ArenaPickParams{ID: arenaID, Winner: agent}, &updated)
		a.Main(func() {
			if a.ArenaAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			replaceArena(d, updated)
			a.Toast(locales.ArenaPickedToast(agent))
		})
	}()
}

func closeArena(a App, d *Ask, arenaID string) {
	if d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		err := cl.Call("arena.close", proto.ArenaCloseParams{ID: arenaID}, nil)
		a.Main(func() {
			if a.ArenaAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Arenas = slices.DeleteFunc(d.Arenas, func(ar proto.Arena) bool { return ar.ID == arenaID })
		})
	}()
}

func replaceArena(d *Ask, updated proto.Arena) {
	for i := range d.Arenas {
		if d.Arenas[i].ID == updated.ID {
			d.Arenas[i] = updated
			return
		}
	}
}
