package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"
)

// Registry is persisted at <proto.Dir()>/plugins.json.
type Registry struct {
	Version int                      `json:"version"`
	Plugins map[string]RegistryEntry `json:"plugins"`
}

// RegistryEntry tracks installation state and granted permissions for one plugin.
type RegistryEntry struct {
	ID                 string   `json:"id"`
	Enabled            bool     `json:"enabled"`
	Version            string   `json:"version,omitempty"`
	Source             string   `json:"source,omitempty"` // "package" | "link" | "builtin"
	SHA256             string   `json:"sha256,omitempty"`
	LinkPath           string   `json:"linkPath,omitempty"`
	PendingInstall     bool     `json:"pendingInstall,omitempty"`
	SyncedEnabled      *bool    `json:"syncedEnabled,omitempty"`
	GrantedPermissions []string `json:"grantedPermissions,omitempty"`
	InstalledAt        int64    `json:"installedAt,omitempty"`
	UpdatedAt          int64    `json:"updatedAt,omitempty"`
}

// BuiltinPlugin describes a plugin bundled directly inside the Hyla binary.
type BuiltinPlugin struct {
	Manifest         *Manifest
	EnabledByDefault bool
	AutoGrant        bool
}

// InstalledPlugin represents a resolved plugin entry together with its parsed manifest.
type InstalledPlugin struct {
	Entry     RegistryEntry
	Manifest  *Manifest
	RootDir   string
	Builtin   bool
	LoadError string
}

var (
	builtinsMu sync.RWMutex
	builtins   = make(map[string]BuiltinPlugin)
)

// RegisterBuiltin registers a built-in bundled plugin (such as hyla.magpie or hyla.feishu).
func RegisterBuiltin(b BuiltinPlugin) error {
	if b.Manifest == nil {
		return fmt.Errorf("builtin plugin manifest must not be nil")
	}
	if err := b.Manifest.Validate(); err != nil {
		return err
	}
	builtinsMu.Lock()
	builtins[b.Manifest.ID] = b
	builtinsMu.Unlock()
	return nil
}

// Builtins returns all registered bundled plugins sorted by ID.
func Builtins() []BuiltinPlugin {
	builtinsMu.RLock()
	defer builtinsMu.RUnlock()
	out := make([]BuiltinPlugin, 0, len(builtins))
	for _, b := range builtins {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Manifest.ID < out[j].Manifest.ID
	})
	return out
}

// HasPermission reports whether scope is granted to this entry.
func (e RegistryEntry) HasPermission(scope string) bool {
	return slices.Contains(e.GrantedPermissions, scope)
}

var registryMu sync.Mutex

// LoadRegistry reads plugins.json, returning an empty v1 Registry if missing.
func (s *Store) LoadRegistry() (*Registry, error) {
	registryMu.Lock()
	defer registryMu.Unlock()
	return s.loadRegistryLocked()
}

func (s *Store) loadRegistryLocked() (*Registry, error) {
	p := s.RegistryPath()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{Version: 1, Plugins: map[string]RegistryEntry{}}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return &Registry{Version: 1, Plugins: map[string]RegistryEntry{}}, nil
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if reg.Version == 0 {
		reg.Version = 1
	}
	if reg.Plugins == nil {
		reg.Plugins = map[string]RegistryEntry{}
	}
	return &reg, nil
}

// SaveRegistry writes plugins.json atomically.
func (s *Store) SaveRegistry(reg *Registry) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	return s.saveRegistryLocked(reg)
}

func (s *Store) saveRegistryLocked(reg *Registry) error {
	if reg == nil {
		reg = &Registry{Version: 1, Plugins: map[string]RegistryEntry{}}
	}
	if reg.Version == 0 {
		reg.Version = 1
	}
	if reg.Plugins == nil {
		reg.Plugins = map[string]RegistryEntry{}
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(s.RegistryPath(), data, 0o755, 0o644)
}

// Enable turns a plugin on or off in plugins.json.
func (s *Store) Enable(id string, enabled bool) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	reg, err := s.loadRegistryLocked()
	if err != nil {
		return err
	}
	entry, ok := reg.Plugins[id]
	if !ok {
		builtinsMu.RLock()
		b, isBuiltin := builtins[id]
		builtinsMu.RUnlock()
		if !isBuiltin {
			// Check whether plugin directory exists on disk even if missing from plugins.json.
			dir, _ := s.PluginDir(id)
			if m, err := LoadFile(dir); err == nil {
				entry = RegistryEntry{
					ID:          id,
					Version:     m.Version,
					Source:      "package",
					InstalledAt: time.Now().UnixMilli(),
				}
				ok = true
			}
		} else {
			entry = builtinDefaultEntry(b)
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("plugin %q is not installed", id)
	}
	entry.Enabled = enabled
	entry.UpdatedAt = time.Now().UnixMilli()
	reg.Plugins[id] = entry
	return s.saveRegistryLocked(reg)
}

// SetPermission grants or revokes one permission scope for plugin id.
func (s *Store) SetPermission(id, scope string, granted bool) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	if !IsKnownPermission(scope) {
		return fmt.Errorf("unknown permission scope %q", scope)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	reg, err := s.loadRegistryLocked()
	if err != nil {
		return err
	}
	entry, ok := reg.Plugins[id]
	if !ok {
		builtinsMu.RLock()
		b, isBuiltin := builtins[id]
		builtinsMu.RUnlock()
		if !isBuiltin {
			return fmt.Errorf("plugin %q is not installed", id)
		}
		entry = builtinDefaultEntry(b)
	}
	has := slices.Contains(entry.GrantedPermissions, scope)
	if granted && !has {
		entry.GrantedPermissions = append(entry.GrantedPermissions, scope)
		slices.Sort(entry.GrantedPermissions)
	} else if !granted && has {
		entry.GrantedPermissions = slices.DeleteFunc(entry.GrantedPermissions, func(v string) bool {
			return v == scope
		})
	}
	entry.UpdatedAt = time.Now().UnixMilli()
	reg.Plugins[id] = entry
	return s.saveRegistryLocked(reg)
}

// GrantPermissions replaces/merges granted permissions on an installed plugin.
func (s *Store) GrantPermissions(id string, scopes []string) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	for _, sc := range scopes {
		if !IsKnownPermission(sc) {
			return fmt.Errorf("unknown permission scope %q", sc)
		}
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	reg, err := s.loadRegistryLocked()
	if err != nil {
		return err
	}
	entry, ok := reg.Plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q is not installed", id)
	}
	uniq := make([]string, 0, len(scopes))
	seen := make(map[string]bool, len(scopes))
	for _, sc := range scopes {
		if !seen[sc] {
			seen[sc] = true
			uniq = append(uniq, sc)
		}
	}
	slices.Sort(uniq)
	entry.GrantedPermissions = uniq
	entry.UpdatedAt = time.Now().UnixMilli()
	reg.Plugins[id] = entry
	return s.saveRegistryLocked(reg)
}

// Link registers a local development directory via Source="link" without copying files.
func (s *Store) Link(dir string, grantAll bool) (*InstalledPlugin, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	m, err := LoadFile(absDir)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	var granted []string
	if grantAll {
		granted = append([]string(nil), m.Permissions.Scopes...)
		slices.Sort(granted)
	}
	entry := RegistryEntry{
		ID:                 m.ID,
		Enabled:            true,
		Version:            m.Version,
		Source:             "link",
		LinkPath:           absDir,
		PendingInstall:     false,
		GrantedPermissions: granted,
		InstalledAt:        now,
		UpdatedAt:          now,
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	reg, err := s.loadRegistryLocked()
	if err != nil {
		return nil, err
	}
	if prev, ok := reg.Plugins[m.ID]; ok {
		if prev.InstalledAt != 0 {
			entry.InstalledAt = prev.InstalledAt
		}
		if !grantAll && len(prev.GrantedPermissions) > 0 {
			entry.GrantedPermissions = intersection(prev.GrantedPermissions, m.Permissions.Scopes)
		}
	}
	reg.Plugins[m.ID] = entry
	if err := s.saveRegistryLocked(reg); err != nil {
		return nil, err
	}
	return &InstalledPlugin{Entry: entry, Manifest: m, RootDir: absDir}, nil
}

// Remove uninstalls a plugin package or link entry. Built-in plugins cannot be
// deleted from the binary, only reset/disabled if requested.
func (s *Store) Remove(id string, purgeConfig bool) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	builtinsMu.RLock()
	_, isBuiltin := builtins[id]
	builtinsMu.RUnlock()
	if isBuiltin {
		return fmt.Errorf("plugin %q is built-in; disable it instead of removing", id)
	}
	registryMu.Lock()
	reg, err := s.loadRegistryLocked()
	if err != nil {
		registryMu.Unlock()
		return err
	}
	entry, ok := reg.Plugins[id]
	if ok {
		delete(reg.Plugins, id)
		if err := s.saveRegistryLocked(reg); err != nil {
			registryMu.Unlock()
			return err
		}
	}
	registryMu.Unlock()

	if entry.Source != "link" {
		if dir, err := s.PluginDir(id); err == nil {
			_ = os.RemoveAll(dir)
		}
	}
	if purgeConfig {
		if p, err := s.ConfigPath(id); err == nil {
			_ = os.Remove(p)
		}
		if p, err := s.SecretsPath(id); err == nil {
			_ = os.Remove(p)
		}
		if p, err := s.DataDir(id); err == nil {
			_ = os.RemoveAll(p)
		}
		if p, err := s.LogPath(id); err == nil {
			_ = os.Remove(p)
		}
	}
	return nil
}

// Discover loads all registered, on-disk, and built-in plugins deterministically sorted by ID.
func (s *Store) Discover() ([]InstalledPlugin, error) {
	reg, err := s.LoadRegistry()
	if err != nil {
		return nil, err
	}

	byID := make(map[string]InstalledPlugin)

	// 1. Seed registered built-ins.
	builtinsMu.RLock()
	for id, b := range builtins {
		entry := builtinDefaultEntry(b)
		if override, ok := reg.Plugins[id]; ok {
			entry.Enabled = override.Enabled
			if len(override.GrantedPermissions) > 0 || override.UpdatedAt > 0 {
				entry.GrantedPermissions = intersection(override.GrantedPermissions, b.Manifest.Permissions.Scopes)
			}
			entry.UpdatedAt = override.UpdatedAt
		}
		byID[id] = InstalledPlugin{
			Entry:    entry,
			Manifest: b.Manifest,
			Builtin:  true,
		}
	}
	builtinsMu.RUnlock()

	// 2. Load registry entries (packages, links, pending-install remote plugins).
	for id, entry := range reg.Plugins {
		if b, isBuiltin := byID[id]; isBuiltin && entry.Source == "builtin" {
			_ = b
			continue
		}
		if isBuiltinEntry(id) && entry.Source == "" {
			continue
		}
		rootDir := entry.LinkPath
		if entry.Source != "link" {
			rootDir, _ = s.PluginDir(id)
		}
		if entry.PendingInstall {
			byID[id] = InstalledPlugin{Entry: entry, RootDir: rootDir}
			continue
		}
		if rootDir == "" {
			entry.PendingInstall = true
			byID[id] = InstalledPlugin{Entry: entry}
			continue
		}
		m, loadErr := LoadFile(rootDir)
		if loadErr != nil {
			if os.IsNotExist(loadErr) {
				entry.PendingInstall = true
			}
			byID[id] = InstalledPlugin{
				Entry:     entry,
				RootDir:   rootDir,
				LoadError: loadErr.Error(),
			}
			continue
		}
		if m.ID != id {
			byID[id] = InstalledPlugin{
				Entry:     entry,
				RootDir:   rootDir,
				LoadError: fmt.Sprintf("manifest id %q does not match directory/registry id %q", m.ID, id),
			}
			continue
		}
		byID[id] = InstalledPlugin{
			Entry:    entry,
			Manifest: m,
			RootDir:  rootDir,
		}
	}

	// 3. Also discover unpacked folders placed directly in <proto.Dir()>/plugins/<id>/
	// even before a registry write (useful for manual copies in development).
	if dirs, err := os.ReadDir(s.PluginsDir()); err == nil {
		for _, de := range dirs {
			if !de.IsDir() {
				continue
			}
			id := de.Name()
			if ValidateID(id) != nil {
				continue
			}
			if _, exists := byID[id]; exists {
				continue
			}
			rootDir := filepath.Join(s.PluginsDir(), id)
			m, loadErr := LoadFile(rootDir)
			entry := RegistryEntry{
				ID:      id,
				Enabled: true,
				Source:  "directory",
			}
			if loadErr != nil {
				byID[id] = InstalledPlugin{Entry: entry, RootDir: rootDir, LoadError: loadErr.Error()}
				continue
			}
			if m.ID != id {
				byID[id] = InstalledPlugin{
					Entry:     entry,
					RootDir:   rootDir,
					LoadError: fmt.Sprintf("manifest id %q does not match folder %q", m.ID, id),
				}
				continue
			}
			entry.Version = m.Version
			byID[id] = InstalledPlugin{Entry: entry, Manifest: m, RootDir: rootDir}
		}
	}

	out := make([]InstalledPlugin, 0, len(byID))
	for _, p := range byID {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Entry.ID < out[j].Entry.ID
	})
	return out, nil
}

func isBuiltinEntry(id string) bool {
	builtinsMu.RLock()
	_, ok := builtins[id]
	builtinsMu.RUnlock()
	return ok
}

func builtinDefaultEntry(b BuiltinPlugin) RegistryEntry {
	var granted []string
	if b.AutoGrant {
		granted = append([]string(nil), b.Manifest.Permissions.Scopes...)
		slices.Sort(granted)
	}
	return RegistryEntry{
		ID:                 b.Manifest.ID,
		Enabled:            b.EnabledByDefault,
		Version:            b.Manifest.Version,
		Source:             "builtin",
		GrantedPermissions: granted,
	}
}

func intersection(prev, allowed []string) []string {
	if len(prev) == 0 || len(allowed) == 0 {
		return nil
	}
	allowSet := make(map[string]bool, len(allowed))
	for _, s := range allowed {
		allowSet[s] = true
	}
	var out []string
	for _, s := range prev {
		if allowSet[s] {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}
