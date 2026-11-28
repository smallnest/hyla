---
title: Multi-Agent Arena & Cross-Review
description: Race multiple coding agents on the same task in isolated worktrees, compare tests and diffs, and trigger adversarial cross-reviews.
---

# Multi-Agent Arena & Cross-Review

Hyla drives Claude Code, Codex, OpenCode, Pi, and Antigravity (`agy`) side by side. With **Arena** and **Cross-Review**, you can either race multiple agents on the same prompt and pick the best result, or invite a second model to audit an implementation in place.

## Arena: race agents in parallel

Start an arena in any git repository. Hyla creates an isolated worktree and branch for each agent, launches them concurrently with the same prompt, and tracks their live status and git diff metrics:

```sh
hyla arena start --agents claude,codex,agy --test "go test ./..." "Add rate limiting to the login handler"
```

Inspect progress, run the test suite concurrently across every contestant's worktree, and merge the winner:

```sh
hyla arena ls
hyla arena status <arena-id>
hyla arena test <arena-id>
hyla arena pick <arena-id> claude
```

`hyla arena pick` verifies the winner's worktree is clean, merges its branch back into the base branch, and cleans up the losing worktrees. Pass `--no-merge` or `--keep-others` to inspect branches before merging.

## Cross-Review: adversarial audit in a split pane

From any terminal pane, open a read-only adversarial review beside it using a different installed agent:

- **Menu**: **Pane › Cross-Review with Another Agent** (<kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>V</kbd> on macOS, <kbd>Ctrl</kbd> <kbd>Alt</kbd> <kbd>R</kbd> on Windows/Linux)
- **CLI**:

```sh
hyla review                  # review uncommitted changes with another installed agent
hyla review --branch         # review full branch diff against main/master
hyla review --reviewer codex # choose a specific reviewer
```

Hyla picks an installed agent different from the pane's current agent and instructs it to audit the diff read-only without modifying files.
