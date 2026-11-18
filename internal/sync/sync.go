// Package sync keeps a machine's configuration the same as the others': one
// encrypted snapshot of the settings file, on a server they all can reach —
// a WebDAV folder, an S3-compatible bucket, or a GitHub repository.
//
// The server holds ciphertext alone. The passphrase is typed on each machine
// and never leaves it, and the settings hold secrets of their own, as a bot's
// or a relay's, which stay inside the snapshot.
package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hyla/internal/settings"
)

// ErrNotFound is a server with no snapshot yet: the first sync makes one.
var ErrNotFound = errors.New("no snapshot on the server")

// Config is where this machine syncs to, and how. It stays on this machine:
// the configuration it syncs does not carry it, so that the passphrase and
// the backend's own credentials never reach the snapshot.
type Config struct {
	Backend string `json:"backend,omitempty"` // "", "webdav", "s3" or "github"
	WebDAV  WebDAV `json:"webdav"`
	S3      S3     `json:"s3"`
	GitHub  GitHub `json:"github"`
	// Passphrase seals the snapshot. Every machine that syncs types the same
	// one; without it a snapshot cannot be read, on this machine or another.
	Passphrase string `json:"passphrase,omitempty"`
	// Auto pulls at launch, and pushes a change a while after it was made.
	Auto bool `json:"auto,omitempty"`
	// Last is what the last sync left here: the configuration both sides had
	// then, which tells a side that has not moved.
	Last State `json:"last"`
}

// WebDAV is a folder on a WebDAV server, as Jianguoyun's or Nextcloud's.
type WebDAV struct {
	URL      string `json:"url,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}

// S3 is a bucket of an S3-compatible service: MinIO, R2, B2, OSS, COS.
type S3 struct {
	Endpoint  string `json:"endpoint,omitempty"` // https://s3.example.com
	Bucket    string `json:"bucket,omitempty"`
	Prefix    string `json:"prefix,omitempty"` // "hyla" by default
	Region    string `json:"region,omitempty"` // "us-east-1" by default
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	PathStyle bool   `json:"pathStyle,omitempty"` // endpoint/bucket/key, as MinIO
}

// GitHub is a repository and the path in it, a private one for a
// configuration that holds secrets.
type GitHub struct {
	Repo   string `json:"repo,omitempty"` // owner/name
	Path   string `json:"path,omitempty"` // "hyla/config.enc" by default
	Branch string `json:"branch,omitempty"`
	Token  string `json:"token,omitempty"`
}

// State is what the last sync left: the hash of the configuration both sides
// had then, and the machine that wrote it.
type State struct {
	Hash string `json:"hash,omitempty"`
	Host string `json:"host,omitempty"`
	At   int64  `json:"at,omitempty"`
	// Backup is where a conflict kept the other side's configuration.
	Backup string `json:"backup,omitempty"`
}

// Path is where this machine keeps its own sync settings, beside the
// configuration it syncs.
func Path() string { return filepath.Join(filepath.Dir(settings.Path()), "sync.json") }

// Load returns this machine's sync settings, and the defaults where the file
// says nothing.
func Load() Config {
	var c Config
	if b, err := os.ReadFile(Path()); err == nil {
		json.Unmarshal(b, &c)
	}
	c.normalize()
	return c
}

func (c *Config) normalize() {
	c.S3.Region = or(c.S3.Region, "us-east-1")
	c.S3.Prefix = or(c.S3.Prefix, "hyla")
	c.GitHub.Path = or(c.GitHub.Path, "hyla/config.enc")
}

func or(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// Save writes this machine's sync settings, for its user alone: they hold a
// passphrase and a token.
func Save(c Config) error {
	c.normalize()
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Configured tells whether c names a server and a passphrase to seal what
// goes there.
func Configured(c Config) bool {
	if c.Backend == "" || strings.TrimSpace(c.Passphrase) == "" {
		return false
	}
	switch c.Backend {
	case "webdav":
		return c.WebDAV.URL != ""
	case "s3":
		return c.S3.Endpoint != "" && c.S3.Bucket != "" && c.S3.AccessKey != "" && c.S3.SecretKey != ""
	case "github":
		return strings.Count(c.GitHub.Repo, "/") == 1 && c.GitHub.Token != ""
	}
	return false
}

// Store is a server holding the snapshot: one object, replaced whole.
type Store interface {
	// Get reads the snapshot, ErrNotFound when the server has none yet.
	Get(ctx context.Context) ([]byte, error)
	Put(ctx context.Context, data []byte) error
}

// store returns the server c names. host is the machine's name, which a
// commit in a repository carries.
func store(c Config, host string) (Store, error) {
	switch c.Backend {
	case "webdav":
		return newWebDAV(c.WebDAV)
	case "s3":
		return newS3(c.S3)
	case "github":
		g, err := newGitHub(c.GitHub)
		if err != nil {
			return nil, err
		}
		g.host = host
		return g, nil
	case "":
		return nil, errors.New("sync: no server is set")
	}
	return nil, fmt.Errorf("sync: unknown server %q", c.Backend)
}

// Result is what a sync did.
type Result struct {
	// Action is what happened: "created" (the server had none), "pushed",
	// "pulled", or "unchanged" (both sides agreed).
	Action string
	// Host is the machine that wrote the snapshot, and At when it did.
	Host string
	At   int64
	// Backup is the file a conflict kept the other side's configuration in.
	Backup string
}

// Run syncs this machine's configuration with the server c names, and returns
// what it did, and c with what it left. dir is "" to decide by what the two
// sides did since the last sync, "push" to send this machine's, or "pull" to
// take the server's.
//
// A conflict, both sides changed since the last sync, leaves this machine's
// configuration as it is: the server's is kept in a file beside it and then
// replaced, so that nothing the user wrote is lost without a copy.
func Run(ctx context.Context, c Config, dir, host string) (Result, Config, error) {
	st, err := store(c, host)
	if err != nil {
		return Result{}, c, err
	}
	if strings.TrimSpace(c.Passphrase) == "" {
		return Result{}, c, errors.New("sync: no passphrase: the snapshot cannot be read")
	}
	local, err := os.ReadFile(settings.Target())
	if err != nil && !os.IsNotExist(err) {
		return Result{}, c, err
	}
	// A machine with no configuration file has nothing to send: an empty
	// one would be a configuration that wipes the others' the moment it
	// starts.
	configErr := err
	stPlugins := syncPluginStore()
	localPlugins, localPluginCfg, perr := collectPluginState(stPlugins)
	if perr != nil {
		return Result{}, c, perr
	}
	haveLocal := configErr == nil || len(localPluginCfg) > 0 || (localPlugins != nil && len(localPlugins.Plugins) > 0)
	localHash := computeStateHash(local, localPlugins, localPluginCfg)

	sealed, err := st.Get(ctx)
	var remote *Snapshot
	switch {
	case errors.Is(err, ErrNotFound):
		remote = nil
	case err != nil:
		return Result{}, c, err
	default:
		if remote, err = open(c.Passphrase, sealed); err != nil {
			return Result{}, c, err
		}
	}

	now := time.Now()
	res := Result{At: now.UnixMilli()}
	takeRemote := false
	switch {
	case remote == nil:
		// Nothing there yet: this machine's goes up, whichever way it was
		// asked to go.
	case !haveLocal:
		// No configuration here yet, as on a machine that just installed
		// Hyla: the server's is the one there is, however it was asked.
		takeRemote = true
	case dir == "pull":
		takeRemote = true
	case dir == "push":
		takeRemote = false
	case remote.Hash == localHash:
		res.Action, res.Host, res.At = "unchanged", remote.Host, remote.UpdatedAt
		c.Last = State{Hash: localHash, Host: remote.Host, At: remote.UpdatedAt}
		return res, c, nil
	case remote.Hash == c.Last.Hash:
		// The server has not moved since the last sync: this machine's
		// change goes up.
	case localHash == c.Last.Hash:
		// This machine has not moved: the server's change comes down.
		takeRemote = true
	default:
		// Both moved. This machine's configuration is the one kept: the
		// server's is put aside first, so that it is not lost.
		backup, err := keepAside(remote)
		if err != nil {
			return Result{}, c, err
		}
		res.Backup = backup
	}

	if takeRemote {
		if err := writeSettings(remote.Content); err != nil {
			return Result{}, c, err
		}
		if err := applyRemotePlugins(stPlugins, remote.Plugins, remote.PluginConfig); err != nil {
			return Result{}, c, err
		}
		res.Action, res.Host, res.At = "pulled", remote.Host, remote.UpdatedAt
		c.Last = State{Hash: remote.Hash, Host: remote.Host, At: remote.UpdatedAt, Backup: res.Backup}
		return res, c, nil
	}

	// This machine's goes up, sealed with the passphrase: the server sees
	// the version and the machine's name alone.
	snap := Snapshot{
		Version:      snapshotVersion,
		UpdatedAt:    res.At,
		Host:         host,
		Hash:         localHash,
		Content:      string(local),
		Plugins:      localPlugins,
		PluginConfig: localPluginCfg,
	}
	sealed, err = seal(c.Passphrase, snap)
	if err != nil {
		return Result{}, c, err
	}
	if err := st.Put(ctx, sealed); err != nil {
		return Result{}, c, err
	}
	res.Action, res.Host = "pushed", host
	if remote == nil && dir != "pull" {
		res.Action = "created"
	}
	c.Last = State{Hash: localHash, Host: host, At: res.At, Backup: res.Backup}
	return res, c, nil
}

// keepAside writes the server's configuration beside this machine's, for a
// conflict that this machine's wins.
func keepAside(s *Snapshot) (string, error) {
	dir := filepath.Dir(settings.Path())
	host := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == ' ' || r == 0 {
			return '-'
		}
		return r
	}, or(s.Host, "server"))
	for i := 0; ; i++ {
		name := fmt.Sprintf("config.remote-%s-%s", host, time.UnixMilli(s.UpdatedAt).Format("20060102-150405"))
		if i > 0 {
			name = fmt.Sprintf("%s-%d", name, i)
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(s.Content), 0o600); err != nil {
			return "", err
		}
		return path, nil
	}
}

// writeSettings puts the pulled configuration where the settings are, keeping
// the file's mode and the symlink a dotfiles repository keeps it behind.
func writeSettings(content string) error {
	path := settings.Target()
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".sync"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	settings.Reload()
	return nil
}

// hashOf is what tells one configuration from another: its contents, not
// their time, so that a file rewritten as it was is no change.
func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
