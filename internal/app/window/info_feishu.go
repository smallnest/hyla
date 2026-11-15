package window

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"hyla/internal/proto"
	"hyla/internal/ui/info"
	"hyla/internal/ui/theme"

	"hyla/locales"

	"hyla/internal/ui/glyph"
)

// feishuStatusTick is the time between the reads of the bot's state.
const feishuStatusTick = 5 * time.Second

// infoFeishu is the Info view's card of the Feishu bot: the app it
// connects to, whether hylad connects, and who may use it, while
// feishu-app names one. While the bot connects, the view reads the state
// again, for the connection to drop.
func (a *App) infoFeishu(c *ui.Context, pal *theme.Palette, p *proto.Pane) {
	set := a.settingsIn.Feishu
	if set == nil || set.App == "" {
		return
	}
	st := a.InfoViews().Of(p)
	connected := st.Feishu != nil && st.Feishu.Connected
	aside := locales.SetFeishuOff()
	if connected {
		aside = locales.SetFeishuOn()
	}
	// The state first, once, else the card would show off for a moment;
	// then again as the bot connects, for the connection to drop.
	if !st.FeishuGot && (st.Feishu == nil || connected && time.Since(st.FeishuAt) > feishuStatusTick) {
		st.FeishuGot = true
		a.callOn(a.hostOf(p.ID), "feishu.status", nil, func(err error) {
			a.UpdateNow(func() {
				if a.InfoViews().Of(p) == st {
					st.FeishuGot = false
				}
			})
		})
	}
	if st.Feishu == nil || st.FeishuGot {
		// The state, once hylad answers; the card waits for it.
		return
	}
	who := locales.SetFeishuAllow() + ": " + locales.SetFeishuOnlySetup()
	if len(set.Allow) > 0 {
		who = locales.SetFeishuAllow() + ": " + strings.Join(set.Allow, ", ")
	}
	info.Card(c, a, pal, info.FoldFeishu, glyph.Link, locales.SetFeishu(), aside, func() {
		info.Rows(c, func() {
			ui.Column(c).Padding(4, 12, 10, 12).Gap(4).Children(func() {
				ui.Text(c, who).FontSize(12).TextColor(pal.TextMuted).Wrap()
				if st.Feishu != nil && !st.Feishu.Connected {
					why := st.Feishu.Error
					if why == "" {
						why = "not connected"
					}
					ui.Text(c, why).FontSize(11.5).TextColor(pal.TextFaint).Wrap()
				}
				ui.Text(c, fmt.Sprintf("%s, %s", locales.SetFeishuSetup(), set.Agent)).FontSize(11.5).TextColor(pal.TextFaint).Wrap().FontFeatures("tnum")
			})
		})
	})
}
