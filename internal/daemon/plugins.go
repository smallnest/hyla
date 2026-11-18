package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"hyla/internal/build"
	"hyla/internal/feishu"
	_ "hyla/internal/plugins/builtins/magpie"
	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
	"hyla/internal/settings"
)

func (d *Daemon) pluginStore() *manifest.Store {
	if d.dir == "" {
		return manifest.DefaultStore()
	}
	return &manifest.Store{BaseDir: d.dir}
}

func (d *Daemon) initPlugins() {
	d.ensureDefaultChannels()
	st := d.pluginStore()
	if _, err := manifest.ApplyBase(st); err != nil && d.log != nil {
		d.log.Printf("plugins base apply: %v", err)
	}
	// Feishu was a bundled plugin for one version: what its page kept of
	// the bot belongs to the configuration, which is what the bot reads.
	// Its entry goes before the manager discovers the plugins, so that
	// nothing lists it.
	if err := feishu.MigratePluginConfig(st); err != nil && d.log != nil {
		d.log.Printf("feishu migration: %v", err)
	}
	d.plugins = daemonhost.NewManager(daemonhost.ManagerOptions{
		HostKind:    daemonhost.HostKindDaemon,
		HylaVersion: build.Version(),
		Store:       st,
		Bridge:      &daemonPluginBridge{d: d},
		LocaleFunc:  func() string { return settings.Load().Language },
		ThemeFunc: func() (string, bool) {
			app := settings.Load().Appearance
			if t := d.theme.Load(); t != nil {
				// Heuristic dark check on background luminance if theme loaded.
				bg := t.Background
				dark := (0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)) < 128
				return app, dark
			}
			return app, app == "dark"
		},
	})
	if err := d.plugins.Reload(); err != nil && d.log != nil {
		d.log.Printf("plugins reload: %v", err)
	}
}

func (d *Daemon) notifyPluginStateChanges(st proto.State) {
	if d.plugins == nil {
		return
	}
	var prevDark bool
	if t := d.theme.Load(); t != nil {
		bg := t.Background
		prevDark = (0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)) < 128
	}
	d.pluginStateMu.Lock()
	prev := d.lastPluginState
	prevWasDark := d.lastPluginDark
	d.lastPluginState = st
	d.lastPluginDark = prevDark
	d.pluginStateMu.Unlock()

	d.plugins.DiffAndNotifyState(prev, st, prevWasDark, prevDark, "")
}

// pluginList returns status for all installed/linked/bundled plugins.
func (d *Daemon) pluginList() []rpc.PluginStatus {
	if d.plugins != nil {
		return d.plugins.ListStatuses()
	}
	tmp := daemonhost.NewManager(daemonhost.ManagerOptions{
		HostKind: daemonhost.HostKindDaemon,
		Store:    d.pluginStore(),
	})
	return tmp.ListStatuses()
}

// pluginCall proxies an explicit RPC method to pluginID hosted in hylad.
func (d *Daemon) pluginCall(p rpc.PluginCallParams) (json.RawMessage, error) {
	if d.plugins == nil {
		return nil, errors.New("plugin host not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var out json.RawMessage
	if err := d.plugins.Call(ctx, p.PluginID, p.Method, p.Params, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		out = json.RawMessage("null")
	}
	return out, nil
}

// pluginDetectAsync asks a plugin's agent.detect RPC to classify pane screen
// outside pn.mu to prevent lock inversion with Plugin->Host callbacks.
func (d *Daemon) pluginDetectAsync(pn *pane, agent, screen, title, progress, proc, dir string) {
	if d.plugins == nil {
		return
	}
	meta, ok := proto.AgentProvider(agent)
	if !ok || meta.PluginID == "" {
		return
	}
	pn.mu.Lock()
	if pn.detectPending {
		pn.mu.Unlock()
		return
	}
	pn.detectPending = true
	paneID, sid := pn.info.ID, pn.sid
	pn.mu.Unlock()

	go func() {
		defer func() {
			pn.mu.Lock()
			pn.detectPending = false
			pn.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var res rpc.AgentDetectResult
		err := d.plugins.Call(ctx, meta.PluginID, "agent.detect", rpc.AgentDetectParams{
			Pane:     paneID,
			Session:  sid,
			Screen:   screen,
			Title:    title,
			Progress: progress,
			Process:  proc,
			Dir:      dir,
		}, &res)
		if err != nil || res.Skip || res.State == "" {
			return
		}
		pn.mu.Lock()
		changed := false
		if !pn.closed && !pn.declared() && pn.info.Status != proto.Exited {
			if res.State != pn.info.Status || res.Message != pn.info.Message {
				pn.setStatus(res.State)
				pn.info.Message = res.Message
				pn.info.Kind = res.WaitKind
				if res.State != proto.Working {
					pn.info.Unread = !pn.viewed()
				}
				changed = true
			}
		}
		pn.mu.Unlock()
		if changed {
			d.changed()
		}
	}()
}

func (d *Daemon) pluginTimeline(p proto.TimelineParams, agent, conv, dir string, pn *pane) (proto.Timeline, bool, error) {
	if d.plugins == nil {
		return proto.Timeline{}, false, nil
	}
	meta, ok := proto.AgentProvider(agent)
	if !ok || meta.PluginID == "" || !meta.Timeline {
		return proto.Timeline{}, false, nil
	}
	paneID, sid := p.Pane, p.Session
	if pn != nil {
		paneID, sid = pn.info.ID, pn.sid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res rpc.AgentReadTimelineResult
	err := d.plugins.Call(ctx, meta.PluginID, "agent.readTimeline", rpc.AgentReadTimelineParams{
		Session:        sid,
		Pane:           paneID,
		Agent:          agent,
		ConversationID: conv,
		Dir:            dir,
	}, &res)
	if err != nil {
		return proto.Timeline{}, true, err
	}
	if res.Timeline.Read == 0 {
		res.Timeline.Read = time.Now().UnixMilli()
	}
	if res.Timeline.Agent == "" {
		res.Timeline.Agent = agent
	}
	if res.Timeline.Pane == "" {
		res.Timeline.Pane = paneID
	}
	if res.Timeline.Session == "" {
		res.Timeline.Session = sid
	}
	return res.Timeline, true, nil
}

func (d *Daemon) pluginChat(p proto.ChatParams, agent, conv, dir, paneID string, takes bool) (proto.Chat, bool, error) {
	if d.plugins == nil {
		return proto.Chat{}, false, nil
	}
	meta, ok := proto.AgentProvider(agent)
	if !ok || meta.PluginID == "" || !meta.Chat {
		return proto.Chat{}, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res rpc.AgentReadChatResult
	err := d.plugins.Call(ctx, meta.PluginID, "agent.readChat", rpc.AgentReadChatParams{
		Session:        p.Session,
		Pane:           paneID,
		Agent:          agent,
		ConversationID: conv,
		Dir:            dir,
	}, &res)
	if err != nil {
		return proto.Chat{}, true, err
	}
	if res.Chat.Read == 0 {
		res.Chat.Read = time.Now().UnixMilli()
	}
	if res.Chat.Agent == "" {
		res.Chat.Agent = agent
	}
	if res.Chat.Pane == "" {
		res.Chat.Pane = paneID
	}
	res.Chat.Input = takes
	return res.Chat, true, nil
}

func (d *Daemon) pluginChatSend(p proto.ChatSendParams, pn *pane, agent, conv, dir string) (bool, bool, error) {
	if d.plugins == nil {
		return false, false, nil
	}
	meta, ok := proto.AgentProvider(agent)
	if !ok || meta.PluginID == "" || !meta.Chat {
		return false, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res rpc.AgentSendChatResult
	err := d.plugins.Call(ctx, meta.PluginID, "agent.sendChat", rpc.AgentSendChatParams{
		Session:        pn.sid,
		Pane:           pn.info.ID,
		Agent:          agent,
		ConversationID: conv,
		Dir:            dir,
		Text:           p.Text,
		AnswerDir:      pn.info.ChannelAnswerDir(),
	}, &res)
	if err != nil {
		return false, true, err
	}
	if res.Error != "" {
		return false, true, errors.New(res.Error)
	}
	return res.Started, true, nil
}

func (d *Daemon) pluginSubagents(pn *pane, agent, conv, dir string) (proto.PaneSubagents, bool) {
	if d.plugins == nil {
		return proto.PaneSubagents{}, false
	}
	meta, ok := proto.AgentProvider(agent)
	if !ok || meta.PluginID == "" || !meta.Subagents {
		return proto.PaneSubagents{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var res rpc.AgentReadSubagentsResult
	err := d.plugins.Call(ctx, meta.PluginID, "agent.readSubagents", rpc.AgentReadSubagentsParams{
		Session:        pn.sid,
		Pane:           pn.info.ID,
		Agent:          agent,
		ConversationID: conv,
		Dir:            dir,
	}, &res)
	if err != nil {
		return proto.PaneSubagents{}, false
	}
	return proto.PaneSubagents{
		Session: pn.sid,
		Pane:    pn.info.ID,
		Agent:   agent,
		Subs:    res.Subs,
		Note:    res.Note,
	}, true
}

func (d *Daemon) pluginResumeCommand(a *proto.AgentSession, dir string) []string {
	if a == nil || !agentSessionRe.MatchString(a.ID) {
		return nil
	}
	if cmd := resumeCommand(a); len(cmd) > 0 {
		return cmd
	}
	if d.plugins == nil {
		return nil
	}
	meta, ok := proto.AgentProvider(a.Agent)
	if !ok || meta.PluginID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var res rpc.AgentResumeCommandResult
	if err := d.plugins.Call(ctx, meta.PluginID, "agent.resumeCommand", rpc.AgentResumeCommandParams{
		Agent:          a.Agent,
		ConversationID: a.ID,
		Dir:            dir,
	}, &res); err != nil {
		return nil
	}
	return res.Argv
}

// daemonPluginBridge implements daemonhost.HostBridge for plugins hosted by hylad.
type daemonPluginBridge struct {
	d *Daemon
}

func (b *daemonPluginBridge) GetState(ctx context.Context, pluginID string, p rpc.HostGetStateParams) (*rpc.HostGetStateResult, error) {
	st := b.d.state()
	app := settings.Load().Appearance
	dark := app == "dark"
	if t := b.d.theme.Load(); t != nil {
		bg := t.Background
		dark = (0.299*float64(bg.R) + 0.587*float64(bg.G) + 0.114*float64(bg.B)) < 128
	}
	res := &rpc.HostGetStateResult{
		State:          st,
		ActiveSession:  b.d.viewedID(),
		Dark:           dark,
		Appearance:     app,
		ControlAllowed: settings.Load().RemoteControl,
	}
	for _, s := range st.Sessions {
		if s.ID == res.ActiveSession {
			res.FocusedPane = s.Focus
			break
		}
	}
	return res, nil
}

func (b *daemonPluginBridge) ReadScreen(ctx context.Context, pluginID string, p rpc.PaneReadScreenParams) (*rpc.PaneReadScreenResult, error) {
	pn := b.d.findPane(p.Pane)
	if pn == nil {
		return nil, rpc.NewError(rpc.ErrCodeTargetNotFound, fmt.Sprintf("pane %q not found", p.Pane))
	}
	return &rpc.PaneReadScreenResult{Text: pn.readText(p.Scrollback, p.Lines)}, nil
}

func (b *daemonPluginBridge) GetEnv(ctx context.Context, pluginID string, p rpc.PaneGetEnvParams) (*rpc.PaneGetEnvResult, error) {
	pn := b.d.findPane(p.Pane)
	if pn == nil {
		return nil, rpc.NewError(rpc.ErrCodeTargetNotFound, fmt.Sprintf("pane %q not found", p.Pane))
	}
	pn.mu.Lock()
	res := &rpc.PaneGetEnvResult{
		Dir:            pn.info.WorkDir(),
		Process:        pn.info.Process,
		Agent:          pn.info.Agent,
		AgentSessionID: pn.info.AgentSessionID,
	}
	pn.mu.Unlock()
	b.d.mu.Lock()
	res.Env = append([]string(nil), b.d.appEnv...)
	b.d.mu.Unlock()
	return res, nil
}

func (b *daemonPluginBridge) CreateSession(ctx context.Context, pluginID string, p proto.CreateParams) (*rpc.SessionCreateResult, error) {
	sess, err := b.d.create(p)
	if err != nil {
		return nil, err
	}
	return &rpc.SessionCreateResult{SessionID: sess.ID, PaneID: sess.Focus}, nil
}

func (b *daemonPluginBridge) SendInput(ctx context.Context, pluginID string, p rpc.PaneSendInputParams) error {
	pn := b.d.findPane(p.Pane)
	if pn == nil {
		return rpc.NewError(rpc.ErrCodeTargetNotFound, fmt.Sprintf("pane %q not found", p.Pane))
	}
	if pn.term == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "pane does not accept terminal input")
	}
	pn.input([]byte(p.Data))
	return nil
}

func (b *daemonPluginBridge) SendKey(ctx context.Context, pluginID string, p rpc.PaneSendKeyParams) error {
	pn := b.d.findPane(p.Pane)
	if pn == nil {
		return rpc.NewError(rpc.ErrCodeTargetNotFound, fmt.Sprintf("pane %q not found", p.Pane))
	}
	if pn.term == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "pane does not accept terminal keys")
	}
	data, ok := encodeKeyName(p.Key)
	if !ok {
		return rpc.NewError(rpc.ErrCodeInvalidParams, fmt.Sprintf("unsupported key %q", p.Key))
	}
	pn.input(data)
	return nil
}

func (b *daemonPluginBridge) ReportStatus(ctx context.Context, pluginID string, p rpc.PaneReportStatusParams) error {
	pn := b.d.findPane(p.Pane)
	if pn == nil {
		return rpc.NewError(rpc.ErrCodeTargetNotFound, fmt.Sprintf("pane %q not found", p.Pane))
	}
	pn.mu.Lock()
	switch strings.ToLower(p.Status) {
	case "idle", "done":
		pn.setStatus(proto.Idle)
	case "working", "busy", "running":
		pn.setStatus(proto.Working)
	case "waiting", "blocked":
		pn.setStatus(proto.Waiting)
		pn.info.Unread = !pn.viewed()
	case "error", "failed":
		pn.setStatus(proto.Failed)
		pn.info.Unread = !pn.viewed()
	}
	if p.Message != "" {
		pn.info.Message = p.Message
	}
	if p.Kind != "" {
		pn.info.Kind = p.Kind
	}
	if p.Progress >= 0 {
		pn.info.Progress = p.Progress
	}
	pn.mu.Unlock()
	b.d.changed()
	return nil
}

func (b *daemonPluginBridge) FocusPane(ctx context.Context, pluginID string, p rpc.PaneFocusParams) error {
	if !settings.Load().RemoteControl {
		return rpc.NewError(rpc.ErrCodeRemoteControlDisabled, "remote control is disabled")
	}
	b.d.mu.Lock()
	var s *session
	var pn *pane
	if p.Pane != "" {
		pn = b.d.findPaneLocked(p.Pane)
		if pn != nil {
			s = b.d.sessionOfLocked(pn)
		}
	} else if p.Session != "" {
		s = b.d.findSessionLocked(p.Session)
		if s != nil {
			pn = focusedLocked(s)
		}
	}
	if s == nil {
		b.d.mu.Unlock()
		return rpc.NewError(rpc.ErrCodeTargetNotFound, "target session/pane not found")
	}
	if pn != nil && slices.Contains(s.panes, pn) {
		s.focus = pn.info.ID
	}
	st := b.d.stateLocked()
	for sub := range b.d.subs {
		sub.event(proto.Message{Event: "state", State: &st})
		sub.event(proto.Message{Event: "select", Select: s.id})
	}
	b.d.mu.Unlock()
	b.d.changed()
	return nil
}

func (b *daemonPluginBridge) InvalidateUI(ctx context.Context, pluginID string, p rpc.UIInvalidateParams) error {
	b.d.mu.Lock()
	b.d.broadcastLocked(proto.Message{
		Event:      "plugin.ui.invalidate",
		Plugin:     pluginID,
		PluginArgs: map[string]string{"slots": strings.Join(p.SlotIDs, ",")},
	})
	b.d.mu.Unlock()
	return nil
}

func (b *daemonPluginBridge) OpenPane(ctx context.Context, pluginID string, p rpc.UIOpenPaneParams) error {
	plug := p.Plugin
	if plug == "" {
		plug = pluginID
	}
	cp := proto.CreateParams{
		Split:       p.Session,
		Direction:   p.Direction,
		Plugin:      plug,
		PluginView:  p.View,
		PluginTitle: p.Title,
		PluginArgs:  p.Args,
	}
	_, err := b.d.create(cp)
	return err
}

func (b *daemonPluginBridge) NotifyUI(ctx context.Context, pluginID string, p rpc.UINotifyParams) error {
	b.d.notice(proto.Notice{
		Session: p.SessionID,
		Pane:    p.PaneID,
		Title:   p.Title,
		Body:    p.Body,
	})
	return nil
}

func (b *daemonPluginBridge) WebViewEmit(ctx context.Context, pluginID string, p rpc.WebViewEmitParams) error {
	raw, _ := json.Marshal(p.Data)
	b.d.mu.Lock()
	b.d.broadcastLocked(proto.Message{
		Event:      "plugin.webview.emit",
		Plugin:     pluginID,
		Topic:      p.Event,
		PluginArgs: map[string]string{"slotId": p.SlotID},
		PluginData: raw,
	})
	b.d.mu.Unlock()
	return nil
}

func encodeKeyName(k string) ([]byte, bool) {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "enter", "return":
		return []byte("\r"), true
	case "tab":
		return []byte("\t"), true
	case "space":
		return []byte(" "), true
	case "backspace":
		return []byte{0x7f}, true
	case "escape", "esc":
		return []byte{0x1b}, true
	case "up":
		return []byte("\x1b[A"), true
	case "down":
		return []byte("\x1b[B"), true
	case "right":
		return []byte("\x1b[C"), true
	case "left":
		return []byte("\x1b[D"), true
	}
	low := strings.ToLower(strings.TrimSpace(k))
	if rest, ok := strings.CutPrefix(low, "ctrl+"); ok && len(rest) == 1 && rest[0] >= 'a' && rest[0] <= 'z' {
		return []byte{rest[0] - 'a' + 1}, true
	}
	return nil, false
}
