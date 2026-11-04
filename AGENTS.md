# Hyla

- Cutting a release: follow [RELEASING.md](RELEASING.md), from `master`.

## The branches

This repository is `smallnest/hyla`, which was the fork `smallnest/kero` of
`egoist/kero` and is a project of its own now.

- `master` is where Hyla lives: build, test and tag releases from it.
  Features branch from `master` and merge back into it.
- `main` and `dev` are the fork's branches, from before the rename: the
  history `master` comes from. Do not branch from them, and do not
  release them.
- `backup/pre-rebase-main` is a backup only.

### Update from upstream

```sh
git checkout master
git fetch upstream
git merge upstream/main
go build ./... && go vet ./... && go test . ./internal/...
git push origin master
```

Resolve conflicts on `master`, keeping both sides: the fork's features
(below) must survive a merge.

### Add a feature

```sh
git checkout master
git checkout -b feat/<name>
# ... commit ...
# merge back into master when done
```

### Take one upstream commit

```sh
git fetch upstream
git checkout master
git cherry-pick -x <sha>
```

Keep `-x`, so the commit names where it came from.

## The fork's features keep working

Each fork feature came as a pull request upstream never merged, and it
lives in `master`, not only in its branch. Merging upstream, or any branch,
must keep every one: resolve a conflict by keeping both sides, never by
taking upstream's file whole, and check them after the merge rather than
only that it builds.

| Feature | PR | Code to find | Tests |
|---|---|---|---|
| Files view in the side panel | #241 | `sideFiles`, `filesView`, `filesRoot` | `TestSideToggle`, `TestFilePane`, `TestFileTreeRel` |
| Sidebar sort and group, project badges, folded projects | #214 | `sortButton`, `sessionSidebar`, `projectBadgeColor`, `projectInitial`, `dateHeader` | `TestProjectOrder`, `TestProjectDrag`, `TestSessionAlign`, `TestCollapseSelectedProject`, `TestSidebarRowKeepsTime` |
| Background opacity and blur | #211 | `washOpacity`, `vibrancyFor`, `WindowBlur` | `TestVibrancyFor`, `TestBlurlessBackgroundIsTranslucent`, `TestTranslucentGlass` |
| The menu of installed monospaced fonts | #209 | `fontOptions` | `TestFontOptionsInstalled` |
| Hyla the frog | #206 | `pet.go`, `petState` | `go test . -run TestPet` |
| Quitting ends this machine's sessions | #219 | `quitRequested`, `quitAndEnd`, `QuitEndsSessions` | `TestQuitEndsSessions` |

```sh
for s in sideFiles filesView sortButton sessionSidebar projectBadgeColor fontOptions petState quitRequested vibrancyFor; do
	grep -rq "$s" --include='*.go' . || echo "missing: $s"
done
go test . -run 'TestSideToggle|TestFilePane|TestProjectOrder|TestProjectDrag|TestSessionAlign|TestCollapseSelectedProject|TestSidebarRowKeepsTime|TestQuitEndsSessions|TestFontOptionsInstalled|TestVibrancyFor|TestBlurlessBackgroundIsTranslucent|TestTranslucentGlass'
```

The branches the pull requests came from (`feat/files-view`,
`sidebar-sort`, `window-opacity`, `font-picker`, `hyla-pet`,
`quit-ends-sessions`) are behind `master` and may be rebased: a commit that
looks missing there says nothing about `master`. Check the code and the
tests above.

Two merges showed why. One took upstream's `view.go` whole: 2.1.21 lost
the sidebar's sort button, its colored badges and its indentation, while
the data side stayed, and nothing failed to build or test. The other left
`files.toggle` with a "this fork has no Files view" case: the menu entry
named Toggle Side Panel opened Changes. Neither shows in the app's
strings either, since a locale file keeps the words of a view that no
code opens, and a feature's test can go missing with its code.

## Which repository releases come from

Hyla here releases as `smallnest/hyla`: `mygo.json` `updates.github`,
`internal/update.Repo`, and the install scripts in `scripts/` name it.
Do not change them back to `egoist/kero`. The old releases app updates
from `egoist/kero`; new builds update from `smallnest/hyla`.

`smallnest/kero` holds what Hyla was called up to 2.1.37. Nothing is
published there any more: a 2.x app does not update itself to Hyla, and
the README tells its user to install Hyla once.

## The rename from Kero

The app, the command, the daemon (`kerod`), the server build, the
directory it keeps settings in and the `KERO_*` variables Hyla exports
were Kero's. Each keeps a way back for an install of 2.x, which do not
look like leftovers:

| Code | Why it stays |
|---|---|
| `proto.Dir`'s `adoptDir`, `AdoptLegacyEnv` | moves the 2.x directory and reads `KERO_*` as `HYLA_*` |
| `daemon.environment`'s `KERO_*` sets | a script written for 2.x still finds its session and pane |
| `settings.ConfigPath`'s `kero` config | moves the 2.x configuration file, outside the directory on macOS |
| `settings.normalize`'s `kero` aliases | the pet and theme a 2.x settings file names |
| `agentHook.ours`, `cli.removeLegacy` | updates a 2.x hook, and removes its skill and plugins |
| `clilink.dropLegacy`, `cli_install_*.go` | removes the `kero` command a 2.x left on the PATH |
| `service.removeLegacy` | removes a 2.x service or task |
| `install-server.sh` / `.ps1`'s `kero` block | replaces a 2.x server install |
| `feishu.cardValue` | answers a card a 2.x sent |

The website and the docs stay at `kero.sh`, because the domain is
Hyla's for now: a doc link to `kero.sh` is not a leftover either.

## Signing and the update key

There is no Apple Developer ID. macOS builds are ad-hoc signed (an
`xattr -d com.apple.quarantine` for a user who downloads one), and no
release takes a certificate or notarization.

`~/.hyla-release-keys/mygo-update.key` signs updates; its public half is
`mygo.json`'s `updates.publicKey`, which `internal/update.PublicKey` holds
too. Never commit the secret, and never replace the pair: every installed
app trusts only that public key.

3.0.0 was an exception: the pair of 2.x was lost, so it starts a new one.
No release is stranded by it, because a 2.x app does not update from this
repository anyway. From here on the pair stays as it is.

## Checks before a push

```sh
go build ./... && go vet ./... && go test . ./internal/...
go tool mygo i18n check
bun install && bun run --filter website build   # when the website changes
```

Before pushing a merge, run the check above for the fork's features too.

`bun install` rewrites `bun.lock`. Restore it unless the dependencies
really changed.
