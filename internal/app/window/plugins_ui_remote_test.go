package window

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/plugins/uihost"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"
)

type fakeRemoteCaller struct {
	statuses []rpc.PluginStatus
	calls    []string
	renderFn func(pluginID, method string, params any) any
}

func (f *fakeRemoteCaller) CallPlugin(pluginID, method string, params any, out any) error {
	f.calls = append(f.calls, pluginID+":"+method)
	if f.renderFn != nil && out != nil {
		res := f.renderFn(pluginID, method, params)
		b, _ := json.Marshal(res)
		return json.Unmarshal(b, out)
	}
	return nil
}

func (f *fakeRemoteCaller) PluginStatuses() []rpc.PluginStatus { return f.statuses }

func TestInfoCardUnwrapSkipEmptyAndRemoteSlots(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HYLA_DIR", tmp)
	t.Setenv("HYLA_NO_KEYCHAIN", "1")

	localH := &host.Host{Conn: host.StateReady}
	remoteH := &host.Host{
		Name: "prod-box",
		Addr: "tcp://10.0.0.2:7777",
		Conn: host.StateReady,
		Client: &client.Client{
			Welcome: proto.Welcome{Capabilities: []string{proto.CapPluginsV1}},
		},
	}

	remoteSess := proto.Session{
		ID:      "rem-sess",
		Project: "proj1",
		Focus:   "rem-pane",
		Panes: []proto.Pane{{
			ID:         "rem-pane",
			Plugin:     "com.remote.ext",
			PluginView: "board",
		}},
	}

	remoteCaller := &fakeRemoteCaller{
		statuses: []rpc.PluginStatus{
			{
				ID:                 "com.remote.card",
				Name:               "Remote Card",
				Enabled:            true,
				Host:               "daemon",
				GrantedPermissions: []string{manifest.PermUISlot},
				Slots: []manifest.SlotSpec{
					{ID: "info", Mount: "info.card", Title: "RemoteInfo", Mode: "declarative", Host: "daemon", Order: 10},
					{ID: "empty", Mount: "info.card", Title: "EmptySlot", Mode: "declarative", Host: "daemon", Order: 20},
					{ID: "rtab", Mount: "side.tab", Title: "RemoteTab", Mode: "declarative", Host: "daemon", Order: 10},
				},
			},
			{
				ID:                 "com.remote.ext",
				Name:               "Remote Pane",
				Enabled:            true,
				Host:               "daemon",
				GrantedPermissions: []string{manifest.PermUISlot, manifest.PermUIWebView},
				Slots: []manifest.SlotSpec{
					{ID: "board", Mount: "pane", Title: "RemoteBoard", Mode: "declarative", Host: "daemon"},
					{ID: "web", Mount: "pane", Title: "RemoteWeb", Mode: "webview", Host: "daemon"},
				},
			},
		},
		renderFn: func(pluginID, method string, params any) any {
			var pr rpc.UIRenderParams
			b, _ := json.Marshal(params)
			_ = json.Unmarshal(b, &pr)
			switch pr.SlotID {
			case "info":
				return rpc.UIRenderResult{
					Title: "Unwrapped Title",
					Badge: "$9.99",
					Root: &rpc.UINode{
						Type:  "infoCard",
						Key:   "inner-card",
						Props: map[string]any{"title": "Inner Header Should Not Render"},
						Children: []*rpc.UINode{
							{Type: "text", Key: "body-txt", Props: map[string]any{"text": "Hello Remote Body"}},
						},
					},
				}
			case "empty":
				return rpc.UIRenderResult{Root: nil}
			case "board":
				return rpc.UIRenderResult{
					Root: &rpc.UINode{Type: "text", Key: "pane-txt", Props: map[string]any{"text": "Remote Pane Active"}},
				}
			default:
				return rpc.UIRenderResult{}
			}
		},
	}

	useKC := false
	coord := uihost.NewCoordinator(&manifest.Store{BaseDir: tmp, UseKeychain: &useKC}, nil, "test", nil)
	app := &App{
		Machines: []*host.Host{localH, remoteH},
		Owner: map[string]*host.Host{
			"rem-sess": remoteH,
			"rem-pane": remoteH,
		},
		St: proto.State{Sessions: []proto.Session{remoteSess}},
		Plugins: coord,
		DaemonPluginStatuses: map[string][]rpc.PluginStatus{
			"remote:prod-box": remoteCaller.statuses,
		},
	}
	coord.DaemonFn = func(hostKey string) uihost.DaemonCaller {
		if hostKey == "remote:prod-box" {
			return remoteCaller
		}
		return nil
	}
	coord.RemoteRef = app.pluginRemoteRef
	coord.PaneHostFn = func(sid, pid string) string {
		if sid != "" {
			return app.pluginHostKey(app.hostOf(sid))
		}
		return app.pluginHostKey(app.hostOf(pid))
	}

	// 1. Verify PluginTabs discovers remote side.tab slot.
	tabs := app.PluginTabs(&remoteSess)
	if len(tabs) != 1 || tabs[0].ID != "plugin:com.remote.card:rtab" {
		t.Fatalf("unexpected remote PluginTabs: %+v", tabs)
	}

	// 2. Render infoPluginCards twice (trigger async fetch, then render cached result):
	//    - "EmptySlot" (Root == nil) must NOT render a card.
	//    - "Unwrapped Title" infoCard must unwrap its inner header ("Inner Header Should Not Render" must NOT appear) and show "Hello Remote Body" + "$9.99".
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(360).Height(500).Children(func() {
			app.infoPluginCards(c, &theme.Light, &remoteSess, &remoteSess.Panes[0])
			app.PluginPaneBody(c, &theme.Light, &remoteSess, &remoteSess.Panes[0])
		})
	}, 400, 600)
	// First frame kicks async render, settle brief wait, second frame draws result.
	coord.OnUpdate = func() {}
	tt.Frame()
	// Call synchronous RenderSlot until fetched cleanly (tiny poll loop).
	for range 40 {
		cr := coord.RenderSlot("remote:prod-box", uihost.SlotEntry{
			PluginID: "com.remote.card",
			Slot:     manifest.SlotSpec{ID: "info", Mount: "info.card", Host: "daemon"},
			Host:     "daemon",
		}, rpc.UIRenderParams{SlotID: "info", SessionID: "rem-sess", PaneID: "rem-pane"})
		cr2 := coord.RenderSlot("remote:prod-box", uihost.SlotEntry{
			PluginID: "com.remote.card",
			Slot:     manifest.SlotSpec{ID: "empty", Mount: "info.card", Host: "daemon"},
			Host:     "daemon",
		}, rpc.UIRenderParams{SlotID: "empty", SessionID: "rem-sess", PaneID: "rem-pane"})
		cr3 := coord.RenderSlot("remote:prod-box", uihost.SlotEntry{
			PluginID: "com.remote.ext",
			Slot:     manifest.SlotSpec{ID: "board", Mount: "pane", Host: "daemon"},
			Host:     "daemon",
		}, rpc.UIRenderParams{SlotID: "board", SessionID: "rem-sess", PaneID: "rem-pane"})
		if !cr.Loading && !cr2.Loading && !cr3.Loading {
			break
		}
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	tt.Frame()

	if !tt.HasText("Unwrapped Title") || !tt.HasText("$9.99") || !tt.HasText("Hello Remote Body") {
		t.Fatalf("expected unwrapped remote info card content, texts=%v", tt.Texts())
	}
	if tt.HasText("Inner Header Should Not Render") {
		t.Fatalf("infoCard should have been unwrapped rather than nested, texts=%v", tt.Texts())
	}
	if tt.HasText("EmptySlot") {
		t.Fatalf("empty Root==nil slot should be skipped, texts=%v", tt.Texts())
	}
	if !tt.HasText("Remote Pane Active") {
		t.Fatalf("expected remote declarative PluginPaneBody text, texts=%v", tt.Texts())
	}

	// 3. Verify appPluginBridge.OpenPane rejects remote host session with ErrCodeRemoteControlDisabled.
	bridge := &appPluginBridge{a: app}
	err := bridge.OpenPane(context.Background(), "com.example.local", rpc.UIOpenPaneParams{
		Session: "rem-sess",
		View:    "board",
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != rpc.ErrCodeRemoteControlDisabled {
		t.Fatalf("expected ErrCodeRemoteControlDisabled for remote session OpenPane, got %v", err)
	}
}
