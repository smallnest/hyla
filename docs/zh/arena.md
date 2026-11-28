---
title: 多智能体竞技场与交叉审查
description: 让多个编程智能体在隔离工作树中同题竞速，横向对比测试与改动并择优合入，或一键发起异构对抗审查。
---

# 多智能体竞技场与交叉审查

Hyla 可以同时驾驭 Claude Code、Codex、OpenCode、Pi 和 Antigravity（`agy`）。通过 **Arena（竞技场）** 与 **Cross-Review（交叉审查）**，你可以让多个智能体在隔离工作树里同题 PK，也可以随时拉起另一个模型对当前改动做只读审查。

## Arena：多智能体同题竞速

在任意 Git 仓库中发起一场竞技场。Hyla 会为每个参赛智能体创建独立的 Git worktree 与分支，同时启动任务并实时统计状态与改动行数：

```sh
hyla arena start --agents claude,codex,agy --test "go test ./..." "为登录接口添加限流保护"
```

查看所有参赛智能体的进度、并发跑通所有工作树的测试，并一键采纳胜者：

```sh
hyla arena ls
hyla arena status <arena-id>
hyla arena test <arena-id>
hyla arena pick <arena-id> claude
```

`hyla arena pick` 会确认胜者工作区已提交干净，将其分支合入基础分支，并清理其余参赛工作树（也可传 `--no-merge` 或 `--keep-others` 保留现场）。

## Cross-Review：分栏异构对抗审查

在任意终端窗格中，一键在旁边分栏拉起另一个已安装的智能体做只读审查：

- **桌面菜单**：**窗格 › 使用其他智能体交叉审查**（macOS 快捷键 <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>V</kbd>，Windows/Linux 为 <kbd>Ctrl</kbd> <kbd>Alt</kbd> <kbd>R</kbd>）
- **命令行**：

```sh
hyla review                  # 自动选择另一个已安装智能体审查当前改动
hyla review --branch         # 审查当前分支相对主干分支的完整差异
hyla review --reviewer codex # 指定特定智能体担任审查者
```

Hyla 会自动避开当前窗格正在使用的智能体，并以只读模式要求审查者指出 Bug、边界情况、安全隐患和缺失测试。
