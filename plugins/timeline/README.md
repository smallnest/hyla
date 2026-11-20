# Timeline Plugin (`hyla.timeline`)

Standalone sandboxed WebView pane plugin for Hyla that displays a coding agent's turn-by-turn timeline (`Think`, `Text`, `Tool`, `Subagent`, `Waiting`, `Failed`), adaptive live polling (`3s` while running / `30s` when idle), agent/pane switching, and expandable per-step details.

## Build & Link for Development

```sh
cd plugins/timeline
mkdir -p bin
go build -o bin/hyla-timeline .
cd ../..
hyla plugin link ./plugins/timeline --grant-all
```

## Package & Install (`.hylapkg`)

```sh
cd plugins/timeline
mkdir -p bin
go build -o bin/hyla-timeline .
cd ../..
hyla plugin pack ./plugins/timeline -o hyla-timeline.hylapkg
hyla plugin install ./hyla-timeline.hylapkg --grant-all
```

Once enabled, open the pane via **Pane › Open Timeline Pane** (`⌥⌘T` on macOS, `Ctrl+Alt+T` on Windows/Linux) or from the Command Palette (`⌘P` / `Ctrl+Shift+P`).
