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
	"hyla/locales"
)

const PluginID = "hyla.magpie"

func init() {
	Register()
}

// Register installs the bundled hyla.magpie manifest and RPC runner.
func Register() {
	_ = manifest.RegisterBuiltin(manifest.BuiltinPlugin{
		Manifest:         manifestIn(""),
		EnabledByDefault: true,
		AutoGrant:        true,
		Words:            manifestIn,
	})
	daemonhost.RegisterBuiltinRunner(PluginID, daemonhost.HostKindDaemon, runServer)
}

// manifestIn is the plugin's manifest in locale: its name is its own, and
// what it says of itself, its action and its setting are the app's words,
// which the app asks for in the language it shows.
func manifestIn(locale string) *manifest.Manifest {
	words := locales.In(locale)
	return &manifest.Manifest{
		APIVersion:  manifest.APIVersion,
		ID:          PluginID,
		Name:        "Magpie",
		Version:     "1.0.0",
		Description: words.PluginMagpieDescription(),
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
				Title:   words.PluginMagpieAction(),
				Palette: true,
				Target:  "daemon",
			},
		},
		UI: manifest.UISpec{
			Slots: []manifest.SlotSpec{
				{
					ID:    "magpie",
					Mount: "info.card",
					Title: words.InfoMagpie(),
					Mode:  "declarative",
					Host:  "daemon",
					Order: 10,
				},
			},
		},
		Settings: []manifest.SettingSpec{
			{
				Key:         "url",
				Title:       words.PluginMagpieSettingURL(),
				Description: words.PluginMagpieSettingURLDetail(),
				Type:        "string",
				Placeholder: "http://127.0.0.1:3425",
			},
		},
	}
}

const (
	magpieOnlineCacheTTL  = 10 * time.Second
	magpieOfflineCacheTTL = 60 * time.Second
	magpieSettingsTTL     = 60 * time.Second
)

type state struct {
	mu         sync.Mutex
	url        string
	locale     string
	urlChecked time.Time
	lastRead   time.Time
	lastSess   string
	usage      *magpie.Usage
	lastErr    string
	fetchFn    func(base, sess string) (*magpie.Usage, error)
	nowFn      func() time.Time
}

// languageOf is the app's language, which the card's words are said in:
// the one a render comes with, as the app's language changes while it
// runs, or else the one initialize gave.
func (st *state) languageOf(render string) string {
	if render != "" {
		return render
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.locale
}

func (st *state) now() time.Time {
	if st.nowFn != nil {
		return st.nowFn()
	}
	return time.Now()
}

func runServer(ctx context.Context, peer *rpc.Peer) {
	runServerWith(ctx, peer, magpie.Fetch, time.Now)
}

func runServerWith(ctx context.Context, peer *rpc.Peer, fetchFn func(base, sess string) (*magpie.Usage, error), nowFn func() time.Time) {
	if fetchFn == nil {
		fetchFn = magpie.Fetch
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	st := &state{
		fetchFn: fetchFn,
		nowFn:   nowFn,
	}

	peer.Handle("initialize", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.InitializeParams
		_ = json.Unmarshal(raw, &p)
		st.mu.Lock()
		// The app's language, which the card's words are said in: this
		// server runs where the app's own catalog was never given one.
		st.locale = p.Locale
		if u, ok := p.Settings["url"].(string); ok {
			st.url = strings.TrimRight(strings.TrimSpace(u), "/")
		} else {
			st.url = ""
		}
		st.urlChecked = st.now()
		st.mu.Unlock()
		return rpc.InitializeResult{
			Name:          "Magpie",
			Version:       "1.0.0",
			Capabilities:  []string{"ui.declarative"},
			Subscriptions: []string{"theme.changed", "settings.changed"},
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
		st.refreshURL(ctx, peer, false)
		agentSessID := resolveAgentSessionID(ctx, peer, p.SessionID, p.PaneID)
		u, errStr := st.fetch(agentSessID, false)
		return buildRenderResult(u, errStr, st.languageOf(p.Locale)), nil
	})
	peer.Handle("ui.event", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.UIEventParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if p.ComponentID == "refresh" || p.EventType == "click" {
			st.refreshURL(ctx, peer, true)
			agentSessID := resolveAgentSessionID(ctx, peer, p.SessionID, p.PaneID)
			u, errStr := st.fetch(agentSessID, true)
			res := buildRenderResult(u, errStr, st.languageOf(p.Locale))
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
		st.urlChecked = time.Time{}
		st.mu.Unlock()
		_ = peer.Call(ctx, "ui.invalidate", rpc.UIInvalidateParams{SlotIDs: []string{"magpie"}}, nil)
		return rpc.ActionInvokeResult{OK: true}, nil
	})
	peer.Handle("event.notify", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.EventNotifyParams
		if json.Unmarshal(raw, &p) == nil && p.Topic == "settings.changed" {
			st.mu.Lock()
			st.urlChecked = time.Time{}
			st.lastRead = time.Time{}
			st.mu.Unlock()
			_ = peer.Call(ctx, "ui.invalidate", rpc.UIInvalidateParams{SlotIDs: []string{"magpie"}}, nil)
		}
		return nil, nil
	})

	_ = peer.Serve(ctx)
}

func (st *state) refreshURL(ctx context.Context, peer *rpc.Peer, force bool) {
	if peer == nil {
		return
	}
	st.mu.Lock()
	due := force || st.urlChecked.IsZero() || st.now().Sub(st.urlChecked) >= magpieSettingsTTL
	st.mu.Unlock()
	if !due {
		return
	}
	var cfg rpc.PluginGetSettingsResult
	err := peer.Call(ctx, "plugin.getSettings", rpc.PluginGetSettingsParams{}, &cfg)
	st.mu.Lock()
	st.urlChecked = st.now()
	if err == nil {
		if u, ok := cfg.Settings["url"].(string); ok {
			st.url = strings.TrimRight(strings.TrimSpace(u), "/")
		} else {
			st.url = ""
		}
	}
	st.mu.Unlock()
}

func (st *state) fetch(sessionID string, force bool) (*magpie.Usage, string) {
	st.mu.Lock()
	now := st.now()
	base := st.url
	if base == "" {
		base = magpie.URL()
	}
	if !force && !st.lastRead.IsZero() {
		age := now.Sub(st.lastRead)
		if st.usage == nil && st.lastErr != "" && age < magpieOfflineCacheTTL {
			u, errStr := st.usage, st.lastErr
			st.mu.Unlock()
			return u, errStr
		}
		if st.usage != nil && st.lastSess == sessionID && age < magpieOnlineCacheTTL {
			u, errStr := st.usage, st.lastErr
			st.mu.Unlock()
			return u, errStr
		}
	}
	fn := st.fetchFn
	st.mu.Unlock()

	u, err := fn(base, sessionID)
	st.mu.Lock()
	st.lastRead = st.now()
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

func isOfflineError(errStr string) bool {
	return errStr == "" ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "magpie does not run at ")
}

func buildRenderResult(u *magpie.Usage, errStr, locale string) rpc.UIRenderResult {
	words := locales.In(locale)
	var children []*rpc.UINode
	refreshMS := 15000
	if u == nil {
		refreshMS = 60000
		msg := words.InfoMagpieDown()
		if !isOfflineError(errStr) {
			msg = errStr
		}
		children = append(children, &rpc.UINode{
			Type:  "text",
			Key:   "offline",
			Props: map[string]any{"text": msg, "tone": "muted", "wrap": true},
		})
	} else {
		children = append(children,
			kvRow("calls", words.InfoMagpieCalls(), fmt.Sprintf("%d", u.Today.Calls)),
			kvRow("cost", words.InfoMagpieCost(), fmt.Sprintf("$%.4f", u.Today.Cost)),
			kvRow("tokens", words.InfoMagpieTokens(), formatTokens(u.Today.Count())),
		)
		if u.Session > 0 {
			children = append(children, kvRow("session", words.InfoMagpieSession(), fmt.Sprintf("%d ($%.4f)", u.Session, u.SessionCost)))
		}
		if u.Latest.Status > 0 {
			children = append(children, kvRow("latest", words.InfoMagpieLeft(), fmt.Sprintf("%d · %dms", u.Latest.Status, u.Latest.Ms)))
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
				Props:   map[string]any{"label": words.InfoMagpieRefresh(), "variant": "ghost"},
				OnClick: "refresh",
			},
		},
	})

	badge := ""
	if u != nil {
		badge = magpie.Cost(u.Today.Cost)
	}
	return rpc.UIRenderResult{
		Title:     words.InfoMagpie(),
		Badge:     badge,
		RefreshMS: refreshMS,
		Root: &rpc.UINode{
			Type:  "infoCard",
			Key:   "magpie-card",
			Props: map[string]any{"title": words.InfoMagpie()},
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
	if peer == nil {
		return ""
	}
	var st rpc.HostGetStateResult
	if err := peer.Call(ctx, "host.getState", rpc.HostGetStateParams{}, &st); err != nil {
		return ""
	}
	if sessionID == "" {
		sessionID = st.ActiveSession
	}
	if paneID == "" {
		paneID = st.FocusedPane
	}
	if paneID == "" && sessionID == "" {
		return ""
	}
	var matchedPaneID string
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
			matchedPaneID = pn.ID
			break
		}
	}
	if matchedPaneID == "" {
		matchedPaneID = paneID
	}
	if matchedPaneID != "" {
		var env rpc.PaneGetEnvResult
		if err := peer.Call(ctx, "pane.getEnv", rpc.PaneGetEnvParams{Pane: matchedPaneID}, &env); err == nil && env.AgentSessionID != "" {
			return env.AgentSessionID
		}
	}
	return ""
}
