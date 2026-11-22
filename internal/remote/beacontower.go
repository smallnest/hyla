package remote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hyla/internal/beacontower"
	"hyla/internal/proto"
)

// BeaconTowerKey holds this machine's persistent BeaconTower identity key and
// pre-shared key on disk so pair codes survive hylad restarts until rotated.
type BeaconTowerKey struct {
	Key string `json:"key"`
	PSK string `json:"psk"`
}

// BeaconTowerKeyPath returns where hylad persists its BeaconTower keypair:
// "beacontower.json" inside Hyla's data directory.
func BeaconTowerKeyPath() string { return filepath.Join(proto.Dir(), "beacontower.json") }

var beaconKeyMu sync.Mutex

// NewBeaconTowerKey generates a fresh X25519 keypair and pre-shared key.
func NewBeaconTowerKey() (*BeaconTowerKey, error) {
	kp, err := beacontower.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	return &BeaconTowerKey{
		Key: base64.RawURLEncoding.EncodeToString(kp.Priv[:]),
		PSK: base64.RawURLEncoding.EncodeToString(kp.PSK[:]),
	}, nil
}

// KeyPair decodes k into its cryptographic material and derives the public key.
func (k *BeaconTowerKey) KeyPair() (beacontower.KeyPair, error) {
	if k == nil {
		return beacontower.KeyPair{}, errors.New("nil beacontower key")
	}
	privBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(k.Key))
	if err != nil || len(privBytes) != 32 {
		return beacontower.KeyPair{}, errors.New("beacontower key: invalid private key")
	}
	pskBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(k.PSK))
	if err != nil || len(pskBytes) != 32 {
		return beacontower.KeyPair{}, errors.New("beacontower key: invalid preshared key")
	}
	var kp beacontower.KeyPair
	copy(kp.Priv[:], privBytes)
	copy(kp.PSK[:], pskBytes)
	pub, err := beacontower.PublicFromPrivate(kp.Priv)
	if err != nil {
		return beacontower.KeyPair{}, err
	}
	kp.Pub = pub
	return kp, nil
}

// LoadBeaconTowerKey reads the key at path, generating and saving one if missing.
func LoadBeaconTowerKey(path string) (*BeaconTowerKey, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		var k BeaconTowerKey
		if err := json.Unmarshal(b, &k); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if _, err := k.KeyPair(); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return &k, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k, err := NewBeaconTowerKey()
	if err != nil {
		return nil, err
	}
	if err := SaveBeaconTowerKey(path, k); err != nil {
		return nil, err
	}
	return k, nil
}

// SaveBeaconTowerKey atomically writes k to path with 0600 permissions.
func SaveBeaconTowerKey(path string, k *BeaconTowerKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_ = f.Chmod(0o600)
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// BeaconTowerKeyAt returns the key stored at path under process synchronization.
func BeaconTowerKeyAt(path string) (*BeaconTowerKey, error) {
	beaconKeyMu.Lock()
	defer beaconKeyMu.Unlock()
	return LoadBeaconTowerKey(path)
}

// RotateBeaconTowerKey replaces the key at path with a new keypair and PSK.
func RotateBeaconTowerKey(path string) (*BeaconTowerKey, error) {
	beaconKeyMu.Lock()
	defer beaconKeyMu.Unlock()
	k, err := NewBeaconTowerKey()
	if err != nil {
		return nil, err
	}
	if err := SaveBeaconTowerKey(path, k); err != nil {
		return nil, err
	}
	return k, nil
}

// BeaconTowerAddress builds a bare bt1_... pair code encoding relayAddr, k, and optional relayToken.
func BeaconTowerAddress(k *BeaconTowerKey, relayAddr, relayToken string) (string, error) {
	kp, err := k.KeyPair()
	if err != nil {
		return "", err
	}
	return beacontower.EncodePairCode(beacontower.PairCode{
		Relay:      relayAddr,
		HostPub:    kp.Pub,
		PSK:        kp.PSK,
		RelayToken: relayToken,
	})
}

// LocalBeaconTowerAddress loads/creates this machine's BeaconTower key and returns its bt1_... code.
func LocalBeaconTowerAddress(relayAddr, relayToken string) (string, error) {
	k, err := BeaconTowerKeyAt(BeaconTowerKeyPath())
	if err != nil {
		return "", err
	}
	return BeaconTowerAddress(k, relayAddr, relayToken)
}

// RotateBeaconTowerCode rotates the key at path and returns its new bt1_... pair code
// if relayAddr is non-empty.
func RotateBeaconTowerCode(path, relayAddr, relayToken string) (string, error) {
	k, err := RotateBeaconTowerKey(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(relayAddr) == "" {
		return "", nil
	}
	return BeaconTowerAddress(k, relayAddr, relayToken)
}

// ValidBeaconRelay reports whether s is a valid BeaconTower relay endpoint.
func ValidBeaconRelay(s string) bool {
	_, err := beacontower.ParseRelayAddr(s)
	return err == nil
}

// ValidBeaconCode reports whether s decodes as a complete BeaconTower pair code.
func ValidBeaconCode(s string) bool { return beacontower.ValidPairCode(s) }

// LooksLikeBeaconCode reports whether s syntactically resembles a BeaconTower pair code.
func LooksLikeBeaconCode(s string) bool { return beacontower.LooksLikePairCode(s) }

// BeaconCode returns the bare bt1_... code without scheme or whitespace.
func BeaconCode(s string) string { return beacontower.StripScheme(s) }

// BeaconRelayDisplay returns a short host:port label extracted from a BeaconTower pair code.
func BeaconRelayDisplay(s string) string {
	pc, err := beacontower.ParsePairCode(s)
	if err != nil {
		return ""
	}
	ra, err := beacontower.ParseRelayAddr(pc.Relay)
	if err != nil {
		return pc.Relay
	}
	return ra.Display()
}

// OpenBeaconTower opens an authenticated stream to a remote host's hylad over the shared
// BeaconTower E2EE multiplexed tunnel pool.
func OpenBeaconTower(ctx context.Context, code string) (net.Conn, error) {
	return beacontower.DefaultPool.Open(ctx, code, "hyla")
}

// CloseBeaconTower closes any pooled BeaconTower tunnel associated with code.
func CloseBeaconTower(code string) {
	beacontower.DefaultPool.CloseCode(code)
}
