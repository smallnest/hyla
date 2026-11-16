package settings

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/client"
	"hyla/internal/feishu"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/settings/kit"
	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// The agents a new session from Feishu runs, unless the configuration
// names others.
var feishuAgents = []string{"default", "claude", "codex", "opencode", "pi", "gemini", "qwen", "kimi", "amp", "droid", "dsh"}

// feishuYoloAgents are the agents Settings › Feishu's yolo modes set:
// those whose commands take an option that skips their permission
// prompts, which their sessions and the prompts from Feishu take.
var feishuYoloAgents = []string{"claude", "codex", "opencode", "pi", "gemini", "qwen", "kimi", "amp", "droid", "cursor-agent"}

// Read asks this machine's hylad for the bot's state.
func Read(x App) {
	h := x.Local()
	if h.Client == nil {
		return
	}
	// A call of its own, which decodes the state: Go reports only errors.
	h.Client.Go("feishu.status", nil, func(err error) {
		if err != nil {
			return
		}
		x.Main(func() {
			if st, ok := stateOf(x, h.Client); ok {
				x.SettingsState().Feishu.St, x.SettingsState().Feishu.Known = st, true
			}
		})
	})
}

// stateOf is the bot's state, as this machine's hylad tells it.
func stateOf(x App, cl *client.Client) (proto.FeishuState, bool) {
	var st proto.FeishuState
	if err := cl.Call("feishu.status", nil, &st); err != nil {
		return st, false
	}
	return st, true
}

// Feishu is Settings › Feishu.
func Feishu(x App, c *ui.Context, pal *theme.Palette) {
	group(x, c, pal)
	if settings.FeishuSet(x.SettingsApplied().Feishu) {
		yoloGroup(x, c, pal)
	}
}

// yoloGroup is Settings › Feishu's yolo modes: every agent runs
// without its permission prompts or sandbox, as feishu-yolo says, unless
// an agent's switch turns it off.
func yoloGroup(x App, c *ui.Context, pal *theme.Palette) {
	s := x.Settings()
	if s.Feishu == nil {
		return
	}
	kit.Group(c, pal, locales.SetFeishuYolo(), locales.SetFeishuYoloDetail(), func() {
		kit.Row(c, pal, locales.SetFeishuYoloEvery(), locales.SetFeishuYoloEveryDetail(), func() {
			style.Toggle(c, &s.FeishuYolo, pal)
		})
		for _, agent := range feishuYoloAgents {
			detail := strings.Join(settings.YoloFlag(agent), " ")
			if _, named := settings.AgentYolo(s.Feishu, agent); named {
				// Its command says: an agent's own switch, not every
				// agent's, offers the mode it names.
				detail += " · " + locales.SetFeishuYoloCommand()
			}
			kit.Row(c, pal, agent, detail, func() {
				on := settings.IsYolo(s.Feishu, s.FeishuYolo, agent)
				style.Toggle(c, &on, pal)
				if on != settings.IsYolo(s.Feishu, s.FeishuYolo, agent) {
					s.Feishu.Yolo = settings.SetYolo(s.Feishu.Yolo, agent, on)
				}
			})
		}
	})
}

// group is Settings › Feishu: the app and its secret, who
// may use the bot, how a prompt runs, and what its cards show.
func group(x App, c *ui.Context, pal *theme.Palette) {
	d := &x.SettingsState().Feishu
	set := x.SettingsApplied().Feishu
	setup := settings.FeishuSet(set)
	if set == nil {
		set = &settings.Feishu{}
	}
	if !d.AppSet {
		d.App = set.App
	}
	d.AppSet = true
	if !d.Known && d.Err == "" {
		// Once the first frames show, with the window's.
		time.AfterFunc(time.Second, func() { Read(x) })
	}
	// The fields', as the settings have them, until they change.
	if !d.Filled {
		d.Allow = strings.Join(set.Allow, ", ")
		d.Hosts = strings.Join(set.Host, ", ")
		d.Dir = set.Dir
		d.Filled = true
	}
	d.Timeout = settings.FormatDuration(time.Duration(timeoutOf(set)))
	d.Stream = settings.FormatDuration(time.Duration(streamOf(set)))
	note := locales.SetFeishuDetail()
	if x.HostCount() > 1 {
		note += " " + locales.SetShellThisComputer()
	}
	kit.Group(c, pal, locales.SetFeishu(), note, func() {
		kit.Row(c, pal, locales.SetFeishuAppId(), statusLine(x, d), func() {
			controls.TextField(c, "feishu-app", &d.App, "cli_…", 200, pal)
			if d.Busy {
				ui.Text(c, locales.SetFeishuConnecting()).FontSize(12.5).TextColor(pal.TextMuted).SingleLine()
			} else if style.Button(c, "feishu-connect", locales.SetFeishuCheck(), "", true, pal) {
				connect(x, d)
			}
		})
		if d.Err != "" {
			ui.Text(c, d.Err).FontSize(12).TextColor(pal.Attention).LineHeight(1.35).Wrap().Padding(0, 0, 10, 0)
		}
	})
	kit.Group(c, pal, "", "", func() {
		kit.Row(c, pal, locales.SetFeishuSecret(), locales.SetFeishuSecretDetail(), func() {
			secret := controls.TextField(c, "feishu-secret", &d.Secret, locales.SetFeishuSecretPlaceholder(), 200, pal)
			if secret.Valid() {
				secret.Password()
			}
		})
	})
	// What a bot does, with the app set: its state and switch off, who may
	// use it, how a prompt runs, and where.
	if !setup {
		return
	}
	rowGroup(x, c, pal, locales.SetFeishuOffState(), statusLine(x, d), func() {
		if style.Button(c, "feishu-off", locales.SetFeishuDisconnect(), "", false, pal) {
			disconnect(x, d)
		}
	})
	rowGroup(x, c, pal, locales.SetFeishuAllow(), list(set.Allow, locales.SetFeishuOnlySetup()), func() {
		controls.TextField(c, "feishu-allow", &d.Allow, locales.SetFeishuAllowPlaceholder(), 200, pal).OnSubmit(func() { rows(x, d) })
	})
	runRow(x, c, pal)
	dirDetail := proto.ShortDir(set.Dir)
	if set.Dir == "" {
		dirDetail = locales.SetFeishuDirDetail() + " · " + locales.SetFeishuDirDefault()
	}
	rowGroup(x, c, pal, locales.SetFeishuAgent(), agentName(x, set.Agent), func() {
		agents := slices.Clone(feishuAgents)
		if !slices.Contains(agents, set.Agent) {
			agents = append(agents, set.Agent)
		}
		controls.Popup(c, "feishu-agent", &feishuSet(x).Agent, agents, func(agent string) string {
			if agent == "default" {
				return locales.SetFeishuAgentDefault()
			}
			return agent
		}, 200, pal)
	})
	rowGroup(x, c, pal, locales.SetFeishuDir(), dirDetail, func() {
		controls.TextField(c, "feishu-dir", &d.Dir, locales.SetFeishuDirDefault(), 200, pal).OnSubmit(func() { rows(x, d) })
	})
	rowGroup(x, c, pal, locales.SetFeishuHost(), locales.SetFeishuHostDetail(), func() {
		controls.TextField(c, "feishu-host", &d.Hosts, locales.SetFeishuHostPlaceholder(), 200, pal).OnSubmit(func() { rows(x, d) })
	})
	rowGroup(x, c, pal, locales.SetFeishuTimeout(), locales.SetFeishuTimeoutDetail(), func() {
		controls.TextField(c, "feishu-timeout", &d.Timeout, "30m", 90, pal).OnSubmit(func() { rows(x, d) })
	})
	rowGroup(x, c, pal, locales.SetFeishuStream(), locales.SetFeishuStreamDetail(), func() {
		controls.TextField(c, "feishu-stream", &d.Stream, "30m", 90, pal).OnSubmit(func() { rows(x, d) })
	})
}

// runRow is the row of how a prompt runs its agent: a setting the
// app applies and saves as the segment changes, as the pages' other
// controls do, not a field of the form, which a submit saves.
func runRow(x App, c *ui.Context, pal *theme.Palette) {
	f := feishuSet(x)
	if f == nil {
		return
	}
	rowGroup(x, c, pal, locales.SetFeishuRun(), locales.SetFeishuRunDetail(), func() {
		kit.Segmented(c, "feishu-run", &f.Run, []string{settings.FeishuRunSession, settings.FeishuRunCommand},
			[]string{locales.SetFeishuRunSession(), locales.SetFeishuRunCommand()})
	})
}

// rowGroup is the group of a row, its controls by build.
func rowGroup(x App, c *ui.Context, pal *theme.Palette, label, detail string, build func()) {
	// The title, as its row reads it.
	title := label
	kit.Group(c, pal, label, "", func() {
		kit.Row(c, pal, title, detail, build)
	})
}

// rowGroups saves what the fields hold, as one submits: a field the app
// cannot read, as a time that does not parse, stays as it was.
func rows(x App, d *draft.FeishuDraft) {
	f := feishuSet(x)
	f.Allow = splitList(d.Allow)
	f.Dir = strings.TrimSpace(d.Dir)
	f.Host = splitList(d.Hosts)
	f.Timeout = timeoutValue(d.Timeout, timeoutOf(f))
	f.Stream = timeoutValue(d.Stream, streamOf(f))
	normal(x)
}

// normal fills in what a Setup of the configuration fills in: the
// times, and the agent and the way a prompt runs, which have each a
// default.
func normal(x App) {
	f := feishuSet(x)
	f.Timeout = timeoutOf(f)
	f.Stream = streamOf(f)
	if f.Agent == "" {
		f.Agent = "default"
	}
	if f.Run == "" {
		f.Run = settings.FeishuRunSession
	}
}

// timeoutOf and streamOf are a turn's time limit, and a card's
// updates, as the configuration has them or the default.
func timeoutOf(f *settings.Feishu) int64 {
	if f.Timeout > 0 {
		return f.Timeout
	}
	return int64(settings.FeishuDefaultTimeout)
}

func streamOf(f *settings.Feishu) int64 {
	if f.Stream > 0 {
		return f.Stream
	}
	return int64(settings.FeishuDefaultStream)
}

// feishu is the Feishu settings the group edits, and applies.
func feishuSet(x App) *settings.Feishu {
	if x.Settings().Feishu == nil {
		x.Settings().Feishu = &settings.Feishu{}
	}
	return x.Settings().Feishu
}

// timeoutValue reads a time as a field types it, 30m or 1h30m: in
// milliseconds, or was when it cannot be read.
func timeoutValue(v string, was int64) int64 {
	d, err := settings.ParseDuration(v)
	if err != nil {
		return was
	}
	return int64(d)
}

// splitList reads a list as the user types it: by commas, without spaces.
func splitList(v string) []string {
	out := []string{}
	for _, x := range strings.Split(v, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// list tells a list, as a row says it, by commas.
func list(l []string, empty string) string {
	if len(l) == 0 {
		return empty
	}
	return strings.Join(l, ", ")
}

// agentName tells the agent a new session runs.
func agentName(x App, agent string) string {
	if agent == "default" || agent == "" {
		return locales.SetFeishuAgentDefault()
	}
	return agent
}

// statusLine tells the bot of this machine: connected to which
// app, off, or why not.
func statusLine(x App, d *draft.FeishuDraft) string {
	if !d.Known {
		return ""
	}
	switch {
	case d.St.Connected:
		return locales.SetFeishuConnected(d.St.App)
	case d.St.Configured && !d.St.Secret:
		return locales.SetFeishuSecretMissing()
	case d.St.Error != "":
		return d.St.Error
	}
	return locales.SetFeishuNotConnected()
}

// connect checks the app with Feishu, keeps its secret and its ID,
// and connects the bot of this machine. The secret is needed when the ID
// is another app's, or none is kept.
func connect(x App, d *draft.FeishuDraft) {
	app := strings.TrimSpace(d.App)
	if app == "" {
		d.Err = locales.SetFeishuSecretNeeded()
		return
	}
	kept, _ := settings.LoadFeishuSecret()
	secret := strings.TrimSpace(d.Secret)
	if secret == "" && app == kept.AppID {
		secret = kept.AppSecret
	}
	if secret == "" {
		d.Err = locales.SetFeishuSecretNeeded()
		return
	}
	d.Err, d.Busy = "", true
	// Feishu first, over the network: never on the window's thread.
	go func() {
		err := feishu.Probe(app, secret)
		x.Main(func() {
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			keep(x, d, app, secret, kept.Me)
		})
	}()
}

// keep saves the app's secret, keeping who set it up, writes the
// app to the configuration, and connects the bot.
func keep(x App, d *draft.FeishuDraft, app, secret, me string) {
	s := settings.FeishuSecret{AppID: app, AppSecret: secret, Me: me}
	if old, err := settings.LoadFeishuSecret(); err == nil && old.AppID == app {
		// The same app keeps the bot it named, which a connection reads
		// again anyway: a network that fails that read goes on with it.
		s.BotID, s.BotName = old.BotID, old.BotName
	}
	if err := settings.SaveFeishuSecret(s); err != nil {
		d.Err = err.Error()
		return
	}
	d.Secret = ""
	f := feishuSet(x)
	f.App = app
	if err := settings.ValidateFeishu(f); err != nil {
		d.Err = err.Error()
		return
	}
	normal(x)
	x.CallOnLocal("feishu.restart", nil, nil)
	Read(x)
}

// disconnect takes the app off the configuration, and stops the
// bot.
func disconnect(x App, d *draft.FeishuDraft) {
	f := feishuSet(x)
	f.App = ""
	normal(x)
	x.CallOnLocal("feishu.restart", nil, nil)
	Read(x)
}
