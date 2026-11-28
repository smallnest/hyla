package checkpointdialog

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Ask holds the Time-Travel Checkpoints dialog state.
type Ask struct {
	Open        bool
	Host        *host.Host
	Target      string
	Label       string
	Loading     bool
	Loaded      bool
	Busy        bool
	Err         string
	Checkpoints []proto.Checkpoint
}

// Open initializes and opens the Time-Travel Checkpoints dialog and loads checkpoints.
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
	target := ""
	if s := a.SelectedSession(); s != nil {
		if p := s.FocusPane(); p != nil {
			target = p.ID
		} else {
			target = s.ID
		}
	}
	d := &Ask{
		Open:   true,
		Host:   h,
		Target: target,
	}
	a.SetCheckpointAsk(d)
	refresh(a, d)
}

func refresh(a App, d *Ask) {
	if d == nil || d.Host == nil || d.Host.Client == nil || d.Loading {
		return
	}
	cl := d.Host.Client
	target := strings.TrimSpace(d.Target)
	d.Loading = true
	go func() {
		var cps []proto.Checkpoint
		err := cl.Call("checkpoint.list", proto.CheckpointListParams{Target: target, Limit: 50}, &cps)
		a.Main(func() {
			if a.CheckpointAskNow() != d {
				return
			}
			d.Loading = false
			d.Loaded = true
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Err = ""
			d.Checkpoints = cps
		})
	}()
}

// View draws the Time-Travel Checkpoints dialog while d is non-nil.
func View(a App, c *ui.Context, pal *theme.Palette, d *Ask) {
	if d == nil {
		return
	}
	ui.DialogBase(c.Key("checkpoint"), &d.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		style.Floating(panel.Width(680).MaxHeight(580).Padding(22).Gap(10).Radius(14).Background(pal.Popover), pal).
			Label(locales.CpTitle())

		ui.Row(c.Key("cp-hdr")).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Column(c.Key("cp-titles")).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, locales.CpTitle()).FontSize(15).FontWeight(650).TextColor(pal.Text)
				ui.Text(c, locales.CpSubtitle()).FontSize(12.5).TextColor(pal.TextMuted).LineHeight(1.35)
			})
			if style.QuietButton(c, "cp-refresh", locales.MqRefresh(), "", pal) {
				refresh(a, d)
			}
		})

		ui.Row(c.Key("cp-form")).Padding(10).Gap(8).Radius(10).Background(pal.Chip).AlignItems(ui.Center).Children(func() {
			lIn := controls.TextField(c, "cp-label", &d.Label, locales.CpSaveLabelPlaceholder(), 0, pal)
			submit := lIn.Submitted()
			if (style.FieldButton(c, "cp-save", locales.CpSave(), true, pal) || submit) && !d.Busy {
				createCheckpoint(a, d)
			}
			if style.Button(c, "cp-close", locales.CommonCancel(), "esc", false, pal) {
				d.Open = false
			}
		})

		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}

		if len(d.Checkpoints) == 0 {
			ui.Box(c.Key("cp-empty")).Padding(22, 12).Radius(10).Background(pal.Chip).Center().Children(func() {
				ui.Text(c, locales.CpEmpty()).FontSize(12.5).TextColor(pal.TextMuted)
			})
		} else {
			ui.Scroll(c.Key("cp-scroll")).MaxHeight(340).Children(func() {
				ui.Column(c.Key("cp-list")).Gap(8).Children(func() {
					for i := range d.Checkpoints {
						drawCheckpointRow(a, c, pal, d, &d.Checkpoints[i])
					}
				})
			})
		}
	})

	if !d.Open {
		a.SetCheckpointAsk(nil)
		a.FocusTerminal()
	}
}

func drawCheckpointRow(a App, c *ui.Context, pal *theme.Palette, d *Ask, cp *proto.Checkpoint) {
	ui.Column(c.Key("cp-" + cp.ID)).Padding(10).Gap(6).Radius(10).Background(pal.Chip).Children(func() {
		ui.Row(c.Key("top")).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c.Key("meta")).Grow(1).MinWidth(0).Gap(2).Children(func() {
				title := strings.TrimSpace(cp.Label)
				if title == "" {
					short := cp.Tree
					if len(short) > 8 {
						short = short[:8]
					}
					title = fmt.Sprintf("%s @ %s", filepath.Base(cp.Root), short)
				}
				ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(pal.Text).SingleLine().Ellipsis("…")
				sub := fmt.Sprintf("%s · %s", time.UnixMilli(cp.At).Format("15:04:05"), cp.ID)
				if cp.Agent != "" {
					sub += " · " + cp.Agent
				}
				ui.Text(c, sub).FontSize(11).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
			})

			badgeText, bg, fg := locales.CpManualBadge(), pal.Accent.Alpha(0.16), pal.Accent
			if cp.Auto {
				badgeText, bg, fg = locales.CpAutoBadge(), pal.Chip, pal.TextMuted
			}
			ui.Box(c.Key("badge-" + cp.ID)).Padding(2, 7).Radius(5).Background(bg).Children(func() {
				ui.Text(c, badgeText).FontSize(11).FontWeight(550).TextColor(fg).SingleLine()
			})

			if style.SizedButton(c, "restore-"+cp.ID, locales.CpRestore(), "", false, 24, 6, pal) && !d.Busy {
				restoreCheckpoint(a, d, cp.ID)
			}
			if style.SizedButton(c, "fork-"+cp.ID, locales.CpFork(), "", true, 24, 6, pal) && !d.Busy {
				forkCheckpoint(a, d, cp.ID)
			}
		})
	})
}

func createCheckpoint(a App, d *Ask) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	target := strings.TrimSpace(d.Target)
	label := strings.TrimSpace(d.Label)
	d.Busy = true
	d.Err = ""
	go func() {
		var cp proto.Checkpoint
		err := cl.Call("checkpoint.create", proto.CheckpointCreateParams{Target: target, Label: label}, &cp)
		a.Main(func() {
			if a.CheckpointAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Label = ""
			d.Checkpoints = append([]proto.Checkpoint{cp}, d.Checkpoints...)
			a.Toast(locales.CpSavedToast(cp.ID))
		})
	}()
}

func restoreCheckpoint(a App, d *Ask, id string) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		var res proto.CheckpointRestoreResult
		err := cl.Call("checkpoint.restore", proto.CheckpointRestoreParams{ID: id}, &res)
		a.Main(func() {
			if a.CheckpointAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			a.Toast(locales.CpRestoredToast(res.SafetyBackupID))
			refresh(a, d)
		})
	}()
}

func forkCheckpoint(a App, d *Ask, id string) {
	if d.Busy || d.Host == nil || d.Host.Client == nil {
		return
	}
	cl := d.Host.Client
	d.Busy = true
	d.Err = ""
	go func() {
		var res proto.CheckpointForkResult
		err := cl.Call("checkpoint.fork", proto.CheckpointForkParams{ID: id}, &res)
		a.Main(func() {
			if a.CheckpointAskNow() != d {
				return
			}
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			d.Open = false
			a.Toast(locales.CpForkedToast(res.Branch))
			if res.Session.ID != "" {
				a.SelectSession(res.Session.ID)
			}
		})
	}()
}
