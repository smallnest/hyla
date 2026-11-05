---
title: Sessions & Projects
description: How Hyla groups terminal sessions by project and keeps them running in hylad.
---

# Sessions & Projects

Hyla organizes your work in two levels: projects (folders) hold sessions (tasks), and every session keeps running in hylad until you close it.

## Sessions

A **session** is a unit of work: one task, one agent, one long-running process. It holds one or more [panes](./panes.md), each running a shell or a program.

| Action                     | Shortcut                                     |
| -------------------------- | -------------------------------------------- |
| New session                | <kbd>⌘</kbd> <kbd>T</kbd>                    |
| New session in home folder | <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>T</kbd>       |
| Go to session…             | <kbd>⌘</kbd> <kbd>P</kbd>                    |
| Resume a conversation…     | <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>O</kbd>       |
| Jump to session needing you | <kbd>⌘</kbd> <kbd>J</kbd>                   |
| Next / previous session    | <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>]</kbd> / <kbd>[</kbd> |
| Rename session             | <kbd>⌘</kbd> <kbd>R</kbd>                    |
| Close session              | <kbd>⌘</kbd> <kbd>⇧</kbd> <kbd>W</kbd>       |

Name sessions after their task, such as `fix-login-redirect` or `review-pr-42`, so you can find them in the sidebar and target them from the [CLI](./cli.md).

### Where a new session starts

The `new-session-directory` setting picks the folder:

- `current` (default): the directory of the session you're in
- `project`: the project's root folder
- `home`: your home folder

### How an unnamed session is named

Until you or an agent name a session, the `session-name` setting (**Name sessions after** in Settings) picks what the sidebar shows:

- `focused` (default): the title of the program in the pane you're in. A pane running only its shell defers to the pane that needs you most.
- `lead`: the title of the program in the pane that needs you most, such as an agent that's working or waiting on you
- `folder`: the first pane's folder, whatever runs in it

## Projects

A **project** is a folder. Hyla groups sessions under their project in the sidebar. Open one with <kbd>⌘</kbd> <kbd>O</kbd>, drag folders from your file manager into the sidebar, or let `hyla new` pick it by the directory a session starts in. Dropping folders opens one session per unique project; folders already registered reuse their project. Native folder drops always open on This Computer, even when a remote session is selected. Files and inaccessible folders are rejected. On macOS, file packages such as `.app` bundles are rejected too, matching the Open Project folder picker.

```sh
hyla projects            # list projects
hyla new -p my-app       # start a session in a project by name
```

### SSH hosts

The sidebar's foot has **Add SSH Host**. Type an address as ssh takes it, such as `root@example.com:2222`, and a name (the address's host by default): Hyla opens a terminal running `ssh` to that machine. It is ssh in a terminal like any other, with your `~/.ssh/config`, keys and agent; the machine needs an SSH server and nothing else, no Hyla.

All the machines live in one project, **SSH Hosts**, with the host's icon. Each row is a machine, its name over the address ssh runs to; a click shows its terminal, or starts one. The project's **+** or the sidebar's **Add SSH Host** adds another machine, and a right-click removes one.

## hylad

Sessions run in **hylad**, a daemon that the app starts on demand. Hyla is a window onto hylad:

- Quitting Hyla leaves every session running.
- Opening Hyla again shows every session as it was, scrollback included.
- After hylad itself restarts, such as after a reboot, Hyla starts your sessions again in their directories. Turn this off with `restore-sessions = false`.
- A pane that was running Claude Code, Codex, Pi, OpenCode or DSH resumes its conversation, as the agent's own resume option would, once [Hyla's skill](./agents.md#install-the-skill) is set up for that agent. Other programs don't start again unasked: their pane comes back at a shell. Turn this off with `resume-agents = false`.

```sh
hyla daemon status       # is hylad running?
hyla daemon restart      # restart it (sessions start again)
hyla daemon stop         # stop it and end every session
```

### Quitting

Quitting Hyla keeps the sessions running, so that opening it again shows them, as they were. To have quitting end them, as **Quit and End All Sessions** does, set `quit-ends-sessions = true`. Then every quit ends this machine's sessions and their programs, and stops hylad. This applies to the close button, ⌘Q and the menu. Hyla asks first while a program runs, as `confirm-close` says. Sessions on other machines keep running.

```ini
quit-ends-sessions = true
```
