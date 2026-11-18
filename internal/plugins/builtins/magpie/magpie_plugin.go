// Package magpieplugin provides the official bundled hyla.magpie plugin
// built atop internal/app/magpie using Hyla's plugin manifest, RPC, and Declarative UI contracts.
package magpieplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"hyla/internal/app/magpie"
	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
)

const PluginID = "hyla.magpie"

func init() {
	Register()
}

// Register installs the bundled hyla.magpie manifest and RPC runner.
func Register() {
	m := &manifest.Manifest{
		APIVersion:  manifest.APIVersion,
		ID:          PluginID,
		Name:        "Magpie",
		Version:     "1.0.0",
		Description: "Displays Magpie coding agent gateway usage, cost, tokens, and plan limits.",
		Author:      "Hyla",
		Permissions: manifest.PermissionsSpec{
			Scopes: []string{manifest.PermUISlot, manifest.PermSessionRead},
		},
		Daemon: &manifest.HostProcessSpec{
			Command:   []string{"builtin:hyla.magpie"},
			Transport: "stdio",
			AutoStart: false,
			KeepAlive: false,
		},
		Actions: []manifest.ActionSpec{
			{
				ID:      "refresh",
				Title:   "Refresh Magpie Usage",
				Palette: true,
				Target:  "daemon",
			},
		},
		UI: manifest.UISpec{
			Slots: []manifest.SlotSpec{
				{
					ID:    "magpie",
					Mount: "info.card",
					Title: "Magpie",
					Mode:  "declarative",
					Host:  "daemon",
					Order: 10,
				},
			},
		},
		Settings: []manifest.SettingSpec{
			{
				Key:         "url",
				Title:       "Gateway URL",
				Description: "Custom Magpie base URL (defaults to MAGPIE_URL or http://127.0.0.1:3425).",
				Type:        "string",
				Placeholder: "http://127.0.0.1:3425",
			},
		},
	}
	_ = manifest.RegisterBuiltin(manifest.BuiltinPlugin{
		Manifest:         m,
		EnabledByDefault: true,
		AutoGrant:        true,
	})
	daemonhost.RegisterBuiltinRunner(PluginID, daemonhost.HostKindDaemon, runServer)
}

type state struct {
	mu       sync.Mutex
	url      string
	lastRead time.Time
	lastSess string
	usage    *magpie.Usage
	lastErr  string
	fetchFn  func(base, sess string) (*magpie.Usage, error)
}

func runServer(ctx context.Context, peer *rpc.Peer) {
	st := &state{
		fetchFn: magpie.Fetch,
	}

	peer.Handle("initialize", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.InitializeParams
		_ = json.Unmarshal(raw, &p)
		if u, ok := p.Settings["url"].(string); ok && strings.TrimSpace(u) != "" {
			st.mu.Lock()
			st.url = strings.TrimRight(strings.TrimSpace(u), "/")
			st.mu.Unlock()
		}
		return rpc.InitializeResult{
			Name:          "Magpie",
			Version:       "1.0.0",
			Capabilities:  []string{"ui.declarative"},
			Subscriptions: []string{"theme.changed"},
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
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		// Refresh live settings in case user updated URL via Settings › Plugins.
		var cfg rpc.PluginGetSettingsResult
		if err := peer.Call(ctx, "plugin.getSettings", rpc.PluginGetSettingsParams{}, &cfg); err == nil {
			if u, ok := cfg.Settings["url"].(string); ok {
				st.mu.Lock()
				st.url = strings.TrimRight(strings.TrimSpace(u), "/")
				st.mu.Unlock()
			}
		}
		agentSessID := resolveAgentSessionID(ctx, peer, p.SessionID, p.PaneID)
		u, errStr := st.fetch(agentSessID, false)
		return buildRenderResult(u, errStr), nil
	})
	peer.Handle("ui.event", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.UIEventParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if p.ComponentID == "refresh" || p.EventType == "click" {
			agentSessID := resolveAgentSessionID(ctx, peer, p.SessionID, p.PaneID)
			u, errStr := st.fetch(agentSessID, true)
			res := buildRenderResult(u, errStr)
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
		st.mu.Lock()
		st.lastRead = time.Time{}
		st.mu.Unlock()
		_ = peer.Call(ctx, "ui.invalidate", rpc.UIInvalidateParams{SlotIDs: []string{"magpie"}}, nil)
		return rpc.ActionInvokeResult{OK: true}, nil
	})
	peer.Handle("event.notify", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		return nil, nil
	})

	_ = peer.Serve(ctx)
}

func (st *state) fetch(sessionID string, force bool) (*magpie.Usage, string) {
	st.mu.Lock()
	base := st.url
	if base == "" {
		base = magpie.URL()
	}
	if !force && st.lastSess == sessionID && time.Since(st.lastRead) < 10*time.Second && (st.usage != nil || st.lastErr != "") {
		u, errStr := st.usage, st.lastErr
		st.mu.Unlock()
		return u, errStr
	}
	fn := st.fetchFn
	st.mu.Unlock()

	u, err := fn(base, sessionID)
	st.mu.Lock()
	st.lastRead = time.Now()
	st.lastSess = sessionID
	if err != nil {
		st.lastErr = err.Error()
		st.usage = nil
	} else {
		st.lastErr = ""
		st.usage = u
	}
	usage, errStr := st.usage, st.lastErr
	st.mu.Unlock()
	return usage, errStr
}

func buildRenderResult(u *magpie.Usage, errStr string) rpc.UIRenderResult {
	var children []*rpc.UINode
	if u == nil {
		msg := "Magpie is not running"
		if errStr != "" && !strings.Contains(errStr, "connection refused") {
			msg = errStr
		}
		children = append(children, &rpc.UINode{
			Type:  "text",
			Key:   "offline",
			Props: map[string]any{"text": msg, "tone": "muted"},
		})
	} else {
		children = append(children,
			kvRow("calls", "Calls today", fmt.Sprintf("%d", u.Today.Calls)),
			kvRow("cost", "Cost", fmt.Sprintf("$%.4f", u.Today.Cost)),
			kvRow("tokens", "Tokens", formatTokens(u.Today.Count())),
		)
		if u.Session > 0 {
			children = append(children, kvRow("session", "This conversation", fmt.Sprintf("%d ($%.4f)", u.Session, u.SessionCost)))
		}
		if u.Latest.Status > 0 {
			children = append(children, kvRow("latest", "Latest call", fmt.Sprintf("%d · %dms", u.Latest.Status, u.Latest.Ms)))
		}
		for i, w := range u.Windows {
			if i >= 4 {
				break
			}
			children = append(children, &rpc.UINode{
				Type: "column",
				Key:  fmt.Sprintf("win-%d", i),
				Style: &rpc.UIStyle{
					Gap: 3,
				},
				Children: []*rpc.UINode{
					kvRow(fmt.Sprintf("wlbl-%d", i), fmt.Sprintf("%s · %s", w.Provider, w.Name), fmt.Sprintf("%.0f%%", w.Left)),
					{
						Type:  "progress",
						Key:   fmt.Sprintf("wbar-%d", i),
						Props: map[string]any{"value": w.Left / 100.0},
					},
				},
			})
		}
	}
	children = append(children, &rpc.UINode{
		Type: "row",
		Key:  "actions",
		Style: &rpc.UIStyle{
			Justify: "end",
		},
		Children: []*rpc.UINode{
			{
				ID:      "refresh",
				Type:    "button",
				Key:     "refresh-btn",
				Props:   map[string]any{"label": "Refresh", "variant": "ghost"},
				OnClick: "refresh",
			},
		},
	})

	badge := ""
	if u != nil {
		badge = magpie.Cost(u.Today.Cost)
	}
	return rpc.UIRenderResult{
		Title:     "Magpie",
		Badge:     badge,
		RefreshMS: 15000,
		Root: &rpc.UINode{
			Type:  "infoCard",
			Key:   "magpie-card",
			Props: map[string]any{"title": "Magpie"},
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

func kvRow(key, label, value string) *rpc.UINode {
	return &rpc.UINode{
		Type:  "row",
		Key:   key,
		Style: &rpc.UIStyle{Justify: "between", Align: "center", Gap: 8},
		Children: []*rpc.UINode{
			{Type: "text", Key: key + "-k", Props: map[string]any{"text": label, "tone": "muted"}, Style: &rpc.UIStyle{FontSize: 12}},
			{Type: "text", Key: key + "-v", Props: map[string]any{"text": value}, Style: &rpc.UIStyle{FontSize: 12, Tabular: true}},
		},
	}
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func resolveAgentSessionID(ctx context.Context, peer *rpc.Peer, sessionID, paneID string) string {
	if peer == nil || (paneID == "" && sessionID == "") {
		return ""
	}
	var st rpc.HostGetStateResult
	if err := peer.Call(ctx, "host.getState", rpc.HostGetStateParams{}, &st); err != nil {
		return ""
	}
	for _, s := range st.State.Sessions {
		for _, pn := range s.Panes {
			if paneID != "" && pn.ID != paneID {
				continue
			}
			if paneID == "" && (s.ID != sessionID || (s.Focus != "" && pn.ID != s.Focus)) {
				continue
			}
			if pn.AgentSessionID != "" {
				return pn.AgentSessionID
			}
			if pn.AgentSession != nil && pn.AgentSession.ID != "" {
				return pn.AgentSession.ID
			}
		}
	}
	return ""
}
