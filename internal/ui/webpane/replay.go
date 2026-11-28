package webpane

import (
	"fmt"
	"os"
	"path/filepath"
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

// ReplayAsk holds the Browser Replay dialog state.
type ReplayAsk struct {
	Open           bool
	PaneID         string
	Host           *host.Host
	FlowPath       string
	FeishuChat     string
	UpdateBaseline bool
	Flows          []string
	Busy           bool
	Err            string
}

// ReplayApp is what the Browser Replay dialog asks of the app window.
type ReplayApp interface {
	ReplayAskNow() *ReplayAsk
	SetReplayAsk(d *ReplayAsk)
	WebsNow() map[string]*Entry
	Toast(msg string)
	Main(fn func())
	FocusTerminal()
}

// OpenReplay discovers saved browser verification flows and opens the replay dialog.
func OpenReplay(a ReplayApp, h *host.Host, paneID, workDir, lastFlow string) {
	if h == nil || h.Client == nil {
		label := "local"
		if h != nil {
			label = h.Label()
		}
		a.Toast(locales.HostNotConnected(label))
		return
	}
	flows := discoverFlows(workDir)
	flowPath := strings.TrimSpace(lastFlow)
	if flowPath == "" && len(flows) > 0 {
		flowPath = flows[0]
	}
	a.SetReplayAsk(&ReplayAsk{
		Open:     true,
		PaneID:   paneID,
		Host:     h,
		FlowPath: flowPath,
		Flows:    flows,
	})
}

func discoverFlows(workDir string) []string {
	var dirs []string
	if workDir != "" {
		dirs = append(dirs, filepath.Join(workDir, ".hyla", "flows"))
	}
	if home := proto.Home(); home != "" {
		dirs = append(dirs, filepath.Join(home, ".hyla", "flows"))
	}
	seen := map[string]bool{}
	var out []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if ent.IsDir() || filepath.Ext(ent.Name()) != ".json" {
				continue
			}
			full := filepath.Join(dir, ent.Name())
			if seen[full] {
				continue
			}
			seen[full] = true
			out = append(out, full)
		}
	}
	slices.Sort(out)
	return out
}

// ReplayView draws the Browser Replay dialog while d is non-nil.
func ReplayView(a ReplayApp, c *ui.Context, pal *theme.Palette, d *ReplayAsk) {
	if d == nil {
		return
	}
	ui.DialogBase(c.Key("browser-replay"), &d.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		style.Floating(panel.Width(520).MaxHeight(520).Padding(22).Gap(10).Radius(14).Background(pal.Popover), pal).
			Label(locales.BrowserReplayTitle())

		ui.Text(c, locales.BrowserReplayTitle()).FontSize(15).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, locales.BrowserReplaySubtitle()).FontSize(12.5).TextColor(pal.TextMuted).LineHeight(1.35)

		if len(d.Flows) > 0 {
			ui.Row(c.Key("flow-chips")).Gap(6).Wrap().Children(func() {
				for _, fp := range d.Flows {
					name := filepath.Base(fp)
					selected := d.FlowPath == fp
					chip := ui.ButtonBase(c.Key("flow-"+fp)).Height(24).Padding(0, 10).Radius(6).
						AlignItems(ui.Center).Label(name).Tooltip(fp)
					if selected {
						chip.Background(pal.Accent).TextColor(ui.Hex("#ffffff"))
					} else {
						chip.Background(pal.Chip).TextColor(pal.Text)
						if chip.Hovered() {
							chip.Background(pal.RowHover)
						}
					}
					chip.Children(func() {
						ui.Text(c, name).FontSize(12).SingleLine()
					})
					if chip.Clicked() {
						d.FlowPath = fp
					}
				}
			})
		} else {
			ui.Text(c, locales.BrowserReplayNoFlows()).FontSize(12).TextColor(pal.TextFaint)
		}

		submit := false
		ui.Row(c.Key("path-row")).Children(func() {
			in := controls.TextField(c, "replay-path", &d.FlowPath, locales.BrowserReplayPathPlaceholder(), 0, pal)
			in.AutoFocus()
			if in.Submitted() {
				submit = true
			}
		})
		ui.Row(c.Key("feishu-row")).Children(func() {
			controls.TextField(c, "replay-feishu", &d.FeishuChat, locales.MqFeishuPlaceholder(), 0, pal)
		})
		ui.Row(c.Key("baseline-row")).Gap(8).AlignItems(ui.Center).Children(func() {
			style.Toggle(c.Key("replay-baseline"), &d.UpdateBaseline, pal)
			ui.Text(c, locales.BrowserReplayUpdateBaseline()).FontSize(12.5).TextColor(pal.Text)
		})

		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Danger)
		}

		canRun := !d.Busy && strings.TrimSpace(d.FlowPath) != ""
		ui.Row(c.Key("replay-actions")).Gap(8).Justify(ui.End).Margin(6, 0, 0, 0).Children(func() {
			if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				d.Open = false
			}
			if style.ButtonDisabled(c, "replay-run", locales.BrowserReplayRun(), "↩", true, !canRun, pal) || (submit && canRun) {
				runReplay(a, d)
			}
		})
	})

	if !d.Open {
		a.SetReplayAsk(nil)
		a.FocusTerminal()
	}
}

func runReplay(a ReplayApp, d *ReplayAsk) {
	if d.Host == nil || d.Host.Client == nil {
		return
	}
	flowPath := strings.TrimSpace(d.FlowPath)
	if flowPath == "" {
		return
	}
	paneID := d.PaneID
	feishuChat := strings.TrimSpace(d.FeishuChat)
	updateBaseline := d.UpdateBaseline
	cl := d.Host.Client

	if e := a.WebsNow()[paneID]; e != nil {
		e.Replaying = true
		e.LastFlowPath = flowPath
	}
	d.Open = false

	go func() {
		var res proto.BrowserReplayResult
		err := cl.Call("browser.replay", proto.BrowserReplayParams{
			Pane:           paneID,
			FlowPath:       flowPath,
			UpdateBaseline: updateBaseline,
			FeishuChat:     feishuChat,
		}, &res)
		a.Main(func() {
			if e := a.WebsNow()[paneID]; e != nil {
				e.Replaying = false
			}
			if err != nil {
				a.Toast(err.Error())
				return
			}
			if res.Passed {
				diff := fmt.Sprintf("%.2f", res.BaselineDiffRatio*100)
				a.Toast(locales.BrowserReplayPassedToast(strconv.FormatInt(res.DurationMs, 10), diff))
			} else {
				a.Toast(locales.BrowserReplayFailedToast(strconv.FormatInt(res.DurationMs, 10), strconv.Itoa(len(res.Steps))))
			}
		})
	}()
}
