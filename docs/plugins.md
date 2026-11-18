---
title: Plugins
description: Extend Hyla with declarative asset packs, out-of-process JSON-RPC 2.0 plugins, native declarative UI, and sandboxed WebView panes.
---

# Hyla Plugin System

Hyla's plugin system allows developers to extend terminal agent detection, UI chrome, command palettes, custom panes, and external chat channels without recompiling Hyla itself.

## Architecture (3 Layers)

Plugins combine one or more progressive layers depending on what they need:

1. **Layer 1 — Declarative Packs (`hyla-plugin.toml` + static assets)**  
   Zero-process, hot-reloadable bundles loaded directly by Hyla:
   - Agent detection rules (`detect/*.toml`)
   - Agent icons/marks (`marks/*.svg|png|webp`)
   - Terminal color themes (`themes/*.ghostty`)
   - Background window washes (`washes/*.toml`)
   - Desktop companion pets (`pets/*` with `pet.json`, `FrameW=192`, `FrameH=208`, `Columns=8`)
   - Coding agent hooks & skills (`[[hooks]]`, `[[skills]]`)
   - Static actions & keybindings (`[[actions]]`)
2. **Layer 2 — Out-of-Process JSON-RPC 2.0 Split Hosts**  
   Long-running or on-demand plugin processes communicating via **NDJSON (max 16 MiB per line)** over **stdio** or a **local Unix domain socket / Windows named pipe**:
   - **Daemon Host (`hylad` / `host = "daemon"`)**: handles coding agent detection/timelines/chats/subagents/resume commands, channel drivers, and host state queries across sessions and headless servers (`hyla-server`).
   - **App Host (`Hyla.app` / `host = "app"`)**: handles local GUI interactions, app actions, native Declarative UI slots, and WebView RPCs.
3. **Layer 3 — UI Payloads (Declarative UI & Sandboxed WebView)**  
   - **3A Declarative UI (`mode = "declarative"`)**: JSON UI trees rendered natively with `mygo/ui` and themed with Hyla's live palette tokens (`$text`, `$accent`, `$card`, etc.).
   - **3B Sandboxed WebView (`mode = "webview"`)**: HTML/JS/CSS views mounted on `hylaplugin://<plugin-id>/...` inside an isolated `Untrusted` WebView session (`"plugin:" + id`) with injected `/_hyla/theme.css` and `/_hyla/bridge.js`.

```text
+-------------------------------------------------------------------+
| Hyla Desktop App (window)                                         |
|  - Mounts: info.card, side.tab, pane, menus, palette, Settings    |
|  - Declarative UI Renderer (uihost -> mygo/ui)                    |
|  - Sandboxed WebView (hylaplugin://<id>/... -> Untrusted session) |
+---------------------------+-----------------------+---------------+
                            | Local/Remote RPC      | JSON-RPC 2.0
                            | (plugins.v1 cap)      | (App Host)
                            v                       v
+----------------------------------------+   +----------------------+
| hylad (Daemon Host)                    |   | App Plugin Process   |
|  - Agent detection, chat, timeline     |   | (stdio or socket)    |
|  - ChannelDriver (a chat channel)      |   +----------------------+
|  - keep_alive & idle-exit supervision  |
+---------------------------+------------+
                            | JSON-RPC 2.0 (NDJSON / stdio or socket)
                            v
                 +-----------------------+
                 | Daemon Plugin Process |
                 +-----------------------+
```

---

## Plugin IDs & Storage Layout

### Plugin Identifier Rules

Every plugin ID MUST be a lowercase DNS-safe reverse-domain name with at least two segments separated by dots (no underscores or uppercase characters):

```text
^[a-z0-9][a-z0-9-]{0,61}[a-z0-9](\.[a-z0-9][a-z0-9-]{0,61}[a-z0-9])+$
```

Valid examples: `com.example.hello-agent`, `org.acme.git-radar`, `hyla.magpie`.  
Invalid examples: `my_plugin`, `hello`, `Com.Example.Test`.

### Isolated Directory Layout

All plugin files reside under `<HYLA_HOME>` (default `~/.hyla`) and never pollute Hyla's main `config` validation:

| Path | Purpose | Synced across machines? |
| --- | --- | --- |
| `~/.hyla/plugins/<id>/` | Unpacked plugin directory (`hyla-plugin.toml`, assets, binaries) | No (missing targets marked `pendingInstall`) |
| `~/.hyla/plugins.json` | Installed/linked/built-in registry, enabled state, SHA-256, granted permissions | Yes (`linkPath` stripped) |
| `~/.hyla/plugin-config/<id>.json` | Non-secret user configuration matching `[[settings]]` | Yes |
| `~/.hyla/plugin-secrets/` | Secret values (`0600` fallback for headless `hyla-server` when OS keychain is disabled/unavailable; desktop stores via `mygo.Secrets` service `"hyla-plugin:" + id`) | **Never** |
| `~/.hyla/plugin-data/<id>/` | Plugin scratch/cache directory passed as `dataDir` / `HYLA_PLUGIN_DATA_DIR` | No |
| `~/.hyla/logs/plugins/<id>.log` | Rotating stdout/stderr/lifecycle log (4 MiB rotation) | No |

---

## Manifest Specification (`hyla-plugin.toml`)

```toml
api_version = 1
id = "com.example.demo"
name = "Demo Plugin"
version = "0.2.0"
description = "Short description shown in Settings > Plugins"
author = "Your Name"
homepage = "https://example.com/demo"

[engines]
hyla = ">=3.0.0"

[permissions]
scopes = ["ui.slot", "session.read"]

[daemon]
command = ["./bin/demo-daemon"]
transport = "stdio"          # "stdio" (default) | "socket"
auto_start = false           # start lazily on first RPC unless true
keep_alive = false           # true keeps hylad running even when idle
activation_events = []
min_restart_ms = 500
max_crashes_per_minute = 5   # triggers degraded state if exceeded

[app]
command = ["bun", "run", "src/app.ts"]
transport = "stdio"          # "stdio" | "socket"
auto_start = false

[[agents]]
id = "my-agent"
name = "My Agent"
detect = "detect/my-agent.toml"
mark = "marks/my-agent.svg"
processes = ["my-agent"]
timeline = true
chat = true
subagents = false
resume_argv = ["my-agent", "--resume", "{session_id}"]

[[actions]]
id = "open"
title = "Open Demo View"
shortcut = "Cmd+Shift+D"
menu = "Plugins"
palette = true
target = "open_pane"         # "app" | "daemon" | "open_pane"
view = "main"                # slot ID when target = "open_pane"

[[ui.slots]]
id = "main"
mount = "pane"               # info.card | side.tab | pane | sidebar.menu | files.menu | pane.menu | palette | settings.plugins | settings.page | settings.<page>
title = "Demo"
mode = "declarative"         # "declarative" | "webview" (settings.* supports declarative only)
host = "app"                 # "app" | "daemon"
icon = "marks/icon.svg"
web_root = "web"             # required for webview
entry = "index.html"         # relative entry inside web_root
order = 10

[[themes]]
name = "Aurora Dusk"
file = "themes/aurora.ghostty"

[[washes]]
id = "aurora-mist"
name = "Aurora Mist"
file = "washes/aurora.toml"

[[pets]]
id = "pixel-fox"
dir = "pets/pixel-fox"

[[hooks]]
agent = "claude"
event = "Stop"
script = "hooks/on-stop.sh"

[[skills]]
agent = "claude"
name = "my-skill"
dir = "skills/my-skill"

[[settings]]
key = "endpoint"
title = "API Endpoint"
description = "Custom backend base URL"
type = "string"              # string | secret | boolean | integer | number | select
default = "http://localhost:8080"
placeholder = "https://..."
options = []                 # choices when type = "select"

[env]
CUSTOM_FLAG = "1"
```

### Action ID Qualification

Actions in `[[actions]]` are qualified automatically as `<plugin-id>.<action-id>` (unless the `id` already begins with `<plugin-id>.` or `plugin.`). Global actions and shortcuts can reference the qualified action ID without modifying Hyla's static action catalog.

### Permission Scopes

Every Plugin -> Host RPC call passes through the Host RPC gateway, which enforces `grantedPermissions` from `plugins.json` and returns JSON-RPC error `-32001` (`ErrCodePermissionDenied`) when ungranted.

| Scope | High Risk | Grants Access To |
| --- | --- | --- |
| `session.read` | No | `host.getState` |
| `pane.read` | No | `pane.readScreen`, `pane.getEnv` |
| `pane.write` | **Yes** | `pane.sendInput`, `pane.sendKey`, `pane.reportStatus` |
| `pane.focus` | No | `pane.focus` |
| `session.create` | **Yes** | `session.create` |
| `agent.provider` | No | `agent.detect`, `agent.resumeCommand`, `agent.sendChat`, agent registration (also grants `agent.readTimeline`/`agent.readChat`/`agent.readSubagents`) |
| `timeline.read` | No | `agent.readTimeline`, `agent.readChat`, `agent.readSubagents` |
| `ui.slot` | No | `ui.invalidate`, `ui.openPane`, `ui.render`, `ui.event`, `action.invoke`, slots & menus |
| `ui.webview` | No | `hylaplugin://` WebView hosting, `webview.rpc`, `webview.emit` |
| `network` | No | Declares outbound network access |
| `secrets` | **Yes** | `secrets` passed to `plugin.getSettings` and written via `plugin.setSettings` |
| `notifications` | No | `ui.notify` |
| `filesystem.workspace` | **Yes** | Workspace file access referenced by panes |
| `channel.provider` | **Yes** | Interactive external chat channel drivers |

---

## Layer 2: Out-of-Process JSON-RPC 2.0 Protocol

### Transport & Authentication

1. **`stdio` (default)**: Host spawns `command` in its own process group (`Setpgid` on Unix; process/job cleanup on Windows). Requests/responses/notifications are newline-delimited JSON (`\n`) with a **16 MiB** line limit. Any `stderr` output is captured into `~/.hyla/logs/plugins/<id>.log`. Plugin binaries using `stdio` **must log strictly to `stderr`**, never `stdout`.
2. **`socket`**: Host creates a local Unix domain socket (or Windows named pipe), exports `HYLA_PLUGIN_SOCKET` and `HYLA_PLUGIN_TOKEN` in the process environment, and requires the plugin's first line on connect to be:

```json
{"token":"<HYLA_PLUGIN_TOKEN>"}
```

Additional environment variables exported to spawned plugins:
- `HYLA_PLUGIN_ID`
- `HYLA_PLUGIN_DIR`
- `HYLA_PLUGIN_DATA_DIR`
- `HYLA_PLUGIN_HOST` (`"daemon"` or `"app"`)

### Lifecycle Sequence

```text
Host                                      Plugin
  |--- initialize {hylaVersion, host, ...} --->|
  |<-- {name, version, capabilities, subs} ----|
  |--- initialized (notification) ------------>|
  |<== bidirectional active JSON-RPC calls ===>|
  |--- shutdown ------------------------------>|
  |<-- {ok: true} -----------------------------|
  |--- exit (notification) ------------------->|
  [grace wait -> SIGTERM/kill process group]
```

### Error Codes

| Code | Constant | Meaning |
| --- | --- | --- |
| `-32700` | `ErrCodeParse` | Invalid JSON frame |
| `-32600` | `ErrCodeInvalidRequest` | Malformed JSON-RPC envelope |
| `-32601` | `ErrCodeMethodNotFound` | Method not handled |
| `-32602` | `ErrCodeInvalidParams` | Invalid parameters |
| `-32603` | `ErrCodeInternal` | Internal plugin/host error |
| `-32001` | `ErrCodePermissionDenied` | Missing granted permission scope |
| `-32002` | `ErrCodeRemoteControlDisabled` | `pane.focus` blocked because `remoteControl` is off |
| `-32003` | `ErrCodeTargetNotFound` | Session, pane, or slot does not exist |
| `-32004` | `ErrCodeCapabilityUnsupported` | Target capability unsupported |
| `-32005` | `ErrCodeTimeout` | RPC deadline exceeded |
| `-32006` | `ErrCodePayloadTooLarge` | Frame exceeds 16 MiB cap |

### Host -> Plugin RPCs

- **Lifecycle**: `initialize`, `initialized` (notify), `shutdown`, `exit` (notify)
- **Agents**: `agent.detect`, `agent.readTimeline`, `agent.readChat`, `agent.sendChat`, `agent.readSubagents`, `agent.resumeCommand`
- **UI & Actions**: `ui.render`, `ui.event`, `action.invoke`, `webview.rpc`
- **Events (`event.notify`)**: `session.state`, `pane.status`, `pane.osc`, `theme.changed` (delivered if subscribed in `InitializeResult.subscriptions`)

### Plugin -> Host RPCs

- **State & Terminal**: `host.getState`, `pane.readScreen`, `pane.getEnv`, `session.create`, `pane.sendInput`, `pane.sendKey`, `pane.reportStatus`, `pane.focus`
- **UI & Notifications**: `ui.invalidate`, `ui.openPane`, `ui.notify`, `webview.emit`
- **Config & Secrets**: `plugin.getSettings`, `plugin.setSettings`

### Daemon RPC Extensions & Protocol Compatibility

Hyla preserves `proto.Version = 4` and advertises `"plugins.v1"` in `Welcome.Capabilities`:
- `plugin.list` — returns runtime statuses (`[]rpc.PluginStatus`)
- `plugin.call` — proxies a call to a daemon plugin (`{pluginId, method, params}`)
- `plugin.reload` — reloads manifests, packs, and processes on `hylad`
- `plugin.restart` — resets circuit breaker state and restarts `{pluginId}`
- `plugin.logs` — reads the tail of a plugin's log `{pluginId, lines}`

When connecting to an older remote `hyla-server` without `"plugins.v1"`, the desktop app gracefully skips remote `plugin.*` calls. On remote hosts (`remoteControl == false`), remote plugins cannot trigger local window actions, session selection, focus changes, or local pane opening.

### Process Hygiene & Circuit Breaker

- Ordinary `hylad` plugins (`keep_alive = false`) **never** block `hylad`'s 30-second idle exit.
- Only enabled plugins actively running with `keep_alive = true` (or active `ChannelDriver`s) keep `hylad` alive.
- Repeated crashes exceeding `max_crashes_per_minute` (default 5 in 60s) trip the circuit breaker and place the plugin into `degraded` state until restarted (`hyla plugin` / Settings UI / `plugin.restart`).

---

## Layer 3A: Declarative UI JSON Schema

Plugins return a `UIRenderResult` containing a `UINode` tree rendered asynchronously by Hyla's native UI engine (`internal/plugins/uihost`).

### Supported Node Types (23)

- **Layout**: `column`, `row`, `box`, `spacer`, `divider`, `scroll`, `list`
- **Containers**: `infoCard`, `settingsGroup`, `settingsRow`
- **Display**: `text`, `richText`, `badge`, `icon`, `image`, `spinner`, `progress`
- **Controls**: `button`, `toggle`, `segmented`, `select`, `textInput`, `slider`

### Palette Tokens & Typography

Color strings in `style.background`, `style.border`, `style.color`, `props.color`, or `text` tones support live theme resolution via `$token` or `$token@alpha` (e.g. `"$accent@0.2"`):

- `$bg`, `$sidebar`, `$card`, `$chip`, `$popover`, `$control`
- `$text`, `$muted`, `$faint`, `$accent`, `$attention`, `$done`, `$danger`, `$divider`

`UIStyle` supports `width`, `height`, `minWidth`, `minHeight`, `maxWidth`, `maxHeight`, `padding` (`[all]`, `[v,h]`, or `[t,r,b,l]`), `gap`, `radius`, `flex`, `align` (`start|center|end|stretch`), `justify` (`start|center|end|between`), `background`, `border`, `borderWidth`, `color`, `fontSize`, `fontWeight`, `monospace`, `tabular`, `wrap`.

### Mount Points

| Mount | Target Location |
| --- | --- |
| `info.card` | Card inside the right-hand Info drawer |
| `side.tab` | Custom tab alongside Files / Info / Changes in the side panel |
| `pane` | Full workspace pane (`proto.CreateParams{Plugin, PluginView, PluginTitle, PluginArgs}`) |
| `sidebar.menu` | Session context menu in the left sidebar |
| `files.menu` | File/directory context menu in the Files tab |
| `pane.menu` | Pane header context menu |
| `palette` | Command Palette commands |
| `settings.plugins` | Custom Declarative UI inside the plugin's detail drawer in `Settings > Plugins` |
| `settings.page` | Standalone top-level page in the Settings window's left sidebar |
| `settings.<page>` | Appended Declarative UI section on a built-in Settings page (`settings.general`, `settings.appearance`, `settings.terminal`, `settings.shell`, `settings.browser`, `settings.notifications`, `settings.keyboard`, `settings.feishu`, `settings.agents`, `settings.sync`, `settings.hosts`, `settings.advanced`) |

Custom `pane` sessions persist across `hylad` restarts via `savedPane` / `restorePane`. If the plugin is disabled, missing, or pending installation on a host, Hyla renders an actionable unavailable fallback card with a button to close the pane.

### Extending Settings

Plugins can extend the Settings window in four ways:

1. **Structured `[[settings]]` fields**: Declared in `hyla-plugin.toml` (`string`, `secret`, `boolean`, `integer`, `number`, `select`). Hyla renders them automatically in `Settings > Plugins` (and on the plugin's first `settings.page`, if declared), stores non-secret values in `~/.hyla/plugin-config/<id>.json` and secrets in credential storage, and indexes each item's `title`, `key`, and `description` in Settings search (selecting a match auto-expands that plugin's drawer).
2. **`mount = "settings.plugins"` (Drawer UI)**: Enabled plugins with `ui.slot` granted can render interactive Declarative UI controls (such as status indicators or a "Test Connection" button using `settingsGroup` / `settingsRow` / `button`) directly inside their `Settings > Plugins` detail drawer.
3. **`mount = "settings.page"` (Top-level Settings page)**: Enabled plugins with `ui.slot` granted add a dedicated first-level entry in the Settings sidebar using the slot's `title` and `icon` (plugin SVG asset with fallback to the default plugin icon), also reachable via Settings search and the Command Palette. If the plugin is disabled or removed while active, Settings falls back cleanly.
4. **`mount = "settings.<page>"` (Built-in page sections)**: Enabled plugins with `ui.slot` granted can append Declarative UI sections onto any built-in Settings page (`settings.general`, `settings.appearance`, `settings.terminal`, `settings.shell`, `settings.browser`, `settings.notifications`, `settings.keyboard`, `settings.feishu`, `settings.agents`, `settings.sync`, `settings.hosts`, `settings.advanced`).

All `settings.*` UI slots render natively via Layer 3A Declarative UI (`mode = "declarative"`, requiring `ui.slot`; `mode = "webview"` is not supported on Settings mounts). Use `ui.invalidate` or return `{root, invalidate, message}` from `ui.event` to repaint or show feedback immediately in the Settings window.

---

## Layer 3B: Sandboxed WebView (`hylaplugin://`)

Slots with `mode = "webview"` load through a single global custom scheme `hylaplugin://<plugin-id>/...` registered before `MainWindow` creation:

- Each plugin gets an isolated `Untrusted` WebView session (`mygo.NewSession("plugin:" + id)`).
- Strict CSP header locks down external navigation and connections (`default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'none'; object-src 'none'`).
- Cross-origin navigation inside the WebView is blocked via `OnWillNavigate` (`e.PreventDefault()`).
- Virtual endpoints served automatically under the plugin origin:
  - `/_hyla/theme.css` — live CSS variables (`--hyla-bg`, `--hyla-sidebar`, `--hyla-card`, `--hyla-chip`, `--hyla-popover`, `--hyla-control`, `--hyla-text`, `--hyla-muted`, `--hyla-text-muted`, `--hyla-faint`, `--hyla-accent`, `--hyla-attention`, `--hyla-done`, `--hyla-danger`, `--hyla-divider`, `--hyla-font-ui`, `--hyla-font-mono`).
  - `/_hyla/bridge.js` — injects `window.hyla.rpc(method, params, slotId)` and `window.hyla.on(event, listener)`.
  - `POST /_hyla/rpc` — authenticates caller via `mygo.CallerPage(r.Context())` and dispatches `webview.rpc` to the plugin host process.
- Host-to-page real-time push events use `Event.EmitPage` on channel `"hyla:plugin"` (`host.WebViewEmit` in Go, `host.webviewEmit` in TS).

---

## Official SDKs

### Go SDK (`sdk/go/hylaplugin`)

Import `github.com/smallnest/hyla/sdk/go/hylaplugin` (`sdk/go/hylaplugin/go.mod`):

```go
package main

import (
	"context"
	"log"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func main() {
	plg := &hylaplugin.Plugin{
		Name:         "hello-go",
		Version:      "0.1.0",
		Capabilities: []string{"ui.declarative"},
		OnUIRender: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIRenderParams) (hylaplugin.UIRenderResult, error) {
			st, err := host.GetState(ctx)
			if err != nil {
				return hylaplugin.UIRenderResult{}, err
			}
			return hylaplugin.UIRenderResult{
				Title: "Sessions",
				Root: hylaplugin.InfoCard("Workspace", "", false, []*hylaplugin.UINode{
					hylaplugin.Text("Active: "+st.ActiveSession, hylaplugin.WithKey("act")),
					hylaplugin.Button("refresh", "Refresh", "secondary", hylaplugin.WithKey("btn"), hylaplugin.WithOnClick("refresh")),
				}),
			}, nil
		},
	}
	if err := hylaplugin.Run(plg); err != nil {
		log.Fatal(err)
	}
}
```

Unit tests can use `hylaplugin.NewTestHarness(plg)` (`Call(ctx, method, params, &out)`, `RegisterHostHandler(method, fn)`, `Close()`). Agent handlers return `Timeline` (`Turns []TimelineStep`), `Chat` (`Lines []ChatLine`), and `[]Subagent` matching Hyla's wire schema.

### TypeScript / Bun SDK (`sdk/ts`)

Zero-dependency TypeScript SDK exporting two entry points:
- `@hyla/plugin-sdk`: `runPlugin`, `attachPluginPeer`, `RPCPeer`, `HostClient`, `ui.*` builders, typed RPC payloads.
- `@hyla/plugin-sdk/web`: browser helpers `rpc`, `onEvent`, `onThemeChanged`, `readThemeTokens`.

---

## Three Complete Walkthrough Examples

### Example 1: Layer 1 Declarative Pack (Theme + Agent Detect + Mark)

```sh
hyla plugin init --template pack --id org.acme.aurora ./aurora-pack
cd aurora-pack
hyla plugin link .
```

Edit `hyla-plugin.toml` (`detect/*.toml`, `marks/*.svg`, `themes/*.ghostty`, `washes/*.toml`). Saving files updates Hyla cleanly when reloaded via `hyla plugin dev .` or `Settings > Plugins > Reload`.

### Example 2: Layer 2 Go Agent & Declarative UI Plugin

See the included runnable example in `sdk/go/hylaplugin/examples/hello-agent`:

```sh
cd sdk/go/hylaplugin/examples/hello-agent
go build -o hello-agent .
hyla plugin link . --grant-all
```

It declares:
- `agent.provider`, `ui.slot`, and `session.read` scopes in `hyla-plugin.toml`.
- `[[agents]]` (`processes = ["hello-agent"]`, `timeline = true`).
- `[[ui.slots]]` (`mount = "info.card"`, `mode = "declarative"`, `host = "daemon"`).

### Example 3: Layer 3 TypeScript + Sandboxed WebView Pane

```sh
hyla plugin init --template ts-webview --id org.acme.inspector ./inspector-webview
cd inspector-webview
hyla plugin link . --grant-all
```

In Hyla, open the **Plugins** menu or Command Palette (`Cmd+K`) and run **Open inspector-webview**. The HTML page inside `web/index.html` automatically inherits Hyla's live theme via `/_hyla/theme.css` and calls `await window.hyla.rpc("ping", { hello: "hyla" })` over `/_hyla/bridge.js`, handled by `src/plugin.ts`'s `webview.rpc` listener.

---

## CLI Reference (`hyla plugin`)

```text
hyla plugin ls [--json]                         List bundled, linked, installed, and pending plugins
hyla plugin init [--template pack|go|ts-webview] [--id <id>] [--name <name>] [dir]
hyla plugin pack <dir> [-o out.hylapkg]         Package a plugin directory into a .hylapkg archive
hyla plugin install <pkg.hylapkg|dir> [--grant-all] [--no-enable]
hyla plugin link <dir> [--grant-all]            Symlink-register a local development folder
hyla plugin dev <dir> [--grant-all]             Link and hot-reload on file changes
hyla plugin enable <id>                         Enable plugin and notify daemon
hyla plugin disable <id>                        Disable plugin and stop host processes
hyla plugin rm <id> [--purge]                   Remove plugin (--purge deletes config/data/logs too)
hyla plugin permissions <id> [grant|revoke <scope…>|--all]
hyla plugin logs <id> [-n N]                    Print plugin log tail (via hylad if running, or local file)
```

---

## Packaging & Security Model (`.hylapkg`)

`hyla plugin pack` creates a deterministic `.hylapkg` ZIP archive with SHA-256 verification stored in `plugins.json`. `InstallPackage` enforces:
- Rejection of `..` path traversal, absolute archive paths, and symlinks inside archives.
- File count and uncompressed byte caps (preventing zip bombs).
- Preservation of `0755` executable mode bits on compiled binaries and hook scripts.
- **No mandatory code signing** and **no OS-level sandbox wrappers**; safety is enforced by path validation, explicit user permission grants (`grantedPermissions`), and WebView origin/session isolation.

---

## Cross-Machine Sync & Built-in Plugins

### Built-in Official Plugins

Hyla bundles one official plugin enabled by default:
- **`hyla.magpie`**: Displays Magpie coding-agent gateway metrics (`info.card` slot + `Refresh Magpie Usage` palette action + configurable gateway URL).

Feishu is a feature of Hyla's own, not a plugin: its bot runs in hylad, and **Settings › Feishu** is where it is set up. An installation that configured it through 3.17.0's `hyla.feishu` plugin keeps that configuration, which the first start of this version moves into the Feishu settings and then removes the plugin.

### Cross-Machine Sync

Hyla's encrypted sync (`Snapshot` v1) includes:
- `~/.hyla/plugins.json` with machine-local `linkPath` fields stripped. If a synced plugin is not present on another machine, Hyla marks it **`pendingInstall: true`** and **`enabled: false`** rather than silently downloading external code.
- `~/.hyla/plugin-config/<id>.json` non-secret settings.
- **`~/.hyla/plugin-secrets/` is strictly excluded** from snapshots and never leaves the local machine.

---

## Troubleshooting

- **Plugin shows `degraded`**: Check `hyla plugin logs <id>` or **Settings > Plugins > Logs**. After fixing the crash, click **Restart** or run `hyla plugin disable <id> && hyla plugin enable <id>`.
- **Permission error `-32001`**: Inspect granted scopes with `hyla plugin permissions <id>` and grant missing scopes via `hyla plugin permissions <id> grant <scope>` or **Settings > Plugins**.
- **WebView blank or `window.hyla` undefined**: Confirm `[[ui.slots]]` sets `mode = "webview"`, `[permissions]` includes `"ui.webview"`, `web/index.html` includes `<script src="/_hyla/bridge.js"></script>`, and `web_root` matches your folder.
- **Remote host doesn't show plugin actions**: Verify the remote `hyla-server` advertises `"plugins.v1"` (update `hyla-server` to a build supporting plugins).
