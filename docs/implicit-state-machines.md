# Implicit state machines

A survey of Hyla for state that behaves like a state machine but is not
written as one. Each entry lists the fields that hold the state, the
states seen in practice, the transitions that nothing prevents, and what
breaks when the state is stale.

The survey found 25 of these. Several were checked against the code by
hand; each such finding says so. The others come from a read of the code
only.

The worst findings, in order:

| # | Machine | Failure |
|---|---|---|
| 1 | Feishu chat queue | A group thread and the chat it is in hold different keys. `/stop` cannot find the thread's waiting prompt or its card, so a blue card stays forever. |
| 2 | Settings file | `Load` reads `Path()`; `Save` writes `Target()`. A symlinked configuration (dotfiles) makes the app and hylad read the old file, so app edits revert and hylad never applies them. |
| 3 | Settings normalize | `apply` keeps the object it was given, but `Save` writes the normalized copy and drops the cache. `normalize` follows a save, so the app can hold a value that the file does not. |
| 4 | hylad lock | `successor` clears the lock and starts the new hylad with no lock owner for a moment. A client that stops hylad in that moment removes the new socket. |
| 5 | Ad blocking | `started` is set when compilation begins. If the first compile fails, no retry follows until the list setting changes. |
| 6 | Pane runtime | `setBackground` sets `closed = false` after closing the shell. A shell that exits in that window leaves a pane with no process and no Restart. |

## 1. Feishu chat queue (high)

**State:** `chat.busy`, `chat.waiting []waitingTurn`, `chat.held []string`
and `chat.running turnInfo` (`internal/feishu/feishu.go:195-237`).

**States of a prompt:** held (in `waiting` and `held`) → running
(`takeTurn` → `dropHold`) → done (`leave`). `/stop` moves a held prompt to
dropped.

**Writers:** `enter`/`leave` (`bridge.go:277-302`), `hold`/`holdCard`
(`:310-340`), `dropHold` (`:342-355`), `dropWaiting` (`:586-601`),
`beginTurn`/`dropTurn` (`resume.go:192-206`), `takeTurn`
(`bridge.go:126-160`).

**Key drift (checked).** The message path keys a group thread as
`chatKey(chatID, rootID)` (`bridge.go:103-105`). `dropWaiting` uses only
the chat ID (`bridge.go:561`, `:586`). `runningTurn` and the `running`
reads use `chatKey(chatID, "")` (`resume.go:170`, `:183`, `:212`). As a
result:

- `/stop` in a group thread does not drop that thread's waiting prompt.
  `waiting` stays full (`bridge.go:591-595` matches other structs). The
  held card stays in the chat. The next turn's `takeTurn` runs the prompt
  after all.
- `dropWaiting` does not find its held cards, so "Dropped the prompt that
  waited" is not shown.

**Other holes:**

- `/close` (`bridge.go:722`) and a session that is gone
  (`internal/feishu/drive.go:81-84`)
  do not drop `waiting`. The prompt runs later against a session that no
  longer exists.
- `dropHold`/`dropWaiting` ignore a delete error (`bridge.go:351`,
  `:601`). The card is already out of its list, so nothing deletes it
  later.
- `hold` releases the lock before it sends the card (`bridge.go:310-317`).
  A turn that ends in that moment leaves the prompt held and unwoken.
  Harmless while `queue == 1`, wrong if the depth rises.
- `/answer` starts a drive with `answering` and no `enter`/`leave`
  (`bridge.go:625-630`, checked). The chat is not busy for that turn. A
  queued prompt's `takeTurn` overwrites `running`, and the answer turn
  never calls `dropTurn`, so `running` stays stale until the timeout.
- `background` runs inside a `goTurn` goroutine that holds the chat
  (`bridge.go:148-159`), so a background turn does block the next message
  (checked; the earlier report was wrong here). Its result is the same:
  a long `command`-mode turn holds the chat for as long as it runs.

**Readers of a stale state:** `/stop` (`bridge.go:566`), the turn cards
(`:641`), `takeOver` (`resume.go:25-35`).

**Fix direction:** store `key` in `waitingTurn`; pass it to `dropWaiting`;
make `runningTurn` take a key; route `/answer` through `enter`/`leave`.

## 2. Settings: the file, the app's copy, hylad's copy (high)

**State:** the configuration file, `a.set`/`a.applied` (`app.go:1243-1258`),
the global cache (`internal/settings/settings.go:349-352`), and each
`settings.Load()` caller (54 in the tree).

**Path drift (checked).** `Load` stats `Path()`
(`settings.go:362-363`). `write` writes `Target()` (`settings.go:434`,
`:457-465`). With the configuration a symlink into dotfiles, the app
saves through the link and then watches the link's own path. The next
`apply` loads the stale file. The app reverts and hylad never sees the
change.

**Normalize drift (checked).** `apply` sets `a.set` and `a.applied` to
the given object (`app.go:1254`) and then calls `settings.Save`
(`:1258`). `Save` normalizes a copy and caches that copy
(`settings.go:419-433`). The app keeps the unnormalized object. While
the file mtime is unchanged, `Load` returns the cache, so hylad sees the
normalized values and the app does not. Key-by-key write-back
(`settingRow`, `settings_view.go:433`) makes this order-sensitive:
`normalize` can copy one setting from another.

**Writers:** the Settings sheet, `hosts_*.go` (`a.set = s` at
`hosts_settings.go:103`, `:211`, `:284`, `:351`; `hosts_tailcat.go:56`,
`:173`, `:366`; `hosts_color.go:72`). The file watcher
(`app.go:1437`, `a.apply(cur, false)`).

**Fix direction:** make `Load` follow the symlink. Adopt the normalized
copy in `apply` (Load after Save, or Return the saved value from Save).

## 3. Remote updates toggle (medium)

`toggle(c.Key("remote-updates"), &a.set.RemoteUpdates, ...)`
(`hosts_settings.go:110`) writes the app's settings in place during a
render. The save comes from the render check
(`settings_view.go:308-311`, `!reflect.DeepEqual(a.set, a.applied)`) or
from the watcher. It works, but any copy race loses the click with no
save. `a.set` is shared by every render and the watcher (`app.go:1437`).

## 4. Ad blocking (fixed)

**State:** `adBlock{started, lists, gen, rules}` (`adblock.go:23-30`).

`compileAds` sets `started` and `lists` before it starts the goroutine
(`adblock.go:48`). An error is only logged (`:52-54`). `gen` still
matches, so `rules = nil` applies, and `started` stays true. The early
return (`:45`) then blocks every retry until the lists change. Most
likely on the first run, when no lists are cached and a download fails.
The same happens when every `CompileContentRules` fails
(`adblock.go:104-113`).

Turning blocking off leaves `a.ads.rules` in place (`adblock.go:35-39`).
That is safe, because `adRules` returns nil, but the object is a second
copy of the setting.

**Fixed** in 174fe8b: a compile that ends with an error clears `started`
and `lists`, so the next page tries the lists again. The rules it did
compile stay. `TestAdsCompileAgainAfterAFailure` holds it.

## 5. hylad lock and socket (high, narrow window)

**State:** `hylad.lock` (a pid), the socket file, and the
`Hello`/`Welcome` version pair.

`successor` calls `ClearLock()` and then starts the new hylad
(`internal/daemon/daemon.go:278-290`). The new process writes its pid only
at `:165-166`. In that window the lock is empty and the old socket is
still there. A client that calls `Stop` on the old pid runs
`ClearDaemon()` (`internal/client/client.go:325`), which removes the new
hylad's socket. The new hylad does not listen again, and the empty lock
rejects another start.

`LockedPID` trusts a pids in use (`client.go:376-379`), so a reused pid
can receive SIGINT. `DaemonPID` ignores the version error
(`client.go:234-235`).

**Fix direction:** write the new pid into the lock over the old one
(`ClearLock(pid uint32)`), so that the lock never names no one.

## 6. Pane runtime: `proc`, `closed`, `bg`, `awaiting` (medium)

**Fields:** `internal/daemon/session.go:44`, `:76`, `:77`, `:84`;
`proto.Pane.CommandExit` (`proto.go:219`); `info.Status`.

**States:** alive (`proc != nil`) → exited and kept (Status `Exited`) →
restarted. Background panes have `bg != nil`.

- `setBackground` calls `p.close()` and then sets `closed = false`
  (`background.go:399-405`). A shell that exits between the two calls
  sets nothing: `exited` sees `closed` (`session.go:744`) and returns.
  The pane stays Idle with `proc == nil` and no Restart. Checked. Worse,
  `close` calls `noteEnded`, which takes `d.mu` and saves beside the
  shell's reader: `setBackground` is usually left with `proc` set and
  the status Exited, after its own work, which then lands in no test and
  in some runs. My first attempt to fix it (wait for the shell behind
  the pane's lock, then reopen) was reverted: it changed the lock order
  and left more holes than it closed. The fix needs one owner of the
  sequence: `close` returning once its reader left, or a `closed` that
  `exited` honours by setting the pane's final state.
- `restart` on a background pane is not prevented: `proc == nil` and
  `term != nil` pass its guard (`session.go:769-783`). `start` then opens
  a shell beside `bg`, so two owners write the status.
- `restart` sets the status after `start` (`session.go:170` against
  `:772-775`). In between, status says Exited with `proc != nil`.
- `awaiting` is cleared only by a 133;D from the shell (`osc.go:240`). A
  command killed without 133;D stays `awaiting` until a restart.

**Fix direction:** make the status a typed enum with one writer and a
switch for the legal moves. Set `closed` before `close` returns, not
after.

## 7. App host connection: `conn`, `cl`, `rc`, `loaded`, `dialing`, `gen`, `busy` (medium)

**Fields:** `hosts.go:30-60`; `connState` (`app.go:29-42`).

- `dialing` is cleared only by the winning attempt (`hosts.go:416`). A
  superseded attempt returns at `:407-414` before it clears the flag.
  `closeHost` bumps `gen` without clearing it (`:309`), so the host can
  never reconnect.
- `loaded` is not cleared by `disconnected` (`hosts.go:525-537`).
  `hostsLoaded` and `busyCount` count the host's old sessions.
- `busy` is cleared only by the request's callback (`hosts_view.go:552`).
  It is not cleared by `closeHost`, so the row stays disabled.
- `restartDaemon` sets `conn = connecting` but leaves `cl` (`app.go:2111`).
  Most `connReady` checks also test `h.remote()` (`drop_files.go:31`,
  `app.go:470`, `:507`, `:920`, `picker.go:281`), so the local race is
  narrower than first reported. `connectHost` bumps `gen` and `dialing`
  itself (`hosts.go:334-340`), so the second connect wins cleanly.
  Checked.

## 8. Overlay and dialog exclusivity (medium)

**State:** `pal`, `find`, `confirm`, `wtAsk`, `sshAsk`, `k8sAsk`,
`picker` (`app.go:163-167`, `:223`), `ss.confirm`
(`settings_view.go:305`).

- The shortcut guard checks `a.confirm != nil` (`palette.go:489`). The
  drop guard checks `!a.confirm.open` (`view.go:128`). Openers always set
  `open: true`, so the second condition never fires: the sidebar drop
  handler is dead while a confirmation is pending.
- The picker opens from the palette with the palette closed
  (`palette.go:412`, `picker.go:46`). A pending confirmation reappears
  over the picker.
- Two goroutines can set `a.confirm` (`worktree.go:115-125`), so one
  confirmation replaces another.
- The main window and the Settings window each have their own slot.

**Fix direction:** one `screen` field with these values: none, palette,
picker, confirm, sheet. A `switch` for the key routes, and one close
function.

## 9. Turn fingerprint (medium)

`proto.Pane.Turn` is written only by `git.snapshot`
(`internal/daemon/git.go:265-268`) and restore (`store.go:215`). It is
never cleared on agent exit (`session.go:344`, `:750`). After the agent
changes, the diff shows the previous conversation's changes from the old
worktree. The snapshot objects can be pruned by git gc.

## 10. Worktree create and remove (medium)

Git holds the state on disk; Hyla keeps no journal (`internal/git/worktree.go`).
A crash after `worktree add` leaves an admin entry that blocks a retry
until `git worktree prune` (`:41-53`). A crash after remove, before the
branch delete, leaves an orphan branch (`:118-126`). `Exclude` is a
read-modify-write with no lock (`:57-72`).

## 11. Pets install (medium)

`os.RemoveAll(dst)` and then `os.Rename(tmp, dst)` (`pets.go:413-416`).
A failure between them deletes the user's pet, and the staged directory
stays and is never cleaned.

## 12. Feishu secret and app ID (medium)

The secret (`feishu.json`) and the app ID (in the configuration) are
saved one after the other (`internal/cli/feishu.go:113`, `:130`;
`settings_feishu.go:406`, `:422`). A failure between them leaves a mismatched
pair. Turning Feishu off leaves the secret and `feishu-env.json`.
`SetAppEnv` writes with a bare `os.WriteFile` (`settings/feishu.go:96`),
so a reader can see a partial file.

## 13. Client attachment (medium)

`pane.close` sends `FrameClosed` before `finish` (`session.go:803-806`).
When the queue is full, `send` drops the frame (`:954-960`). The
client's `Stream.Gone` then never closes, and the terminal shows frozen
output. `detach` before `attach` leaves the mirror open
(`session.go:865-884`). `termEntry` (`app.go:96-101`) has attaching,
attached and failed states, but no gone state.

## 14. Agent session ownership (medium)

`setBackground` (`internal/daemon/rpc.go:111-125`, `background.go:380`) does not clear a
foreground `AgentSession`. `state.json` saves it (`store.go:95`), and the
next start restores it as a pane with an agent and then runs the chat's
command over it (`background.go:122-128`).

## 15. Wait / turn completion (medium)

`d.wait` (`rpc.go:895-933`) treats "not Working" as done. A pane that
starts and fails before the detector sees Working returns at once, so
`hyla prompt` reports failure as a completed turn.

## 16. Quit (low–medium)

`quitting` is cleared only at the next quit request (`app.go:1964-1968`),
so a cancelled quit leaves it true. `quitTimer` calls `os.Exit(0)` after
2 s even if `shutdown` is still going (`app.go:2005`, `:2021`).

## 17. Uploads and find bar (low)

`uploading` (`app.go:217`) drops the first entry when a second upload
starts on the same pane (`drop_upload.go:58-88`). `find.pane`
(`find.go:15`) is cleared only by `dropTerm`, so it can name a gone pane.

## 18. Chat feed and pane chat (low)

`chatFeed.busy`/`got` (`chat.go:58-67`) drops a read that arrives during
another (`:98`), so the change that triggered it is lost. `paneChat`
(`app.go:134`) is never deleted by `closePane` (`panes.go:936-990`).

## 19. Derived git states (low)

`Status.State` decides by the order of about five marker files
(`internal/git/git.go:186-198`). The loose `refs/remotes/origin/HEAD`
wins over `packed-refs` (`:221-233`). `Stamp` keys on mtime and size
(`:247-265`), so a same-tick same-size write is unseen.

## 20. Update and build fingerprints (low)

`hyladStale` accepts an empty fingerprint (`hylad_build.go:171-176`).
`fontsLoaded` stops trying font directories after the first one succeeds
(`fonts.go:25`). `shell.Login` caches for the process lifetime
(`internal/shell/shell.go:9-28`). `detect.Load` fails every agent
silently on one bad manifest (`internal/detect/detect.go:112-141`).

## 21. Named but unfinished states (low)

`d.ending` (`daemon.go:80`) is written once (`rpc.go:811`) and read
nowhere. `Pane.Restored` (`proto.go:228`) is a one-shot event kept as a
flag, with no reader. `fileOpeners` (`side.go:58`) has no writer or
reader. The accelerator key `backspace` maps to the literal string
`"Backspace"` (`config.go:588`), and `actions.go:239` agrees with it, so
the typo now works.

## 22. Remote mux handshake (medium)

**State:** `MuxHello{Mux, Version, Compress}` and
`MuxWelcome{Mux, Compress}` (`internal/remote/mux.go:153`, `:366-376`).

- Two versions travel in one handshake. The mux compares `Mux` only
  (`mux.go:174`, `:370`). `Version` is checked per stream in hylad
  (`internal/daemon/daemon.go:784`). A mux can connect and then fail every
  stream with a version error.
- Compression is a convention: the client always offers `deflate`, and
  the server takes it if offered (`mux.go:373-375`). No value means off,
  so a change to the client's list changes the framing with no
  negotiation.
- `Done()` is `sess.CloseChan()` (`mux.go:250`). The server ends only
  when `AcceptStream` fails, so a dead link is seen only by yamux
  keepalive (5 s ping, 20 s timeout) and reconnects can lag by up to 20 s.
- `shells` (`transport.go:74`) holds a shell per address, never expires,
  and is not keyed by host identity.

## 23. Tailscale access (medium)

**State:** `tailscale.Self{IPs, Owner, Tags}` from `Status`
(`internal/tailscale/tailscale.go:194-215`), the listener's tailnet
sockets (`internal/daemon/listen.go:229-259`), `l.allow`, `selfStatus`,
and `whoCache`.

- NeedsLogin, Stopped and Expired all become one `ErrNotRunning`
  (`tailscale.go:197-199`), so nothing tells the user which one it is.
- `tailnet()` returns after the first successful listen. It never sees
  Tailscale stop or the tailnet address change.
- A tagged machine has `Owner == ""` (`tailscale.go:203-205`). Then only
  `allow` admits anyone, and nothing reports that as a state.
- `whoCache` keeps a login for 30 s (`listen.go:349-362`). `l.set`
  replaces `l.allow` (`listen.go:137`) but does not clear the cache, so a
  revoked user keeps access for up to 30 s.
- `TailcatRotate` is a bool used as an event (`internal/cli/remote.go:442`,
  seen at `daemon/listen.go:95`). It is lost if rotated while hylad is
  down, and rotating twice returns to the old value, so no rotation
  follows.

## 24. Update install (medium)

**On-disk state:** `dir`, `dir.new`, `dir.old` on POSIX;
`<file>.hyla-old` on Windows (`internal/update/update.go:283-360`).

POSIX renames `dir` to `old` and then `staging` to `dir` (`:300-307`).
Nothing exists at the install path between the two renames, and no lock
stops a second updater. Windows moves each old file aside and then moves
each new file in (`:321-360`). A crash in the middle leaves a mixture of
old and new files.

## 25. CLI link (low)

`clilink.Link` checks the symlink target and returns if it matches
(`link_unix.go:17-21`). Otherwise it removes the link and then creates it
(`:22-23`). A `bin/hyla` run between the two calls sees ENOENT.

## Where an explicit machine already exists

| Machine | Where |
|---|---|
| `connState` (four values) | `app.go:29-42` |
| `proto.Status` | `proto.go:108-123`, one writer `session.go:288` |
| OSC 7501 records, priority order | `internal/daemon/osc.go:262`, `:325-392` |
| `detect.State` screen classifier | `internal/daemon/session.go:402`, `internal/detect/detect.go:26-31` |
| `background.ended/err/code` | `internal/daemon/background.go:23-37` |
| Daemon `restarting`/`ending` | `internal/daemon/daemon.go:80-83` |
| Feishu turn settle rules | `internal/feishu/drive.go:198-215` |
| Feishu card state to colour | `internal/feishu/card.go:444` |
| Feishu run mode whitelist | `internal/settings/feishu.go:540` |
| Settings choice whitelists and `normalize` | `internal/settings/config.go:93-137`, `settings.go:580-680` |
| `pets.State` frame table | `internal/pets/pets.go:47-101` |
| `git.Base` switch | `internal/git/diff.go:58-82` |
| Ad-block rule order | `internal/adblock/convert.go:56-79` |
| `dropZone` | `panes.go:459-490` |

`proto.Status` is the obvious one to promote: a typed value with a
transition table would fix findings 6 and 15, and would let findings 9
and 14 hook off the agent start and end.
