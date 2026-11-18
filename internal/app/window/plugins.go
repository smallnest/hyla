package window

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/app/host"
	"hyla/internal/build"
	"hyla/internal/feishu"
	"hyla/internal/pets"
	_ "hyla/internal/plugins/builtins/magpie"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/plugins/uihost"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/marks"
	"hyla/internal/ui/menus"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/wash"
)

// RegisterPluginProtocol registers the single global hylaplugin:// custom scheme handler
// before MainWindow creation.
func RegisterPluginProtocol() error {
	return uihost.RegisterProtocol(uihost.NewWebBridge(manifest.DefaultStore(), nil, nil))
}

// preloadPluginBase applies Layer 1 plugin actions, agent providers, detect
// rules, and wash backgrounds before settings.Load() validates config values.
func preloadPluginBase() {
	snap, err := manifest.ApplyBase(manifest.DefaultStore())
	if err != nil {
		return
	}
	wash.ReloadPlugins(snap.Washes)
	var washIDs []string
	for _, w := range wash.PluginList() {
		washIDs = append(washIDs, w.ID)
	}
	settings.SetPluginBackgrounds(washIDs)
	// Feishu was a bundled plugin for one version: what its page kept of
	// the bot belongs to the configuration, which the window reads from
	// here. It runs after the plugin base, as the settings it merges are
	// read with the plugins' washes known.
	_ = feishu.MigratePluginConfig(manifest.DefaultStore())
}

func (a *App) initPlugins() {
	store := manifest.DefaultStore()
	if a.DaemonPluginStatuses == nil {
		a.DaemonPluginStatuses = map[string][]rpc.PluginStatus{}
	}
	coord := uihost.NewCoordinator(store, &appPluginBridge{a: a}, build.Version(), func() {
		a.InvalidatePluginUI()
	})
	coord.OnMessage = func(msg string) {
		a.ShowPluginMessage(msg)
	}
	coord.DaemonFn = a.pluginDaemonCaller
	coord.RemoteRef = a.pluginRemoteRef
	coord.PaneHostFn = func(sessionID, paneID string) string {
		if sessionID != "" {
			return a.pluginHostKey(a.hostOf(sessionID))
		}
		if paneID != "" {
			return a.pluginHostKey(a.hostOf(paneID))
		}
		return "local"
	}
	coord.Web.SetCallbacks(
		store,
		a.pluginPalette,
		nil,
		func(rawURL string) { _ = mygo.Shell.OpenExternal(rawURL) },
		a.main,
	)
	_ = uihost.RegisterProtocol(coord.Web)
	a.Plugins = coord
	_ = a.reloadPlugins(false)
}

func (a *App) InvalidatePluginUI() {
	if a.Win != nil {
		a.Win.Update(func() {
			if a.SetWin != nil {
				a.SetWin.Invalidate()
			}
		})
		return
	}
	if a.SetWin != nil {
		a.SetWin.Invalidate()
	}
}

func (a *App) ShowPluginMessage(msg string) {
	if msg == "" {
		return
	}
	a.main(func() {
		a.toast(msg)
		if a.SetWin != nil {
			a.Ss.Toast = msg
			a.SetWin.Invalidate()
		}
		if a.Win != nil {
			a.Win.Invalidate()
		}
	})
}

func (a *App) startPluginHooks() {
	if a.themeOff != nil {
		a.themeOff()
	}
	a.themeOff = mygo.Theme.OnUpdated(func() {
		a.Update(func() {
			a.broadcastPluginTheme()
		})
	})
}

func (a *App) shutdownPlugins() {
	if a.themeOff != nil {
		a.themeOff()
		a.themeOff = nil
	}
	if a.Plugins != nil {
		a.Plugins.Shutdown()
	}
}

func (a *App) pluginPalette() *theme.Palette {
	if a.pluginIsDark() {
		return &theme.Dark
	}
	return &theme.Light
}

func (a *App) pluginIsDark() bool {
	switch a.settingsIn.Appearance {
	case "dark":
		return true
	case "light":
		return false
	default:
		if a.Win == nil {
			return false
		}
		return mygo.Theme.IsDark()
	}
}

func (a *App) broadcastPluginTheme() {
	if a.Plugins == nil || a.Win == nil {
		return
	}
	dark := a.pluginIsDark()
	name := a.settingsIn.LightTheme
	if dark {
		name = a.settingsIn.DarkTheme
	}
	if name == "" {
		name = "hyla"
	}
	if a.Plugins.Web != nil {
		a.Plugins.Web.BroadcastThemeChange(dark, name)
	}
	if a.Plugins.AppMgr != nil {
		a.Plugins.AppMgr.NotifyTopic("theme.changed", map[string]any{
			"dark":      dark,
			"themeName": name,
		})
	}
}

// reloadPlugins re-discovers plugins, updates Layer 1 registries (marks, washes,
// themes, pets, actions), reloads [app] hosts, and optionally asks local hylad to reload.
func (a *App) reloadPlugins(notifyDaemon bool) error {
	if a.Plugins == nil {
		return nil
	}
	snap, err := a.Plugins.Reload()
	if err != nil {
		return err
	}
	if a.Plugins.Web != nil {
		enabledWeb := make(map[string]bool)
		for _, p := range snap.Plugins {
			if p.Entry.Enabled && !p.Entry.PendingInstall && p.Entry.HasPermission(manifest.PermUIWebView) {
				enabledWeb[p.Entry.ID] = true
			}
		}
		a.Plugins.Web.ReleaseDisabledPlugins(enabledWeb)
	}
	marks.LoadPluginMarks(snap.Marks)
	wash.ReloadPlugins(snap.Washes)
	var washIDs []string
	for _, w := range wash.PluginList() {
		washIDs = append(washIDs, w.ID)
	}
	settings.SetPluginBackgrounds(washIDs)
	_ = theme.ReloadPluginThemes(snap.Themes)
	pets.ReloadPlugins(snap.Pets)
	a.petsGen++

	if a.Win != nil {
		mygo.App.SetMenu(a.menu())
		a.applyGlobalShortcuts()
		if a.Pet.Win != nil {
			a.UpdateNow(a.petSetReload)
		}
		a.Win.Invalidate()
	}
	if a.SetWin != nil {
		a.SetWin.Invalidate()
	}
	if notifyDaemon {
		h := a.Local()
		if a.hostSupportsPlugins(h) && h.Client != nil {
			h.Client.Go("plugin.reload", nil, func(err error) {
				if err != nil {
					a.UpdateNow(func() { a.toast(err.Error()) })
					return
				}
				a.RefreshDaemonPluginStatuses(h)
			})
		}
	}
	return nil
}

func (a *App) hostSupportsPlugins(h *host.Host) bool {
	return h != nil && h.Client != nil && h.Conn == host.StateReady &&
		slices.Contains(h.Client.Welcome.Capabilities, proto.CapPluginsV1)
}

func (a *App) pluginHostKey(h *host.Host) string {
	if h == nil || h == a.Local() {
		return "local"
	}
	if h.Name != "" {
		return "remote:" + h.Name
	}
	return "remote:" + h.Addr
}

func (a *App) pluginHostByKey(hostKey string) *host.Host {
	if hostKey == "" || hostKey == "local" {
		return a.Local()
	}
	for _, h := range a.Machines[1:] {
		if a.pluginHostKey(h) == hostKey {
			return h
		}
	}
	return nil
}

func (a *App) pluginRemoteRef(hostKey string) bool {
	h := a.pluginHostByKey(hostKey)
	return h != nil && h.Remote()
}

func (a *App) pluginDaemonCaller(hostKey string) uihost.DaemonCaller {
	h := a.pluginHostByKey(hostKey)
	if !a.hostSupportsPlugins(h) {
		return nil
	}
	return &hostDaemonCaller{a: a, h: h, key: hostKey}
}

func (a *App) RefreshDaemonPluginStatuses(h *host.Host) {
	if !a.hostSupportsPlugins(h) {
		return
	}
	cl := h.Client
	key := a.pluginHostKey(h)
	go func() {
		var list []rpc.PluginStatus
		if err := cl.Call("plugin.list", nil, &list); err != nil {
			return
		}
		a.main(func() {
			if a.DaemonPluginStatuses == nil {
				a.DaemonPluginStatuses = map[string][]rpc.PluginStatus{}
			}
			a.DaemonPluginStatuses[key] = list
			if a.SetWin != nil {
				a.SetWin.Invalidate()
			}
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}()
}

type hostDaemonCaller struct {
	a   *App
	h   *host.Host
	key string
}

func (d *hostDaemonCaller) CallPlugin(pluginID, method string, params any, out any) error {
	if d.h == nil || d.h.Client == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "daemon client unavailable")
	}
	var rawParams json.RawMessage
	if params != nil {
		if b, ok := params.(json.RawMessage); ok {
			rawParams = b
		} else {
			b, err := json.Marshal(params)
			if err != nil {
				return err
			}
			rawParams = b
		}
	}
	var rawOut json.RawMessage
	if err := d.h.Client.Call("plugin.call", rpc.PluginCallParams{
		PluginID: pluginID,
		Method:   method,
		Params:   rawParams,
	}, &rawOut); err != nil {
		return err
	}
	if out != nil && len(rawOut) > 0 && string(rawOut) != "null" {
		if err := json.Unmarshal(rawOut, out); err != nil {
			return err
		}
	}
	return nil
}

func (d *hostDaemonCaller) PluginStatuses() []rpc.PluginStatus {
	if d.a == nil {
		return nil
	}
	return d.a.DaemonPluginStatuses[d.key]
}

// runPluginAction handles action IDs registered by enabled plugins.
func (a *App) runPluginAction(id string, args map[string]string) bool {
	if a.Plugins == nil {
		return false
	}
	snap := a.Plugins.Snapshot()
	if snap == nil {
		return false
	}
	meta, ok := snap.ActionTargets[id]
	if !ok {
		return false
	}

	s := a.session(a.Selected)
	var sessID, paneID, projectID string
	var h *host.Host = a.Local()
	if s != nil {
		sessID = s.ID
		projectID = s.Project
		h = a.hostOf(s.ID)
		paneID = a.focusOf(s)
	}

	// Open-pane actions open the declared plugin pane directly without an RPC round-trip.
	if meta.Action.Target == "open_pane" && meta.Action.View != "" {
		a.openPluginPane(s, meta.PluginID, meta.Action.View, meta.Action.Title, "", args)
		return true
	}

	hostKey := a.pluginHostKey(h)
	req := rpc.ActionInvokeParams{
		ActionID:  meta.Action.ID,
		Args:      args,
		SessionID: sessID,
		PaneID:    paneID,
		ProjectID: projectID,
		Host:      hostKey,
	}
	go func() {
		res, err := a.Plugins.CallAction(hostKey, meta, req)
		a.main(func() {
			if err != nil {
				a.toast(err.Error())
				if a.Win != nil {
					a.Win.Invalidate()
				}
				return
			}
			if res != nil && res.Message != "" {
				a.toast(res.Message)
			}
			if a.Plugins != nil {
				a.Plugins.Invalidate(meta.PluginID, nil)
			}
			if a.Win != nil {
				a.Win.Invalidate()
			}
		})
	}()
	return true
}

func (a *App) openPluginPane(s *proto.Session, pluginID, slotID, title, direction string, args map[string]string) {
	if s == nil {
		h := a.Local()
		if h.Client == nil {
			return
		}
		a.create(h, proto.CreateParams{
			Plugin:      pluginID,
			PluginView:  slotID,
			PluginTitle: title,
			PluginArgs:  args,
		}, "")
		return
	}
	h := a.hostOf(s.ID)
	if a.remoteControl(h) == false && h.Remote() {
		return
	}
	if direction == "" {
		direction = "right"
	}
	at := a.focusOf(s)
	a.split(s, proto.CreateParams{
		Split:       at,
		Direction:   direction,
		Plugin:      pluginID,
		PluginView:  slotID,
		PluginTitle: title,
		PluginArgs:  args,
	})
}

// runPluginShortcuts evaluates shortcuts of enabled plugin actions not already bound to the menu bar.
func (a *App) runPluginShortcuts(c *ui.Context) {
	if a.Plugins == nil {
		return
	}
	s := a.session(a.Selected)
	var p *proto.Pane
	if s != nil {
		p = s.PaneByID(a.focusOf(s))
	}
	for _, act := range actions.List() {
		if act.PluginID == "" {
			continue
		}
		// Menubar items claim their accelerators at OS level when Menu != "" and Menu != "Context".
		// Still match non-menubar items (Menu == "") via UI shortcut polling.
		if act.Menu != "" {
			continue
		}
		if !uihost.MatchesWhen(act.When, s, p) {
			continue
		}
		accel := actions.Keys(act, a.settingsIn.Shortcuts)
		mods, k, ok := menus.ParseShortcut(accel)
		if !ok {
			continue
		}
		if c.Shortcut(mods, k) {
			a.run(act.ID, nil)
		}
	}
}

func (a *App) callMainSync(ctx context.Context, fn func()) error {
	win := a.Win
	if win == nil {
		win = a.SetWin
	}
	if win == nil {
		fn()
		return nil
	}
	done := make(chan struct{})
	win.Update(func() {
		defer close(done)
		fn()
	})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// appPluginBridge implements daemonhost.HostBridge for [app]-hosted plugin processes.
type appPluginBridge struct {
	a *App
}

func (b *appPluginBridge) GetState(ctx context.Context, pluginID string, p rpc.HostGetStateParams) (*rpc.HostGetStateResult, error) {
	res := &rpc.HostGetStateResult{}
	if err := b.a.callMainSync(ctx, func() {
		st := b.a.St
		st.Projects = slices.Clone(b.a.St.Projects)
		st.Sessions = slices.Clone(b.a.St.Sessions)
		var focusedPane string
		if s := b.a.session(b.a.Selected); s != nil {
			focusedPane = b.a.focusOf(s)
		}
		res = &rpc.HostGetStateResult{
			State:          st,
			ActiveSession:  b.a.Selected,
			FocusedPane:    focusedPane,
			Dark:           b.a.pluginIsDark(),
			Appearance:     b.a.settingsIn.Appearance,
			ControlAllowed: b.a.remoteControl(b.a.Local()),
		}
	}); err != nil {
		return nil, err
	}
	return res, nil
}

func (b *appPluginBridge) ReadScreen(ctx context.Context, pluginID string, p rpc.PaneReadScreenParams) (*rpc.PaneReadScreenResult, error) {
	h := b.a.Local()
	if h.Client == nil {
		return nil, rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "local daemon not connected")
	}
	var out proto.ReadResult
	if err := h.Client.Call("read", proto.ReadParams{ID: p.Pane, Scrollback: p.Scrollback, Lines: p.Lines}, &out); err != nil {
		return nil, err
	}
	return &rpc.PaneReadScreenResult{Text: out.Text}, nil
}

func (b *appPluginBridge) GetEnv(ctx context.Context, pluginID string, p rpc.PaneGetEnvParams) (*rpc.PaneGetEnvResult, error) {
	res := &rpc.PaneGetEnvResult{
		Env: os.Environ(),
	}
	if err := b.a.callMainSync(ctx, func() {
		for i := range b.a.St.Sessions {
			for _, pn := range b.a.St.Sessions[i].Panes {
				if pn.ID == p.Pane {
					res.Dir = pn.WorkDir()
					res.Process = pn.Process
					res.Agent = pn.Agent
					res.AgentSessionID = pn.AgentSessionID
					return
				}
			}
		}
	}); err != nil {
		return nil, err
	}
	return res, nil
}

func (b *appPluginBridge) CreateSession(ctx context.Context, pluginID string, p proto.CreateParams) (*rpc.SessionCreateResult, error) {
	h := b.a.Local()
	if h.Client == nil {
		return nil, rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "local daemon not connected")
	}
	var s proto.Session
	if err := h.Client.Call("create", p, &s); err != nil {
		return nil, err
	}
	_ = b.a.callMainSync(ctx, func() {
		b.a.addPending(h, s)
		b.a.selectSession(s.ID)
	})
	return &rpc.SessionCreateResult{SessionID: s.ID, PaneID: s.Focus}, nil
}

func (b *appPluginBridge) SendInput(ctx context.Context, pluginID string, p rpc.PaneSendInputParams) error {
	h := b.a.Local()
	if h.Client == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "local daemon not connected")
	}
	return h.Client.Call("send", proto.SendParams{ID: p.Pane, Data: p.Data}, nil)
}

func (b *appPluginBridge) SendKey(ctx context.Context, pluginID string, p rpc.PaneSendKeyParams) error {
	h := b.a.Local()
	if h.Client == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "local daemon not connected")
	}
	data, ok := encodePluginKeyName(p.Key)
	if !ok {
		return rpc.NewError(rpc.ErrCodeInvalidParams, fmt.Sprintf("unsupported key %q", p.Key))
	}
	return h.Client.Call("send", proto.SendParams{ID: p.Pane, Data: string(data)}, nil)
}

func (b *appPluginBridge) ReportStatus(ctx context.Context, pluginID string, p rpc.PaneReportStatusParams) error {
	h := b.a.Local()
	if h.Client == nil {
		return rpc.NewError(rpc.ErrCodeCapabilityUnsupported, "local daemon not connected")
	}
	st := strings.ToLower(strings.TrimSpace(p.Status))
	switch st {
	case "waiting":
		st = "blocked"
	case "failed":
		st = "error"
	case "running", "busy":
		st = "working"
	case "idle", "working", "done", "blocked", "error", "clear":
	default:
		return rpc.NewError(rpc.ErrCodeInvalidParams, fmt.Sprintf("invalid status %q", p.Status))
	}
	parts := []string{"state=" + st}
	if p.Progress >= 0 {
		parts = append(parts, "progress="+strconv.Itoa(p.Progress))
	}
	if p.Kind != "" && isSafeOSCValue(p.Kind) {
		parts = append(parts, "kind="+p.Kind)
	}
	return h.Client.Call("pane.report", proto.ReportParams{ID: p.Pane, Report: strings.Join(parts, ":")}, nil)
}

func (b *appPluginBridge) FocusPane(ctx context.Context, pluginID string, p rpc.PaneFocusParams) error {
	if !b.a.remoteControl(b.a.Local()) {
		return rpc.NewError(rpc.ErrCodeRemoteControlDisabled, "remote control is disabled")
	}
	return b.a.callMainSync(ctx, func() {
		if p.Session != "" {
			b.a.selectSession(p.Session)
		}
		if p.Pane != "" {
			if s := b.a.sessionOfPane(p.Pane); s != nil {
				b.a.selectSession(s.ID)
				b.a.setFocus(s, p.Pane)
			}
		}
		if b.a.Win != nil {
			b.a.ShowWindow()
		}
	})
}

func (b *appPluginBridge) InvalidateUI(ctx context.Context, pluginID string, p rpc.UIInvalidateParams) error {
	if b.a.Plugins != nil {
		b.a.Plugins.Invalidate(pluginID, p.SlotIDs)
	}
	return nil
}

func (b *appPluginBridge) OpenPane(ctx context.Context, pluginID string, p rpc.UIOpenPaneParams) error {
	plug := p.Plugin
	if plug == "" {
		plug = pluginID
	}
	var openErr error
	if err := b.a.callMainSync(ctx, func() {
		var s *proto.Session
		if p.Session != "" {
			s = b.a.session(p.Session)
		} else {
			s = b.a.session(b.a.Selected)
		}
		if s != nil && b.a.hostOf(s.ID).Remote() && !b.a.remoteControl(b.a.hostOf(s.ID)) {
			openErr = rpc.NewError(rpc.ErrCodeRemoteControlDisabled, "remote control is disabled for remote host")
			return
		}
		b.a.openPluginPane(s, plug, p.View, p.Title, p.Direction, p.Args)
	}); err != nil {
		return err
	}
	return openErr
}

func (b *appPluginBridge) NotifyUI(ctx context.Context, pluginID string, p rpc.UINotifyParams) error {
	return b.a.callMainSync(ctx, func() {
		title := p.Title
		if title == "" {
			title = pluginID
		}
		if p.SessionID != "" {
			b.a.showNotification(p.SessionID, title, p.Body)
			return
		}
		msg := title
		if p.Body != "" {
			msg += ": " + p.Body
		}
		b.a.toast(msg)
		if b.a.SetWin != nil {
			b.a.Ss.Toast = msg
			b.a.SetWin.Invalidate()
		}
	})
}

func (b *appPluginBridge) WebViewEmit(ctx context.Context, pluginID string, p rpc.WebViewEmitParams) error {
	if b.a.Plugins == nil || b.a.Plugins.Web == nil {
		return nil
	}
	raw, _ := json.Marshal(p.Data)
	b.a.Plugins.Web.Emit(pluginID, p.SlotID, p.Event, raw)
	return nil
}

func isSafeOSCValue(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("_.,+/=-", r):
		default:
			return false
		}
	}
	return true
}

func encodePluginKeyName(k string) ([]byte, bool) {
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
