# Plan: Hyla on remote servers

Status: Phases 1 and 2 are implemented (2026-10-22), with the changes to the
plan below. Phases 3 and 4 are still to do. Written 2026-10-20 after looking
at Rex (superlogical.com/rex), which ships a Linux server mode, multi-user
servers and remote access over Tailscale. Updated after 2.0 shipped for macOS,
Windows and Linux.

## What shipped, and how it differs

User docs: [docs/remote.md](../docs/remote.md).

- **Every platform is a host.** The server build ships for Linux, macOS and
  Windows, on amd64 and arm64, each archive installed and run on its own
  platform in CI (.github/workflows/server.yml). Windows is reached over
  OpenSSH: the bridge's `sh -c` has none of cmd's special characters, so
  cmd and PowerShell say they have no sh, which tells Hyla to run the
  bridge as that shell takes it, from then on. hylad runs there as a Task
  Scheduler task at logon, through `conhost --headless`, and an update
  moves the running files aside, as Windows can't replace them.
- **SSH and WSL are transports too, beside HTTP.** Herdr (herdrdev/herdr)
  reaches machines only over SSH: `ssh host herdr remote-client-bridge` pipes
  stdio to the remote daemon's socket. That needs nothing set up on the host
  but the binary, so it's the easiest first step for most people. Hyla does
  the same with `hyla __bridge`, and with `wsl.exe` for a WSL distribution.
  The objections below to SSH as *the* transport still hold, so the tailnet
  listeners are there for always-on servers, and are where an iOS client
  would connect.
- **One multiplexed connection, whatever the transport.** A connection opens
  with a `mux` Hello, then carries yamux streams. Each stream is a local
  socket connection as before (control or attach), so the protocol and
  `proto.Version` are unchanged. Over HTTP, the connection is a WebSocket.
  `MuxVersion` versions the mux layer alone, and the host streams
  (`host.restart`, `host.update`) work across protocol versions, so the app
  can always tell the user what's wrong and update the host.
- **Compression is DEFLATE across the whole connection**, flushed on every
  write, rather than of attach frames alone. The state, sent whole on every
  change, compresses as well as the frames.
- **Heartbeats** are yamux pings every 5 seconds, with a link that doesn't
  answer for 10 seconds counted as dead, as Herdr does.
- **Version skew:** the app offers **Update** for a server build of another
  version. It installs the app's version, signed with the updates' key, then
  restarts hylad after asking. It never updates on a reconnect.
- **Listeners:** `tailnet://:port` (identity from tailscaled's whois) and
  `tailnet-serve://` (Serve's headers, on a 0600 socket). Built-in tsnet is
  not done: it would add Tailscale's whole client to hylad. Requests with an
  `Origin`, or without Hyla's header, are refused, so no web page can reach
  hylad through a browser on the tailnet.
- **Settings split:** documented and noted in Settings rather than edited
  remotely. `hyla config edit` on the host edits the host's.
- Not done from Phase 2: uploading files dragged onto a remote pane (MyGo's
  terminal pastes dropped paths itself; pasting them works).

## Goal

Run sessions on another machine, such as a Linux dev box, a cloud VM or a
beefy desktop, and see them in the same Hyla window as local ones: the same
sidebar, statuses, notifications, `hyla` CLI and agent skill. Quitting Hyla or
closing the laptop leaves remote agents running, as it does locally.

Not goals, for now: sharing one session between two people, a web client, an
iOS client.

## Where we start

hylad is already most of a server:

- It owns the sessions and runs them on their own. Quitting the app leaves them
  running.
- Each pane has its own headless libghostty-vt emulator (`internal/daemon/session.go`).
  An attach stream carries rendered diffs from `terminal.Mirror`, not raw PTY
  bytes. A client that falls behind is dropped and reattaches from a fresh
  frame. Both suit slow or lossy links better than forwarding a PTY.
- Control is newline-delimited JSON (`Hello`, then `{id, method, params}`), and
  attach uses binary frames (`internal/proto/frame.go`). Neither cares what
  carries it.
- `pty_linux.go` and `proc_linux.go` exist, and libghostty-vt ships for linux
  amd64 and arm64.

What ties it to one machine today:

- **Transport:** a Unix socket only (`proto.SocketPath`), mode 0600.
  - There's no authentication and no peer-credential check. The 0600 chmod
    comes after `listen`, so there's a short window before it applies.
- **Lifetime:** hylad exits 30s after its last session and connection close,
  and the app starts it on demand (`client.Connect` runs `hyla __hylad`).
- **Packaging:** there's one binary. `main.go` imports the UI, and the daemon
  imports `mygo` only for `mygo.App.Version()`.
  - Releases ship for macOS, Windows and Linux, on arm64 and amd64. Linux gets
    a `.deb`, plus an `install.sh` that installs into `~/.local` without root.
  - Every one is the full app. On Linux it links GTK, so a headless box needs
    GTK installed just to run hylad.
- **The app assumes local paths:**
  - The Open Project dialog picks a local folder.
  - ⌘-clicking a file path runs `os.Stat` and opens the file locally.
  - Image paste gives programs local file paths.
  - `ShortDir` and display names compare against the local home.
- **Shell lookup:** `internal/shell/account_unix.go` reads `/etc/passwd`
  directly, so LDAP and SSSD accounts are missed.

## Phase 1: headless hylad on Linux

The smallest useful step: hylad running as a service on a Linux box, driven by
the `hyla` CLI there. That already helps agents on that box, before any remote
UI exists.

1. **A daemon-only build** (`cmd/hylad`, or a build tag) without the UI
   packages.
   - Pass the version in with `-ldflags` instead of `mygo.App.Version()`.
   - Check that it starts on a machine with no display.
2. **`hyla daemon serve`:** run in the foreground, with no idle exit, logging
   to stderr. This is the entry point for systemd.
3. **`hyla daemon install-service`:** write and enable a systemd *user* unit
   (`~/.config/systemd/user/hylad.service`), plus `loginctl enable-linger` so
   it survives logout.
   - Remove the service with `hyla daemon uninstall-service`.
4. **Release artifacts:** the release workflow already builds linux
   amd64/arm64 `.deb`s and `install.sh` from the same tags as the other
   platforms. Add a server flavour beside them:
   - a tarball with the daemon-only `hyla` and `libghostty-vt.so`, and no GTK
     dependency;
   - `install.sh --server`, which installs that build and runs
     `hyla daemon install-service`.

   Arch packages and a Nix flake come later, on demand.
5. **Fixes that matter once a box has several users:**
   - Set the umask before `listen` instead of chmodding after.
   - Check the peer's uid with `SO_PEERCRED` and reject other users.
   - Look up the shell through NSS (`getent passwd`) instead of parsing
     `/etc/passwd`.

## Phase 2: remote access over the network, one user

The app connects to a hylad on another machine that runs as the same person.

### Transport: HTTP, with Tailscale saying who connects

hylad gets network listeners beside its Unix socket, off by default. As in
Rex, there's no SSH: the connection is HTTP(S), and the network says who the
caller is.

- **One connection per client, multiplexed.** Locally, the app opens one
  socket for control and one per visible pane. Over a network, that would
  mean a handshake per pane and per reconnect.
  - Instead, the client opens one HTTP connection and carries the existing
    control JSON and attach frames as streams in it: WebSockets, or HTTP/2
    streams. A spike picks one.
  - The Unix socket keeps today's protocol.
- **Why HTTP:** Tailscale Serve proxies HTTPS to a local socket, so an HTTP
  transport works with every Tailscale setup below. HTTP also passes
  corporate proxies, and it's what an iOS client would speak.
- **Why not SSH:**
  - It needs a process and a channel per connection.
  - It renegotiates on every reconnect, which happens each time a laptop
    wakes.
  - It depends on each user's `~/.ssh/config`.
  - It can't sit behind Tailscale Serve, and it gives no path to iOS.

  SSH keys may come back later as one way to authenticate (Phase 4), not as
  the transport.

### Listeners

Each listener lets in only callers it can identify:

| Listener | Needs | Identity from |
| --- | --- | --- |
| Built-in Tailscale (`tsnet` in hylad) | A Tailscale account; the machine needs no Tailscale | `WhoIs` on the embedded node |
| Tailscale Serve, to a local socket | Tailscale on the machine | The headers Serve adds for tailnet users |
| Tailnet address, such as `tailnet://:<port>` | Tailscale on the machine | `whois` on the local tailscaled API |

- By default, only the Tailscale user who owns the machine gets in. An
  allowlist, or a flag, lets in other tailnet members.
- Serve doesn't identify tagged devices, such as CI runners. They use a
  tailnet address instead.
- No anonymous listener at first. Rex has one behind `--allow-anonymous` for
  people who bring their own VPN. Wait until someone asks.
- Tailscale's ACLs do the network-level restriction. Hyla doesn't need its own
  rules for that.

### Protocol changes

- `Hello` negotiates **compression** for attach frames (zstd, or deflate from
  the standard library). Rendered diffs compress well, and they cost bandwidth
  only on scroll-heavy output.
- **Version skew:** today a client refuses a hylad of another
  `proto.Version` and offers to restart it. That's fine locally, but a remote
  hylad can't be restarted quietly, because it may hold someone's
  long-running agents.
  - Show it per host, with "Update Hyla on <host>". That has hylad fetch the
    matching release, using the updater the app already has, then restart
    when the user confirms.
  - Hyla is released now, so hosts will run older versions than the app for
    days at a time. Decide whether the app speaks the previous
    `proto.Version` too, or only offers the update. Either way, the UI has to
    explain the mismatch rather than fail.
- **Heartbeats** on the connection, so a dead link shows as "reconnecting"
  within seconds, and the app reattaches when it comes back.

### App changes

- **Hosts:**
  - Settings › Hosts lists remote hylads by address, such as
    `https://build.<tailnet>.ts.net`.
  - In the config file: `host = name=address`.
  - The sidebar groups projects by host, with the local machine first and
    unlabeled. Each host has its own connection state.
- **Open Project on a host:** a folder picker served by a new `fs.list` RPC.
  The NSOpenPanel stays for the local machine.
- **⌘-click a path in a remote pane:** resolve it on the host (`fs.stat`).
  Open it there in `$EDITOR` in a new pane, or copy the path. Never open a
  local file with the same name.
- **Image paste into a remote pane:** upload the image to a temp dir on the
  host (`paste.file`), then paste the *remote* path, so TUIs that take image
  paths keep working.
- **Settings split:**
  - Appearance and keyboard come from the app.
  - Shell, `env`, scrollback and `restore-sessions` come from the host's own
    config, since hylad reads them there.
  - Settings shows which is which, and `hyla config` edits the host's.

### CLI and agents

- `hyla -H <host> ls` and `HYLA_HOST` drive a remote hylad over the same
  transport. `hyla hosts add|ls|remove` saves addresses.
- Inside a remote pane, `hyla` already talks to the remote hylad through
  `HYLA_SOCKET`, so the skill works there unchanged. `hyla skill setup` needs
  running on the host.
- **Remote Control from a remote agent:** `hyla focus` and `hyla do` reach the
  local app, which is a new kind of trust (code on a server driving my
  laptop's UI). Add a per-host toggle, off by default for remote hosts.

## Phase 3: multi-user servers

Rex has three server modes:

- **Personal:** one hylad per user.
- **Shared:** one service account that a group connects to.
- **Login:** many OS users, with one front door that routes each to their own
  account.

For Hyla:

- **Personal is multi-user-safe locally once Phase 1's fixes land.** Each user
  has their own hylad, `~/.config/hyla` at 0700, a 0600 socket and a uid
  check.
- **Login server**, for reaching several accounts on one machine over the
  network: a root-owned `hyla gateway` (a systemd system service). It:
  - holds the network listener, in place of each user's hylad;
  - identifies the caller through Tailscale (Phase 2);
  - maps the caller to the OS accounts they may use, through an accounts
    file, such as `alice@example.com` to `alice`, and to `root` only when the
    file says so;
  - starts that account's hylad through PAM, so limits, `nologin` and session
    setup apply as at a console login (for example with
    `systemd-run --uid=<user> --user`);
  - relays the connection to that account's socket.

  It's the only piece that runs privileged, so keep it small and audit it.
  The client picks an account as `https://alice@host`.
- **Shared server** (several people's programs running as one `hyla` account)
  is the weakest model: everyone can read everyone's sessions. Skip it unless
  someone asks. A shared session for pairing is a different feature, with its
  own design.

## Phase 4: Hyla's own authentication

For networks without Tailscale, and for the iOS client.

- **Pair a device with a host once**, from the host's CLI or an
  already-trusted client. The device gets a key. hylad, or the gateway, keeps
  a list of trusted keys per account, which `hyla devices` lists and revokes.
- **Existing SSH keys as one more way to log in:** a client proves it holds a
  key listed in the account's `~/.ssh/authorized_keys`. That's authentication
  only, still over the HTTP transport. Rex plans the same.
- **TLS:** automatic through Tailscale. Without it, use a self-signed
  certificate pinned on first connect, or the user's own. Plain unencrypted
  HTTP is never offered for an authenticated listener.

## Security notes

- **A connection can do anything its account can,** including starting
  programs. Every listener is off by default, and every one identifies its
  callers.
- **Remote panes are untrusted, as local ones are.** OSC 52 clipboard reads
  keep their per-pane `ask` policy, and OSC 7501 text is still neutralized.
  - A remote host must not be able to pose as a local pane: show the host's
    name on its notifications and in the sidebar.
- **Remote Control** is per host and off by default for remote hosts (above).
- **Identify first:** listeners and the gateway identify the caller before
  they parse any `Hello` past its version.
- **Seeing who's connected:** `hyla clients` on a host lists who is connected,
  with each one's identity.

## Order and rough size

1. **Phase 1, headless Linux hylad:** small to medium. Mostly packaging and
   CI, plus the socket and uid fixes. Worth doing on its own.
2. **Phase 2, network transport and Tailscale:** large.
   - The multiplexed HTTP transport, and the three Tailscale listeners.
   - Hosts in the app, compression and reconnects.
   - The `fs.*` and `paste.file` RPCs, and the settings split.
3. **Phase 3, login gateway:** medium. The accounts file, PAM, and starting
   and relaying to each account's hylad.
4. **Phase 4, own authentication:** medium. Device keys first, then SSH keys.

## Open questions

- Does `hyla __hylad` from the Linux `.deb` start on a machine with no
  display, given that it links GTK? Try it on a fresh server VM. Even if it
  starts, needing GTK installed argues for the daemon-only build.
- **WebSockets or HTTP/2 streams** for the multiplexed transport? Check what
  passes Tailscale Serve, and what an iOS client can speak, before choosing.
- **Built-in tsnet or the machine's Tailscale?**
  - Built-in tsnet needs nothing installed on the server, but it adds size to
    hylad and a node per server.
  - The machine's Tailscale is lighter, but it's one more thing to have set
    up.
  - Rex offers both. Start with whichever the spike finds simpler, and add
    the other on demand.
- How does input latency feel over a slow link? hylad renders every frame, so
  a typed key needs a round trip before it shows. Measure before considering
  local echo prediction.
- Should a remote session show the host's own terminal theme, or always the
  app's? Today the app styles the terminal, so it would be the app's.
