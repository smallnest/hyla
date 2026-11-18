package feishu

import (
	"os"
	"slices"
	"strings"

	"hyla/internal/plugins/manifest"
	"hyla/internal/settings"
)

// pluginID is the id Hyla 3.17.0 bundled Feishu under, while it was a
// plugin. Feishu is a feature of Hyla's own again: what its plugin page
// kept of the bot belongs to the configuration, which is what reads it.
const pluginID = "hyla.feishu"

// MigratePluginConfig moves what the hyla.feishu plugin kept of the bot,
// its settings and its App Secret, into the configuration Feishu reads
// (the feishu-* keys, and feishu.json for the secret), then removes the
// plugin: its entry, its config and its secret, which nothing reads any
// more. It is what an upgrade from 3.17.0 runs, and it does nothing once
// it has.
func MigratePluginConfig(st *manifest.Store) error {
	if st == nil {
		st = manifest.DefaultStore()
	}
	// Fast-path guard: skip Keychain GetSecret IPC and RemoveAll once migrated or absent.
	reg, regErr := st.LoadRegistry()
	var hasReg bool
	if regErr == nil && reg != nil {
		_, hasReg = reg.Plugins[pluginID]
	}
	cfgPath, cfgPathErr := st.ConfigPath(pluginID)
	_, cfgStatErr := os.Stat(cfgPath)
	secPath, secPathErr := st.SecretsPath(pluginID)
	_, secStatErr := os.Stat(secPath)
	hasCfg := cfgPathErr == nil && cfgStatErr == nil
	hasSecFile := secPathErr == nil && secStatErr == nil
	if !hasReg && !hasCfg && !hasSecFile {
		return nil
	}

	cfg, err := st.LoadConfig(pluginID)
	if err != nil {
		cfg = nil
	}
	secret, hasSecret, err := st.GetSecret(pluginID, "appSecret")
	if err != nil {
		secret, hasSecret = "", false
	}
	if err := migrateSettings(cfg, secret, hasSecret); err != nil {
		// The plugin keeps what it has, for a later start to try again.
		return err
	}
	keepPluginConfig(st)
	_ = st.DeleteSecret(pluginID, "appSecret")
	return st.Remove(pluginID, true)
}

// keepPluginConfig writes the plugin's config beside itself, as
// hyla.feishu.json.migrated, before the migration removes it: what the
// upgrade read is then still there to look at.
func keepPluginConfig(st *manifest.Store) {
	p, err := st.ConfigPath(pluginID)
	if err != nil {
		return
	}
	if _, err := os.Stat(p + ".migrated"); err == nil {
		return
	}
	if b, err := os.ReadFile(p); err == nil {
		_ = os.WriteFile(p+".migrated", b, 0o644)
	}
}

// migrateSettings applies what the plugin kept to the configuration: only
// the values it has, as an empty field was never the plugin's to say, and
// in the order the plugin's own reads had, where its config was the last
// word.
func migrateSettings(cfg map[string]any, secret string, hasSecret bool) error {
	cur := settings.Feishu{}
	if f := settings.Load().Feishu; f != nil {
		cur = *f
		cur.Allow = slices.Clone(f.Allow)
		cur.Host = slices.Clone(f.Host)
	}
	changed := false
	if app := strings.TrimSpace(text(cfg["app"])); app != "" && app != cur.App {
		cur.App = app
		changed = true
	}
	if raw, ok := cfg["allow"]; ok {
		if allow := parseAllow(raw); len(allow) > 0 && !slices.Equal(allow, cur.Allow) {
			cur.Allow = allow
			changed = true
		}
	}
	if agent := strings.TrimSpace(text(cfg["agent"])); agent != "" && agent != cur.Agent {
		cur.Agent = agent
		changed = true
	}
	if run := strings.TrimSpace(text(cfg["run"])); (run == settings.FeishuRunCommand || run == settings.FeishuRunSession) && run != cur.Run {
		cur.Run = run
		changed = true
	}
	if dir := strings.TrimSpace(text(cfg["dir"])); dir != "" && dir != cur.Dir {
		cur.Dir = dir
		changed = true
	}
	if changed {
		if err := settings.SetFeishu(cur); err != nil {
			return err
		}
	}

	secret = strings.TrimSpace(secret)
	if !hasSecret || secret == "" || cur.App == "" {
		return nil
	}
	kept, err := settings.LoadFeishuSecret()
	if err == nil && kept.AppSecret == secret && kept.AppID == cur.App {
		return nil
	}
	if err != nil {
		kept = settings.FeishuSecret{}
	}
	// The bot's own identity stays: the plugin never kept it.
	kept.AppID, kept.AppSecret = cur.App, secret
	return settings.SaveFeishuSecret(kept)
}

// text is a value of the plugin's config as a string.
func text(v any) string {
	s, _ := v.(string)
	return s
}

// parseAllow reads an allow value of the plugin's config: the comma
// separated string its page wrote, or a list.
func parseAllow(v any) []string {
	var out []string
	switch x := v.(type) {
	case string:
		for _, part := range strings.Split(x, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	case []string:
		out = slices.Clone(x)
	case []any:
		for _, item := range x {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
}
