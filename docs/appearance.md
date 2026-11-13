---
title: Appearance
description: Make the window yours — translucency, blur and materials, pictures behind the sidebar and terminals, the session-switch animation, and fonts.
---

# Appearance

Hyla's window is drawn to sit on your desktop, not in front of it. **Settings › Appearance** edits all of this, and the [configuration file](./configuration.md#appearance) holds the same keys.

## Translucency and materials

| Key                 | Does                                                        |
| ------------------- | ----------------------------------------------------------- |
| `window-translucent` | Lets the desktop show through the window                    |
| `window-opacity`    | How much the window's wash covers, 0 to 1                    |
| `window-blur`       | Blurs the desktop behind the window; off is plain transparency |
| `window-material`   | What the window is made of: `sidebar`, `window`, `popover`, `plastic`, `glass` (Liquid Glass, as the Dock's) or `glass-clear` |
| `pane-opacity`      | The same, for the terminals' cards, so the terminal can be clearer than the sidebar |
| `pane-blur`         | Blurs what shows through a terminal                          |
| `pane-material`     | The terminal's own glass: `glass`, `plastic` or `glass-clear` |

The text always stays opaque, whatever the wash behind it: turning the opacity down changes what you see around the letters, never their contrast.

## Pictures

Two pictures can sit behind the work: one behind the sidebar's sessions, one in the terminals' cards.

| Key                    | Does                                                    |
| ---------------------- | ------------------------------------------------------- |
| `sidebar-image`        | A picture behind the sidebar's sessions                 |
| `sidebar-image-opacity` | How much of it shows                                    |
| `pane-image`           | A picture in the terminals, under their text            |
| `pane-image-opacity`   | How much of it shows                                    |
| `pane-image-fit`       | `fill`, `fit`, `stretch`, `center` or `tile`, as a desktop picture's options |

## Switching sessions

`session-switch` decides how the next session comes in: `dissolve` fades it in as the last fades away; `slide`, `push` and `cover` slide it in from the right; `zoom` grows it into place; `none` shows it at once.

## The wash

`background` is the colour the window is washed with: `petal`, `mist`, `meadow`, `plain`, or `dock`, the Dock's own glass under a thin veil. `window-theme` follows the system's light or dark appearance, or holds one of them.

## Fonts

The terminal uses JetBrains Mono Nerd Font, which Hyla includes, unless `font-family` names another. The font menu in **Settings** lists the monospaced fonts installed on this machine, not a fixed list, so a font you installed is in it.

`font-weight` runs from 100 to 900, and Hyla draws the 200 and 300 weights itself when the font has none. `font-ligatures`, `font-thicken`, `line-height` and `minimum-contrast` are the rest of the type, as [Configuration](./configuration.md#terminal) lists them.
