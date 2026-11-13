package remote

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A pair code names the key of a tailcat listener, as its region does.
func TestTailcatCode(t *testing.T) {
	k, err := NewTailcatKey()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	code, err := TailcatAddress(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "tc") {
		t.Errorf("pair code %q", code)
	}
	// The same key's code is the same, its region kept.
	again, err := TailcatAddress(ctx, k)
	if err != nil || again != code {
		t.Errorf("again %q, %v", again, err)
	}
	if TailcatCode("tailcat://"+code) != code || TailcatCode(code) != code {
		t.Errorf("TailcatCode(%q)", code)
	}
	if !LooksLikePairCode(code) || LooksLikePairCode("nope") {
		t.Errorf("LooksLikePairCode(%q)", code)
	}
}

// The key a tailcat listener makes stays, for the user alone: its pair
// code is a secret.
func TestTailcatKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tailcat.json")
	k, err := LoadTailcatKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file %v, %v", st, err)
	}
	again, err := LoadTailcatKey(path)
	if err != nil || again.Key != k.Key || again.PSK != k.PSK {
		t.Errorf("read back %+v, %v", again, err)
	}
	// A new key's code is another, and keeps the region.
	k, err = RotateTailcatKey(path)
	if err != nil {
		t.Fatal(err)
	}
	again, err = LoadTailcatKey(path)
	if err != nil || again.Key != k.Key || again.PSK != k.PSK {
		t.Errorf("rotated read back %+v, %v", again, err)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("rotated key file %v, %v", st, err)
	}
}

// The region a key picks is kept with it, so that the app, the CLI and
// hylad's listener make one code, at one relay; and a rotated key keeps
// that relay.
func TestTailcatKeyRegionKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tailcat.json")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	k, err := TailcatKeyAt(ctx, path)
	if err != nil {
		t.Skip("no relay: ", err)
	}
	if k.Region == "" {
		t.Fatal("no region kept")
	}
	kept, err := LoadTailcatKey(path)
	if err != nil || kept.Region != k.Region || kept.Key != k.Key {
		t.Fatalf("kept %+v, %v; picked %q", kept, err, k.Region)
	}
	// The code names that region, and is the same next time.
	code, err := TailcatAddress(ctx, kept)
	if err != nil {
		t.Fatal(err)
	}
	again, err := TailcatKeyAt(ctx, path)
	if err != nil || again.Region != k.Region || again.Key != k.Key {
		t.Fatalf("again %+v, %v", again, err)
	}
	if second, err := TailcatAddress(ctx, again); err != nil || second != code {
		t.Errorf("code %q, %v, was %q", second, err, code)
	}
	// A rotated key is another code, at the same relay.
	rotated, err := RotateTailcatCode(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if rotated == code {
		t.Error("the pair code stays")
	}
	if rk, err := LoadTailcatKey(path); err != nil || rk.Region != k.Region {
		t.Errorf("rotated key %+v, %v", rk, err)
	}
}
