package magpieplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"hyla/internal/app/magpie"
	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
	"hyla/locales"
)

func boolPtr(b bool) *bool { return &b }

type noopBridge struct{}

func (noopBridge) GetState(context.Context, string, rpc.HostGetStateParams) (*rpc.HostGetStateResult, error) {
	return &rpc.HostGetStateResult{}, nil
}
func (noopBridge) ReadScreen(context.Context, string, rpc.PaneReadScreenParams) (*rpc.PaneReadScreenResult, error) {
	return &rpc.PaneReadScreenResult{}, nil
}
func (noopBridge) GetEnv(context.Context, string, rpc.PaneGetEnvParams) (*rpc.PaneGetEnvResult, error) {
	return &rpc.PaneGetEnvResult{}, nil
}
func (noopBridge) CreateSession(context.Context, string, proto.CreateParams) (*rpc.SessionCreateResult, error) {
	return &rpc.SessionCreateResult{}, nil
}
func (noopBridge) SendInput(context.Context, string, rpc.PaneSendInputParams) error { return nil }
func (noopBridge) SendKey(context.Context, string, rpc.PaneSendKeyParams) error     { return nil }
func (noopBridge) ReportStatus(context.Context, string, rpc.PaneReportStatusParams) error {
	return nil
}
func (noopBridge) FocusPane(context.Context, string, rpc.PaneFocusParams) error     { return nil }
func (noopBridge) InvalidateUI(context.Context, string, rpc.UIInvalidateParams) error { return nil }
func (noopBridge) OpenPane(context.Context, string, rpc.UIOpenPaneParams) error     { return nil }
func (noopBridge) NotifyUI(context.Context, string, rpc.UINotifyParams) error       { return nil }
func (noopBridge) WebViewEmit(context.Context, string, rpc.WebViewEmitParams) error { return nil }

func TestMagpieBuiltinPluginRenderAndSettings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"today": map[string]any{
					"calls": 12,
					"input": 2500,
					"cost":  0.042,
				},
			})
		case "/v1/requests":
			_ = json.NewEncoder(w).Encode(map[string]any{"requests": []any{}})
		case "/v1/quota":
			_ = json.NewEncoder(w).Encode(map[string]any{"providers": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}
	if err := st.SaveConfig(PluginID, map[string]any{"url": srv.URL}); err != nil {
		t.Fatal(err)
	}

	mgr := daemonhost.NewManager(daemonhost.ManagerOptions{
		HostKind: daemonhost.HostKindDaemon,
		Store:    st,
		Bridge:   noopBridge{},
	})
	t.Cleanup(mgr.ShutdownAll)
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var res rpc.UIRenderResult
	if err := mgr.Call(ctx, PluginID, "ui.render", rpc.UIRenderParams{SlotID: "magpie", Mount: "info.card"}, &res); err != nil {
		t.Fatalf("ui.render hyla.magpie: %v", err)
	}
	if res.Root == nil || res.Root.Type != "infoCard" || res.Title != "Magpie" {
		t.Fatalf("unexpected render result: %+v", res)
	}
	raw, _ := json.Marshal(res.Root)
	if string(raw) == "" || len(res.Root.Children) == 0 {
		t.Fatal("expected rendered infoCard children")
	}
}

func TestMagpieOfflineBackoffLocalizedAndSettingsCache(t *testing.T) {
	cHost, cPlugin := net.Pipe()
	defer cHost.Close()
	defer cPlugin.Close()

	var (
		mu             sync.Mutex
		getSettingsCnt int
		fetchCalls     int
		now            = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
		fetchErr       = fmt.Errorf("magpie does not run at http://127.0.0.1:3425")
		curUsage       *magpie.Usage
	)

	hostPeer := rpc.NewPeer(cHost, cHost, cHost)
	hostPeer.Handle("plugin.getSettings", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		mu.Lock()
		getSettingsCnt++
		mu.Unlock()
		return rpc.PluginGetSettingsResult{Settings: map[string]any{"url": "http://127.0.0.1:3999"}}, nil
	})
	hostPeer.Handle("host.getState", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return rpc.HostGetStateResult{
			ActiveSession: "s1",
			FocusedPane:   "p1",
			State: proto.State{Sessions: []proto.Session{{
				ID:    "s1",
				Focus: "p1",
				Panes: []proto.Pane{{ID: "p1", Agent: "claude"}},
			}}},
		}, nil
	})
	hostPeer.Handle("pane.getEnv", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return rpc.PaneGetEnvResult{AgentSessionID: "conv-via-getenv"}, nil
	})
	hostPeer.Handle("ui.invalidate", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return struct{}{}, nil
	})

	pluginPeer := rpc.NewPeer(cPlugin, cPlugin, cPlugin)
	defer pluginPeer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hostPeer.Start(ctx)
	go runServerWith(ctx, pluginPeer, func(base, sess string) (*magpie.Usage, error) {
		mu.Lock()
		defer mu.Unlock()
		fetchCalls++
		if curUsage != nil {
			return curUsage, nil
		}
		return nil, fetchErr
	}, func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	})

	var initRes rpc.InitializeResult
	if err := hostPeer.Call(ctx, "initialize", rpc.InitializeParams{
		Settings: map[string]any{"url": "http://127.0.0.1:3425"},
	}, &initRes); err != nil {
		t.Fatal(err)
	}

	// First offline render hits fetchFn once, uses localized down text, and does not re-read settings.
	var r1 rpc.UIRenderResult
	if err := hostPeer.Call(ctx, "ui.render", rpc.UIRenderParams{SlotID: "magpie"}, &r1); err != nil {
		t.Fatal(err)
	}
	raw1, _ := json.Marshal(r1.Root)
	if !strings.Contains(string(raw1), locales.InfoMagpieDown()) {
		t.Fatalf("expected localized offline message %q, got %s", locales.InfoMagpieDown(), raw1)
	}
	if r1.RefreshMS != 60000 {
		t.Fatalf("expected 60000ms refresh when offline, got %d", r1.RefreshMS)
	}

	// 25s later: offline backoff still avoids fetchFn & plugin.getSettings.
	mu.Lock()
	now = now.Add(25 * time.Second)
	mu.Unlock()
	var r2 rpc.UIRenderResult
	if err := hostPeer.Call(ctx, "ui.render", rpc.UIRenderParams{SlotID: "magpie"}, &r2); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if fetchCalls != 1 || getSettingsCnt != 0 {
		t.Fatalf("expected offline backoff & cached settings (fetchCalls=%d, getSettings=%d)", fetchCalls, getSettingsCnt)
	}
	// Clicking refresh forces settings refresh and immediate fetch (test connection refused too).
	fetchErr = fmt.Errorf("dial tcp 127.0.0.1:3425: connection refused")
	mu.Unlock()

	var evRes rpc.UIEventResult
	if err := hostPeer.Call(ctx, "ui.event", rpc.UIEventParams{
		SlotID: "magpie", ComponentID: "refresh", EventType: "click",
	}, &evRes); err != nil {
		t.Fatal(err)
	}
	rawEv, _ := json.Marshal(evRes.Root)
	if !strings.Contains(string(rawEv), locales.InfoMagpieDown()) {
		t.Fatalf("expected localized connection refused text, got %s", rawEv)
	}
	mu.Lock()
	if fetchCalls != 2 || getSettingsCnt != 1 {
		t.Fatalf("expected forced refresh to call fetch & getSettings (fetchCalls=%d, getSettings=%d)", fetchCalls, getSettingsCnt)
	}
	curUsage = &magpie.Usage{Today: magpie.Bucket{Calls: 5, Cost: 0.25}, Session: 2, SessionCost: 0.1}
	mu.Unlock()

	// settings.changed notification resets both settings & usage caches.
	if err := hostPeer.Notify("event.notify", rpc.EventNotifyParams{Topic: "settings.changed"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)

	var r3 rpc.UIRenderResult
	if err := hostPeer.Call(ctx, "ui.render", rpc.UIRenderParams{SlotID: "magpie"}, &r3); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if getSettingsCnt != 2 || fetchCalls != 3 {
		t.Fatalf("expected settings.changed to trigger refresh on render (fetchCalls=%d, getSettings=%d)", fetchCalls, getSettingsCnt)
	}
	mu.Unlock()
	if r3.RefreshMS != 15000 {
		t.Fatalf("expected 15000ms online refresh, got %d", r3.RefreshMS)
	}
}
