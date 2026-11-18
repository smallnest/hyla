package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"hyla/internal/plugins/manifest"
	"hyla/internal/proto"
)

func runPluginHook(args []string) error {
	if len(args) < 3 {
		return nil
	}
	pluginID, agent, event := args[0], args[1], args[2]
	if manifest.ValidateID(pluginID) != nil {
		return nil
	}
	store := manifest.DefaultStore()
	plugins, err := store.Discover()
	if err != nil {
		return nil
	}
	var target *manifest.InstalledPlugin
	for i := range plugins {
		if plugins[i].Entry.ID == pluginID {
			target = &plugins[i]
			break
		}
	}
	if target == nil || !target.Entry.Enabled || target.Manifest == nil || target.RootDir == "" {
		return nil
	}

	var matched []string
	for _, hk := range target.Manifest.Hooks {
		if strings.EqualFold(hk.Agent, agent) && hk.Event == event {
			resolved, err := manifest.ResolveWithin(target.RootDir, hk.Script)
			if err == nil {
				matched = append(matched, resolved)
			}
		}
	}
	if len(matched) == 0 {
		return nil
	}

	var stdinData []byte
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		stdinData, _ = io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	}

	for _, scriptPath := range matched {
		if err := execSinglePluginHook(target.Entry.ID, target.RootDir, agent, event, scriptPath, stdinData); err != nil {
			// Hook failures should never crash the user's coding agent session,
			// though non-zero debug info can go to plugin log if desired.
			_ = err
		}
	}
	return nil
}

func execSinglePluginHook(pluginID, rootDir, agent, event, scriptPath string, stdinData []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if strings.EqualFold(filepath.Ext(scriptPath), ".sh") && runtime.GOOS != "windows" {
		raw, err := os.ReadFile(scriptPath)
		if err != nil {
			return err
		}
		expanded := strings.ReplaceAll(string(raw), "__HYLA__", shellQuote(hyla()))
		if string(raw) != expanded {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", expanded)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", scriptPath)
		}
	} else {
		cmd = exec.CommandContext(ctx, scriptPath)
	}
	cmd.Dir = rootDir
	cmd.Stdin = bytes.NewReader(stdinData)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"HYLA_BIN="+hyla(),
		"HYLA_PLUGIN_ID="+pluginID,
		"HYLA_PLUGIN_DIR="+rootDir,
		"HYLA_AGENT="+agent,
		"HYLA_HOOK_EVENT="+event,
	)
	return cmd.Run()
}

type syncedPluginAgentAssets struct {
	SkillDirs []string `json:"skillDirs,omitempty"`
	HookFiles []string `json:"hookFiles,omitempty"`
}

func pluginAgentAssetsStatePath(store *manifest.Store) string {
	if store != nil && store.BaseDir != "" {
		return filepath.Join(store.BaseDir, "plugin-agent-assets.json")
	}
	return filepath.Join(proto.Dir(), "plugin-agent-assets.json")
}

func loadSyncedPluginAgentAssets(store *manifest.Store) syncedPluginAgentAssets {
	data, err := os.ReadFile(pluginAgentAssetsStatePath(store))
	if err != nil {
		return syncedPluginAgentAssets{}
	}
	var st syncedPluginAgentAssets
	_ = json.Unmarshal(data, &st)
	return st
}

func saveSyncedPluginAgentAssets(store *manifest.Store, st syncedPluginAgentAssets) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(pluginAgentAssetsStatePath(store), append(data, '\n'))
}

// SyncPluginAgentIntegrations reconciles plugin hooks and skills for agents
// where Hyla's base skill or hook integration is already installed.
func SyncPluginAgentIntegrations(store *manifest.Store, snap *manifest.Snapshot) error {
	_, err := syncExistingPluginAgentIntegrationsWithSnap(store, snap)
	return err
}

func syncExistingPluginAgentIntegrations(store *manifest.Store) error {
	_, err := syncExistingPluginAgentIntegrationsWithSnap(store, nil)
	return err
}

func syncExistingPluginAgentIntegrationsWithSnap(store *manifest.Store, snap *manifest.Snapshot) (*manifest.Snapshot, error) {
	if store == nil {
		store = manifest.DefaultStore()
	}
	claudeInstalled, agentsInstalled := false, false
	for _, s := range SkillStates() {
		if s.State == "missing" {
			continue
		}
		if s.Agent == "claude" {
			claudeInstalled = true
		} else if s.Agent == "agents" {
			agentsInstalled = true
		}
	}
	prev := loadSyncedPluginAgentAssets(store)
	if !claudeInstalled && !agentsInstalled {
		if len(prev.SkillDirs) == 0 && len(prev.HookFiles) == 0 {
			return snap, nil
		}
		return applyPluginAgentIntegrationsWithTargets(store, snap, "", false, map[string]bool{})
	}
	targets := map[string]bool{
		"claude": claudeInstalled,
		"agents": agentsInstalled,
	}
	return applyPluginAgentIntegrationsWithTargets(store, snap, "", true, targets)
}

func applyPluginAgentIntegrations(store *manifest.Store, agentFilter string, install bool) error {
	_, err := applyPluginAgentIntegrationsWithTargets(store, nil, agentFilter, install, nil)
	return err
}

func applyPluginAgentIntegrationsWithTargets(store *manifest.Store, snap *manifest.Snapshot, agentFilter string, install bool, targets map[string]bool) (*manifest.Snapshot, error) {
	if store == nil {
		store = manifest.DefaultStore()
	}
	if snap == nil {
		var err error
		snap, err = manifest.ApplyBase(store)
		if err != nil {
			return nil, err
		}
	}

	wantAgent := func(agent string) bool {
		if !matchesAgentFilter(agent, agentFilter) {
			return false
		}
		if targets != nil {
			switch strings.ToLower(strings.TrimSpace(agent)) {
			case "claude":
				return targets["claude"]
			case "codex", "opencode", "pi", "agents":
				return targets["agents"]
			default:
				return false
			}
		}
		return true
	}

	prev := loadSyncedPluginAgentAssets(store)
	next := syncedPluginAgentAssets{}

	// 1. Reconcile plugin skills cleanly without touching Hyla's built-in skill.
	desiredSkills := map[string]manifest.SkillAsset{}
	if install {
		for _, sk := range snap.Skills {
			if !wantAgent(sk.Agent) {
				continue
			}
			targetDir, ok := pluginSkillTargetDir(sk.Agent, sk.Name)
			if !ok {
				continue
			}
			desiredSkills[targetDir] = sk
		}
	}

	for _, oldDir := range prev.SkillDirs {
		if _, keep := desiredSkills[oldDir]; !keep {
			if agentFilter == "" || skillDirMatchesFilter(oldDir, agentFilter) {
				_ = os.RemoveAll(oldDir)
			} else {
				next.SkillDirs = append(next.SkillDirs, oldDir)
			}
		}
	}

	for targetDir, sk := range desiredSkills {
		if err := syncPluginSkillDir(sk.Dir, targetDir); err != nil {
			return snap, err
		}
		next.SkillDirs = append(next.SkillDirs, targetDir)
	}
	slices.Sort(next.SkillDirs)

	// 2. Reconcile plugin hooks in JSON settings (claude / codex) and extension dirs (pi / opencode).
	home := proto.Home()
	dirOf := func(env string, def ...string) string {
		if d := os.Getenv(env); d != "" {
			return d
		}
		return filepath.Join(append([]string{home}, def...)...)
	}
	claudeDir := dirOf("CLAUDE_CONFIG_DIR", ".claude")
	codexDir := dirOf("CODEX_HOME", ".codex")
	piDir := dirOf("PI_CODING_AGENT_DIR", ".pi", "agent")
	opencodeDir := filepath.Join(dirOf("XDG_CONFIG_HOME", ".config"), "opencode")

	if agentFilter == "" || agentFilter == "claude" {
		claudeInstall := install && wantAgent("claude")
		hooks := collectHooks(snap.Hooks, "claude", claudeInstall)
		if err := reconcileJSONPluginHooks(filepath.Join(claudeDir, "settings.json"), "claude", hooks); err != nil {
			return snap, err
		}
	}
	if agentFilter == "" || agentFilter == "codex" || agentFilter == "agents" {
		codexInstall := install && wantAgent("codex")
		hooks := collectHooks(snap.Hooks, "codex", codexInstall)
		if err := reconcileJSONPluginHooks(filepath.Join(codexDir, "hooks.json"), "codex", hooks); err != nil {
			return snap, err
		}
	}

	// Pi & OpenCode file-based hook scripts
	desiredFiles := map[string][]byte{}
	if install {
		for _, hk := range snap.Hooks {
			if !wantAgent(hk.Agent) {
				continue
			}
			if hk.Agent != "pi" && hk.Agent != "opencode" {
				continue
			}
			raw, err := os.ReadFile(hk.Script)
			if err != nil {
				continue
			}
			rendered := expandPluginTemplateBytes(raw)
			var dst string
			if hk.Agent == "pi" {
				dst = filepath.Join(piDir, "extensions", fmt.Sprintf("hyla-plugin-%s-%s", hk.PluginID, filepath.Base(hk.Script)))
			} else {
				dst = filepath.Join(opencodeDir, "plugins", "hyla-plugin-"+hk.PluginID, filepath.Base(hk.Script))
			}
			desiredFiles[dst] = rendered
		}
	}
	for _, oldFile := range prev.HookFiles {
		if _, keep := desiredFiles[oldFile]; !keep {
			if agentFilter == "" || hookFileMatchesFilter(oldFile, agentFilter, piDir, opencodeDir) {
				_ = os.Remove(oldFile)
				_ = os.Remove(filepath.Dir(oldFile))
			} else {
				next.HookFiles = append(next.HookFiles, oldFile)
			}
		}
	}
	for dst, data := range desiredFiles {
		if cur, rerr := os.ReadFile(dst); rerr != nil || !bytes.Equal(cur, data) {
			if err := writeFileAtomic(dst, data); err != nil {
				return snap, err
			}
		}
		next.HookFiles = append(next.HookFiles, dst)
	}
	slices.Sort(next.HookFiles)

	if slices.Equal(prev.SkillDirs, next.SkillDirs) && slices.Equal(prev.HookFiles, next.HookFiles) {
		return snap, nil
	}
	return snap, saveSyncedPluginAgentAssets(store, next)
}

func collectHooks(all []manifest.HookAsset, agent string, install bool) []manifest.HookAsset {
	if !install {
		return nil
	}
	var out []manifest.HookAsset
	for _, h := range all {
		if h.Agent == agent {
			out = append(out, h)
		}
	}
	return out
}

func matchesAgentFilter(assetAgent, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	assetAgent = strings.ToLower(strings.TrimSpace(assetAgent))
	if filter == "" {
		return true
	}
	if filter == assetAgent {
		return true
	}
	if filter == "agents" && (assetAgent == "codex" || assetAgent == "opencode" || assetAgent == "pi" || assetAgent == "agents") {
		return true
	}
	return false
}

func pluginSkillTargetDir(agent, name string) (string, bool) {
	if name == "" || name == "hyla" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", false
	}
	home := proto.Home()
	claude := os.Getenv("CLAUDE_CONFIG_DIR")
	if claude == "" {
		claude = filepath.Join(home, ".claude")
	}
	switch strings.ToLower(strings.TrimSpace(agent)) {
	case "claude":
		return filepath.Join(claude, "skills", name), true
	case "codex", "opencode", "pi", "agents":
		return filepath.Join(home, ".agents", "skills", name), true
	default:
		return "", false
	}
}

func skillDirMatchesFilter(dir, filter string) bool {
	claudeRoot := filepath.Join(proto.Home(), ".claude", "skills")
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		claudeRoot = filepath.Join(d, "skills")
	}
	if filter == "claude" {
		return strings.HasPrefix(dir, claudeRoot+string(filepath.Separator))
	}
	if filter == "agents" || filter == "codex" || filter == "opencode" || filter == "pi" {
		agentsRoot := filepath.Join(proto.Home(), ".agents", "skills")
		return strings.HasPrefix(dir, agentsRoot+string(filepath.Separator))
	}
	return false
}

func hookFileMatchesFilter(file, filter, piDir, opencodeDir string) bool {
	sep := string(filepath.Separator)
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "pi":
		return strings.HasPrefix(file, filepath.Join(piDir, "extensions")+sep)
	case "opencode":
		return strings.HasPrefix(file, filepath.Join(opencodeDir, "plugins")+sep)
	case "agents", "codex":
		return strings.HasPrefix(file, filepath.Join(piDir, "extensions")+sep) ||
			strings.HasPrefix(file, filepath.Join(opencodeDir, "plugins")+sep)
	default:
		return false
	}
}

func syncPluginSkillDir(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	expected := map[string]bool{}
	err := filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlink forbidden in plugin skill: %s", p)
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dst := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if isTemplateTextFile(p) {
			data = expandPluginTemplateBytes(data)
		}
		expected[dst] = true
		if cur, err := os.ReadFile(dst); err == nil && bytes.Equal(cur, data) {
			return nil
		}
		return writeFileAtomic(dst, data)
	})
	if err != nil {
		return err
	}
	// Remove orphaned files inside dstDir left by previous versions of this skill.
	_ = filepath.WalkDir(dstDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if !expected[p] {
			_ = os.Remove(p)
		}
		return nil
	})
	return nil
}

func isTemplateTextFile(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".txt", ".json", ".sh", ".js", ".ts", ".toml", ".yaml", ".yml":
		return true
	}
	return false
}

func expandPluginTemplateBytes(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("__HYLA__"), []byte(hyla()))
}

func shellQuote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func pluginHookCommand(pluginID, agent, event string) string {
	return shellQuote(hyla()) + fmt.Sprintf(" hook plugin %s %s %s", pluginID, agent, event)
}

func isPluginHookCommand(cmd string) bool {
	return strings.Contains(cmd, " hook plugin ")
}

func reconcileJSONPluginHooks(file, agent string, desired []manifest.HookAsset) error {
	b, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && len(desired) == 0 {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	settings := object{}
	if len(bytes.TrimSpace(b)) > 0 {
		if settings, err = parseObject(b); err != nil {
			return fmt.Errorf("%s: %w", proto.ShortDir(file), err)
		}
	}
	hooksObj, err := parseObject(settings.get("hooks"))
	if err != nil {
		return fmt.Errorf("%s: hooks: %w", proto.ShortDir(file), err)
	}

	byEvent := map[string][]string{}
	for _, h := range desired {
		byEvent[h.Event] = append(byEvent[h.Event], pluginHookCommand(h.PluginID, agent, h.Event))
	}

	// Collect all events currently present in hooksObj plus desired events.
	seenEvents := map[string]bool{}
	var allEvents []string
	for _, m := range hooksObj {
		seenEvents[m.Key] = true
		allEvents = append(allEvents, m.Key)
	}
	for ev := range byEvent {
		if !seenEvents[ev] {
			seenEvents[ev] = true
			allEvents = append(allEvents, ev)
		}
	}
	slices.Sort(allEvents)

	changed := false
	for _, ev := range allEvents {
		var groups []object
		if raw := hooksObj.get(ev); raw != nil {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return fmt.Errorf("%s: hooks.%s: %w", proto.ShortDir(file), ev, err)
			}
		}
		var kept []object
		existingCmds := []string{}
		for _, g := range groups {
			var handlers []json.RawMessage
			_ = json.Unmarshal(g.get("hooks"), &handlers)
			n := len(handlers)
			var filtered []json.RawMessage
			for _, raw := range handlers {
				var hd struct{ Type, Command string }
				_ = json.Unmarshal(raw, &hd)
				if hd.Type == "command" && isPluginHookCommand(hd.Command) {
					existingCmds = append(existingCmds, hd.Command)
					continue
				}
				filtered = append(filtered, raw)
			}
			if n > 0 && len(filtered) == 0 {
				continue
			}
			if len(filtered) != n {
				g = g.set("hooks", marshal(filtered))
			}
			kept = append(kept, g)
		}

		wantCmds := byEvent[ev]
		slices.Sort(wantCmds)
		slices.Sort(existingCmds)
		if slices.Equal(wantCmds, existingCmds) {
			continue
		}
		changed = true
		for _, cmd := range wantCmds {
			handler := object{{"type", marshal("command")}, {"command", marshal(cmd)}, {"timeout", marshal(10)}}
			kept = append(kept, object{{"hooks", marshal([]object{handler})}})
		}
		if len(kept) > 0 {
			hooksObj = hooksObj.set(ev, marshal(kept))
		} else {
			hooksObj = hooksObj.del(ev)
		}
	}
	if !changed {
		return nil
	}
	if len(hooksObj) > 0 {
		settings = settings.set("hooks", hooksObj.marshal())
	} else {
		settings = settings.del("hooks")
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, settings.marshal(), "", "  "); err != nil {
		return err
	}
	pretty.WriteByte('\n')
	if bytes.Equal(pretty.Bytes(), b) {
		return nil
	}
	return writeFileAtomic(file, pretty.Bytes())
}
