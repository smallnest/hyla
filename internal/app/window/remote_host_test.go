package window

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/app/link"
	"hyla/internal/beacontower"
	"hyla/internal/client"
	"hyla/internal/pocket"
	"hyla/internal/proto"
	"hyla/internal/remote"
	"hyla/internal/settings"
	"hyla/internal/ui/hosts"
	uisettings "hyla/internal/ui/settings"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/settings/kit"
	"hyla/locales"
)

// aRemoteHylad answers as another machine's hylad at a tcp:// listener:
// the password, then Hyla's own protocol. Its state has one session, of
// one pane.
func aRemoteHylad(t *testing.T, token string, st proto.State) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveRemoteHylad(c, token, st)
		}
	}()
	return ln.Addr().String()
}

func serveRemoteHylad(c net.Conn, token string, st proto.State) {
	defer c.Close()
	r := bufio.NewReaderSize(c, 64<<10)
	line, err := r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != token {
		return
	}
	hello, err := proto.ReadLine(r, 64<<10)
	if err != nil {
		return
	}
	var h proto.Hello
	if json.Unmarshal(hello, &h) != nil {
		return
	}
	json.NewEncoder(c).Encode(proto.Welcome{Version: proto.Version, PID: 1, Started: time.Now().UnixMilli(), App: "test"})
	if h.Kind == "attach" {
		// A pane's stream: the terminal's frames go the other way, and the
		// test needs only that it opened.
		io.Copy(io.Discard, r)
		return
	}
	for {
		line, err := proto.ReadLine(r, proto.MaxLine)
		if err != nil {
			return
		}
		var req proto.Request
		if json.Unmarshal(line, &req) != nil {
			return
		}
		var res any = map[string]any{}
		if req.Method == "state" {
			res = st
		}
		json.NewEncoder(c).Encode(map[string]any{"id": req.ID, "result": res})
	}
}

// remoteState is the state of the fake machine: a project with a session.
func remoteState() proto.State {
	return proto.State{
		Projects: []proto.Project{{ID: "rp", Name: "remote app", Dir: "/srv/app"}},
		Sessions: []proto.Session{{
			ID: "rs", Project: "rp", Name: "remote shell", Dir: "/srv/app",
			Panes:  []proto.Pane{{ID: "rpp", Dir: "/srv/app", Status: proto.Idle, Progress: -1}},
			Focus:  "rpp",
			Layout: nil,
		}},
	}
}

// A machine added by its address and password shows its sessions beside
// this one's, and a pane of it attaches through the same address.
func TestRemoteHost(t *testing.T) {
	a := testApp(t)
	addr := aRemoteHylad(t, "s3cret-pass", remoteState())
	if err := hosts.Add(a, "studio", "tcp://"+addr, "s3cret-pass", ""); err != nil {
		t.Fatal(err)
	}
	h := a.hostNamed("studio")
	if h == nil || len(a.Machines) != 2 {
		t.Fatalf("hosts %+v", a.Machines)
	}
	// The app connects, as its window's loop would: the test drives it.
	cl, err := client.ConnectVia(h.Dialer())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	link.Connected(a, h, h.Gen, cl, nil)
	var st proto.State
	if err := cl.Call("state", nil, &st); err != nil {
		t.Fatal(err)
	}
	// As setState does, without the badge the app's own loop takes: a test
	// has none.
	h.Loaded = true
	a.mergeState(h, st)
	if got := a.SessionsOf(h); len(got) != 1 || got[0].ID != "rs" {
		t.Fatalf("the machine holds %+v", got)
	}
	// Its session is the machine's, not this one's: what the app sends it
	// goes there, and its panes attach there.
	if a.hostOf("rs") != h || a.hostOf("rpp") != h || a.hostOf("rp") != h {
		t.Error("the session is not the machine's")
	}
	if a.hostOf("s1") == h {
		t.Error("a session of this machine is the machine's")
	}
	stream, err := client.AttachVia(h.Dialer(), "rpp")
	if err != nil {
		t.Fatal(err)
	}
	stream.Close()

	// The sidebar shows it under the machine's name.
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	for _, want := range []string{"studio", "remote app", "remote shell"} {
		if !tt.HasText(want) {
			t.Errorf("the sidebar shows no %q; texts %q", want, tt.Texts())
		}
	}
	// It goes as the user removes it.
	hosts.Remove(a, "studio")
	if len(a.Machines) != 1 || a.hostNamed("studio") != nil {
		t.Errorf("hosts %+v", a.Machines)
	}
	if a.hostOf("rs") != a.Local() {
		t.Error("a session of the machine stayed")
	}
}

// A machine that answers with another password, or none, keeps this one's
// sessions alone: nothing of it shows.
func TestRemoteHostWrongPassword(t *testing.T) {
	a := testApp(t)
	addr := aRemoteHylad(t, "s3cret-pass", remoteState())
	if err := hosts.Add(a, "studio", addr, "wrong", ""); err != nil {
		t.Fatal(err)
	}
	h := a.hostNamed("studio")
	if _, err := client.ConnectVia(h.Dialer()); err == nil {
		t.Fatal("a wrong password connected")
	}
	if got := a.SessionsOf(h); len(got) != 0 {
		t.Errorf("the machine holds %+v", got)
	}
}

// Settings › Tailcat lists the machines added, and takes another's
// address and password.
func TestRemoteHostsSettings(t *testing.T) {
	a := testApp(t)
	a.Prefs.RemoteHosts = []hosts.Remote{{Name: "studio", Addr: "192.168.1.5:7297", Token: "s3cret-pass"}}
	a.Ss.Page = "hosts"
	tt := ui.NewTester(uisettings.View(a), 820, 900)
	tt.Frame()
	for _, want := range []string{"studio", "192.168.1.5:7297"} {
		if !tt.HasText(want) {
			t.Errorf("Settings shows no %q; texts %q", want, tt.Texts())
		}
	}
}

// The form takes an address and a password, and names the machine for the
// address when the user does not.
func TestAddRemoteHost(t *testing.T) {
	a := testApp(t)
	if err := hosts.Add(a, "", "192.168.1.5:7297", "s3cret-pass", ""); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("192.168.1.5"); h == nil {
		t.Fatalf("hosts %+v", a.Prefs.RemoteHosts)
	}
	// A Tailcat machine comes as its pair code, or as the code a phone
	// scans, and its port is the listener's; Tailcat off reaches none.
	a.settingsIn.Tailcat = true
	k, err := remote.NewTailcatKey()
	if err != nil {
		t.Fatal(err)
	}
	k.Region = "1" // a region of its own: this test needs no relay
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	code, err := remote.TailcatAddress(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if err := hosts.Add(a, "relay", code, "", ""); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("relay"); h == nil || !h.TC || h.Code != code || h.Port == "" || h.Addr != "tailcat://"+code {
		t.Errorf("the pair code is %+v", h)
	}
	if err := hosts.Add(a, "", "hyla://pair?code="+code+"&name=scanned", "", "7400"); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("scanned"); h == nil || h.Code != code || h.Port != "7400" {
		t.Errorf("the scanned code is %+v", h)
	}
	// A whole code pasted in the address field takes its password.
	if err := hosts.Add(a, "code", "tcp://192.168.1.7:7297?token=s3cret-pass", "", ""); err != nil {
		t.Fatal(err)
	}
	if h := a.hostNamed("code"); h == nil || h.Addr != "192.168.1.7:7297" || h.Token != "s3cret-pass" {
		t.Errorf("the code is %+v", h)
	}
	for _, c := range []struct{ name, addr, token string }{
		{"studio", "192.168.1.5", "s3cret-pass"},
		{"studio", "192.168.1.5:7297", ""},
		{"192.168.1.5", "192.168.1.6:7297", "s3cret-pass"},
	} {
		if err := hosts.Add(a, c.name, c.addr, c.token, ""); err == nil {
			t.Errorf("%+v was taken", c)
		}
	}
}

// Settings › Hosts keeps this machine's two listeners apart: each form
// draws its own listener's port and secret, never the other's, and the
// tailcat switch off leaves the tcp listener as it was.
func TestHostsListenersApart(t *testing.T) {
	home, err := os.MkdirTemp("", "hyla-hosts-")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_CONFIG", filepath.Join(home, "config"))
	t.Cleanup(func() { os.RemoveAll(home) })
	a := testApp(t)
	a.Ss.Page = "hosts"
	settings.Reload()
	set := settings.Load()
	set.Listen = nil
	set.SetTCPListen("", "7401")
	set.SetTailcatListen("7402")
	set.ListenToken = "s3cret-pass"
	set.Tailcat = true
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
	a.applySettings(settings.Load())

	// The two forms hold their own listener's fields.
	hosts.ResetTailcat(a)
	hosts.ResetP2P(a)
	if a.Ss.Tailcat.ListenPort != "7402" || strings.Contains(a.Ss.Tailcat.ListenPort, "7401") {
		t.Errorf("the tailcat form holds port %q", a.Ss.Tailcat.ListenPort)
	}
	if a.Ss.P2P.Addr != ":7401" || a.Ss.P2P.Token != "s3cret-pass" {
		t.Errorf("the p2p form holds %+v", a.Ss.P2P)
	}

	// The page shows both, each with its own port and secret, and drawing
	// it changes no setting. The forms' fields are the drafts', which a
	// frame shows.
	kit.Reset()
	tt := ui.NewTester(uisettings.View(a), 820, 2600)
	tt.Frame()

	for _, want := range []string{"7401", "s3cret-pass"} {
		if !tt.HasText(want) {
			t.Errorf("Settings shows no %q; texts %q", want, tt.Texts())
		}
	}
	if got := settings.Load().Listen; !slices.Equal(got, []string{"tcp://:7401", "tailcat://:7402"}) {
		t.Errorf("drawing the page changes the listeners: %q", got)
	}

	// The tailcat switch off: the tailcat listener goes, and the tcp one
	// stays, in the settings, the file, and the P2P form.
	toggleTailcat(t, a)
	ap := settings.Load()
	if l, _, ok := ap.TailcatListen(); ok {
		t.Errorf("the tailcat switch off keeps %q of %q", l, ap.Listen)
	}
	if l, port, ok := ap.TCPListen(); !ok || l != "tcp://:7401" || port != "7401" {
		t.Errorf("the tailcat switch off changes the tcp listener: %q", ap.Listen)
	}
	hosts.ResetP2P(a)
	if a.Ss.P2P.Addr != ":7401" || a.Ss.P2P.Token != "s3cret-pass" {
		t.Errorf("the tailcat switch off empties the P2P form: %+v", a.Ss.P2P)
	}

	// The P2P switch off: the tcp listener goes, and the tailcat one
	// stays.
	toggleP2P(t, a)
	ap = settings.Load()
	if _, _, ok := ap.TCPListen(); ok {
		t.Errorf("the p2p switch off keeps %q", ap.Listen)
	}
	if l, port, ok := ap.TailcatListen(); ok {
		t.Errorf("the p2p switch off adds a tailcat listener: %q %q", l, port)
	}
	if ap.ListenToken != "s3cret-pass" {
		t.Errorf("the p2p switch off changes the password: %q", ap.ListenToken)
	}
}

// toggleTailcat does what the Tailcat switch does: it turns this machine's
// tailcat listener on, or off, and saves.
func toggleTailcat(t *testing.T, a *App) {
	t.Helper()
	s := settings.Load()
	if _, _, ok := s.TailcatListen(); ok {
		s.SetTailcatListen("")
	} else {
		s.SetTailcatListen(settings.TailcatListenPortDefault())
	}
	if err := settings.Save(s); err != nil {
		t.Fatal(err)
	}
	a.SetSettingsNow(settings.Load())
	hosts.ResetTailcat(a)
}

// toggleP2P does what the P2P switch does: it turns this machine's tcp
// listener on, or off, and saves.
func toggleP2P(t *testing.T, a *App) {
	t.Helper()
	s := settings.Load()
	if _, _, ok := s.TCPListen(); ok {
		s.SetTCPListen("", "")
	} else {
		s.SetTCPListen("", settings.TailcatListenPortDefault())
		if s.ListenToken == "" {
			s.ListenToken = "s3cret-pass"
		}
	}
	if err := settings.Save(s); err != nil {
		t.Fatal(err)
	}
	a.SetSettingsNow(settings.Load())
	hosts.ResetP2P(a)
}

// The question a page asks is the Settings window's own, over the page:
// the main window may stand behind Settings, where the question, and the
// news after it, would not be seen.
func TestTailcatCodeChangeAskedInSettings(t *testing.T) {
	a := testApp(t)
	s := a.SettingsNow().Clone()
	s.Tailcat = true
	s.SetTailcatListen("7473")
	a.SetSettingsNow(s)
	a.Ss.Page = "hosts"
	a.Ss.Tailcat = draft.TailcatDraft{ListenPort: "7473", Code: "tc" + strings.Repeat("x", 20), At: time.Now()}

	kit.Reset()
	tt := ui.NewTester(uisettings.View(a), 820, 2600)
	tt.Frame()
	if err := tt.Click(locales.SetTailcatChange()); err != nil {
		t.Fatalf("the change button: %v", err)
	}
	tt.Frame()
	if a.Ss.Confirm == nil || !a.Ss.Confirm.Open || a.Ss.Confirm.Title != locales.SetTailcatChange() {
		t.Fatalf("Settings asked %+v", a.Ss.Confirm)
	}
	if a.Confirm != nil {
		t.Errorf("the question went to the main window, behind Settings: %+v", a.Confirm)
	}
	// Answered or not, it shows over the page it was asked on.
	if !tt.HasText(locales.SetTailcatChange()) {
		t.Errorf("Settings draws no question; texts %q", tt.Texts())
	}

	// The news of a page is Settings' too.
	a.Ss.Confirm = nil
	hosts.RotateTailcatCode(a)
	if a.Ss.Toast == "" {
		t.Error("the news of a new pair code went to the main window")
	}
}

func serveRemoteE2EEHylad(c net.Conn, st proto.State) {
	defer c.Close()
	r := bufio.NewReaderSize(c, 64<<10)
	hello, err := proto.ReadLine(r, 64<<10)
	if err != nil {
		return
	}
	var h proto.Hello
	if json.Unmarshal(hello, &h) != nil {
		return
	}
	_ = json.NewEncoder(c).Encode(proto.Welcome{Version: proto.Version, PID: 1, Started: time.Now().UnixMilli(), App: "test"})
	if h.Kind == "attach" {
		io.Copy(io.Discard, r)
		return
	}
	for {
		line, err := proto.ReadLine(r, proto.MaxLine)
		if err != nil {
			return
		}
		var req proto.Request
		if json.Unmarshal(line, &req) != nil {
			return
		}
		var res any = map[string]any{}
		if req.Method == "state" {
			res = st
		}
		_ = json.NewEncoder(c).Encode(map[string]any{"id": req.ID, "result": res})
	}
}

func TestBeaconTowerRemoteHostEndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HYLA_HOME", home)
	t.Setenv("HYLA_CONFIG", filepath.Join(home, "config"))
	settings.Reload()
	t.Cleanup(func() { beacontower.DefaultPool.CloseAll() })

	srv := beacontower.NewServer(beacontower.ServerConfig{
		ListenAddr: "127.0.0.1:0",
		Token:      "relay-e2e-token",
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	relayAddr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	btRelay, err := remote.ListenBeaconTower(ctx, filepath.Join(home, "beacontower.json"), relayAddr, "relay-e2e-token")
	if err != nil {
		t.Fatal(err)
	}
	defer btRelay.Close()

	go func() {
		for {
			c, err := btRelay.Accept()
			if err != nil {
				return
			}
			go serveRemoteE2EEHylad(c, remoteState())
		}
	}()

	a := testApp(t)
	if err := hosts.Add(a, "beacon-box", btRelay.PairCode, "", ""); err != nil {
		t.Fatal(err)
	}
	h := a.hostNamed("beacon-box")
	if h == nil || !h.BT || h.TC || h.Code == "" {
		t.Fatalf("unexpected host %+v", h)
	}

	cl, err := client.ConnectVia(h.Dialer())
	if err != nil {
		t.Fatalf("ConnectVia BeaconTower: %v", err)
	}
	t.Cleanup(func() { cl.Close() })
	link.Connected(a, h, h.Gen, cl, nil)
	var st proto.State
	if err := cl.Call("state", nil, &st); err != nil {
		t.Fatal(err)
	}
	h.Loaded = true
	a.mergeState(h, st)
	if got := a.SessionsOf(h); len(got) != 1 || got[0].ID != "rs" {
		t.Fatalf("remote sessions = %+v", got)
	}

	stream, err := client.AttachVia(h.Dialer(), "rpp")
	if err != nil {
		t.Fatalf("AttachVia BeaconTower: %v", err)
	}
	defer stream.Close()

	// Both ConnectVia and AttachVia must reuse the single pooled E2EE relay session.
	if registered, sessions := srv.Stats(); registered != 1 || sessions != 1 {
		t.Fatalf("relay stats = (hosts=%d, sessions=%d); want (1, 1)", registered, sessions)
	}

	// Verify Settings › Remote Connection renders BeaconTower -> Tailcat -> P2P and independent listeners.
	set := settings.Load()
	set.BeaconTowerRelay = relayAddr
	set.BeaconTowerToken = "relay-e2e-token"
	set.SetBeaconTowerListen(true)
	set.SetTailcatListen("7402")
	set.SetTCPListen("", "7401")
	set.ListenToken = "s3cret-pass"
	if err := settings.Save(set); err != nil {
		t.Fatal(err)
	}
	a.applySettings(settings.Load())
	a.Ss.Page = "hosts"
	hosts.ResetBeaconTower(a)
	hosts.ResetTailcat(a)
	hosts.ResetP2P(a)

	kit.Reset()
	tt := ui.NewTester(uisettings.View(a), 820, 3000)
	tt.Frame()
	texts := tt.Texts()
	joined := strings.Join(texts, "\n")
	btIdx := strings.Index(joined, locales.SetBeaconTowerTitle())
	tcIdx := strings.Index(joined, locales.SetTailcatTitle())
	p2pIdx := strings.Index(joined, locales.SetTCPTitle())
	if btIdx < 0 || tcIdx < 0 || p2pIdx < 0 || !(btIdx < tcIdx && tcIdx < p2pIdx) {
		t.Fatalf("unexpected group order: bt=%d tc=%d p2p=%d\n%s", btIdx, tcIdx, p2pIdx, joined)
	}
	if !strings.Contains(joined, "beacontower://bt1_") {
		t.Fatalf("Settings missing rendered beacontower pair code:\n%s", joined)
	}
	if a.Ss.BeaconTower.QR == nil {
		t.Fatal("Settings missing rendered beacontower QR code")
	}

	// HylaPocket's hyla://pair?code=bt1_...&name=... QR format must also be
	// accepted when pasted into Desktop Hyla and preserve the encoded name.
	rawBT := strings.TrimPrefix(btRelay.PairCode, "beacontower://")
	pocketURL := pocket.Code{Pair: rawBT, Name: "phone-box"}.CodeURL()
	if _, err := pocket.ParseCodeURL(pocketURL); err != nil {
		t.Fatalf("pocket.ParseCodeURL(%q): %v", pocketURL, err)
	}
	if err := hosts.Add(a, "", pocketURL, "", ""); err != nil {
		t.Fatalf("hosts.Add(%q): %v", pocketURL, err)
	}
	if ph := a.hostNamed("phone-box"); ph == nil || !ph.BT || ph.Code != rawBT {
		t.Fatalf("phone-box host = %+v; want BT=true code=%q", ph, rawBT)
	}
	hosts.Remove(a, "phone-box")

	// Rotating BeaconTower code triggers toast in Settings and updates saved rotate flag.
	hosts.RotateBeaconTowerCode(a)
	if a.Ss.Toast == "" {
		t.Error("RotateBeaconTowerCode did not set Settings Toast")
	}

	hosts.Remove(a, "beacon-box")
	if a.hostNamed("beacon-box") != nil {
		t.Fatal("beacon-box was not removed")
	}
}

func TestBeaconTowerToggleWithInvalidRelayPreservesError(t *testing.T) {
	a := testApp(t)
	s := a.SettingsNow().Clone()
	s.BeaconTowerRelay = ""
	s.SetBeaconTowerListen(false)
	a.SetSettingsNow(s)
	a.Ss.Page = "hosts"
	hosts.ResetBeaconTower(a)
	a.Ss.BeaconTower.Relay = "http://invalid-relay"

	kit.Reset()
	tt := ui.NewTester(uisettings.View(a), 820, 3000)
	tt.Frame()
	r, ok := tt.Find(locales.SetBeaconTowerListen())
	if !ok {
		t.Fatalf("missing %q row in Settings", locales.SetBeaconTowerListen())
	}
	tt.ClickAt(752, r.Y+r.H/2)
	tt.Frame()
	tt.Frame()
	if a.Ss.BeaconTower.Relay != "http://invalid-relay" {
		t.Fatalf("draft relay was wiped: got %q", a.Ss.BeaconTower.Relay)
	}
	if a.Ss.BeaconTower.Err.Err == "" {
		t.Fatal("expected validation error on invalid relay draft after toggle")
	}
	if _, on := a.SettingsNow().BeaconTowerListen(); on {
		t.Fatal("listener unexpectedly enabled with invalid relay draft")
	}
}

func closedTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// Settings › Remote Connection shows each host's connection status across
// BeaconTower, Tailcat, and P2P, and Disconnect / Connect buttons update
// both live host state and persisted preferences.
func TestRemoteHostStatusAndActionsInSettings(t *testing.T) {
	a := testApp(t)
	a.Ss.Page = "hosts"

	// Make sample valid codes for BeaconTower and Tailcat so all three
	// transport groups render in Settings.
	kp, err := beacontower.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	btCode, err := beacontower.EncodePairCode(beacontower.PairCode{
		Relay:   "relay.example.com:7474",
		HostPub: kp.Pub,
		PSK:     kp.PSK,
	})
	if err != nil {
		t.Fatal(err)
	}
	k, err := remote.NewTailcatKey()
	if err != nil {
		t.Fatal(err)
	}
	k.Region = "1"
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	tcCode, err := remote.TailcatAddress(ctx, k)
	if err != nil {
		t.Fatal(err)
	}

	deadAddr := closedTCPAddr(t)
	a.Prefs.RemoteHosts = []hosts.Remote{
		{Name: "bt-ready", Addr: "beacontower://" + btCode},
		{Name: "bt-disc", Addr: "beacontower://" + btCode, Disconnected: true},
		{Name: "tc-conn", Addr: "tailcat://" + tcCode, Port: "7473"},
		{Name: "tc-ver", Addr: "tailcat://" + tcCode, Port: "7473"},
		{Name: "p2p-fail", Addr: deadAddr, Token: "pass"},
	}
	a.Machines = append(a.Machines,
		&host.Host{Name: "bt-ready", Addr: "beacontower://" + btCode, BT: true, Code: btCode, Conn: host.StateReady, Loaded: true},
		&host.Host{Name: "bt-disc", Addr: "beacontower://" + btCode, BT: true, Code: btCode, Conn: host.StateDisconnected},
		&host.Host{Name: "tc-conn", Addr: "tailcat://" + tcCode, TC: true, Code: tcCode, Port: "7473", Conn: host.StateConnecting},
		&host.Host{Name: "tc-ver", Addr: "tailcat://" + tcCode, TC: true, Code: tcCode, Port: "7473", Conn: host.StateVersion},
		&host.Host{Name: "p2p-fail", Addr: deadAddr, Token: "pass", Conn: host.StateFailed, Err: "dial tcp: refused"},
	)

	kit.Reset()
	tt := ui.NewTester(uisettings.View(a), 820, 3200)
	tt.Frame()
	for _, want := range []string{
		"bt-ready", locales.HostStatusConnected(),
		"bt-disc", locales.StatusDisconnected(),
		"tc-conn", locales.StatusConnecting(),
		"tc-ver", locales.HostStatusVersion(), locales.HostVersionMismatchDetail(),
		"p2p-fail", locales.StatusFailed(), "dial tcp: refused",
		locales.HostDisconnect(), locales.CommonConnect(),
	} {
		if !tt.HasText(want) {
			t.Errorf("Settings missing %q; texts %q", want, tt.Texts())
		}
	}

	// Single-host Settings interaction: Disconnect -> Disconnected -> Connect -> Connecting…
	a2 := testApp(t)
	a2.Ss.Page = "hosts"
	addr := aRemoteHylad(t, "s3cret-pass", remoteState())
	if err := hosts.Add(a2, "studio", "tcp://"+addr, "s3cret-pass", ""); err != nil {
		t.Fatal(err)
	}
	h := a2.hostNamed("studio")
	cl, err := client.ConnectVia(h.Dialer())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	link.Connected(a2, h, h.Gen, cl, nil)
	h.Loaded = true
	a2.mergeState(h, remoteState())

	kit.Reset()
	tt2 := ui.NewTester(uisettings.View(a2), 820, 2600)
	tt2.Frame()
	if !tt2.HasText(locales.HostStatusConnected()) {
		t.Fatalf("expected Connected status in Settings; texts %q", tt2.Texts())
	}
	if err := tt2.Click(locales.HostDisconnect()); err != nil {
		t.Fatalf("click Disconnect: %v", err)
	}
	tt2.Frame()
	tt2.Frame()
	if h.Conn != host.StateDisconnected || h.Client != nil {
		t.Fatalf("host after Disconnect = (conn=%v, client=%v); want StateDisconnected, nil", h.Conn, h.Client)
	}
	if got := a2.SessionsOf(h); len(got) != 0 {
		t.Fatalf("expected remote sessions cleared after Disconnect, got %+v", got)
	}
	if !a2.Prefs.RemoteHosts[0].Disconnected {
		t.Fatal("expected Disconnected=true persisted in RemoteHosts")
	}
	if !tt2.HasText(locales.StatusDisconnected()) || !tt2.HasText(locales.CommonConnect()) {
		t.Fatalf("Settings did not switch to Disconnected/Connect; texts %q", tt2.Texts())
	}

	if err := tt2.Click(locales.CommonConnect()); err != nil {
		t.Fatalf("click Connect: %v", err)
	}
	tt2.Frame()
	tt2.Frame()
	if h.Conn != host.StateConnecting {
		t.Fatalf("host.Conn after Connect = %v; want StateConnecting", h.Conn)
	}
	if a2.Prefs.RemoteHosts[0].Disconnected {
		t.Fatal("expected Disconnected=false persisted after Connect")
	}
	if !tt2.HasText(locales.StatusConnecting()) || !tt2.HasText(locales.HostDisconnect()) {
		t.Fatalf("Settings did not switch to Connecting/Disconnect; texts %q", tt2.Texts())
	}

	// ConnectName on a RemoteHost configured before Sync runs creates and connects the host.
	a3 := testApp(t)
	a3.Prefs.RemoteHosts = []hosts.Remote{{Name: "pre", Addr: closedTCPAddr(t), Token: "p", Disconnected: true}}
	hosts.ConnectName(a3, "pre")
	if h3 := a3.hostNamed("pre"); h3 == nil || h3.Conn != host.StateConnecting {
		t.Fatalf("ConnectName before Sync = %+v; want StateConnecting", h3)
	}
	if a3.Prefs.RemoteHosts[0].Disconnected {
		t.Fatal("ConnectName did not clear Disconnected flag")
	}
}

// Disconnecting a host cancels/ignores in-flight dials, persists across Sync
// (e.g. app restarts), and allows reconnecting cleanly afterwards.
func TestRemoteHostDisconnectPersistenceAndStaleConnect(t *testing.T) {
	a := testApp(t)
	addr := aRemoteHylad(t, "s3cret-pass", remoteState())
	if err := hosts.Add(a, "studio", "tcp://"+addr, "s3cret-pass", ""); err != nil {
		t.Fatal(err)
	}
	h := a.hostNamed("studio")
	oldGen := h.Gen

	// Dial a client that simulates an in-flight connection attempt from before Disconnect.
	staleCl, err := client.ConnectVia(h.Dialer())
	if err != nil {
		t.Fatal(err)
	}
	defer staleCl.Close()

	// Connect normally first so sessions and statuses exist.
	cl, err := client.ConnectVia(h.Dialer())
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	link.Connected(a, h, h.Gen, cl, nil)
	h.Loaded = true
	a.mergeState(h, remoteState())
	a.Statuses["rs"] = proto.Working

	// Disconnect clears sessions, statuses, and client, and persists Disconnected=true.
	hosts.Disconnect(a, h)
	if h.Conn != host.StateDisconnected || h.Client != nil || h.Loaded {
		t.Fatalf("after Disconnect: conn=%v client=%v loaded=%v", h.Conn, h.Client, h.Loaded)
	}
	if len(a.SessionsOf(h)) != 0 {
		t.Fatalf("sessions not cleared: %+v", a.SessionsOf(h))
	}
	if _, ok := a.Statuses["rs"]; ok {
		t.Fatal("stale session status not forgotten on Disconnect")
	}
	if !a.Prefs.RemoteHosts[0].Disconnected {
		t.Fatal("Disconnected flag not saved in Prefs")
	}

	// An earlier in-flight dial finishing after Disconnect must be rejected and closed.
	link.Connected(a, h, oldGen, staleCl, nil)
	if h.Conn != host.StateDisconnected || h.Client != nil {
		t.Fatalf("stale Connected overwrote disconnected state: conn=%v client=%v", h.Conn, h.Client)
	}
	select {
	case <-staleCl.Done():
	case <-time.After(time.Second):
		t.Fatal("stale client was not closed after Disconnect")
	}

	// Simulated app restart: Sync recreates studio in StateDisconnected without dialing.
	a.Machines = []*host.Host{a.Local()}
	hosts.Sync(a)
	h = a.hostNamed("studio")
	if h == nil || h.Conn != host.StateDisconnected || h.Dialing {
		t.Fatalf("after Sync restart: host = %+v; want StateDisconnected and Dialing=false", h)
	}

	// Connecting again clears Disconnected and succeeds against the remote hylad.
	hosts.Connect(a, h)
	if h.Conn != host.StateConnecting || a.Prefs.RemoteHosts[0].Disconnected {
		t.Fatalf("after Connect: conn=%v prefsDisconnected=%v", h.Conn, a.Prefs.RemoteHosts[0].Disconnected)
	}
	cl2, err := client.ConnectVia(h.Dialer())
	if err != nil {
		t.Fatal(err)
	}
	defer cl2.Close()
	link.Connected(a, h, h.Gen, cl2, nil)
	if h.Conn != host.StateReady || h.Client != cl2 {
		t.Fatalf("reconnected host = (conn=%v, client=%v); want StateReady", h.Conn, h.Client)
	}
}

// Sidebar host headers render status badges/tooltips and expose Connect,
// Reconnect, Disconnect, and Remove actions via context menu and hover buttons.
func TestRemoteHostSidebarStatusAndMenu(t *testing.T) {
	a := testApp(t)
	addr := aRemoteHylad(t, "s3cret-pass", remoteState())
	if err := hosts.Add(a, "studio", "tcp://"+addr, "s3cret-pass", ""); err != nil {
		t.Fatal(err)
	}
	h := a.hostNamed("studio")
	cl, err := client.ConnectVia(h.Dialer())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	link.Connected(a, h, h.Gen, cl, nil)
	h.Loaded = true
	a.mergeState(h, remoteState())

	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if !tt.HasText("studio") || !tt.HasText("remote shell") {
		t.Fatalf("sidebar missing connected remote host; texts %q", tt.Texts())
	}

	// Hovering the connected host header reveals the Disconnect icon button.
	r, ok := tt.Find("studio")
	if !ok {
		t.Fatal("missing studio header")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if _, ok := tt.Find(locales.HostDisconnect()); !ok {
		t.Fatalf("hovering connected host did not show Disconnect button; texts %q", tt.Texts())
	}

	// Move away to restore clean non-hover state, then open context menu and Disconnect.
	tt.Move(0, 0)
	tt.Frame()
	if err := tt.RightClick("studio"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	wantReadyMenu := []string{locales.HostReconnect(), locales.HostDisconnect(), "-", locales.HostRemove()}
	if got := tt.Menu(); !reflect.DeepEqual(got, wantReadyMenu) {
		t.Fatalf("connected host menu = %q, want %q", got, wantReadyMenu)
	}
	if err := tt.ChooseMenuItem(locales.HostDisconnect()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if h.Conn != host.StateDisconnected || tt.HasText("remote shell") || !tt.HasText(locales.StatusDisconnected()) {
		t.Fatalf("sidebar after Disconnect: conn=%v texts=%q", h.Conn, tt.Texts())
	}

	// Hovering the disconnected host header reveals the Connect icon button; clicking it connects.
	r, ok = tt.Find("studio")
	if !ok {
		t.Fatal("missing disconnected studio header")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if err := tt.Click(locales.CommonConnect()); err != nil {
		t.Fatalf("click hover Connect button: %v", err)
	}
	tt.Frame()
	tt.Frame()
	if h.Conn != host.StateConnecting || a.Prefs.RemoteHosts[0].Disconnected {
		t.Fatalf("hover Connect did not reconnect: conn=%v prefsDisconnected=%v", h.Conn, a.Prefs.RemoteHosts[0].Disconnected)
	}

	// Disconnected host context menu has Connect and Remove.
	hosts.Disconnect(a, h)
	tt.Move(0, 0)
	tt.Frame()
	if err := tt.RightClick("studio"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	wantDiscMenu := []string{locales.CommonConnect(), "-", locales.HostRemove()}
	if got := tt.Menu(); !reflect.DeepEqual(got, wantDiscMenu) {
		t.Fatalf("disconnected host menu = %q, want %q", got, wantDiscMenu)
	}
	if err := tt.ChooseMenuItem(locales.CommonConnect()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if h.Conn != host.StateConnecting {
		t.Fatalf("menu Connect did not start connecting: conn=%v", h.Conn)
	}

	// Failed host shows status & error when not hovered, Connect when hovered,
	// and offers Connect + Disconnect in menu.
	h.Conn = host.StateFailed
	h.Err = "connection timed out"
	tt.Move(0, 0)
	tt.Frame()
	if !tt.HasText(locales.StatusFailed()) || !tt.HasText("connection timed out") {
		t.Fatalf("failed host missing status/error; texts %q", tt.Texts())
	}
	r, ok = tt.Find("studio")
	if !ok {
		t.Fatal("missing failed studio header")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if _, ok := tt.Find(locales.CommonConnect()); !ok {
		t.Fatalf("hovering failed host did not show Connect button; texts %q", tt.Texts())
	}
	tt.Move(0, 0)
	tt.Frame()
	if err := tt.RightClick("studio"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	wantFailMenu := []string{locales.CommonConnect(), locales.HostDisconnect(), "-", locales.HostRemove()}
	if got := tt.Menu(); !reflect.DeepEqual(got, wantFailMenu) {
		t.Fatalf("failed host menu = %q, want %q", got, wantFailMenu)
	}
	if err := tt.ChooseMenuItem(locales.HostDisconnect()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if h.Conn != host.StateDisconnected || tt.HasText("connection timed out") {
		t.Fatalf("failed host after Disconnect: conn=%v texts=%q", h.Conn, tt.Texts())
	}
}
