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
    which the sidebar and the panes read too. The confirmation and the
    window's shortcuts stay in the root.
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
  - Also: `host` is `internal/app/host`.Host — one hylad's address, its
    connection's state and error, what it holds, and the dialers — which
    the views read directly, as the root's field was. Its fields are
    exported and its methods say what they do (`Remote`, `Label`,
    `Dialer`).
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
