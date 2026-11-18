package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"hyla/internal/build"
	"hyla/internal/client"
	"hyla/internal/plugins/manifest"
	"hyla/internal/plugins/rpc"
)

func pluginCmd(args []string) error {
	sub := "ls"
	if len(args) > 0 {
		sub = args[0]
	}
	rest := args[min(1, len(args)):]
	store := manifest.DefaultStore()

	switch sub {
	case "ls", "list":
		return pluginListCmd(store, rest)
	case "init":
		return pluginInitCmd(rest)
	case "pack":
		return pluginPackCmd(rest)
	case "install":
		return pluginInstallCmd(store, rest)
	case "link":
		return pluginLinkCmd(store, rest)
	case "dev":
		return pluginDevCmd(store, rest)
	case "enable":
		return pluginEnableCmd(store, rest, true)
	case "disable":
		return pluginEnableCmd(store, rest, false)
	case "rm", "remove":
		return pluginRemoveCmd(store, rest)
	case "permissions", "perms":
		return pluginPermissionsCmd(store, rest)
	case "logs":
		return pluginLogsCmd(store, rest)
	default:
		return usageErr("hyla plugin [ls|init|pack|install|link|dev|enable|disable|rm|permissions|logs]")
	}
}

func notifyDaemonPluginReload() {
	cl, err := client.Connect(true)
	if err != nil {
		return
	}
	defer cl.Close()
	if !slices.Contains(cl.Welcome.Capabilities, "plugins.v1") {
		return
	}
	_ = cl.Call("plugin.reload", nil, nil)
}

func notifyDaemonPluginRestart(pluginID string) {
	if pluginID == "" {
		return
	}
	cl, err := client.Connect(true)
	if err != nil {
		return
	}
	defer cl.Close()
	if !slices.Contains(cl.Welcome.Capabilities, "plugins.v1") {
		return
	}
	_ = cl.Call("plugin.restart", rpc.PluginControlParams{PluginID: pluginID}, nil)
}

func pluginListCmd(store *manifest.Store, args []string) error {
	f, err := parse(args, nil, map[string]string{"json": "json", "j": "json"})
	if err != nil {
		return err
	}
	plugins, err := store.Discover()
	if err != nil {
		return err
	}

	statusByID := map[string]rpc.PluginStatus{}
	if cl, cerr := client.Connect(true); cerr == nil {
		defer cl.Close()
		if slices.Contains(cl.Welcome.Capabilities, "plugins.v1") {
			var live []rpc.PluginStatus
			if cl.Call("plugin.list", nil, &live) == nil {
				for _, st := range live {
					statusByID[st.ID] = st
				}
			}
		}
	}

	if f.set["json"] {
		type item struct {
			manifest.InstalledPlugin
			Runtime *rpc.PluginStatus `json:"runtime,omitempty"`
		}
		out := make([]item, 0, len(plugins))
		for _, p := range plugins {
			it := item{InstalledPlugin: p}
			if st, ok := statusByID[p.Entry.ID]; ok {
				copySt := st
				it.Runtime = &copySt
			}
			out = append(out, it)
		}
		return printJSON(out)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tVERSION\tSOURCE\tENABLED\tSTATE")
	for _, p := range plugins {
		name := "-"
		if p.Manifest != nil && p.Manifest.Name != "" {
			name = p.Manifest.Name
		}
		ver := p.Entry.Version
		if ver == "" {
			ver = "-"
		}
		src := p.Entry.Source
		if p.Builtin {
			src = "builtin"
		}
		state := "stopped"
		if p.Entry.PendingInstall {
			state = "pending-install"
		} else if st, ok := statusByID[p.Entry.ID]; ok && st.State != "" {
			state = st.State
		}
		if p.LoadError != "" {
			state = "error"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\t%s\n", p.Entry.ID, name, ver, src, p.Entry.Enabled, state)
	}
	return w.Flush()
}

func pluginPackCmd(args []string) error {
	f, err := parse(args, map[string]string{"out": "out", "o": "out"}, nil)
	if err != nil {
		return err
	}
	if len(f.pos) != 1 {
		return usageErr("hyla plugin pack <dir> [-o out.hylapkg]")
	}
	res, err := manifest.Pack(f.pos[0], f.vals["out"])
	if err != nil {
		return err
	}
	fmt.Printf("packed %s (%d files, %d bytes)\nsha256: %s\n", res.ArchivePath, res.Files, res.Bytes, res.SHA256)
	return nil
}

func pluginInstallCmd(store *manifest.Store, args []string) error {
	f, err := parse(args, nil, map[string]string{
		"grant-all": "grantAll",
		"no-enable": "noEnable",
	})
	if err != nil {
		return err
	}
	if len(f.pos) != 1 {
		return usageErr("hyla plugin install <pkg.hylapkg|dir> [--grant-all] [--no-enable]")
	}
	src := f.pos[0]
	archive := src
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		tmpDir, err := os.MkdirTemp("", "hyla-plugin-install-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		res, err := manifest.Pack(src, filepath.Join(tmpDir, "pkg.hylapkg"))
		if err != nil {
			return err
		}
		archive = res.ArchivePath
	}
	inst, err := store.InstallPackage(archive, manifest.InstallOptions{
		Enable:         !f.set["noEnable"],
		GrantAll:       f.set["grantAll"],
		CurrentVersion: build.Version(),
	})
	if err != nil {
		return err
	}
	_ = syncExistingPluginAgentIntegrations(store)
	notifyDaemonPluginReload()

	fmt.Printf("installed %s v%s (enabled=%t)\n", inst.Entry.ID, inst.Entry.Version, inst.Entry.Enabled)
	if inst.Manifest != nil {
		var ungranted []string
		for _, sc := range inst.Manifest.Permissions.Scopes {
			if !inst.Entry.HasPermission(sc) {
				ungranted = append(ungranted, sc)
			}
		}
		if len(ungranted) > 0 {
			fmt.Printf("pending permissions (grant via `hyla plugin permissions %s grant <scope>`): %s\n",
				inst.Entry.ID, strings.Join(ungranted, ", "))
		}
	}
	return nil
}

func pluginLinkCmd(store *manifest.Store, args []string) error {
	f, err := parse(args, nil, map[string]string{"grant-all": "grantAll"})
	if err != nil {
		return err
	}
	if len(f.pos) != 1 {
		return usageErr("hyla plugin link <dir> [--grant-all]")
	}
	inst, err := store.Link(f.pos[0], f.set["grantAll"])
	if err != nil {
		return err
	}
	_ = syncExistingPluginAgentIntegrations(store)
	notifyDaemonPluginReload()
	fmt.Printf("linked %s -> %s\n", inst.Entry.ID, inst.RootDir)
	return nil
}

func pluginDevCmd(store *manifest.Store, args []string) error {
	f, err := parse(args, nil, map[string]string{"grant-all": "grantAll"})
	if err != nil {
		return err
	}
	if len(f.pos) != 1 {
		return usageErr("hyla plugin dev <dir> [--grant-all]")
	}
	dir, err := filepath.Abs(f.pos[0])
	if err != nil {
		return err
	}
	inst, err := store.Link(dir, f.set["grantAll"])
	if err != nil {
		return err
	}
	_ = syncExistingPluginAgentIntegrations(store)
	notifyDaemonPluginReload()
	fmt.Printf("watching %s (%s) — press Ctrl+C to stop\n", inst.Entry.ID, dir)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	lastSig := treeStamp(dir)
	ticker := time.NewTicker(600 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-sigCh:
			fmt.Println("\nstopped watching")
			return nil
		case <-ticker.C:
			cur := treeStamp(dir)
			if cur != lastSig {
				lastSig = cur
				fmt.Printf("[%s] change detected, reloading %s…\n", time.Now().Format("15:04:05"), inst.Entry.ID)
				if _, err := manifest.ApplyBase(store); err != nil {
					fmt.Fprintf(os.Stderr, "reload error: %v\n", err)
				}
				_ = syncExistingPluginAgentIntegrations(store)
				notifyDaemonPluginReload()
				if curInst, lerr := store.Link(dir, f.set["grantAll"]); lerr == nil {
					inst = curInst
				}
				if inst.Manifest != nil && inst.Manifest.Daemon != nil {
					notifyDaemonPluginRestart(inst.Entry.ID)
				}
			}
		}
	}
}

func treeStamp(root string) string {
	var parts []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() && (name == ".git" || name == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if info, serr := d.Info(); serr == nil {
			parts = append(parts, p+"|"+strconv.FormatInt(info.ModTime().UnixNano(), 10)+"|"+strconv.FormatInt(info.Size(), 10))
		}
		return nil
	})
	slices.Sort(parts)
	return strings.Join(parts, "\n")
}

func pluginEnableCmd(store *manifest.Store, args []string, enabled bool) error {
	if len(args) != 1 {
		if enabled {
			return usageErr("hyla plugin enable <id>")
		}
		return usageErr("hyla plugin disable <id>")
	}
	id := args[0]
	if err := store.Enable(id, enabled); err != nil {
		return err
	}
	_ = syncExistingPluginAgentIntegrations(store)
	notifyDaemonPluginReload()
	if enabled {
		fmt.Printf("enabled %s\n", id)
	} else {
		fmt.Printf("disabled %s\n", id)
	}
	return nil
}

func pluginRemoveCmd(store *manifest.Store, args []string) error {
	f, err := parse(args, nil, map[string]string{"purge": "purge"})
	if err != nil {
		return err
	}
	if len(f.pos) != 1 {
		return usageErr("hyla plugin rm <id> [--purge]")
	}
	id := f.pos[0]
	if err := store.Remove(id, f.set["purge"]); err != nil {
		return err
	}
	_ = syncExistingPluginAgentIntegrations(store)
	notifyDaemonPluginReload()
	fmt.Printf("removed %s\n", id)
	return nil
}

func pluginPermissionsCmd(store *manifest.Store, args []string) error {
	if len(args) < 1 {
		return usageErr("hyla plugin permissions <id> [grant|revoke <scope…>|--all]")
	}
	id := args[0]
	if err := manifest.ValidateID(id); err != nil {
		return err
	}
	plugins, err := store.Discover()
	if err != nil {
		return err
	}
	var target *manifest.InstalledPlugin
	for i := range plugins {
		if plugins[i].Entry.ID == id {
			target = &plugins[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("%w: plugin %q", errNotFound, id)
	}

	if len(args) == 1 {
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "SCOPE\tGRANTED\tRISK\tDESCRIPTION")
		var scopes []string
		if target.Manifest != nil {
			scopes = append(scopes, target.Manifest.Permissions.Scopes...)
		}
		for _, g := range target.Entry.GrantedPermissions {
			if !slices.Contains(scopes, g) {
				scopes = append(scopes, g)
			}
		}
		for _, sc := range scopes {
			risk := "standard"
			desc := ""
			if info, ok := manifest.DescribePermission(sc); ok {
				if info.HighRisk {
					risk = "high-risk"
				}
				desc = info.Description
			}
			fmt.Fprintf(w, "%s\t%t\t%s\t%s\n", sc, target.Entry.HasPermission(sc), risk, desc)
		}
		return w.Flush()
	}

	action := args[1]
	rest := args[2:]
	if action != "grant" && action != "revoke" {
		return usageErr("hyla plugin permissions <id> [grant|revoke <scope…>|--all]")
	}
	grant := action == "grant"
	f, err := parse(rest, nil, map[string]string{"all": "all"})
	if err != nil {
		return err
	}
	var scopes []string
	if f.set["all"] && target.Manifest != nil {
		scopes = append(scopes, target.Manifest.Permissions.Scopes...)
	} else {
		scopes = f.pos
	}
	if len(scopes) == 0 {
		return usageErr("specify at least one permission scope or --all")
	}
	for _, sc := range scopes {
		if err := store.SetPermission(id, sc, grant); err != nil {
			return err
		}
	}
	notifyDaemonPluginReload()
	fmt.Printf("updated permissions for %s\n", id)
	return nil
}

func pluginLogsCmd(store *manifest.Store, args []string) error {
	f, err := parse(args, map[string]string{"n": "lines", "lines": "lines"}, nil)
	if err != nil {
		return err
	}
	if len(f.pos) != 1 {
		return usageErr("hyla plugin logs <id> [-n N]")
	}
	id := f.pos[0]
	if err := manifest.ValidateID(id); err != nil {
		return err
	}
	lines := 100
	if raw := f.vals["lines"]; raw != "" {
		n, perr := strconv.Atoi(raw)
		if perr != nil || n <= 0 {
			return usageErr("-n must be a positive integer")
		}
		lines = n
	}

	if cl, cerr := client.Connect(true); cerr == nil {
		defer cl.Close()
		if slices.Contains(cl.Welcome.Capabilities, "plugins.v1") {
			var res rpc.PluginLogsResult
			if cl.Call("plugin.logs", rpc.PluginLogsParams{PluginID: id, Lines: lines}, &res) == nil {
				if res.Text != "" {
					fmt.Print(res.Text)
					if !strings.HasSuffix(res.Text, "\n") {
						fmt.Println()
					}
				}
				return nil
			}
		}
	}

	path, err := store.LogPath(id)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	all := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	fmt.Println(strings.Join(all, "\n"))
	return nil
}

func pluginInitCmd(args []string) error {
	f, err := parse(args, map[string]string{
		"template": "template",
		"t":        "template",
		"id":       "id",
		"name":     "name",
	}, nil)
	if err != nil {
		return err
	}
	dir := "."
	if len(f.pos) == 1 {
		dir = f.pos[0]
	} else if len(f.pos) > 1 {
		return usageErr("hyla plugin init [--template pack|go|ts-webview] [--id <id>] [--name <name>] [dir]")
	}
	tmpl := f.vals["template"]
	if tmpl == "" {
		tmpl = "pack"
	}
	if tmpl != "pack" && tmpl != "go" && tmpl != "ts-webview" {
		return usageErr("unknown template %q (want pack, go, or ts-webview)", tmpl)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(absDir, manifest.ManifestFileName)
	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("%s already exists", manifestPath)
	}
	id := f.vals["id"]
	if id == "" {
		slug := sanitizeSlug(filepath.Base(absDir))
		id = "com.example." + slug
	}
	if err := manifest.ValidateID(id); err != nil {
		return err
	}
	name := f.vals["name"]
	if name == "" {
		name = strings.ReplaceAll(filepath.Base(absDir), "-", " ")
		if name == "" || name == "." {
			name = "Sample Plugin"
		}
	}

	var files map[string]string
	switch tmpl {
	case "pack":
		files = scaffoldPackFiles(id, name)
	case "go":
		files = scaffoldGoFiles(id, name)
	case "ts-webview":
		files = scaffoldTSWebViewFiles(id, name)
	}

	for rel, content := range files {
		dst := filepath.Join(absDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("created %s plugin %s in %s\n", tmpl, id, absDir)
	return nil
}

func sanitizeSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			if b.Len() > 0 {
				b.WriteByte('-')
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) < 2 {
		return "myplugin"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func scaffoldPackFiles(id, name string) map[string]string {
	return map[string]string{
		manifest.ManifestFileName: fmt.Sprintf(`api_version = 1
id = %q
name = %q
version = "0.1.0"
description = "Declarative asset pack plugin for Hyla"
author = "You"

[[agents]]
id = "sample-agent"
name = "Sample Agent"
detect = "detect/agent.toml"
mark = "marks/agent.svg"
processes = ["sample-agent"]

[[themes]]
name = "Aurora Dusk"
file = "themes/aurora.ghostty"

[[washes]]
id = "aurora-mist"
name = "Aurora Mist"
file = "washes/aurora.toml"
`, id, name),
		"detect/agent.toml": `name = "sample-agent"

[[rules]]
state = "waiting"
wait_kind = "permission"
pattern = "(?i)allow|approve\\?"
`,
		"marks/agent.svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="5.5"/><path d="M5.5 8l1.8 1.8L10.8 6.2"/></svg>
`,
		"themes/aurora.ghostty": `background = #161822
foreground = #e2e8f0
cursor-color = #7dd3fc
`,
		"washes/aurora.toml": `id = "aurora-mist"
name = "Aurora Mist"
light = "#f4f7fb"
dark = "#141824"
light_glow = "#cfe5ff"
dark_glow = "#263b5e"
`,
		"README.md": fmt.Sprintf("# %s (%s)\n\nLink for local development:\n\n```sh\nhyla plugin link .\n```\n", name, id),
	}
}

func scaffoldGoFiles(id, name string) map[string]string {
	return map[string]string{
		manifest.ManifestFileName: fmt.Sprintf(`api_version = 1
id = %q
name = %q
version = "0.1.0"
description = "Go out-of-process Hyla plugin"
author = "You"

[permissions]
scopes = ["ui.slot", "session.read"]

[daemon]
command = ["./bin/plugin"]
transport = "stdio"
auto_start = false

[[actions]]
id = "refresh"
title = "Refresh %s"
palette = true

[[ui.slots]]
id = "card"
mount = "info.card"
title = %q
mode = "declarative"
host = "daemon"
`, id, name, name, name),
		"go.mod": "module " + strings.ReplaceAll(id, ".", "/") + "\n\ngo 1.24\n",
		"main.go": `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type rpcReq struct {
	JSONRPC string          ` + "`json:\"jsonrpc\"`" + `
	ID      json.RawMessage ` + "`json:\"id,omitempty\"`" + `
	Method  string          ` + "`json:\"method\"`" + `
	Params  json.RawMessage ` + "`json:\"params,omitempty\"`" + `
}

type rpcResp struct {
	JSONRPC string          ` + "`json:\"jsonrpc\"`" + `
	ID      json.RawMessage ` + "`json:\"id,omitempty\"`" + `
	Result  any             ` + "`json:\"result,omitempty\"`" + `
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	enc := json.NewEncoder(os.Stdout)
	clicks := 0
	for sc.Scan() {
		var req rpcReq
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		var res any
		switch req.Method {
		case "initialize":
			res = map[string]any{"name": "go-plugin", "version": "0.1.0"}
		case "ui.render", "ui.event":
			if req.Method == "ui.event" {
				clicks++
			}
			res = map[string]any{
				"root": map[string]any{
					"type": "column",
					"children": []any{
						map[string]any{"type": "text", "props": map[string]any{"text": fmt.Sprintf("Clicks: %d", clicks)}},
						map[string]any{"type": "button", "id": "inc", "onClick": "inc", "props": map[string]any{"label": "Click Me"}},
					},
				},
			}
		case "action.invoke":
			res = map[string]any{"ok": true, "message": "Refreshed"}
		case "shutdown":
			if len(req.ID) > 0 {
				_ = enc.Encode(rpcResp{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"ok": true}})
			}
			return
		}
		if len(req.ID) > 0 {
			_ = enc.Encode(rpcResp{JSONRPC: "2.0", ID: req.ID, Result: res})
		}
	}
}
`,
		"Makefile":  "build:\n\tgo build -o bin/plugin .\n",
		".gitignore": "bin/\n*.hylapkg\n",
		"README.md": fmt.Sprintf("# %s\n\n```sh\nmake build\nhyla plugin link . --grant-all\n```\n", name),
	}
}

func scaffoldTSWebViewFiles(id, name string) map[string]string {
	return map[string]string{
		manifest.ManifestFileName: fmt.Sprintf(`api_version = 1
id = %q
name = %q
version = "0.1.0"
description = "TypeScript + Sandboxed WebView Hyla plugin"
author = "You"

[permissions]
scopes = ["ui.webview", "session.read"]

[app]
command = ["bun", "run", "src/plugin.ts"]
transport = "stdio"

[[actions]]
id = "open"
title = "Open %s"
menu = "Plugins"
palette = true
target = "open_pane"
view = "main"

[[ui.slots]]
id = "main"
mount = "pane"
title = %q
mode = "webview"
web_root = "web"
entry = "index.html"
host = "app"
`, id, name, name, name),
		"package.json": fmt.Sprintf(`{
  "name": %q,
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "start": "bun run src/plugin.ts"
  }
}
`, id),
		"src/plugin.ts": `const dec = new TextDecoder();
let buf = "";

for await (const chunk of Bun.stdin.stream()) {
  buf += dec.decode(chunk, { stream: true });
  while (true) {
    const idx = buf.indexOf("\n");
    if (idx < 0) break;
    const line = buf.slice(0, idx).trim();
    buf = buf.slice(idx + 1);
    if (!line) continue;
    const msg = JSON.parse(line);
    if (msg.id === undefined) continue;
    let result: unknown = {};
    if (msg.method === "initialize") {
      result = { name: "ts-webview", version: "0.1.0" };
    } else if (msg.method === "webview.rpc") {
      result = { data: { echo: msg.params?.method, time: new Date().toISOString() } };
    } else if (msg.method === "shutdown") {
      process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id: msg.id, result: { ok: true } }) + "\n");
      process.exit(0);
    }
    process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id: msg.id, result }) + "\n");
  }
}
`,
		"web/index.html": fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>%s</title>
    <link rel="stylesheet" href="/_hyla/theme.css" />
    <script src="/_hyla/bridge.js"></script>
    <style>
      body { font-family: system-ui, sans-serif; margin: 16px; color: var(--hyla-text); background: transparent; }
      button { padding: 6px 12px; border-radius: 6px; border: 1px solid var(--hyla-divider); background: var(--hyla-control); color: var(--hyla-text); cursor: pointer; }
      pre { margin-top: 12px; font-size: 12px; color: var(--hyla-muted); }
    </style>
  </head>
  <body>
    <h3>%s</h3>
    <button id="ping">Ping Plugin Host</button>
    <pre id="out">Ready</pre>
    <script>
      document.getElementById("ping").addEventListener("click", async () => {
        const res = await window.hyla.rpc("ping", { hello: "hyla" });
        document.getElementById("out").textContent = JSON.stringify(res, null, 2);
      });
    </script>
  </body>
</html>
`, name, name),
		"README.md": fmt.Sprintf("# %s\n\n```sh\nhyla plugin link . --grant-all\n```\n", name),
	}
}
