import net from "node:net";
import type { Readable, Writable } from "node:stream";
import {
  ErrorCode,
  JSONRPC_VERSION,
  MAX_FRAME_BYTES,
  PluginRPCError,
  type ActionInvokeParams,
  type ActionInvokeResult,
  type AgentDetectParams,
  type AgentDetectResult,
  type AgentReadChatParams,
  type AgentReadChatResult,
  type AgentReadSubagentsParams,
  type AgentReadSubagentsResult,
  type AgentReadTimelineParams,
  type AgentReadTimelineResult,
  type AgentResumeCommandParams,
  type AgentResumeCommandResult,
  type AgentSendChatParams,
  type AgentSendChatResult,
  type EventNotifyParams,
  type HostGetStateResult,
  type InitializeParams,
  type InitializeResult,
  type PaneGetEnvResult,
  type PaneReadScreenParams,
  type PaneReportStatusParams,
  type PluginGetSettingsResult,
  type RPCEnvelope,
  type SessionCreateParams,
  type SessionCreateResult,
  type UIEventParams,
  type UIEventResult,
  type UINotifyParams,
  type UIOpenPaneParams,
  type UIRenderParams,
  type UIRenderResult,
  type WebViewRPCParams,
  type WebViewRPCResult,
} from "./types.ts";

type MethodHandler = (params: unknown, host: HostClient) => Promise<unknown> | unknown;

export interface PluginHandlers {
  name?: string;
  version?: string;
  capabilities?: string[];
  subscriptions?: string[];

  onInitialize?: (params: InitializeParams, host: HostClient) => Promise<void> | void;
  onDetect?: (params: AgentDetectParams, host: HostClient) => Promise<AgentDetectResult> | AgentDetectResult;
  onReadTimeline?: (
    params: AgentReadTimelineParams,
    host: HostClient,
  ) => Promise<AgentReadTimelineResult> | AgentReadTimelineResult;
  onReadChat?: (params: AgentReadChatParams, host: HostClient) => Promise<AgentReadChatResult> | AgentReadChatResult;
  onSendChat?: (params: AgentSendChatParams, host: HostClient) => Promise<AgentSendChatResult> | AgentSendChatResult;
  onReadSubagents?: (
    params: AgentReadSubagentsParams,
    host: HostClient,
  ) => Promise<AgentReadSubagentsResult> | AgentReadSubagentsResult;
  onResumeCommand?: (
    params: AgentResumeCommandParams,
    host: HostClient,
  ) => Promise<AgentResumeCommandResult> | AgentResumeCommandResult;
  onUIRender?: (params: UIRenderParams, host: HostClient) => Promise<UIRenderResult> | UIRenderResult;
  onUIEvent?: (params: UIEventParams, host: HostClient) => Promise<UIEventResult> | UIEventResult;
  onAction?: (params: ActionInvokeParams, host: HostClient) => Promise<ActionInvokeResult> | ActionInvokeResult;
  onWebViewRPC?: (params: WebViewRPCParams, host: HostClient) => Promise<WebViewRPCResult> | WebViewRPCResult;
  onNotify?: (params: EventNotifyParams, host: HostClient) => Promise<void> | void;
  onShutdown?: () => Promise<void> | void;
}

export class RPCPeer {
  private nextID = 1;
  private pending = new Map<
    string,
    { resolve: (val: unknown) => void; reject: (err: Error) => void }
  >();
  private handlers = new Map<string, MethodHandler>();
  private buffer = "";
  private droppingOversized = false;
  private closed = false;
  private hostClient: HostClient;
  private readonly handleData = (chunk: string): void => this.onChunk(chunk);
  private readonly handleEnd = (): void => this.close(new Error("stream closed"));
  private readonly handleError = (err: unknown): void =>
    this.close(err instanceof Error ? err : new Error(String(err)));

  constructor(
    private readonly reader: Readable,
    private readonly writer: Writable,
    private readonly onExit?: () => void,
  ) {
    this.hostClient = new HostClient(this);
    this.reader.setEncoding("utf8");
    this.reader.on("data", this.handleData);
    this.reader.on("end", this.handleEnd);
    this.reader.on("error", this.handleError);
  }

  get host(): HostClient {
    return this.hostClient;
  }

  handle(method: string, fn: MethodHandler): void {
    this.handlers.set(method, fn);
  }

  call<T = unknown>(method: string, params?: unknown, timeoutMs = 15000): Promise<T> {
    if (this.closed) {
      return Promise.reject(new Error("RPC peer closed"));
    }
    const id = this.nextID++;
    const key = String(id);
    const frame: RPCEnvelope = {
      jsonrpc: JSONRPC_VERSION,
      id,
      method,
      params,
    };
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(key);
        reject(new PluginRPCError(ErrorCode.Timeout, `RPC ${method} timed out`));
      }, timeoutMs);
      this.pending.set(key, {
        resolve: (v) => {
          clearTimeout(timer);
          resolve(v as T);
        },
        reject: (err) => {
          clearTimeout(timer);
          reject(err);
        },
      });
      try {
        this.writeFrame(frame);
      } catch (err) {
        clearTimeout(timer);
        this.pending.delete(key);
        reject(err instanceof Error ? err : new Error(String(err)));
      }
    });
  }

  notify(method: string, params?: unknown): void {
    this.writeFrame({ jsonrpc: JSONRPC_VERSION, method, params });
  }

  close(reason = new Error("closed")): void {
    if (this.closed) return;
    this.closed = true;
    this.buffer = "";
    this.droppingOversized = false;
    this.reader.off?.("data", this.handleData);
    this.reader.off?.("end", this.handleEnd);
    this.reader.off?.("error", this.handleError);
    const r = this.reader as Readable & { pause?: () => void; unref?: () => void; destroy?: () => void };
    r.pause?.();
    r.unref?.();
    if (this.reader !== process.stdin) {
      r.destroy?.();
    }
    for (const [k, p] of this.pending) {
      this.pending.delete(k);
      p.reject(reason);
    }
    this.onExit?.();
  }

  private writeFrame(env: RPCEnvelope): void {
    if (this.closed) return;
    const raw = JSON.stringify(env) + "\n";
    if (Buffer.byteLength(raw, "utf8") > MAX_FRAME_BYTES) {
      throw new PluginRPCError(ErrorCode.PayloadTooLarge, "frame exceeds 16 MiB limit");
    }
    this.writer.write(raw);
  }

  private onChunk(chunk: string): void {
    if (this.closed) return;
    this.buffer += chunk;
    while (true) {
      if (this.droppingOversized) {
        const nl = this.buffer.indexOf("\n");
        if (nl < 0) {
          this.buffer = "";
          return;
        }
        this.buffer = this.buffer.slice(nl + 1);
        this.droppingOversized = false;
        continue;
      }
      const idx = this.buffer.indexOf("\n");
      if (idx < 0) break;
      const rawLine = this.buffer.slice(0, idx);
      this.buffer = this.buffer.slice(idx + 1);
      if (Buffer.byteLength(rawLine, "utf8") > MAX_FRAME_BYTES) {
        this.writeFrame({
          jsonrpc: JSONRPC_VERSION,
          error: { code: ErrorCode.PayloadTooLarge, message: "incoming line exceeds 16 MiB limit" },
        });
        continue;
      }
      const line = rawLine.trim();
      if (!line) continue;
      this.onLine(line);
    }
    if (!this.droppingOversized && Buffer.byteLength(this.buffer, "utf8") > MAX_FRAME_BYTES) {
      this.buffer = "";
      this.droppingOversized = true;
      this.writeFrame({
        jsonrpc: JSONRPC_VERSION,
        error: { code: ErrorCode.PayloadTooLarge, message: "incoming line exceeds 16 MiB limit" },
      });
    }
  }

  private onLine(line: string): void {
    let msg: RPCEnvelope;
    try {
      msg = JSON.parse(line) as RPCEnvelope;
    } catch {
      this.writeFrame({
        jsonrpc: JSONRPC_VERSION,
        error: { code: ErrorCode.Parse, message: "invalid JSON" },
      });
      return;
    }
    if (!msg.method && msg.id !== undefined) {
      const p = this.pending.get(String(msg.id));
      if (p) {
        this.pending.delete(String(msg.id));
        if (msg.error) {
          p.reject(new PluginRPCError(msg.error.code, msg.error.message, msg.error.data));
        } else {
          p.resolve(msg.result);
        }
      }
      return;
    }
    if (!msg.method) return;
    void this.dispatch(msg);
  }

  private async dispatch(msg: RPCEnvelope): Promise<void> {
    const isReq = msg.id !== undefined;
    const handler = this.handlers.get(msg.method!);
    if (!handler) {
      if (isReq) {
        this.writeFrame({
          jsonrpc: JSONRPC_VERSION,
          id: msg.id,
          error: { code: ErrorCode.MethodNotFound, message: `method "${msg.method}" not found` },
        });
      }
      return;
    }
    try {
      const result = await handler(msg.params, this.hostClient);
      if (isReq) {
        this.writeFrame({
          jsonrpc: JSONRPC_VERSION,
          id: msg.id,
          result: result ?? null,
        });
      }
    } catch (err) {
      if (!isReq) return;
      if (err instanceof PluginRPCError) {
        this.writeFrame({
          jsonrpc: JSONRPC_VERSION,
          id: msg.id,
          error: { code: err.code, message: err.message, data: err.data },
        });
      } else {
        this.writeFrame({
          jsonrpc: JSONRPC_VERSION,
          id: msg.id,
          error: {
            code: ErrorCode.Internal,
            message: err instanceof Error ? err.message : String(err),
          },
        });
      }
    }
  }
}

export class HostClient {
  private initParams: InitializeParams | null = null;

  constructor(private readonly peer: RPCPeer) {}

  setInitParams(p: InitializeParams): void {
    this.initParams = p;
  }

  getInitParams(): InitializeParams | null {
    return this.initParams;
  }

  get pluginId(): string {
    return this.initParams?.pluginId ?? process.env.HYLA_PLUGIN_ID ?? "";
  }

  get pluginDir(): string {
    return this.initParams?.pluginDir ?? process.env.HYLA_PLUGIN_DIR ?? "";
  }

  get dataDir(): string {
    return this.initParams?.dataDir ?? process.env.HYLA_PLUGIN_DATA_DIR ?? "";
  }

  getState(): Promise<HostGetStateResult> {
    return this.peer.call("host.getState", {});
  }

  readScreen(params: PaneReadScreenParams): Promise<{ text: string }> {
    return this.peer.call("pane.readScreen", params);
  }

  getEnv(pane: string): Promise<PaneGetEnvResult> {
    return this.peer.call("pane.getEnv", { pane });
  }

  createSession(params: SessionCreateParams): Promise<SessionCreateResult> {
    return this.peer.call("session.create", params);
  }

  sendInput(pane: string, data: string): Promise<void> {
    return this.peer.call("pane.sendInput", { pane, data }).then(() => undefined);
  }

  sendKey(pane: string, key: string): Promise<void> {
    return this.peer.call("pane.sendKey", { pane, key }).then(() => undefined);
  }

  reportStatus(params: PaneReportStatusParams): Promise<void> {
    return this.peer.call("pane.reportStatus", params).then(() => undefined);
  }

  focusPane(opts: { session?: string; pane?: string }): Promise<void> {
    return this.peer.call("pane.focus", opts).then(() => undefined);
  }

  invalidateUI(slotIds?: string[]): Promise<void> {
    return this.peer.call("ui.invalidate", { slotIds }).then(() => undefined);
  }

  openPane(params: UIOpenPaneParams): Promise<void> {
    return this.peer.call("ui.openPane", params).then(() => undefined);
  }

  notifyUI(params: UINotifyParams): Promise<void> {
    return this.peer.call("ui.notify", params).then(() => undefined);
  }

  webviewEmit(slotId: string, event: string, data?: unknown): Promise<void> {
    return this.peer.call("webview.emit", { slotId, event, data }).then(() => undefined);
  }

  getSettings(opts?: { keys?: string[]; includeSecrets?: boolean }): Promise<PluginGetSettingsResult> {
    return this.peer.call("plugin.getSettings", opts ?? {});
  }

  setSettings(params: { settings?: Record<string, unknown>; secrets?: Record<string, string> }): Promise<void> {
    return this.peer.call("plugin.setSettings", params).then(() => undefined);
  }
}

export function attachPluginPeer(
  reader: Readable,
  writer: Writable,
  handlers: PluginHandlers,
  onExit?: () => void,
): RPCPeer {
  const peer = new RPCPeer(reader, writer, onExit);

  peer.handle("initialize", async (raw, host) => {
    const p = (raw ?? {}) as InitializeParams;
    host.setInitParams(p);
    await handlers.onInitialize?.(p, host);
    return {
      name: handlers.name,
      version: handlers.version,
      capabilities: handlers.capabilities,
      subscriptions: handlers.subscriptions,
    } satisfies InitializeResult;
  });

  peer.handle("initialized", () => null);
  peer.handle("shutdown", async () => {
    await handlers.onShutdown?.();
    return { ok: true };
  });
  peer.handle("exit", () => {
    peer.close();
    return null;
  });

  if (handlers.onDetect) {
    peer.handle("agent.detect", (p, h) => handlers.onDetect!(p as AgentDetectParams, h));
  }
  if (handlers.onReadTimeline) {
    peer.handle("agent.readTimeline", (p, h) => handlers.onReadTimeline!(p as AgentReadTimelineParams, h));
  }
  if (handlers.onReadChat) {
    peer.handle("agent.readChat", (p, h) => handlers.onReadChat!(p as AgentReadChatParams, h));
  }
  if (handlers.onSendChat) {
    peer.handle("agent.sendChat", (p, h) => handlers.onSendChat!(p as AgentSendChatParams, h));
  }
  if (handlers.onReadSubagents) {
    peer.handle("agent.readSubagents", (p, h) => handlers.onReadSubagents!(p as AgentReadSubagentsParams, h));
  }
  if (handlers.onResumeCommand) {
    peer.handle("agent.resumeCommand", (p, h) => handlers.onResumeCommand!(p as AgentResumeCommandParams, h));
  }
  if (handlers.onUIRender) {
    peer.handle("ui.render", (p, h) => handlers.onUIRender!(p as UIRenderParams, h));
  }
  if (handlers.onUIEvent) {
    peer.handle("ui.event", (p, h) => handlers.onUIEvent!(p as UIEventParams, h));
  }
  if (handlers.onAction) {
    peer.handle("action.invoke", (p, h) => handlers.onAction!(p as ActionInvokeParams, h));
  }
  if (handlers.onWebViewRPC) {
    peer.handle("webview.rpc", (p, h) => handlers.onWebViewRPC!(p as WebViewRPCParams, h));
  }
  peer.handle("event.notify", (p, h) => {
    return handlers.onNotify?.(p as EventNotifyParams, h);
  });

  return peer;
}

export async function runPlugin(handlers: PluginHandlers): Promise<void> {
  const socketPath = process.env.HYLA_PLUGIN_SOCKET;
  if (socketPath) {
    const token = process.env.HYLA_PLUGIN_TOKEN ?? "";
    const conn = await new Promise<net.Socket>((resolve, reject) => {
      const s = net.connect(socketPath, () => resolve(s));
      s.once("error", reject);
    });
    conn.write(JSON.stringify({ token }) + "\n");
    await new Promise<void>((resolve) => {
      attachPluginPeer(conn, conn, handlers, resolve);
    });
    return;
  }

  await new Promise<void>((resolve) => {
    attachPluginPeer(process.stdin, process.stdout, handlers, resolve);
  });
}
