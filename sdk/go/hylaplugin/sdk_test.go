package hylaplugin

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestUIBuildersCoverAll23Nodes(t *testing.T) {
	tree := Column([]*UINode{
		Row([]*UINode{Badge("OK", "done"), Icon("terminal", 14), Image("a.png", 16, 16), Spinner("Busy")}),
		Box([]*UINode{Progress(0.5, "accent"), Divider(false), Spacer()}),
		Scroll([]*UINode{List(true, []*UINode{Text("item", WithKey("it1"))})}),
		InfoCard("Card", "v1", true, []*UINode{
			RichText([]TextSpan{{Text: "Bold", Bold: true}, {Text: "Mono", Mono: true}}),
		}),
		SettingsGroup("Group", "Note", []*UINode{
			SettingsRow("Button", "Click", Button("btn", "Run", "primary", WithOnClick("go"))),
			SettingsRow("Toggle", "Flip", Toggle("tog", true, WithOnChange("flip"))),
			SettingsRow("Segmented", "Pick", Segmented("seg", "a", []SelectOption{{Value: "a", Label: "A"}})),
			SettingsRow("Select", "Choose", Select("sel", "b", []SelectOption{{Value: "b", Label: "B"}})),
			SettingsRow("Input", "Type", TextInput("inp", "hi", "placeholder", WithOnSubmit("sub"))),
			SettingsRow("Slider", "Slide", Slider("sli", 0.5, 0, 1)),
		}),
	}, WithStyle(UIStyle{Padding: []float32{8}, Gap: 6}))

	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 200 {
		t.Fatalf("unexpectedly small JSON: %s", raw)
	}
	mounts := []UIMount{
		MountInfoCard, MountSideTab, MountPane, MountSidebarMenu, MountFilesMenu,
		MountPaneMenu, MountPalette, MountSettingsPlugins, MountSettingsPage,
		MountSettingsGeneral, MountSettingsAppearance, MountSettingsTerminal,
		MountSettingsShell, MountSettingsBrowser, MountSettingsNotifications,
		MountSettingsKeyboard, MountSettingsFeishu, MountSettingsAgents,
		MountSettingsSync, MountSettingsHosts, MountSettingsAdvanced,
	}
	if len(mounts) != 21 || MountSettingsPage != "settings.page" || MountSettingsGeneral != "settings.general" {
		t.Fatalf("unexpected UIMount constants: %v", mounts)
	}
}

func TestPluginHarnessFullRPCLifecycle(t *testing.T) {
	var mu sync.Mutex
	var notifyTopics []string
	var shutdownCalled bool

	plg := &Plugin{
		Name:    "test-plugin",
		Version: "1.2.3",
		OnInitialize: func(ctx context.Context, host *Host, p InitializeParams) error {
			return nil
		},
		OnDetect: func(ctx context.Context, host *Host, p AgentDetectParams) (AgentDetectResult, error) {
			return AgentDetectResult{State: StatusWaiting, Message: "Approve tool?"}, nil
		},
		OnReadTimeline: func(ctx context.Context, host *Host, p AgentReadTimelineParams) (AgentReadTimelineResult, error) {
			return AgentReadTimelineResult{
				Timeline: Timeline{
					Turns: []TimelineStep{{
						Kind:   "turn",
						Title:  "hello",
						Detail: "step detail",
						State:  StepDone,
						Children: []TimelineStep{{
							Kind:  "tool",
							Title: "read_file",
							State: StepDone,
						}},
					}},
				},
			}, nil
		},
		OnReadChat: func(ctx context.Context, host *Host, p AgentReadChatParams) (AgentReadChatResult, error) {
			return AgentReadChatResult{
				Chat: Chat{
					Session: "s1",
					Model:   "demo-1",
					Input:   true,
					Lines:   []ChatLine{{Kind: "agent", Text: "hi", State: StepDone}},
				},
			}, nil
		},
		OnSendChat: func(ctx context.Context, host *Host, p AgentSendChatParams) (AgentSendChatResult, error) {
			return AgentSendChatResult{Started: true, ConversationID: "c1"}, nil
		},
		OnReadSubagents: func(ctx context.Context, host *Host, p AgentReadSubagentsParams) (AgentReadSubagentsResult, error) {
			return AgentReadSubagentsResult{Subs: []Subagent{{ID: "sub1", Type: "explore", Task: "Worker", State: StepRunning}}}, nil
		},
		OnResumeCommand: func(ctx context.Context, host *Host, p AgentResumeCommandParams) (AgentResumeCommandResult, error) {
			return AgentResumeCommandResult{Argv: []string{"my-agent", "--resume", p.ConversationID}}, nil
		},
		OnUIRender: func(ctx context.Context, host *Host, p UIRenderParams) (UIRenderResult, error) {
			st, err := host.GetState(ctx)
			if err != nil {
				return UIRenderResult{}, err
			}
			return UIRenderResult{
				Title: "Status",
				Root:  Text(st.ActiveSession),
			}, nil
		},
		OnUIEvent: func(ctx context.Context, host *Host, p UIEventParams) (UIEventResult, error) {
			_ = host.InvalidateUI(ctx, p.SlotID)
			return UIEventResult{Handled: true, Root: Text("Clicked")}, nil
		},
		OnAction: func(ctx context.Context, host *Host, p ActionInvokeParams) (ActionInvokeResult, error) {
			return ActionInvokeResult{OK: true, Message: "ran " + p.ActionID}, nil
		},
		OnWebViewRPC: func(ctx context.Context, host *Host, p WebViewRPCParams) (WebViewRPCResult, error) {
			_ = host.WebViewEmit(ctx, p.SlotID, "pong", map[string]any{"ok": true})
			return WebViewRPCResult{Data: json.RawMessage(`{"reply":"ok"}`)}, nil
		},
		OnNotify: func(ctx context.Context, host *Host, p EventNotifyParams) {
			mu.Lock()
			notifyTopics = append(notifyTopics, p.Topic)
			mu.Unlock()
		},
		OnShutdown: func(ctx context.Context) {
			mu.Lock()
			shutdownCalled = true
			mu.Unlock()
		},
	}

	h := NewTestHarness(plg)
	defer h.Close()

	var invalidates int
	var emitted []string
	h.RegisterHostHandler("host.getState", func(ctx context.Context, params json.RawMessage) (any, error) {
		return HostGetStateResult{ActiveSession: "sess-42", ControlAllowed: true}, nil
	})
	h.RegisterHostHandler("ui.invalidate", func(ctx context.Context, params json.RawMessage) (any, error) {
		mu.Lock()
		invalidates++
		mu.Unlock()
		return map[string]any{"ok": true}, nil
	})
	h.RegisterHostHandler("webview.emit", func(ctx context.Context, params json.RawMessage) (any, error) {
		var ep WebViewEmitParams
		_ = json.Unmarshal(params, &ep)
		mu.Lock()
		emitted = append(emitted, ep.Event)
		mu.Unlock()
		return map[string]any{"ok": true}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var initRes InitializeResult
	if err := h.Call(ctx, "initialize", InitializeParams{PluginID: "com.example.test", Host: "daemon"}, &initRes); err != nil {
		t.Fatal(err)
	}
	if initRes.Name != "test-plugin" {
		t.Fatalf("unexpected init result: %+v", initRes)
	}

	var det AgentDetectResult
	if err := h.Call(ctx, "agent.detect", AgentDetectParams{Pane: "p1", Screen: "Approve tool?"}, &det); err != nil || det.State != StatusWaiting {
		t.Fatalf("detect failed: %+v err=%v", det, err)
	}
	var tl AgentReadTimelineResult
	if err := h.Call(ctx, "agent.readTimeline", AgentReadTimelineParams{Agent: "x"}, &tl); err != nil || len(tl.Timeline.Turns) != 1 || tl.Timeline.Turns[0].Kind != "turn" || len(tl.Timeline.Turns[0].Children) != 1 {
		t.Fatalf("timeline failed: %+v err=%v", tl, err)
	}
	var ch AgentReadChatResult
	if err := h.Call(ctx, "agent.readChat", AgentReadChatParams{Agent: "x"}, &ch); err != nil || len(ch.Chat.Lines) != 1 || ch.Chat.Lines[0].Kind != "agent" {
		t.Fatalf("readChat failed: %+v err=%v", ch, err)
	}
	var sch AgentSendChatResult
	if err := h.Call(ctx, "agent.sendChat", AgentSendChatParams{Agent: "x", Text: "go"}, &sch); err != nil || !sch.Started {
		t.Fatalf("sendChat failed: %+v err=%v", sch, err)
	}
	var sub AgentReadSubagentsResult
	if err := h.Call(ctx, "agent.readSubagents", AgentReadSubagentsParams{Agent: "x"}, &sub); err != nil || len(sub.Subs) != 1 || sub.Subs[0].Task != "Worker" {
		t.Fatalf("readSubagents failed: %+v err=%v", sub, err)
	}
	var resCmd AgentResumeCommandResult
	if err := h.Call(ctx, "agent.resumeCommand", AgentResumeCommandParams{Agent: "x", ConversationID: "123"}, &resCmd); err != nil || len(resCmd.Argv) != 3 {
		t.Fatalf("resumeCommand failed: %+v err=%v", resCmd, err)
	}
	var uiRes UIRenderResult
	if err := h.Call(ctx, "ui.render", UIRenderParams{SlotID: "card"}, &uiRes); err != nil || uiRes.Root.Props["text"] != "sess-42" {
		t.Fatalf("ui.render failed: %+v err=%v", uiRes, err)
	}
	var evRes UIEventResult
	if err := h.Call(ctx, "ui.event", UIEventParams{SlotID: "card", ComponentID: "btn", EventType: "click"}, &evRes); err != nil || !evRes.Handled {
		t.Fatalf("ui.event failed: %+v err=%v", evRes, err)
	}
	var actRes ActionInvokeResult
	if err := h.Call(ctx, "action.invoke", ActionInvokeParams{ActionID: "sync"}, &actRes); err != nil || actRes.Message != "ran sync" {
		t.Fatalf("action.invoke failed: %+v err=%v", actRes, err)
	}
	var webRes WebViewRPCResult
	if err := h.Call(ctx, "webview.rpc", WebViewRPCParams{SlotID: "web", Method: "ping"}, &webRes); err != nil || string(webRes.Data) != `{"reply":"ok"}` {
		t.Fatalf("webview.rpc failed: %s err=%v", webRes.Data, err)
	}
	_ = h.HostPeer.Notify("event.notify", EventNotifyParams{Topic: "session.state"})
	time.Sleep(30 * time.Millisecond)

	if err := h.Call(ctx, "shutdown", nil, nil); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if invalidates != 1 || len(emitted) != 1 || emitted[0] != "pong" || len(notifyTopics) != 1 || !shutdownCalled {
		t.Fatalf("unexpected callback state: invalidates=%d emitted=%v notify=%v shutdown=%v",
			invalidates, emitted, notifyTopics, shutdownCalled)
	}
}
