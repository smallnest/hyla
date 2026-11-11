package main

import (
	"context"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"hyla/internal/remote"
	"hyla/internal/settings"

	"hyla/locales"
)

// The Tailcat switch governs the tailcat hosts, off, and this machine's
// listener with them.
func TestTailcatSwitch(t *testing.T) {
	a := testApp(t)
	s := a.set.Clone()
	s.Tailcat = true
	s.SetTailcatListen("7473")
	a.set = s
	a.resetTailcat()
	if !a.set.Tailcat || !hasTailcatListener(&a.set) {
		t.Fatalf("on: Tailcat %v, listen %q", a.set.Tailcat, a.set.Listen)
	}
	a.setTailcat(false)
	if a.set.Tailcat || hasTailcatListener(&a.set) {
		t.Fatalf("off: Tailcat %v, listen %q", a.set.Tailcat, a.set.Listen)
	}
	a.setTailcat(true)
	if !a.set.Tailcat || hasTailcatListener(&a.set) {
		t.Fatalf("on again: Tailcat %v, listen %q", a.set.Tailcat, a.set.Listen)
	}
}

// A host the configuration names asks for no switch of its own: turned
// off, the app saves it off, and lists none of its hosts.
func TestTailcatSwitchOffSaved(t *testing.T) {
	k, err := remote.NewTailcatKey()
	if err != nil {
		t.Fatal(err)
	}
	// The code names a region of its own: this test needs no relay.
	k.Region = "1"
	code, err := remote.TailcatAddress(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	s := a.set.Clone()
	s.Tailcat = true
	// As a configuration line makes one: no Added flag.
	s.TailcatHosts = []settings.TailcatHost{{Name: "box", Addr: "tailcat://" + code}}
	s.SetTailcatListen("7473")
	a.set = s
	a.resetTailcat()
	if err := settings.Save(a.set); err != nil {
		t.Fatal(err)
	}
	a.setTailcat(false)
	// The settings view saves what the switch set, as it does each frame.
	a.applySettings(a.set)
	settings.Reload()
	if back := settings.Load(); back.Tailcat || len(back.TailcatHosts) != 1 {
		t.Fatalf("saved off, reads Tailcat %v, hosts %+v", back.Tailcat, back.TailcatHosts)
	}
	a.syncHosts()
	if a.hostNamed("box") != nil {
		t.Fatal("a tailcat host listed with the switch off")
	}
}

// A tailcat host reads from a pair code, and lists with the hosts.
func TestTailcatAddHost(t *testing.T) {
	a := testApp(t)
	if err := addTailcatHost(&a.set, "box", "not a code"); err == nil {
		t.Fatal("a bad code added")
	}
	if err := addTailcatHost(&a.set, "box", "devbox"); err == nil {
		t.Fatal("an ssh address added as a tailcat host")
	}
	if !validPairCodeForTest(t) {
		t.Skip("no pair code")
	}
	s := settings.Default()
	if err := addTailcatHost(&s, "box", "tailcat://"+testCode); err != nil {
		t.Fatal(err)
	}
	if !s.Tailcat || len(s.TailcatHosts) != 1 || s.TailcatHosts[0].Addr != "tailcat://"+testCode || !s.TailcatHosts[0].Added {
		t.Fatalf("settings %+v", s.TailcatHosts)
	}
	// Its code is a secret: the host's name is what shows.
	a2, err := remote.ParseAddr(s.TailcatHosts[0].Addr)
	if err != nil || a2.Label() == s.TailcatHosts[0].Addr {
		t.Errorf("label %q", a2.Label())
	}
}

// Off, the app lists no tailcat host, and lists it again on.
func TestTailcatHostsListed(t *testing.T) {
	a := testApp(t)
	if !validPairCodeForTest(t) {
		t.Skip("no pair code")
	}
	s := a.set.Clone()
	s.TailcatHosts = []settings.TailcatHost{{Name: "box", Addr: "tailcat://" + testCode, Added: true}}
	a.set = s
	a.syncHosts()
	if a.hostNamed("box") != nil {
		t.Fatal("a tailcat host listed with Tailcat off")
	}
	s = a.set.Clone()
	s.Tailcat = true
	a.set = s
	a.syncHosts()
	h := a.hostNamed("box")
	if h == nil || h.addr.Scheme != "tailcat" || !h.tc {
		t.Fatalf("host %+v", h)
	}
	s = a.set.Clone()
	s.Tailcat = false
	a.set = s
	a.syncHosts()
	if a.hostNamed("box") != nil {
		t.Fatal("a tailcat host listed with Tailcat off")
	}
}

// The form's fields check what they keep, and say why under themselves.
func TestTailcatFields(t *testing.T) {
	a := testApp(t)
	s := a.set.Clone()
	s.Tailcat = true
	a.set, a.ss.tailcat = s, tailcatDraft{port: "80", relay: "https://relay.example/derpmap.json"}
	a.resetTailcat()
	d := &a.ss.tailcat
	d.port, d.relay = "http", "ftp://x"
	d.save(a, "tailcat-port")
	if a.set.TailcatPort == "http" || d.err.key != "tailcat-port" {
		t.Fatalf("port %q, err %+v", a.set.TailcatPort, d.err)
	}
	d.port = "80"
	d.save(a, "tailcat-port")
	if a.set.TailcatPort != "80" || d.err.key != "" {
		t.Fatalf("port %q, err %+v", a.set.TailcatPort, d.err)
	}
	d.relay = "ftp://x"
	d.save(a, "tailcat-relay")
	if a.set.TailcatRelay == "ftp://x" || d.err.key != "tailcat-relay" {
		t.Fatalf("relay %q, err %+v", a.set.TailcatRelay, d.err)
	}
	// This machine as a host: a port of its own, which its listen names.
	d.listenPort = "7400"
	d.save(a, "tailcat-listen-port")
	if d.err.key != "" || !hasTailcatListener(&a.set) {
		t.Fatalf("listen %q, err %+v", a.set.Listen, d.err)
	}
	if _, port, _ := a.set.TailcatListen(); port != "7400" {
		t.Fatalf("listen %q", a.set.Listen)
	}
}

// The Hosts page shows one Tailcat group: off, the switch and how to keep
// it off, on, the fields of the hosts Hyla adds, the relay, and this
// machine as a host. HYLA_SHOTS=dir writes the look.
func TestTailcatSettingRows(t *testing.T) {
	a := testApp(t)
	a.ss.draft, a.ss.host = map[string]string{}, hostDraft{}
	s := a.set.Clone()
	s.Tailcat = false
	a.set = s
	a.resetTailcat()
	a.ss.page = "hosts"
	tt := ui.NewTester(a.settingsView, 820, 1500)
	tt.Frame()
	shot(t, tt, "settings-tailcat-off")
	for _, want := range []string{locales.SetTailcat(), locales.SetTailcatOffNote()} {
		if !tt.HasText(want) {
			t.Errorf("off: no %q", want)
		}
	}
	if tt.HasText(locales.SetTailcatRelay()) {
		t.Errorf("off: the relay lists: %q", tt.Texts())
	}

	s = a.set.Clone()
	s.Tailcat = true
	s.SetTailcatListen("")
	a.set, a.ss.tailcat = s, tailcatDraft{}
	tt.Frame()
	if !tt.HasText(locales.SetTailcat()) || tt.HasText(locales.SetTailcatOffNote()) {
		t.Errorf("no switch, or a note to keep it off: %q", tt.Texts())
	}
	for _, want := range []string{locales.SetTailcatAdd(), locales.SetTailcatPort(), locales.SetTailcatRelay(), locales.SetTailcatListen()} {
		if !tt.HasText(want) {
			t.Errorf("no %q; texts %q", want, tt.Texts())
		}
	}
	if tt.HasText(locales.SetTailcatListenPort()) {
		t.Errorf("a listener's port without a listener: %q", tt.Texts())
	}
	s = a.set.Clone()
	s.SetTailcatListen("7473")
	a.set, a.ss.tailcat = s, tailcatDraft{code: "tailcat://" + testCode}
	tt.Frame()
	for _, want := range []string{locales.SetTailcatListenPort(), locales.SetTailcatCode(), locales.SetTailcatCodeDetail(), locales.SetTailcatChange()} {
		if !tt.HasText(want) {
			t.Errorf("no %q; texts %q", want, tt.Texts())
		}
	}
	shot(t, tt, "settings-tailcat")
}

// testCode is a pair code of a region of the relay map, as the app's
// hosts take: "" when there is none.
var testCode = func() string {
	k, err := remote.NewTailcatKey()
	if err != nil {
		return ""
	}
	code, err := remote.TailcatAddress(context.Background(), k)
	if err != nil {
		return ""
	}
	return code
}()

func validPairCodeForTest(t *testing.T) bool { return testCode != "" }

// Changing this machine's pair code keeps its listener, for the key's new
// code, which hylad starts its listener again for.
func TestTailcatRotate(t *testing.T) {
	a := testApp(t)
	old, err := remote.LocalTailcatAddress(context.Background())
	if err != nil {
		t.Skip("no relay: ", err)
	}
	s := a.set.Clone()
	s.Tailcat = true
	s.SetTailcatListen("7473")
	rot := s.TailcatRotate
	a.set, a.ss.tailcat = s, tailcatDraft{}
	a.rotateTailcatCode()
	if a.set.TailcatRotate == rot {
		t.Error("hylad told not to start its listener again")
	}
	if l, port, ok := a.set.TailcatListen(); !ok || l != "tailcat://:7473" || port != "7473" {
		t.Fatalf("listen %q", a.set.Listen)
	}
	if a.pendingToast == "" {
		t.Error("no toast")
	}
	code, err := remote.LocalTailcatAddress(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if code == old {
		t.Error("the pair code stays")
	}
	if st, err := os.Stat(remote.TailcatKeyPath()); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file %v, %v", st, err)
	}
}
