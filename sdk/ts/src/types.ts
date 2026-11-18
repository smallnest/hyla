export const JSONRPC_VERSION = "2.0" as const;
export const MAX_FRAME_BYTES = 16 * 1024 * 1024;

export const ErrorCode = {
  Parse: -32700,
  InvalidRequest: -32600,
  MethodNotFound: -32601,
  InvalidParams: -32602,
  Internal: -32603,
  PermissionDenied: -32001,
  RemoteControlDisabled: -32002,
  TargetNotFound: -32003,
  CapabilityUnsupported: -32004,
  Timeout: -32005,
  PayloadTooLarge: -32006,
} as const;

export type PermissionScope =
  | "session.read"
  | "pane.read"
  | "pane.write"
  | "pane.focus"
  | "session.create"
  | "agent.provider"
  | "timeline.read"
  | "ui.slot"
  | "ui.webview"
  | "network"
  | "secrets"
  | "notifications"
  | "filesystem.workspace";

export interface RPCErrorObject {
  code: number;
  message: string;
  data?: unknown;
}

export class PluginRPCError extends Error {
  readonly code: number;
  readonly data?: unknown;

  constructor(code: number, message: string, data?: unknown) {
    super(message);
    this.name = "PluginRPCError";
    this.code = code;
    this.data = data;
  }
}

export interface RPCEnvelope {
  jsonrpc: "2.0";
  id?: string | number;
  method?: string;
  params?: unknown;
  result?: unknown;
  error?: RPCErrorObject;
}

export interface InitializeParams {
  hylaVersion: string;
  host: "daemon" | "app";
  pluginId: string;
  pluginDir: string;
  dataDir: string;
  locale?: string;
  appearance?: string;
  dark?: boolean;
  grantedPermissions?: PermissionScope[];
  settings?: Record<string, unknown>;
  capabilities?: string[];
}

export interface InitializeResult {
  name?: string;
  version?: string;
  capabilities?: string[];
  subscriptions?: string[];
}

export type Status = "idle" | "working" | "waiting" | "failed" | "exited";

export interface Project {
  id: string;
  name: string;
  dir: string;
  default?: boolean;
}

export interface Pane {
  id: string;
  shell?: string;
  dir?: string;
  process?: string;
  processDir?: string;
  agent?: string;
  agentSessionId?: string;
  status?: Status;
  message?: string;
  kind?: string;
  progress?: number;
  unread?: boolean;
  plugin?: string;
  pluginView?: string;
  pluginTitle?: string;
  pluginArgs?: Record<string, string>;
}

export interface Session {
  id: string;
  project: string;
  name: string;
  status?: Status;
  message?: string;
  agent?: string;
  unread?: boolean;
  focus?: string;
  panes?: Pane[];
  createdAt?: number;
}

export interface State {
  projects: Project[];
  sessions: Session[];
}

export type StepState = "done" | "running" | "waiting" | "failed";

export interface TimelineStep {
  kind: "turn" | "step" | "tool" | "think" | "text" | "subagent" | string;
  title?: string;
  detail?: string;
  start?: number;
  end?: number;
  state?: StepState | string;
  children?: TimelineStep[];
}

export interface Timeline {
  session?: string;
  pane?: string;
  agent?: string;
  read?: number;
  turns?: TimelineStep[];
  note?: string;
}

export interface ChatLine {
  kind: "user" | "agent" | "think" | "tool" | string;
  text?: string;
  title?: string;
  detail?: string;
  start?: number;
  end?: number;
  state?: StepState | string;
}

export interface Chat {
  session?: string;
  pane?: string;
  agent?: string;
  read?: number;
  model?: string;
  input?: boolean;
  turn?: boolean;
  lines?: ChatLine[];
  note?: string;
}

export interface Subagent {
  id?: string;
  type?: string;
  task?: string;
  tool?: string;
  turn?: string;
  path?: string;
  start?: number;
  end?: number;
  state?: StepState | string;
}

export interface AgentDetectParams {
  pane: string;
  session?: string;
  screen: string;
  title?: string;
  progress?: string;
  process?: string;
  dir?: string;
}

export interface AgentDetectResult {
  state: Status;
  rule?: string;
  skip?: boolean;
  message?: string;
  waitKind?: string;
}

export interface AgentReadTimelineParams {
  session?: string;
  pane?: string;
  agent: string;
  conversationId?: string;
  dir?: string;
}

export interface AgentReadTimelineResult {
  timeline: Timeline;
}

export interface AgentReadChatParams {
  session?: string;
  pane?: string;
  agent: string;
  conversationId?: string;
  dir?: string;
}

export interface AgentReadChatResult {
  chat: Chat;
}

export interface AgentSendChatParams {
  session?: string;
  pane?: string;
  agent: string;
  conversationId?: string;
  dir?: string;
  text: string;
  answerDir?: string;
}

export interface AgentSendChatResult {
  started: boolean;
  conversationId?: string;
  error?: string;
}

export interface AgentReadSubagentsParams {
  session?: string;
  pane?: string;
  agent: string;
  conversationId?: string;
  dir?: string;
}

export interface AgentReadSubagentsResult {
  subs: Subagent[];
  note?: string;
}

export interface AgentResumeCommandParams {
  agent: string;
  conversationId: string;
  dir?: string;
}

export interface AgentResumeCommandResult {
  argv: string[];
}

export interface UIStyle {
  width?: number;
  height?: number;
  minWidth?: number;
  minHeight?: number;
  maxWidth?: number;
  maxHeight?: number;
  padding?: number[];
  gap?: number;
  radius?: number;
  flex?: number;
  align?: "start" | "center" | "end" | "stretch";
  justify?: "start" | "center" | "end" | "between";
  background?: string;
  border?: string;
  borderWidth?: number;
  color?: string;
  fontSize?: number;
  fontWeight?: number;
  monospace?: boolean;
  tabular?: boolean;
  wrap?: boolean;
}

export interface UINode {
  id?: string;
  type:
    | "column"
    | "row"
    | "box"
    | "spacer"
    | "divider"
    | "scroll"
    | "list"
    | "infoCard"
    | "settingsGroup"
    | "settingsRow"
    | "text"
    | "richText"
    | "badge"
    | "icon"
    | "image"
    | "spinner"
    | "progress"
    | "button"
    | "toggle"
    | "segmented"
    | "select"
    | "textInput"
    | "slider";
  key?: string;
  props?: Record<string, unknown>;
  style?: UIStyle;
  onClick?: string;
  onChange?: string;
  onSubmit?: string;
  children?: UINode[];
}

export type UIMount =
  | "info.card"
  | "side.tab"
  | "pane"
  | "sidebar.menu"
  | "files.menu"
  | "pane.menu"
  | "palette"
  | "settings.plugins"
  | "settings.page"
  | "settings.general"
  | "settings.appearance"
  | "settings.terminal"
  | "settings.shell"
  | "settings.browser"
  | "settings.notifications"
  | "settings.keyboard"
  | "settings.feishu"
  | "settings.agents"
  | "settings.sync"
  | "settings.hosts"
  | "settings.advanced";

export const UIMounts = {
  InfoCard: "info.card",
  SideTab: "side.tab",
  Pane: "pane",
  SidebarMenu: "sidebar.menu",
  FilesMenu: "files.menu",
  PaneMenu: "pane.menu",
  Palette: "palette",
  SettingsPlugins: "settings.plugins",
  SettingsPage: "settings.page",
  SettingsGeneral: "settings.general",
  SettingsAppearance: "settings.appearance",
  SettingsTerminal: "settings.terminal",
  SettingsShell: "settings.shell",
  SettingsBrowser: "settings.browser",
  SettingsNotifications: "settings.notifications",
  SettingsKeyboard: "settings.keyboard",
  SettingsFeishu: "settings.feishu",
  SettingsAgents: "settings.agents",
  SettingsSync: "settings.sync",
  SettingsHosts: "settings.hosts",
  SettingsAdvanced: "settings.advanced",
} as const satisfies Record<string, UIMount>;

export interface UIRenderParams {
  slotId: string;
  mount?: UIMount | (string & {});
  sessionId?: string;
  paneId?: string;
  projectId?: string;
  dark: boolean;
  themeName?: string;
  locale?: string;
  width?: number;
  height?: number;
  args?: Record<string, string>;
}

export interface UIRenderResult {
  root?: UINode;
  title?: string;
  subtitle?: string;
  badge?: string;
  collapsed?: boolean;
  refreshMs?: number;
}

export interface UIEventParams {
  slotId: string;
  componentId: string;
  eventType: string;
  value?: unknown;
  sessionId?: string;
  paneId?: string;
  args?: Record<string, string>;
}

export interface UIEventResult {
  handled?: boolean;
  root?: UINode;
  invalidate?: boolean;
  message?: string;
}

export interface ActionInvokeParams {
  actionId: string;
  args?: Record<string, string>;
  sessionId?: string;
  paneId?: string;
  projectId?: string;
  host?: string;
}

export interface ActionInvokeResult {
  ok: boolean;
  message?: string;
}

export interface WebViewRPCParams<T = unknown> {
  slotId: string;
  method: string;
  params?: T;
  sessionId?: string;
  paneId?: string;
}

export interface WebViewRPCResult<T = unknown> {
  data?: T;
}

export interface EventNotifyParams {
  topic: string;
  data?: unknown;
}

export interface HostGetStateResult {
  state: State;
  activeSession?: string;
  focusedPane?: string;
  dark: boolean;
  appearance?: string;
  controlAllowed: boolean;
}

export interface PaneReadScreenParams {
  pane: string;
  scrollback?: boolean;
  lines?: number;
}

export interface PaneGetEnvResult {
  env?: string[];
  dir?: string;
  process?: string;
  agent?: string;
  agentSessionId?: string;
}

export interface SessionCreateParams {
  project?: string;
  dir?: string;
  name?: string;
  command?: string[];
  split?: string;
  direction?: string;
  url?: string;
  plugin?: string;
  pluginView?: string;
  pluginTitle?: string;
  pluginArgs?: Record<string, string>;
}

export interface SessionCreateResult {
  sessionId: string;
  paneId?: string;
}

export interface PaneReportStatusParams {
  pane: string;
  status?: string;
  message?: string;
  progress?: number;
  kind?: string;
}

export interface UIOpenPaneParams {
  session?: string;
  plugin?: string;
  view: string;
  title?: string;
  args?: Record<string, string>;
  direction?: "right" | "down" | "";
}

export interface UINotifyParams {
  title: string;
  body?: string;
  kind?: "info" | "success" | "warning" | "error";
  sessionId?: string;
  paneId?: string;
}

export interface PluginGetSettingsResult {
  settings: Record<string, unknown>;
  secrets?: Record<string, string>;
  secretsSet?: Record<string, boolean>;
}
