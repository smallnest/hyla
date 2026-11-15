# Plan: the root package's move into internal/

Status: Step 0 (the layer rules) and Step 2's first moves are built, on
`refactor/step0-1-arch-and-moves`. Written 2026-11-15, after 3.10.3.

## Where we start

Hyla's root is one `package main`: about 87 Go files and 30,000 lines,
the app, its views, its menus, and the code it shares with hylad. A
package of that size is hard to read and hard to change: anything can
reach anything, and a file's place says nothing of what it depends on.

The packages that already moved into `internal/` show what a piece of
the root should look like: `proto`, `settings`, `actions`, `client`,
`daemon`, `git`, `pets`, `shell`, `cli`. The root files import them
heavily, and almost every helper in the root is a method on `App`.

## The target

`package main` keeps `main.go`, `app.go` (the `App` struct and its
lifecycle), and the wiring that starts it. Everything else moves into
`internal/`, in layers that import only downward:

```
root      main.go, app.go, the wiring
  ui      the views: sidebar, panes, settings, dialogs, theme, icons
  app     the app's state and behaviour: browser, hosts, files, sync
  engine  hylad, the CLI, and what they run on
  base    proto, settings, git, shell, and the like
```

`internal/arch` holds the rules as data and as a test: each package of
`internal/` is of a layer, and a package imports only from its own
layer's list and below it. The guard runs `go list` for darwin, linux
and windows, so a file of one platform's imports cannot slip by.

### Why the lists are exceptions

A layer's list names every package of that layer and below that its
packages import, and nothing else. A package that moves down leaves the
list that names it stale, and the guard fails: moving a package means
editing the layers, which keeps them honest.

## What `App` needs first

`App` cannot move until its fields can. Each field is of a type the root
holds (`host`, `termEntry`, `chrome`, `paletteState`, and some twenty
more), and each of those is used by the views. The work is to take the
app apart by domain, and to give each part a package of its own:

| Part | Fields today | Package |
| --- | --- | --- |
| browser panes | `webs`, `ads`, `icons` | `internal/app/browser` |
| file panes | `files` | `internal/app/files` |
| terminals | `terms`, `refs`, `focus`, `claims` | `internal/app/terms` |
| hosts and their connections | `hosts`, `owner`, `st`, `loaded` | `internal/app/hosts` |
| preferences and settings | `prefs`, `set`, `applied`, `ss` | `internal/app/prefs` |
| the sidebar's state | `side`, `chrome`, `drag*`, `art` | `internal/app/side` |
| Hyla the frog | `pet`, `petSet`, `petList` | `internal/app/pet` |

Each is a struct of its own with methods, embedded in `App` (exported,
as the fields are), and the views call it as they call `a.set` today.
A part moves into `internal/` when it no longer reads another part's
fields directly.

## Steps

Each step is a pull request of its own: it changes structure, not
behaviour, and `go vet ./...` and `go test ./...` pass.

- **Step 0: the layer rules** — done. `internal/arch`, with the guard
  test and the unit tests of the rules.
- **Step 1: `App`'s fields exported** — the fields named as JSON and
  used by other files need to leave the root. `app.go` is 2,300 lines
  of methods; its parts go as the table above names them.
  - Started: `sync` is `internal/app/sync`'s `State`, embedded in `App`,
    which the state reads through its `Hosts` interface (whether a state
    came, the main thread, and the lines it says). The Sync page's view
    stays in the root, `sync_page.go`, until the views are `internal/`.
  - Also: `App`'s fields are exported, and Magpie's reads are
    `internal/app/magpie`, which the Info view's card calls.
  - Also: the file panes' reads are `internal/app/filepane` (daemon
    layer, as they call a client), with the viewer's bounds and
    `IsBinary`, which the Files view shares. The Info views' reads are
    `internal/app/paneinfo`, in the same layer and the same shape.
  - Also: where the pet stands is `internal/app/petspot`, which the pet
    window and the pet's drawing read. The Settings search's entries and
    match are `internal/ui/settings/search`; the pages and the actions'
    titles stay in the root, as the views do. A worktree's git state in
    words is `internal/app/gitinfo`; the palette's and the picker's
    scoring is `internal/ui/fuzzy`.
  - Also: the command palette is `internal/ui/palette` — its state, the
    items it finds (sessions, conversations, the actions, the pages and
    the restart), and its view, which take an `App` interface of what
    they ask. What they draw of a session — its status icon, a
    shortcut's keys and how long ago a time was — is `internal/ui/paints`,
    which the sidebar and the panes read too. The window's shortcuts
    stay in the root.
  - Also: the folder picker is `internal/ui/picker` — its state, the
    listing of a host's folders (`fs.list`), the items and the view —
    over the same `App` shape; the root keeps `openProjectOn`,
    `pickerGo` and `pickerView` as its adapters.
  - Also: the sidebar's ended and archived conversations are
    `internal/app/history` — the hosts' asking, the sections and their
    menus — in the ui layer, which reads `internal/cli`'s skill states as
    its own `SkillState`, so the layer map need not name `cli`. The
    icon button and the pointer tooltip it draws with moved there too,
    and the root keeps thin adapters.
  - Also: the side panel's Info view is `internal/ui/info` — the pane's
    directory, processes and ports, each a card of `info.Card`, and the
    fold keys. Magpie's and Feishu's cards draw through it from the root.
  - Also: the Files tab and its viewer are `internal/ui/files` — the
    tree, the listing, the reading, and the making, renaming and
    deleting of entries, over one state of exported fields. The icon
    button, the pointer tooltip, `Stamp` and `ReplaceText`, which the
    sidebar and the Changes view share, are `internal/ui/paints`.
  - Also: the confirmation the app asks before it cannot undo
    something is `internal/ui/confirm` — its question, its two
    actions and the dialog that draws and keys it. The callers in the
    root, the palette and the Files tab fill a `confirm.Ask` and leave
    it to `App.Confirm`.
  - Also: the Settings window's groups, rows, segmented control and the
    search's marks are `internal/ui/settings/kit`, which the pages, the
    Sync page and the hosts' forms draw with.
  - Also: `hyla browser`'s commands — a page's text, its clicks and
    typing, its snapshot and picture, and the cookies — are
    `internal/app/browse`, in the daemon layer, as they drive a page;
    the root keeps `automate.go` as its adapters, and the entries'
    loads.
  - Also: the side panel's Changes view is `internal/ui/changesview` —
    its state and the files' diffs, the bases it lists since, the
    reading of a file's text, and the folding, the side-by-side layout
    and the Revert and Keep of a turn's changes. The root keeps the
    panel's shell, which holds the state.
  - Also: the side panel itself is `internal/ui/side` — its views'
    names and sizes, opening and closing it, the edge a drag moves, the
    moved card and its tabs — over an `App` of the views it shows,
    which the root's `side_adapters.go` gives it. The Info view's
    place, its scroll, is the panel's state's too.
  - Also: the panes' actions — a split, a close, a zoom and the moves
    between the panes — are `internal/ui/panes`, which the actions, the
    panes' controls, a browser and the Info view call; the root keeps
    `panes_adapters.go`, and the session a replaced one arrives in.
  - Also: the machines the app reaches — the entries they are added by,
    their connecting and going, and the Settings pages' P2P and Tailcat
    lists and the sidebar's headers — are `internal/ui/hosts`, over an
    `App` whose names the palette's `Local` and `SelectSession` already
    take (`PaletteLocal`, `PaletteSelectSession`), as `DropTerms` does
    (`DropHostTerms`). The forms' drafts moved to
    `internal/ui/settings/draft`, which the pages and the machines' read.
  - Also: the Tailcat and network listeners' group — its forms, the
    machine's pair code, its rotation and what a phone scans — joined
    `internal/ui/hosts`, which changes the settings through
    `App.SettingsNow`.
  - Also: the find bar over a pane, a terminal's or a browser's, is
    `internal/ui/findbar`, which `find.go` answers.
  - Also: the browser panes — the page, its toolbar and address bar, its
    error and find — are `internal/ui/webpane`, whose `Entry` is the
    app's `Webs`. It draws the panes' buttons through `App.Panes()`.
`browser.go` is its adapters.
  - Also: the quick terminal — its panel, the session it shows, its
    slide and its shortcut — is `internal/ui/quick`, over an `App` of the
    app's window and the side panel's state. `quick.go` is its adapters,
    and `quickTerm` its state; the `gated` stream it attaches with is
    `quick.Gated`.
  - Also: the pet is `internal/ui/petme` — its mood and the words it
    says, the frog it draws, the Codex pets' spritesheets, the pets it
    lists and the window that follows the main one. Its state is the
    app's field, and `pet.go` is its adapters. The app's focused flag is
    its own field, which `Focused` answers, as `focused` had to leave.
    `petspot` moved from the engine layer to the ui layer, which the
    pet's window reads it from.
  - Also: the machines of the sidebar's SSH project — the dialog that
    adds one, the rows they show as, and their terminals — are
    `internal/ui/sshdialog`, beside the dialog's view. Its `Host` is the
    app's prefs' machine, and `ssh_dialog.go` is its adapters. The row's
    drag and its insertion line are the machines' own.
  - Also: the pods of the sidebar's K8s project — the kubectl exec the
    dialog reads, the rows they show as, and their terminals — are
    `internal/ui/k8sdialog`, beside the dialog's view; its `Pod` is the
    app's prefs' pod, and `k8s_adapters.go` is its adapters. The SSH and
    K8s rows keep the app's `DragAtAddr` and `DragAfterSide` for their
    insertion lines.
  - Also: the Feishu page is `internal/ui/settings/feishu.go` — its
    groups, its app's check and secret, and the bot's state — and the
    environment the bot's agents run in is the package's `AppEnv`, which
    the app reads as it applies its settings. The page asks the app for
    this machine, and for a call on it, as `Local` and `CallOnLocal`.
  - Also: a paste of the clipboard's images and files, which a pane's
    terminal cannot carry, is `internal/app/paste` — the images it saves
    for a program, the paths it pastes and how it quotes them — and
    `paste_adapters.go` is its adapter. The app's `termFor` is `TermFor`,
    as the package reads it.
  - Also: the menu, its items, the actions' titles and the shortcuts they
    show are `internal/ui/menus`, which the tray and the palette read
    too. It is of the cli layer, as it reads Settings' key names;
    `actions.go` is its adapters, and an action's run as a menu's item is
    `RunID`, as the palette runs one by its own name.
  - Also: what the app makes of the machines it reaches — the state it
    merges from their hylads', which host each session is of, and how it
    stands with each — is `internal/ui/hostlink`, which `hoststate.go`
    answers. The app's `sessionsOf` and `projectsOf` are its `SessionsOf`
    and `ProjectsOf`, over `sessionsOn` and `projectsOn`.
  - Also: what the sidebar lists — the groups of a project's sessions,
    their order by drag, by created or updated time, and by date — is
    `internal/ui/sidebar`, whose `Group` and `DateGroup` hold session
    ids rather than the state's sessions, as the state moves as they
    read it. The preferences it sets are a copy's: `PrefsNow` gives the
    app's own back, as its `SetProjectOrder` and `SetSessionOrder` set
    them.
  - Also: the icon in the menu bar or the notification area, its menu and
    its images are `internal/ui/tray`, of the cli layer as it reads the
    menus' titles; its images moved from the root's `icons/`. `tray.go`
    is its adapters, and keeps the window's closing, which the icon makes
    the app's to.
  - Also: the panes' drawing is `internal/ui/panes`, beside their
    actions: the card, the layout's nodes and its divider, the header and
    its controls, the terminal's body, the drop preview and the moves a
    drop makes. `PaneCard` reads the session as the app has it when it
    builds, as the header's `ensureTerm` can take the keyboard from a
    pane the app has lost. The switch that brings a session in stays in
    the root's `panes.go`: it reads and writes the app's own state.
  - Also: the Chat view is `internal/ui/chat` — a session's windows,
    their reads (`feishu.feed`) and the lines. The root keeps the state
    in the side panel's struct, and `chat.go` as its adapters.
  - Also: the browser panes' icons are `internal/app/favicon` — the
    cache, the disk, the download and the decoders, in the engine layer —
    and `internal/ui/webicon`, which asks a page for its icons and the
    app's cache what it holds.
  - Also: the dialogs that add an SSH host and a kubectl pod are
    `internal/ui/sshdialog` and `internal/ui/k8sdialog`; the root keeps
    the hosts they add, and the name a machine takes while none is typed.
  - Also: the dialog of a new session's worktree is
    `internal/ui/worktree`; its closing question stays in the root, with
    the confirm dialog.
  - Also: the Settings window is `internal/ui/settings` — its state
    (`State`, with the hosts' and Feishu's drafts), its pages, the
    keyboard's shortcut recording and the window's opening. It asks an
    `App` of the settings, the window and what the controls change, and
    names its own layer "cli", as it reads `internal/cli`'s skills; the
    root keeps `settings.go` as its adapters. The Sync, Feishu and hosts
    pages stay in the root, which the window draws through `App.Sync`,
    `App.Feishu`, `App.Tailcat` and `App.P2P`. The pages read an `App`,
    not the app: `App.Machines` is the hosts' field, renamed from
    `Hosts`, as `HostCount` answers the window.
  - Also: `host` is `internal/app/host`.Host — one hylad's address, its
    connection's state and error, what it holds, and the dialers — which
    the views read directly, as the root's field was. Its fields are
    exported and its methods say what they do (`Remote`, `Label`,
    `Dialer`).
  - Also: Magpie's card in the Info view is `internal/ui/magpieview` —
    its state (`Info`, `Usage`), its reading, its tick and the pane's
    agent session. The root keeps `magpie_adapters.go`, which holds the
    app's `Magpie` field as `magpieview.Info`.
  - Also: the image a path printed in a pane shows while Command hovers
    it is `internal/ui/panes`.Peek — the flyout, its decoding and its
    view, with `End` at the end of the frame the app draws. `panes.Path`
    is the file a link names, the root's `resolvePath` among the
    `Peeker` callbacks.
  - Also: the pictures of the Appearance page are `internal/ui/art` —
    the `Picture` the sidebar and the terminals show, choosing one with
    the system's dialog, decoding one off the main thread, and where the
    sidebar's goes. The root keeps the pickers' paths, the sidebar's
    view, and `pickImage`, which asks with the Settings' window.
  - Also: the Settings › Sync page is `internal/ui/syncedit`, which
    draws `internal/app/sync`'s `State`; the root keeps its page's
    `SyncState`, `SyncSaved` and `SyncRun` as the callbacks, while
    `App.Sync` stays the Settings window's page.
  - Also: where the window's parts go is `internal/ui/layout` — the
    title bar's middle and the sidebar's top, a card's gap, the rows'
    heights, and `Chrome`, what the sidebar's slide leaves the cards'
    column and the header, with `For` and `SidebarMotion`. The views
    read them as they drew them.
  - Also: the sidebar is `internal/ui/sidebarview` — its buttons, the
    sessions under their projects or days, a machine's under its name,
    the rows' menus and the foot that tells what wants a look. Its
    `App` names what a row asks (`Shortcut`, `NewSessionOn`,
    `CloseSession`); `App.Root` is the app as the order asks it. A
    project's square and its first letter, and a drag's line, are
    `sidebar.BadgeColor`, `sidebar.ProjectInitial` and `sidebar.DragLine`.
  - Also: the card of the session shown is `layout.Card`, which asks a
    `layout.Carder`: its panes, the side panel beside them, the card of
    no session with its messages and the empty state, and the app's
    settings through `Glass`. The root keeps `card_adapters.go`. The
    root's `sessionSwitch` is `panes.SessionSwitch`, which `panes.Switch`
    names; its ids are the settings' strings.
  - Also: the window's row is `internal/ui/root` — the sidebar's slide,
    the sidebar's picture and its drops, the card, the pages a screenshot
    stages, the dialogs, and the frame's end with its toasts, its peek
    and the actions the view asked for. The root keeps `root_adapters.go`
    and `view.go`'s sidebar handle; the settings' window and the
    machines' pages share a name with some of these, which take another
    (`AppSettings`, `PickerView`).
  - Also: a machine's connection is `internal/app/link` — the dial, the
    state a hylad sends as it comes, its events, the retry, the wait and
    the reselect. It is of the ui layer, as it reads the hosts' states as
    the views do. The root keeps `hosts.go`'s lookups and `link_adapters.go`,
    whose names take the "Now" suffix where the app's own already take
    theirs (`Local`, `Update`, `ReportView`, `MergeState`, `Reselect`,
    `DropTerms`), and `Connected` is exported for the test that drives it.
  - Also: `automate.go` is `internal/app/shell`, which is of the ui layer
    as it holds a page — the commands a shell sends the window, `hyla do`
    through `browse` and `hyla browser` through its page, made if the pane
    had none. `shell_adapters.go` answers them.
  - Also: `worktree.go` joined `internal/ui/worktree/app.go` — the ask of
    a new session's branch, and whether a closed session's worktree goes
    with it, asked off the main thread through the app's call. Its `Sess`
    asks more than the dialog's `App`: `AskNew`, `AskClose`, `Create`,
    `EndSession` and `UpdateNow`. `root_adapters.go` answers them.
  - The guard now fails on a package of `internal/` that no layer's
    names hold. It found `buildinfo`, `fileicons` and `profiling` never
    listed.
  - The table's parts that the views read deeply — `hosts` (the
    connection, the error, the conversations) and the sidebar's state —
    wait for Step 4: a part moves only when it reads another's fields
    nowhere, and the views read these on nearly every frame.
- **Step 2: the packages with no view** — done: `profiling`,
  `buildinfo`, `fileicons`, `adblock`.
- **Step 2b: the views' foundations, which needed no `App`** — done:
  `internal/ui/wash` (the window's and the cards' color),
  `internal/ui/theme` (the palette, the terminal's colors, the backdrop
  and the card materials), `internal/ui/glyph` (the icons),
  `internal/ui/marks` (the agents'), `internal/ui/style` (surfaces and
  buttons), `internal/ui/controls` (pop-ups, menus, fields, sliders).
  Also done: `internal/platform` (`fileManager`, `openPath`,
  `revealPath`, `openTextFile`, `framed`), `internal/app/magpie` (the
  Info view's reads of Magpie). `windowControls` stays in the root: it
  draws Hyla's own buttons, and reads the header's metrics.
- **Step 3: the app's behaviour** — `automate.go`, `browser.go`,
  `hosts*.go`, `files*.go`, `sync.go`, `branch.go`, `worktree.go` into
  `internal/app/`.
- **Step 4: the views** — `theme.go`, `palette.go`, `icons.go`,
  `elevation.go`, `controls.go` into `internal/ui/theme`; `view.go`,
  `side.go`, `panes.go` into `internal/ui/layout`; the settings views,
  the dialogs, and the pets, each into its own package.
- **Step 5: the end** — `package main` is `main.go` and `app.go`, and the
  architecture test names every package.

## What a move looks like

1. `git mv` the files and their tests into the package, `package name`.
2. Export what the root calls: a method on `App` becomes a method on a
   part, or a function that takes the arguments it needs.
3. Where a moved file reads `App`, pass the values in, as `adblock.State`
   takes the setting, the lists, the main-thread runner and the pages'
   callback. That is the seam the next move uses.
4. Add the package to its layer's list, and run the guard.
5. `go generate ./internal/buildinfo`, then `go vet ./...` and
   `go test ./...`.

## Notes from the first moves

- **`buildinfo` embeds a copy.** `//go:embed` reaches only a package's
  own directory, and the code hylad runs is the module's, so
  `scripts/sync-buildinfo.py` copies it into `_hyladSource`, which a
  leading underscore keeps out of `./...`. `go generate ./internal/buildinfo`
  makes it, and a build fails without it.
- **`fileicons`' SVGs moved with it**, and `scripts/fileicons` writes to
  the new place.
- **The ad blocking tests** now set their hooks through
  `adblock.Hook`, rather than setting the root's unexported variables.
