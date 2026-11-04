---
title: Remote Hosts
description: Run sessions on a Linux box, a cloud VM or another computer, and see them in the same sidebar as your local ones.
---

# Remote Hosts

Hyla can show sessions that run on other machines, such as a Linux dev box, a cloud VM, a Mac mini under your desk or a WSL distribution, in the same window as your local ones. They get the same sidebar, statuses, notifications and `hyla` command. Each machine runs its own hylad, so closing your laptop or quitting Hyla leaves remote agents running, as it does locally.

## Quick start

A host is any machine with Hyla on it. It can be another computer where you already use the Hyla app, or a server that has no display.

### A computer with the Hyla app

The app needs nothing installed beside it: its own hylad serves as the host. Pick how your other machines reach it:

- **Over Tailscale.** In the app on the host, turn on **Settings › Hyla Hosts › Let other machines connect**. It shows the address to add elsewhere, such as `http://studio.tail1234.ts.net:7473`. See [Tailscale](#tailscale).
- **Over SSH.** If the computer takes SSH logins (on a Mac, **System Settings › General › Sharing › Remote Login**), add it by the address you'd give `ssh`. Hyla finds the app there on its own.

Sessions on the host keep running while its app is closed, as they do on your own computer.

### A server

On a Linux box, a cloud VM or any machine without the app, install the server build of Hyla. It needs no root or administrator, no display and no GTK. On Linux or macOS:

```sh
curl -fsSL https://github.com/smallnest/hyla/releases/latest/download/install-server.sh | sh
```

On Windows, in PowerShell:

```powershell
irm https://github.com/smallnest/hyla/releases/latest/download/install-server.ps1 | iex
```

### Adding the host

On your computer, open **Settings › Hyla Hosts**, type the host's address, such as `me@devbox` for SSH or the address the host's app showed, and press **Add**. The host's projects appear in the sidebar with yours, each tagged with the host's name.

The same from the command line:

```sh
hyla hosts add devbox me@devbox
hyla -H devbox new -n review -- claude
```

::: tip
If Hyla isn't installed on a host you add, the sidebar says so and offers **Install Hyla…**, which runs the script above on the host over SSH.
:::

## How Hyla reaches a host

An address says how to reach a host. Hyla takes the same address in Settings, in the configuration file and after `hyla -H`.

| Address | How |
| --- | --- |
| `devbox`, `me@devbox`, `me@devbox:2222`, `ssh://me@devbox` | SSH, with your `~/.ssh/config`, keys and agent |
| `wsl://Ubuntu`, `wsl://` | A WSL distribution of this Windows PC, through `wsl.exe` |
| `http://devbox:7473` | hylad's listener on a tailnet address (see [Tailscale](#tailscale)) |
| `https://devbox.tail1234.ts.net` | hylad behind Tailscale Serve |
| `tailcat://tc…` | A pair code, through a relay: no Tailscale needed (see [Tailcat](#tailcat)) |

Hyla opens one connection per host and carries everything over it: the sidebar's state, every pane you look at, and the CLI's commands. The connection is compressed, and a heartbeat notices a dead link within seconds. When a link drops, as when a laptop sleeps, the host's sessions stay in the sidebar, faded, and Hyla reconnects on its own, waiting longer between tries up to a minute.

### SSH

Over SSH, Hyla runs `hyla __bridge` on the host, which starts hylad there if it isn't running and passes the connection to it. There is nothing to set up on the host beyond Hyla itself, the app or the server build, and nothing listens on the network.

Hyla never prompts for anything over SSH, as there's no one to answer in the app:

- **Keys with a passphrase** need your SSH agent. Hyla uses the agent your shell sets in `SSH_AUTH_SOCK`, as 1Password's, as `ssh` in a terminal does.
- **A host you haven't connected to before** needs its key trusted first. Run `ssh me@devbox` once in a terminal.
- **Password logins** aren't supported. Use a key.

Hyla looks for `hyla` on the host in `~/.local/bin` (on Windows, `%LOCALAPPDATA%\Programs\hyla-server`), the desktop app and its command line tool, and your `PATH`. If yours is elsewhere, name it in the address: `me@devbox?hyla=/opt/hyla/hyla`, or on Windows `me@pc?hyla=C:/hyla/hyla.exe`.

### Windows hosts

Hyla reaches a Windows machine over SSH, so turn on its OpenSSH Server first (**Settings › System › Optional features**, as an administrator). Hyla works with the server's default shell, cmd, and with PowerShell. It finds out which one the first time it connects, which costs that connection a moment.

The install script runs hylad as a task of yours in Task Scheduler, which starts it as you log on to the machine, in your session. While no one is logged on, hylad starts as Hyla connects and keeps running after the connection ends. A hylad started that way runs without your Windows password, as programs do over SSH with a key, so programs in its sessions can't reach network shares or the credentials Windows keeps for you. Log on to the machine once, or restart hylad after you do, to run them with your full account.

### WSL

On Windows, `wsl://Ubuntu` reaches the Hyla installed in that WSL distribution, as SSH would. `wsl://` is the default distribution. If Hyla isn't installed there, Hyla offers to install it, as it does over SSH.

WSL may stop a distribution a while after its last program started from Windows ends, and hylad with it. Keep agents running by keeping the distribution up, for example with [systemd enabled](https://learn.microsoft.com/windows/wsl/systemd) and `hyla daemon install-service` inside it.

### Tailscale

hylad can also listen on your tailnet, so that it needs no SSH at all. Tailscale tells hylad who connects, and by default only the Tailscale user who owns the machine gets in. On a host with the app, turn on **Settings › Hyla Hosts › Let other machines connect**, which shows the address to add elsewhere. On a server, or to set it by hand, add this to the configuration on the host:

```ini
# ~/.config/hyla/config on the host
listen = tailnet://:7473
```

Then add the host as `http://devbox:7473`. The connection is plain HTTP, which WireGuard encrypts between tailnet devices.

To serve it over HTTPS with Tailscale's certificate instead, let hylad listen on a socket and point Tailscale Serve at it:

```ini
listen = tailnet-serve://
```

```sh
tailscale serve --bg --https=443 unix:$HOME/.config/hyla/serve.sock
```

Then add the host as `https://devbox.tail1234.ts.net`. The socket is `serve.sock` in Hyla's data folder: `~/.config/hyla` on Linux, `~/Library/Application Support/hyla` on macOS.

To let others in besides the owner, list them with `listen-allow`: a tailnet login, `tag:ci` for tagged devices, or `*` for anyone on the tailnet. A tagged machine has no owner, so only `listen-allow` lets anyone in to it. Tailscale Serve doesn't tell hylad who tagged devices are: they connect to a `tailnet://` listener.

```ini
listen-allow = alice@example.com
listen-allow = tag:ci
```

hylad follows `listen` and `listen-allow` as they change, and a listener turned off drops the connections it took. A `tailnet-serve://` listener added while hylad runs waits for `hyla daemon restart`. hylad listens only when the configuration asks, and only on Tailscale or through a Tailcat relay. It refuses requests from web browsers, so a web page you visit can't reach it. Funnel's requests from the internet never get in. To keep it listening with the machine, run it as a service (below).

### Tailcat

Tailcat reaches a host with a pair code instead of Tailscale. It needs no Tailscale account, and no Tailscale installation on either machine. It uses [Tailcat](https://github.com/tailscale/tailcat), a library made from Tailscale's parts: it makes a WireGuard tunnel between the two machines, through a public DERP relay that lets them find each other, and directly once they do.

The pair code is the host's address. It contains the host's key, a secret pre-shared key and the relay's region, as a long `tc…` string. **Treat it as a password: anyone who has it can connect to the host.**

**Settings › Hyla Hosts › Tailcat** holds all Tailcat settings, in one group. The switch **Use Tailcat to Reach Other Hosts** turns them all on. With it on, the group has two parts:

- **Add a Host** adds a host that this machine reaches. Copy the pair code from the host's Tailcat group, or from `hyla hosts tailcat code` on the host. **Remote Tailcat Port** is the port of the tailcat listener of the hosts that you add: 7473 unless one of them names another.
- **Provide Tailcat Service** makes this machine a host. Hyla then shows its pair code, with a button to copy it, and **This Computer's Tailcat Listener Port** is the port that hylad listens on here. **Change the pair code** makes a new code.

The pair code does not have the port of the listener in it. The **Remote Tailcat Port** of the other machines must be the **This Computer's Tailcat Listener Port** of the host.

**Relay** is the DERP map that all machines use to find their relay. It applies to both parts.

The client needs this version of Hyla, or a later one.

The same settings from the command line:

```sh
# On the host: turn on Tailcat, then listen.
# tailcat = true goes in the configuration first.
hyla hosts tailcat enable  # or hyla hosts tailcat enable 7400
hyla hosts tailcat code      # the pair code, its secret
hyla hosts tailcat rotate    # a new pair code: the old one stops working

# On the other machine:
hyla hosts add box tailcat://tc…
```

The pair code stays the same across restarts. hylad also logs it at start:

```sh
grep 'listening on tailcat' ~/.config/hyla/hylad.log
```

The log is in `~/Library/Application Support/hyla` on macOS, and in `%LOCALAPPDATA%\hyla` on Windows.

**The switch.** Tailcat is off until you turn it on. With it off, the app lists no Tailcat host and connects to none of them, and hylad starts no `tailcat://` listener. Leave it off if you do not trust a relay: Hyla then starts nothing for Tailcat.

**The relay.** By default, Tailcat uses Tailscale's public relays at `https://tailcat.dev/derpmap.json`. Tailscale limits their rate, and does not promise they stay available. The sessions themselves do not go through the relay if the machines can reach each other directly over the network: the relay only carries what the machines use to find each other. To use a relay of your own, as Tailscale's [derper](https://github.com/tailscale/tailscale/tree/main/cmd/derper) provides, set `tailcat-relay` to the URL of your DERP map, or set **Relay** in the app. Set the same map on the host and on the other machines: the pair code keeps the relay's number, and each side looks it up in its own map.

**Changing the pair code.** **Change the pair code** in the app, or `hyla hosts tailcat rotate`, makes a new key. hylad starts its listener again with the new key, and the old code no longer connects. Add this machine again on your other machines. To remove the code from a machine, delete its key file, `tailcat.json` in Hyla's data folder.


**The keys.** hylad keeps its Tailcat key in `tailcat.json` in Hyla's data folder, which only your user can read. The pair code stays the same across restarts, while the file stays. Delete the file and restart hylad to get a new key, as **Change the pair code** does: every machine that has the old code loses the host.

## Running hylad as a service

A host with the app starts hylad as the app opens, or as Hyla connects over SSH. On a server, run hylad as a service of your own, so that it starts with the machine and keeps your sessions running while no one is logged in. The install scripts do this for you. On a Mac, the service is a launchd agent, which runs while you're logged in to the Mac, so keep a session logged in there; the script installs it when you are. On Windows, it's a Task Scheduler task, which starts hylad as you log on.

```sh
hyla daemon install-service      # a systemd user unit, a launchd agent on macOS, a task on Windows
hyla daemon status               # how it runs, and where it listens
hyla daemon uninstall-service
```

On Linux this first enables lingering (`loginctl enable-linger`), which keeps your services running after you log out. Without it, a service would stop as your last SSH session ends, which a hylad started on demand outlives, so `install-service` refuses to go on without it. If your system asks an administrator for lingering, ask them to run `sudo loginctl enable-linger <you>`, or install the service anyway with `--no-linger`.

`hyla daemon serve` runs hylad in the foreground and logs to its standard error, for a service manager of your own. It never exits for being idle. `--listen` takes the place of the configuration's `listen` keys.

## Working on a host

- **The sidebar.** It lists projects, yours first, then each host's in the order of **Settings › Hyla Hosts**. A host's projects carry its name after theirs, as `api` `devbox`, so the same repository on two machines sits side by side. A host gets a row of its own when it needs you: when Hyla can't reach it, wants updating, or isn't installed there. A host with no project shows its name with the address it is reached at under it: click it to start a session there. Right-click one of a host's projects for the host's menu: reconnect, restart hylad, update Hyla, its color, its settings.
- **Colors.** Each host has a color, picked from its name: its tag in the sidebar and its name in a pane's header wear it, so a remote pane is plain to see. Pick another from the host's menu, **Color**, or with `color=` after its address (`host = devbox=me@devbox color=teal`).
- **New sessions.** <kbd>⌘</kbd> <kbd>T</kbd> and **New Session** start a session on the host of the session you're looking at. A project's **+** starts one in it, on its machine.
- **Open Project.** With hosts added, <kbd>⌘</kbd> <kbd>O</kbd> lists folders in Hyla instead of your computer's dialog, starting on the machine of the session you're looking at; the switch at its top lists another's. Type to filter, or type a path such as `~/code`. <kbd>↩</kbd> goes into a folder, and <kbd>⌘</kbd> <kbd>↩</kbd> or **Open** opens it as a project.
- **Paths.** <kbd>⌘</kbd>-clicking a file path in a remote pane opens the file in the host's `$EDITOR`, in a pane below. A folder opens in a new session there. Hyla never opens a file of your computer that happens to have the same path. Hovering an image's path with <kbd>⌘</kbd> previews the host's image.
- **Pasting images and files.** Pasting an image or files into a remote pane copies them to a private temporary folder on the host and pastes their paths there, so agents that read image paths, such as Claude Code, keep working. Dragging files onto a remote pane still pastes your computer's paths.
- **Agents.** Inside a remote pane, `hyla` talks to that host's hylad, so the agent skill works there unchanged. Run `hyla skill setup` on the host.
- **Notifications.** A host's notifications and the window's title carry the host's name, so a remote program can't pass for a local one.

### Remote Control

`hyla focus` and `hyla do` let a program drive the app. For a host, that's code on another machine driving your computer's window, so it's off by default. Turn it on per host in **Settings › Hyla Hosts**, or with `remote-control` after the address:

```ini
host = devbox=me@devbox remote-control
```

### Settings

Appearance, keyboard and notifications are the app's. A host's shell, `env`, scrollback and session restore come from the configuration on that host, since hylad reads them there. Run `hyla config edit` on the host to change them.

## Versions and updates

The app and a host can run different versions of Hyla. When the host runs an older release than the app, its first project in the sidebar shows **Update**. It downloads the app's version of Hyla onto the host, checks it against the key Hyla's own updates are signed with, for that version and platform, and then restarts hylad there. Restarting starts the host's sessions again where they were, with their history, and the programs running in them end, so Hyla asks first.

- Hyla updates a host only when you ask, never on a reconnect, and never back to an older version.
- When the host runs a newer Hyla than the app, update the app instead (**Settings › General**).
- Hyla updates an install of its install scripts. If you installed Hyla on the host another way, update it the way you installed it.
- A host running the Hyla desktop app takes updates from your other machines once you let it: on that computer, turn on **Settings › Hyla Hosts › Let them update Hyla here** (`remote-updates = true`). The app there then updates to the newest release, relaunches, and restarts hylad. Until then, update it on that computer.

From the command line:

```sh
hyla -H devbox version           # this hyla's version, and the host's
hyla -H devbox daemon status
hyla -H devbox daemon update     # to this hyla's version
hyla -H devbox daemon restart
hyla daemon update               # on the host, to the latest release
hyla daemon update 2.1.0         # on the host, to a version, older ones too
```

## The command line

`-H` (or `--host`), before the command, points a command that acts on sessions at a host's hylad. It takes a host's name, an address, or `local` for this machine. `$HYLA_HOST` does the same, except inside a pane, where `hyla` acts on the pane's own machine. Commands of this machine alone, such as `hyla config` and `hyla hook`, ignore `$HYLA_HOST`.

```sh
hyla hosts                       # the hosts you added (--json for JSON)
hyla hosts add build ci@build.lab --remote-control
hyla hosts check build           # connect, and say how it went; a name or an address
hyla hosts install build         # install Hyla there, over SSH or WSL
hyla hosts remove build
hyla ls --all                    # the sessions here and on every host
hyla -H build ls
hyla -H build prompt review "run the tests and fix them"
hyla clients                     # who is connected to this machine's hylad
```

The app and the CLI share one list of hosts, the configuration's `host` keys:

```ini
host = devbox=me@devbox
host = build=ssh://ci@build.lab:2222 remote-control
host = ubuntu=wsl://Ubuntu
```

| Variable | What it does |
| --- | --- |
| `HYLA_HOST` | The host commands act on, as `-H` |
| `HYLA_SSH` | The `ssh` program Hyla runs, in place of the one on your `PATH` |
| `HYLA_RELEASES` | A mirror of Hyla's releases, for the install scripts and `hyla daemon update` |

## Security

- **A connection can do anything its account can,** including starting programs, as a local one can. Over SSH it's your own account, as SSH logs you in. Over Tailscale, only who the machine's owner lets in.
- **The socket is yours alone.** hylad's socket is created readable by its user only, and hylad refuses connections from other users' processes.
- **Updates are signed.** The server build updates itself only from archives signed with the key of Hyla's releases.
- `hyla clients` lists who is connected to a hylad, with how they came: `uid 501 over unix`, `me from 10.0.0.5 over ssh`, `alice@example.com over tailnet`.

## Limits

- The server build is for Linux, with glibc 2.28 or later, macOS and Windows 10 (1809) or later, on x86-64 and ARM64. A machine with the Hyla app needs no server build.
- Dragging files onto a remote pane pastes your computer's paths. Paste them instead.
- One host has one account. Login servers, where several people reach their own accounts on one machine, are planned.
