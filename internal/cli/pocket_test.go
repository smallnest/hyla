package cli

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hyla/internal/remote"
	"hyla/internal/settings"
)

// stdout runs f and returns what it wrote to stdout.
func stdout(t *testing.T, f func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	runErr := f()
	os.Stdout = old
	w.Close()
	b, _ := io.ReadAll(r)
	r.Close()
	return string(b), runErr
}

// pocketHome points hyla at a temporary home, with no listener and a
// BeaconTower relay configured, and saves this machine's key.
func pocketHome(t *testing.T) *remote.BeaconTowerKey {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_CONFIG", filepath.Join(home, "config"))
	settings.Reload()
	t.Cleanup(settings.Reload)
	set := settings.Default()
	set.BeaconTowerRelay = "relay.example.com:7474"
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
	k, err := remote.NewBeaconTowerKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.SaveBeaconTowerKey(remote.BeaconTowerKeyPath(), k); err != nil {
		t.Fatal(err)
	}
	return k
}

// beaconText is the code line a phone scans, as hylad derives it for k on
// the configuration's relay: the hyla://pair? URL of beaconCode.
func beaconText(t *testing.T, k *remote.BeaconTowerKey) string {
	t.Helper()
	q := url.Values{}
	q.Set("code", beaconCode(t, k))
	if name := remote.LocalName(); name != "" {
		q.Set("name", name)
	}
	return "hyla://pair?" + q.Encode()
}

// beaconCode is the bare BeaconTower code of k, which a QR code of the
// line carries.
func beaconCode(t *testing.T, k *remote.BeaconTowerKey) string {
	t.Helper()
	set := settings.Load()
	code, err := remote.BeaconTowerAddress(k, set.BeaconTowerRelay, set.BeaconTowerToken)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

// `hyla pocket beacon` prints the BeaconTower code of this machine's key
// and relay: the plain line hylad's listener gives, which a phone scans.
func TestPocketBeacon(t *testing.T) {
	k := pocketHome(t)
	if err := os.Remove(remote.BeaconTowerKeyPath()); err != nil {
		t.Fatal(err)
	}
	if out, err := stdout(t, func() error { return pocketCmd([]string{"beacon"}) }); err == nil || out != "" {
		t.Fatalf("beacon without a key printed %q, %v; want an error", out, err)
	}
	if err := remote.SaveBeaconTowerKey(remote.BeaconTowerKeyPath(), k); err != nil {
		t.Fatal(err)
	}

	// No listener: the code still, which no phone reaches yet.
	out, err := stdout(t, func() error { return pocketCmd([]string{"beacon"}) })
	if want := beaconText(t, k); err != nil || strings.TrimSpace(out) != want {
		t.Fatalf("pocket beacon = %q, %v; want %s", out, err, want)
	}

	// QR, as hyla pocket qr draws it: the same line, with the code drawn.
	out, err = stdout(t, func() error { return pocketBeacon(true) })
	if err != nil || !strings.Contains(out, "█") || !strings.Contains(out, beaconText(t, k)) {
		t.Fatalf("pocket beacon with a QR code = %q, %v", out, err)
	}

	// A listener of its own: the same line.
	enableBeaconListener(t)
	out, err = stdout(t, func() error { return pocketCmd([]string{"beacon"}) })
	if want := beaconText(t, k); err != nil || strings.TrimSpace(out) != want {
		t.Fatalf("pocket beacon = %q, %v; want %s", out, err, want)
	}

	// A relay that is not one is no code.
	set := settings.Load()
	set.BeaconTowerRelay = "ftp://relay.example.com"
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
	if out, err := stdout(t, func() error { return pocketCmd([]string{"beacon"}) }); err == nil || out != "" {
		t.Fatalf("beacon with no relay printed %q, %v; want an error", out, err)
	}
}

// `hyla pocket code` and `qr` print the BeaconTower code when hylad listens
// over BeaconTower and not over tailcat.
func TestPocketCodeBeaconTower(t *testing.T) {
	k := pocketHome(t)
	enableBeaconListener(t)
	out, err := stdout(t, func() error { return pocketCmd([]string{"code"}) })
	if want := beaconText(t, k); err != nil || strings.TrimSpace(out) != want {
		t.Fatalf("pocket code = %q, %v; want %s", out, err, want)
	}
	out, err = stdout(t, func() error { return pocketCmd([]string{"qr"}) })
	if want := beaconText(t, k); err != nil || !strings.Contains(out, "█") || !strings.Contains(out, want) {
		t.Fatalf("pocket qr = %q, %v", out, err)
	}
}

func enableBeaconListener(t *testing.T) {
	t.Helper()
	set := settings.Load()
	set.SetBeaconTowerListen(true)
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
}
