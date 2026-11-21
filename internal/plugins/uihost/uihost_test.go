package uihost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"
)

func boolPtr(b bool) *bool { return &b }

func TestDeclarativeUIRenderAll23NodesAndClick(t *testing.T) {
	var mu sync.Mutex
	var events []string

	tree := &rpc.UINode{
		Type: "column",
		Key:  "root",
		Style: &rpc.UIStyle{
			Padding: []float32{8},
			Gap:     6,
		},
		Children: []*rpc.UINode{
			{
				Type:  "infoCard",
				Key:   "card1",
				Props: map[string]any{"title": "Summary", "aside": "v1"},
				Children: []*rpc.UINode{
					{Type: "row", Key: "r1", Style: &rpc.UIStyle{Justify: "between", Align: "center"}, Children: []*rpc.UINode{
						{Type: "text", Key: "t1", Props: map[string]any{"text": "Status", "tone": "muted"}},
						{Type: "badge", Key: "b1", Props: map[string]any{"text": "OK", "tone": "done"}},
					}},
					{Type: "richText", Key: "rt1", Props: map[string]any{"spans": []any{
						map[string]any{"text": "Bold", "bold": true, "tone": "accent"},
						map[string]any{"text": "Mono", "mono": true},
					}}},
					{Type: "divider", Key: "d1"},
					{Type: "spacer", Key: "sp1"},
					{Type: "spinner", Key: "spin1"},
					{Type: "progress", Key: "prog1", Props: map[string]any{"value": 0.6}},
					{Type: "icon", Key: "ico1", Props: map[string]any{"name": "terminal"}},
					{Type: "icon", Key: "ico2", Props: map[string]any{"name": "nonexistent-plugin-icon"}},
					{Type: "image", Key: "img1", Props: map[string]any{"src": "missing.png", "width": 16, "height": 16}},
					{Type: "box", Key: "box1", Style: &rpc.UIStyle{Background: "$accent@0.2", Padding: []float32{4}}, Children: []*rpc.UINode{
						{ID: "run-btn", Type: "button", Key: "btn1", Props: map[string]any{"label": "Run Action", "variant": "primary"}, OnClick: "run"},
					}},
				},
			},
			{
				Type:  "settingsGroup",
				Key:   "grp1",
				Props: map[string]any{"title": "Options", "note": "Note text"},
				Children: []*rpc.UINode{
					{Type: "settingsRow", Key: "sr1", Props: map[string]any{"label": "Enable"}, Children: []*rpc.UINode{
						{ID: "tog1", Type: "toggle", Key: "tog1", Props: map[string]any{"value": true}},
					}},
					{Type: "settingsRow", Key: "sr2", Props: map[string]any{"label": "Mode"}, Children: []*rpc.UINode{
						{ID: "seg1", Type: "segmented", Key: "seg1", Props: map[string]any{"value": "fast", "options": []any{"fast", "safe"}}},
					}},
					{Type: "settingsRow", Key: "sr3", Props: map[string]any{"label": "Env"}, Children: []*rpc.UINode{
						{ID: "sel1", Type: "select", Key: "sel1", Props: map[string]any{"value": "dev", "options": []any{"dev", "prod"}}},
					}},
					{Type: "settingsRow", Key: "sr4", Props: map[string]any{"label": "Query"}, Children: []*rpc.UINode{
						{ID: "inp1", Type: "textInput", Key: "inp1", Props: map[string]any{"value": "abc", "placeholder": "Type…"}},
					}},
					{Type: "settingsRow", Key: "sr5", Props: map[string]any{"label": "Scale"}, Children: []*rpc.UINode{
						{ID: "sli1", Type: "slider", Key: "sli1", Props: map[string]any{"min": 0, "max": 10, "value": 5}},
					}},
				},
			},
			{
				Type: "scroll",
				Key:  "sc1",
				Style: &rpc.UIStyle{
					Height: 80,
				},
				Children: []*rpc.UINode{
					{
						Type:  "list",
						Key:   "l1",
						Props: map[string]any{"dividers": true},
						Children: []*rpc.UINode{
							{Type: "text", Key: "item1", Props: map[string]any{"text": "Item 1"}},
							{Type: "text", Key: "item2", Props: map[string]any{"text": "Item 2"}},
						},
					},
				},
			},
		},
	}

	rs := NewRenderState()
	tt := ui.NewTester(func(c *ui.Context) {
		Render(c, RenderContext{
			PluginID: "com.example.all",
			SlotID:   "main",
			Palette:  &theme.Light,
			State:    rs,
			OnEvent: func(compID, evType string, val any) {
				mu.Lock()
				events = append(events, compID+":"+evType)
				mu.Unlock()
			},
		}, tree)
	}, 480, 640)
	tt.Frame()

	if !tt.HasText("Summary") || !tt.HasText("Run Action") {
		t.Fatalf("missing expected texts in tree: %v", tt.Texts())
	}
	if err := tt.Click("Run Action"); err != nil {
		t.Fatalf("Click Run Action: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 || events[0] != "run-btn:run" {
		t.Fatalf("expected click callback run-btn:run, got %v", events)
	}
}

func TestResolveColorTokens(t *testing.T) {
	c, ok := ResolveColor("$accent@0.5", &theme.Light)
	if !ok || c.A < 120 || c.A > 135 {
		t.Fatalf("ResolveColor($accent@0.5) = (%+v, %v)", c, ok)
	}
	if _, ok := ResolveColor("$nonexistent", &theme.Light); ok {
		t.Fatal("expected unknown token to fail")
	}
	c2, ok := ResolveColor("rgba(10, 20, 30, 0.5)", &theme.Light)
	if !ok || c2.R != 10 || c2.G != 20 || c2.B != 30 {
		t.Fatalf("ResolveColor rgba = (%+v, %v)", c2, ok)
	}
}

func TestWebBridgeSecurityCSPAndRPCAuth(t *testing.T) {
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	pDir := filepath.Join(base, "web-plugin")
	if err := os.MkdirAll(filepath.Join(pDir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.panel"
name = "Web Panel"
version = "1.0.0"

[permissions]
scopes = ["ui.webview"]

[[ui.slots]]
id = "board"
mount = "pane"
title = "Board"
mode = "webview"
web_root = "web"
entry = "index.html"
`), 0o644)
	_ = os.WriteFile(filepath.Join(pDir, "web", "index.html"), []byte("<html><body>hello</body></html>"), 0o644)

	if _, err := st.Link(pDir, true); err != nil {
		t.Fatal(err)
	}

	fakePageValid := new(mygo.Page)
	fakePageOther := new(mygo.Page)

	type ctxKey struct{}
	bridge := NewWebBridge(st, func() *theme.Palette { return &theme.Dark }, func(ctx context.Context, pluginID, slotID, sessionID, paneID string, args map[string]string, method string, params json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"echo": method, "slot": slotID, "targetPane": args["targetPane"]})
	})
	bridge.callerPageFn = func(ctx context.Context) *mygo.Page {
		pg, _ := ctx.Value(ctxKey{}).(*mygo.Page)
		return pg
	}
	bridge.bindings[fakePageValid] = pageBinding{PluginID: "com.example.panel", SlotID: "board", Args: map[string]string{"targetPane": "p1"}, Page: fakePageValid}
	bridge.bindings[fakePageOther] = pageBinding{PluginID: "com.other.evil", SlotID: "x", Page: fakePageOther}

	// 1. Static asset + CSP header
	req := httptest.NewRequest(http.MethodGet, "hylaplugin://com.example.panel/index.html?slot=board", nil)
	rec := httptest.NewRecorder()
	bridge.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hello") {
		t.Fatalf("static asset failed: %d %s", rec.Code, rec.Body.String())
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-src 'none'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("weak CSP header: %q", csp)
	}

	// 2. Theme CSS + bridge.js
	for _, vp := range []string{"/_hyla/theme.css", "/_hyla/bridge.js"} {
		r := httptest.NewRequest(http.MethodGet, "hylaplugin://com.example.panel"+vp, nil)
		rw := httptest.NewRecorder()
		bridge.ServeHTTP(rw, r)
		if rw.Code != http.StatusOK || rw.Body.Len() == 0 {
			t.Fatalf("virtual endpoint %s failed: %d", vp, rw.Code)
		}
	}

	// 3. Path traversal blocked
	trav := httptest.NewRequest(http.MethodGet, "hylaplugin://com.example.panel/../../hyla-plugin.toml", nil)
	tRec := httptest.NewRecorder()
	bridge.ServeHTTP(tRec, trav)
	if tRec.Code == http.StatusOK {
		t.Fatalf("path traversal escaped web root: %s", tRec.Body.String())
	}

	// 4. RPC rejects missing caller page & cross-plugin caller page
	for _, tc := range []struct {
		name string
		pg   *mygo.Page
		want int
	}{
		{"nilCaller", nil, http.StatusForbidden},
		{"otherPlugin", fakePageOther, http.StatusForbidden},
		{"validCaller", fakePageValid, http.StatusOK},
	} {
		rpcReq := httptest.NewRequest(http.MethodPost, "hylaplugin://com.example.panel/_hyla/rpc", bytes.NewBufferString(`{"method":"ping"}`))
		if tc.pg != nil {
			rpcReq = rpcReq.WithContext(context.WithValue(rpcReq.Context(), ctxKey{}, tc.pg))
		}
		rr := httptest.NewRecorder()
		bridge.ServeHTTP(rr, rpcReq)
		if rr.Code != tc.want {
			t.Fatalf("rpc %s status = %d, want %d (%s)", tc.name, rr.Code, tc.want, rr.Body.String())
		}
		if tc.want == http.StatusOK && !strings.Contains(rr.Body.String(), `"targetPane":"p1"`) {
			t.Fatalf("expected forwarded binding args in rpc response, got %s", rr.Body.String())
		}
	}
}

func TestMatchesWhen(t *testing.T) {
	s := &proto.Session{ID: "s1", Project: "p1"}
	term := &proto.Pane{ID: "p1", Agent: "claude"}
	web := &proto.Pane{ID: "p2", URL: "https://example.com"}

	if !MatchesWhen("", s, term) || !MatchesWhen("pane.terminal,pane.agent,agent:claude", s, term) {
		t.Fatal("expected pane.terminal + claude condition to match")
	}
	if MatchesWhen("!pane.browser,pane.terminal", s, web) {
		t.Fatal("expected !pane.browser to reject browser pane")
	}
	if !MatchesWhen("pane.browser,session.selected", s, web) {
		t.Fatal("expected pane.browser to match")
	}
}

type fakeDaemonCaller struct {
	fn func(pluginID, method string, params any, out any) error
}

func (f *fakeDaemonCaller) CallPlugin(pluginID, method string, params any, out any) error {
	return f.fn(pluginID, method, params, out)
}

func (f *fakeDaemonCaller) PluginStatuses() []rpc.PluginStatus { return nil }

func TestCoordinatorRenderSlotThemeLocaleReloadAndDispatchRace(t *testing.T) {
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	pDir := filepath.Join(base, "demo-plugin")
	if err := os.MkdirAll(pDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.coord"
name = "Coord Demo"
version = "1.0.0"

[permissions]
scopes = ["ui.slot"]

[daemon]
command = ["echo"]

[[ui.slots]]
id = "page1"
mount = "settings.page"
title = "Coord"
mode = "declarative"
host = "daemon"
`), 0o644)
	if _, err := st.Link(pDir, true); err != nil {
		t.Fatal(err)
	}

	var updateCh = make(chan struct{}, 32)
	var msgMu sync.Mutex
	var msgs []string

	coord := NewCoordinator(st, nil, "3.16.1", func() {
		select {
		case updateCh <- struct{}{}:
		default:
		}
	})
	defer coord.Shutdown()
	coord.OnMessage = func(msg string) {
		msgMu.Lock()
		msgs = append(msgs, msg)
		msgMu.Unlock()
	}

	var releaseFirst atomic.Bool
	firstStarted := make(chan struct{}, 4)
	var renderCalls atomic.Int32
	var returnEventErr atomic.Bool

	coord.DaemonFn = func(hostKey string) DaemonCaller {
		return &fakeDaemonCaller{
			fn: func(pluginID, method string, params any, out any) error {
				switch method {
				case "ui.render":
					n := renderCalls.Add(1)
					if n == 1 && !releaseFirst.Load() {
						select {
						case firstStarted <- struct{}{}:
						default:
						}
						for !releaseFirst.Load() {
							time.Sleep(5 * time.Millisecond)
						}
					}
					p := params.(rpc.UIRenderParams)
					res := out.(*rpc.UIRenderResult)
					res.Root = &rpc.UINode{
						Type:  "text",
						Props: map[string]any{"text": fmt.Sprintf("dark=%v locale=%s call=%d", p.Dark, p.Locale, n)},
					}
					return nil
				case "ui.event":
					if returnEventErr.Load() {
						return errors.New("boom-event")
					}
					res := out.(*rpc.UIEventResult)
					res.Handled = true
					res.Root = &rpc.UINode{
						Type:  "text",
						Props: map[string]any{"text": "event-updated"},
					}
					return nil
				}
				return nil
			},
		}
	}

	if _, err := coord.Reload(); err != nil {
		t.Fatal(err)
	}
	sl, ok := coord.FindSlot("com.example.coord", "page1")
	if !ok {
		t.Fatal("missing slot")
	}

	// 1. Start slow initial render, then Reload() before it finishes, then trigger new render.
	cr := coord.RenderSlot("local", sl, rpc.UIRenderParams{SlotID: "page1", Dark: false, Locale: "en"})
	if !cr.Loading {
		t.Fatal("expected initial Loading=true")
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first render start")
	}
	if _, err := coord.Reload(); err != nil {
		t.Fatal(err)
	}
	// Release the stale pre-reload render AFTER starting post-reload render.
	cr2 := coord.RenderSlot("local", sl, rpc.UIRenderParams{SlotID: "page1", Dark: false, Locale: "en"})
	if !cr2.Loading {
		t.Fatal("expected post-reload Loading=true")
	}
	releaseFirst.Store(true)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, ok := coord.CachedRender("local", "com.example.coord", "page1", "", "")
		if ok && !got.InFlight && got.Result != nil && got.Result.Root != nil {
			txt, _ := got.Result.Root.Props["text"].(string)
			if strings.Contains(txt, "call=2") {
				break
			}
			t.Fatalf("stale pre-reload render overwrote fresh entry: %q", txt)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 2. Switching Dark or Locale triggers a re-fetch without flashing Loading=true.
	crDark := coord.RenderSlot("local", sl, rpc.UIRenderParams{SlotID: "page1", Dark: true, Locale: "zh-Hans"})
	if crDark.Loading {
		t.Fatal("theme/locale refresh should keep previous root instead of flashing Loading=true")
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := coord.CachedRender("local", "com.example.coord", "page1", "", "")
		if !got.InFlight && got.Result != nil && got.Result.Root != nil {
			txt, _ := got.Result.Root.Props["text"].(string)
			if strings.Contains(txt, "dark=true locale=zh-Hans") {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, _ := coord.CachedRender("local", "com.example.coord", "page1", "", "")
	if txt, _ := got.Result.Root.Props["text"].(string); !strings.Contains(txt, "dark=true locale=zh-Hans") {
		t.Fatalf("expected re-render with dark=true locale=zh-Hans, got %q", txt)
	}

	// 3. Concurrent DispatchEvent + CachedRender reads under -race must be race-free.
	stopRead := make(chan struct{})
	var readWG sync.WaitGroup
	readWG.Add(1)
	go func() {
		defer readWG.Done()
		for {
			select {
			case <-stopRead:
				return
			default:
				if snap, ok := coord.CachedRender("local", "com.example.coord", "page1", "", ""); ok && snap.Result != nil && snap.Result.Root != nil {
					_ = snap.Result.Root.Props["text"]
				}
			}
		}
	}()
	for range 10 {
		coord.DispatchEvent("local", sl, rpc.UIEventParams{SlotID: "page1", ComponentID: "btn", EventType: "click"})
		time.Sleep(2 * time.Millisecond)
	}
	close(stopRead)
	readWG.Wait()

	// 4. DispatchEvent error surfaces via OnMessage.
	returnEventErr.Store(true)
	coord.DispatchEvent("local", sl, rpc.UIEventParams{SlotID: "page1", ComponentID: "btn", EventType: "click"})
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msgMu.Lock()
		hasErr := len(msgs) > 0
		msgMu.Unlock()
		if hasErr {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	msgMu.Lock()
	defer msgMu.Unlock()
	if len(msgs) == 0 || !strings.Contains(msgs[0], "boom-event") {
		t.Fatalf("expected DispatchEvent error surfaced via OnMessage, got %v", msgs)
	}
}

func TestCoordinatorApplyLocaleAndBroadcastWebViewLocale(t *testing.T) {
	t.Cleanup(actions.ClearPluginActions)
	base := t.TempDir()
	st := &manifest.Store{BaseDir: base, UseKeychain: boolPtr(false)}

	pDir := filepath.Join(base, "i18n-plg")
	if err := os.MkdirAll(filepath.Join(pDir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pDir, manifest.ManifestFileName), []byte(`
api_version = 1
id = "com.example.locale"
name = "Timeline"
description = "EN desc"
version = "1.0.0"

[permissions]
scopes = ["ui.slot", "ui.webview"]

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
description = "中文描述"

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
`), 0o644)
	_ = os.WriteFile(filepath.Join(pDir, "web", "index.html"), []byte("<html>ok</html>"), 0o644)
	if _, err := st.Link(pDir, true); err != nil {
		t.Fatal(err)
	}

	loc := "zh-Hans"
	updates := 0
	coord := NewCoordinator(st, nil, "3.18.0", func() { updates++ })
	coord.LocaleFunc = func() string { return loc }
	if _, err := coord.Reload(); err != nil {
		t.Fatal(err)
	}

	act, ok := actions.Find("com.example.locale.open")
	if !ok || act.Title != "打开时间线窗格" {
		t.Fatalf("initial zh-Hans action title=%q ok=%v", act.Title, ok)
	}
	if sl, ok := coord.FindSlot("com.example.locale", "timeline"); !ok || sl.Slot.Title != "时间线" {
		t.Fatalf("initial zh-Hans slot=%+v ok=%v", sl, ok)
	}

	// Switch to Japanese and verify actions, slots, snapshot, and empty-target BroadcastLocaleChange.
	loc = "ja"
	coord.ApplyLocale("ja")
	if act, _ = actions.Find("com.example.locale.open"); act.Title != "タイムラインペインを開く" {
		t.Fatalf("ja action title=%q", act.Title)
	}
	if sl, _ := coord.FindSlot("com.example.locale", "timeline"); sl.Slot.Title != "タイムライン" {
		t.Fatalf("ja slot=%+v", sl)
	}

	// Switch back to English and verify source strings are intact (not overwritten).
	loc = "en"
	coord.ApplyLocale("en")
	if act, _ = actions.Find("com.example.locale.open"); act.Title != "Open Timeline Pane" {
		t.Fatalf("en action title=%q", act.Title)
	}
	if snap := coord.Snapshot(); snap == nil || snap.Plugins[0].Manifest.Name != "Timeline" {
		t.Fatalf("en snapshot manifest=%+v", snap)
	}
	if updates < 2 {
		t.Fatalf("expected OnUpdate callbacks on ApplyLocale, got %d", updates)
	}

	// Verify BroadcastLocaleChange pushes locale.changed to open plugin WebView pages.
	var gotEvs []WebEventPayload
	coord.Web.emitPageFn = func(pg *mygo.Page, payload WebEventPayload) error {
		gotEvs = append(gotEvs, payload)
		return nil
	}
	pg := new(mygo.Page)
	coord.Web.bindings[pg] = pageBinding{PluginID: "com.example.locale", SlotID: "timeline", PaneID: "p1", Page: pg}
	coord.Web.BroadcastLocaleChange("zh-Hans")
	if len(gotEvs) != 1 || gotEvs[0].Event != "locale.changed" || gotEvs[0].SlotID != "timeline" || !strings.Contains(gotEvs[0].Data, `"zh-Hans"`) {
		t.Fatalf("unexpected BroadcastLocaleChange payloads: %+v", gotEvs)
	}
}

func TestPictureInPictureTogglesPluginPages(t *testing.T) {
	bridge := NewWebBridge(manifest.DefaultStore(), func() *theme.Palette { return &theme.Dark }, nil)
	page := new(mygo.Page)
	bridge.bindings[page] = pageBinding{PluginID: "hyla.iptv", SlotID: "web", Page: page}

	// A page the plugin does not show fails: the window has no video to move.
	if err := bridge.PictureInPicture("com.other.evil"); err == nil {
		t.Fatalf("a plugin with no open page gave no error")
	}
	var ran bool
	var toggled []*mygo.Page
	bridge.mainFn = func(fn func()) { ran = true; fn() }
	bridge.togglePIPFn = func(pg *mygo.Page) { toggled = append(toggled, pg) }
	if err := bridge.PictureInPicture("hyla.iptv"); err != nil {
		t.Fatalf("picture in picture of an open page: %v", err)
	}
	if !ran || len(toggled) != 1 || toggled[0] != page {
		t.Fatalf("picture in picture did not toggle the page on the main thread: ran=%v toggled=%d", ran, len(toggled))
	}
}
