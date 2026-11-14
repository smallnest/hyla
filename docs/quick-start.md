---
title: Quick Start
description: Install Hyla, open a project and start your first agent session.
---

# Quick Start

Install Hyla, open a project, and run your first agent in a session that keeps going after you quit.

## Install

Download the latest Hyla from the [releases page](https://github.com/smallnest/hyla/releases). On macOS, drag **Hyla.app** into your Applications folder; on Windows, run the **Hyla Setup** installer, which needs no administrator rights.

Or with Homebrew, which fetches the Apple Silicon or Intel disk image for you:

```sh
brew tap smallnest/hyla
brew trust smallnest/hyla
brew install --cask hyla
```

The cask lives in a tap of its own, and Homebrew 7 asks you to trust a tap outside `homebrew-cask` once; `brew trust` is that answer.


On Windows, sessions run in PowerShell 7 when it is installed, else Windows PowerShell; choose another shell, such as `cmd` or Git Bash's `bash`, in **Settings › Shell**. Hyla needs Windows 10 1809 or later, for its pseudo consoles.

### Install the command line tool

Choose **Hyla › Install Command Line Tool…** to put `hyla` on your `PATH`: a link in `/usr/local/bin` on macOS, in `~/.local/bin` on Linux, and on Windows Hyla's `bin` folder added to your user `PATH`, for terminals you open from then on. Run it in any terminal to check:

```sh
hyla help
```

Typing `hyla` alone in a terminal opens the app.

## Open a project

Press <kbd>⌘</kbd> <kbd>O</kbd> and choose a folder. Hyla adds it to the sidebar as a project and opens a session there.

You can also start from the command line. A new session goes into the project that matches its directory:

```sh
cd ~/code/my-app
hyla new --focus
```

## Start an agent

Run your agent in the session the way you would in any terminal:

```sh
claude
```

Or give it a session of its own with a name you'll recognize in the sidebar:

```sh
hyla new -n fix-login -- claude
```

While the agent works, its session shows as **working**. When it stops to ask you something, it turns to **waiting** and Hyla notifies you. Press <kbd>⌘</kbd> <kbd>J</kbd> to jump to it.

## Quit without losing anything

Press <kbd>⌘</kbd> <kbd>Q</kbd>. Your sessions keep running in hylad. Open Hyla again and they're all there, with their scrollback.

## Next steps

- [Panes](./panes.md): split a session to run a dev server beside your agent
- [Coding Agents](./agents.md): run `hyla skill setup` so agents learn to use Hyla
