---
title: Guardrails, Checkpoints & Overnight Factory
description: Coordinate parallel agents safely with Path Claims, Clash Radar, per-turn Time-Travel Checkpoints, Deny-to-Teach rules, Web Games, and Overnight Autonomous Factory.
---

# Guardrails, Checkpoints & Overnight Factory

Hyla includes four coordinated workflows for running multiple agents unsupervised without colliding edits, irreversible mistakes, or runaway loops.

## 1. Cross-Agent Path Claims & Clash Radar (`#20`)

Reserve sensitive files or directory globs for one active pane and detect branch integration conflicts before merging:

```sh
hyla claim add "internal/auth/*" --mode block --reason "Migrating auth tokens" --ttl 1800
hyla claim ls
hyla claim release <claim-id>

hyla clash check --base main --branches feat/a,feat/b --cmd "go test ./..."
```

When Claude Code or another hook-enabled agent attempts a write tool inside Hyla, `PreToolUse` queries `claim.check` and injects a context warning (`warn`) or returns `permissionDecision: deny` (`block`).

## 2. Universal Turn Time-Travel Checkpoints & Deny-to-Teach (`#23`)

Hyla captures a git tree snapshot automatically at every turn start across all coding agents, and lets you create manual snapshots, roll back safely (saving a pre-restore safety backup), or fork a worktree from any point:

- **Native GUI**: Open **Session › Time-Travel Checkpoints…** (`⌘⇧H` on macOS / `Ctrl+Alt+H` on Windows & Linux) to save manual checkpoints, restore safely, or fork an isolated worktree session.
- **Deny & Teach**: In the **Changes** panel, click **Deny & Teach** on any turn diff hunk to revert that change and permanently append a rule to `AGENTS.md` or `.hyla/rules.md`.

```sh
hyla checkpoint ls
hyla checkpoint create --label "before refactor"
hyla checkpoint restore <checkpoint-id>
hyla checkpoint fork <checkpoint-id> --branch exp/alt
hyla rule add "Never bypass parameterized SQL" --file internal/db/query.go --dest agents
```

## 3. Overnight Autonomous Factory & Runaway Circuit-Breaker (`#24`)

Batch-run GitHub issues or prompts overnight in parallel isolated worktrees, protected by wall-clock, output-size, and repeated-error loop circuit-breakers:

```sh
hyla factory start --issues 17,20,23 --agent claude --max-parallel 2 --max-minutes 45 --cmd "go test ./..." --auto-enqueue --feishu-chat <chat-id>
hyla factory status <run-id>
hyla factory digest <run-id> --feishu-chat <chat-id>
hyla factory stop <run-id>
```

Verified branches can auto-enqueue straight into `hyla mq`, and `hyla factory digest` summarizes verified, failed, and circuit-breaker tripped tasks to stdout and Feishu.

## 4. Built-in Web Games Plugin (`#17`)

Install or open **`hyla.webgames`** from **Plugins** to play 2048, Snake, Tetris, Breakout, or Minesweeper (or custom bookmarked URLs) in a WebView pane while agents work; the game automatically pauses the moment your foreground agent finishes or asks for input.
