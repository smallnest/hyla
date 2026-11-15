package main

import (
	"os"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/client"
	"hyla/internal/feishu"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/controls"
	uisettings "hyla/internal/ui/settings"
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

// feishuRead asks this machine's hylad for the bot's state.
func (a *App) feishuRead() {
	h := a.local()
	if h.Client == nil {
		return
	}
	// A call of its own, which decodes the state: Go reports only errors.
	h.Client.Go("feishu.status", nil, func(err error) {
		if err != nil {
			return
		}
		a.update(func() {
			if st, ok := a.feishuState(h.Client); ok {
				a.Ss.Feishu.St, a.Ss.Feishu.Known = st, true
			}
		})
	})
}

// feishuState is the bot's state, as this machine's hylad tells it.
func (a *App) feishuState(cl *client.Client) (proto.FeishuState, bool) {
	var st proto.FeishuState
	if err := cl.Call("feishu.status", nil, &st); err != nil {
		return st, false
	}
	return st, true
}

// feishuPage is Settings › Feishu.
func (a *App) feishuPage(c *ui.Context, pal *theme.Palette) {
	a.feishuGroup(c, pal)
	if settings.FeishuSet(a.Applied.Feishu) {
		a.feishuYoloGroup(c, pal)
	}
}

// feishuYoloGroup is Settings › Feishu's yolo modes: every agent runs
// without its permission prompts or sandbox, as feishu-yolo says, unless
// an agent's switch turns it off.
func (a *App) feishuYoloGroup(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
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

// feishuGroup is Settings › Feishu: the app and its secret, who
// may use the bot, how a prompt runs, and what its cards show.
func (a *App) feishuGroup(c *ui.Context, pal *theme.Palette) {
	d := &a.Ss.Feishu
	set := a.Applied.Feishu
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
		time.AfterFunc(time.Second, func() { a.feishuRead() })
	}
	// The fields', as the settings have them, until they change.
	if !d.Filled {
		d.Allow = strings.Join(set.Allow, ", ")
		d.Hosts = strings.Join(set.Host, ", ")
		d.Dir = set.Dir
		d.Filled = true
	}
	d.Timeout = settings.FormatDuration(time.Duration(feishuTimeout(set)))
	d.Stream = settings.FormatDuration(time.Duration(feishuStream(set)))
	note := locales.SetFeishuDetail()
	if len(a.Machines) > 1 {
		note += " " + locales.SetShellThisComputer()
	}
	kit.Group(c, pal, locales.SetFeishu(), note, func() {
		kit.Row(c, pal, locales.SetFeishuAppId(), a.feishuStatusLine(d), func() {
			controls.TextField(c, "feishu-app", &d.App, "cli_…", 200, pal)
			if d.Busy {
				ui.Text(c, locales.SetFeishuConnecting()).FontSize(12.5).TextColor(pal.TextMuted).SingleLine()
			} else if style.Button(c, "feishu-connect", locales.SetFeishuCheck(), "", true, pal) {
				a.feishuConnect(d)
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
	a.feishuRow(c, pal, locales.SetFeishuOffState(), a.feishuStatusLine(d), func() {
		if style.Button(c, "feishu-off", locales.SetFeishuDisconnect(), "", false, pal) {
			a.feishuDisconnect(d)
		}
	})
	a.feishuRow(c, pal, locales.SetFeishuAllow(), list(set.Allow, locales.SetFeishuOnlySetup()), func() {
		controls.TextField(c, "feishu-allow", &d.Allow, locales.SetFeishuAllowPlaceholder(), 200, pal).OnSubmit(func() { a.feishuRows(d) })
	})
	a.feishuRunRow(c, pal)
	dirDetail := proto.ShortDir(set.Dir)
	if set.Dir == "" {
		dirDetail = locales.SetFeishuDirDetail() + " · " + locales.SetFeishuDirDefault()
	}
	a.feishuRow(c, pal, locales.SetFeishuAgent(), a.feishuAgent(set.Agent), func() {
		agents := slices.Clone(feishuAgents)
		if !slices.Contains(agents, set.Agent) {
			agents = append(agents, set.Agent)
		}
		controls.Popup(c, "feishu-agent", &a.settingsIn.Feishu.Agent, agents, func(agent string) string {
			if agent == "default" {
				return locales.SetFeishuAgentDefault()
			}
			return agent
		}, 200, pal)
	})
	a.feishuRow(c, pal, locales.SetFeishuDir(), dirDetail, func() {
		controls.TextField(c, "feishu-dir", &d.Dir, locales.SetFeishuDirDefault(), 200, pal).OnSubmit(func() { a.feishuRows(d) })
	})
	a.feishuRow(c, pal, locales.SetFeishuHost(), locales.SetFeishuHostDetail(), func() {
		controls.TextField(c, "feishu-host", &d.Hosts, locales.SetFeishuHostPlaceholder(), 200, pal).OnSubmit(func() { a.feishuRows(d) })
	})
	a.feishuRow(c, pal, locales.SetFeishuTimeout(), locales.SetFeishuTimeoutDetail(), func() {
		controls.TextField(c, "feishu-timeout", &d.Timeout, "30m", 90, pal).OnSubmit(func() { a.feishuRows(d) })
	})
	a.feishuRow(c, pal, locales.SetFeishuStream(), locales.SetFeishuStreamDetail(), func() {
		controls.TextField(c, "feishu-stream", &d.Stream, "30m", 90, pal).OnSubmit(func() { a.feishuRows(d) })
	})
}

// feishuRunRow is the row of how a prompt runs its agent: a setting the
// app applies and saves as the segment changes, as the pages' other
// controls do, not a field of the form, which a submit saves.
func (a *App) feishuRunRow(c *ui.Context, pal *theme.Palette) {
	f := a.settingsIn.Feishu
	if f == nil {
		return
	}
	a.feishuRow(c, pal, locales.SetFeishuRun(), locales.SetFeishuRunDetail(), func() {
		kit.Segmented(c, "feishu-run", &f.Run, []string{settings.FeishuRunSession, settings.FeishuRunCommand},
			[]string{locales.SetFeishuRunSession(), locales.SetFeishuRunCommand()})
	})
}

// feishuRow is the group of a row, its controls by build.
func (a *App) feishuRow(c *ui.Context, pal *theme.Palette, label, detail string, build func()) {
	// The title, as its row reads it.
	title := label
	kit.Group(c, pal, label, "", func() {
		kit.Row(c, pal, title, detail, build)
	})
}

// feishuRows saves what the fields hold, as one submits: a field the app
// cannot read, as a time that does not parse, stays as it was.
func (a *App) feishuRows(d *uisettings.FeishuDraft) {
	f := a.feishu()
	f.Allow = splitList(d.Allow)
	f.Dir = strings.TrimSpace(d.Dir)
	f.Host = splitList(d.Hosts)
	f.Timeout = timeoutValue(d.Timeout, feishuTimeout(f))
	f.Stream = timeoutValue(d.Stream, feishuStream(f))
	a.feishuNormal()
}

// feishuNormal fills in what a Setup of the configuration fills in: the
// times, and the agent and the way a prompt runs, which have each a
// default.
func (a *App) feishuNormal() {
	f := a.feishu()
	f.Timeout = feishuTimeout(f)
	f.Stream = feishuStream(f)
	if f.Agent == "" {
		f.Agent = "default"
	}
	if f.Run == "" {
		f.Run = settings.FeishuRunSession
	}
}

// feishuAppEnv is the app's environment, which the bot's agent commands
// run in, as hylad reads it now: a command reaches hylad as its window
// starts it, or from a login shell's. The variables a login shell's
// profile reads live, as a token or a socket that changes, are read
// again with each setting the group applies. The variables of AUTH,
// Sockets, MCP proxies and tokens are left out, and every variable with
// a secret in its name: the configuration keeps them as they were.
func feishuAppEnv() []string {
	drop := func(k string) bool {
		k = strings.ToUpper(k)
		for _, p := range []string{"AUTH", "TOKEN", "SECRET", "PASSWORD", "KEY", "CREDENTIAL", "SOCK", "PROXY"} {
			if strings.Contains(k, p) {
				return true
			}
		}
		return k == "PATH" || k == "PWD" || k == "OLDPWD" || k == "SHLVL" || k == "_" || k == "TMPDIR" ||
			strings.HasPrefix(k, "TMP") || k == "TERM" || k == "COLORTERM" || k == "SHELL" || strings.HasPrefix(k, "SSH_")
	}
	// The providers' keys, as ANTHROPIC_API_KEY, which an agent needs.
	keep := []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "DEEPSEEK_API_KEY", "MOONSHOT_API_KEY",
		"ARK_API_KEY", "ZHIPU_API_KEY", "MIMOCODE_API_KEY", "OPENROUTER_API_KEY", "GROQ_API_KEY", "MISTRAL_API_KEY", "XAI_API_KEY"}
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if len(kv) > 32<<10 || strings.ContainsAny(kv, "\n\r\x00") {
			continue
		}
		if slices.Contains(keep, k) || !drop(k) {
			env = append(env, kv)
		}
	}
	return env
}

// feishuTimeout and feishuStream are a turn's time limit, and a card's
// updates, as the configuration has them or the default.
func feishuTimeout(f *settings.Feishu) int64 {
	if f.Timeout > 0 {
		return f.Timeout
	}
	return int64(settings.FeishuDefaultTimeout)
}

func feishuStream(f *settings.Feishu) int64 {
	if f.Stream > 0 {
		return f.Stream
	}
	return int64(settings.FeishuDefaultStream)
}

// feishu is the Feishu settings the group edits, and applies.
func (a *App) feishu() *settings.Feishu {
	if a.settingsIn.Feishu == nil {
		a.settingsIn.Feishu = &settings.Feishu{}
	}
	return a.settingsIn.Feishu
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

// feishuAgent tells the agent a new session runs.
func (a *App) feishuAgent(agent string) string {
	if agent == "default" || agent == "" {
		return locales.SetFeishuAgentDefault()
	}
	return agent
}

// feishuStatusLine tells the bot of this machine: connected to which
// app, off, or why not.
func (a *App) feishuStatusLine(d *uisettings.FeishuDraft) string {
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

// feishuConnect checks the app with Feishu, keeps its secret and its ID,
// and connects the bot of this machine. The secret is needed when the ID
// is another app's, or none is kept.
func (a *App) feishuConnect(d *uisettings.FeishuDraft) {
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
		a.update(func() {
			d.Busy = false
			if err != nil {
				d.Err = err.Error()
				return
			}
			a.feishuKeep(d, app, secret, kept.Me)
		})
	}()
}

// feishuKeep saves the app's secret, keeping who set it up, writes the
// app to the configuration, and connects the bot.
func (a *App) feishuKeep(d *uisettings.FeishuDraft, app, secret, me string) {
	if err := settings.SaveFeishuSecret(settings.FeishuSecret{AppID: app, AppSecret: secret, Me: me}); err != nil {
		d.Err = err.Error()
		return
	}
	d.Secret = ""
	f := a.feishu()
	f.App = app
	if err := settings.ValidateFeishu(f); err != nil {
		d.Err = err.Error()
		return
	}
	a.feishuNormal()
	a.callOn(a.local(), "feishu.restart", nil, nil)
	a.feishuRead()
}

// feishuDisconnect takes the app off the configuration, and stops the
// bot.
func (a *App) feishuDisconnect(d *uisettings.FeishuDraft) {
	f := a.feishu()
	f.App = ""
	a.feishuNormal()
	a.callOn(a.local(), "feishu.restart", nil, nil)
	a.feishuRead()
}
