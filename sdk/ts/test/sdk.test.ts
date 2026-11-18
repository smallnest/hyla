import { describe, expect, test } from "bun:test";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import fs from "node:fs";
import { PassThrough } from "node:stream";
import {
  attachPluginPeer,
  ErrorCode,
  PluginRPCError,
  RPCPeer,
  ui,
  UIMounts,
  type UIMount,
} from "../src/index.ts";
import { onThemeChanged, rpc as webRPC } from "../src/web.ts";

describe("@hyla/plugin-sdk UI builders", () => {
  test("builds all 23 declarative UI node types", () => {
    const root = ui.column(
      [
        ui.row([ui.badge("OK", "done"), ui.icon("terminal"), ui.image("a.png", 16, 16), ui.spinner("Loading")]),
        ui.box([ui.progress(0.75), ui.divider(), ui.spacer()], { center: true }),
        ui.scroll([ui.list([ui.text("Row 1", { key: "r1" })])]),
        ui.infoCard("Stats", [ui.richText([{ text: "Hello", bold: true }])]),
        ui.settingsGroup("Options", [
          ui.settingsRow("Run", ui.button("btn1", "Execute", { variant: "primary" })),
          ui.settingsRow("Flag", ui.toggle("tog1", true)),
          ui.settingsRow("Mode", ui.segmented("seg1", "a", [{ value: "a", label: "A" }])),
          ui.settingsRow("Choice", ui.select("sel1", "b", [{ value: "b", label: "B" }])),
          ui.settingsRow("Query", ui.textInput("inp1", "val", { placeholder: "Search" })),
          ui.settingsRow("Level", ui.slider("sli1", 0.5, 0, 1)),
        ]),
      ],
      { style: { gap: 8, padding: [12] } },
    );

    const json = JSON.stringify(root);
    expect(json).toContain('"type":"infoCard"');
    expect(json).toContain('"type":"slider"');
    expect(json).toContain('"type":"richText"');
  });

  test("exports Settings UIMount constants", () => {
    const m: UIMount = UIMounts.SettingsPage;
    expect(m).toBe("settings.page");
    expect(UIMounts.SettingsPlugins).toBe("settings.plugins");
    expect(UIMounts.SettingsGeneral).toBe("settings.general");
    expect(UIMounts.SettingsAdvanced).toBe("settings.advanced");
  });
});

describe("@hyla/plugin-sdk bidirectional JSON-RPC lifecycle", () => {
  test("handles initialize, ui.render, reverse Plugin->Host RPC, webview.rpc, and shutdown", async () => {
    const hostToPlug = new PassThrough();
    const plugToHost = new PassThrough();

    let notifiedTopic = "";
    let shutdownRan = false;
    let emittedWeb = "";

    attachPluginPeer(hostToPlug, plugToHost, {
      name: "ts-demo",
      version: "1.0.0",
      subscriptions: ["session.state"],
      onUIRender: async (p, host) => {
        const st = await host.getState();
        return {
          title: "TS Demo",
          root: ui.text(`Session ${st.activeSession ?? p.sessionId}`),
        };
      },
      onWebViewRPC: async (p, host) => {
        await host.webviewEmit(p.slotId, "refresh", { ok: true });
        return { data: { method: p.method, echo: p.params } };
      },
      onNotify: (ev) => {
        notifiedTopic = ev.topic;
      },
      onShutdown: () => {
        shutdownRan = true;
      },
    });

    const hostPeer = new RPCPeer(plugToHost, hostToPlug);
    hostPeer.handle("host.getState", () => ({
      state: { projects: [], sessions: [] },
      activeSession: "sess-ts-99",
      dark: true,
      controlAllowed: true,
    }));
    hostPeer.handle("webview.emit", (p) => {
      emittedWeb = (p as { event: string }).event;
      return { ok: true };
    });

    const initRes = await hostPeer.call<{ name: string; subscriptions: string[] }>("initialize", {
      hylaVersion: "3.16.1",
      host: "app",
      pluginId: "com.example.ts",
      pluginDir: "/tmp/p",
      dataDir: "/tmp/d",
    });
    expect(initRes.name).toBe("ts-demo");
    expect(initRes.subscriptions).toEqual(["session.state"]);

    const renderRes = await hostPeer.call<{ root: { props: { text: string } } }>("ui.render", {
      slotId: "card",
      dark: true,
    });
    expect(renderRes.root.props.text).toBe("Session sess-ts-99");

    const webRes = await hostPeer.call<{ data: { method: string } }>("webview.rpc", {
      slotId: "board",
      method: "getData",
      params: { filter: "all" },
    });
    expect(webRes.data.method).toBe("getData");
    expect(emittedWeb).toBe("refresh");

    hostPeer.notify("event.notify", { topic: "session.state" });
    await new Promise((r) => setTimeout(r, 15));
    expect(notifiedTopic).toBe("session.state");

    await expect(hostPeer.call("unknown.method", {})).rejects.toMatchObject({
      code: ErrorCode.MethodNotFound,
    });

    await hostPeer.call("shutdown", {});
    expect(shutdownRan).toBe(true);
    hostPeer.close();
  });

  test("supports socket transport auth frame", async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "hyla-ts-sdk-"));
    const sockPath = path.join(tmpDir, "app.sock");
    let gotToken = "";

    const srv = net.createServer((sock: net.Socket) => {
      let buf = "";
      sock.setEncoding("utf8");
      sock.once("data", (chunk: string | Buffer) => {
        buf += typeof chunk === "string" ? chunk : chunk.toString();
        const line = buf.split("\n")[0];
        gotToken = (JSON.parse(line) as { token: string }).token;
        const peer = new RPCPeer(sock, sock);
        peer.call<{ name: string }>("initialize", {
          hylaVersion: "3.16.1",
          host: "app",
          pluginId: "com.example.sock",
          pluginDir: tmpDir,
          dataDir: tmpDir,
        }).then(() => {
          sock.end();
        });
      });
    });

    await new Promise<void>((resolve) => srv.listen(sockPath, resolve));
    const conn = await new Promise<net.Socket>((resolve) => {
      const c = net.connect(sockPath, () => resolve(c));
    });
    conn.write(JSON.stringify({ token: "secret-abc" }) + "\n");
    await new Promise<void>((resolve) => {
      attachPluginPeer(conn, conn, { name: "sock-plugin", version: "1.0.0" }, resolve);
    });
    srv.close();
    fs.rmSync(tmpDir, { recursive: true, force: true });
    expect(gotToken).toBe("secret-abc");
  });

  test("web bridge helpers delegate to window.hyla", async () => {
    const prevWin = (globalThis as { window?: unknown }).window;
    const listeners = new Map<string, (d: unknown) => void>();
    (globalThis as { window: unknown }).window = {
      hyla: {
        rpc: async (m: string, p: unknown) => ({ m, p }),
        on: (ev: string, fn: (d: unknown) => void) => {
          listeners.set(ev, fn);
          return () => listeners.delete(ev);
        },
      },
    };
    try {
      const r = await webRPC<{ m: string }>("hello", { x: 1 });
      expect(r.m).toBe("hello");
      let sawDark = false;
      const off = onThemeChanged((ev) => {
        sawDark = ev.dark;
      });
      listeners.get("theme.changed")?.({ dark: true, themeName: "Dark" });
      expect(sawDark).toBe(true);
      off();
    } finally {
      (globalThis as { window?: unknown }).window = prevWin;
    }
  });
});
