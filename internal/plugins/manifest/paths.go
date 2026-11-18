package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
	"hyla/internal/proto"
)

// Store manages plugin paths, registry persistence, per-plugin JSON configs,
// and isolated secrets under BaseDir (which defaults to proto.Dir()).
type Store struct {
	BaseDir string
	// UseKeychain selects mygo.Secrets.Service("hyla-plugin:"+id) before
	// falling back to 0600 JSON files. Defaults to true when BaseDir == "" or
	// BaseDir == proto.Dir(), and false when isolated to a custom/temp directory.
	UseKeychain *bool
}

// DefaultStore returns the default store backed by proto.Dir().
func DefaultStore() *Store {
	return &Store{BaseDir: proto.Dir()}
}

func (s *Store) base() string {
	if s != nil && s.BaseDir != "" {
		return s.BaseDir
	}
	return proto.Dir()
}

func (s *Store) useKeychain() bool {
	if os.Getenv("HYLA_NO_KEYCHAIN") != "" {
		return false
	}
	if s != nil && s.UseKeychain != nil {
		return *s.UseKeychain
	}
	return s == nil || s.BaseDir == "" || s.BaseDir == proto.Dir()
}

func (s *Store) PluginsDir() string   { return filepath.Join(s.base(), "plugins") }
func (s *Store) RegistryPath() string { return filepath.Join(s.base(), "plugins.json") }
func (s *Store) ConfigDir() string    { return filepath.Join(s.base(), "plugin-config") }
func (s *Store) SecretsDir() string   { return filepath.Join(s.base(), "plugin-secrets") }
func (s *Store) DataRoot() string     { return filepath.Join(s.base(), "plugin-data") }
func (s *Store) LogsDir() string      { return filepath.Join(s.base(), "logs", "plugins") }

func (s *Store) PluginDir(id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.PluginsDir(), id), nil
}

func (s *Store) ConfigPath(id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.ConfigDir(), id+".json"), nil
}

func (s *Store) SecretsPath(id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.SecretsDir(), id+".json"), nil
}

func (s *Store) DataDir(id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.DataRoot(), id), nil
}

func (s *Store) LogPath(id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.LogsDir(), id+".log"), nil
}

// LoadConfig reads plugin-config/<id>.json as a JSON object map.
// Missing file returns an empty map without error.
func (s *Store) LoadConfig(id string) (map[string]any, error) {
	p, err := s.ConfigPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("read config for %s: %w", id, err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// SaveConfig atomically writes plugin-config/<id>.json with 0644 permissions.
func (s *Store) SaveConfig(id string, cfg map[string]any) error {
	p, err := s.ConfigPath(id)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(p, data, 0o755, 0o644)
}

var secretsMu sync.Mutex

// GetSecret retrieves key for plugin id using OS keychain if enabled, falling back
// to 0600 file storage at plugin-secrets/<id>.json on unsupported/headless hosts.
func (s *Store) GetSecret(id, key string) (string, bool, error) {
	if err := ValidateID(id); err != nil {
		return "", false, err
	}
	if key == "" {
		return "", false, fmt.Errorf("secret key must not be empty")
	}
	if s.useKeychain() {
		val, err := mygo.Secrets.Service("hyla-plugin:" + id).Get(key)
		if err == nil {
			return string(val), true, nil
		}
		if errors.Is(err, mygo.ErrSecretNotFound) {
			// Also inspect file fallback in case a large secret or previously migrated secret lives there.
			return s.getSecretFile(id, key)
		}
		if !errors.Is(err, errors.ErrUnsupported) {
			return "", false, err
		}
	}
	return s.getSecretFile(id, key)
}

// SetSecret stores value under key for plugin id using OS keychain if available
// (and within mygo.MaxSecretSize), falling back cleanly to 0600 JSON file storage.
func (s *Store) SetSecret(id, key, value string) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("secret key must not be empty")
	}
	if s.useKeychain() && len(value) <= mygo.MaxSecretSize {
		err := mygo.Secrets.Service("hyla-plugin:"+id).Set(key, []byte(value))
		if err == nil {
			// Remove any stale fallback copy so keychain stays single source of truth.
			_ = s.deleteSecretFile(id, key)
			return nil
		}
		if !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
	}
	return s.setSecretFile(id, key, value)
}

// DeleteSecret removes key for plugin id from both keychain and fallback file.
func (s *Store) DeleteSecret(id, key string) error {
	if err := ValidateID(id); err != nil {
		return err
	}
	if key == "" {
		return nil
	}
	var firstErr error
	if s.useKeychain() {
		if err := mygo.Secrets.Service("hyla-plugin:"+id).Delete(key); err != nil &&
			!errors.Is(err, mygo.ErrSecretNotFound) && !errors.Is(err, errors.ErrUnsupported) {
			firstErr = err
		}
	}
	if err := s.deleteSecretFile(id, key); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *Store) getSecretFile(id, key string) (string, bool, error) {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	m, err := s.readSecretFileLocked(id)
	if err != nil {
		return "", false, err
	}
	v, ok := m[key]
	return v, ok, nil
}

func (s *Store) setSecretFile(id, key, value string) error {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	m, err := s.readSecretFileLocked(id)
	if err != nil {
		return err
	}
	m[key] = value
	return s.writeSecretFileLocked(id, m)
}

func (s *Store) deleteSecretFile(id, key string) error {
	secretsMu.Lock()
	defer secretsMu.Unlock()
	m, err := s.readSecretFileLocked(id)
	if err != nil {
		return err
	}
	if _, ok := m[key]; !ok {
		return nil
	}
	delete(m, key)
	return s.writeSecretFileLocked(id, m)
}

func (s *Store) readSecretFileLocked(id string) (map[string]string, error) {
	p, err := s.SecretsPath(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("read secrets for %s: %w", id, err)
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

func (s *Store) writeSecretFileLocked(id string, m map[string]string) error {
	p, err := s.SecretsPath(id)
	if err != nil {
		return err
	}
	if len(m) == 0 {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(p, data, 0o700, 0o600)
}

func writeFileAtomic(path string, data []byte, dirMode, fileMode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return os.Chmod(path, fileMode)
}
