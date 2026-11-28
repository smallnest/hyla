---
title: Merge Queue & Browser Verification Gate
description: Land parallel agent worktrees safely with a train-based merge queue, anti-cheat diff guard, zero-LLM browser replay, and Feishu visual approval cards.
---

# Merge Queue & Browser Verification Gate

Parallel coding agents write code fast, but landing multiple branches into `main` without breaking tests, losing assertions, or introducing UI regressions is the hard part. Hyla solves this by pairing **deterministic browser replay** with a **local merge queue and anti-cheat guard**.

## 1. Record & replay browser flows (zero LLM)

Use the **Record** / **Replay** buttons in any Hyla browser pane toolbar to start/stop recording navigation steps or open the Replay dialog, or drive interactive verification from the CLI into a portable JSON flow:

```sh
hyla browser record start --name checkout -o .hyla/flows/checkout.json
hyla browser open http://localhost:3000/checkout
hyla browser click '#pay-now'
hyla browser wait --text "Order confirmed"
hyla browser screenshot
hyla browser record stop
```

Replay that exact flow later with **zero LLM calls** and compare screenshots pixel-by-pixel against the saved baseline:

```sh
hyla browser replay .hyla/flows/checkout.json --update-baseline
hyla browser replay .hyla/flows/checkout.json --feishu-chat <chat-id>
```

## 2. Local Merge Queue (`hyla mq`)

Manage queued agent branches visually from **Session › Merge Queue…** (<kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>M</kbd> on macOS / <kbd>Ctrl</kbd> <kbd>Alt</kbd> <kbd>M</kbd> on Windows/Linux)—including Enqueue, Run Queue, Approve/Reject anti-cheat flags, and jumping to bounced agent panes—or via CLI:

Queue finished agent worktree sessions and land them in trains of up to 4 branches inside a temporary integration worktree:

```sh
hyla mq add <session-id> --check "go test ./..." --flow .hyla/flows/checkout.json
hyla mq ls
hyla mq run --train 4 --cleanup --browser <browser-pane> --feishu-chat <chat-id>
```

How `hyla mq run` handles failures:

- **Merge conflicts**: the conflicting lane is aborted and bounced back to its originating agent pane with the error so the agent can rebase and fix it.
- **Train check failures**: if a combined train fails its test or browser replay, Hyla resets the integration worktree and tests each lane individually—landing clean lanes and bouncing only the culprit.
- **Escalation**: after repeated bounces (`--max-bounces`, default `2`), a lane moves to `needs_review`.

## 3. Anti-cheat diff guard & Feishu visual approval

Before merging any lane, Hyla inspects `git diff <base>...<branch>` for:

- Deleted test functions or assertions without replacement (`deleted_test`)
- Newly added test skips such as `t.Skip`, `.skip(`, or `#[ignore]` (`test_skip`)
- Tampering with CI workflows, linter configs, or `package.json` scripts (`config_tamper`)

Flagged lanes are held in `needs_review`. When `--feishu-chat` is set, Hyla uploads the verification screenshot and sends an interactive Feishu approval card with **Approve & Requeue** and **Reject** buttons, or you can approve locally via `hyla mq approve <lane-id>`.
