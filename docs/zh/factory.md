---
title: 多智能体护栏、时间旅行与夜间工厂
description: 通过路径认领与冲突雷达、逐轮时间旅行检查点、否决并立规（Deny-to-Teach）、内置网页游戏与夜间自治工厂熔断机制，安全并行驱动多智能体。
---

# 多智能体护栏、时间旅行与夜间工厂

Hyla 提供四项协同机制，让你放心同时运行多个编程智能体甚至夜间无人值守批处理，杜绝互相踩踏、不可逆改坏与死循环烧钱。

## 1. 跨智能体路径认领与行为冲突雷达（`#20`）

为指定窗格独占文件、目录或通配符路径，并在合并前用一次性沙盒工作树预先演练多分支集成与测试：

```sh
hyla claim add "internal/auth/*" --mode block --reason "Migrating auth tokens" --ttl 1800
hyla claim ls
hyla claim release <claim-id>

hyla clash check --base main --branches feat/a,feat/b --cmd "go test ./..."
```

当 Hyla 窗格内的 Claude Code 等智能体触发写操作时，`PreToolUse` Hook 会实时查询 `claim.check`：命中 `warn` 模式会注入冲突警告，命中 `block` 模式直接返回拒绝拦截。

## 2. 全智能体时间旅行检查点与“否决并立规”（`#23`）

Hyla 会在每个智能体每轮对话开始时自动记录一次 Git working tree 快照（并可随时手动保存），支持带安全备份的零损耗回滚或原地分叉实验工作树：

- **桌面端 GUI**：通过 **会话 › 时间旅行检查点…**（macOS `⌘⇧H` / Windows 与 Linux `Ctrl+Alt+H`）打开面板，一键保存、回滚或分叉。
- **否决并立规（Deny & Teach）**：在 **Changes** 面板的任意代码 hunk 上点击 **否决并立规**，立即回退该改动并把避坑原因永久追加到 `AGENTS.md` 或 `.hyla/rules.md`。

```sh
hyla checkpoint ls
hyla checkpoint create --label "before refactor"
hyla checkpoint restore <checkpoint-id>
hyla checkpoint fork <checkpoint-id> --branch exp/alt
hyla rule add "禁止拼接未参数化 SQL" --file internal/db/query.go --dest agents
```

## 3. 夜间自治工厂、暴走熔断与飞书晨报（`#24`）

按最大并发数批量在独立工作树中并行实现多个 GitHub Issue，配合墙上时钟超时、输出体积上限与重复报错死循环熔断器：

```sh
hyla factory start --issues 17,20,23 --agent claude --max-parallel 2 --max-minutes 45 --cmd "go test ./..." --auto-enqueue --feishu-chat <chat-id>
hyla factory status <run-id>
hyla factory digest <run-id> --feishu-chat <chat-id>
hyla factory stop <run-id>
```

通过验收的分支可自动入队 `hyla mq`，并在结束时向终端与飞书发送结构化晨报。

## 4. 内置网页小游戏窗格（`#17`）

在插件市场启用 **`hyla.webgames`**（内置 2048、贪吃蛇、俄罗斯方块、打砖块、扫雷及自定义网页书签）。当智能体从运行中切换到空闲或等待审批时，游戏会自动暂停并提醒切回主窗格。
