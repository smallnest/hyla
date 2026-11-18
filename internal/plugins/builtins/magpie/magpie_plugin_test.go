package magpieplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hyla/internal/plugins/daemonhost"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
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
