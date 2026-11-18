// Package feishuplugin provides the official bundled hyla.feishu plugin,
// exposing Feishu configuration, migration compatibility with legacy feishu-*
// settings & feishu.json, keep_alive lifecycle integration, and Declarative UI
// status rendering in Hyla's Info view.
package feishuplugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
	"hyla/internal/settings"
)

// PluginID is the canonical reverse-domain identifier of the bundled Feishu plugin.
const PluginID = "hyla.feishu"

// RuntimeStatus describes the live connection state of the Feishu channel driver.
type RuntimeStatus struct {
	Configured bool     `json:"configured"`
	Secret     bool     `json:"secret"`
	Connected  bool     `json:"connected"`
	App        string   `json:"app,omitempty"`
	Error      string   `json:"error,omitempty"`
	Allow      []string `json:"allow,omitempty"`
	Agent      string   `json:"agent,omitempty"`
	Run        string   `json:"run,omitempty"`
}

// RuntimeHooks bridges the bundled hyla.feishu plugin RPC server to hylad's
// active Feishu ChannelDriver without creating package import cycles.
type RuntimeHooks struct {
	Status   func() RuntimeStatus
	Restart  func(force bool) error
	Stop     func()
	BaseDir  func() string
	IsActive func() bool
}

var (
	hooksMu sync.RWMutex
	hooks   RuntimeHooks
)

// SetRuntime installs host runtime hooks for hyla.feishu.
func SetRuntime(h RuntimeHooks) {
	hooksMu.Lock()
	hooks = h
	hooksMu.Unlock()
}

func getRuntime() RuntimeHooks {
	hooksMu.RLock()
	h := hooks
	hooksMu.RUnlock()
	return h
}

func init() {
	Register()
}

// Register registers the bundled hyla.feishu plugin manifest and daemon runner.
func Register() {
	m := &manifest.Manifest{
		APIVersion:  manifest.APIVersion,
		ID:          PluginID,
		Name:        "Feishu",
		Version:     "1.0.0",
		Description: "Connects Feishu bots to Hyla sessions via WebSocket and exposes live connection status.",
		Author:      "Hyla",
		Permissions: manifest.PermissionsSpec{
			Scopes: []string{
				manifest.PermUISlot,
				manifest.PermSessionRead,
				manifest.PermPaneWrite,
				manifest.PermChannelProvider,
				manifest.PermSecrets,
				manifest.PermNotifications,
				manifest.PermNetwork,
			},
		},
		Daemon: &manifest.HostProcessSpec{
			Command:   []string{"builtin:hyla.feishu"},
			Transport: "stdio",
			AutoStart: false,
			KeepAlive: true,
		},
		Actions: []manifest.ActionSpec{
			{
				ID:      "reconnect",
				Title:   "Reconnect Feishu Bot",
				Palette: true,
				Target:  "daemon",
			},
		},
		UI: manifest.UISpec{
			Slots: []manifest.SlotSpec{
				{
					ID:    "status",
					Mount: "info.card",
					Title: "Feishu",
					Mode:  "declarative",
					Host:  "daemon",
					Order: 20,
				},
			},
		},
		Settings: []manifest.SettingSpec{
			{
				Key:         "app",
				Title:       "App ID",
				Description: "Feishu App ID (cli_...) synced with feishu-app.",
				Type:        "string",
				Placeholder: "cli_...",
			},
			{
				Key:         "appSecret",
				Title:       "App Secret",
				Description: "Feishu App Secret stored in plugin secrets or feishu.json.",
				Type:        "secret",
			},
			{
				Key:         "allow",
				Title:       "Allowed Users",
				Description: "Comma-separated open_id, user_id, email, or *.",
				Type:        "string",
				Placeholder: "*",
			},
			{
				Key:         "agent",
				Title:       "Default Agent",
				Description: "Agent used for new chats (e.g. default, claude, codex).",
				Type:        "string",
				Default:     "default",
			},
			{
				Key:     "run",
				Title:   "Prompt Mode",
				Type:    "select",
				Default: settings.FeishuRunCommand,
				Options: []string{settings.FeishuRunCommand, settings.FeishuRunSession},
			},
		},
	}
	_ = manifest.RegisterBuiltin(manifest.BuiltinPlugin{
		Manifest:         m,
		EnabledByDefault: true,
		AutoGrant:        true,
	})
	daemonhost.RegisterBuiltinRunner(PluginID, daemonhost.HostKindDaemon, runServer)
	daemonhost.RegisterBuiltinShouldStart(PluginID, daemonhost.HostKindDaemon, ShouldAutoStart)
}

// Resolved holds effective Feishu settings & secrets combined from legacy config
// (feishu-*, feishu.json) and plugin storage (plugin-config/hyla.feishu.json).
type Resolved struct {
	Settings  settings.Feishu
	YoloAll   bool
	Secret    settings.FeishuSecret
	HasSecret bool
	Digest    string
}

// Resolve loads effective Feishu configuration and secrets from st (or the active
// Hyla directory), seamlessly migrating legacy feishu-* config into plugin-config
// and honoring plugin config/secret overrides.
func Resolve(st *manifest.Store) Resolved {
	baseDir := proto.Dir()
	if st != nil && st.BaseDir != "" {
		baseDir = st.BaseDir
	}
	isPrimaryDir := baseDir == proto.Dir()

	var cfg settings.Feishu
	yoloAll := true
	if isPrimaryDir {
		s := settings.Load()
		yoloAll = s.FeishuYolo
		if s.Feishu != nil {
			cfg = *s.Feishu
			cfg.Allow = slices.Clone(s.Feishu.Allow)
			if s.Feishu.Agents != nil {
				cfg.Agents = map[string]string{}
				for k, v := range s.Feishu.Agents {
					cfg.Agents[k] = v
				}
			}
			if s.Feishu.Yolo != nil {
				cfg.Yolo = map[string]bool{}
				for k, v := range s.Feishu.Yolo {
					cfg.Yolo[k] = v
				}
			}
			cfg.Host = slices.Clone(s.Feishu.Host)
		}
	}

	// Read non-secret plugin config.
	var pcfg map[string]any
	if st != nil {
		if m, err := st.LoadConfig(PluginID); err == nil {
			pcfg = m
		}
	}

	// If legacy Feishu is configured and plugin-config lacks an app entry,
	// lazily mirror legacy non-secret settings into plugin-config/hyla.feishu.json.
	if cfg.App != "" && stringVal(pcfg["app"]) == "" && st != nil {
		_ = saveMigratedConfig(st, cfg)
	}

	// Apply plugin-config values if set (allows Settings > Plugins edits to take effect).
	if pcfg != nil {
		if v := strings.TrimSpace(stringVal(pcfg["app"])); v != "" {
			cfg.App = v
		}
		if allowRaw, exists := pcfg["allow"]; exists {
			cfg.Allow = parseAllowValue(allowRaw)
		}
		if v := strings.TrimSpace(stringVal(pcfg["agent"])); v != "" {
			cfg.Agent = v
		}
		if v := strings.TrimSpace(stringVal(pcfg["run"])); v == settings.FeishuRunCommand || v == settings.FeishuRunSession {
			cfg.Run = v
		}
		if v := strings.TrimSpace(stringVal(pcfg["dir"])); v != "" {
			cfg.Dir = v
		}
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = int64(settings.FeishuDefaultTimeout)
	}
	if cfg.Stream <= 0 {
		cfg.Stream = int64(settings.FeishuDefaultStream)
	}
	if cfg.Run == "" {
		cfg.Run = settings.FeishuRunCommand
	}
	if cfg.Agent == "" {
		cfg.Agent = "default"
	}

	// Resolve secret only if configured or feishu.json exists cleanly.
	var sec settings.FeishuSecret
	var hasSecret bool
	secPath := filepath.Join(baseDir, "feishu.json")
	if b, err := os.ReadFile(secPath); err == nil {
		if jerr := json.Unmarshal(b, &sec); jerr == nil && sec.AppSecret != "" {
			hasSecret = true
		}
	}
	if (!hasSecret || sec.AppSecret == "") && cfg.App != "" && st != nil {
		if val, ok, err := st.GetSecret(PluginID, "appSecret"); err == nil && ok && strings.TrimSpace(val) != "" {
			sec.AppID = cfg.App
			sec.AppSecret = strings.TrimSpace(val)
			hasSecret = true
		}
	}
	if sec.AppID == "" && cfg.App != "" {
		sec.AppID = cfg.App
	}

	h := sha256.New()
	enc, _ := json.Marshal(struct {
		Settings  settings.Feishu       `json:"settings"`
		YoloAll   bool                  `json:"yoloAll"`
		Secret    settings.FeishuSecret `json:"secret"`
		HasSecret bool                  `json:"hasSecret"`
	}{cfg, yoloAll, sec, hasSecret})
	_, _ = h.Write(enc)

	return Resolved{
		Settings:  cfg,
		YoloAll:   yoloAll,
		Secret:    sec,
		HasSecret: hasSecret,
		Digest:    hex.EncodeToString(h.Sum(nil)),
	}
}

// SyncToLegacy propagates non-empty plugin config / secrets from st into legacy
// config + feishu.json when configured via Settings > Plugins, keeping
// `hyla feishu status` and `Settings > Feishu` synchronized.
func SyncToLegacy(st *manifest.Store) error {
	if st == nil {
		return nil
	}
	baseDir := st.BaseDir
	if baseDir == "" {
		baseDir = proto.Dir()
	}
	if baseDir != proto.Dir() {
		return nil
	}
	pcfg, err := st.LoadConfig(PluginID)
	if err != nil {
		return err
	}
	app, hasAppKey := pcfg["app"]
	appStr := strings.TrimSpace(stringVal(app))
	if appStr != "" || hasAppKey {
		cur := settings.Load()
		var f settings.Feishu
		if cur.Feishu != nil {
			f = *cur.Feishu
		}
		f.App = appStr
		if allowRaw, ok := pcfg["allow"]; ok {
			f.Allow = parseAllowValue(allowRaw)
		}
		if ag := strings.TrimSpace(stringVal(pcfg["agent"])); ag != "" {
			f.Agent = ag
		}
		if r := strings.TrimSpace(stringVal(pcfg["run"])); r == settings.FeishuRunCommand || r == settings.FeishuRunSession {
			f.Run = r
		}
		if err := settings.SetFeishu(f); err != nil {
			return err
		}
		if appStr != "" {
			if secVal, ok, err := st.GetSecret(PluginID, "appSecret"); err == nil && ok && strings.TrimSpace(secVal) != "" {
				existing, _ := settings.LoadFeishuSecret()
				existing.AppID = appStr
				existing.AppSecret = strings.TrimSpace(secVal)
				if err := settings.SaveFeishuSecret(existing); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ShouldAutoStart returns true only when Feishu is configured with both App ID
// and App Secret, preventing hyla.feishu from holding hylad alive when unused.
func ShouldAutoStart(st *manifest.Store) bool {
	res := Resolve(st)
	return res.Settings.App != "" && res.HasSecret && res.Secret.AppSecret != ""
}

func saveMigratedConfig(st *manifest.Store, cfg settings.Feishu) error {
	m, err := st.LoadConfig(PluginID)
	if err != nil {
		m = map[string]any{}
	}
	if m == nil {
		m = map[string]any{}
	}
	m["app"] = cfg.App
	if len(cfg.Allow) > 0 {
		m["allow"] = strings.Join(cfg.Allow, ", ")
	}
	if cfg.Agent != "" {
		m["agent"] = cfg.Agent
	}
	if cfg.Run != "" {
		m["run"] = cfg.Run
	}
	if cfg.Dir != "" {
		m["dir"] = cfg.Dir
	}
	return st.SaveConfig(PluginID, m)
}

func stringVal(v any) string {
	s, _ := v.(string)
	return s
}

func parseAllowValue(v any) []string {
	switch x := v.(type) {
	case string:
		var out []string
		for _, part := range strings.Split(x, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	case []string:
		return slices.Clone(x)
	case []any:
		var out []string
		for _, item := range x {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func currentStatus(storeDir string) RuntimeStatus {
	rt := getRuntime()
	if rt.Status != nil {
		return rt.Status()
	}
	var st *manifest.Store
	if storeDir != "" {
		st = &manifest.Store{BaseDir: storeDir}
	} else {
		st = manifest.DefaultStore()
	}
	res := Resolve(st)
	out := RuntimeStatus{
		Configured: res.Settings.App != "",
		Secret:     res.HasSecret,
		Connected:  false,
		App:        res.Settings.App,
		Allow:      res.Settings.Allow,
		Agent:      res.Settings.Agent,
		Run:        res.Settings.Run,
	}
	if out.Configured && !out.Connected {
		out.Error = "not connected"
	}
	return out
}

func runServer(ctx context.Context, peer *rpc.Peer) {
	var (
		mu       sync.Mutex
		storeDir string
	)

	peer.Handle("initialize", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.InitializeParams
		_ = json.Unmarshal(raw, &p)
		mu.Lock()
		if rt := getRuntime(); rt.BaseDir != nil {
			storeDir = rt.BaseDir()
		}
		if storeDir == "" && p.DataDir != "" {
			storeDir = filepath.Dir(filepath.Dir(p.DataDir))
		}
		mu.Unlock()
		if rt := getRuntime(); rt.Restart != nil {
			go func() { _ = rt.Restart(false) }()
		}
		return rpc.InitializeResult{
			Name:         "Feishu",
			Version:      "1.0.0",
			Capabilities: []string{"ui.declarative"},
		}, nil
	})
	peer.Handle("initialized", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		return nil, nil
	})
	peer.Handle("shutdown", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		return struct{}{}, nil
	})
	peer.Handle("exit", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		_ = peer.Close()
		return nil, nil
	})

	peer.Handle("ui.render", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.UIRenderParams
		_ = json.Unmarshal(raw, &p)
		if p.SlotID != "status" && p.SlotID != "" {
			return rpc.UIRenderResult{}, nil
		}
		mu.Lock()
		dir := storeDir
		mu.Unlock()
		st := currentStatus(dir)
		return buildStatusCard(st), nil
	})

	peer.Handle("ui.event", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.UIEventParams
		_ = json.Unmarshal(raw, &p)
		if p.ComponentID == "reconnect" || p.EventType == "click" {
			if rt := getRuntime(); rt.Restart != nil {
				_ = rt.Restart(true)
			}
			mu.Lock()
			dir := storeDir
			mu.Unlock()
			res := buildStatusCard(currentStatus(dir))
			return rpc.UIEventResult{
				Handled:    true,
				Invalidate: true,
				Root:       res.Root,
			}, nil
		}
		return rpc.UIEventResult{}, nil
	})

	peer.Handle("action.invoke", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.ActionInvokeParams
		_ = json.Unmarshal(raw, &p)
		if rt := getRuntime(); rt.Restart != nil {
			if err := rt.Restart(true); err != nil {
				return rpc.ActionInvokeResult{OK: false, Message: err.Error()}, nil
			}
		}
		_ = peer.Call(ctx, "ui.invalidate", rpc.UIInvalidateParams{SlotIDs: []string{"status"}}, nil)
		return rpc.ActionInvokeResult{OK: true, Message: "Feishu bot reconnecting"}, nil
	})

	peer.Handle("event.notify", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		return nil, nil
	})

	defer func() {
		mu.Lock()
		dir := storeDir
		mu.Unlock()
		var st *manifest.Store
		if dir != "" {
			st = &manifest.Store{BaseDir: dir}
		} else {
			st = manifest.DefaultStore()
		}
		if !st.IsEnabled(PluginID) {
			if rt := getRuntime(); rt.Stop != nil {
				rt.Stop()
			}
		}
	}()

	_ = peer.Serve(ctx)
}

func buildStatusCard(st RuntimeStatus) rpc.UIRenderResult {
	if !st.Configured || st.App == "" {
		return rpc.UIRenderResult{}
	}
	aside := "Off"
	tone := "attention"
	badgeText := "Disconnected"
	if st.Connected {
		aside = "On"
		tone = "done"
		badgeText = "Connected"
	}
	who := "Allow: setup user only"
	if len(st.Allow) > 0 {
		who = "Allow: " + strings.Join(st.Allow, ", ")
	}
	agentLabel := st.Agent
	if agentLabel == "" {
		agentLabel = "default"
	}
	runLabel := st.Run
	if runLabel == "" {
		runLabel = settings.FeishuRunCommand
	}

	children := []*rpc.UINode{
		{
			Type:  "row",
			Key:   "status-row",
			Style: &rpc.UIStyle{Justify: "between", Align: "center", Gap: 8},
			Children: []*rpc.UINode{
				{
					Type:  "text",
					Key:   "app-id",
					Props: map[string]any{"text": st.App},
					Style: &rpc.UIStyle{FontSize: 12, Tabular: true},
				},
				{
					Type:  "badge",
					Key:   "conn-badge",
					Props: map[string]any{"text": badgeText, "tone": tone},
				},
			},
		},
		{
			Type:  "text",
			Key:   "allow",
			Props: map[string]any{"text": who, "tone": "muted"},
			Style: &rpc.UIStyle{FontSize: 12},
		},
		{
			Type:  "text",
			Key:   "mode",
			Props: map[string]any{"text": runLabel + ", " + agentLabel, "tone": "faint"},
			Style: &rpc.UIStyle{FontSize: 11.5, Tabular: true},
		},
	}
	if !st.Connected && st.Error != "" {
		children = append(children, &rpc.UINode{
			Type:  "text",
			Key:   "err",
			Props: map[string]any{"text": st.Error, "tone": "faint"},
			Style: &rpc.UIStyle{FontSize: 11.5},
		})
	}
	children = append(children, &rpc.UINode{
		Type:  "row",
		Key:   "actions",
		Style: &rpc.UIStyle{Justify: "end"},
		Children: []*rpc.UINode{
			{
				ID:      "reconnect",
				Type:    "button",
				Key:     "reconnect-btn",
				Props:   map[string]any{"label": "Reconnect", "variant": "ghost"},
				OnClick: "reconnect",
			},
		},
	})

	return rpc.UIRenderResult{
		Title:     "Feishu",
		RefreshMS: 5000,
		Root: &rpc.UINode{
			Type:  "infoCard",
			Key:   "feishu-card",
			Props: map[string]any{"title": "Feishu", "aside": aside, "icon": "link"},
			Style: &rpc.UIStyle{Gap: 6},
			Children: []*rpc.UINode{
				{
					Type:     "column",
					Key:      "body",
					Style:    &rpc.UIStyle{Gap: 5},
					Children: children,
				},
			},
		},
	}
}
