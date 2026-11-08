# Hyla's site

`../index.html` is the entry point. This folder holds everything else the page
needs:

- `style.css` — the light, paper-and-ink theme.
- `site.js` — the menu, the reveal-on-scroll, the copy buttons, the frog.
- `images/` — the app icon, a screenshot of Hyla's Settings, and one of
  its window at work.
- `evidence/` — the before/after captures used when reviewing the change;
  they are not loaded by the page.

There is no build step. Serve the `docs/` directory from any static server, or
open `docs/index.html` directly.

## The screenshots

`images/appearance-top.webp` and `images/everyday.webp` were captured from the
Hyla app running on this machine (`macOS`, light appearance): the Settings
window over a session, and the window at work, with a Claude Code session in
it.
The rest of the page's window is a labelled HTML/CSS illustration.

`evidence/before-after.jpg` compares the old `kero.sh` landing (dark) with
`docs/index.html` (light); `after-features.jpg` and `after-shots.jpg` show
the new page's feature grid and screenshot section.
