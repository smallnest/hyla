---
title: Drag & Drop
description: Drop files on a pane to paste their paths, drag a pane onto another to split or swap, and drop folders on the sidebar to open projects.
---

# Drag & Drop

Files, folders and panes you drag inside Hyla go where they land.

## Files into a pane

Drop files on a pane and Hyla pastes their paths into the program, quoted the way the shell takes them. A dropped folder is walked: its files go in one line, up to 2000 files and 8 levels deep.

On a pane that runs on [another machine](./remote.md), the files are uploaded first and then pasted, so the program sees them where it runs.

## Panes onto panes

Drag a pane by its title bar onto another pane and it previews the room it will take:

- onto the **left**, **right**, **top** or **bottom** edge, it splits that pane and sits beside it;
- onto the **middle**, it swaps places with it.

The preview glides from zone to zone while you drag, so you can see where the pane will land before you let go.

## Folders onto the sidebar

Drop a folder on the sidebar to open it as a project. Dropping several opens one session per project that is not already open; folders already registered reuse their project. Files, and folders Hyla cannot read, are rejected. This always opens on this machine, even when a remote session is selected.
