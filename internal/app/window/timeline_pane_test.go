package window

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/plugins/uihost"
	"hyla/internal/proto"
	"hyla/internal/ui/menus"
	uisettings "hyla/internal/ui/settings"
	"hyla/locales"
)

func TestLegacyTimelinePaneRendersThroughPluginBody(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "t", Timeline: "a", Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
	s.Focus = "t"

	// Without hyla.timeline installed, a legacy Timeline pane displays the plugin unavailable card.
	tt := ui.NewTester(a.view, 1280, 720)
	tt.Frame()
	if !tt.HasText(locales.PluginUnavailableTitle()) || !tt.HasText(locales.PluginUnavailableDetail("hyla.timeline")) {
		t.Fatalf("expected plugin unavailable fallback for legacy timeline pane, texts=%q", tt.Texts())
	}

	// Once hyla.timeline is linked & enabled, the legacy Timeline pane renders through the plugin webview slot.
	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "hyla.timeline", `
api_version = 1
id = "hyla.timeline"
name = "Timeline"
version = "1.0.0"

[permissions]
scopes = ["ui.webview", "ui.slot", "session.read", "timeline.read"]

[[ui.slots]]
id = "timeline"
mount = "pane"
title = "Timeline"
mode = "webview"
host = "app"
web_root = "web"
entry = "index.html"
`)
	if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web", "index.html"), []byte("<html><body>timeline</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	tt2 := ui.NewTester(a.view, 1280, 720)
	tt2.Frame()
	if tt2.HasText(locales.PluginUnavailableTitle()) {
		t.Fatalf("expected legacy timeline pane to stop showing unavailable fallback when hyla.timeline is enabled, texts=%q", tt2.Texts())
	}
	if !tt2.HasText("timeline") {
		t.Fatalf("expected headless plugin pane name \"timeline\", texts=%q", tt2.Texts())
	}
}

func TestAppBridgeGetTimeline(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	var gotParams proto.TimelineParams
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		if method != "timeline.get" {
			return nil, errors.New("unexpected method " + method)
		}
		if err := json.Unmarshal(raw, &gotParams); err != nil {
			return nil, err
		}
		return proto.Timeline{
			Session: "s1",
			Pane:    "a",
			Agent:   "claude",
			Read:    1761285600000,
			Turns: []proto.TimelineStep{
				{Kind: proto.TimelineTurn, Title: "Fix the bug", Start: 1000, End: 4000, State: proto.TimelineDone},
			},
		}, nil
	})
	a.Local().Client = cl
	defer stop()

	bridge := &appPluginBridge{a: a}
	res, err := bridge.GetTimeline(context.Background(), "hyla.timeline", rpc.PaneGetTimelineParams{
		Session: "s1",
		Pane:    "a",
	})
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}
	if gotParams.Session != "s1" || gotParams.Pane != "a" {
		t.Fatalf("unexpected timeline.get params: %+v", gotParams)
	}
	if res.Timeline.Agent != "claude" || len(res.Timeline.Turns) != 1 || res.Timeline.Turns[0].Title != "Fix the bug" {
		t.Fatalf("unexpected GetTimeline result: %+v", res)
	}
}

func TestAppBridgeOpenPaneSplitAndFocusExisting(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	s := a.session("s1")
	s.Focus = "b"
	a.Focus["s1"] = "b"

	splits := make(chan proto.CreateParams, 2)
	focuses := make(chan proto.FocusParams, 2)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "split":
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			splits <- p
			return proto.Session{ID: "s1"}, nil
		case "focus":
			var p proto.FocusParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			focuses <- p
			return nil, nil
		}
		return nil, errors.New("unexpected method " + method)
	})
	a.Local().Client = cl
	defer stop()

	bridge := &appPluginBridge{a: a}
	req := rpc.UIOpenPaneParams{
		Session:       "s1",
		Split:         "a",
		View:          "timeline",
		Title:         "claude",
		Args:          map[string]string{"targetPane": "a"},
		Direction:     "right",
		FocusExisting: true,
	}
	if err := bridge.OpenPane(context.Background(), "hyla.timeline", req); err != nil {
		t.Fatalf("OpenPane first call failed: %v", err)
	}
	select {
	case p := <-splits:
		if p.Split != "a" || p.Direction != "right" || p.Plugin != "hyla.timeline" || p.PluginView != "timeline" || p.PluginTitle != "claude" || p.PluginArgs["targetPane"] != "a" {
			t.Fatalf("unexpected split CreateParams: %+v", p)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for split")
	}

	// Simulate daemon adding the timeline plugin pane; second OpenPane with FocusExisting focuses it without splitting again.
	s.Panes = append(s.Panes, proto.Pane{
		ID:          "t",
		Dir:         "/tmp",
		Status:      proto.Idle,
		Progress:    -1,
		Plugin:      "hyla.timeline",
		PluginView:  "timeline",
		PluginTitle: "claude",
		PluginArgs:  map[string]string{"targetPane": "a"},
	})
	s.Layout = s.Layout.Split("a", "t", "right", 0)
	s.Focus = "a"
	a.Focus["s1"] = "a"

	if err := bridge.OpenPane(context.Background(), "hyla.timeline", req); err != nil {
		t.Fatalf("OpenPane second call failed: %v", err)
	}
	select {
	case p := <-splits:
		t.Fatalf("expected FocusExisting to focus existing pane without splitting again, got %+v", p)
	case <-time.After(50 * time.Millisecond):
	}
	if got := a.Focus["s1"]; got != "t" {
		t.Fatalf("expected focus on existing timeline pane %q, got %q", "t", got)
	}
	select {
	case fp := <-focuses:
		if fp.Pane != "t" {
			t.Fatalf("expected daemon focus on t, got %+v", fp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for focus RPC")
	}
}

func TestLegacyTimelineOpenActionDelegation(t *testing.T) {
	a := testApp(t)
	a.run("timeline.open", nil)
	if a.PendingToast != locales.PluginUnavailableDetail("hyla.timeline") {
		t.Fatalf("expected PluginUnavailableDetail toast when hyla.timeline is missing, got %q", a.PendingToast)
	}
	a.PendingToast = ""

	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "hyla.timeline", `
api_version = 1
id = "hyla.timeline"
name = "Timeline"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[daemon]
command = ["echo"]

[[actions]]
id = "open"
title = "Open Timeline Pane"
menu = "Pane"
shortcut = "CmdOrCtrl+Alt+T"
palette = true
remote = true
target = "daemon"
`)
	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	called := make(chan string, 1)
	mockDC := &mockDaemonCaller{
		renderFn: func(pluginID, method string, params json.RawMessage) (any, error) {
			if method == "action.invoke" {
				var p rpc.ActionInvokeParams
				_ = json.Unmarshal(params, &p)
				called <- pluginID + ":" + p.ActionID
			}
			return rpc.ActionInvokeResult{OK: true}, nil
		},
	}
	a.Plugins.DaemonFn = func(hostKey string) uihost.DaemonCaller {
		return mockDC
	}
	a.run("timeline.open", nil)
	if a.PendingToast != "" {
		t.Fatalf("did not expect unavailable toast once hyla.timeline is installed, got %q", a.PendingToast)
	}
	select {
	case got := <-called:
		if got != "hyla.timeline:open" {
			t.Fatalf("expected action.invoke open on hyla.timeline, got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for delegated timeline.open action.invoke")
	}
}

func TestTimelinePluginManifestI18nAndLiveSwitch(t *testing.T) {
	a := testApp(t)
	a.settingsIn.Language = "zh-Hans"
	a.refreshPluginLocaleAndTheme()

	home := os.Getenv("HYLA_HOME")
	dir := writeTestPlugin(t, home, "hyla.timeline", `
api_version = 1
id = "hyla.timeline"
name = "Timeline"
description = "EN desc"
version = "1.0.0"

[permissions]
scopes = ["ui.webview", "ui.slot", "session.read", "timeline.read"]

[[actions]]
id = "open"
title = "Open Timeline Pane"
menu = "Pane"
target = "app"
view = "timeline"

[[ui.slots]]
id = "timeline"
mount = "pane"
title = "Timeline"
mode = "webview"
host = "app"
web_root = "web"
entry = "index.html"

[i18n.zh-Hans]
name = "时间线"
description = "中文时间线描述"

[i18n.zh-Hans.actions]
open = "打开时间线窗格"

[i18n.zh-Hans.slots]
timeline = "时间线"

[i18n.ja]
name = "タイムライン"
description = "日本語の説明"

[i18n.ja.actions]
open = "タイムラインペインを開く"

[i18n.ja.slots]
timeline = "タイムライン"
`)
	if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web", "index.html"), []byte("<html>ok</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := manifest.DefaultStore()
	if _, err := st.Link(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := a.reloadPlugins(false); err != nil {
		t.Fatal(err)
	}

	paneLabel := func(items []*mygo.MenuItem) string {
		for _, it := range items {
			if it.ID == "hyla.timeline.open" {
				return it.Label
			}
		}
		return ""
	}
	if got := paneLabel(menus.PaneMenuItems(a)); got != "打开时间线窗格" {
		t.Fatalf("expected zh-Hans Pane menu label \"打开时间线窗格\", got %q", got)
	}
	snap := a.Plugins.Snapshot()
	if snap == nil || len(snap.Plugins) == 0 {
		t.Fatal("missing snapshot plugins")
	}
	var tlPlg manifest.InstalledPlugin
	for _, plg := range snap.Plugins {
		if plg.Entry.ID == "hyla.timeline" {
			tlPlg = plg
			break
		}
	}
	if title := uisettings.PluginRowTitle(tlPlg); title != "时间线 · v1.0.0" {
		t.Fatalf("expected localized PluginRowTitle \"时间线 · v1.0.0\", got %q", title)
	}

	// Switch live to Japanese and verify menu and row title update immediately.
	a.settingsIn.Language = "ja"
	a.refreshPluginLocaleAndTheme()
	a.Plugins.ApplyLocale(a.pluginLocaleValue())
	if got := paneLabel(menus.PaneMenuItems(a)); got != "タイムラインペインを開く" {
		t.Fatalf("expected ja Pane menu label \"タイムラインペインを開く\", got %q", got)
	}
	for _, plg := range a.Plugins.Snapshot().Plugins {
		if plg.Entry.ID == "hyla.timeline" {
			tlPlg = plg
			break
		}
	}
	if title := uisettings.PluginRowTitle(tlPlg); title != "タイムライン · v1.0.0" {
		t.Fatalf("expected ja PluginRowTitle \"タイムライン · v1.0.0\", got %q", title)
	}
}
