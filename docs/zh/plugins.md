---
title: 插件
description: 用声明式的资源包、进程外的 JSON-RPC 2.0 插件、原生声明式界面和沙箱 WebView 窗格扩展 Hyla。
---

# Hyla 插件系统

Hyla 的插件系统让开发者无需重新编译 Hyla，就能扩展终端智能体识别、界面外壳、命令面板、自定义窗格和外部会话通道。

::: tip
刚开始接触插件？[写一个插件](./plugin-tutorial.md)把三层各做一遍——主题与智能体包、daemon 里的卡片、WebView 窗格——并讲清设置、日志与打包。
:::

## 架构（三层）

插件按需要组合这几层，可以只取其中一层：

1. **第 1 层 —— 声明式包（`hyla-plugin.toml` + 静态资源）**
   零进程、可热重载，由 Hyla 直接加载：
   - 智能体识别规则（`detect/*.toml`）
   - 智能体图标（`marks/*.svg|png|webp`）
   - 终端配色主题（`themes/*.ghostty`）
   - 窗口背景（`washes/*.toml`）
   - 桌面宠物（`pets/*`，带 `pet.json`、`FrameW=192`、`FrameH=208`、`Columns=8`）
   - 编程智能体的钩子与技能（`[[hooks]]`、`[[skills]]`）
   - 静态动作与快捷键（`[[actions]]`）
2. **第 2 层 —— 进程外 JSON-RPC 2.0 宿主**
   长驻或按需启动的插件进程，通过 **NDJSON（每行上限 16 MiB）** 经 **stdio** 或 **本地 Unix socket / Windows 命名管道**通信：
   - **Daemon 宿主（`hylad` / `host = "daemon"`）**：负责智能体识别、时间线、对话、子代理、恢复命令、通道驱动，以及跨会话的宿主状态查询，包括无头服务器 `hyla-server`。
   - **App 宿主（`Hyla.app` / `host = "app"`）**：负责本地界面交互、应用动作、原生声明式 UI 挂载和 WebView RPC。
3. **第 3 层 —— 界面载荷（声明式 UI 与沙箱 WebView）**
   - **3A 声明式 UI（`mode = "declarative"`）**：JSON UI 树，由 `mygo/ui` 原生渲染，配色取自 Hyla 实时的调色板令牌（`$text`、`$accent`、`$card` 等）。
   - **3B 沙箱 WebView（`mode = "webview"`）**：HTML/JS/CSS 挂在 `hylaplugin://<plugin-id>/...`，在隔离的 `Untrusted` WebView 会话里（`"plugin:" + id`），自动注入 `/_hyla/theme.css` 和 `/_hyla/bridge.js`。

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

## 插件 ID 与目录布局

### 插件 ID 规则

每个插件 ID 必须是小写的、DNS 安全的倒域名，至少两段，用点分隔（不许下划线或大写）：

```text
^[a-z0-9][a-z0-9-]{0,61}[a-z0-9](\.[a-z0-9][a-z0-9-]{0,61}[a-z0-9])+$
```

合法示例：`com.example.hello-agent`、`org.acme.git-radar`、`hyla.magpie`。
非法示例：`my_plugin`、`hello`、`Com.Example.Test`。

### 隔离的目录布局

插件文件都在 `<HYLA_HOME>`（默认 `~/.hyla`）下，不会混进 Hyla 主配置的校验里：

| 路径 | 用途 | 是否跨机器同步 |
| --- | --- | --- |
| `~/.hyla/plugins/<id>/` | 解包后的插件目录（`hyla-plugin.toml`、资源、二进制） | 否（缺失的目标标为 `pendingInstall`） |
| `~/.hyla/plugins.json` | 已安装/已链接/自带插件的注册表、启用状态、SHA-256、已授权限 | 是（去掉 `linkPath`） |
| `~/.hyla/plugin-config/<id>.json` | 与 `[[settings]]` 对应的非密钥配置 | 是 |
| `~/.hyla/plugin-secrets/` | 密钥值（无头 `hyla-server` 在系统钥匙串不可用时退化为 `0600` 文件；桌面版用 `mygo.Secrets` 的 `"hyla-plugin:" + id` 服务） | **永不** |
| `~/.hyla/plugin-data/<id>/` | 插件的暂存/缓存目录，作为 `dataDir` / `HYLA_PLUGIN_DATA_DIR` 传入 | 否 |
| `~/.hyla/logs/plugins/<id>.log` | 轮转的 stdout/stderr/生命周期日志（4 MiB 轮转） | 否 |

---

## 清单规范（`hyla-plugin.toml`）

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
transport = "stdio"          # "stdio"（默认）| "socket"
auto_start = false           # 首次 RPC 时才启动；true 则随宿主启动
keep_alive = false           # true 时即使空闲也让 hylad 常驻
min_restart_ms = 500
max_crashes_per_minute = 5   # 超过进入 degraded 状态

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
view = "main"                # target = "open_pane" 时的挂载 ID

[[ui.slots]]
id = "main"
mount = "pane"               # info.card | side.tab | pane | sidebar.menu | files.menu | pane.menu | palette | settings.plugins | settings.page | settings.<page>
title = "Demo"
mode = "declarative"         # "declarative" | "webview"（settings.* 只支持声明式）
host = "app"                 # "app" | "daemon"
icon = "marks/icon.svg"
web_root = "web"             # webview 必填
entry = "index.html"         # web_root 内的入口文件
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
options = []                 # type = "select" 时的候选值

[env]
CUSTOM_FLAG = "1"
```

### 动作 ID 的限定

`[[actions]]` 里的动作会自动限定为 `<plugin-id>.<action-id>`（若 `id` 已以 `<plugin-id>.` 或 `plugin.` 开头则不变）。全局动作和快捷键可以直接引用这个限定后的 ID，无需改动 Hyla 的静态动作表。

### 权限范围

每个从插件到宿主的 RPC 都经过宿主 RPC 网关，网关按 `plugins.json` 里的 `grantedPermissions` 校验，未授予时返回 JSON-RPC 错误 `-32001`（`ErrCodePermissionDenied`）。

| 权限 | 高危 | 允许访问 |
| --- | --- | --- |
| `session.read` | 否 | `host.getState` |
| `pane.read` | 否 | `pane.readScreen`、`pane.getEnv` |
| `pane.write` | **是** | `pane.sendInput`、`pane.sendKey`、`pane.reportStatus` |
| `pane.focus` | 否 | `pane.focus` |
| `session.create` | **是** | `session.create` |
| `agent.provider` | 否 | `agent.detect`、`agent.resumeCommand`、`agent.sendChat`、智能体注册（同时授予 `agent.readTimeline`/`agent.readChat`/`agent.readSubagents`） |
| `timeline.read` | 否 | `agent.readTimeline`、`agent.readChat`、`agent.readSubagents` |
| `ui.slot` | 否 | `ui.invalidate`、`ui.openPane`、`ui.render`、`ui.event`、`action.invoke`、挂载点与菜单 |
| `ui.webview` | 否 | `hylaplugin://` WebView 托管、`webview.rpc`、`webview.emit` |
| `network` | 否 | 声明需要访问外网 |
| `secrets` | **是** | `plugin.getSettings` 里带出、`plugin.setSettings` 写入的密钥 |
| `notifications` | 否 | `ui.notify` |
| `filesystem.workspace` | **是** | 访问窗格引用到的工程文件 |

### 包里的文件：清单指向的格式

清单只是给文件起名，这些是文件的格式。

#### 智能体识别（`detect/*.toml`）

`[[agents]]` 的 `processes` 说明窗格里跑的是哪个程序，识别文件说明它跑着的时候屏幕意味着什么：

```toml
id = "my-agent"

[[rules]]
id = "my-agent.permission"
state = "blocked"            # idle | working | blocked | unknown
visible_blocker = true       # 把会话升到等待，并通知
priority = 10
contains = ["Do you want to proceed?"]
line_regex = ["^\\s*[›>]\\s"]
```

一条规则的门要全部成立：`contains`（子串，忽略大小写）、`regex`、`line_regex`（匹配某一行的正则），以及可以互相嵌套的 `all`、`any`、`not`。`region` 只读屏幕的一部分：

| 字段 | 含义 |
| --- | --- |
| `state` | 智能体处于什么状态：`idle`、`working`、`blocked` 或 `unknown` |
| `visible_*` | `visible_idle`、`visible_working`、`visible_blocker`：该状态要显示出来，而不是沿用上一次显示的状态 |
| `skip_state_update` | 屏幕匹配了，但状态保持原样——例如盖在「工作中」之上的阻塞提示 |
| `priority` | 多条规则同时命中时谁优先，数字大的先 |

被显示出来的 `blocked` 规则，会把会话变成*等待*，在边栏里提前，并通知你。

智能体的 `mark` 是 16×16、用 `currentColor` 画的 SVG，所以会跟着显示它的地方取色；`mark_full_color = true` 则保留 SVG 自己的颜色。

#### 主题（`themes/*.ghostty`）

就是 Ghostty 主题文件，按 Ghostty 的写法：`background`、`foreground`、`cursor-color`、`selection-background`，以及 `palette = 0=#…` 到 `15=#…`。

#### 背景（`washes/*.toml`）

窗口的底色，明暗各一组渐变：

```toml
id = "aurora-mist"          # 不能与 Hyla 自带的背景重名
name = "Aurora Mist"
light = ["#f4f7fb", "#e6eefb"]   # 渐变的色标
light_glow = "#cfe5ff"           # 窗口后面的光
light_angle = 135                # 角度
dark = ["#141824", "#1b2438"]
dark_glow = "#263b5e"
dark_angle = 135
```

#### 宠物（`pets/<dir>/pet.json`）

一张精灵图，旁边一份 `pet.json`：

```json
{
  "id": "pixel-fox",
  "displayName": "Pixel Fox",
  "name": "Pixel Fox",
  "description": "A fox that watches the panes",
  "spritesheetPath": "sheet.webp",
  "spriteVersionNumber": 2
}
```

每帧 **192×208** 像素，**一行 8 帧**，一行一个动画，顺序是：idle、向右跑、向左跑、挥手、跳、失败、等待、工作、审查（版本 2 追加向右看、向左看）。一行的前几帧可能就是这个动画的全部——idle 只用 6 帧——其余留空。

#### 钩子与技能

```toml
[[hooks]]
agent = "claude"      # claude | codex
event = "Stop"        # 该智能体自己的事件名
script = "hooks/on-stop.sh"

[[skills]]
agent = "claude"
name = "my-skill"
dir = "skills/my-skill"
```

钩子会装进那个智能体自己的钩子配置，随它的事件运行。技能会被复制进该智能体的技能目录，智能体就能像读自己其他技能那样读它。插件被禁用或删除时，两者一起撤掉。

### 挂载点与动作何时出现

`[[ui.slots]]` 或 `[[actions]]` 上的 `when = "…"` 说明它什么时候出现。条件用 `,` 或 `&` 连接——都要成立——`!` 取反：

```toml
when = "pane.agent,!agent:claude"
```

| 原子 | 何时成立 |
| --- | --- |
| `session.selected` | 有会话正在显示 |
| `project.selected` | 该会话属于某个项目 |
| `pane.selected` | 该会话有焦点窗格 |
| `pane.terminal` | 焦点窗格跑的是终端 |
| `pane.agent` | 其中跑着编程智能体 |
| `pane.browser`、`pane.file`、`pane.plugin` | 它是浏览器、文件或插件窗格 |
| `agent:<id>` | Hyla 认出的智能体是 `<id>`，例如 `agent:claude` |
| `always` | 总是——默认值 |

动作的 `target` 说明谁来处理：`app` 和 `daemon` 交给该宿主里插件的进程，`open_pane` 打开 `view` 指定的挂载点。`menu` 把它放进那个菜单，`palette = true` 放进命令面板，`shortcut` 绑快捷键，`remote = true` 允许远程会话触发它。

---

## 第 2 层：进程外 JSON-RPC 2.0 协议

### 传输与认证

1. **`stdio`（默认）**：宿主在独立的进程组里拉起 `command`（Unix 上用 `Setpgid`；Windows 上做进程/作业清理）。请求、应答、通知都是按行分隔的 JSON（`\n`），**每行上限 16 MiB**。`stderr` 的输出会收进 `~/.hyla/logs/plugins/<id>.log`。用 `stdio` 的插件**只能往 `stderr` 打日志**，绝不能写 `stdout`。
2. **`socket`**：宿主建一个本地 Unix socket（或 Windows 命名管道），在进程环境里导出 `HYLA_PLUGIN_SOCKET` 和 `HYLA_PLUGIN_TOKEN`，并要求插件连上后的第一行是：

```json
{"token":"<HYLA_PLUGIN_TOKEN>"}
```

另外还会导出这些环境变量：
- `HYLA_PLUGIN_ID`
- `HYLA_PLUGIN_DIR`
- `HYLA_PLUGIN_DATA_DIR`
- `HYLA_PLUGIN_HOST`（`"daemon"` 或 `"app"`）

### 生命周期

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

### 错误码

| 码 | 常量 | 含义 |
| --- | --- | --- |
| `-32700` | `ErrCodeParse` | JSON 帧不合法 |
| `-32600` | `ErrCodeInvalidRequest` | JSON-RPC 信封不合法 |
| `-32601` | `ErrCodeMethodNotFound` | 方法未实现 |
| `-32602` | `ErrCodeInvalidParams` | 参数不合法 |
| `-32603` | `ErrCodeInternal` | 插件或宿主内部错误 |
| `-32001` | `ErrCodePermissionDenied` | 缺少已授予的权限 |
| `-32002` | `ErrCodeRemoteControlDisabled` | `remoteControl` 关闭时 `pane.focus` 被拦下 |
| `-32003` | `ErrCodeTargetNotFound` | 会话、窗格或挂载点不存在 |
| `-32004` | `ErrCodeCapabilityUnsupported` | 目标不支持该能力 |
| `-32005` | `ErrCodeTimeout` | RPC 超时 |
| `-32006` | `ErrCodePayloadTooLarge` | 帧超过 16 MiB |

### 宿主 → 插件的 RPC

- **生命周期**：`initialize`、`initialized`（通知）、`shutdown`、`exit`（通知）
- **智能体**：`agent.detect`、`agent.readTimeline`、`agent.readChat`、`agent.sendChat`、`agent.readSubagents`、`agent.resumeCommand`
- **界面与动作**：`ui.render`、`ui.event`、`action.invoke`、`webview.rpc`
- **事件（`event.notify`）**：`session.state`、`pane.status`、`pane.osc`、`theme.changed`（在 `InitializeResult.subscriptions` 里订阅后才推送）

### 插件 → 宿主的 RPC

- **状态与终端**：`host.getState`、`pane.readScreen`、`pane.getEnv`、`session.create`、`pane.sendInput`、`pane.sendKey`、`pane.reportStatus`、`pane.focus`
- **界面与通知**：`ui.invalidate`、`ui.openPane`、`ui.notify`、`webview.emit`
- **配置与密钥**：`plugin.getSettings`、`plugin.setSettings`

### Daemon RPC 扩展与协议兼容

Hyla 保持 `proto.Version = 4`，并在 `Welcome.Capabilities` 里声明 `"plugins.v1"`：
- `plugin.list` —— 返回运行时状态（`[]rpc.PluginStatus`）
- `plugin.call` —— 转发到某个 daemon 插件（`{pluginId, method, params}`）
- `plugin.reload` —— 在 `hylad` 上重载清单、包和进程
- `plugin.restart` —— 重置熔断状态并重启 `{pluginId}`
- `plugin.logs` —— 读取插件日志的末尾（`{pluginId, lines}`）

连到不支持 `"plugins.v1"` 的旧 `hyla-server` 时，桌面端会跳过远程 `plugin.*` 调用。在远程主机上（`remoteControl == false`），远程插件不能触发本地窗口动作、选择会话、改变焦点或在本地打开窗格。

### 进程管理与熔断

- 普通的 `hylad` 插件（`keep_alive = false`）**不会**拦住 `hylad` 30 秒的空闲退出。
- 只有启用了 `keep_alive = true` 且正在运行的插件（或有活动 `ChannelDriver`）才让 `hylad` 常驻。
- 崩溃次数超过 `max_crashes_per_minute`（默认 60 秒 5 次）会触发熔断，插件进入 `degraded` 状态，直到被重启（`hyla plugin`、设置界面或 `plugin.restart`）。

---

## 第 3A 层：声明式 UI JSON

插件返回 `UIRenderResult`，里面的 `UINode` 树由 Hyla 的原生界面引擎异步渲染（`internal/plugins/uihost`）。

### 支持的节点类型（23）

- **布局**：`column`、`row`、`box`、`spacer`、`divider`、`scroll`、`list`
- **容器**：`infoCard`、`settingsGroup`、`settingsRow`
- **展示**：`text`、`richText`、`badge`、`icon`、`image`、`spinner`、`progress`
- **控件**：`button`、`toggle`、`segmented`、`select`、`textInput`、`slider`

### 调色板令牌与字体

`style.background`、`style.border`、`style.color`、`props.color` 或 text 的色调里，颜色可以写成 `$token` 或 `$token@alpha`（如 `"$accent@0.2"`），按当前主题实时解析：

- `$bg`、`$sidebar`、`$card`、`$chip`、`$popover`、`$control`
- `$text`、`$muted`、`$faint`、`$accent`、`$attention`、`$done`、`$danger`、`$divider`

`UIStyle` 支持 `width`、`height`、`minWidth`、`minHeight`、`maxWidth`、`maxHeight`、`padding`（`[all]`、`[v,h]` 或 `[t,r,b,l]`）、`gap`、`radius`、`flex`、`align`（`start|center|end|stretch`）、`justify`（`start|center|end|between`）、`background`、`border`、`borderWidth`、`color`、`fontSize`、`fontWeight`、`monospace`、`tabular`、`wrap`。

### 挂载点

| 挂载 | 位置 |
| --- | --- |
| `info.card` | 右侧信息面板里的一张卡片 |
| `side.tab` | 侧栏里与文件 / 信息 / 更改并列的自定义页签 |
| `pane` | 完整的工作区窗格（`proto.CreateParams{Plugin, PluginView, PluginTitle, PluginArgs}`） |
| `sidebar.menu` | 左侧边栏的会话右键菜单 |
| `files.menu` | 文件页签里文件/目录的右键菜单 |
| `pane.menu` | 窗格标题栏的右键菜单 |
| `palette` | 命令面板里的命令 |
| `settings.plugins` | **设置 › 插件** 里该插件抽屉内的自定义声明式 UI |
| `settings.page` | 设置窗口左侧独立的一页 |
| `settings.<page>` | 追加到某个内置设置页的声明式 UI 区块（`settings.general`、`settings.appearance`、`settings.terminal`、`settings.shell`、`settings.browser`、`settings.notifications`、`settings.keyboard`、`settings.feishu`、`settings.agents`、`settings.sync`、`settings.hosts`、`settings.advanced`） |

自定义 `pane` 会话会随 `hylad` 重启保留（`savedPane` / `restorePane`）。插件被禁用、缺失或在某台机器上待安装时，Hyla 会画出一张可操作的「暂不可用」卡片，带一个关闭按钮。

### 扩展设置

插件有四种方式扩展设置窗口：

1. **结构化的 `[[settings]]` 字段**：写在 `hyla-plugin.toml` 里（`string`、`secret`、`boolean`、`integer`、`number`、`select`）。Hyla 自动渲染在 **设置 › 插件**（以及插件的第一个 `settings.page`，如果声明了），非密钥值存进 `~/.hyla/plugin-config/<id>.json`，密钥存进凭据存储，并把每一项的 `title`、`key`、`description` 收进设置搜索（选中结果会自动展开该插件的抽屉）。
2. **`mount = "settings.plugins"`（抽屉 UI）**：已启用且拿到 `ui.slot` 的插件，可以在 **设置 › 插件** 的详情抽屉里用 `settingsGroup` / `settingsRow` / `button` 渲染可交互的声明式控件（例如状态指示或「测试连接」按钮）。
3. **`mount = "settings.page"`（独立设置页）**：已启用且拿到 `ui.slot` 的插件，用挂载点的 `title` 和 `icon`（插件里的 SVG，缺省用默认插件图标）在设置左侧加一个一级入口，也能从设置搜索和命令面板进入。插件被禁用或删除时，设置会干净地回退。
4. **`mount = "settings.<page>"`（内置设置页的区块）**：已启用且拿到 `ui.slot` 的插件，可以把声明式 UI 追加到任意内置设置页（`settings.general`、`settings.appearance`、`settings.terminal`、`settings.shell`、`settings.browser`、`settings.notifications`、`settings.keyboard`、`settings.feishu`、`settings.agents`、`settings.sync`、`settings.hosts`、`settings.advanced`）。

所有 `settings.*` 挂载点都用 3A 声明式 UI 原生渲染（`mode = "declarative"`，需要 `ui.slot`；设置类挂载不支持 `mode = "webview"`）。调 `ui.invalidate`，或让 `ui.event` 返回 `{root, invalidate, message}`，就能立刻在设置窗口里重绘或给出提示。

---

## 第 3B 层：沙箱 WebView（`hylaplugin://`）

`mode = "webview"` 的挂载点通过一个全局自定义协议加载，注册在 `MainWindow` 创建之前：`hylaplugin://<plugin-id>/...`。

- 每个插件一个隔离的 `Untrusted` WebView 会话（`mygo.NewSession("plugin:" + id)`）。
- 严格的 CSP 头锁死外部导航与连接（`default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'none'; object-src 'none'`）。
- WebView 内的跨源导航由 `OnWillNavigate`（`e.PreventDefault()`）拦下。
- 插件来源下自动提供这些虚拟地址：
  - `/_hyla/theme.css` —— 实时的 CSS 变量（`--hyla-bg`、`--hyla-sidebar`、`--hyla-card`、`--hyla-chip`、`--hyla-popover`、`--hyla-control`、`--hyla-text`、`--hyla-muted`、`--hyla-text-muted`、`--hyla-faint`、`--hyla-accent`、`--hyla-attention`、`--hyla-done`、`--hyla-danger`、`--hyla-divider`、`--hyla-font-ui`、`--hyla-font-mono`）。
  - `/_hyla/bridge.js` —— 注入 `window.hyla.rpc(method, params, slotId)` 和 `window.hyla.on(event, listener)`。
  - `POST /_hyla/rpc` —— 用 `mygo.CallerPage(r.Context())` 认证调用方，把 `webview.rpc` 派发给插件宿主进程。
- 宿主往页面推送用 `Event.EmitPage`，频道是 `"hyla:plugin"`（Go 里是 `host.WebViewEmit`，TS 里是 `host.webviewEmit`）。

---

## 官方 SDK

### Go SDK（`sdk/go/hylaplugin`）

引入 `github.com/smallnest/hyla/sdk/go/hylaplugin`（`sdk/go/hylaplugin/go.mod`）：

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

单测可以用 `hylaplugin.NewTestHarness(plg)`（`Call(ctx, method, params, &out)`、`RegisterHostHandler(method, fn)`、`Close()`）。智能体相关的处理器返回 `Timeline`（`Turns []TimelineStep`）、`Chat`（`Lines []ChatLine`）和 `[]Subagent`，与 Hyla 的线上格式一致。

### TypeScript / Bun SDK（`sdk/ts`）

零依赖的 TypeScript SDK，导出两个入口：
- `@hyla/plugin-sdk`：`runPlugin`、`attachPluginPeer`、`RPCPeer`、`HostClient`、`ui.*` 构造器、带类型的 RPC 载荷。
- `@hyla/plugin-sdk/web`：浏览器侧的 `rpc`、`onEvent`、`onThemeChanged`、`readThemeTokens`。

---

## 三个完整示例

### 示例 1：第 1 层声明式包（主题 + 智能体识别 + 图标）

```sh
hyla plugin init --template pack --id org.acme.aurora ./aurora-pack
cd aurora-pack
hyla plugin link .
```

改 `hyla-plugin.toml`（以及 `detect/*.toml`、`marks/*.svg`、`themes/*.ghostty`、`washes/*.toml`）。保存后通过 `hyla plugin dev .` 或 **设置 › 插件 › 重新加载** 重载即可生效。

### 示例 2：第 2 层 Go 智能体与声明式 UI 插件

见 `sdk/go/hylaplugin/examples/hello-agent` 里可直接运行的例子：

```sh
cd sdk/go/hylaplugin/examples/hello-agent
go build -o hello-agent .
hyla plugin link . --grant-all
```

它声明了：
- `hyla-plugin.toml` 里的 `agent.provider`、`ui.slot` 和 `session.read` 权限。
- `[[agents]]`（`processes = ["hello-agent"]`、`timeline = true`）。
- `[[ui.slots]]`（`mount = "info.card"`、`mode = "declarative"`、`host = "daemon"`）。

### 示例 3：第 3 层 TypeScript + 沙箱 WebView 窗格

```sh
hyla plugin init --template ts-webview --id org.acme.inspector ./inspector-webview
cd inspector-webview
hyla plugin link . --grant-all
```

在 Hyla 里打开 **插件** 菜单或命令面板（`Cmd+K`），运行 **Open inspector-webview**。`web/index.html` 里的页面通过 `/_hyla/theme.css` 自动继承 Hyla 的实时主题，并通过 `/_hyla/bridge.js` 调用 `await window.hyla.rpc("ping", { hello: "hyla" })`，由 `src/plugin.ts` 里的 `webview.rpc` 处理。

---

## 命令行参考（`hyla plugin`）

```text
hyla plugin ls [--json]                         列出自带、已链接、已安装和待安装的插件
hyla plugin init [--template pack|go|ts-webview] [--id <id>] [--name <name>] [dir]
hyla plugin pack <dir> [-o out.hylapkg]         把插件目录打成 .hylapkg
hyla plugin install <pkg.hylapkg|dir> [--grant-all] [--no-enable]
hyla plugin link <dir> [--grant-all]            以符号链接方式注册本地开发目录
hyla plugin dev <dir> [--grant-all]             链接并随文件变化热重载
hyla plugin enable <id>                         启用插件并通知 daemon
hyla plugin disable <id>                        禁用插件并停止宿主进程
hyla plugin rm <id> [--purge]                   删除插件（--purge 连配置/数据/日志一起删）
hyla plugin permissions <id> [grant|revoke <scope…>|--all]
hyla plugin logs <id> [-n N]                    打印插件日志末尾（有 hylad 时走它，否则读本地文件）
```

---

## 打包与安全模型（`.hylapkg`）

`hyla plugin pack` 生成确定性的 `.hylapkg` ZIP，SHA-256 记在 `plugins.json` 里。`InstallPackage` 会强制：

- 拒绝 `..` 路径穿越、绝对路径和压缩包里的符号链接。
- 限制文件数与解压后字节数（防 zip 炸弹）。
- 保留二进制和钩子脚本的 `0755` 可执行位。
- **没有强制代码签名**，也**没有操作系统级沙箱**；安全靠路径校验、用户显式授权（`grantedPermissions`）和 WebView 的来源/会话隔离。

---

## 跨机器同步与自带插件

### 官方自带插件

Hyla 自带一个默认启用的官方插件：
- **`hyla.magpie`**：显示 Magpie 编程智能体网关的指标（`info.card` 挂载点 + `Refresh Magpie Usage` 命令面板动作 + 可配置的网关地址）。

飞书是 Hyla 自己的功能，不是插件：它的机器人跑在 hylad 里，在 **设置 › 飞书** 里配置。曾用 3.17.0 的 `hyla.feishu` 插件配置过的安装，会保留那份配置，本版本首次启动时把它搬进飞书设置并删除该插件。

### 跨机器同步

Hyla 的加密同步（`Snapshot` v1）包含：
- `~/.hyla/plugins.json`，去掉本机的 `linkPath` 字段。若某个同步来的插件在另一台机器上不存在，Hyla 会把它标成 **`pendingInstall: true`** 和 **`enabled: false`**，而不是悄悄下载外部代码。
- `~/.hyla/plugin-config/<id>.json` 里的非密钥设置。
- **`~/.hyla/plugin-secrets/` 严格排除**在快照之外，永不离开本机。

---

## 排查

- **插件显示 `degraded`**：看 `hyla plugin logs <id>` 或 **设置 › 插件 › 日志**。修好后点 **重启**，或执行 `hyla plugin disable <id> && hyla plugin enable <id>`。
- **权限错误 `-32001`**：用 `hyla plugin permissions <id>` 看已授予的权限，用 `hyla plugin permissions <id> grant <scope>` 或 **设置 › 插件** 补授。
- **WebView 空白或 `window.hyla` 未定义**：确认 `[[ui.slots]]` 设了 `mode = "webview"`、`[permissions]` 含 `"ui.webview"`、`web/index.html` 里有 `<script src="/_hyla/bridge.js"></script>`，并且 `web_root` 指向正确的目录。
- **远程主机上看不到插件动作**：确认远程的 `hyla-server` 声明了 `"plugins.v1"`（把 `hyla-server` 升到支持插件的版本）。
