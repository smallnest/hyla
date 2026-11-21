# Hyla IPTV Plugin (`hyla.iptv`)

Minimalist Apple TV–style live IPTV pane for Hyla as an out-of-process external plugin.

## Features

- **Single & Batch Source Import**: Paste one M3U URL, multiple URLs (one per line), raw M3U/M3U8 text, TVBox `#genre#` TXT lists, or a web page that holds `.m3u` links (the pane finds its playlist). The presets come from [iptv.md](../../iptv.md) of the repository.
- **Apple TV–Inspired UI**:
  - Cinema 16:9 stage with auto-hiding translucent HUD (`LIVE` indicator, resolution badge, stream line switcher, Picture-in-Picture, full-pane theater mode).
  - Clean channel poster tiles highlighting **channel logo + channel name** (with monogram fallback and disk-cached logos).
  - Horizontal category pill rail (`央视频道`, `卫视频道`, etc.), instant search, favorites, and recently watched history.
- **Program Guide (EPG)**:
  - Reads the XMLTV guides that the playlists name (`x-tvg-url`), in UTF-8 or GBK, and keeps only the programmes of your channels, three days of them.
  - "On Now" rail with progress, the current programme on every channel tile, and a Guide tab with a row per channel (now, next, the day's line-up).
  - The player shows what is on and what comes next, with the time left.
  - Channel names match the guide the way the guides write them: `CCTV1`, `CCTV-1`, `CCTV1综合`, `CCTV5PLUS` and `CCTV5+` all find their channel.
- **Reliable Local Streaming Proxy**:
  - Token-gated `127.0.0.1:0` loopback media server rewriting nested HLS (`.m3u8` / `.ts` / `#EXT-X-KEY`) URLs with strict allowlisting (no open proxy) and direct-first smart proxy fallback.
  - Automatic multi-line deduplication when adding multiple sources with overlapping channels, plus automatic stream line failover.

## Keyboard Shortcuts

- `Space` / `K`: Play / Pause
- `ArrowUp` / `ArrowDown`: Previous / Next channel
- `M`: Mute / Unmute
- `F`: Toggle Full-Pane Theater mode
- `/`: Focus channel search
- `G`: Show the Guide tab
- `Escape`: Exit Theater mode / close modals

## Local Development

```sh
(cd plugins/iptv && go build -o bin/hyla-iptv .)
hyla plugin link /Users/smallnest/ai/hyla/plugins/iptv
```

Once enabled, open via **Pane › Open IPTV Pane** (`⌥⌘L` / `Ctrl+Alt+L`) or Command Palette (`⌘P`).

## Program Guide

The guide comes from the `x-tvg-url` of the playlist (`#EXTM3U x-tvg-url="..."`), and a playlist may name several, comma-separated. The plugin reads a guide on start, keeps it in `epg.json` of the plugin's data directory, reads it again after six hours, and again when a source changes. **Refresh Guide** in the Guide tab reads it at once.

## Sources from iptv.md

The pane shows the sources of the repository's [iptv.md](../../iptv.md) as presets. It reads that file at start from the `master` branch:

```
https://raw.githubusercontent.com/smallnest/hyla/master/iptv.md
```

A change to a row of its table reaches every pane without a new plugin. The pane keeps the last good read in `presets.md` of its data directory, so a network that cannot reach GitHub keeps the sources that it had. If there is no read yet, the pane shows one built-in source: the best-fan playlist for live TV in China.

## Tests

```sh
cd plugins/iptv && go test -race -v ./...
```
