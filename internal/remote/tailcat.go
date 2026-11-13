package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tailscale/tailcat"

	"hyla/internal/proto"
	"tailscale.com/tailcfg"
)

// A tailcat address reaches a host over Tailcat (github.com/tailscale/
// tailcat): WireGuard between the two machines, through a DERP relay to
// find each other, and directly once they do. There is no Tailscale, and
// no account: hylad's tailcat://:port listener makes a key and a pair
// code (TailcatAddress), a tc… string of the host's key, its pre-shared
// key and its region, which is the host's address here. Whoever has the
// pair code may connect, as whoever has a key may, so the address is a
// secret as much as a name.
//
// The pair code is whole in the address. A code whose region is only a
// number has the region's details looked up in the relay set (SetRelay),
// as hylad makes codes when the client's relay is not that.
//
// The region a key picked is kept with it (TailcatKeyAt), so that the
// listener, the app and `hyla hosts tailcat code` make one code, at one
// relay, across restarts. A code that names a relay the host is not on
// reaches nobody, as a code of a key the host has rotated does.

// DefaultDERPMapURL is where tailcat finds its relays, unless SetRelay
// names a map of others: Tailscale's public, rate-limited ones.
const DefaultDERPMapURL = tailcat.DefaultDERPMapURL

// SetRelay sets the URL of the DERP map where tailcat finds relays: ""
// for DefaultDERPMapURL.
func SetRelay(u string) { relay.Store(&u) }

var relay atomic.Pointer[string]

func relayURL() string {
	if p := relay.Load(); p != nil && *p != "" {
		return *p
	}
	return DefaultDERPMapURL
}

// TailcatKey is what hylad's tailcat listener keeps on disk: its key,
// its pre-shared key, and the region of the relay it chose, so that its
// pair code stays the same across hylad's restarts.
type TailcatKey struct {
	Key    string `json:"key"`
	PSK    string `json:"psk"`
	Region string `json:"region,omitempty"` // a region of the relay set's
}

// TailcatKeyPath is where hylad keeps its tailcat key: "tailcat.json" in
// Hyla's data folder.
func TailcatKeyPath() string { return filepath.Join(proto.Dir(), "tailcat.json") }

// LocalTailcatAddress is this machine's tailcat address, as hylad's key
// makes it: "tailcat://…", for hylad's listener at the settings' port.
func LocalTailcatAddress(ctx context.Context) (string, error) {
	k, err := TailcatKeyAt(ctx, TailcatKeyPath())
	if err != nil {
		return "", err
	}
	code, err := TailcatAddress(ctx, k)
	if err != nil {
		return "", err
	}
	return "tailcat://" + code, nil
}

// LoadTailcatKey reads hylad's tailcat key at path, or makes one, which
// it keeps there for the user alone: the pair code is a secret.
func LoadTailcatKey(path string) (*TailcatKey, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		var k TailcatKey
		if err := json.Unmarshal(b, &k); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return &k, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k, err := NewTailcatKey()
	if err != nil {
		return nil, err
	}
	if err := SaveTailcatKey(path, k); err != nil {
		return nil, err
	}
	return k, nil
}

// SaveTailcatKey keeps k at path, for the user alone: the pair code is a
// secret. It writes a file beside it and renames that over it, so that a
// reader finds a whole key or none, as hylad's listener and the app read
// it at once.
func SaveTailcatKey(path string, k *TailcatKey) error {
	b, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// tailcatKeyMu makes the pick of TailcatKeyAt one of a process's at a
// time, as the app's settings and its pair code ask for one together.
var tailcatKeyMu sync.Mutex

// TailcatKeyAt reads the key at path, picks its region if it has none,
// and keeps it there, so that this machine's pair code names one relay
// for good: the listener's, the app's and `hyla hosts tailcat code`'s.
// TailcatAddress then makes the code without a netcheck.
func TailcatKeyAt(ctx context.Context, path string) (*TailcatKey, error) {
	tailcatKeyMu.Lock()
	defer tailcatKeyMu.Unlock()
	k, err := LoadTailcatKey(path)
	if err != nil {
		return nil, err
	}
	before := k.Region
	if _, err := TailcatAddress(ctx, k); err != nil {
		return nil, err
	}
	if k.Region == before {
		return k, nil
	}
	if before == "" {
		// Another Hyla here may have kept a region meanwhile: the file's
		// is this machine's, and the one its listener joined.
		if cur, err := LoadTailcatKey(path); err == nil && cur.Region != "" {
			kept := cur.Region
			if _, err := TailcatAddress(ctx, cur); err != nil {
				return nil, err
			}
			if cur.Region != kept {
				if err := SaveTailcatKey(path, cur); err != nil {
					return nil, err
				}
			}
			return cur, nil
		}
	}
	if err := SaveTailcatKey(path, k); err != nil {
		return nil, err
	}
	return k, nil
}

// RotateTailcatKey gives the tailcat key at path a new pair code, which
// the old key's no longer opens, as when a code has gone to the wrong
// person. The new key keeps the region, which callers read again.
func RotateTailcatKey(path string) (*TailcatKey, error) {
	old, err := LoadTailcatKey(path)
	if err != nil {
		return nil, err
	}
	k, err := NewTailcatKey()
	if err != nil {
		return nil, err
	}
	k.Region = old.Region
	if err := SaveTailcatKey(path, k); err != nil {
		return nil, err
	}
	return k, nil
}

// RotateTailcatCode is RotateTailcatKey, and the code the new key makes:
// the one its listener joins through, which it picks and keeps if the
// old key kept no region.
func RotateTailcatCode(ctx context.Context, path string) (string, error) {
	if _, err := RotateTailcatKey(path); err != nil {
		return "", err
	}
	k, err := TailcatKeyAt(ctx, path)
	if err != nil {
		return "", err
	}
	return TailcatAddress(ctx, k)
}

// NewTailcatKey makes a key for a tailcat listener.
func NewTailcatKey() (*TailcatKey, error) {
	t := tailcat.NewPrivateKey()
	node, err := t.Private.MarshalText()
	if err != nil {
		return nil, err
	}
	psk, err := t.Public.PresharedKey.MarshalText()
	if err != nil {
		return nil, err
	}
	return &TailcatKey{Key: string(node), PSK: string(psk)}, nil
}

// TailcatAddress is k's pair code at the region k keeps, as "derp302" in
// the relay set's list, and at the nearest one if k has none, which it
// then keeps, for TailcatKeyAt to save with the key. The code names the
// region without its details, which a client finds in its relay set.
func TailcatAddress(ctx context.Context, k *TailcatKey) (string, error) {
	t := &tailcat.PrivateKey{Public: tailcat.ConnInfo{RegionID: -1}}
	if err := t.Private.UnmarshalText([]byte(k.Key)); err != nil {
		return "", fmt.Errorf("tailcat key: %w", err)
	}
	if err := t.Public.PresharedKey.UnmarshalText([]byte(k.PSK)); err != nil {
		return "", fmt.Errorf("tailcat key's pre-shared key: %w", err)
	}
	region := hostRegion(k.Region)
	if region == nil {
		dm, err := tailcat.FetchDERPMap(ctx, tailcat.DERPMapURL(relayURL()))
		if err != nil {
			return "", fmt.Errorf("the tailcat relay: %w", err)
		}
		// The region the key kept, as the listener joined it; the
		// nearest, for a key that has none, or one the relay set no
		// longer names.
		if region = relayRegion(dm, k.Region); region == nil {
			id, err := tailcat.PickBestRegion(ctx, dm)
			if err != nil {
				return "", fmt.Errorf("the tailcat relay: %w", err)
			}
			if region = dm.Regions[id]; region == nil {
				return "", errors.New("the tailcat relay: no region reached")
			}
			k.Region = regionNode(region)
		}
	}
	t.Public.ServerPublic = tailcat.NodePublic{NodePublic: t.Private.Public()}
	t.Public.ServerDiscoPublic = tailcat.DiscoPublicForNode(t.Private)
	t.Public.Region = []*tailcfg.DERPRegion{region}
	t.Public.RegionID = 0
	return string(t.Public.Addr()), nil
}

// relayRegion is the region of the map named "derp302", as the map's
// nodes name their regions.
func relayRegion(dm *tailcfg.DERPMap, node string) *tailcfg.DERPRegion {
	for _, r := range dm.Regions {
		for _, n := range r.Nodes {
			if n.Name == node || n.HostName == node {
				return r
			}
		}
	}
	return nil
}

// hostRegion is the region k saved: the one of a node of the region,
// which tailcat takes for a region of the client's relay map. The node's
// name is the region's number, as the map's are.
func hostRegion(saved string) *tailcfg.DERPRegion {
	id, err := strconv.ParseInt(saved, 10, 64)
	if err != nil || !tailcfg.DERPRegionID(id).IsValid() {
		return nil
	}
	return &tailcfg.DERPRegion{
		RegionID:   tailcfg.DERPRegionID(id),
		RegionCode: "hyla",
		RegionName: "Tailcat relay",
		Nodes:      []*tailcfg.DERPNode{{Name: saved, HostName: "relay", RegionID: tailcfg.DERPRegionID(id)}},
	}
}

// regionNode is the name k keeps of a region: its first node's name, its
// number in the relay map.
func regionNode(r *tailcfg.DERPRegion) string {
	if len(r.Nodes) > 0 && r.Nodes[0].Name != "" {
		return r.Nodes[0].Name
	}
	return strconv.Itoa(int(r.RegionID))
}

// LooksLikePairCode reports whether s is shaped as a tailcat pair code
// makes it: "tc", then base64url. A whole one decodes as a ConnInfo
// (ValidPairCode); a code holds no dots, slashes or spaces.
func LooksLikePairCode(s string) bool {
	if len(s) < 16 || !strings.HasPrefix(s, "tc") {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// ValidPairCode reports whether s may be a tailcat pair code.
func ValidPairCode(s string) bool {
	_, err := tailcat.ParseAddr(tailcat.Addr(s))
	return err == nil
}

// TailcatCode is a pair code as the settings hold it: "tailcat://" and
// the code, "tc…".
func TailcatCode(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "tailcat://") }

// watch tells now and then how the client reaches the host: directly,
// where the networks let them, or through the relay.
// LocalName is this machine's name, as a code gives it: the hostname
// up to its first dot, which is the label HylaPocket saves a host under.
func LocalName() string {
	h, _ := os.Hostname()
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}
