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
	// Tailcat is on unless a file turns it off: a tailcat listener needs
	// nothing else of the file.
	if got, probs := parse("listen = tailcat://:7473\n"); len(probs) > 0 || len(got.Listen) != 1 {
		t.Errorf("a tailcat listener: listeners %q, problems %q", got.Listen, probs)
	}
	if got, probs := parse("tailcat = false\nlisten = tailcat://:7473\n"); len(probs) > 0 || got.Tailcat {
		t.Errorf("a file that turns Tailcat off: Tailcat %v, problems %q", got.Tailcat, probs)
	}
	// A tcp listener is the network's, and takes the password it names.
	if s, probs := parse("tailcat = true\nlisten = tcp://:7297\nlisten-token = s3cret-pass\n"); len(probs) > 0 || len(s.Listen) != 1 || s.ListenToken != "s3cret-pass" {
		t.Errorf("a tcp listener: listeners %q, token %q, problems %q", s.Listen, s.ListenToken, probs)
	}
	if _, probs := parse("listen = tcp://:7297\n"); len(probs) == 0 {
		t.Error("a tcp listener without a password read")
	}
	if _, probs := parse("listen-token = short\n"); len(probs) == 0 {
		t.Error("a password of four characters read")
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
