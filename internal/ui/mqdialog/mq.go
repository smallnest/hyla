package mqdialog

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

// Ask holds the Local Merge Queue dialog state.
type Ask struct {
	Open       bool
	Host       *host.Host
	Repo       string
	Target     string
	CheckCmd   string
	FlowPath   string
	FeishuChat string
	Cleanup    bool
	Loading    bool
	Loaded     bool
	Busy       bool
	Err        string
	Lanes      []proto.MergeLane
}

// Open initializes and opens the Local Merge Queue dialog and refreshes lanes.
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
	target, repo := "", ""
	if s := a.SelectedSession(); s != nil {
		target = s.ID
		if p := s.FocusPane(); p != nil {
			repo = p.WorkDir()
		}
	}
	d := &Ask{
		Open:    true,
		Host:    h,
		Repo:    repo,
		Target:  target,
		Cleanup: true,
	}
	a.SetMQAsk(d)
	refresh(a, d)
}

func refresh(a App, d *Ask) {
	if d == nil || d.Host == nil || d.Host.Client == nil || d.Loading {
		return
	}
	cl := d.Host.Client
	d.Loading = true
	go func() {
		var lanes []proto.MergeLane
		err := cl.Call("mq.list", proto.MergeQueueListParams{}, &lanes)
		a.Main(func() {
			if a.MQAskNow() != d {
				return
			}
			d.Loading = false
			d.Loaded = true
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Err = ""
			d.Lanes = lanes
		})
	}()
}

// View draws the Local Merge Queue dialog while d is non-nil.
func View(a App, c *ui.Context, pal *theme.Palette, d *Ask) {
	if d == nil {
		return
	}
	ui.DialogBase(c.Key("mq"), &d.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		style.Floating(panel.Width(680).MaxHeight(600).Padding(22).Gap(10).Radius(14).Background(pal.Popover), pal).
			Label(locales.MqTitle())

		ui.Row(c.Key("mq-hdr")).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Column(c.Key("mq-titles")).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, locales.MqTitle()).FontSize(15).FontWeight(650).TextColor(pal.Text)
				ui.Text(c, locales.MqSubtitle()).FontSize(12.5).TextColor(pal.TextMuted).LineHeight(1.35)
			})
			if style.QuietButton(c, "mq-refresh", locales.MqRefresh(), "", pal) {
				refresh(a, d)
			}
		})

		ui.Column(c.Key("mq-form")).Padding(10).Gap(8).Radius(10).Background(pal.Chip).Children(func() {
			submit := false
			ui.Row(c.Key("row1")).Gap(8).AlignItems(ui.Center).Children(func() {
				tIn := controls.TextField(c, "mq-target", &d.Target, locales.MqTargetPlaceholder(), 0, pal)
				tIn.AutoFocus()
				if tIn.Submitted() {
					submit = true
				}
				cIn := controls.TextField(c, "mq-check", &d.CheckCmd, locales.MqCheckPlaceholder(), 0, pal)
				if cIn.Submitted() {
					submit = true
				}
				canAdd := !d.Busy && strings.TrimSpace(d.Target) != ""
				if (style.FieldButton(c, "mq-enqueue", locales.MqEnqueue(), false, pal) || submit) && canAdd {
					enqueue(a, d)
				}
			})
			ui.Row(c.Key("row2")).Gap(8).AlignItems(ui.Center).Children(func() {
				controls.TextField(c, "mq-flow", &d.FlowPath, locales.MqFlowPlaceholder(), 0, pal)
				controls.TextField(c, "mq-feishu", &d.FeishuChat, locales.MqFeishuPlaceholder(), 0, pal)
			})
		})

		ui.Row(c.Key("mq-actions")).Gap(8).AlignItems(ui.Center).Children(func() {
			if style.ButtonDisabled(c, "mq-run", locales.MqRunQueue(), "", true, d.Busy || len(d.Lanes) == 0, pal) {
				runQueue(a, d)
			}
			if style.ButtonDisabled(c, "mq-clear", locales.MqClear(), "", false, d.Busy || len(d.Lanes) == 0, pal) {
				clearQueue(a, d)
			}
			ui.Spacer(c)
			if style.Button(c, "mq-close", locales.CommonCancel(), "esc", false, pal) {
				d.Open = false
			}
		})

		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}

		if len(d.Lanes) == 0 {
			ui.Box(c.Key("mq-empty")).Padding(22, 12).Radius(10).Background(pal.Chip).Center().Children(func() {
				ui.Text(c, locales.MqEmpty()).FontSize(12.5).TextColor(pal.TextMuted)
			})
		} else {
			ui.Scroll(c.Key("mq-scroll")).MaxHeight(320).Children(func() {
				ui.Column(c.Key("mq-list")).Gap(8).Children(func() {
					for i := range d.Lanes {
						drawLaneRow(a, c, pal, d, &d.Lanes[i])
					}
				})
			})
		}
	})

	if !d.Open {
		a.SetMQAsk(nil)
		a.FocusTerminal()
	}
}

func drawLaneRow(a App, c *ui.Context, pal *theme.Palette, d *Ask, lane *proto.MergeLane) {
	ui.Column(c.Key("lane-"+lane.ID)).Padding(10).Gap(6).Radius(10).Background(pal.Chip).Children(func() {
		ui.Row(c.Key("top")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c.Key("meta")).Grow(1).MinWidth(0).Gap(2).Children(func() {
				title := fmt.Sprintf("%s → %s", lane.Branch, lane.Base)
				if lane.Branch == "" {
					title = lane.ID
				}
				ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…")
				sub := lane.ID
				if lane.CheckCmd != "" {
					sub += " · " + lane.CheckCmd
				}
				if lane.Bounces > 0 {
					sub += " · bounces: " + strconv.Itoa(lane.Bounces)
				}
				ui.Text(c, sub).FontSize(11).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
			})

			laneBadge(c, pal, "st-"+lane.ID, lane.Status)

			if lane.SessionID != "" {
				if style.SizedButton(c, "focus-"+lane.ID, locales.MqFocus(), "", false, 24, 6, pal) {
					sid := lane.SessionID
					d.Open = false
					a.SelectSession(sid)
				}
			}
			if lane.Status == "needs_review" {
				if style.SizedButton(c, "approve-"+lane.ID, locales.MqApprove(), "", true, 24, 6, pal) && !d.Busy {
					approveLane(a, d, lane.ID)
				}
			}
			if lane.Status == "needs_review" || lane.Status == "queued" || lane.Status == "bounced" {
				if style.SizedButton(c, "reject-"+lane.ID, locales.MqReject(), "", false, 24, 6, pal) && !d.Busy {
					rejectLane(a, d, lane.ID)
				}
			}
			if style.SizedButton(c, "rm-"+lane.ID, locales.MqRemove(), "", false, 24, 6, pal) && !d.Busy {
				removeLane(a, d, lane.ID)
			}
		})

		if len(lane.Findings) > 0 {
			ui.Column(c.Key("findings")).Padding(6, 8).Gap(2).Radius(6).Background(pal.AttnSoft).Children(func() {
				ui.Text(c, locales.MqGuardWarning()).FontSize(11.5).FontWeight(600).TextColor(pal.Attention)
				for idx, f := range lane.Findings {
					msg := fmt.Sprintf("[%s] %s — %s", f.Kind, f.File, f.Reason)
					ui.Text(c, msg).FontSize(11).TextColor(pal.Text).SingleLine().Ellipsis("…")
					if idx >= 2 && len(lane.Findings) > 3 {
						break
					}
				}
			})
		}

		if lane.LastError != "" {
			ui.Text(c, lane.LastError).FontSize(11.5).TextColor(pal.Danger).SingleLine().Ellipsis("…")
		}
	})
}

func laneBadge(c *ui.Context, pal *theme.Palette, key, status string) {
	bg, fg := pal.Chip, pal.TextMuted
	switch status {
	case "landed":
		bg, fg = pal.Online.Alpha(0.16), pal.Online
	case "rejected":
		bg, fg = pal.Danger.Alpha(0.16), pal.Danger
	case "queued", "running":
		bg, fg = pal.Accent.Alpha(0.16), pal.Accent
	case "needs_review", "bounced":
		bg, fg = pal.Attention.Alpha(0.18), pal.Attention
	}
	ui.Box(c.Key(key)).Padding(2, 7).Radius(5).Background(bg).Children(func() {
		ui.Text(c, status).FontSize(11).FontWeight(550).TextColor(fg).SingleLine()
	})
}

func enqueue(a App, d *Ask) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	target := strings.TrimSpace(d.Target)
	if target == "" {
		return
	}
	cl := d.Host.Client
	repo := strings.TrimSpace(d.Repo)
	checkCmd := strings.TrimSpace(d.CheckCmd)
	flowPath := strings.TrimSpace(d.FlowPath)
	d.Busy = true
	d.Err = ""
	go func() {
		var lane proto.MergeLane
		err := cl.Call("mq.add", proto.MergeQueueAddParams{
			Target:   target,
			Repo:     repo,
			CheckCmd: checkCmd,
			FlowPath: flowPath,
		}, &lane)
		a.Main(func() {
			if a.MQAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Lanes = append(d.Lanes, lane)
			branch := lane.Branch
			if branch == "" {
				branch = lane.ID
			}
			a.Toast(locales.MqEnqueuedToast(branch))
		})
	}()
}

func runQueue(a App, d *Ask) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	checkCmd := strings.TrimSpace(d.CheckCmd)
	feishuChat := strings.TrimSpace(d.FeishuChat)
	cleanup := d.Cleanup
	browserPane := ""
	if s := a.SelectedSession(); s != nil {
		for _, p := range s.Panes {
			if p.Browser() {
				browserPane = p.ID
				break
			}
		}
	}
	d.Busy = true
	d.Err = ""
	go func() {
		var res proto.MergeQueueRunResult
		err := cl.Call("mq.run", proto.MergeQueueRunParams{
			Train:       4,
			CheckCmd:    checkCmd,
			Cleanup:     cleanup,
			FeishuChat:  feishuChat,
			BrowserPane: browserPane,
		}, &res)
		a.Main(func() {
			if a.MQAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Lanes = res.Lanes
			a.Toast(locales.MqRunDoneToast(
				strconv.Itoa(len(res.Landed)),
				strconv.Itoa(len(res.Bounced)),
				strconv.Itoa(len(res.NeedsReview)),
			))
		})
	}()
}

func approveLane(a App, d *Ask, id string) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		var lane proto.MergeLane
		err := cl.Call("mq.approve", proto.MergeQueueLaneParams{ID: id}, &lane)
		a.Main(func() {
			if a.MQAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			replaceLane(d, lane)
			a.Toast(locales.MqApprovedToast())
		})
	}()
}

func rejectLane(a App, d *Ask, id string) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		var lane proto.MergeLane
		err := cl.Call("mq.reject", proto.MergeQueueLaneParams{ID: id}, &lane)
		a.Main(func() {
			if a.MQAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			replaceLane(d, lane)
			a.Toast(locales.MqRejectedToast())
		})
	}()
}

func removeLane(a App, d *Ask, id string) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		err := cl.Call("mq.remove", proto.MergeQueueLaneParams{ID: id}, nil)
		a.Main(func() {
			if a.MQAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Lanes = slices.DeleteFunc(d.Lanes, func(l proto.MergeLane) bool { return l.ID == id })
		})
	}()
}

func clearQueue(a App, d *Ask) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		var remaining []proto.MergeLane
		err := cl.Call("mq.clear", proto.MergeQueueListParams{}, &remaining)
		a.Main(func() {
			if a.MQAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Lanes = remaining
		})
	}()
}

func replaceLane(d *Ask, lane proto.MergeLane) {
	for i := range d.Lanes {
		if d.Lanes[i].ID == lane.ID {
			d.Lanes[i] = lane
			return
		}
	}
}
