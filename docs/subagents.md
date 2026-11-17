---
title: Subagents
description: Watch the agents your coding agents gave their work to — which run, what each was asked, and what it does now — in a pane of its own.
---

# Subagents

An agent that splits its work gives parts of it to subagents. Hyla shows them in a **subagents pane**: every agent a session's coding agents started, from when to when it ran, what it runs now, and how long it has been at it.

## Opening the pane

**Pane › Open Subagents Pane** (<kbd>⌥</kbd> <kbd>⌘</kbd> <kbd>S</kbd>, `hyla do subagents.open`) splits it beside the pane you are in. It follows the session: its panes, and the subagents of each. It splits, zooms, moves and closes like any other pane, and comes back when Hyla restarts.

## What it shows

The line at the top counts what runs and what has ended. Below it, each pane whose agent gave work away shows its subagents:

- **The kind**: what the agent calls the subagent, as `Explore` or `general-purpose`.
- **What it was asked**, in the words the agent gave it.
- **How long it ran**, or has been running.
- **The step it runs now** while it runs, such as the tool it called last.

A click on a subagent goes to its pane. The ones that still run come first; the newest of all comes first after them, up to twenty per pane.

## Where the facts come from

Hyla reads the agent's own records: Claude Code's transcript of the session, where each `Agent` call is a subagent, and OpenCode's store, where a subtask is one. A subagent that has not finished yet shows as running, because its result is not in the transcript.

Claude Code also tells Hyla as each subagent starts and ends, through the hooks `hyla skill setup` installs (`SubagentStart` and `SubagentStop`). That is what makes a subagent show the step it runs now: Hyla reads the subagent's own transcript, which the hook points it at. Hyla works without those hooks too, from the records alone, one read behind; **Settings › Integrations** says when the hooks an older Hyla installed want updating.

## What it does not show

- **Codex and DeepSeek Harness**: their records name no subagents, so their panes show none.
- **Other machines**: Hyla reads this machine's records alone, as the timeline does. A pane on another host shows no subagents.
- **A subagent's own turns**: the view shows it as one line, not the steps inside it. The [timeline](./panes.md#timeline-panes) shows the turns of the agent that started it.
