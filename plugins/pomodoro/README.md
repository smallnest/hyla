# Hyla Pomodoro Plugin (`hyla.pomodoro`)

A refined, translucent glass Pomodoro focus studio pane for Hyla, built as an out-of-process WebView plugin.

## Features

- **Precision Radial Watch Dial**: Dual-layer SVG dial with 60 radial ticks, smooth gradient arc, cycle rhythm dots, and live `tabular-nums` countdown synced across light and dark Hyla themes.
- **Mood-Adaptive Canvas**: Smoothly shifts accent palette across **Pomodoro** (coral terracotta), **Short Break** (eucalyptus teal), and **Long Break** (twilight cobalt).
- **Task Queue & Finish-Time Estimator**: Add tasks with estimated pomodoros and notes, reorder or mark tasks done, automatically track `done / est` progress on completion, and see the dynamically calculated `Finish At` time.
- **7-Day Focus Analytics (`Report` tab)**: Today's focus duration, daily goal progress bar, consecutive day streak, inline SVG 7-day bar chart, and recent session history.
- **100% Local Web Audio Engine**: Synthesized completion chimes (*Bell*, *Singing Bowl*, *Marimba*, *Digital*) and ambient focus soundscapes (*Soft Tick*, *Warm Rain*, *Brown Noise*) with zero external network requests.
- **Desktop Notifications & Sync**: Fires Hyla notifications when a focus session or break completes, and persists settings cleanly with **Settings › Plugins**.

## Keyboard Shortcuts

- `Space`: Start / Pause timer
- `R`: Reset current phase
- `N`: Skip to next phase
- `1` / `2` / `3`: Switch to Pomodoro / Short Break / Long Break
- `M`: Cycle ambient focus sound
- `,`: Open Pomodoro Settings
- `Escape`: Close Settings modal or task composer

## Local Development

```sh
(cd plugins/pomodoro && go build -o bin/hyla-pomodoro .)
hyla plugin link /Users/smallnest/ai/hyla/plugins/pomodoro --grant-all
```

Once enabled, open via **Pane › Open Pomodoro Pane** (`⌥⌘O` / `Ctrl+Alt+O`) or the Command Palette.

## Tests

```sh
cd plugins/pomodoro && go test -race -v ./...
```
