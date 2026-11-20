// Package rpc defines the JSON-RPC 2.0 protocol, NDJSON codec, error codes,
// and Declarative UI schema shared between Hyla hosts (hylad and the GUI app)
// and out-of-process plugins.
package rpc

import (
	"encoding/json"
	"fmt"

	"hyla/internal/plugins/manifest"
	"hyla/internal/proto"
)

const (
	JSONRPCVersion = "2.0"
	MaxFrameBytes  = 16 << 20 // 16 MiB maximum NDJSON line size
)

// Standard JSON-RPC 2.0 and Hyla plugin error codes.
const (
	ErrCodeParse                 = -32700
	ErrCodeInvalidRequest        = -32600
	ErrCodeMethodNotFound        = -32601
	ErrCodeInvalidParams         = -32602
	ErrCodeInternal              = -32603
	ErrCodePermissionDenied      = -32001
	ErrCodeRemoteControlDisabled = -32002
	ErrCodeTargetNotFound        = -32003
	ErrCodeCapabilityUnsupported = -32004
	ErrCodeTimeout               = -32005
	ErrCodePayloadTooLarge       = -32006
)

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc error (%d): %s", e.Code, e.Message)
}

func NewError(code int, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// Envelope is a JSON-RPC 2.0 request, response, or notification frame.
type Envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// InitializeParams is sent by the host to a plugin process upon startup.
type InitializeParams struct {
	HylaVersion        string         `json:"hylaVersion"`
	Host               string         `json:"host"` // "daemon" | "app"
	PluginID           string         `json:"pluginId"`
	PluginDir          string         `json:"pluginDir"`
	DataDir            string         `json:"dataDir"`
	Locale             string         `json:"locale,omitempty"`
	Appearance         string         `json:"appearance,omitempty"`
	Dark               bool           `json:"dark,omitempty"`
	GrantedPermissions []string       `json:"grantedPermissions,omitempty"`
	Settings           map[string]any `json:"settings,omitempty"`
	Capabilities       []string       `json:"capabilities,omitempty"`
}

// InitializeResult is returned by the plugin during startup handshake.
type InitializeResult struct {
	Name          string   `json:"name,omitempty"`
	Version       string   `json:"version,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Subscriptions []string `json:"subscriptions,omitempty"` // e.g. "session.state", "pane.status", "pane.osc", "theme.changed"
}

// PluginStatus summarizes runtime state for CLI and Settings › Plugins.
type PluginStatus struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	Version              string                 `json:"version"`
	Description          string                 `json:"description,omitempty"`
	Author               string                 `json:"author,omitempty"`
	Homepage             string                 `json:"homepage,omitempty"`
	Enabled              bool                   `json:"enabled"`
	Source               string                 `json:"source,omitempty"`
	LinkPath             string                 `json:"linkPath,omitempty"`
	PendingInstall       bool                   `json:"pendingInstall,omitempty"`
	Builtin              bool                   `json:"builtin,omitempty"`
	LoadError            string                 `json:"loadError,omitempty"`
	Host                 string                 `json:"host,omitempty"`  // "daemon" | "app" | "pack" | "both"
	State                string                 `json:"state,omitempty"` // "stopped" | "starting" | "running" | "degraded" | "error"
	PID                  int                    `json:"pid,omitempty"`
	CrashCount           int                    `json:"crashCount,omitempty"`
	LastError            string                 `json:"lastError,omitempty"`
	UptimeMS             int64                  `json:"uptimeMs,omitempty"`
	RequestedPermissions []string               `json:"requestedPermissions,omitempty"`
	GrantedPermissions   []string               `json:"grantedPermissions,omitempty"`
	Slots                []manifest.SlotSpec    `json:"slots,omitempty"`
	SettingsSchema       []manifest.SettingSpec `json:"settingsSchema,omitempty"`
	Actions              []manifest.ActionSpec  `json:"actions,omitempty"`
	KeepAlive            bool                   `json:"keepAlive,omitempty"`
}

// --- Host -> Plugin RPC Payloads ---

type AgentDetectParams struct {
	Pane     string `json:"pane"`
	Session  string `json:"session,omitempty"`
	Screen   string `json:"screen"`
	Title    string `json:"title,omitempty"`
	Progress string `json:"progress,omitempty"`
	Process  string `json:"process,omitempty"`
	Dir      string `json:"dir,omitempty"`
}

type AgentDetectResult struct {
	State    proto.Status `json:"state"`
	Rule     string       `json:"rule,omitempty"`
	Skip     bool         `json:"skip,omitempty"`
	Message  string       `json:"message,omitempty"`
	WaitKind string       `json:"waitKind,omitempty"`
}

type AgentReadTimelineParams struct {
	Session        string `json:"session,omitempty"`
	Pane           string `json:"pane,omitempty"`
	Agent          string `json:"agent"`
	ConversationID string `json:"conversationId,omitempty"`
	Dir            string `json:"dir,omitempty"`
}

type AgentReadTimelineResult struct {
	Timeline proto.Timeline `json:"timeline"`
}

type AgentReadChatParams struct {
	Session        string `json:"session,omitempty"`
	Pane           string `json:"pane,omitempty"`
	Agent          string `json:"agent"`
	ConversationID string `json:"conversationId,omitempty"`
	Dir            string `json:"dir,omitempty"`
}

type AgentReadChatResult struct {
	Chat proto.Chat `json:"chat"`
}

type AgentSendChatParams struct {
	Session        string `json:"session,omitempty"`
	Pane           string `json:"pane,omitempty"`
	Agent          string `json:"agent"`
	ConversationID string `json:"conversationId,omitempty"`
	Dir            string `json:"dir,omitempty"`
	Text           string `json:"text"`
	AnswerDir      string `json:"answerDir,omitempty"`
}

type AgentSendChatResult struct {
	Started        bool   `json:"started"`
	ConversationID string `json:"conversationId,omitempty"`
	Error          string `json:"error,omitempty"`
}

type AgentReadSubagentsParams struct {
	Session        string `json:"session,omitempty"`
	Pane           string `json:"pane,omitempty"`
	Agent          string `json:"agent"`
	ConversationID string `json:"conversationId,omitempty"`
	Dir            string `json:"dir,omitempty"`
}

type AgentReadSubagentsResult struct {
	Subs []proto.Subagent `json:"subs"`
	Note string           `json:"note,omitempty"`
}

type AgentResumeCommandParams struct {
	Agent          string `json:"agent"`
	ConversationID string `json:"conversationId"`
	Dir            string `json:"dir,omitempty"`
}

type AgentResumeCommandResult struct {
	Argv []string `json:"argv"`
}

type UIRenderParams struct {
	SlotID    string            `json:"slotId"`
	Mount     string            `json:"mount,omitempty"`
	SessionID string            `json:"sessionId,omitempty"`
	PaneID    string            `json:"paneId,omitempty"`
	ProjectID string            `json:"projectId,omitempty"`
	Dark      bool              `json:"dark"`
	ThemeName string            `json:"themeName,omitempty"`
	Locale    string            `json:"locale,omitempty"`
	Width     float32           `json:"width,omitempty"`
	Height    float32           `json:"height,omitempty"`
	Args      map[string]string `json:"args,omitempty"`
}

type UIRenderResult struct {
	Root      *UINode `json:"root,omitempty"`
	Title     string  `json:"title,omitempty"`
	Subtitle  string  `json:"subtitle,omitempty"`
	Badge     string  `json:"badge,omitempty"`
	Collapsed bool    `json:"collapsed,omitempty"`
	RefreshMS int     `json:"refreshMs,omitempty"`
}

type UIEventParams struct {
	SlotID      string `json:"slotId"`
	ComponentID string `json:"componentId"`
	EventType   string `json:"eventType"` // "click" | "change" | "submit" | "select"
	Value       any    `json:"value,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	PaneID      string `json:"paneId,omitempty"`
	// Locale is the language the app shows, which the words an event's
	// answer shows are said in.
	Locale string            `json:"locale,omitempty"`
	Args   map[string]string `json:"args,omitempty"`
}

type UIEventResult struct {
	Handled    bool    `json:"handled,omitempty"`
	Root       *UINode `json:"root,omitempty"`
	Invalidate bool    `json:"invalidate,omitempty"`
	Message    string  `json:"message,omitempty"`
}

type ActionInvokeParams struct {
	ActionID  string            `json:"actionId"`
	Args      map[string]string `json:"args,omitempty"`
	SessionID string            `json:"sessionId,omitempty"`
	PaneID    string            `json:"paneId,omitempty"`
	ProjectID string            `json:"projectId,omitempty"`
	Host      string            `json:"host,omitempty"`
	Locale    string            `json:"locale,omitempty"`
}

type ActionInvokeResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type WebViewRPCParams struct {
	SlotID    string            `json:"slotId"`
	Method    string            `json:"method"`
	Params    json.RawMessage   `json:"params,omitempty"`
	SessionID string            `json:"sessionId,omitempty"`
	PaneID    string            `json:"paneId,omitempty"`
	Locale    string            `json:"locale,omitempty"`
	Args      map[string]string `json:"args,omitempty"`
}

type WebViewRPCResult struct {
	Data json.RawMessage `json:"data,omitempty"`
}

type EventNotifyParams struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// --- Plugin -> Host RPC Payloads ---

type HostGetStateParams struct {
	Session string `json:"session,omitempty"`
	Pane    string `json:"pane,omitempty"`
}

type HostGetStateResult struct {
	State          proto.State `json:"state"`
	ActiveSession  string      `json:"activeSession,omitempty"`
	FocusedPane    string      `json:"focusedPane,omitempty"`
	Dark           bool        `json:"dark,omitempty"`
	Appearance     string      `json:"appearance,omitempty"`
	ThemeName      string      `json:"themeName,omitempty"`
	Remote         bool        `json:"remote,omitempty"`
	ControlAllowed bool        `json:"controlAllowed"`
}

type PaneReadScreenParams struct {
	Pane       string `json:"pane"`
	Scrollback bool   `json:"scrollback,omitempty"`
	Lines      int    `json:"lines,omitempty"`
}

type PaneReadScreenResult struct {
	Text string `json:"text"`
}

type PaneGetEnvParams struct {
	Pane string `json:"pane"`
}

type PaneGetEnvResult struct {
	Env            []string `json:"env,omitempty"`
	Dir            string   `json:"dir,omitempty"`
	Process        string   `json:"process,omitempty"`
	Agent          string   `json:"agent,omitempty"`
	AgentSessionID string   `json:"agentSessionId,omitempty"`
}

type SessionCreateResult struct {
	SessionID string `json:"sessionId"`
	PaneID    string `json:"paneId,omitempty"`
}

type PaneSendInputParams struct {
	Pane string `json:"pane"`
	Data string `json:"data"`
}

type PaneSendKeyParams struct {
	Pane string `json:"pane"`
	Key  string `json:"key"`
}

type PaneReportStatusParams struct {
	Pane     string `json:"pane"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message,omitempty"`
	Progress int    `json:"progress,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type PaneFocusParams struct {
	Session string `json:"session,omitempty"`
	Pane    string `json:"pane,omitempty"`
}

type UIInvalidateParams struct {
	SlotIDs []string `json:"slotIds,omitempty"` // empty invalidates all slots owned by plugin
}

type UIOpenPaneParams struct {
	Session       string            `json:"session,omitempty"`
	Split         string            `json:"split,omitempty"`
	Plugin        string            `json:"plugin,omitempty"`
	View          string            `json:"view"`
	Title         string            `json:"title,omitempty"`
	Args          map[string]string `json:"args,omitempty"`
	Direction     string            `json:"direction,omitempty"` // "right" | "down" | ""
	FocusExisting bool              `json:"focusExisting,omitempty"`
}

type PaneGetTimelineParams struct {
	Session string `json:"session,omitempty"`
	Pane    string `json:"pane"`
}

type PaneGetTimelineResult struct {
	Timeline proto.Timeline `json:"timeline"`
}

type UINotifyParams struct {
	Title     string `json:"title"`
	Body      string `json:"body,omitempty"`
	Kind      string `json:"kind,omitempty"` // "info" | "success" | "warning" | "error"
	SessionID string `json:"sessionId,omitempty"`
	PaneID    string `json:"paneId,omitempty"`
}

type WebViewEmitParams struct {
	SlotID string `json:"slotId"`
	Event  string `json:"event"`
	Data   any    `json:"data,omitempty"`
}

type PluginGetSettingsParams struct {
	Keys           []string `json:"keys,omitempty"`
	IncludeSecrets bool     `json:"includeSecrets,omitempty"`
}

type PluginGetSettingsResult struct {
	Settings   map[string]any    `json:"settings"`
	Secrets    map[string]string `json:"secrets,omitempty"`
	SecretsSet map[string]bool   `json:"secretsSet,omitempty"`
}

type PluginSetSettingsParams struct {
	Settings map[string]any    `json:"settings,omitempty"`
	Secrets  map[string]string `json:"secrets,omitempty"`
}

// Daemon RPC proxy wrappers (used between GUI App/CLI and hylad).
type PluginCallParams struct {
	PluginID string          `json:"pluginId"`
	Method   string          `json:"method"`
	Params   json.RawMessage `json:"params,omitempty"`
}

type PluginControlParams struct {
	PluginID string `json:"pluginId"`
}

type PluginLogsParams struct {
	PluginID string `json:"pluginId"`
	Lines    int    `json:"lines,omitempty"`
}

type PluginLogsResult struct {
	Path  string `json:"path,omitempty"`
	Text  string `json:"text"`
	Lines int    `json:"lines"`
}

// --- Layer 3A Declarative UI JSON Schema ---

// UINode represents one node in a declarative UI tree rendered natively into mygo/ui.
type UINode struct {
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type"`
	Key      string         `json:"key,omitempty"`
	Props    map[string]any `json:"props,omitempty"`
	Style    *UIStyle       `json:"style,omitempty"`
	OnClick  string         `json:"onClick,omitempty"`
	OnChange string         `json:"onChange,omitempty"`
	OnSubmit string         `json:"onSubmit,omitempty"`
	Children []*UINode      `json:"children,omitempty"`
}

type UIStyle struct {
	Width      float32   `json:"width,omitempty"`
	Height     float32   `json:"height,omitempty"`
	MinWidth   float32   `json:"minWidth,omitempty"`
	MinHeight  float32   `json:"minHeight,omitempty"`
	MaxWidth   float32   `json:"maxWidth,omitempty"`
	MaxHeight  float32   `json:"maxHeight,omitempty"`
	Padding    []float32 `json:"padding,omitempty"` // [all] | [v, h] | [t, r, b, l]
	Gap        float32   `json:"gap,omitempty"`
	Radius     float32   `json:"radius,omitempty"`
	Flex       float32   `json:"flex,omitempty"`
	Align      string    `json:"align,omitempty"`      // "start" | "center" | "end" | "stretch"
	Justify    string    `json:"justify,omitempty"`    // "start" | "center" | "end" | "between"
	Background string    `json:"background,omitempty"` // "$token[@alpha]" or "#rrggbb"
	Border     string    `json:"border,omitempty"`
	BorderSize float32   `json:"borderWidth,omitempty"`
	Color      string    `json:"color,omitempty"`
	FontSize   float32   `json:"fontSize,omitempty"`
	FontWeight int       `json:"fontWeight,omitempty"`
	Monospace  bool      `json:"monospace,omitempty"`
	Tabular    bool      `json:"tabular,omitempty"`
	Wrap       bool      `json:"wrap,omitempty"`
}
