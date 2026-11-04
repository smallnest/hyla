# Hyla

A terminal for coding agents, on macOS, Windows and Linux.

Run Claude Code, Codex, a dev server or a test watcher each in its own session. Hyla groups sessions by project in a sidebar that shows which are working, which need you and which are done.

- **Sessions outlive the window.** They run in `hylad`, a small daemon, so quitting Hyla leaves them running. Reopen it and every pane and its scrollback is back.
- **Status at a glance.** Each pane is working, waiting, idle, error or exited. <kbd>⌘</kbd> <kbd>J</kbd> jumps to the one that needs you, and Hyla notifies you when an agent finishes, fails or is blocked.
- **A frog on the window's edge.** Hyla, the tree frog, peeks over the top of the window: it hops when a session needs you, reads along while an agent works, and dozes when all is calm. Press it, or turn it off in **Settings › Appearance**.
- **A CLI agents can drive.** `hyla` starts sessions, splits panes, types into them, waits on them and reads their screens.
- **Remote hosts.** Run agents on a Linux box, a cloud VM, a Mac, a Windows PC or WSL and see them in the same sidebar, over SSH or Tailscale. They keep running when your laptop sleeps. See [Remote Hosts](docs/remote.md).
- **Ghostty under the hood.** Terminals render with libghostty, so Ghostty themes, fonts and config keys just work.

## Install

Download Hyla from the [releases page](https://github.com/smallnest/hyla/releases), then choose **Hyla › Install Command Line Tool…** to put `hyla` on your `PATH`.

> **On macOS, the app is not signed with an Apple Developer ID.** The first time you open it, Control-click (or right-click) **Hyla.app** and choose **Open**, then confirm. If macOS still refuses, run:
>
> ```sh
> xattr -d com.apple.quarantine /Applications/Hyla.app
> ```
>
> Apps installed from the earlier `egoist/kero` releases do not update to these releases on their own: install one of these once.

Hyla was called **Kero** up to 2.1.37. Apps of those releases (`smallnest/kero`) do not update to Hyla on their own either: install Hyla once. The first start moves your settings and sessions to `~/.config/hyla` (`~/Library/Application Support/hyla` on macOS), and `KERO_*` environment variables keep working as `HYLA_*`.

```sh
hyla new -n fix-login -- claude
```

To run sessions on a Linux server, a Mac or a Windows PC, install the server build there, then add the machine in **Settings › Hosts**:

```sh
curl -fsSL https://github.com/smallnest/hyla/releases/latest/download/install-server.sh | sh
```

On Windows, in PowerShell:

```powershell
irm https://github.com/smallnest/hyla/releases/latest/download/install-server.ps1 | iex
```

See the [docs](docs/index.md) for more.

## Prior art

- [**Rex**](https://www.superlogical.com/rex/docs/installation): Mitchell Hashimoto's terminal and multiplexer in one native app, with sessions that persist through a server and a scriptable CLI.
- [**Herdr**](https://github.com/herdrdev/herdr): a terminal multiplexer for running many coding agents in one terminal, with agent status detection.

## License

MIT
