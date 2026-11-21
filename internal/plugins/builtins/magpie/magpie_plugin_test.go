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
func (noopBridge) FocusPane(context.Context, string, rpc.PaneFocusParams) error       { return nil }
func (noopBridge) InvalidateUI(context.Context, string, rpc.UIInvalidateParams) error { return nil }
func (noopBridge) OpenPane(context.Context, string, rpc.UIOpenPaneParams) error       { return nil }
func (noopBridge) GetTimeline(context.Context, string, rpc.PaneGetTimelineParams) (*rpc.PaneGetTimelineResult, error) {
	return &rpc.PaneGetTimelineResult{}, nil
}
func (noopBridge) NotifyUI(context.Context, string, rpc.UINotifyParams) error       { return nil }
func (noopBridge) WebViewEmit(context.Context, string, rpc.WebViewEmitParams) error { return nil }
func (noopBridge) WebViewPictureInPicture(context.Context, string, rpc.WebViewPIPParams) error {
	return nil
}

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

// The card's words, and the manifest's, are the app's language: the card
// is rendered where the catalogs were never given the app's locale, so
// the language comes with initialize, and the manifest is what the app
// asks for in the language it shows.
func TestWordsFollowTheAppsLanguage(t *testing.T) {
	zh := locales.In("zh-Hans")
	u := &magpie.Usage{Today: magpie.Bucket{Calls: 3, Cost: 0.5}, Session: 2, SessionCost: 0.1}
	res := buildRenderResult(u, "", "zh-Hans")
	raw, err := json.Marshal(res.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{zh.InfoMagpie(), zh.InfoMagpieCalls(), zh.InfoMagpieCost(), zh.InfoMagpieTokens(), zh.InfoMagpieSession(), zh.InfoMagpieRefresh()} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the card lacks %q: %s", want, raw)
		}
	}
	if res.Title != zh.InfoMagpie() {
		t.Errorf("the card is titled %q, not %q", res.Title, zh.InfoMagpie())
	}
	if got := buildRenderResult(nil, "", "zh-Hans"); !strings.Contains(got.Root.Children[0].Children[0].Props["text"].(string), zh.InfoMagpieDown()) {
		t.Errorf("an offline card says %q, not %q", got.Root.Children[0].Children[0].Props["text"], zh.InfoMagpieDown())
	}

	m := manifestIn("zh-Hans")
	if m.Description != zh.PluginMagpieDescription() {
		t.Errorf("the plugin says of itself %q, not %q", m.Description, zh.PluginMagpieDescription())
	}
	if len(m.Actions) != 1 || m.Actions[0].Title != zh.PluginMagpieAction() {
		t.Errorf("its action is %+v, not %q", m.Actions, zh.PluginMagpieAction())
	}
	if len(m.Settings) != 1 || m.Settings[0].Title != zh.PluginMagpieSettingURL() || m.Settings[0].Description != zh.PluginMagpieSettingURLDetail() {
		t.Errorf("its setting is %+v", m.Settings)
	}
	if m.Name != "Magpie" || m.ID != PluginID {
		t.Errorf("the manifest says %q %q", m.ID, m.Name)
	}
	// A system's tag, as a Mac names it, is the language it names.
	if got := locales.In("zh-Hans-CN").InfoMagpieCalls(); got != zh.InfoMagpieCalls() {
		t.Errorf("zh-Hans-CN says %q, not %q", got, zh.InfoMagpieCalls())
	}
	// A language it has no words for says the source ones.
	if m := manifestIn("de"); m.Description != locales.In("en").PluginMagpieDescription() {
		t.Errorf("a language of its own says %q", m.Description)
	}
}
