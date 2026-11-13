package settings

import (
	"strings"
	"testing"
)

// The tailcat keys read as the configuration writes them, and a listener
// needs Tailcat on.
func TestTailcatConfig(t *testing.T) {
	file := "tailcat = true\ntailcat-relay = https://relay.example/derpmap.json\n"
	s, probs := parse(file)
	if len(probs) > 0 {
		t.Fatalf("problems %q", probs)
	}
	if !s.Tailcat || s.TailcatRelay != "https://relay.example/derpmap.json" {
		t.Errorf("settings %+v", s)
	}
	out := format(s, file)
	for _, line := range []string{"tailcat = true", "tailcat-relay = https://relay.example/derpmap.json"} {
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
	// A bad relay keeps none of it, and a key of the hosts Hyla used to
	// add is no setting at all.
	for _, bad := range []string{"tailcat-relay = ftp://x\n", "tailcat-host = box=tailcat://x\n", "host = box=me@devbox\n"} {
		if _, probs := parse(bad); len(probs) == 0 {
			t.Errorf("%q read", bad)
		}
	}
}

// SetTailcatListen gives the settings one tailcat listener, on the port it
// names, and none for "".
func TestTailcatSetListen(t *testing.T) {
	s := Default()
	s.SetTailcatListen("7400")
	s.SetTailcatListen("7401")
	l, port, ok := s.TailcatListen()
	if !ok || l != "tailcat://:7401" || port != "7401" || len(s.Listen) != 1 || s.Listen[0] != "tailcat://:7401" {
		t.Fatalf("listeners %q", s.Listen)
	}
	s.SetTailcatListen("")
	if _, _, ok := s.TailcatListen(); ok || len(s.Listen) != 0 {
		t.Fatalf("listeners %q", s.Listen)
	}
}
