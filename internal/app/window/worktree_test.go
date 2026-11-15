package window

import (
	"encoding/json"
	"testing"

	"hyla/internal/app/host"
)

// A new session in a worktree asks its branch, of the project of the
// session the user is in, whose folder the session starts in.
func TestNewSessionWorktreeAsks(t *testing.T) {
	a := testApp(t)
	cl, stop := projectClient(t, func(string, json.RawMessage) (any, error) { return nil, nil })
	defer stop()
	a.Local().Conn, a.Local().Client = host.StateReady, cl
	a.Selected = "s1"
	a.run("session.newWorktree", nil)
	w := a.WtAsk
	if w == nil || !w.Open {
		t.Fatalf("no ask: %+v", w)
	}
	if w.Host != a.Local() {
		t.Fatalf("the ask is of %v", w.Host.Label())
	}
	// The project of the session the user is in, and its folder.
	if w.P.Project != "p1" || w.P.After != "s1" || w.P.Dir != "/tmp" {
		t.Fatalf("the ask is %+v", w.P)
	}
}

// A machine that does not connect asks nothing: it says so.
func TestNewSessionWorktreeNotConnected(t *testing.T) {
	a := testApp(t)
	a.PendingToast = ""
	a.run("session.newWorktree", nil)
	if a.WtAsk != nil {
		t.Fatalf("an ask of a machine that does not connect: %+v", a.WtAsk)
	}
	if a.PendingToast == "" {
		t.Fatal("no toast")
	}
}
