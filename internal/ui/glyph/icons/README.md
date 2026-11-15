# Icons

- `f7-*.svg`: [Framework7 Icons](https://framework7.io/icons/) (MIT, © Vladimir Kharlampidi), drawn after Apple's SF Symbols.
- `meteor-*.svg`: [Meteor Icons](https://github.com/zkreations/icons) (MIT, © zkreations).
- `si-*.svg`: [Simple Icons](https://simpleicons.org) (CC0), programs' logos; the brands are their owners' trademarks.
- `octicon-*.svg`: [Octicons](https://primer.style/octicons) (MIT, © GitHub), for what Framework7 has no icon of, as a file's diff.
- `lucide-*.svg`: [Lucide](https://lucide.dev) (ISC, © Lucide Contributors), with a heavier stroke than Iconify serves (`stroke-width`), as SF Symbols' semibold: 2.5 for `chevrons-up-down`, 3 for `check`.

These are `package glyph`'s (`internal/ui/glyph`), for every icon Hyla draws
as `glyph.Of("f7-gear-alt")`.

The Files tree's file and folder icons are elsewhere: [Pierre Icons](https://diffs.com/icons) (MIT, © The Pierre Computer Company) in `internal/fileicons/icons/files` (LICENSE.md there), and the coding agents' marks, which the sidebar puts before a session's name, in `internal/ui/marks/agents` (README.md there).

All but the Pierre set's as [Iconify](https://iconify.design) serves them: `https://api.iconify.design/f7/<name>.svg`.

The app's icon files, which the bundle and its installer use, are in
`resources/` and `internal/ui/tray/icons/`: `internal/ui/tray/icons/tray-template.png` is the frog of the app's icon (`resources/icon.png`) as a silhouette of its green, without the dark outline, black on transparent for macOS to tint in the menu bar, with its eyes, mouth and paws cut out of it: 14 points tall, centered in 18×18 points, at 2×. `internal/ui/tray/icons/tray.png` is the app's icon at 64×64, for the notification area of Windows and Linux.
