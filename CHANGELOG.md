# Changelog

This is smallnest's fork of Kero, which Hyla was called. A fork release
takes a number above upstream's, so it never takes an upstream tag's
number; the notes of the upstream releases it includes follow its own.

## Unreleased

- A prompt sent from Feishu while the agent works waits its turn in a blue
  card titled 加入请求队列，请等待 (Added to the queue), which goes when its
  turn begins, or when /stop ends it.

## 3.5.5

- Closing a session from the button on its row asks first while a program
  other than the shell runs in it, as the menu does: one wrong click no
  longer ends a working agent. Closing all of a project's sessions says
  how many end.
- Removing a host, an SSH host or a K8s pod offers Undo, which puts it
  back where it was.
- A host that runs another version of Hyla says which side is older:
  only one of them is updated from here, and the other says what
  to do. Waiting for Hyla on a host that restarted says so once
  it did.
- When Hyla's background service does not stop, Hyla says why instead of
  waiting forever; a host still being reached has a row of its own in the
  sidebar.
- The quick terminal says when it can't reach the service, with Try
  Again, instead of spinning.
- The sidebar's hierarchy reads top-down: a project's name outweighs its
  sessions, a long session name keeps its room, and its details stand on
  one line. Dark and light themes lay rows out alike. Picking a session
  from **Go to Session** (⌘P), or the one that needs you (⌘J), unfolds
  its project.
- The first screen offers **Connect a Machine…**, and the sort menu's
  headings say what they group and order by.
- Japanese and Chinese no longer leave English sentences in the window.

## 3.5.4

- Hyla starts hylad when it finds none answering, as after **hyla daemon
  stop**, a crash, or a login that opens Hyla before its daemon: it shows
  the sessions instead of a host that needs starting, and keeps trying
  while a slow start takes.
- A hylad whose socket went but still holds its lock answers nothing, and
  Hyla starts none beside it: it stops that hylad, which saves its
  sessions for the one that starts in its place. A process that holds the
  lock and is not hylad is left alone, and Hyla says which pid it is.

## 3.5.3

- Files and folders dragged from this machine onto the pane of another
  machine go to the directory the pane's program runs in, as scp puts
  them: where the shell is, or where an agent in it went. Their paths
  there are pasted into the program, which reads them as a drop there's.
  A file of a name that is there already goes beside it, as a Finder's
  save does. Hyla on the host must be updated for it.
- Files dropped on a pane of this machine paste their paths into the
  program in it, as a drop on a terminal window does: the file stays
  where it is.
- A session, an SSH host's row or a K8s pod's row shows a close button
  under the pointer, in place of its status: it ends the terminal at
  once.
- A project that holds a session waiting on you wears the color of
  attention in the sidebar, even folded away.
- A host's name reads as a section's title: "Dev Box" for "dev box".
- A terminal can show its own background picture, apart from the window's:
  **设置 › 外观 › 终端背景图**, or `pane-image` and `pane-image-opacity`.
  The picture shows under the text, in place of the window's background,
  as wide as the card from its top; **铺放方式**, or `pane-image-fit`, fills
  a card as a macOS desktop picture's options do: fill, fit, stretch,
  center or tile.
- Switching sessions animates: the session coming in fades in and lifts a
  little, springing, while the one it leaves blurs and fades away over it.
  **设置 › 外观 › 切换会话动画**, or `session-switch`, chooses the
  animation, as a slide transition's: dissolve (the default), slide from
  the right, push, cover, zoom, or none.

## 3.5.2

- **SSH 主机** and **K8s Pods** rows offer **重连主机** in their menu: it
  opens a new terminal in place of the one the row had, whether it still
  runs or has ended, as when ssh or kubectl exits with the connection.

## 3.5.1

- Hyla's Feishu bot keeps a chat's coding-agent conversation across a
  restart, in `feishu-chats.json`: every message of the chat goes on with
  the conversation before it, instead of starting a new agent session
  under the chat's session and losing the context.

## 3.5.0

- **Settings › Appearance › Sidebar background** puts a picture of this
  machine behind the sidebar's sessions: as tall as 80% of the sidebar,
  vertically centred and against its left edge, and cropped by the
  sidebar where it is wider.
- **Picture opacity** dims the picture, 35% by default and 0 for none,
  so that the sessions stay easy to read.

## 3.4.0

- **Settings › Appearance › Window material** chooses what the window's
  translucent background is made of: the sidebar's, a window's, a
  popover's, **Plastic** (a frosted content background), Liquid Glass as
  the Dock's, or clear glass.
- **Settings › Appearance › Background** adds **Dock**: Liquid Glass
  under a thin veil, as the Dock's own background.
- **Settings › Terminal › Terminal blur** blurs what shows through
  translucent terminals, and **Terminal material** chooses Liquid Glass,
  **Plastic** or clear glass for it.
- The **Dusk** background is gone, too close to **Petal**; a
  configuration that names `dusk` gets Petal.
- Quitting Hyla always quits, quickly.

## 3.3.2

- **Settings › Terminal › Weight** offers **Thin** and **Extra Light**
  beside the other weights, for terminals drawn in JetBrains Mono Nerd
  Font.

## 3.3.1

- **Add K8s Pod** at the foot of the sidebar opens a terminal running a
  `kubectl exec` command, such as
  `kubectl exec -it web-0 -n prod -- /bin/bash`: kubectl runs on this
  computer, with your kubeconfig, and the pod needs no Hyla. The pods
  live in one **K8s Pods** project, each a row with its name over the
  command; a click shows its terminal or starts one, **+** adds another,
  and a right-click removes one.

## 3.3.0

- **DSH** (DeepSeek Harness) is a coding agent Hyla knows: the sidebar
  shows it working, waiting for an approval or idle, it resumes after a
  restart, and a Feishu chat can run it with `/new <project> dsh`. Its
  chats start the session DSH makes and continue it.
- **Settings › Terminal › Fallback font** adds the fonts Hyla's terminals
  draw what their font lacks with, such as Chinese, in the order they are
  tried; the configuration file sets them with `font-fallback`.
- **Settings › Terminal › Cell width** widens or narrows the terminals'
  cells from 0.8 to 2 times the font's, without changing the size of the
  text; the configuration file sets it with `cell-width`.
- The quick terminal is <kbd>Control</kbd> <kbd>`</kbd> on every platform,
  as Ghostty's documentation gives it, so it no longer takes
  <kbd>Control</kbd> <kbd>Shift</kbd> <kbd>T</kbd>, which terminals paste
  with. A keybind of your own keeps it.
- Removing a Hyla host in **Settings › Hyla Hosts** works again.

## 3.2.0

- **Add SSH Host** at the foot of the sidebar opens a terminal running ssh
  to a machine: an address as ssh takes it, such as
  `root@example.com:2222`, and a name. The machines live in one **SSH
  Hosts** project, each a row with its name over the address; a click
  shows its terminal or starts one, **+** adds another, and a right-click
  removes one. Nothing runs on the machine but an SSH server: no Hyla is
  needed there, and ssh uses your `~/.ssh/config`, keys and agent.
- **Settings › Hosts** is **Settings › Hyla Hosts** now, in all three
  languages, so that it does not read as the sidebar's SSH hosts.
- A host added in **Settings › Hyla Hosts** shows a row of its own in the
  sidebar while it has no project there, its name over the address it is
  reached at; a click starts a session on it, rather than the host
  staying hidden until one of its projects comes.
- **Settings › Terminal › Font features** sets the font's OpenType
  features, as Ghostty's `font-feature`: `ss01` or `+ss01` turns one on,
  `-calt` off, `cv05=2` picks an alternate; the preview shows them too.

## 3.1.3

- The sidebar's detailed rows show a session's second line again: its
  status message, else its branch and changes, else its folder. **Settings
  › General › Sidebar rows** keeps the choice, detailed or compact.
- The quick terminal drops onto Hyla's panes instead of across the whole
  screen: as wide as them and half the window tall, at their top left.
  With Hyla's window hidden or minimized, it still drops from the top of
  the screen.
- It has a shortcut by default now: **Control+`** (⌃` on a Mac), as
  Ghostty's quick terminal does, and it works in every app. **Settings ›
  Keyboard** gives it another, or none.

## 3.1.2

- A Feishu chat whose agent is Pi or Gemini shows what the turn used in
  its card's footer too. Their turns run with the JSON their CLIs print
  (Pi's `--mode json`, Gemini's `--output-format json`), which names the
  answer, the model, the tokens and the context, as OpenCode's and
  Codex's do.

## 3.1.1

- A Feishu chat whose agent is OpenCode shows what the turn used in its
  card's footer again. The turn now runs with `--format json`, whose
  steps name the tokens it took, gave and cached, and the context they
  make. Codex's turns do too, from their `turn.completed` usage. An agent
  that does not say how much its model holds shows the context without a
  size, as `ctx 61.8k`.

## 3.1.0

- Hyla's icon is orange now, and its frog is a red-eyed tree frog: red eyes
  with a black slit. The pet on the window's edge wears the same colours
  and eyes. Development builds keep the green icon, so they still stand
  apart from the installed Hyla.
- **The default port of hylad's network listeners moved from 7373 to
  7473**, so that Hyla and Kero can run on one machine side by side. A
  host that listens on 7373 keeps working when its `listen` address names
  the port (`tailnet://:7373`); a host added as plain `http://name` now
  asks for 7473.
- Settings › Appearance calls the pet "Hyla, the tree frog" (小树蛙 Hyla,
  アマガエルの Hyla).

## 3.0.0

Hyla was called Kero up to 2.1.37, and this release renames it: the app
is **Hyla.app**, the command `hyla`, the daemon `hylad`, the server build
`hyla-server`, and settings and sessions live in `~/.config/hyla`
(`~/Library/Application Support/hyla` on macOS, `%AppData%\hyla` on
Windows). The website stays at kero.sh for now.

- **Your settings and sessions carry over.** The first Hyla to start
  moves the directory of 2.x to the new name, and `KERO_*` environment
  variables are read as `HYLA_*`, so hooks and scripts written for Kero
  keep working. The `kero` command of 2.x, its skill, its plugins and its
  service are replaced by Hyla's.
- **2.x does not update itself to this release.** Install Hyla once from
  the releases page; later releases update as before. A server build of
  2.x on another machine: run install-server.sh again, which removes the
  old install and keeps its settings and sessions.
- The update key is the same, so a release signed for Kero verifies as
  Hyla's.

## 2.1.37

- OpenCode works as the agent a Feishu chat runs. A prompt in the
  background found no program, as hylad did not look where OpenCode
  installs, and a turn now continues a session of OpenCode's own, as
  OpenCode asks. On another machine, this works once that host gets
  this update.
- A command of a chat gets its input only when it asks for it, as
  Claude Code's permission tool does, so a turn no longer waits for an
  input no one ends. **Settings › Feishu › How a prompt runs** keeps the
  choice it shows.
- Codex works as the agent a Feishu chat runs. Its turns run with their
  events, which name the thread the chat continues and hold the agent's
  messages alone, without the banner and the tokens line. A chat whose
  thread Codex no longer knows starts anew rather than failing every
  turn.
- A chat's window shows the turns Hyla ran for it, also when the agent
  is OpenCode, Codex or Pi, whose own transcripts Hyla does not read: it
  showed none of Codex's and Pi's before.

## 2.1.36

- A Feishu card whose answer asks the chat to choose from a list of two or
  more options shows the question and the options as buttons, with a field
  for an answer of its own. A click answers the agent with the option's
  text, as /answer does. A question takes one answer.
- A message sent to a Feishu chat while a turn there still runs waits for
  it, one deep, and runs as that turn ends: it is no longer refused with
  "Hyla is still working on the last message there". A command, as /status
  or /stop, runs at once, and /stop drops the message that waited.
- A pane that the agent draws as a screen of its own, as OpenCode's, is
  answered with the turn's text: the bot waits for the turn to end, and
  replies with the answer under the prompt, without the rows of the
  agent's thinking and the terminal's.
- The bot logs each message as it arrives, so a chat that gets no answer
  tells where it stopped.
- A pane's chat shows only where it was opened (a session's menu › Open
  Chat), not in every pane while Feishu is on.
- A login shell whose profile starts a program that outlives it, as a
  prompt's, no longer holds hylad at its start: the bot and the sessions
  come back.
- The gap between the panes and the side panel is half what it was.

## 2.1.35

- Feishu has its own page in Settings, with a YOLO mode (feishu-yolo). In
  YOLO mode, which is on by default, the agents from Feishu run every tool
  without asking. Settings › Feishu shows the app, the connection, who may
  use it, the run mode, the agent and the YOLO modes of each agent.
- In the command mode, Claude Code asks the chat before it runs a tool
  beyond its permissions: the card names the tool, and its buttons allow
  it once, allow it for the session, or refuse it. With YOLO on, it asks
  nothing.
- A turn shows on the user's message: the message takes an "OnIt"
  reaction as the turn starts, and gives it back as the answer comes.
- The answer's card has a footer under a rule: the state in a colored
  tag, the model, the tokens the turn took and gave, the cached tokens,
  the context used, and the time. The card shows only the values that it
  knows.
- Feishu no longer shows Claude Code's warning that it received no input,
  and no longer fails a tool with "AbortError: Stream closed".

## 2.1.34

- The Files tab shows each file and folder with the icon of its kind.
- The Files tab makes, renames and deletes files. A row's menu (right
  click) makes a file or folder in its folder, or renames or deletes the
  row; the buttons above the tree make a file or folder at its top, and
  fold the folders open. A local file deleted goes to the Trash; a file
  on a remote host is deleted for good, as hylad has no trash there.
- A file opened as an image shows as an image, and its header opens it
  in a file pane or in a program. A file's text follows the file as it
  changes, and a click on it again shows the viewer once its button
  hides it.

## 2.1.33

- The side panel has an Info tab (⌥⌘I): the focused pane's working
  directory, with buttons to copy it, show it in Finder, or open it in a
  file pane or an editor; the processes of its terminal, with their ports,
  which a click opens in a browser; and, when Magpie runs on this machine,
  its usage: today's calls, tokens and cost, the cache hit rate, the plan
  windows closest to their limits, and the latest call.
- Sessions no longer come back with many "restored" lines. A restore
  marks one, once. And two hylads no longer stop each other: the app stops
  a hylad of an older Hyla as it connects to it. hylad's log says who asked
  it to stop.

## Upstream releases

Upstream's notes for 2.1.22 to 2.1.24, which this fork has.

## 2.1.24

- On macOS, dragging Hyla's window by its top no longer makes it
  shake.

## 2.1.23

- On macOS, Hyla keeps its place in the Dock after it updates itself.

## 2.1.22

- With no session open, Split Right (⌘D) starts one. Thanks @iamvicliu
- Images a program like Pi showed no longer stay over the screen once
  it clears it, as with Pi's `/new`. Thanks @jorben for the report
- After Hyla relaunches to use a newer hylad, quitting it quits it
  instead of leaving it stuck open.

## 2.1.21

- In a narrow sidebar, a session's time, and its ⌘ number while you
  hold ⌘, show whole beside a long name, which ends in an ellipsis
  instead. Thanks @jamdaniels for the report

## 2.1.20

- A quick terminal drops down from the top of the screen over any app
  and goes back up when you press its shortcut again. Give it a
  shortcut in Settings › Keyboard (**Quick Terminal**); **View › Quick
  Terminal** and the menu bar icon open it too. Its session keeps
  running while it's hidden.
- On Windows, an agent finishing in the session you have selected
  shows its notification while another app is in front. Thanks @Maasea
  for the report
- Return presses a confirmation's action, such as Close Pane, as its
  ↩ says. Thanks @iamvicliu
- On a Mac, after Hyla updates itself, quitting an app you started in
  a pane no longer ends every pane's shell.

## 2.1.19

- **Session › New Session in Worktree…** (⌘⌥T) starts a session on a
  new branch in a git worktree of its own, so agents working side by
  side don't trip over each other's changes. Closing the session offers
  to remove the worktree. `hyla new --worktree <branch>` does the same
  from the command line.
- Hyla remembers the conversations of Claude Code, Codex, Pi and
  OpenCode that ran in its panes, so you can resume one after its
  session is closed: ⌘P lists recent ones, and **Session › Resume
  Conversation…** (⌘⇧O) lists them all. Turn on Settings › General ›
  **Session history** to list them at the bottom of the sidebar too,
  under **Ended** and **Archived**. `hyla history` and `hyla resume` do
  the same from the command line. Thanks @ak4duy
- Drop folders onto the sidebar to open them as projects. Thanks
  @p1xelll
- Dragging the sidebar's empty space moves the window, as the top bar
  does. Thanks @jamdaniels for the request


## 2.1.31 (smallnest/hyla)

- A pet stays on the window's edge while Hyla is in front: it no longer
  goes behind the window as the app comes forward, which hid it where it
  stood at the window's bottom edge. It goes as another app comes
  forward, and comes back with Hyla.
- This release has everything 2.1.30 has, whose notes follow.

## 2.1.30 (smallnest/hyla)

- A Codex pet no longer loses its lower part under the window: it shows
  whole on the window's edge, and stays on the edge as the window is
  dragged near the top of the screen, where the screen keeps its window
  below the menu bar. Where the screen leaves no room for it above, it
  stands at the window's bottom edge.
- A session that waits shows as the pet's waiting row, and a pat shows
  the pet's wave.
- This release has everything 2.1.29 has, whose notes follow.

## 2.1.29 (smallnest/hyla)

- The Tailcat settings in **Settings › Hosts** say what they do: **Use
  Tailcat to Reach Other Hosts**, **Remote Tailcat Port**, **Provide
  Tailcat Service** and **This Computer's Tailcat Listener Port**. The
  pair code has a **This Computer's Pair Code** label above it.
- This release has everything 2.1.28 has, whose notes follow.

## 2.1.28 (smallnest/hyla)

- Reaching hosts over **Tailcat** again: this release has the feature. A
  host that listened on Tailcat stopped when its configuration file
  wrote **Listen** before the Tailcat setting; hosts get the fix with
  their next update.
- `hyla daemon status` no longer crashes when hylad runs on this
  machine alone.

## Unreleased (smallnest/hyla)

- While the window is translucent, the sidebar's rows and the cards let
  the desktop through as much as the background does, so that it shows
  through the window rather than only at its edges.

## 2.1.21 (smallnest/hyla)

- A **quick terminal** drops down from the top of the screen over any
  app, as Ghostty's does. Give it a shortcut in **Settings › Keyboard**,
  which works in every app, or bind it in the configuration file. It
  keeps a session of its own while it is hidden.
- **Background blur** off now leaves the background translucent, as the
  setting says: it used to make the window opaque.
- Return presses the action of a confirmation, such as **Close Pane**,
  though Cancel comes first. Thanks @iamvicliu.
- On Windows and Linux, Hyla tells when an agent finishes in the
  session you watch only while Hyla is the app in front, as on macOS.

## 2.1.20 (smallnest/hyla)

- This release has everything upstream's 2.1.19 has, whose notes
  follow. The **Ended** section of the sidebar now lists the
  conversations that this 2.1.19 records, so it works for every
  conversation, on every host, and a conversation leaves the list when
  it runs again.
- **Quitting Hyla** can end this computer's sessions and their programs,
  and stop hylad, as **Quit and End All Sessions** does: set
  `quit-ends-sessions = true`. It asks first while a program runs.
- The window takes a **window-opacity** and a **window-blur** setting:
  how much the background wash covers, 0 to 1, and whether the desktop
  behind it is blurred.
- **Hyla the frog** stays in the window, and follows your sessions.
  `pet = false` hides it, and `pet-offset` sets where it stands.
- The sidebar groups sessions by project, with an icon for each, and
  sorts them by date or by name, or in the order you drag them to.
- The side panel has a **Files** view and a **Changes** view. ⇧⌘E shows
  or hides the panel, and ⌥⌘F goes to Files.
- The question that **Quit and End All Sessions** asks takes a click or
  Escape, and **Cancel** closes it.
- Chinese text keeps its monospaced font on Windows. Upstream took this
  fix out of 2.1.19; this fork keeps it.
- The window has more strings in Japanese.
- Updates come from this fork's releases again, and they take their key
  from this fork.

## 2.1.19 (smallnest/hyla)

- **Session history**, off until you turn it on in **Settings › General**,
  or with `session-history = true`: when Claude Code, Codex, Pi or OpenCode
  ends, Hyla keeps its conversation, in an **Ended** section at the bottom
  of the sidebar. A click on one starts a session in its folder that
  resumes the conversation; **Archived**, below it, holds the ones you put
  aside. Hyla keeps the last 50 of each, from every host, and the
  section's menus resume, archive, restore, copy or delete them. Ended
  sessions of this host come with the next update of hylad. Thanks
  @ak4duy
- Hyla the frog has the colors of the icon's frog.
- The question that **Quit and End All Sessions** asks no longer takes
  Return, which a program may be reading: it takes a click or Escape, and
  **Cancel** closes it.
- The window has settings in Japanese.

## 2.1.18

- A row above the tree of the **Files** view picks what a click on a
  file does: show it under the tree, open it in a file pane with Hyla's
  editor and its syntax highlighting, or open it in the host's editor.
  Hyla keeps the choice.
- This release also has what upstream's 2.1.18 has, whose notes
  follow, credited to those who reported it.
- This release takes in upstream's 2.1.15 to 2.1.17, whose notes follow:
  Hyla's icon in the menu bar and the menu it opens, `hyla events`, the
  browser's snapshots and saved cookies, programs that can ask for the
  microphone and the camera, and Japanese.

## 2.1.14 (smallnest/hyla)

- The side panel has a **Files** view beside **Changes**: a tree of the
  session's folder, each file opening under it as its text, with syntax
  highlighting, or as its image. It lists folders on the session's own
  computer, a remote host's as easily as this one's, and hides `.git`.
- **Open in a File Pane** shows a file of the Files view in a pane of its
  own, in the app's editor, colored by its language. The pane comes back
  after a restart, as a browser pane does. **Open in the Host's Editor**
  still runs `$EDITOR` for the files an editor reads better than a view.
- The panel's tabs switch on a click again: a click started a drag of the
  window.
- **View › Toggle Side Panel** (⇧⌘E, Ctrl+Shift+E on Windows and Linux)
  opens the panel on the view you saw last, and closes it. **Toggle
  Files** (⌥⌘F, Ctrl+Alt+F) goes straight to Files.

## 2.1.13 (smallnest/hyla)

- Open a project by dropping a folder into the sidebar: drop one or more
  folders on it, and Hyla opens a session in each project not open yet.
  Files, folders it cannot read and macOS app bundles are refused, and
  folders always open on this computer, also with a remote session
  selected. Thanks @p1xelll
- Hyla the frog sits on the window's edge: it hops when a session needs
  you, reads along while an agent works, and dozes when all is calm.
  Drag it along the edge, or turn it off in **Settings › Appearance**.
- A **Sort and Group** menu in the sidebar orders sessions by when they
  were created, and groups them by project, with a colored initial for
  each project. Drag a project's name to reorder the sidebar, and fold a
  project to hide its sessions.
- **Settings › Appearance › Background opacity** goes from 0% to 100%,
  with a blur toggle, so the window can be see-through or solid.
- A new setting `quit-ends-sessions` ends this computer's sessions, and
  their programs, when you quit Hyla, as **Quit and End All Sessions**
  does. Other computers' sessions keep running.
- Remote hosts find `hyla` when the path to it has a space in it, on
  macOS and Windows, whose `sh` is bash and splits a list of paths at
  spaces.

## 2.1.18 (egoist/kero)

- On a Mac, programs run in Hyla can use the microphone and the camera,
  and control other apps: macOS asks first, as it does for other
  terminals. Thanks @jamdaniels for the report
- Hyla reaches SSH hosts with the SSH agent your shell sets up, such as
  1Password's, as `ssh` in a terminal does. Thanks @frankittee for the
  report
- **Close All Sessions** in a project's menu ends every session in the
  project and keeps the project. Thanks @netube99
- Pressing Command while a trackpad scroll is still coasting no longer
  changes the font size. Thanks @iamvicliu

## 2.1.17

- Codex 0.164's panes resume their own conversations after hylad
  restarts, now that one Codex app server runs every pane's hooks. On
  another machine, this works once that host gets this update. Thanks
  @voidyuu for the report
- `hyla browser snapshot` prints what a page shows as roles and names,
  each element with a ref that `click`, `type` and the rest take with
  `--ref`. `hyla browser screenshot` works on a pane not on screen, and
  `hyla browser cookies save` and `load` keep a browser's sign-ins, as
  Playwright's storage state or a `cookies.txt`.
- With Hyla's icon showing, Command+W and the close button close the
  main window rather than hide it, and Hyla keeps running in the menu
  bar or the notification area.
- macOS 27 no longer runs an AutoFill helper process for Hyla.

## 2.1.16

- **Settings › General › Show app icon** on a Mac puts Hyla in the Dock
  and the menu bar, the Dock only, or the menu bar only. Windows and
  Linux always show it in the notification area. Its menu opens Hyla
  and Settings and checks for updates. While the icon shows, closing
  the window keeps Hyla running there.
- `hyla events` prints what happens to sessions and panes as it
  happens, such as a pane coming to need you, so a script can follow
  every agent without polling `hyla status`. On another machine, it
  works once that host gets this update.
- macOS no longer runs an AutoFill helper process for Hyla.

## 2.1.15

- yazi's image previews show the image, not a box of placeholder
  characters, locally and over SSH. Thanks @voidyuu for the report

## 2.1.14

- On a Mac, Command+Left and Command+Right go to the start and end of
  the line in the shell, as in Ghostty. Thanks @eliottwantz
- Hyla speaks Japanese: pick it under **Language** in Settings.
- Powerline's rounded separators in prompts fill the line's height,
  with no gap when the line height is above 1.
- On Linux, closing Settings no longer closes the app.

## 2.1.13

- Mouse clicks and drags work in herdr and other apps built on
  crossterm, locally and over SSH.
- Programs that change what Backspace or Alt sends, or how the keypad
  works with NumLock, get the keys as they asked for them.

## 2.1.12

- Pasting an image alone, such as a screenshot, into a pane pastes the
  path of a file with it, which agents like Claude Code attach as an
  image. Before, it pasted nothing. Thanks @jorben
- **Settings › Terminal › Font** lists every monospaced font you have
  installed, such as Nerd Font builds, not only the common ones.
  Thanks @smallnest
- The Changes panel no longer closes the app when it shows Python code
  with a single-quoted f-string, such as `f'{x}'`.

## 2.1.11

- You can drag the terminal's scroll bar. It shows when you scroll or
  point at the right edge of a pane, even at the bottom, widens under
  the pointer, and fades out a second after you stop.
- In panes on Windows, Ctrl+C ends programs such as `vite` or
  `ping -t`. Windows hosts get it with their next update.

## 2.1.10

- On Windows, pasting with Ctrl+V into a text field, such as a host's
  address in **Settings › Hosts**, pastes once, not twice.

## 2.1.9

- On a Mac, Command+Backspace clears the line you're typing in the
  shell, as in Ghostty. Thanks @eliottwantz

## 2.1.8

- With the Tailscale app on a Mac, **Settings › Hosts › Let other
  machines connect** no longer says Tailscale isn't running when it is,
  and hylad listens on your tailnet as it should.

## 2.1.7

- Hyla can update a host that runs the Hyla desktop app, not only the
  server build: turn on **Settings › Hosts › Let them update Hyla here**
  on that computer (`remote-updates = true`), and its project in your
  sidebar shows **Update** when it's behind. The app there updates,
  relaunches and restarts hylad. It's off unless you turn it on, and the
  host needs 2.1.7 itself, so update it there once more first.

## 2.1.6

- A session open in Hyla on two machines no longer takes the size of a
  machine you've walked away from: Hyla sizes it for the one you're at,
  and gives it back to the other as soon as you return there or this one
  quits or sleeps. Hosts get it with their next update.
- Panes and sessions on a Windows host are named by their folder
  (`mygo`), not its whole path (`C:\Users\you\dev\mygo`).

## 2.1.5

- A session open in Hyla on two machines takes the size of the one you're
  using, the one you last typed, scrolled or switched to the session in,
  rather than of the last to resize it. Resizing a split on one machine
  no longer cuts off the bottom of the pane on the other. Hosts get it
  with their next update.
- Dragging the divider between panes, or the sidebar's edge, leaves the
  keyboard in the pane you were typing in.
- After an update, Hyla offers to restart hylad only when the update
  changed it.

## 2.1.4

- The folder picker and the command palette scroll with the mouse wheel
  and trackpad, rather than snapping back to the highlighted row. Thanks
  @ak4duy.

## 2.1.3

- On Windows, turning on **Let other machines connect** in Settings ›
  Hosts no longer flashes a console window open, nor does Hyla on
  another machine connecting to it.
- Settings › Hosts leaves the commands for setting up a host to the guide
  it links to.

## 2.1.2

- Settings › Hosts can share this computer with Hyla on your other
  machines: turn on **Let other machines connect**, and copy the address
  it shows to add it there. hylad then listens on your tailnet, and only
  your Tailscale account may connect. hylad now follows `listen` and
  `listen-allow` in the config file as they change, without a restart,
  and a listener turned off drops the connections it took.
- Hosts can be edited in Settings › Hosts: a host renamed keeps its
  connection and sessions, and one given another address connects again.
  The page links to the guide to setting up hosts.
- On Windows, a restored pane's shell starts under its history, rather
  than drawing what you type at the top of the pane.

## 2.1.1

- On Windows, the mouse wheel scrolls full-screen programs that take the
  mouse as they start, such as pi, rather than stepping through their
  prompt history. Hyla now runs panes in the newer console Windows
  Terminal uses, which it ships, so programs get the mouse, the wheel and
  what they draw as they would there. Windows hosts get it with their
  next update.

## 2.1.0

- Browser panes show web pages beside your terminals: the dev server an
  agent works on, the docs it reads, the pull request it opened. One
  splits, zooms, moves and closes like any pane, and comes back on the
  same page when Hyla restarts. Open one with <kbd>⌘</kbd> <kbd>⇧</kbd>
  <kbd>B</kbd>, by typing an address in the command palette, or with
  `hyla split --url localhost:3000`; <kbd>⌘</kbd> <kbd>L</kbd> goes to
  its address bar, which shows the site's icon.
- `hyla browser` drives a browser pane's page the way `hyla send` and
  `hyla read` drive a terminal: it opens pages, clicks, types, waits for
  elements or text, reads the page's text and HTML, runs JavaScript and
  takes screenshots. Like `hyla do`, it needs Remote Control.
- Browser panes block ads and trackers, with EasyList and EasyPrivacy to
  begin with. Settings › Browser turns it off or picks other filter
  lists, or `browser-block-ads` and `browser-filter-list` in the config
  file, which takes the URL of any list in Adblock Plus syntax too.
- With `focus-follows-mouse`, the pane the pointer moves into gets the
  focus.
- Dim unfocused terminals, in Settings › Terminal, or
  `dim-unfocused-terminals = false`, keeps the terminals you're not in
  from fading.
- The command palette finds commands and Settings pages by keywords, and
  by their English names in any language.
- Hyla shows sessions that run on other machines, such as a Linux dev box,
  a cloud VM or a WSL distribution, in the same sidebar, tagged with each
  host's name, with their statuses and notifications. Add a host in
  Settings › Hosts by the address you'd give ssh, `me@devbox`: Hyla
  reaches it over
  SSH with your keys and agent, or through hylad's listener on your
  tailnet (`listen = tailnet://:7373`), which Tailscale tells who
  connects to. One compressed connection carries all of a host's panes,
  and a host whose link drops shows its sessions faded until Hyla
  reconnects on its own.
- On a host, New Session, Open Project (which lists the host's folders),
  Command-clicking a path (which opens it in the host's `$EDITOR`) and
  pasting an image or files (which copies them to the host) all act on
  the host. Its programs drive the app with `hyla focus` and `hyla do`
  only once you turn on its Remote Control.
- The sidebar stays flat with hosts: their projects follow yours, each
  tagged with its host's name, so the same repository on two machines
  sits side by side. A host has a row of its own only when it needs you,
  as when it can't be reached; one with no sessions doesn't show. Its
  menu is in its projects'. Each host wears a color, picked from its name or set
  with `color=`, on its tag and its name in a pane's header. With hosts,
  Open Project lists any machine's folders in Hyla, with a switch
  between them.
- The server build of Hyla runs hylad and the CLI on Linux, macOS and
  Windows with no app, display or GTK: `curl -fsSL
  https://github.com/egoist/kero/releases/latest/download/install-server.sh
  | sh`, or on Windows `irm .../install-server.ps1 | iex`. The app offers
  to install it on a host that has no Hyla, and to update it to the app's
  version, signed as the app's updates are. Windows hosts are reached
  over OpenSSH, whose shell may be cmd or PowerShell.
- `hyla -H <host>` and `$HYLA_HOST` run commands on a host's sessions; `hyla
  hosts` adds, checks and installs hosts; `hyla clients` lists who is
  connected to hylad; `hyla daemon install-service` runs hylad as a
  systemd user service, a launchd agent or a Task Scheduler task, and
  `hyla daemon serve` in the foreground.
- hylad's socket is its user's from the moment it is made, and hylad
  refuses connections of other users' processes. The login shell is
  looked up through NSS, so LDAP and SSSD accounts get theirs.

## 2.0.11

- The changes panel no longer jumps to the bottom of the list the first
  time a long file's diff scrolls into view, and the header of the file at
  the top of the list stays there as a file is folded, rather than
  vanishing for a moment. Only the diffs in view are built, so a long list
  of changes scrolls as quickly as a short one.
- On Windows 11 24H2 and later, clearing a pane clears the console's screen
  too, so PowerShell's inline predictions no longer draw on the line the
  prompt was on before.

## 2.0.10

- Each pane's title shows the branch it works on and the lines its changes
  add and remove since the last commit, `fix-login +40 −2`, and a merge or
  rebase in progress; click it to open the changes panel. A session on a
  branch other than the repository's default shows that branch in the
  sidebar, and its tooltip how far it is ahead of or behind its upstream
  and how many files changed.
- The changes panel (Option-Command-G, Ctrl+Shift+G on Windows and Linux)
  lists the files a session changed since its agent's last turn, since the
  last commit, or since the branch left the default one, each with its diff
  below it, which follows the file as it changes. Fold a file, resize the
  panel, switch between unified and side-by-side diffs, and in Last Turn
  revert a change or mark it kept.
- `hyla diff` prints a pane's changes as `git diff` does, untracked files
  included; `--turn` and `--branch` choose what to compare against. `hyla
  status --json` and `hyla ls --json` include each pane's branch and
  changes.
- Sessions in a worktree outside its repository join the repository's
  project instead of starting one of their own.
- `git-status = false` stops Hyla running git in your repositories, and
  `diff-layout` keeps the changes panel's diffs unified or side by side.

## 2.0.9

- Only agents that show their work with a progress bar, as Kiro and
  Qwen, turn their pane working by it: Neovim's language-server progress
  no longer marks its pane as working.

## 2.0.8

- Agents no longer rename the session they start in: an unnamed session
  keeps showing its agent's own title, as "Duplicate spinner in pane
  header", not a name in words joined by dashes.
- Files dropped on a pane paste their paths quoted for the program's
  shell, newlines and PowerShell's included.
- Find in a pane highlights a match that wraps in from above the view.
- A pane's scrollback stays whole as it reaches its limit, and a restored
  pane keeps the blank rows at the bottom of its screen.

## 2.0.7

- A split pane's title leads with its program's logo: Claude Code's,
  Codex's and other agents', Node.js's for a script node runs, as vite,
  and those of bun, deno, python, vim, git and docker; a shell's is a
  bolder terminal icon.
- A split pane shows its status once, by its title, not again in the
  chip beside it.
- An unnamed session takes its name from the program in the pane you're
  in, else in the one that most needs you; `session-name` chooses
  between these and the first pane's folder.
- Clear Scrollback in a pane's context menu clears as Cmd+K does, the
  screen too, and works on a full-screen program's.

## 2.0.6

- Images programs show appear in their panes, as grok's preview of a
  pasted image, through the Kitty graphics protocol, as in Ghostty.
- Command-hovering an image's path in a pane previews the image.
- Files dropped on a pane paste their paths, escaped for the shell, as in
  Ghostty: an agent takes a dropped image as a file it attaches.
- The mouse lands where it points in programs that take it in pixels:
  Claude Code's "Jump to bottom" lights up and takes a click, and dragging
  selects its text again.
- Programs asking the terminal's colors get the theme's: Copilot CLI's
  input box is light on a light theme. Colors a program sets for the
  terminal show, and programs that follow the appearance, as Claude Code
  and Copilot CLI, change with it when it goes from light to dark.
- Box drawing and blocks keep their colors under `minimum-contrast`, and
  only rows a program fills extend into a pane's padding, as in Ghostty:
  Copilot CLI's input box and tabs draw as they should.
- A name an agent gave its session with `hyla rename` goes when a new
  conversation starts in the session's first pane.
- After updating, Hyla asks to restart hylad, whose sessions restore as
  after a restart of the Mac.

## 2.0.5

- Find in a pane's scrollback with Cmd+F (Ctrl+F on Windows).
- Text stays readable on a light background: colors meant for a dark one,
  as Claude Code's near-white grays, get enough contrast with it. The new
  `minimum-contrast` setting sets how much, as in Ghostty.
- Command-hovering a link in a terminal underlines it.
- A pane shows where its agent works: the worktree Claude Code went into,
  rather than the folder its shell started in. Links, Copy Directory and
  Reveal follow it.
- Codex sessions are told to the pane that runs them, for resuming after
  hylad restarts, rather than to the pane that first started Codex's
  background service.

## 2.0.4

- Terminals show the cursor chosen in Settings, its shape and whether it
  blinks, rather than always a blinking block.

## 2.0.3

- An agent sitting at its prompt no longer shows as needing you, as Claude
  Code did a while after answering. A bell or a program's desktop
  notification leaves its status alone; programs' notifications show as
  they are.

## 2.0.2

- Installing the command line tool on macOS asks for an administrator's
  password instead of failing.

## 2.0.1

- Hyla speaks Simplified Chinese; pick the language in Settings.
- Rename a project from its menu in the sidebar.
- `hyla skill setup` hooks Claude Code, Codex, Pi and OpenCode into Hyla as
  they start: an agent learns its session and pane, names an unnamed session
  after its task, and resumes when hylad restarts.
- `hyla new|split --wait` and `hyla wait --exit` exit with the command's
  status.
- When hylad runs another version of Hyla, a Restart Service chip offers to
  restart it. Settings shows updates on its General page.
- A home directory pane shows `~` once in its header.

## 2.0.0

Hyla, rewritten from the ground up: sessions grouped by project in a sidebar
that shows which are working, which need you and which are done. Sessions run
in hylad, a daemon of their own, so quitting Hyla leaves them running.

- Runs on macOS, Windows and Linux.
- The `hyla` CLI drives sessions from a shell or an agent.
- Updates itself from Hyla's releases.
