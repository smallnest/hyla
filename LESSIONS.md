# Hyla 开发经验与教训（Lessons Learned）

本文记录在开发与演进 Hyla（桌面端、守护进程 `hylad`、插件系统、时间线解析与 CI/Release 流水线）过程中沉淀的工程经验与踩坑教训，供后续迭代与维护参考。

---

## 一、架构分层与构建生成规范

### 1. 严格遵守 `internal/arch` 单向分层与根包禁令
- **教训**：新增 `internal/plugins/market` 时，起初在仓库根目录创建了 `plugin_market_embed.go`（`package hyla`）并从 `market` 导入 `"hyla"`，同时未及时将新包登记到 `internal/arch/arch.go`，导致 `internal/arch` 的 `TestListed` 与 `TestLayers` 检查阻塞。
- **经验**：
  - `internal/arch/arch.go` 对 `internal/*` 包按 `base -> wire -> engine -> daemon -> ui -> cli` 强制单向分层，`TestListed` 和 `TestLayers` 会跨 `darwin` / `linux` / `windows` 三平台校验。
  - `internal/*` **禁止导入模块根包 `"hyla"`**（`arch.Check` 视为环依赖：`if imp == Module`），非 `internal/` 的独立子包（如 `locales`）不受此限。
  - 若需要用 `//go:embed` 内置仓库根目录的配置文件（如 `plugin_market.json`），最整洁的做法是在包内保留同步副本并通过单测用 `bytes.Equal` 强校验其与根目录源文件字节一致（如 `TestEmbeddedCatalogValid`），同时将新包录入 `arch.Layers` 和 `arch.layerNames`。

### 2. 修改 `hylad` 相关代码后必须重新生成 `internal/buildinfo`
- `internal/buildinfo/buildinfo.go` 通过 `//go:embed _hyladSource` 内嵌守护进程源码指纹，用于在客户端与 `hylad` 版本比对。
- 每次修改 `internal/daemon`、`internal/proto`、`internal/timeline`、`internal/plugins/*` 等被 `hylad` 依赖的包后，必须执行 `go generate ./internal/buildinfo` 再构建或跑全量测试；切勿手动编辑 `_hyladSource/`。

### 3. 本地多语言目录修改必须通过工具生成与校验
- 不要手改 `locales/catalog_gen.go`。修改 `locales/{en,zh-Hans,ja}.json` 后，统一运行：
  ```sh
  go tool mygo generate
  go tool mygo i18n check
  ```
- 翻译术语需全仓一致（例如中文界面将「面板」统一规范为「窗格」）。

---

## 二、MyGo UI、线程模型与测试实战

### 1. 后台任务回写 UI 必须切回主线程
- 下载插件包、异步拉取远程 `plugin_market.json`、调用 `host.OpenPane` / `SetWin.Invalidate()` / `reloadPlugins` 等操作，后台 goroutine 完成后**必须通过 `a.main(func() { ... })`** 或 `a.InvalidatePluginUI()` 回到主线程修改状态和刷新窗口。

### 2. 无头单测中避免直接触发原生窗口对象方法
- **教训**：在 `internal/plugins/uihost` 测试语言/主题广播事件时，若构造空结构体 `new(mygo.Page)` 并直接调用 `PluginWebEvent.EmitPage`，会因底层访问原生窗口状态而触发 `mygo.(*Window).IsDestroyed` panic。
- **经验**：为原生桥接调用抽出一层可替换的函数钩子（如 `emitPageFn`，默认指向 `PluginWebEvent.EmitPage`）并共用 `broadcastAll` 等逻辑，既不破坏真实运行链路，又能让单元测试安全覆盖广播与缓存逻辑。

### 3. UI 布局与弹窗的常见边界陷阱
- **提前 `return` 导致下方分组消失**：在 `Settings › Plugins`（`pluginsPage`）中，当 `len(installed) == 0` 时曾直接 `return`，结果导致其下方的「插件市场」分组无法展示。条件分支渲染空状态卡片后应继续执行后续分组。
- **确认弹框长文本换行**：[confirm.Dialog](file:///Users/smallnest/ai/hyla/internal/ui/confirm/confirm.go) 中多行权限清单或长说明必须链式调用 `.Wrap()`，否则在固定宽度浮层中可能截断或溢出。
- **单测子进程超时抖动**：`TestPluginSettingsRealAppSubprocessE2E` 等涉及启动真实插件子进程的端到端测试（轮询窗口约 900ms）在系统并发跑多组重型编译/测试时容易因 CPU 抢占出现偶发 `"Loading…"` 超时；排障时应先确认是否为并发负载导致，再单独或顺序重跑验证。

---

## 三、插件机制与独立 WebView 插件演进

### 1. 启用/禁用状态与运行时生命周期严格对齐
- 启动时与运行中切换 `Enabled` 开关，都需要保持一致语义：`enabled == true` 自动启动并进入 `running`；`enabled == false` 必须停止进程进入 `stopped`，同时禁用界面上的「重启（Restart）」按钮，防止对已禁用插件误操作。

### 2. 内置窗格插件化时的兼容与解耦原则
- 将时间线从内置 `timelineview` 迁移为独立插件 `plugins/timeline`（`hyla.timeline`）时，保留并补齐以下能力才能无缝升级：
  - **旧会话与动作兼容**：保留 `timeline.open` 动作/快捷键向 `hyla.timeline` 的转发，以及历史保存会话字段 `savedPane.Timeline` 向插件窗格的兼容迁移。
  - **数据读取复用**：保留 `internal/timeline` 与服务端 `pane.getTimeline` RPC，供插件、Chat 视图与 Subagents 视图共同消费，避免在插件内重复维护各 Agent 日志解析逻辑。
  - **原生窗格能力对等**：扩展 `UIOpenPaneParams`（支持 `Split` 分栏目标与 `FocusExisting` 聚焦已有窗格）和 `WebViewRPCParams.Args`，让第三方 WebView 插件也能拥有接近原生窗格的交互体验。

### 3. 插件国际化（i18n）要打通完整链路
- 插件自身做 i18n 不够只翻译 Web 页面，需要从宿主机到前端全链路打通：
  1. `hyla-plugin.toml` 支持 `[i18n.<locale>]` 覆盖 `name` / `description` / `actions` / `slots` / `settings`；宿主侧用 `InLanguage` 生成本地化副本且不污染基础快照。
  2. `ActionInvokeParams` 与 `WebViewRPCParams` 携带运行时 `Locale`，WebView URL 附带 `?locale=` 参数。
  3. 宿主通过 `Coordinator.ApplyLocale` 与 `WebBridge.BroadcastLocaleChange` 在用户切换语言时通知插件后端和已打开的 WebView 实时刷新。

### 4. 避免界面元素与窗格标题栏冗余
- Hyla 窗格标题栏本身已经展示正在跟踪的 Agent 名称（如 `claude`、`agy`），WebView 内部顶栏再放置 Agent 徽章与状态点会造成视觉重复，应精简顶部工具栏，仅在存在多个可选目标窗格时展示切换下拉框以及图例和刷新按钮。

---

## 四、插件市场、安全边界与跨平台 CI/Release

### 1. 插件市场的安全底线
- **URL 与协议限制**：对 `download_url`、`sha256_url`、`homepage` 以及外链打开入口强制校验 `http`/`https` 协议与非空 Host，拒绝 `file://` 等非法 scheme。
- **校验与防篡改**：下载 `.hylapkg` 后校验流式计算的 SHA-256，并在调用 `Store.InstallPackage` 前先校验压缩包内 `hyla-plugin.toml` 的 `id` 与市场条目完全一致，防止URL被替换后覆盖安装其他插件。
- **最小权限确认**：点击「安装/更新」前必须弹出确认框清晰列出声明的权限及用途描述；安装时只授予市场确认清单中的 `Permissions`（`GrantPermissions: p.Permissions`），禁止在未审核包内新增权限时使用 `GrantAll: true`。
- **在线优先 + 内置兜底**：默认从 GitHub 拉取最新 `plugin_market.json`（支持 `HYLA_PLUGIN_MARKET_URL` 环境变量覆盖测试），网络异常时平滑降级到 `//go:embed` 内置目录并明确提示离线状态。

### 2. 跨平台插件构建与发布规范
- **统一清单写法 + Windows `.exe` 回退**：`hyla-plugin.toml` 只需写 `"./bin/hyla-timeline"`，由 `daemonhost.resolveCommandForOS` 在 Windows 下自动回退查找 `bin/hyla-timeline.exe`，避免为不同操作系统维护多份清单。
- **干净打包与权限位**：通过 `scripts/build-plugins` 交叉编译 6 平台（`CGO_ENABLED=0`）时，仅将运行时资产（清单、静态前端、新编译二进制）写入临时 staging 目录，剥离 `*.go`、`go.mod`、本地 `bin/` 与旧 `.hylapkg`；对 Unix 二进制显式 `os.Chmod(..., 0o755)` 后再调用 `manifest.Pack`，并用 `Store.InstallPackage` 解包自检。
- **Release 防漏检机制**：在 `.github/workflows/release.yml` 的 `assets` 校验任务中运行 `go run ./scripts/build-plugins -list-assets`，动态比对 Release 附件，确保所有平台插件包及 `.sha256` 齐全后才执行自动 publish。

---

## 五、Coding Agent 时间线解析实战经验

在为 `claude`、`codex`、`dsh`、`opencode` 以及 Antigravity CLI（`agy` / `antigravity`）实现时间线解析时，踩过不少细节坑：

1. **进程名与展示名别名要全覆盖**
   - `agy` 与 `antigravity` 对应同一 Agent 的不同启动方式，必须在 `StartsTimeline`、`timeline.Find`、`daemon/timeline.go`、`daemon/chat.go` 及 `daemon/subagents.go` 等所有分派点同时兼容两个名称，避免漏接导致对话或子代理视图误走默认 Claude 解析器。
2. **绝不在 `append` 期间缓存切片元素指针**
   - 解析 JSONL 流时往 `turns[cur].Children` 持续 `append` 会触发底层数组扩容迁移。若用 `map[string]*Step` 记录进行中的工具调用，一旦扩容指针即失效。应改为保存索引结构（如 `{turn, child}`），通过索引安全读写原切片元素。
3. **适配真实存储与参数格式怪癖**
   - **双层编码参数**：`agy` 的 `tool_calls[].args` 字段内部常为二次 JSON 编码字符串（如 `"\"/path\""`），需逐层反序列化解包才能正常显示参数摘要。
   - **只读与子代理隔离**：读取 `conversation_summaries.db` 必须使用 `openStore`（`mode=ro` + `busy_timeout`），并在查询时过滤 `coalesce(parent_conversation_id, '') = ''`，防止工作区匹配误选中子代理的会话。
   - **符号链接与跨平台路径**：macOS 上 `/tmp` 与 `/private/tmp` 等软链差异会导致路径直比失败，目录匹配需结合 `filepath.Clean` 与 `filepath.EvalSymlinks`，并正确处理 `file:///C:/...` URI 前导斜杠。
   - **结构化状态精确识别**：如 `Command exited with code N` 只能匹配响应正文首行，不可全文子串搜索（防止 `read_file` 读到源码里的同类字面量被误判为命令失败）。
4. **不要把空闲时间算进已结束的 Turn 时长**
   - **教训**：初版 `Agy` 将 `lastTime` 初始化为 `start`（或用下一条 `USER_INPUT` / `now` 作为已结束轮次的 `End`），导致历史已完成轮次的耗时被拉长到数分钟。
   - **经验**：已完成的轮次应截断在其最后一个子步骤或最终 `PLANNER_RESPONSE` 的真实时间戳上，仅在仍有未关闭工具、运行中后台任务或待确认 `ask_question` 时才保持 `End == 0`。

---

## 六、日常开发、本地安装与发布纪律

- **本地体验安装需签名且勿误杀宿主进程**：本地打包 `/Applications/Hyla.app` 后需保证签名为 `Hyla Release Signing`（避免 macOS 权限记忆丢失）；更重要的是，当开发会话本身运行在 Hyla 终端窗格内时，绝对不能执行 `killall Hyla` 或杀死 `Hyla __hylad`，否则会中断当前宿主机进程与会话。
- **主分支集成与 Fork 特性守护**：严格按照 [AGENTS.md](file:///Users/smallnest/ai/hyla/AGENTS.md) 在 `master` 上工作，不做冗余特性分支；每次合并上游或大改 UI/动作后，务必复核表内各项 fork 独占特性（侧边栏 Files/排序徽章、窗口透明度与毛玻璃、等宽字体选择器、桌宠、退出结束会话）代码与对应单测是否完好。
- **先过全部门禁再汇报**：任何代码改动后的最终结论只引用最后一次编辑之后实际运行通过的测试、静态检查与构建产物，不凭记忆引用符号或跳过校验。
