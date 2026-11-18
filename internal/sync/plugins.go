package sync

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"hyla/internal/plugins/manifest"
	"hyla/internal/settings"
)

// syncPluginStore picks the plugin storage root for sync operations: when a
// test overrides HYLA_CONFIG without HYLA_HOME, keep plugin files alongside
// that isolated config dir rather than touching the user's real ~/.hyla.
func syncPluginStore() *manifest.Store {
	if os.Getenv("HYLA_HOME") == "" && os.Getenv("HYLA_CONFIG") != "" {
		return &manifest.Store{BaseDir: filepath.Dir(settings.Path())}
	}
	return manifest.DefaultStore()
}

// collectPluginState gathers plugins.json and non-secret plugin-config/<id>.json
// entries suitable for syncing across machines. Machine-local LinkPath values
// are stripped, PendingInstall entries preserve their synced desire, and
// plugin-secrets/ is never read or included.
func collectPluginState(st *manifest.Store) (*manifest.Registry, map[string]json.RawMessage, error) {
	if st == nil {
		st = syncPluginStore()
	}

	var outReg *manifest.Registry
	if _, err := os.Stat(st.RegistryPath()); err == nil {
		reg, lerr := st.LoadRegistry()
		if lerr != nil {
			return nil, nil, lerr
		}
		if len(reg.Plugins) > 0 {
			ver := reg.Version
			if ver == 0 {
				ver = 1
			}
			sanitized := &manifest.Registry{
				Version: ver,
				Plugins: make(map[string]manifest.RegistryEntry, len(reg.Plugins)),
			}
			for id, e := range reg.Plugins {
				if id == "hyla.feishu" || manifest.ValidateID(id) != nil {
					continue
				}
				e.LinkPath = ""
				if e.Source == "link" {
					if manifest.IsBuiltin(id) {
						e.Source = "builtin"
					} else {
						e.Source = "package"
					}
				}
				if e.PendingInstall {
					e.PendingInstall = false
					if e.SyncedEnabled != nil {
						e.Enabled = *e.SyncedEnabled
					}
				}
				e.SyncedEnabled = nil
				if len(e.GrantedPermissions) > 0 {
					perms := slices.Clone(e.GrantedPermissions)
					slices.Sort(perms)
					e.GrantedPermissions = perms
				}
				sanitized.Plugins[id] = e
			}
			if len(sanitized.Plugins) > 0 {
				outReg = sanitized
			}
		}
	}

	// Collect ONLY non-secret plugin-config/<id>.json files; plugin-secrets/
	// is strictly excluded from snapshots.
	var cfgMap map[string]json.RawMessage
	entries, err := os.ReadDir(st.ConfigDir())
	if err == nil {
		for _, de := range entries {
			if de.IsDir() || de.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := de.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			id := strings.TrimSuffix(name, ".json")
			if id == "hyla.feishu" || manifest.ValidateID(id) != nil {
				continue
			}
			raw, rerr := os.ReadFile(filepath.Join(st.ConfigDir(), name))
			if rerr != nil || !json.Valid(raw) {
				continue
			}
			var buf bytes.Buffer
			if cerr := json.Compact(&buf, raw); cerr != nil {
				continue
			}
			if cfgMap == nil {
				cfgMap = map[string]json.RawMessage{}
			}
			cfgMap[id] = json.RawMessage(buf.Bytes())
		}
	}

	return outReg, cfgMap, nil
}

func computeStateHash(local []byte, reg *manifest.Registry, cfgs map[string]json.RawMessage) string {
	if reg == nil && len(cfgs) == 0 {
		return hashOf(local)
	}
	payload := struct {
		Config       string                     `json:"config"`
		Plugins      *manifest.Registry         `json:"plugins,omitempty"`
		PluginConfig map[string]json.RawMessage `json:"pluginConfig,omitempty"`
	}{
		Config:       string(local),
		Plugins:      reg,
		PluginConfig: cfgs,
	}
	b, _ := json.Marshal(payload)
	return hashOf(b)
}

func applyRemotePlugins(st *manifest.Store, remoteReg *manifest.Registry, remoteCfgs map[string]json.RawMessage, pluginsSynced bool) error {
	if st == nil {
		st = syncPluginStore()
	}
	hasPluginState := pluginsSynced || remoteReg != nil || len(remoteCfgs) > 0
	if !hasPluginState {
		return nil
	}

	if remoteReg != nil || pluginsSynced {
		localReg, err := st.LoadRegistry()
		if err != nil {
			return err
		}
		ver := 1
		remoteCount := 0
		if remoteReg != nil {
			if remoteReg.Version != 0 {
				ver = remoteReg.Version
			}
			remoteCount = len(remoteReg.Plugins)
		}
		merged := &manifest.Registry{
			Version: ver,
			Plugins: make(map[string]manifest.RegistryEntry, remoteCount),
		}
		if remoteReg != nil {
			for id, rEntry := range remoteReg.Plugins {
				if id == "hyla.feishu" || manifest.ValidateID(id) != nil {
					continue
				}
				rEntry.LinkPath = ""
				rEntry.SyncedEnabled = nil

				localEntry, hasLocal := localReg.Plugins[id]
				isBuiltin := manifest.IsBuiltin(id)
				hasLocalLink := hasLocal && localEntry.Source == "link" && localEntry.LinkPath != "" && manifestExists(localEntry.LinkPath)
				pDir, _ := st.PluginDir(id)
				hasUnpacked := pDir != "" && manifestExists(pDir)

				if hasLocal && localEntry.InstalledAt != 0 && rEntry.InstalledAt == 0 {
					rEntry.InstalledAt = localEntry.InstalledAt
				}

				switch {
				case isBuiltin:
					rEntry.Source = "builtin"
					rEntry.PendingInstall = false
				case hasLocalLink:
					rEntry.Source = "link"
					rEntry.LinkPath = localEntry.LinkPath
					rEntry.PendingInstall = false
				case hasUnpacked:
					if rEntry.Source == "link" || rEntry.Source == "" {
						rEntry.Source = "package"
					}
					rEntry.PendingInstall = false
				default:
					// Plugin is not installed on this machine: mark pending_install and
					// keep it disabled instead of silently downloading external code.
					desired := rEntry.Enabled
					rEntry.PendingInstall = true
					rEntry.Enabled = false
					rEntry.SyncedEnabled = &desired
					if rEntry.Source == "link" || rEntry.Source == "" {
						rEntry.Source = "package"
					}
				}
				merged.Plugins[id] = rEntry
			}
		}
		_, regStatErr := os.Stat(st.RegistryPath())
		if len(merged.Plugins) > 0 || len(localReg.Plugins) > 0 || regStatErr == nil {
			if err := st.SaveRegistry(merged); err != nil {
				return err
			}
		}
	}

	if entries, err := os.ReadDir(st.ConfigDir()); err == nil {
		for _, de := range entries {
			if de.IsDir() || de.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := de.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			id := strings.TrimSuffix(name, ".json")
			if id == "hyla.feishu" || manifest.ValidateID(id) != nil {
				continue
			}
			if _, keep := remoteCfgs[id]; !keep {
				_ = os.Remove(filepath.Join(st.ConfigDir(), name))
			}
		}
	}

	if len(remoteCfgs) > 0 {
		if err := os.MkdirAll(st.ConfigDir(), 0o755); err != nil {
			return err
		}
		for id, raw := range remoteCfgs {
			if id == "hyla.feishu" || manifest.ValidateID(id) != nil || !json.Valid(raw) {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err == nil && obj != nil {
				if err := st.SaveConfig(id, obj); err != nil {
					return err
				}
				continue
			}
			cfgPath, err := st.ConfigPath(id)
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, raw, "", "  "); err != nil {
				pretty.Write(raw)
			}
			pretty.WriteByte('\n')
			if err := os.WriteFile(cfgPath, pretty.Bytes(), 0o644); err != nil {
				return err
			}
		}
	}

	return nil
}

func manifestExists(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, manifest.ManifestFileName))
	return err == nil && !st.IsDir()
}
