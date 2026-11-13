---
title: Introduction
description: Hyla is a terminal for coding agents. Sessions keep running when you quit, and the sidebar shows which ones need you.
---

# Introduction

Hyla is a terminal for macOS, Windows and Linux built for working with coding agents. You run Claude Code, Codex, a dev server or a test watcher in its own **session**. Hyla groups sessions by project in a sidebar that shows which are working, which need you and which are done.

## Sessions that outlive the window

Sessions don't run inside the app. They run in **hylad**, a small daemon that Hyla starts for you. Quit Hyla and everything keeps running. Open it again and every session, pane and scrollback is where you left it.

::: tip
Quitting Hyla (<kbd>⌘</kbd> <kbd>Q</kbd>) leaves your sessions running. To end them all, use **Hyla › Quit and End All Sessions…** (<kbd>⌘</kbd> <kbd>⌥</kbd> <kbd>Q</kbd>).
:::

## Built for agents

- **Status at a glance.** Each pane is *working*, *waiting*, *idle*, *error* or *exited*. A session that needs you rises to the top of your attention, and <kbd>⌘</kbd> <kbd>J</kbd> jumps straight to it.
- **Notifications when it matters.** Hyla notifies you when an agent finishes, fails or is blocked on you while you're looking elsewhere.
- **A CLI agents can drive.** The `hyla` command starts sessions, splits panes, types into them, waits for them and reads their screens. Agents can run work beside themselves and report their own progress.
- **Ghostty under the hood.** Terminals render with libghostty, so Ghostty themes, fonts and configuration keys behave the way you expect.

## More than a terminal

- **[Files](./files.md)** in the side panel: look at a file, or drag it into a pane, without leaving the window.
- **[Drag & drop](./drops.md)**: files into panes, panes onto panes, folders onto the sidebar.
- **[Remote machines](./remote.md)**: SSH, WSL, a Kubernetes pod or another Hyla, listed beside your own sessions.
- **[Hyla Pocket](./pocket.md)**: the same sessions on an Android phone.
- **[Appearance](./appearance.md)**: translucency, glass, pictures, and the animation between sessions.
- **[The frog](./frog.md)** on the window's edge, if you keep it.

## Next steps

- [Quick Start](./quick-start.md): install Hyla and open your first session
- [Sessions & Projects](./sessions.md): how Hyla organizes your work
- [Coding Agents](./agents.md): teach your agents to use Hyla
