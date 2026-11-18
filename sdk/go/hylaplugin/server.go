package hylaplugin

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Plugin describes handlers and lifecycle callbacks for an out-of-process Hyla plugin.
type Plugin struct {
	Name          string
	Version       string
	Capabilities  []string
	Subscriptions []string

	OnInitialize    func(ctx context.Context, host *Host, p InitializeParams) error
	OnDetect        func(ctx context.Context, host *Host, p AgentDetectParams) (AgentDetectResult, error)
	OnReadTimeline  func(ctx context.Context, host *Host, p AgentReadTimelineParams) (AgentReadTimelineResult, error)
	OnReadChat      func(ctx context.Context, host *Host, p AgentReadChatParams) (AgentReadChatResult, error)
	OnSendChat      func(ctx context.Context, host *Host, p AgentSendChatParams) (AgentSendChatResult, error)
	OnReadSubagents func(ctx context.Context, host *Host, p AgentReadSubagentsParams) (AgentReadSubagentsResult, error)
	OnResumeCommand func(ctx context.Context, host *Host, p AgentResumeCommandParams) (AgentResumeCommandResult, error)
	OnUIRender      func(ctx context.Context, host *Host, p UIRenderParams) (UIRenderResult, error)
	OnUIEvent       func(ctx context.Context, host *Host, p UIEventParams) (UIEventResult, error)
	OnAction        func(ctx context.Context, host *Host, p ActionInvokeParams) (ActionInvokeResult, error)
	OnWebViewRPC    func(ctx context.Context, host *Host, p WebViewRPCParams) (WebViewRPCResult, error)
	OnNotify        func(ctx context.Context, host *Host, p EventNotifyParams)
	OnShutdown      func(ctx context.Context)
}

// Host exposes strongly typed Plugin -> Host RPC calls and startup context.
type Host struct {
	p *peer

	mu   sync.RWMutex
	init InitializeParams
}

func (h *Host) InitParams() InitializeParams {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.init
}

func (h *Host) PluginID() string {
	if p := h.InitParams().PluginID; p != "" {
		return p
	}
	return os.Getenv("HYLA_PLUGIN_ID")
}

func (h *Host) PluginDir() string {
	if p := h.InitParams().PluginDir; p != "" {
		return p
	}
	return os.Getenv("HYLA_PLUGIN_DIR")
}

func (h *Host) DataDir() string {
	if p := h.InitParams().DataDir; p != "" {
		return p
	}
	return os.Getenv("HYLA_PLUGIN_DATA_DIR")
}

func (h *Host) GetState(ctx context.Context) (*HostGetStateResult, error) {
	var out HostGetStateResult
	if err := h.p.Call(ctx, "host.getState", HostGetStateParams{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *Host) ReadScreen(ctx context.Context, p PaneReadScreenParams) (*PaneReadScreenResult, error) {
	var out PaneReadScreenResult
	if err := h.p.Call(ctx, "pane.readScreen", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *Host) GetEnv(ctx context.Context, paneID string) (*PaneGetEnvResult, error) {
	var out PaneGetEnvResult
	if err := h.p.Call(ctx, "pane.getEnv", PaneGetEnvParams{Pane: paneID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *Host) CreateSession(ctx context.Context, p SessionCreateParams) (*SessionCreateResult, error) {
	var out SessionCreateResult
	if err := h.p.Call(ctx, "session.create", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *Host) SendInput(ctx context.Context, paneID, data string) error {
	return h.p.Call(ctx, "pane.sendInput", PaneSendInputParams{Pane: paneID, Data: data}, nil)
}

func (h *Host) SendKey(ctx context.Context, paneID, key string) error {
	return h.p.Call(ctx, "pane.sendKey", PaneSendKeyParams{Pane: paneID, Key: key}, nil)
}

func (h *Host) ReportStatus(ctx context.Context, p PaneReportStatusParams) error {
	return h.p.Call(ctx, "pane.reportStatus", p, nil)
}

func (h *Host) FocusPane(ctx context.Context, sessionID, paneID string) error {
	return h.p.Call(ctx, "pane.focus", PaneFocusParams{Session: sessionID, Pane: paneID}, nil)
}

func (h *Host) InvalidateUI(ctx context.Context, slotIDs ...string) error {
	return h.p.Call(ctx, "ui.invalidate", UIInvalidateParams{SlotIDs: slotIDs}, nil)
}

func (h *Host) OpenPane(ctx context.Context, p UIOpenPaneParams) error {
	return h.p.Call(ctx, "ui.openPane", p, nil)
}

func (h *Host) NotifyUI(ctx context.Context, p UINotifyParams) error {
	return h.p.Call(ctx, "ui.notify", p, nil)
}

func (h *Host) WebViewEmit(ctx context.Context, slotID, event string, data any) error {
	return h.p.Call(ctx, "webview.emit", WebViewEmitParams{SlotID: slotID, Event: event, Data: data}, nil)
}

func (h *Host) GetSettings(ctx context.Context, includeSecrets bool, keys ...string) (*PluginGetSettingsResult, error) {
	var out PluginGetSettingsResult
	if err := h.p.Call(ctx, "plugin.getSettings", PluginGetSettingsParams{
		Keys:           keys,
		IncludeSecrets: includeSecrets,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (h *Host) SetSettings(ctx context.Context, settings map[string]any, secrets map[string]string) error {
	return h.p.Call(ctx, "plugin.setSettings", PluginSetSettingsParams{
		Settings: settings,
		Secrets:  secrets,
	}, nil)
}

// Run starts the plugin over stdio (or HYLA_PLUGIN_SOCKET when transport="socket"),
// redirecting Go's standard logger to stderr when using stdio so stdout stays clean NDJSON.
func Run(plg *Plugin) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	r, w, isStdio, err := openTransport()
	if err != nil {
		return err
	}
	if isStdio {
		log.SetOutput(os.Stderr)
	}
	defer func() {
		if !isStdio {
			_ = r.Close()
		}
	}()
	return Serve(ctx, plg, r, w, nil)
}

// Serve runs the plugin JSON-RPC loop over arbitrary streams r/w until shutdown, EOF, or ctx cancel.
func Serve(ctx context.Context, plg *Plugin, r io.Reader, w io.Writer, closer io.Closer) error {
	if plg == nil {
		plg = &Plugin{}
	}
	srvCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	p := newPeer(r, w, closer)
	host := &Host{p: p}

	bindTyped(p, "initialize", func(c context.Context, req InitializeParams) (InitializeResult, error) {
		host.mu.Lock()
		host.init = req
		host.mu.Unlock()
		if plg.OnInitialize != nil {
			if err := plg.OnInitialize(c, host, req); err != nil {
				return InitializeResult{}, err
			}
		}
		return InitializeResult{
			Name:          plg.Name,
			Version:       plg.Version,
			Capabilities:  plg.Capabilities,
			Subscriptions: plg.Subscriptions,
		}, nil
	})
	p.handle("initialized", func(c context.Context, _ json.RawMessage) (any, error) {
		return nil, nil
	})
	p.handle("shutdown", func(c context.Context, _ json.RawMessage) (any, error) {
		if plg.OnShutdown != nil {
			plg.OnShutdown(c)
		}
		return map[string]any{"ok": true}, nil
	})
	p.handle("exit", func(c context.Context, _ json.RawMessage) (any, error) {
		cancel()
		return nil, nil
	})
	bindTyped(p, "agent.detect", func(c context.Context, req AgentDetectParams) (AgentDetectResult, error) {
		if plg.OnDetect == nil {
			return AgentDetectResult{Skip: true}, NewError(ErrCodeMethodNotFound, "agent.detect not implemented")
		}
		return plg.OnDetect(c, host, req)
	})
	bindTyped(p, "agent.readTimeline", func(c context.Context, req AgentReadTimelineParams) (AgentReadTimelineResult, error) {
		if plg.OnReadTimeline == nil {
			return AgentReadTimelineResult{}, NewError(ErrCodeMethodNotFound, "agent.readTimeline not implemented")
		}
		return plg.OnReadTimeline(c, host, req)
	})
	bindTyped(p, "agent.readChat", func(c context.Context, req AgentReadChatParams) (AgentReadChatResult, error) {
		if plg.OnReadChat == nil {
			return AgentReadChatResult{}, NewError(ErrCodeMethodNotFound, "agent.readChat not implemented")
		}
		return plg.OnReadChat(c, host, req)
	})
	bindTyped(p, "agent.sendChat", func(c context.Context, req AgentSendChatParams) (AgentSendChatResult, error) {
		if plg.OnSendChat == nil {
			return AgentSendChatResult{}, NewError(ErrCodeMethodNotFound, "agent.sendChat not implemented")
		}
		return plg.OnSendChat(c, host, req)
	})
	bindTyped(p, "agent.readSubagents", func(c context.Context, req AgentReadSubagentsParams) (AgentReadSubagentsResult, error) {
		if plg.OnReadSubagents == nil {
			return AgentReadSubagentsResult{}, NewError(ErrCodeMethodNotFound, "agent.readSubagents not implemented")
		}
		return plg.OnReadSubagents(c, host, req)
	})
	bindTyped(p, "agent.resumeCommand", func(c context.Context, req AgentResumeCommandParams) (AgentResumeCommandResult, error) {
		if plg.OnResumeCommand == nil {
			return AgentResumeCommandResult{}, NewError(ErrCodeMethodNotFound, "agent.resumeCommand not implemented")
		}
		return plg.OnResumeCommand(c, host, req)
	})
	bindTyped(p, "ui.render", func(c context.Context, req UIRenderParams) (UIRenderResult, error) {
		if plg.OnUIRender == nil {
			return UIRenderResult{}, NewError(ErrCodeMethodNotFound, "ui.render not implemented")
		}
		return plg.OnUIRender(c, host, req)
	})
	bindTyped(p, "ui.event", func(c context.Context, req UIEventParams) (UIEventResult, error) {
		if plg.OnUIEvent == nil {
			return UIEventResult{}, NewError(ErrCodeMethodNotFound, "ui.event not implemented")
		}
		return plg.OnUIEvent(c, host, req)
	})
	bindTyped(p, "action.invoke", func(c context.Context, req ActionInvokeParams) (ActionInvokeResult, error) {
		if plg.OnAction == nil {
			return ActionInvokeResult{}, NewError(ErrCodeMethodNotFound, "action.invoke not implemented")
		}
		return plg.OnAction(c, host, req)
	})
	bindTyped(p, "webview.rpc", func(c context.Context, req WebViewRPCParams) (WebViewRPCResult, error) {
		if plg.OnWebViewRPC == nil {
			return WebViewRPCResult{}, NewError(ErrCodeMethodNotFound, "webview.rpc not implemented")
		}
		return plg.OnWebViewRPC(c, host, req)
	})
	p.handle("event.notify", func(c context.Context, raw json.RawMessage) (any, error) {
		if plg.OnNotify != nil {
			var ev EventNotifyParams
			if json.Unmarshal(raw, &ev) == nil {
				plg.OnNotify(c, host, ev)
			}
		}
		return nil, nil
	})

	p.start(srvCtx)
	select {
	case <-srvCtx.Done():
		_ = p.Close()
		return nil
	case <-p.Done():
		return nil
	}
}

func bindTyped[Req any, Res any](p *peer, method string, fn func(context.Context, Req) (Res, error)) {
	p.handle(method, func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req Req
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &req); err != nil {
				return nil, NewError(ErrCodeInvalidParams, err.Error())
			}
		}
		return fn(ctx, req)
	})
}

// TestHarness runs plg over an in-memory net.Pipe() pair for deterministic unit tests.
type TestHarness struct {
	HostPeer *peer
	cancel   context.CancelFunc
	done     chan error
}

func NewTestHarness(plg *Plugin) *TestHarness {
	cHost, cPlug := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	hp := newPeer(cHost, cHost, cHost)
	hp.start(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, plg, cPlug, cPlug, cPlug)
	}()
	return &TestHarness{HostPeer: hp, cancel: cancel, done: done}
}

func (h *TestHarness) RegisterHostHandler(method string, fn func(ctx context.Context, params json.RawMessage) (any, error)) {
	h.HostPeer.handle(method, fn)
}

func (h *TestHarness) Call(ctx context.Context, method string, params any, out any) error {
	return h.HostPeer.Call(ctx, method, params, out)
}

func (h *TestHarness) Close() {
	h.cancel()
	_ = h.HostPeer.Close()
	<-h.done
}
