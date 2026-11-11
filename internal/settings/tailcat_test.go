package settings

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hyla/internal/remote"
)

// The tailcat keys read as the configuration writes them, and a listener
// needs Tailcat on.
func TestTailcatConfig(t *testing.T) {
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
	file := "tailcat = true\ntailcat-host = box=tailcat://" + code + "\ntailcat-port = 7400\ntailcat-relay = https://relay.example/derpmap.json\n"
	s, probs := parse(file)
	if len(probs) > 0 {
		t.Fatalf("problems %q", probs)
	}
	if !s.Tailcat || len(s.TailcatHosts) != 1 || s.TailcatHosts[0].Name != "box" || s.TailcatPort != "7400" || s.TailcatRelay != "https://relay.example/derpmap.json" {
		t.Errorf("settings %+v", s)
	}
	out := format(s, file)
	for _, line := range []string{"tailcat = true", "tailcat-host = box=tailcat://" + code, "tailcat-port = 7400", "tailcat-relay = https://relay.example/derpmap.json"} {
		if !strings.Contains(out, line+"\n") {
			t.Errorf("formatted without %q:\n%s", line, out)
		}
	}
	// Off, a tailcat listener is a problem, and is not kept.
	if _, probs := parse("listen = tailcat://:7473\n"); len(probs) == 0 {
		t.Error("a tailcat listener read with Tailcat off")
	}
	// Also when listen comes before tailcat: the listener is then kept.
	if s, probs := parse("listen = tailcat://:7473\ntailcat = true\n"); len(probs) > 0 || len(s.Listen) != 1 {
		t.Errorf("listen before tailcat: listeners %q, problems %q", s.Listen, probs)
	}
	// A bad host, port or relay keeps none of them.
	for _, bad := range []string{"tailcat-host = box=devbox\n", "tailcat-port = http\n", "tailcat-relay = ftp://x\n"} {
		if _, probs := parse(bad); len(probs) == 0 {
			t.Errorf("%q read", bad)
		}
	}
}

// The switch turned off stays off: a host is no switch of its own, and a
// file of hosts alone asks for the switch, so off is written.
func TestTailcatSwitchOff(t *testing.T) {
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
	// A file of hosts from before the switch asked for it.
	host := "tailcat-host = box=tailcat://" + code + "\n"
	if s, probs := parse(host); len(probs) > 0 || !s.Tailcat || len(s.TailcatHosts) != 1 {
		t.Errorf("a file from before the switch: Tailcat %v, hosts %+v, problems %q", s.Tailcat, s.TailcatHosts, probs)
	}
	// A file that names the switch says which way it wants it, whichever
	// of its lines comes first.
	for _, off := range []string{"tailcat = false\n" + host, host + "tailcat = false\n"} {
		if s, probs := parse(off); len(probs) > 0 || s.Tailcat || len(s.TailcatHosts) != 1 {
			t.Errorf("%q: Tailcat %v, hosts %+v, problems %q", off, s.Tailcat, s.TailcatHosts, probs)
		}
	}
	// Off, saved, stays off: the file says the switch is off, which one of
	// hosts alone cannot.
	path := filepath.Join(t.TempDir(), "config")
	t.Setenv("HYLA_CONFIG", path)
	if err := os.WriteFile(path, []byte(host), 0o600); err != nil {
		t.Fatal(err)
	}
	Reload()
	s := Load()
	if !s.Tailcat {
		t.Fatal("a file of hosts alone reads Tailcat off")
	}
	s.Tailcat = false
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	Reload()
	if back := Load(); back.Tailcat || len(back.TailcatHosts) != 1 {
		t.Fatalf("saved off, reads Tailcat %v, hosts %+v", back.Tailcat, back.TailcatHosts)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "tailcat = false\n") {
		t.Errorf("the file says nothing of the switch:\n%s", out)
	}
	// Back on, the host is listed, and the switch says so.
	s.Tailcat = true
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	Reload()
	if back := Load(); !back.Tailcat || len(back.TailcatHosts) != 1 {
		t.Fatalf("saved on, reads Tailcat %v, hosts %+v", back.Tailcat, back.TailcatHosts)
	}
}

// SetTailcatListen gives the settings one tailcat listener, on the port it
// names, and none for "".
func TestTailcatSetListen(t *testing.T) {
	s := Default()
	s.Listen = []string{"tailnet://:7473"}
	if _, _, ok := s.TailcatListen(); ok {
		t.Fatal("a tailcat listener without one")
	}
	s.SetTailcatListen("7400")
	s.SetTailcatListen("7401")
	l, port, ok := s.TailcatListen()
	if !ok || l != "tailcat://:7401" || port != "7401" || len(s.Listen) != 2 || s.Listen[1] != "tailcat://:7401" {
		t.Fatalf("listeners %q", s.Listen)
	}
	s.SetTailcatListen("")
	if _, _, ok := s.TailcatListen(); ok || len(s.Listen) != 1 || s.Listen[0] != "tailnet://:7473" {
		t.Fatalf("listeners %q", s.Listen)
	}
}
