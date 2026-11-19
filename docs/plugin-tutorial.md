---
title: Build a Plugin
description: "A hands-on tutorial: a theme and agent pack in ten minutes, a daemon plugin with a native card, and a sandboxed WebView pane, with settings, permissions, debugging and packaging."
---

# Build a Plugin

Hyla plugins extend the app without recompiling it: a coding agent it doesn't know, a theme, a card in the side panel, a page in Settings, a pane of your own. There are three layers, and a plugin takes the ones it needs — most take one.

| Layer | What it is | Takes |
| --- | --- | --- |
| **1 — Pack** | `hyla-plugin.toml` and static files: agent detection, marks, themes, washes, pets, hooks, skills | No process, no code |
| **2 — Host process** | A program Hyla speaks JSON-RPC 2.0 to, over stdio or a socket: agents, timelines, chats, actions, UI trees | Go, TypeScript, anything that reads stdin |
| **3 — UI** | Native declarative UI (JSON nodes), or a sandboxed WebView for HTML | A slot to mount it in |

This page builds one of each. [Plugins](./plugins.md) is the reference: every manifest field, RPC method, UI node, permission scope and error code.

::: tip
The fastest way to see a finished plugin is the one Hyla bundles: **`hyla.magpie`** draws its card in the Info panel, has a palette action and a setting. Its source is `internal/plugins/builtins/magpie` in the repository.
:::

## Before you start

- Hyla 3.3 or later, with the `hyla` command on your `PATH` (`hyla --version`).
- A folder for your work. Plugins are linked or installed from a directory; they don't have to live inside Hyla's own folders.
- For layer 2 or 3: Go 1.22+ or [Bun](https://bun.sh) — the templates use one or the other.

Every plugin has an id: a reverse-domain name of at least two segments, lowercase, `com.example.hello`. It names the plugin's directory, its settings, its actions and its WebView origin, so pick one and keep it.

```sh
hyla plugin ls            # what is installed, linked, or bundled
```

## Part 1 — A pack: a theme, a wash, an agent

A pack is files and a manifest. Scaffold one:

```sh
hyla plugin init --template pack --id com.example.aurora --name "Aurora" ./aurora
cd aurora
```

```text
aurora/
├── hyla-plugin.toml
├── detect/agent.toml
├── marks/agent.svg
├── themes/aurora.ghostty
└── washes/aurora.toml
```

The manifest lists what the pack brings:

```toml
api_version = 1
id = "com.example.aurora"
name = "Aurora"
version = "0.1.0"
description = "Declarative asset pack plugin for Hyla"
author = "You"

[[agents]]
id = "sample-agent"
name = "Sample Agent"
detect = "detect/agent.toml"
mark = "marks/agent.svg"
processes = ["sample-agent"]

[[themes]]
name = "Aurora Dusk"
file = "themes/aurora.ghostty"

[[washes]]
id = "aurora-mist"
name = "Aurora Mist"
file = "washes/aurora.toml"
```

**The agent** is recognised by its process name, drawn with its mark, and read by `detect/agent.toml` — rules that turn a pane's screen into a state:

```toml
id = "sample-agent"

[[rules]]
id = "sample-agent.permission"
state = "blocked"
visible_blocker = true
regex = ["(?i)allow|approve\\?"]
```

A rule's `state` is `working`, `blocked` or `idle`; a `blocked` one with `visible_blocker = true` is what raises the session to *waiting* and notifies you. The mark is an SVG drawn at 16×16 with `currentColor`, so it takes the sidebar's tint.

**The theme** is a Ghostty theme file — Hyla renders terminals with libghostty, so the format is Ghostty's:

```text
background = #161822
foreground = #e2e8f0
cursor-color = #7dd3fc
```

**The wash** is the window's background, a pair of colours per appearance:

```toml
id = "aurora-mist"
name = "Aurora Mist"
light = ["#f4f7fb", "#e6eefb"]
dark = ["#141824", "#1b2438"]
light_glow = "#cfe5ff"
dark_glow = "#263b5e"
```

Link the folder, and it is live:

```sh
hyla plugin link .
```

Open **Settings › Plugins** and it is there with the scopes it asks for (none, for a pack) and where it came from. *Aurora Dusk* is in **Settings › Appearance**, *Aurora Mist* beside it, and a pane running `sample-agent` is drawn as Hyla's own agents are.

Editing files: link is enough — press **Reload** in Settings › Plugins, or watch the folder and let Hyla reload as you save:

```sh
hyla plugin dev .
```

::: tip
A pack needs no restart of anything. `hyla plugin dev` re-links the folder on every change, which is what a theme or a mark needs to show.
:::

## Part 2 — A card, in the daemon

Now a plugin that runs. This one draws a card in the Info panel, counts the clicks on its button, and shows the session it is in.

```sh
hyla plugin init --template go --id com.example.hello --name "Hello" ./hello
cd hello
```

```text
hello/
├── hyla-plugin.toml
├── main.go
├── Makefile
└── go.mod
```

The manifest asks for the card's slot and the session's state, and runs the binary in `hylad`:

```toml
[permissions]
scopes = ["ui.slot", "session.read"]

[daemon]
command = ["./bin/plugin"]
transport = "stdio"
auto_start = false

[[actions]]
id = "refresh"
title = "Refresh Hello"
palette = true

[[ui.slots]]
id = "card"
mount = "info.card"
title = "Hello"
mode = "declarative"
host = "daemon"
```

`hyla plugin init` writes a `main.go` that speaks the protocol by hand: a line of JSON in, a line of JSON out, `ui.render` answering with a tree and `ui.event` counting a click.

```go
case "initialize":
    res = map[string]any{"name": "go-plugin", "version": "0.1.0"}
case "ui.render", "ui.event":
    if req.Method == "ui.event" {
        clicks++
    }
    res = map[string]any{
        "root": map[string]any{
            "type": "column",
            "children": []any{
                map[string]any{"type": "text", "props": map[string]any{"text": fmt.Sprintf("Clicks: %d", clicks)}},
                map[string]any{"type": "button", "id": "inc", "onClick": "inc", "props": map[string]any{"label": "Click Me"}},
            },
        },
    }
```

That's the whole wire protocol for a card: `initialize` says who you are, `ui.render` returns `{root}`, `ui.event` returns the tree again (`invalidate: true` asks for the next render), `action.invoke` answers a palette action, `shutdown` ends. **stdout carries JSON-RPC and nothing else** — a stray `fmt.Println` corrupts the stream. Log to stderr, which Hyla keeps in the plugin's log.

Build it, link it, and open a session's Info panel:

```sh
make                      # builds ./bin/plugin, as the manifest's command says
hyla plugin link . --grant-all
```

`--grant-all` grants the scopes the manifest asks for, which is what a development link wants; a plugin installed by someone else asks you in **Settings › Plugins**.

### The same card, with the SDK

Writing the loop by hand is fine for one card. For agents, timelines, settings and tests, use the Go SDK — `github.com/smallnest/hyla/sdk/go/hylaplugin`:

```go
plg := &hylaplugin.Plugin{
    Name:    "hello-agent",
    Version: "0.1.0",
    OnUIRender: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIRenderParams) (hylaplugin.UIRenderResult, error) {
        st, err := host.GetState(ctx)          // needs session.read
        if err != nil {
            return hylaplugin.UIRenderResult{}, err
        }
        return hylaplugin.UIRenderResult{
            Title: "Hello Agent",
            Root: hylaplugin.Column([]*hylaplugin.UINode{
                hylaplugin.Text("Active: "+st.ActiveSession, hylaplugin.WithProp("tone", "muted")),
                hylaplugin.Button("ping", "Ping", "primary", hylaplugin.WithOnClick("ping")),
            }, hylaplugin.WithStyle(hylaplugin.UIStyle{Gap: 8})),
        }, nil
    },
    OnUIEvent: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIEventParams) (hylaplugin.UIEventResult, error) {
        return hylaplugin.UIEventResult{Handled: true, Invalidate: true}, nil
    },
}

if err := hylaplugin.Run(plg); err != nil {
    log.Fatal(err)
}
```

`hylaplugin.Run` is the loop, the handshake and the logging rule; `Host` is the calls back into Hyla (`GetState`, `ReadScreen`, `SendInput`, `Focus`, `Notify`, `GetSettings`). The SDK also has builders for every node: `Column`, `Row`, `Text`, `Badge`, `Button`, `Toggle`, `TextInput`, `Progress`, `InfoCard`. `sdk/go/hylaplugin/examples/hello-agent` is this plugin complete, with an agent and a timeline: build it, `hyla plugin link sdk/go/hylaplugin/examples/hello-agent --grant-all`, and run `hello-agent` in a pane.

The SDK's `NewTestHarness(plg)` calls your handlers without Hyla, which is how the SDK's own tests and the bundled Magpie plugin are tested.

::: tip
Build for the machine that runs it. A daemon plugin runs in `hylad`, which may be another machine's — **Go** binaries need to be built for that platform, while a **TypeScript** plugin runs under that machine's Bun.
:::

## Part 3 — A pane of your own, in HTML

A WebView slot is a page Hyla serves from the plugin's own origin, in its own sandboxed session. The template is TypeScript with the page served from `web/`:

```sh
hyla plugin init --template ts-webview --id com.example.inspector --name "Inspector" ./inspector
cd inspector
```

```text
inspector/
├── hyla-plugin.toml
├── package.json
├── src/plugin.ts
└── web/index.html
```

The manifest opens a pane with a palette action, and mounts the page in it:

```toml
[permissions]
scopes = ["ui.webview", "session.read"]

[app]
command = ["bun", "run", "src/plugin.ts"]
transport = "stdio"

[[actions]]
id = "open"
title = "Open Inspector"
menu = "Plugins"
palette = true
target = "open_pane"
view = "main"

[[ui.slots]]
id = "main"
mount = "pane"
title = "Inspector"
mode = "webview"
web_root = "web"
entry = "index.html"
host = "app"
```

The page is ordinary HTML, with two virtual endpoints Hyla serves beside it: the live theme, and the bridge to the plugin's process.

```html
<link rel="stylesheet" href="/_hyla/theme.css" />
<script src="/_hyla/bridge.js"></script>
<script>
  document.getElementById("ping").addEventListener("click", async () => {
    const res = await window.hyla.rpc("ping", { hello: "hyla" });   // → the plugin's webview.rpc
    document.getElementById("out").textContent = JSON.stringify(res, null, 2);
  });
</script>
```

`/_hyla/theme.css` is the window's palette as CSS variables (`--hyla-bg`, `--hyla-text`, `--hyla-accent`, `--hyla-divider`, `--hyla-font-ui`, …), so the page follows the theme as it changes. `window.hyla.rpc(method, params)` becomes a `webview.rpc` call on your plugin process — `src/plugin.ts` answers it:

```ts
if (msg.method === "webview.rpc") {
  result = { data: { echo: msg.params?.method, time: new Date().toISOString() } };
}
```

Run it: `hyla plugin link . --grant-all`, then **Plugins › Open Inspector** or the palette. The page runs in an `Untrusted` WebView with a strict CSP, cross-origin navigation blocked, and no access to Hyla beyond `window.hyla` — the plugin's pane is a web page, not the app.

To push to the page instead of waiting for a click, the process calls `webview.emit`, which the page hears with `window.hyla.on(event, listener)`.

## Part 4 — Settings and secrets

A plugin's settings are declared in the manifest and rendered by Hyla; no UI code:

```toml
[permissions]
scopes = ["secrets"]

[[settings]]
key = "endpoint"
title = "API Endpoint"
description = "Base URL of the backend this plugin reads"
type = "string"
default = "http://localhost:8080"
placeholder = "https://…"

[[settings]]
key = "token"
title = "API Token"
description = "Kept in the system keychain, never in the configuration"
type = "secret"
```

Hyla draws them in the plugin's drawer in **Settings › Plugins**, indexes them in Settings search, and stores the values: non-secret ones in the plugin's config file, `secret` ones in the system keychain — they are never written to the config, never logged, and never synced to another machine.

The plugin reads them with a call, not from its environment:

```go
var cfg hylaplugin.PluginGetSettingsResult
if err := host.Call(ctx, "plugin.getSettings", hylaplugin.PluginGetSettingsParams{}, &cfg); err != nil {
    return err
}
endpoint, _ := cfg.Settings["endpoint"].(string)
```

`plugin.setSettings` writes a value back — which is how a plugin refreshes a token it got from a sign-in, and why it needs the `secrets` scope. A settings change also arrives as an `event.notify` with topic `settings.changed`, for a plugin that caches what it read.

### What a plugin may do

Every call out of a plugin passes a permission gate. A plugin declares what it wants in `[permissions] scopes`, and Hyla shows exactly that when you link or install it:

| Scope | What it allows |
| --- | --- |
| `session.read` | `host.getState` — sessions, panes, which is focused |
| `pane.read` | read a pane's screen and environment |
| `pane.write` | type into a pane, press keys, report its status |
| `pane.focus` | focus a pane |
| `session.create` | start sessions |
| `agent.provider` | provide an agent: detect it, read its timeline, chat, subagents |
| `timeline.read` | read agents' timelines, chats and subagents |
| `ui.slot` | mount UI, render, take events, invalidate, open panes |
| `ui.webview` | serve a WebView and its bridge |
| `secrets` | read and write secret settings |
| `notifications` | post a notification |
| `filesystem.workspace` | read and write files in the panes' folders |

`hyla plugin permissions <id>` prints what is granted, and `hyla plugin permissions <id> grant pane.write` changes it. The full table, with which scopes are high-risk, is in [Plugins](./plugins.md#permission-scopes).

## Part 5 — Debug it, pack it, share it

Logs are the first stop. A plugin's stderr, its lifecycle and its crashes go to one log:

```sh
hyla plugin logs com.example.hello -n 100      # through hylad when it runs
```

| Symptom | What it is |
| --- | --- |
| **degraded** in Settings › Plugins | It crashed more than `max_crashes_per_minute` allows. Fix it, then **Restart** (or `hyla plugin disable <id> && hyla plugin enable <id>`). |
| Error `-32001` | A scope the manifest asks for was never granted: `hyla plugin permissions <id> grant <scope>`. |
| Card is blank, or `window.hyla` is undefined | `mode = "webview"` without `ui.webview`, a missing `<script src="/_hyla/bridge.js">`, or `web_root` naming the wrong folder. |
| Nothing happens on a save | A link shows the files as they are; a *pack* needs **Reload**, `hyla plugin dev`, or a restart of the host process. |
| A Go plugin runs, then exits at once | Its stdout carried something that wasn't JSON-RPC — log to stderr. |

To give a plugin to someone else, pack it:

```sh
hyla plugin pack . -o aurora.hylapkg
hyla plugin install aurora.hylapkg
```

A `.hylapkg` is a deterministic ZIP with a SHA-256 recorded in the registry. Its install checks every path (no `..`, no absolute paths, no symlinks), caps the file count and the unpacked size, and keeps the executable bit on binaries and hook scripts. **There is no code signing** — a package is trusted because you installed it and granted its scopes.

Plugins travel between your machines with Hyla's sync, minus what should not travel: the registry and the non-secret settings go, secrets never do, and a plugin that is not on the other machine arrives **pending install** rather than running itself.

## Where to go next

- [Plugins](./plugins.md) — the reference: manifest, RPC methods and events, UI nodes and palette tokens, mount points, error codes.
- `sdk/go/hylaplugin` — the Go SDK, with `examples/hello-agent` (an agent, a timeline and a card) and `examples/git-summary`.
- `sdk/ts` — `@hyla/plugin-sdk` for a host process, and its `/web` entry for a WebView page.
- The bundled `hyla.magpie` plugin, in `internal/plugins/builtins/magpie`, is the pattern for a card that reads a service and has a setting.
- `hyla plugin --help` lists every command the CLI has.
