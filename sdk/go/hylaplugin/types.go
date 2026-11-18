package hylaplugin

import (
	"encoding/json"
	"fmt"
)

const (
	JSONRPCVersion = "2.0"
	MaxFrameBytes  = 16 << 20
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

// Known permission scopes that a plugin manifest may request (matching manifest.KnownPermissions).
const (
	PermSessionRead         = "session.read"
	PermPaneRead            = "pane.read"
	PermPaneWrite           = "pane.write"
	PermPaneFocus           = "pane.focus"
	PermSessionCreate       = "session.create"
	PermAgentProvider       = "agent.provider"
	PermTimelineRead        = "timeline.read"
	PermUISlot              = "ui.slot"
	PermUIWebView           = "ui.webview"
	PermNetwork             = "network"
	PermSecrets             = "secrets"
	PermNotifications       = "notifications"
	PermFilesystemWorkspace = "filesystem.workspace"
)

// RPCError represents a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("rpc error (%d): %s", e.Code, e.Message)
}

func NewError(code int, message string) *RPCError {
	return &RPCError{Code: code, Message: message}
}

type Envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

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

type InitializeResult struct {
	Name          string   `json:"name,omitempty"`
	Version       string   `json:"version,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Subscriptions []string `json:"subscriptions,omitempty"`
}

// --- Session / Pane / Agent Domain Types ---

type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusWaiting Status = "waiting"
	StatusFailed  Status = "failed"
	StatusExited  Status = "exited"
)

type Project struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Dir     string `json:"dir"`
	Default bool   `json:"default,omitempty"`
}

type Pane struct {
	ID             string            `json:"id"`
	Shell          string            `json:"shell,omitempty"`
	Dir            string            `json:"dir,omitempty"`
	Process        string            `json:"process,omitempty"`
	ProcessDir     string            `json:"processDir,omitempty"`
	Agent          string            `json:"agent,omitempty"`
	AgentSessionID string            `json:"agentSessionId,omitempty"`
	Status         Status            `json:"status,omitempty"`
	Message        string            `json:"message,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	Progress       int               `json:"progress,omitempty"`
	Unread         bool              `json:"unread,omitempty"`
	Plugin         string            `json:"plugin,omitempty"`
	PluginView     string            `json:"pluginView,omitempty"`
	PluginTitle    string            `json:"pluginTitle,omitempty"`
	PluginArgs     map[string]string `json:"pluginArgs,omitempty"`
}

type Session struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Name      string `json:"name"`
	Status    Status `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Unread    bool   `json:"unread,omitempty"`
	Focus     string `json:"focus,omitempty"`
	Panes     []Pane `json:"panes,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

type State struct {
	Projects []Project `json:"projects"`
	Sessions []Session `json:"sessions"`
}

// StepState matches proto.StepState values ("done", "running", "waiting", "failed").
type StepState = string

const (
	StepDone    StepState = "done"
	StepRunning StepState = "running"
	StepWaiting StepState = "waiting"
	StepFailed  StepState = "failed"
)

// TimelineStep matches proto.TimelineStep JSON wire format.
type TimelineStep struct {
	Kind     string         `json:"kind"`
	Title    string         `json:"title,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Start    int64          `json:"start,omitempty"`
	End      int64          `json:"end,omitempty"`
	State    StepState      `json:"state,omitempty"`
	Children []TimelineStep `json:"children,omitempty"`
}

// Timeline matches proto.Timeline JSON wire format.
type Timeline struct {
	Session string         `json:"session,omitempty"`
	Pane    string         `json:"pane,omitempty"`
	Agent   string         `json:"agent,omitempty"`
	Read    int64          `json:"read,omitempty"`
	Turns   []TimelineStep `json:"turns,omitempty"`
	Note    string         `json:"note,omitempty"`
}

// ChatLine matches proto.ChatLine JSON wire format.
type ChatLine struct {
	Kind   string    `json:"kind"`
	Text   string    `json:"text,omitempty"`
	Title  string    `json:"title,omitempty"`
	Detail string    `json:"detail,omitempty"`
	Start  int64     `json:"start,omitempty"`
	End    int64     `json:"end,omitempty"`
	State  StepState `json:"state,omitempty"`
}

// Chat matches proto.Chat JSON wire format.
type Chat struct {
	Session string     `json:"session,omitempty"`
	Pane    string     `json:"pane,omitempty"`
	Agent   string     `json:"agent,omitempty"`
	Read    int64      `json:"read,omitempty"`
	Model   string     `json:"model,omitempty"`
	Input   bool       `json:"input,omitempty"`
	Turn    bool       `json:"turn,omitempty"`
	Lines   []ChatLine `json:"lines,omitempty"`
	Note    string     `json:"note,omitempty"`
}

// Subagent matches proto.Subagent JSON wire format.
type Subagent struct {
	ID    string    `json:"id,omitempty"`
	Type  string    `json:"type,omitempty"`
	Task  string    `json:"task,omitempty"`
	Tool  string    `json:"tool,omitempty"`
	Turn  string    `json:"turn,omitempty"`
	Path  string    `json:"path,omitempty"`
	Start int64     `json:"start,omitempty"`
	End   int64     `json:"end,omitempty"`
	State StepState `json:"state,omitempty"`
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
	State    Status `json:"state"`
	Rule     string `json:"rule,omitempty"`
	Skip     bool   `json:"skip,omitempty"`
	Message  string `json:"message,omitempty"`
	WaitKind string `json:"waitKind,omitempty"`
}

type AgentReadTimelineParams struct {
	Session        string `json:"session,omitempty"`
	Pane           string `json:"pane,omitempty"`
	Agent          string `json:"agent"`
	ConversationID string `json:"conversationId,omitempty"`
	Dir            string `json:"dir,omitempty"`
}

type AgentReadTimelineResult struct {
	Timeline Timeline `json:"timeline"`
}

type AgentReadChatParams struct {
	Session        string `json:"session,omitempty"`
	Pane           string `json:"pane,omitempty"`
	Agent          string `json:"agent"`
	ConversationID string `json:"conversationId,omitempty"`
	Dir            string `json:"dir,omitempty"`
}

type AgentReadChatResult struct {
	Chat Chat `json:"chat"`
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
	Subs []Subagent `json:"subs"`
	Note string     `json:"note,omitempty"`
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
	Mount     UIMount           `json:"mount,omitempty"`
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

// UIMount identifies a plugin UI slot mount point (matching manifest [[ui.slots]].mount).
type UIMount = string

const (
	MountInfoCard              UIMount = "info.card"
	MountSideTab               UIMount = "side.tab"
	MountPane                  UIMount = "pane"
	MountSidebarMenu           UIMount = "sidebar.menu"
	MountFilesMenu             UIMount = "files.menu"
	MountPaneMenu              UIMount = "pane.menu"
	MountPalette               UIMount = "palette"
	MountSettingsPlugins       UIMount = "settings.plugins"
	MountSettingsPage          UIMount = "settings.page"
	MountSettingsGeneral       UIMount = "settings.general"
	MountSettingsAppearance    UIMount = "settings.appearance"
	MountSettingsTerminal      UIMount = "settings.terminal"
	MountSettingsShell         UIMount = "settings.shell"
	MountSettingsBrowser       UIMount = "settings.browser"
	MountSettingsNotifications UIMount = "settings.notifications"
	MountSettingsKeyboard      UIMount = "settings.keyboard"
	MountSettingsFeishu        UIMount = "settings.feishu"
	MountSettingsAgents        UIMount = "settings.agents"
	MountSettingsSync          UIMount = "settings.sync"
	MountSettingsHosts         UIMount = "settings.hosts"
	MountSettingsAdvanced      UIMount = "settings.advanced"
)

type UIRenderResult struct {
	Root      *UINode `json:"root,omitempty"`
	Title     string  `json:"title,omitempty"`
	Subtitle  string  `json:"subtitle,omitempty"`
	Badge     string  `json:"badge,omitempty"`
	Collapsed bool    `json:"collapsed,omitempty"`
	RefreshMS int     `json:"refreshMs,omitempty"`
}

type UIEventParams struct {
	SlotID      string            `json:"slotId"`
	ComponentID string            `json:"componentId"`
	EventType   string            `json:"eventType"`
	Value       any               `json:"value,omitempty"`
	SessionID   string            `json:"sessionId,omitempty"`
	PaneID      string            `json:"paneId,omitempty"`
	Args        map[string]string `json:"args,omitempty"`
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
}

type ActionInvokeResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type WebViewRPCParams struct {
	SlotID    string          `json:"slotId"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	PaneID    string          `json:"paneId,omitempty"`
}

type WebViewRPCResult struct {
	Data json.RawMessage `json:"data,omitempty"`
}

type EventNotifyParams struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// --- Plugin -> Host RPC Payloads ---

type HostGetStateParams struct{}

type HostGetStateResult struct {
	State          State  `json:"state"`
	ActiveSession  string `json:"activeSession,omitempty"`
	FocusedPane    string `json:"focusedPane,omitempty"`
	Dark           bool   `json:"dark"`
	Appearance     string `json:"appearance,omitempty"`
	ControlAllowed bool   `json:"controlAllowed"`
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

type SessionCreateParams struct {
	Project     string            `json:"project,omitempty"`
	Dir         string            `json:"dir,omitempty"`
	Name        string            `json:"name,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Split       string            `json:"split,omitempty"`
	Direction   string            `json:"direction,omitempty"`
	URL         string            `json:"url,omitempty"`
	Plugin      string            `json:"plugin,omitempty"`
	PluginView  string            `json:"pluginView,omitempty"`
	PluginTitle string            `json:"pluginTitle,omitempty"`
	PluginArgs  map[string]string `json:"pluginArgs,omitempty"`
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
	SlotIDs []string `json:"slotIds,omitempty"`
}

type UIOpenPaneParams struct {
	Session   string            `json:"session,omitempty"`
	Plugin    string            `json:"plugin,omitempty"`
	View      string            `json:"view"`
	Title     string            `json:"title,omitempty"`
	Args      map[string]string `json:"args,omitempty"`
	Direction string            `json:"direction,omitempty"`
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

// --- Declarative UI Tree ---

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
	Padding    []float32 `json:"padding,omitempty"`
	Gap        float32   `json:"gap,omitempty"`
	Radius     float32   `json:"radius,omitempty"`
	Flex       float32   `json:"flex,omitempty"`
	Align      string    `json:"align,omitempty"`
	Justify    string    `json:"justify,omitempty"`
	Background string    `json:"background,omitempty"`
	Border     string    `json:"border,omitempty"`
	BorderSize float32   `json:"borderWidth,omitempty"`
	Color      string    `json:"color,omitempty"`
	FontSize   float32   `json:"fontSize,omitempty"`
	FontWeight int       `json:"fontWeight,omitempty"`
	Monospace  bool      `json:"monospace,omitempty"`
	Tabular    bool      `json:"tabular,omitempty"`
	Wrap       bool      `json:"wrap,omitempty"`
}
