package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func TestOpenActionSplitsAndFocusesExisting(t *testing.T) {
	h := hylaplugin.NewTestHarness(newPlugin())
	defer h.Close()

	var mu sync.Mutex
	var state hylaplugin.HostGetStateResult
	var opens []hylaplugin.UIOpenPaneParams

	h.RegisterHostHandler("host.getState", func(ctx context.Context, params json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		return state, nil
	})
	h.RegisterHostHandler("ui.openPane", func(ctx context.Context, params json.RawMessage) (any, error) {
		var req hylaplugin.UIOpenPaneParams
		_ = json.Unmarshal(params, &req)
		mu.Lock()
		opens = append(opens, req)
		mu.Unlock()
		return map[string]any{"ok": true}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Call(ctx, "initialize", hylaplugin.InitializeParams{
		PluginID: pluginID,
		Host:     "app",
		Locale:   "zh-Hans",
	}, nil); err != nil {
		t.Fatal(err)
	}

	// 1. Open from focused agent terminal pane "a".
	mu.Lock()
	state = hylaplugin.HostGetStateResult{
		ActiveSession: "s1",
		FocusedPane:   "a",
		State: hylaplugin.State{
			Sessions: []hylaplugin.Session{{
				ID:    "s1",
				Focus: "a",
				Panes: []hylaplugin.Pane{
					{ID: "a", Agent: "claude", Dir: "/repo/hyla", Status: hylaplugin.StatusWorking},
				},
			}},
		},
	}
	mu.Unlock()

	var actRes hylaplugin.ActionInvokeResult
	if err := h.Call(ctx, "action.invoke", hylaplugin.ActionInvokeParams{
		ActionID:  "open",
		SessionID: "s1",
		PaneID:    "a",
	}, &actRes); err != nil || !actRes.OK {
		t.Fatalf("action.invoke from pane a failed: %+v err=%v", actRes, err)
	}

	mu.Lock()
	if len(opens) != 1 {
		mu.Unlock()
		t.Fatalf("expected 1 openPane call, got %d", len(opens))
	}
	got1 := opens[0]
	opens = nil
	// 2. Open from inside a timeline pane whose original target "gone" closed -> falls back to pane "a".
	state = hylaplugin.HostGetStateResult{
		ActiveSession: "s1",
		FocusedPane:   "t",
		State: hylaplugin.State{
			Sessions: []hylaplugin.Session{{
				ID:    "s1",
				Focus: "t",
				Panes: []hylaplugin.Pane{
					{ID: "a", Agent: "codex", Dir: "/repo/hyla", Status: hylaplugin.StatusIdle},
					{ID: "t", Plugin: pluginID, PluginView: slotID, PluginArgs: map[string]string{"targetPane": "gone"}},
				},
			}},
		},
	}
	mu.Unlock()

	if got1.Session != "s1" || got1.Split != "a" || got1.View != slotID || got1.Title != "claude" ||
		!got1.FocusExisting || got1.Direction != "right" || got1.Args["targetPane"] != "a" {
		t.Fatalf("unexpected openPane params from pane a: %+v", got1)
	}

	if err := h.Call(ctx, "action.invoke", hylaplugin.ActionInvokeParams{
		ActionID:  "open",
		SessionID: "s1",
		PaneID:    "t",
	}, &actRes); err != nil || !actRes.OK {
		t.Fatalf("action.invoke from pane t failed: %+v err=%v", actRes, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(opens) != 1 {
		t.Fatalf("expected 1 openPane fallback call, got %d", len(opens))
	}
	got2 := opens[0]
	if got2.Split != "a" || got2.Title != "codex" || got2.Args["targetPane"] != "a" || !got2.FocusExisting {
		t.Fatalf("unexpected openPane params after target gone: %+v", got2)
	}
}

func TestWebViewSnapshotAndDirtyEvents(t *testing.T) {
	h := hylaplugin.NewTestHarness(newPlugin())
	defer h.Close()

	var mu sync.Mutex
	var state hylaplugin.HostGetStateResult
	var tlResult hylaplugin.Timeline
	var emitted []hylaplugin.WebViewEmitParams

	h.RegisterHostHandler("host.getState", func(ctx context.Context, params json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		return state, nil
	})
	h.RegisterHostHandler("pane.getTimeline", func(ctx context.Context, params json.RawMessage) (any, error) {
		var req hylaplugin.PaneGetTimelineParams
		_ = json.Unmarshal(params, &req)
		mu.Lock()
		defer mu.Unlock()
		res := tlResult
		res.Pane = req.Pane
		res.Session = req.Session
		return hylaplugin.PaneGetTimelineResult{Timeline: res}, nil
	})
	h.RegisterHostHandler("webview.emit", func(ctx context.Context, params json.RawMessage) (any, error) {
		var req hylaplugin.WebViewEmitParams
		_ = json.Unmarshal(params, &req)
		mu.Lock()
		emitted = append(emitted, req)
		mu.Unlock()
		return map[string]any{"ok": true}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Call(ctx, "initialize", hylaplugin.InitializeParams{
		PluginID: pluginID,
		Host:     "app",
		Locale:   "en",
	}, nil); err != nil {
		t.Fatal(err)
	}

	// 1. Running pane -> pollMs == 3000.
	mu.Lock()
	state = hylaplugin.HostGetStateResult{
		ActiveSession: "s1",
		State: hylaplugin.State{
			Sessions: []hylaplugin.Session{{
				ID: "s1",
				Panes: []hylaplugin.Pane{
					{ID: "a", Agent: "claude", Dir: "/repo/hyla", Status: hylaplugin.StatusWorking},
					{ID: "t", Plugin: pluginID, PluginView: slotID, PluginArgs: map[string]string{"targetPane": "a"}},
				},
			}},
		},
	}
	tlResult = hylaplugin.Timeline{
		Agent: "claude",
		Read:  5000,
		Turns: []hylaplugin.TimelineStep{{
			Kind:  "turn",
			Title: "Running task",
			Start: 1000,
			End:   0,
			State: hylaplugin.StepRunning,
		}},
	}
	mu.Unlock()

	var webRes hylaplugin.WebViewRPCResult
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID:    slotID,
		Method:    "timeline.snapshot",
		SessionID: "s1",
		PaneID:    "t",
		Args:      map[string]string{"targetPane": "a"},
	}, &webRes); err != nil {
		t.Fatalf("webview.rpc running failed: %v", err)
	}
	var snap1 snapshotResponse
	if err := json.Unmarshal(webRes.Data, &snap1); err != nil {
		t.Fatal(err)
	}
	if snap1.PollMS != 3000 || snap1.TargetPane != "a" || snap1.Timeline == nil || len(snap1.Timeline.Turns) != 1 {
		t.Fatalf("unexpected running snapshot: %+v", snap1)
	}

	// 2. Idle pane + missing targetPane falls back to remaining content pane "b" and pollMs == 30000.
	mu.Lock()
	state = hylaplugin.HostGetStateResult{
		ActiveSession: "s1",
		State: hylaplugin.State{
			Sessions: []hylaplugin.Session{{
				ID: "s1",
				Panes: []hylaplugin.Pane{
					{ID: "b", Agent: "claude", Dir: "/repo/other", Status: hylaplugin.StatusIdle},
					{ID: "t", Plugin: pluginID, PluginView: slotID, PluginArgs: map[string]string{"targetPane": "gone"}},
				},
			}},
		},
	}
	tlResult = hylaplugin.Timeline{
		Agent: "claude",
		Read:  8000,
		Turns: []hylaplugin.TimelineStep{{
			Kind:  "turn",
			Title: "Finished task",
			Start: 1000,
			End:   6000,
			State: hylaplugin.StepDone,
		}},
	}
	mu.Unlock()

	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID:    slotID,
		Method:    "timeline.snapshot",
		SessionID: "s1",
		PaneID:    "t",
	}, &webRes); err != nil {
		t.Fatalf("webview.rpc idle fallback failed: %v", err)
	}
	var snap2 snapshotResponse
	if err := json.Unmarshal(webRes.Data, &snap2); err != nil {
		t.Fatal(err)
	}
	if snap2.PollMS != 30000 || snap2.TargetPane != "b" || len(snap2.Candidates) != 1 || snap2.Candidates[0].ID != "b" {
		t.Fatalf("unexpected idle fallback snapshot: %+v", snap2)
	}

	// 3. pane.status notification triggers timeline.dirty WebView emit.
	if err := h.HostPeer.Notify("event.notify", hylaplugin.EventNotifyParams{
		Topic: "pane.status",
		Data:  json.RawMessage(`{"paneId":"b","status":"working"}`),
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		n := len(emitted)
		mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(emitted) != 1 || emitted[0].SlotID != slotID || emitted[0].Event != "timeline.dirty" {
		t.Fatalf("expected timeline.dirty emit on pane.status, got %+v", emitted)
	}
}

func TestPluginI18nLocaleActionRPCAndNotify(t *testing.T) {
	h := hylaplugin.NewTestHarness(newPlugin())
	defer h.Close()

	var mu sync.Mutex
	var lastOpen hylaplugin.UIOpenPaneParams
	state := hylaplugin.HostGetStateResult{
		ActiveSession: "s1",
		FocusedPane:   "a",
		State: hylaplugin.State{
			Sessions: []hylaplugin.Session{{
				ID:    "s1",
				Focus: "a",
				// No Agent set -> uses localized Timeline title.
				Panes: []hylaplugin.Pane{{ID: "a", Shell: "zsh", Status: hylaplugin.StatusIdle}},
			}},
		},
	}

	h.RegisterHostHandler("host.getState", func(ctx context.Context, params json.RawMessage) (any, error) {
		return state, nil
	})
	h.RegisterHostHandler("ui.openPane", func(ctx context.Context, params json.RawMessage) (any, error) {
		mu.Lock()
		_ = json.Unmarshal(params, &lastOpen)
		mu.Unlock()
		return map[string]any{"ok": true}, nil
	})
	h.RegisterHostHandler("pane.getTimeline", func(ctx context.Context, params json.RawMessage) (any, error) {
		return hylaplugin.PaneGetTimelineResult{Timeline: hylaplugin.Timeline{Pane: "a"}}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Call(ctx, "initialize", hylaplugin.InitializeParams{
		PluginID: pluginID,
		Host:     "app",
		Locale:   "en",
	}, nil); err != nil {
		t.Fatal(err)
	}

	// action.invoke with zh-Hans uses localized fallback title "时间线".
	var actRes hylaplugin.ActionInvokeResult
	if err := h.Call(ctx, "action.invoke", hylaplugin.ActionInvokeParams{
		ActionID:  "open",
		SessionID: "s1",
		PaneID:    "a",
		Locale:    "zh-Hans",
	}, &actRes); err != nil || !actRes.OK {
		t.Fatalf("action.invoke zh-Hans failed: %+v err=%v", actRes, err)
	}
	mu.Lock()
	gotTitle := lastOpen.Title
	mu.Unlock()
	if gotTitle != "时间线" {
		t.Fatalf("expected localized zh-Hans pane title \"时间线\", got %q", gotTitle)
	}

	// webview.rpc honors per-request Locale.
	var webRes hylaplugin.WebViewRPCResult
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID:    slotID,
		Method:    "timeline.snapshot",
		SessionID: "s1",
		Locale:    "ja-JP",
	}, &webRes); err != nil {
		t.Fatalf("webview.rpc ja-JP failed: %v", err)
	}
	var snap snapshotResponse
	if err := json.Unmarshal(webRes.Data, &snap); err != nil || snap.Locale != "ja" {
		t.Fatalf("expected snapshot locale \"ja\", got %+v err=%v", snap, err)
	}

	// event.notify("locale.changed") updates stored locale for subsequent calls without explicit Locale.
	if err := h.HostPeer.Notify("event.notify", hylaplugin.EventNotifyParams{
		Topic: "locale.changed",
		Data:  json.RawMessage(`{"locale":"zh-CN"}`),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
		SlotID:    slotID,
		Method:    "timeline.snapshot",
		SessionID: "s1",
	}, &webRes); err != nil {
		t.Fatalf("webview.rpc after locale.changed failed: %v", err)
	}
	if err := json.Unmarshal(webRes.Data, &snap); err != nil || snap.Locale != "zh-Hans" {
		t.Fatalf("expected updated stored locale \"zh-Hans\", got %+v err=%v", snap, err)
	}
}
