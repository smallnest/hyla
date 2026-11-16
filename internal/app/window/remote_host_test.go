package window

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/link"
	"hyla/internal/client"
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
