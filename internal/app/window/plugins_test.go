package window

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/app/host"
	"hyla/internal/client"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/plugins/uihost"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/menus"
	uisettings "hyla/internal/ui/settings"
	"hyla/internal/ui/settings/kit"
	"hyla/locales"
)

type mockDaemonCaller struct {
	mu       sync.Mutex
	calls    []string
	statuses []rpc.PluginStatus
	renderFn func(pluginID, method string, params json.RawMessage) (any, error)
}

func (m *mockDaemonCaller) CallPlugin(pluginID, method string, params any, out any) error {
	raw, _ := json.Marshal(params)
	m.mu.Lock()
	m.calls = append(m.calls, pluginID+":"+method)
	fn := m.renderFn
	m.mu.Unlock()
	if fn == nil {
		return nil
	}
	res, err := fn(pluginID, method, raw)
	if err != nil {
		return err
	}
	if out != nil && res != nil {
		b, _ := json.Marshal(res)
		return json.Unmarshal(b, out)
	}
	return nil
}

func (m *mockDaemonCaller) PluginStatuses() []rpc.PluginStatus {
	return m.statuses
}

func writeTestPlugin(t *testing.T, base, id, body string) string {
	t.Helper()
	dir := filepath.Join(base, "src-"+id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifest.ManifestFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPluginSideTabInfoCardAndShortcut(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "com.example.panel", `
api_version = 1
id = "com.example.panel"
name = "Panel Ext"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[daemon]
command = ["echo"]

[[ui.slots]]
id = "summary"
mount = "info.card"
title = "Build Info"
mode = "declarative"
host = "daemon"

[[ui.slots]]
id = "inspector"
mount = "side.tab"
title = "Inspect"
mode = "declarative"
host = "daemon"

[[ui.slots]]
id = "board"
mount = "pane"
title = "Board View"
mode = "declarative"
host = "daemon"

[[actions]]
id = "ping"
title = "Ping Panel"
shortcut = "CmdOrCtrl+Shift+U"
target = "daemon"
palette = true
`)
	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var invoked []string
	mockDC := &mockDaemonCaller{
		renderFn: func(pluginID, method string, raw json.RawMessage) (any, error) {
			mu.Lock()
			invoked = append(invoked, method)
			mu.Unlock()
			switch method {
			case "ui.render":
				var p rpc.UIRenderParams
				_ = json.Unmarshal(raw, &p)
				if p.SlotID == "summary" {
					return rpc.UIRenderResult{
						Title: "Build Info",
						Badge: "v2",
						Root: &rpc.UINode{
							Type:  "text",
							Props: map[string]any{"text": "Build Status: Green"},
						},
					}, nil
				}
				return rpc.UIRenderResult{
					Root: &rpc.UINode{
						Type: "column",
						Children: []*rpc.UINode{
							{Type: "text", Props: map[string]any{"text": "Inspector Ready"}},
							{Type: "button", ID: "inc", OnClick: "inc", Props: map[string]any{"label": "Increment"}},
						},
					},
				}, nil
			case "ui.event":
				return rpc.UIEventResult{
					Handled: true,
					Root: &rpc.UINode{
						Type: "column",
						Children: []*rpc.UINode{
							{Type: "text", Props: map[string]any{"text": "Inspector Updated"}},
							{Type: "button", ID: "inc", OnClick: "inc", Props: map[string]any{"label": "Increment"}},
						},
					},
				}, nil
			case "action.invoke":
				return rpc.ActionInvokeResult{OK: true, Message: "Pong!"}, nil
			}
			return nil, nil
		},
	}
	a.Plugins.DaemonFn = func(hostKey string) uihost.DaemonCaller { return mockDC }

	a.showSide(sideInfo)
	tt := ui.NewTester(a.view, 1200, 760)
	for i := 0; i < 15 && !tt.HasText("Build Status: Green"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Build Info") || !tt.HasText("Build Status: Green") {
		t.Fatalf("expected Info card Build Info/Build Status: Green, got texts: %v", tt.Texts())
	}

	// Switch to custom plugin side tab and let panel slide animation settle before clicking.
	a.showSide("plugin:com.example.panel:inspector")
	for i := 0; i < 60 && tt.FramePending(); i++ {
		tt.Frame()
		tt.Advance(time.Second)
	}
	for i := 0; i < 20 && !tt.HasText("Inspector Ready"); i++ {
		tt.Frame()
		tt.Advance(50 * time.Millisecond)
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Inspector Ready") {
		t.Fatalf("expected custom side tab body, got: %v", tt.Texts())
	}
	for i := 0; i < 6; i++ {
		tt.Frame()
		tt.Advance(50 * time.Millisecond)
	}
	if err := tt.Click("Increment"); err != nil {
		t.Fatalf("click Increment: %v", err)
	}
	tt.Frame()
	for i := 0; i < 25 && !tt.HasText("Inspector Updated"); i++ {
		tt.Frame()
		time.Sleep(15 * time.Millisecond)
	}
	if !tt.HasText("Inspector Updated") {
		mockDC.mu.Lock()
		calls := slices.Clone(mockDC.calls)
		mockDC.mu.Unlock()
		t.Fatalf("expected updated UI after click (calls=%v), got: %v", calls, tt.Texts())
	}

	// Non-menubar shortcut should invoke plugin action.
	tt.Key(ui.Cmd|ui.Shift, ui.KeyU)
	tt.Frame()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		hasAction := slices.Contains(invoked, "action.invoke")
		mu.Unlock()
		if hasAction {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	hasAction := slices.Contains(invoked, "action.invoke")
	mu.Unlock()
	if !hasAction {
		t.Fatalf("expected CmdOrCtrl+Shift+U shortcut to trigger action.invoke, got %v", invoked)
	}
	if _, ok := actions.Find(manifest.QualifyActionID("com.example.panel", "ping")); !ok {
		t.Fatal("expected qualified plugin action registered")
	}
}

func TestPluginPaneRenderFallbackAndWebMenu(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "com.example.panes", `
api_version = 1
id = "com.example.panes"
name = "Pane Ext"
version = "1.0.0"

[permissions]
scopes = ["ui.slot", "ui.webview"]

[daemon]
command = ["echo"]

[[ui.slots]]
id = "board"
mount = "pane"
title = "Board View"
mode = "declarative"
host = "daemon"

[[ui.slots]]
id = "webboard"
mount = "pane"
title = "Web Board"
mode = "webview"
web_root = "web"
entry = "index.html"

[[actions]]
id = "open"
title = "Open Board"
menu = "Plugins"
target = "open_pane"
view = "board"
`)
	_ = os.MkdirAll(filepath.Join(dir, "web"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "web", "index.html"), []byte("<html><body>board</body></html>"), 0o644)

	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	mockDC := &mockDaemonCaller{
		renderFn: func(pluginID, method string, raw json.RawMessage) (any, error) {
			if method == "ui.render" {
				return rpc.UIRenderResult{
					Root: &rpc.UINode{Type: "text", Props: map[string]any{"text": "Kanban Column Alpha"}},
				}, nil
			}
			return nil, nil
		},
	}
	a.Plugins.DaemonFn = func(hostKey string) uihost.DaemonCaller { return mockDC }

	// Replace session pane layout with plugin pane.
	a.St.Sessions[0].Panes = []proto.Pane{
		{
			ID:          "plg-1",
			Plugin:      "com.example.panes",
			PluginView:  "board",
			PluginTitle: "Board View",
			Status:      proto.Idle,
			Progress:    -1,
		},
	}
	a.St.Sessions[0].Layout = nil
	a.St.Sessions[0].Focus = "plg-1"

	tt := ui.NewTester(a.view, 1000, 680)
	for i := 0; i < 15 && !tt.HasText("Kanban Column Alpha"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Kanban Column Alpha") {
		t.Fatalf("expected declarative plugin pane content, got %v", tt.Texts())
	}

	// Headless WebView plugin pane renders gracefully without crash.
	a.St.Sessions[0].Panes[0].PluginView = "webboard"
	a.St.Sessions[0].Panes[0].PluginTitle = "Web Board"
	tt.Frame()
	if !tt.HasText("Web Board") {
		t.Fatalf("expected WebView plugin pane title in headless test, got %v", tt.Texts())
	}

	// Disabling plugin shows friendly fallback card.
	if err := st.Enable("com.example.panes", false); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText(locales.PluginUnavailableTitle()) || !tt.HasText(locales.PluginClosePane()) {
		t.Fatalf("expected fallback Plugin Unavailable card, got %v", tt.Texts())
	}

	// Re-enable and verify Plugins submenu items include action & Manage Plugins.
	_ = st.Enable("com.example.panes", true)
	_ = a.reloadPlugins(false)
	var labels []string
	for _, sub := range menus.PluginMenuItems(a) {
		labels = append(labels, sub.Label)
	}
	if !slices.Contains(labels, "Open Board") || !slices.Contains(labels, locales.MenuManagePlugins()) {
		t.Fatalf("unexpected Plugins submenu items: %v", labels)
	}
}

func TestPluginRemoteCapabilityCompatAndControlGuard(t *testing.T) {
	a := testApp(t)

	oldRemote := &host.Host{
		Name: "legacy-box",
		Addr: "10.0.0.5:2222",
		Conn: host.StateReady,
		Client: &client.Client{
			Welcome: proto.Welcome{
				Version: proto.Version,
				// Older server without plugins.v1 capability
			},
		},
	}
	capRemote := &host.Host{
		Name: "modern-box",
		Addr: "10.0.0.6:2222",
		Conn: host.StateReady,
		Client: &client.Client{
			Welcome: proto.Welcome{
				Version:      proto.Version,
				Capabilities: []string{proto.CapPluginsV1},
			},
		},
	}
	a.Machines = append(a.Machines, oldRemote, capRemote)

	if a.hostSupportsPlugins(oldRemote) {
		t.Fatal("oldRemote without CapPluginsV1 must not report plugin support")
	}
	if got := a.pluginDaemonCaller(a.pluginHostKey(oldRemote)); got != nil {
		t.Fatal("expected nil DaemonCaller for oldRemote lacking plugins.v1")
	}

	if !a.hostSupportsPlugins(capRemote) {
		t.Fatal("capRemote with CapPluginsV1 should support plugins")
	}
	if got := a.pluginDaemonCaller(a.pluginHostKey(capRemote)); got == nil {
		t.Fatal("expected non-nil DaemonCaller for capRemote")
	}

	// Remote hosts must NEVER gain remoteControl even if settingsIn.RemoteControl = true.
	a.settingsIn.RemoteControl = true
	if a.remoteControl(capRemote) {
		t.Fatal("remoteControl must remain false for remote hosts")
	}

	// Remote action message must be ignored cleanly.
	remActionID := manifest.QualifyActionID("com.example.rem", "ping")
	actions.SetPluginActions("com.example.rem", []actions.Action{{
		ID:       remActionID,
		Title:    "Remote Ping",
		Remote:   true,
		PluginID: "com.example.rem",
	}})
	t.Cleanup(actions.ClearPluginActions)

	a.EventOf(capRemote, capRemote.Client, proto.Message{
		Event:  "action",
		Action: remActionID,
	})
	if a.From != nil {
		t.Fatal("expected remote action dispatch to be blocked")
	}
}

func TestSettingsPluginsPage(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "com.example.cfg", `
api_version = 1
id = "com.example.cfg"
name = "Config Demo"
version = "0.2.0"

[permissions]
scopes = ["ui.slot", "pane.read"]

[[settings]]
key = "endpoint"
title = "API Endpoint"
type = "string"
default = "http://localhost:8080"

[[settings]]
key = "token"
title = "API Secret"
type = "secret"
`)
	st := manifest.DefaultStore()
	if _, err := st.Link(dir, false); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	a.Ss.Page = "plugins"
	tt := ui.NewTester(uisettings.View(a), 860, 720)
	tt.Frame()

	texts := strings.Join(tt.Texts(), "\n")
	if !strings.Contains(texts, "Config Demo") || !strings.Contains(texts, "Magpie") {
		t.Fatalf("expected Config Demo and Magpie in Settings › Plugins, got:\n%s", texts)
	}

	// Saving config via PluginsView writes exclusively to plugin-config/<id>.json, not settings.json.
	pa := a.PluginsView()
	pa.SaveConfig("com.example.cfg", map[string]any{"endpoint": "https://api.example.com"})
	cfg, err := st.LoadConfig("com.example.cfg")
	if err != nil || cfg["endpoint"] != "https://api.example.com" {
		t.Fatalf("unexpected plugin config: %#v err=%v", cfg, err)
	}
	mainCfgBytes, _ := os.ReadFile(settings.Path())
	if strings.Contains(string(mainCfgBytes), "com.example.cfg") {
		t.Fatal("plugin config must not leak into main Hyla settings file")
	}

	// Permissions toggle updates GrantedPermissions.
	pa.SetPermission("com.example.cfg", manifest.PermPaneRead, true)
	reg, err := st.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(reg.Plugins["com.example.cfg"].GrantedPermissions, manifest.PermPaneRead) {
		t.Fatalf("expected granted pane.read permission, got %v", reg.Plugins["com.example.cfg"].GrantedPermissions)
	}

	// Secret write goes cleanly through Store.
	pa.SetSecret("com.example.cfg", "token", "s3cr3t-value")
	sec, ok, err := st.GetSecret("com.example.cfg", "token")
	if err != nil || !ok || sec != "s3cr3t-value" {
		t.Fatalf("secret mismatch: ok=%v sec=%q err=%v", ok, sec, err)
	}

	// Disable/enable toggle persists cleanly.
	pa.SetEnabled("com.example.cfg", false)
	if a.Plugins.IsPluginEnabled("com.example.cfg") {
		t.Fatal("expected com.example.cfg disabled")
	}
}

func TestPluginSettingsPageLifecycleEventsAndFallback(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "com.example.radar", `
api_version = 1
id = "com.example.radar"
name = "Radar"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[daemon]
command = ["echo"]

[[ui.slots]]
id = "radar"
mount = "settings.page"
title = "Radar"
icon = "icons/radar.svg"
mode = "declarative"
host = "daemon"
order = 10

[[ui.slots]]
id = "extra"
mount = "settings.page"
title = "Radar Extra"
mode = "declarative"
host = "daemon"
order = 20

[[settings]]
key = "interval"
title = "Scan Interval"
description = "Polling interval in seconds"
type = "integer"
default = 30
`)
	_ = os.MkdirAll(filepath.Join(dir, "icons"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "icons", "radar.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><circle cx="8" cy="8" r="5"/></svg>`), 0o644)

	st := manifest.DefaultStore()
	// Initially ungranted: must NOT show in SettingsPages.
	if _, err := st.Link(dir, false); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}
	pa := a.PluginsView()
	if pages := pa.SettingsPages(); len(pages) != 0 {
		t.Fatalf("expected no plugin settings pages before ui.slot permission granted, got %+v", pages)
	}

	// Grant ui.slot and verify pages, custom SVG vs fallback glyph.Plugins, and Palette integration.
	if err := st.SetPermission("com.example.radar", manifest.PermUISlot, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}
	pages := pa.SettingsPages()
	if len(pages) != 2 {
		t.Fatalf("expected 2 plugin settings pages, got %+v", pages)
	}
	if pages[0].ID != "plugin:com.example.radar:radar" || pages[0].Title != "Radar" {
		t.Fatalf("unexpected first page: %+v", pages[0])
	}
	if pages[0].Icon == nil || pages[0].Icon == glyph.Plugins {
		t.Fatalf("expected custom SVG icon loaded for Radar page, got %v", pages[0].Icon)
	}
	if pages[1].ID != "plugin:com.example.radar:extra" || pages[1].Icon != glyph.Plugins {
		t.Fatalf("expected fallback glyph.Plugins icon for extra page, got %+v", pages[1])
	}
	var foundInPalette bool
	for _, p := range a.Pages() {
		if p.ID == "plugin:com.example.radar:radar" {
			foundInPalette = true
		}
	}
	if !foundInPalette {
		t.Fatalf("expected plugin settings.page in Command Palette Pages(), got %+v", a.Pages())
	}

	var updateCount atomic.Int32
	origUpdate := a.Plugins.OnUpdate
	a.Plugins.OnUpdate = func() {
		updateCount.Add(1)
		if origUpdate != nil {
			origUpdate()
		}
	}

	mockDC := &mockDaemonCaller{
		renderFn: func(pluginID, method string, raw json.RawMessage) (any, error) {
			switch method {
			case "ui.render":
				var p rpc.UIRenderParams
				_ = json.Unmarshal(raw, &p)
				if p.Mount != manifest.MountSettingsPage {
					t.Errorf("expected mount=settings.page, got %q", p.Mount)
				}
				return rpc.UIRenderResult{
					Root: &rpc.UINode{
						Type: "settingsGroup",
						Key:  "grp",
						Props: map[string]any{
							"title": "Radar Status",
						},
						Children: []*rpc.UINode{
							{
								Type:  "settingsRow",
								Key:   "status-row",
								Props: map[string]any{"label": "Scanner State", "detail": "Radar Status: Ready"},
								Children: []*rpc.UINode{
									{ID: "scan", Type: "button", Key: "scan-btn", OnClick: "runScan", Props: map[string]any{"label": "Run Scan", "variant": "primary"}},
								},
							},
						},
					},
				}, nil
			case "ui.event":
				return rpc.UIEventResult{
					Handled: true,
					Message: "Scan complete",
					Root: &rpc.UINode{
						Type: "settingsGroup",
						Key:  "grp",
						Props: map[string]any{
							"title": "Radar Status",
						},
						Children: []*rpc.UINode{
							{
								Type:  "settingsRow",
								Key:   "status-row",
								Props: map[string]any{"label": "Scanner State", "detail": "Radar Status: Scanned"},
								Children: []*rpc.UINode{
									{ID: "scan", Type: "button", Key: "scan-btn", OnClick: "runScan", Props: map[string]any{"label": "Run Scan", "variant": "primary"}},
								},
							},
						},
					},
				}, nil
			}
			return nil, nil
		},
	}
	a.Plugins.DaemonFn = func(hostKey string) uihost.DaemonCaller { return mockDC }

	a.Ss.Page = "general"
	tt := ui.NewTester(uisettings.View(a), 860, 720)
	tt.Frame()
	if !tt.HasText("Radar") {
		t.Fatalf("expected Radar nav item in Settings sidebar, texts=%v", tt.Texts())
	}

	// Navigate to standalone plugin Settings page via sidebar click.
	if err := tt.Click("Radar"); err != nil {
		t.Fatalf("click Radar sidebar item: %v", err)
	}
	if a.Ss.Page != "plugin:com.example.radar:radar" {
		t.Fatalf("expected Ss.Page=plugin:com.example.radar:radar, got %q", a.Ss.Page)
	}
	for i := 0; i < 25 && !tt.HasText("Radar Status: Ready"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Radar Status: Ready") || !tt.HasText("Scan Interval") {
		t.Fatalf("expected Declarative UI + [[settings]] on standalone page, texts=%v", tt.Texts())
	}
	if updateCount.Load() == 0 {
		t.Fatal("expected OnUpdate callback after async ui.render")
	}

	// Click Run Scan button -> verify ui.event updates UI and OnMessage sets toast.
	prevUpdates := updateCount.Load()
	if err := tt.Click("Run Scan"); err != nil {
		t.Fatalf("click Run Scan: %v", err)
	}
	for i := 0; i < 25 && !tt.HasText("Radar Status: Scanned"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Radar Status: Scanned") {
		t.Fatalf("expected updated Declarative UI after ui.event, texts=%v", tt.Texts())
	}
	if updateCount.Load() <= prevUpdates {
		t.Fatal("expected OnUpdate callback after async ui.event")
	}
	if a.PendingToast != "Scan complete" {
		t.Fatalf("expected PendingToast='Scan complete', got %q", a.PendingToast)
	}

	// Verify ui.invalidate RPC triggers OnUpdate & marks slot invalid.
	prevUpdates = updateCount.Load()
	bridge := &appPluginBridge{a: a}
	if err := bridge.InvalidateUI(context.Background(), "com.example.radar", rpc.UIInvalidateParams{SlotIDs: []string{"radar"}}); err != nil {
		t.Fatalf("InvalidateUI: %v", err)
	}
	if updateCount.Load() <= prevUpdates {
		t.Fatal("expected OnUpdate callback after ui.invalidate")
	}

	// Verify second settings.page of the same plugin does NOT duplicate [[settings]], and still renders Declarative UI.
	a.Ss.Page = "plugin:com.example.radar:extra"
	for i := 0; i < 25 && !tt.HasText("Radar Status: Ready"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if tt.HasText("Scan Interval") {
		t.Fatal("expected [[settings]] rendered only on plugin's first settings.page")
	}

	// Disable plugin while its standalone page is active -> must gracefully fall back to "plugins".
	a.Ss.Page = "plugin:com.example.radar:radar"
	pa.SetEnabled("com.example.radar", false)
	tt.Frame()
	if a.Ss.Page != "plugins" {
		t.Fatalf("expected graceful fallback to plugins when plugin disabled, got %q", a.Ss.Page)
	}
	if tt.HasText("Radar Status") {
		t.Fatalf("disabled plugin settings page should no longer render, texts=%v", tt.Texts())
	}

	// Re-enable, open standalone page, then remove plugin -> must also fall back gracefully.
	pa.SetEnabled("com.example.radar", true)
	a.Ss.Page = "plugin:com.example.radar:radar"
	tt.Frame()
	pa.Remove("com.example.radar")
	tt.Frame()
	if a.Ss.Page != "plugins" {
		t.Fatalf("expected graceful fallback to plugins when plugin removed, got %q", a.Ss.Page)
	}
}

func TestPluginSettingsStaticPackPage(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "com.example.packset", `
api_version = 1
id = "com.example.packset"
name = "Pack Settings"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[[ui.slots]]
id = "prefs"
mount = "settings.page"
title = "Pack Prefs"
mode = "declarative"

[[settings]]
key = "greeting"
title = "Greeting Text"
type = "string"
default = "Hello"
`)
	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	a.Ss.Page = "plugin:com.example.packset:prefs"
	tt := ui.NewTester(uisettings.View(a), 860, 720)
	tt.Frame()
	if !tt.HasText("Greeting Text") {
		t.Fatalf("expected [[settings]] rendered for static pack settings.page, texts=%v", tt.Texts())
	}
	for _, txt := range tt.Texts() {
		if strings.Contains(strings.ToLower(txt), "unavailable") || strings.Contains(strings.ToLower(txt), "error") {
			t.Fatalf("static pack settings.page should not show RPC error/unavailable: %v", tt.Texts())
		}
	}
}

func TestPluginSettingsDrawerAndBuiltinSectionSlots(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")

	// Plugin A: settings.plugins slot with zero [[settings]] and zero extra scopes beyond ui.slot,
	// plus another test verifying zero scopes & zero [[settings]].
	dirDrawer := writeTestPlugin(t, home, "com.example.drawer", `
api_version = 1
id = "com.example.drawer"
name = "Drawer Ext"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[daemon]
command = ["echo"]

[[ui.slots]]
id = "diag"
mount = "settings.plugins"
title = "Live Diagnostics"
mode = "declarative"
host = "daemon"
`)

	// Plugin B: sections appended onto built-in General and Appearance pages.
	dirSec := writeTestPlugin(t, home, "com.example.sections", `
api_version = 1
id = "com.example.sections"
name = "Section Ext"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[daemon]
command = ["echo"]

[[ui.slots]]
id = "gen-sec"
mount = "settings.general"
title = "General Addon"
mode = "declarative"
host = "daemon"

[[ui.slots]]
id = "app-sec"
mount = "settings.appearance"
title = "Plain Addon"
mode = "declarative"
host = "daemon"
`)

	// Plugin C: settings.plugins slot with ZERO settings and ZERO permission scopes still shows Settings button.
	dirSlotOnly := writeTestPlugin(t, home, "com.example.slotonly", `
api_version = 1
id = "com.example.slotonly"
name = "Slot Only"
version = "1.0.0"

[[ui.slots]]
id = "panel"
mount = "settings.plugins"
title = "Only Slot"
mode = "declarative"
`)

	st := manifest.DefaultStore()
	if _, err := st.Link(dirDrawer, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Link(dirSec, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Link(dirSlotOnly, false); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	mockDC := &mockDaemonCaller{
		renderFn: func(pluginID, method string, raw json.RawMessage) (any, error) {
			switch method {
			case "ui.render":
				var p rpc.UIRenderParams
				_ = json.Unmarshal(raw, &p)
				switch p.SlotID {
				case "diag":
					return rpc.UIRenderResult{
						Root: &rpc.UINode{
							Type:  "settingsGroup",
							Key:   "diag-grp",
							Props: map[string]any{"title": "Diagnostics"},
							Children: []*rpc.UINode{
								{
									Type:  "settingsRow",
									Key:   "conn-row",
									Props: map[string]any{"label": "Connection", "detail": "Connection Idle"},
									Children: []*rpc.UINode{
										{ID: "ping", Type: "button", Key: "ping-btn", OnClick: "ping", Props: map[string]any{"label": "Ping Host"}},
									},
								},
							},
						},
					}, nil
				case "gen-sec":
					return rpc.UIRenderResult{
						Root: &rpc.UINode{
							Type:  "settingsGroup",
							Key:   "gen-grp",
							Props: map[string]any{"title": "General Addon"},
							Children: []*rpc.UINode{
								{
									Type:  "settingsRow",
									Key:   "gen-row",
									Props: map[string]any{"label": "Sync Mode", "detail": "Background"},
									Children: []*rpc.UINode{
										{ID: "sync-btn", Type: "button", Key: "sync-btn", OnClick: "sync", Props: map[string]any{"label": "Sync Now"}},
									},
								},
							},
						},
					}, nil
				case "app-sec":
					return rpc.UIRenderResult{
						Root: &rpc.UINode{
							Type: "column",
							Key:  "app-col",
							Children: []*rpc.UINode{
								{Type: "text", Key: "app-txt", Props: map[string]any{"text": "Appearance Addon Active"}},
							},
						},
					}, nil
				}
			case "ui.event":
				var ev rpc.UIEventParams
				_ = json.Unmarshal(raw, &ev)
				if ev.SlotID == "diag" {
					return rpc.UIEventResult{
						Handled: true,
						Root: &rpc.UINode{
							Type:  "settingsGroup",
							Key:   "diag-grp",
							Props: map[string]any{"title": "Diagnostics"},
							Children: []*rpc.UINode{
								{
									Type:  "settingsRow",
									Key:   "conn-row",
									Props: map[string]any{"label": "Connection", "detail": "Connection Healthy"},
									Children: []*rpc.UINode{
										{ID: "ping", Type: "button", Key: "ping-btn", OnClick: "ping", Props: map[string]any{"label": "Ping Host"}},
									},
								},
							},
						},
					}, nil
				}
				return rpc.UIEventResult{
					Handled: true,
					Root: &rpc.UINode{
						Type:  "settingsGroup",
						Key:   "gen-grp",
						Props: map[string]any{"title": "General Addon"},
						Children: []*rpc.UINode{
							{
								Type:  "settingsRow",
								Key:   "gen-row",
								Props: map[string]any{"label": "Sync Mode", "detail": "Synced Just Now"},
							},
						},
					},
				}, nil
			}
			return nil, nil
		},
	}
	a.Plugins.DaemonFn = func(hostKey string) uihost.DaemonCaller { return mockDC }

	// 1. Settings › Plugins drawer renders settings.plugins Declarative UI and dispatches ui.event.
	a.Ss.Page = "plugins"
	tt := ui.NewTester(uisettings.View(a), 860, 900)
	tt.Frame()

	rowRect, ok := tt.Find("Drawer Ext · v1.0.0")
	if !ok {
		t.Fatalf("expected Drawer Ext row in Settings › Plugins, texts=%v", tt.Texts())
	}
	nextRect, ok := tt.Find("Section Ext · v1.0.0")
	if !ok {
		t.Fatalf("expected Section Ext row in Settings › Plugins, texts=%v", tt.Texts())
	}
	var drawerBtn *ui.AccessInfo
	for _, ac := range tt.AccessAll() {
		if ac.Role == ui.RoleButton && ac.Name == locales.SetPluginsSettings() && ac.Bounds.Y >= rowRect.Y && ac.Bounds.Y < nextRect.Y {
			cp := ac
			drawerBtn = &cp
			break
		}
	}
	if drawerBtn == nil {
		t.Fatalf("expected Settings button on Drawer Ext row, access=%+v", tt.AccessAll())
	}
	tt.ClickAt(drawerBtn.Bounds.X+drawerBtn.Bounds.W/2, drawerBtn.Bounds.Y+drawerBtn.Bounds.H/2)
	for i := 0; i < 25 && !tt.HasText("Connection Idle"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if a.Ss.PluginDetail != "com.example.drawer" || !tt.HasText("Connection Idle") {
		t.Fatalf("expected Drawer Ext settings.plugins slot rendered, detail=%q texts=%v", a.Ss.PluginDetail, tt.Texts())
	}
	if err := tt.Click("Ping Host"); err != nil {
		t.Fatalf("click Ping Host inside drawer: %v", err)
	}
	for i := 0; i < 25 && !tt.HasText("Connection Healthy"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Connection Healthy") {
		t.Fatalf("expected Connection Healthy after drawer button click, texts=%v", tt.Texts())
	}

	// Verify Slot Only (0 settings, 0 scopes, has settings.plugins slot) shows Settings drawer button.
	slotOnlyRow, ok := tt.Find("Slot Only · v1.0.0")
	if !ok {
		t.Fatalf("expected Slot Only row in Settings › Plugins, texts=%v", tt.Texts())
	}
	feishuRow, ok := tt.Find("Feishu · v1.0.0")
	if !ok {
		t.Fatalf("expected Feishu row in Settings › Plugins, texts=%v", tt.Texts())
	}
	var slotOnlyBtn bool
	for _, ac := range tt.AccessAll() {
		if ac.Role == ui.RoleButton && ac.Name == locales.SetPluginsSettings() && ac.Bounds.Y >= slotOnlyRow.Y && ac.Bounds.Y < feishuRow.Y {
			slotOnlyBtn = true
			break
		}
	}
	if !slotOnlyBtn {
		t.Fatalf("expected Settings button for plugin with only settings.plugins slot, access=%+v", tt.AccessAll())
	}

	// 2. Built-in General page appends settings.general section and handles events.
	a.Ss.Page = "general"
	for i := 0; i < 25 && !tt.HasText("Sync Mode"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("General Addon") || !tt.HasText("Sync Mode") {
		t.Fatalf("expected settings.general section on General page, texts=%v", tt.Texts())
	}
	tt.Scroll(500, 400, 0, 700)
	tt.Frame()
	if err := tt.Click("Sync Now"); err != nil {
		t.Fatalf("click Sync Now on General page: %v", err)
	}
	for i := 0; i < 25 && !tt.HasText("Synced Just Now"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Synced Just Now") {
		t.Fatalf("expected Synced Just Now after General section button click, texts=%v", tt.Texts())
	}

	// 3. Built-in Appearance page wraps plain column section in kit.Group with slot title.
	a.Ss.Page = "appearance"
	for i := 0; i < 25 && !tt.HasText("Appearance Addon Active"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Plain Addon") || !tt.HasText("Appearance Addon Active") {
		t.Fatalf("expected settings.appearance section on Appearance page, texts=%v", tt.Texts())
	}

	// 4. Revoking ui.slot removes section from built-in page.
	a.PluginsView().SetPermission("com.example.sections", manifest.PermUISlot, false)
	a.Ss.Page = "general"
	tt.Frame()
	if tt.HasText("General Addon") {
		t.Fatalf("expected General Addon hidden after revoking ui.slot, texts=%v", tt.Texts())
	}
}

func TestPluginSettingsSearchAndDrawerReveal(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "com.example.search", `
api_version = 1
id = "com.example.search"
name = "Alert Bridge"
version = "1.0.0"
description = "Forwards workspace alerts to Slack"

[permissions]
scopes = ["ui.slot"]

[[ui.slots]]
id = "slack"
mount = "settings.page"
title = "Slack Integration"
mode = "declarative"

[[ui.slots]]
id = "drawer-slot"
mount = "settings.plugins"
title = "Slack Connection Check"
mode = "declarative"

[[ui.slots]]
id = "gen-slot"
mount = "settings.general"
title = "Slack General Banner"
mode = "declarative"

[[settings]]
key = "webhook_url"
title = "Custom Webhook"
description = "Delivers build alerts to Slack"
type = "string"

[[settings]]
key = "restore_hook"
title = "Restore Hook"
description = "Session restore notification hook"
type = "boolean"
default = false
`)
	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	a.Ss.Page = "general"
	tt := ui.NewTester(uisettings.View(a), 860, 720)
	tt.Frame()

	// 1. Search by [[settings]] description -> auto-switches to "plugins", opens drawer, highlights & shows row.
	a.Ss.Query = "Delivers build alerts"
	tt.Frame()
	if a.Ss.Page != "plugins" {
		t.Fatalf("expected Ss.Page=plugins, got %q", a.Ss.Page)
	}
	if a.Ss.PluginDetail != "com.example.search" {
		t.Fatalf("expected Ss.PluginDetail=com.example.search, got %q", a.Ss.PluginDetail)
	}
	if !kit.Hits["Custom Webhook"] {
		t.Fatalf("expected kit.Hits['Custom Webhook']=true, got %v", kit.Hits)
	}
	if !tt.HasText("Custom Webhook") {
		t.Fatalf("expected Custom Webhook visible in expanded drawer, texts=%v", tt.Texts())
	}

	// Manually closing the drawer while query stays unchanged must NOT immediately re-open it.
	a.Ss.PluginDetail = ""
	tt.Frame()
	if a.Ss.PluginDetail != "" {
		t.Fatalf("manual close should stick while query unchanged, got %q", a.Ss.PluginDetail)
	}

	// 2. Search by [[settings]] key when current page ("general") ALSO matches -> clicking "Plugins" opens drawer.
	a.Ss.Query = ""
	a.Ss.Page = "general"
	tt.Frame()
	a.Ss.Query = "restore"
	tt.Frame()
	if a.Ss.Page != "general" {
		t.Fatalf("expected to remain on general while it has matches, got %q", a.Ss.Page)
	}
	if err := tt.Click("Plugins"); err != nil {
		t.Fatalf("click Plugins in filtered sidebar: %v (texts=%v)", err, tt.Texts())
	}
	if a.Ss.Page != "plugins" || a.Ss.PluginDetail != "com.example.search" {
		t.Fatalf("expected clicking Plugins to switch page and expand com.example.search, got page=%q detail=%q", a.Ss.Page, a.Ss.PluginDetail)
	}
	if !kit.Hits["Restore Hook"] || !tt.HasText("Restore Hook") {
		t.Fatalf("expected Restore Hook highlighted and visible, hits=%v texts=%v", kit.Hits, tt.Texts())
	}

	// 3. Search for standalone settings.page -> navigates directly to plugin's Settings page.
	a.Ss.Page = "general"
	a.Ss.Query = "Slack Integration"
	tt.Frame()
	if a.Ss.Page != "plugin:com.example.search:slack" {
		t.Fatalf("expected navigation to standalone plugin settings page, got %q", a.Ss.Page)
	}
	if !tt.HasText("Custom Webhook") {
		t.Fatalf("expected plugin settings rendered on standalone page, texts=%v", tt.Texts())
	}

	// 4. Search by settings.plugins slot title -> opens plugins page and expands drawer.
	a.Ss.Page = "general"
	a.Ss.PluginDetail = ""
	a.Ss.Query = "Slack Connection Check"
	tt.Frame()
	if a.Ss.Page != "plugins" || a.Ss.PluginDetail != "com.example.search" {
		t.Fatalf("expected settings.plugins slot search to open drawer, got page=%q detail=%q", a.Ss.Page, a.Ss.PluginDetail)
	}

	// 5. Search by built-in settings.general slot title -> finds general page.
	if resGen := uisettings.SearchResults(a, "Slack General Banner"); len(resGen["general"]) == 0 {
		t.Fatalf("expected settings.general slot indexed under general, got %v", resGen)
	}

	// 6. Disabled plugin's settings.page and slots disappear from SearchResults.
	a.PluginsView().SetEnabled("com.example.search", false)
	res := uisettings.SearchResults(a, "Slack Integration")
	if res["plugin:com.example.search:slack"] != nil {
		t.Fatalf("disabled plugin settings.page should not appear in SearchResults: %v", res)
	}
	if resGen := uisettings.SearchResults(a, "Slack General Banner"); len(resGen["general"]) != 0 {
		t.Fatalf("disabled plugin settings.general slot should not appear in SearchResults: %v", resGen)
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("HYLA_TEST_WINDOW_PLUGIN_HELPER") == "1" {
		runWindowPluginHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runWindowPluginHelper() {
	rwc := struct {
		io.Reader
		io.WriteCloser
	}{os.Stdin, os.Stdout}
	peer := rpc.NewPeer(rwc, rwc, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu      sync.Mutex
		counter int
	)

	peer.Handle("initialize", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		return rpc.InitializeResult{
			Name:         "App Helper",
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
		cancel()
		_ = peer.Close()
		return nil, nil
	})
	peer.Handle("ui.render", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		var p rpc.UIRenderParams
		_ = json.Unmarshal(raw, &p)
		mu.Lock()
		n := counter
		mu.Unlock()
		return rpc.UIRenderResult{
			Root: &rpc.UINode{
				Type: "column",
				Key:  "helper-col",
				Children: []*rpc.UINode{
					{Type: "text", Key: "helper-status", Props: map[string]any{"text": fmt.Sprintf("App Mount %s Count %d", p.Mount, n)}},
					{ID: "bump", Type: "button", Key: "helper-bump", OnClick: "bump", Props: map[string]any{"label": "Bump & Save", "variant": "primary"}},
				},
			},
		}, nil
	})
	peer.Handle("ui.event", func(ctx context.Context, method string, raw json.RawMessage) (any, error) {
		mu.Lock()
		counter++
		n := counter
		mu.Unlock()
		_ = peer.Call(ctx, "plugin.setSettings", rpc.PluginSetSettingsParams{
			Settings: map[string]any{"endpoint": fmt.Sprintf("https://helper.local/%d", n)},
		}, nil)
		_ = peer.Call(ctx, "ui.notify", rpc.UINotifyParams{
			Title: "Helper",
			Body:  fmt.Sprintf("Updated %d", n),
		}, nil)
		return rpc.UIEventResult{
			Handled:    true,
			Invalidate: true,
			Root: &rpc.UINode{
				Type: "column",
				Key:  "helper-col",
				Children: []*rpc.UINode{
					{Type: "text", Key: "helper-status", Props: map[string]any{"text": fmt.Sprintf("App Mount Updated %d", n)}},
					{ID: "bump", Type: "button", Key: "helper-bump", OnClick: "bump", Props: map[string]any{"label": "Bump & Save", "variant": "primary"}},
				},
			},
		}, nil
	})
	peer.Start(ctx)
	select {
	case <-ctx.Done():
	case <-peer.Done():
	}
}

func TestPluginSettingsRealAppSubprocessE2E(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "com.example.appproc", fmt.Sprintf(`
api_version = 1
id = "com.example.appproc"
name = "App Process Plugin"
version = "1.0.0"

[permissions]
scopes = ["ui.slot", "notifications"]

[app]
command = [%q]
transport = "stdio"

[env]
HYLA_TEST_WINDOW_PLUGIN_HELPER = "1"

[[ui.slots]]
id = "app-page"
mount = "settings.page"
title = "App Helper"
mode = "declarative"
host = "app"

[[settings]]
key = "endpoint"
title = "Helper Endpoint"
type = "string"
default = "https://helper.local/0"
`, exe))

	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	a.Ss.Page = "plugin:com.example.appproc:app-page"
	tt := ui.NewTester(uisettings.View(a), 860, 720)
	for i := 0; i < 60 && !tt.HasText("App Mount settings.page Count 0"); i++ {
		tt.Frame()
		time.Sleep(15 * time.Millisecond)
	}
	if !tt.HasText("App Mount settings.page Count 0") {
		t.Fatalf("expected live [app] subprocess ui.render in Settings, texts=%v", tt.Texts())
	}

	// Verify [[settings]] shows initial default before clicking button.
	if a.Ss.PluginDraft["com.example.appproc|endpoint"] != "https://helper.local/0" {
		t.Fatalf("expected initial draft https://helper.local/0, got %q", a.Ss.PluginDraft["com.example.appproc|endpoint"])
	}

	// Click button -> runs ui.event -> calls plugin.setSettings + ui.notify -> repaints updated tree & syncs untouched draft.
	if err := tt.Click("Bump & Save"); err != nil {
		t.Fatalf("click Bump & Save: %v", err)
	}
	for i := 0; i < 60 && !tt.HasText("App Mount settings.page Count 1") && !tt.HasText("App Mount Updated 1"); i++ {
		tt.Frame()
		time.Sleep(15 * time.Millisecond)
	}
	if !tt.HasText("App Mount Updated 1") && !tt.HasText("App Mount settings.page Count 1") {
		t.Fatalf("expected updated text from real [app] subprocess ui.event, texts=%v", tt.Texts())
	}
	if a.PendingToast != "Helper: Updated 1" {
		t.Fatalf("expected ui.notify toast 'Helper: Updated 1', got %q", a.PendingToast)
	}
	// Reloading plugin settings view should pick up endpoint written by plugin.setSettings because user did not edit field.
	tt.Frame()
	if got := a.Ss.PluginDraft["com.example.appproc|endpoint"]; got != "https://helper.local/1" {
		t.Fatalf("expected PluginDraft synced to plugin.setSettings value, got %q", got)
	}
}

func TestPluginSettingsSidebarOverflowAndAllBuiltinMounts(t *testing.T) {
	a := testApp(t)
	home := os.Getenv("HYLA_HOME")

	var slotsTOML strings.Builder
	for i := 1; i <= 10; i++ {
		slotsTOML.WriteString(fmt.Sprintf(`
[[ui.slots]]
id = "p%d"
mount = "settings.page"
title = "Plugin Page %02d"
mode = "declarative"
host = "daemon"
order = %d
`, i, i, i))
	}
	builtinMounts := []string{
		"terminal", "shell", "browser", "keyboard", "feishu",
		"agents", "sync", "hosts", "advanced",
	}
	for _, pg := range builtinMounts {
		slotsTOML.WriteString(fmt.Sprintf(`
[[ui.slots]]
id = "sec-%s"
mount = "settings.%s"
title = "Addon %s"
mode = "declarative"
host = "daemon"
`, pg, pg, pg))
	}
	// Also test a non-group settings.plugins slot + mismatched host=app slot on daemon-only plugin.
	slotsTOML.WriteString(`
[[ui.slots]]
id = "custom-drawer"
mount = "settings.plugins"
title = "Drawer Custom Controls"
mode = "declarative"
host = "daemon"

[[ui.slots]]
id = "badhost"
mount = "settings.page"
title = "Mismatched Host"
mode = "declarative"
host = "app"
order = 99
`)

	dir := writeTestPlugin(t, home, "com.example.overflow", `
api_version = 1
id = "com.example.overflow"
name = "Overflow Ext"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[daemon]
command = ["echo"]
`+slotsTOML.String())

	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	mockDC := &mockDaemonCaller{
		renderFn: func(pluginID, method string, raw json.RawMessage) (any, error) {
			if method != "ui.render" {
				return nil, nil
			}
			var p rpc.UIRenderParams
			_ = json.Unmarshal(raw, &p)
			if p.SlotID == "custom-drawer" {
				return rpc.UIRenderResult{
					Root: &rpc.UINode{
						Type: "column",
						Key:  "dcol",
						Children: []*rpc.UINode{
							{Type: "text", Key: "dt", Props: map[string]any{"text": "Drawer Non-Group Content"}},
						},
					},
				}, nil
			}
			return rpc.UIRenderResult{
				Root: &rpc.UINode{
					Type:  "text",
					Key:   "txt-" + p.SlotID,
					Props: map[string]any{"text": "Rendered " + p.SlotID},
				},
			}, nil
		},
	}
	a.Plugins.DaemonFn = func(hostKey string) uihost.DaemonCaller { return mockDC }

	// 1. At minimum Settings window height (700x480), Open Configuration remains visible and nav scrolls to Plugin Page 10.
	a.Ss.Page = "general"
	tt := ui.NewTester(uisettings.View(a), 700, 480)
	tt.Frame()

	cfgRect, ok := tt.Find(locales.SettingsOpenConfig())
	if !ok {
		t.Fatalf("expected Open Configuration button visible at 700x480, texts=%v", tt.Texts())
	}
	if cfgRect.Y+cfgRect.H > 480 || cfgRect.H < 12 {
		t.Fatalf("Open Configuration squeezed or clipped at 480px height: %+v", cfgRect)
	}
	if tt.HasText("Plugin Page 10") {
		// If it's offscreen it may or may not appear in Texts; check bounds instead.
		if p10, ok := tt.Find("Plugin Page 10"); ok && p10.Y < cfgRect.Y {
			t.Fatalf("expected Plugin Page 10 to overflow below 480px fold (cfg=%+v, p10=%+v)", cfgRect, p10)
		}
	}
	tt.Scroll(100, 200, 0, 900)
	tt.Frame()
	if err := tt.Click("Plugin Page 10"); err != nil {
		t.Fatalf("click scrolled Plugin Page 10 at 700x480: %v", err)
	}
	for i := 0; i < 20 && !tt.HasText("Rendered p10"); i++ {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if a.Ss.Page != "plugin:com.example.overflow:p10" || !tt.HasText("Rendered p10") {
		t.Fatalf("expected Plugin Page 10 active and rendered, page=%q texts=%v", a.Ss.Page, tt.Texts())
	}

	// 2. Programmatic jump back to Page 01 scrolls nav item into view (NavRevealed).
	a.Ss.Page = "plugin:com.example.overflow:p1"
	tt.Frame()
	if a.Ss.NavRevealed != "plugin:com.example.overflow:p1" {
		t.Fatalf("expected NavRevealed updated, got %q", a.Ss.NavRevealed)
	}

	// 3. Mismatched host=app on daemon-only plugin renders no RPC error.
	a.Ss.Page = "plugin:com.example.overflow:badhost"
	tt.Frame()
	for _, txt := range tt.Texts() {
		if strings.Contains(strings.ToLower(txt), "has no app host process") {
			t.Fatalf("mismatched host slot should not emit RPC error, texts=%v", tt.Texts())
		}
	}

	// 4. All built-in settings.<page> mounts render cleanly.
	for _, pg := range builtinMounts {
		a.Ss.Page = pg
		for i := 0; i < 20 && !tt.HasText("Rendered sec-"+pg); i++ {
			tt.Frame()
			time.Sleep(10 * time.Millisecond)
		}
		if !tt.HasText("Addon "+pg) || !tt.HasText("Rendered sec-"+pg) {
			t.Fatalf("built-in page %q missing plugin section, texts=%v", pg, tt.Texts())
		}
	}

	// 5. Searching for non-group settings.plugins slot title opens drawer AND highlights slot header.
	a.Ss.Page = "general"
	a.Ss.PluginDetail = ""
	a.Ss.Query = "Drawer Custom Controls"
	tt.Frame()
	if a.Ss.Page != "plugins" || a.Ss.PluginDetail != "com.example.overflow" {
		t.Fatalf("expected search for Drawer Custom Controls to open drawer, page=%q detail=%q", a.Ss.Page, a.Ss.PluginDetail)
	}
	if !kit.Hits["Drawer Custom Controls"] {
		t.Fatalf("expected kit.Hits['Drawer Custom Controls']=true, hits=%v", kit.Hits)
	}

	// 6. Explicit uisettings.Open with non-empty page clears stale search query.
	var openedView func(c *ui.Context)
	uisettings.Open(a, "plugin:com.example.overflow:p2", func(content any) {
		openedView, _ = content.(func(*ui.Context))
	})
	if a.Ss.Page != "plugin:com.example.overflow:p2" || a.Ss.Query != "" || openedView == nil {
		t.Fatalf("expected Open(a, page) to clear stale query and select target page, got page=%q query=%q view=%v", a.Ss.Page, a.Ss.Query, openedView != nil)
	}
	tt.Frame()
	if a.Ss.Page != "plugin:com.example.overflow:p2" {
		t.Fatalf("expected page to remain on p2 after frame, got %q", a.Ss.Page)
	}
}
