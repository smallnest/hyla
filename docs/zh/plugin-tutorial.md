---
title: 写一个插件
description: 手把手教程：十分钟做一个主题与智能体包、一个在 daemon 里画原生卡片的插件、一个沙箱 WebView 窗格，以及设置、权限、调试与打包。
---

# 写一个插件

Hyla 的插件让你不重新编译就能扩展它：教它认识一个新的编程智能体、换一套主题、在侧栏画一张卡片、在设置里加一页、做一个属于你的窗格。插件分三层，做一个插件只需按需取用——大多数只用到一层。

| 层 | 是什么 | 需要 |
| --- | --- | --- |
| **1 —— 包** | `hyla-plugin.toml` 加静态文件：智能体识别、图标、主题、背景、宠物、钩子、技能 | 不需要进程，不需要写代码 |
| **2 —— 宿主进程** | 一个 Hyla 用 JSON-RPC 2.0 与之对话的程序，走 stdio 或本地 socket：智能体、时间线、对话、动作、UI 树 | Go、TypeScript，任何能读写 stdin 的语言 |
| **3 —— 界面** | 原生声明式 UI（JSON 节点），或给 HTML 用的沙箱 WebView | 一个可挂载的位置 |

这一页每层各做一个。[插件参考](./plugins.md)是查阅用的：每一个清单字段、RPC 方法、UI 节点、权限范围和错误码。

::: tip
想看一个做完的插件，最好是 Hyla 自带的 **`hyla.magpie`**：它在信息面板画卡片、有一个命令面板动作、还有一个设置项。源码在仓库的 `internal/plugins/builtins/magpie`。
:::

## 开始之前

- Hyla 3.3 或更新，`hyla` 命令在 `PATH` 里（`hyla --version`）。
- 一个放代码的目录。插件是从目录 link 或安装进来的，不必放进 Hyla 自己的目录。
- 用到第 2、3 层时：Go 1.22+ 或 [Bun](https://bun.sh)，模板各用一个。

每个插件都有一个 id：至少两段的倒域名，全小写，例如 `com.example.hello`。它决定插件的目录、设置、动作和 WebView 的来源，所以选好就别改。

```sh
hyla plugin ls            # 已安装、已链接、以及自带的插件
```

## 第 1 部分 —— 一个包：主题、背景和一个智能体

包就是一堆文件加一份清单。先生成一个：

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

清单说明这个包带来了什么：

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

**智能体**靠进程名认出来，用图标画出来，用 `detect/agent.toml` 读懂——把窗格的屏幕变成状态的规则：

```toml
id = "sample-agent"

[[rules]]
id = "sample-agent.permission"
state = "blocked"
visible_blocker = true
regex = ["(?i)allow|approve\\?"]
```

规则的 `state` 是 `working`、`blocked` 或 `idle`；带 `visible_blocker = true` 的 `blocked` 规则会把会话升到*等待*并通知你。图标是 16×16、用 `currentColor` 画的 SVG，所以它会跟着边栏的色调走。

**主题**是 Ghostty 的主题文件——Hyla 的终端由 libghostty 渲染，格式就是 Ghostty 的：

```text
background = #161822
foreground = #e2e8f0
cursor-color = #7dd3fc
```

**背景**是窗口的底，按明暗各给一组颜色：

```toml
id = "aurora-mist"
name = "Aurora Mist"
light = ["#f4f7fb", "#e6eefb"]
dark = ["#141824", "#1b2438"]
light_glow = "#cfe5ff"
dark_glow = "#263b5e"
```

把目录 link 进来，它立刻生效：

```sh
hyla plugin link .
```

打开 **设置 › 插件**，它就在那里，写着它要的权限（一个包不要权限）和它的来源。*Aurora Dusk* 出现在 **设置 › 外观**，*Aurora Mist* 在它旁边，跑着 `sample-agent` 的窗格也会像 Hyla 自己的智能体一样被画出来。

改文件：link 就够了——在 **设置 › 插件** 里按 **重新加载**，或者盯着目录、保存即重载：

```sh
hyla plugin dev .
```

::: tip
包不需要重启任何东西。`hyla plugin dev` 每次改动都会重新 link 一遍，主题和图标正是这样才显示出来。
:::

## 第 2 部分 —— 一张卡片，跑在 daemon 里

现在做一个会跑起来的插件。它在信息面板画一张卡片，数按钮被点了多少次，并显示它所在的会话。

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

清单要一个卡片的挂载位置和会话状态，并让二进制跑在 `hylad` 里：

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

`hyla plugin init` 写下的 `main.go` 手写协议：一行 JSON 进、一行 JSON 出，`ui.render` 回一棵树，`ui.event` 数一次点击。

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

一张卡片的协议就这么多：`initialize` 说明你是谁，`ui.render` 返回 `{root}`，`ui.event` 再把树返回一次（`invalidate: true` 会让它再渲染），`action.invoke` 回应命令面板的动作，`shutdown` 结束。**stdout 只走 JSON-RPC，别的都不行**——多一句 `fmt.Println` 就把流弄坏了。要打日志就写 stderr，Hyla 会把它收进插件的日志里。

编译、link，然后打开某个会话的信息面板：

```sh
make                      # 按清单里的命令编译出 ./bin/plugin
hyla plugin link . --grant-all
```

`--grant-all` 把清单里要的权限一次性授予，开发时用它可以省事；别人安装你的插件时，是在 **设置 › 插件** 里点同意。

### 同一张卡片，用 SDK 写

只做一张卡片，手写循环没问题。要用智能体、时间线、设置和测试，就用 Go SDK —— `github.com/smallnest/hyla/sdk/go/hylaplugin`：

```go
plg := &hylaplugin.Plugin{
    Name:    "hello-agent",
    Version: "0.1.0",
    OnUIRender: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIRenderParams) (hylaplugin.UIRenderResult, error) {
        st, err := host.GetState(ctx)          // 需要 session.read
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

`hylaplugin.Run` 就是那个循环、握手和日志规则；`Host` 是回调 Hyla 的入口（`GetState`、`ReadScreen`、`SendInput`、`Focus`、`Notify`、`GetSettings`）。每种节点都有构造器：`Column`、`Row`、`Text`、`Badge`、`Button`、`Toggle`、`TextInput`、`Progress`、`InfoCard`。`sdk/go/hylaplugin/examples/hello-agent` 就是这个插件写全的样子，还带一个智能体和一条时间线：编译它，`hyla plugin link sdk/go/hylaplugin/examples/hello-agent --grant-all`，然后在窗格里跑 `hello-agent`。

SDK 的 `NewTestHarness(plg)` 不用 Hyla 就能调用你的处理器，SDK 自己的测试和自带的 Magpie 插件都是这么测的。

::: tip
要为运行它的那台机器编译。daemon 插件跑在 `hylad` 里，而那可能是另一台机器——**Go** 二进制得按那个平台编译，**TypeScript** 插件则跑在那台机器的 Bun 上。
:::

## 第 3 部分 —— 你自己的窗格，用 HTML 画

WebView 挂载的是 Hyla 从插件自己的来源提供的页面，跑在它自己的沙箱会话里。模板是 TypeScript，页面放在 `web/`：

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

清单用一个命令面板动作打开窗格，并把页面挂进去：

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

页面就是普通 HTML，Hyla 在它旁边提供两个虚拟地址：实时主题，和通往插件进程的桥。

```html
<link rel="stylesheet" href="/_hyla/theme.css" />
<script src="/_hyla/bridge.js"></script>
<script>
  document.getElementById("ping").addEventListener("click", async () => {
    const res = await window.hyla.rpc("ping", { hello: "hyla" });   // → 插件的 webview.rpc
    document.getElementById("out").textContent = JSON.stringify(res, null, 2);
  });
</script>
```

`/_hyla/theme.css` 把窗口的配色变成 CSS 变量（`--hyla-bg`、`--hyla-text`、`--hyla-accent`、`--hyla-divider`、`--hyla-font-ui` …），所以页面跟着主题一起变。`window.hyla.rpc(method, params)` 会变成你插件进程上的 `webview.rpc` 调用——由 `src/plugin.ts` 回应：

```ts
if (msg.method === "webview.rpc") {
  result = { data: { echo: msg.params?.method, time: new Date().toISOString() } };
}
```

跑起来：`hyla plugin link . --grant-all`，然后 **插件 › 打开 Inspector** 或者命令面板。页面跑在 `Untrusted` 的 WebView 里，CSP 很严，跨源导航被拦下，除了 `window.hyla` 之外碰不到 Hyla——插件的窗格是一个网页，不是应用本体。

想反过来由进程推给页面，就调 `webview.emit`，页面用 `window.hyla.on(event, listener)` 收。

## 第 4 部分 —— 设置与密钥

插件的设置写在清单里，Hyla 负责渲染，不用写界面代码：

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

Hyla 把它们画在 **设置 › 插件** 里该插件的抽屉中，加进设置搜索，并且负责保存：非密钥的写进插件的配置文件，`secret` 的进系统钥匙串——既不写进配置，也不打日志，更不会同步到别的机器。

插件用调用去读，而不是读环境变量：

```go
var cfg hylaplugin.PluginGetSettingsResult
if err := host.Call(ctx, "plugin.getSettings", hylaplugin.PluginGetSettingsParams{}, &cfg); err != nil {
    return err
}
endpoint, _ := cfg.Settings["endpoint"].(string)
```

`plugin.setSettings` 把值写回去——插件拿到登录令牌后刷新，就是走这条，也是 `secrets` 权限的用途。设置改了还会作为 `event.notify` 的 `settings.changed` 到来，适合缓存过读取结果的插件。

### 插件能做什么

插件每一次对外调用都要过权限门。插件在 `[permissions] scopes` 里声明它要什么，Hyla 在你 link 或安装时原样列给你看：

| 权限 | 允许什么 |
| --- | --- |
| `session.read` | `host.getState` —— 会话、窗格、焦点 |
| `pane.read` | 读窗格的屏幕和环境变量 |
| `pane.write` | 往窗格里输入、按键、上报状态 |
| `pane.focus` | 聚焦窗格 |
| `session.create` | 新建会话 |
| `agent.provider` | 提供一个智能体：识别、时间线、对话、子代理 |
| `timeline.read` | 读智能体的时间线、对话和子代理 |
| `ui.slot` | 挂载界面、渲染、接收事件、失效、打开窗格 |
| `ui.webview` | 提供 WebView 和它的桥 |
| `secrets` | 读写密钥设置 |
| `notifications` | 发通知 |
| `filesystem.workspace` | 读写窗格所在目录里的文件 |

`hyla plugin permissions <id>` 打印已授予的权限，`hyla plugin permissions <id> grant pane.write` 改它。完整表格（含哪些是高危）见[插件参考](./plugins.md#permission-scopes)。

## 第 5 部分 —— 调试、打包、分享

先看日志。插件的 stderr、生命周期和崩溃都进同一个日志：

```sh
hyla plugin logs com.example.hello -n 100      # 有 hylad 时走它
```

| 现象 | 是什么 |
| --- | --- |
| 设置 › 插件 里显示 **degraded** | 崩溃次数超过了 `max_crashes_per_minute`。修好之后按 **重启**（或 `hyla plugin disable <id> && hyla plugin enable <id>`）。 |
| 错误 `-32001` | 清单里要的某个权限没授予：`hyla plugin permissions <id> grant <scope>`。 |
| 卡片空白，或 `window.hyla` 是 undefined | 用了 `mode = "webview"` 却没有 `ui.webview`、少了 `<script src="/_hyla/bridge.js">`，或者 `web_root` 指向了错的目录。 |
| 存了文件却什么都没变 | link 看到的就是当下的文件；*包* 要按 **重新加载**、用 `hyla plugin dev`，或者重启宿主进程。 |
| Go 插件跑一下就退了 | stdout 里有不是 JSON-RPC 的东西——日志走 stderr。 |

要交给别人，就打包：

```sh
hyla plugin pack . -o aurora.hylapkg
hyla plugin install aurora.hylapkg
```

`.hylapkg` 是一个确定性的 ZIP，安装时把 SHA-256 记进注册表。安装会逐条检查路径（不许 `..`、不许绝对路径、不许符号链接）、限制文件数和解包大小，并保留二进制和钩子脚本的可执行位。**没有代码签名**——包之所以可信，是因为你装了它，并且授予了它的权限。

插件通过 Hyla 的同步在你的机器之间走，但不该走的不会走：注册表和非密钥设置会过去，密钥永远不过去，另一台机器上没有的插件到达时是**待安装**，而不是自己跑起来。

## 接下来

- [插件参考](./plugins.md) —— 清单、RPC 方法与事件、UI 节点与配色令牌、挂载点、错误码。
- `sdk/go/hylaplugin` —— Go SDK，带 `examples/hello-agent`（智能体、时间线和卡片）和 `examples/git-summary`。
- `sdk/ts` —— `@hyla/plugin-sdk` 写宿主进程，它的 `/web` 入口给 WebView 页面用。
- 自带的 `hyla.magpie` 插件（`internal/plugins/builtins/magpie`）是「读一个服务、画一张卡片、带一个设置」的样板。
- `hyla plugin --help` 列出命令行支持的全部命令。
