package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"hyla/internal/proto"
)

func TestCanForkSession(t *testing.T) {
	cases := []struct {
		name string
		pane *proto.Pane
		want bool
	}{
		{"nil pane", nil, false},
		{"no agent session", &proto.Pane{ID: "p1", Status: proto.Idle}, false},
		{"exited pane", &proto.Pane{ID: "p1", Status: proto.Exited, AgentSession: &proto.AgentSession{Agent: "claude", ID: "sess-1"}}, false},
		{"browser pane", &proto.Pane{ID: "p1", URL: "https://example.com", AgentSession: &proto.AgentSession{Agent: "claude", ID: "sess-1"}}, false},
		{"file pane", &proto.Pane{ID: "p1", File: "/tmp/a.go", AgentSession: &proto.AgentSession{Agent: "claude", ID: "sess-1"}}, false},
		{"invalid session id", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "claude", ID: "../bad"}}, false},
		{"unsupported agent dsh", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "dsh", ID: "sess-1"}}, false},
		{"unsupported agent agy", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "agy", ID: "sess-1"}}, false},
		{"unsupported agent vim", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "vim", ID: "sess-1"}}, false},
		{"claude", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "claude", ID: "13b997c3-f8b3-4b3e-bb8a-ca1b5d1b9787"}}, true},
		{"codex", &proto.Pane{ID: "p1", Status: proto.Working, AgentSession: &proto.AgentSession{Agent: "codex", ID: "019a2b3c-4d5e"}}, true},
		{"pi", &proto.Pane{ID: "p1", Status: proto.Waiting, AgentSession: &proto.AgentSession{Agent: "pi", ID: "0f3c2a1b-8e9d"}}, true},
		{"opencode", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "opencode", ID: "ses_ea7b9b964ffemfrVYxK9yw55Gz"}}, true},
	}
	for _, tc := range cases {
		if got := tc.pane.CanForkSession(); got != tc.want {
			t.Errorf("%s: CanForkSession() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestResolveForkCommand(t *testing.T) {
	orig := runOpenCodeFork
	t.Cleanup(func() { runOpenCodeFork = orig })

	for _, tc := range []struct {
		agent string
		id    string
		want  []string
	}{
		{"claude", "13b997c3-f8b3", []string{"claude", "--resume", "13b997c3-f8b3", "--fork-session"}},
		{"codex", "019a2b3c-4d5e", []string{"codex", "fork", "019a2b3c-4d5e"}},
		{"pi", "0f3c2a1b-8e9d", []string{"pi", "--fork", "0f3c2a1b-8e9d"}},
	} {
		got, err := resolveForkCommand(context.Background(), &proto.AgentSession{Agent: tc.agent, ID: tc.id}, "/repo")
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.agent, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.agent, got, tc.want)
		}
	}

	// OpenCode v2: server returns forked child session ID.
	var gotID, gotDir string
	runOpenCodeFork = func(_ context.Context, id, dir string) (string, error) {
		gotID, gotDir = id, dir
		return "ses_child_456", nil
	}
	got, err := resolveForkCommand(context.Background(), &proto.AgentSession{Agent: "opencode", ID: "ses_parent_123"}, "/repo/wt")
	if err != nil {
		t.Fatalf("opencode v2 error: %v", err)
	}
	if gotID != "ses_parent_123" || gotDir != "/repo/wt" {
		t.Errorf("opencode hook args = (%q, %q), want (%q, %q)", gotID, gotDir, "ses_parent_123", "/repo/wt")
	}
	if want := []string{"opencode", "--session", "ses_child_456"}; !slices.Equal(got, want) {
		t.Errorf("opencode v2 got %v, want %v", got, want)
	}

	// OpenCode v1 fallback: legacy CLI without `api` subcommand.
	runOpenCodeFork = func(context.Context, string, string) (string, error) {
		return "", errOpenCodeLegacyCLI
	}
	got, err = resolveForkCommand(context.Background(), &proto.AgentSession{Agent: "opencode", ID: "ses_parent_123"}, "/repo")
	if err != nil {
		t.Fatalf("opencode v1 fallback error: %v", err)
	}
	if want := []string{"opencode", "--session", "ses_parent_123", "--fork"}; !slices.Equal(got, want) {
		t.Errorf("opencode v1 fallback got %v, want %v", got, want)
	}

	// OpenCode error propagates.
	runOpenCodeFork = func(context.Context, string, string) (string, error) {
		return "", errors.New("server unreachable")
	}
	if _, err := resolveForkCommand(context.Background(), &proto.AgentSession{Agent: "opencode", ID: "ses_parent_123"}, "/repo"); err == nil {
		t.Error("expected opencode error to propagate")
	}

	// Unsupported or unsafe session ID rejected.
	for _, bad := range []*proto.AgentSession{
		nil,
		{Agent: "claude", ID: "bad;id"},
		{Agent: "dsh", ID: "sess-1"},
	} {
		if _, err := resolveForkCommand(context.Background(), bad, "/repo"); err == nil {
			t.Errorf("expected error for %+v", bad)
		}
	}
}

func TestPaneForkRPC(t *testing.T) {
	d := testDaemon(t)
	dir := d.projects[0].Dir
	worktreeDir := filepath.Join(t.TempDir(), "wt")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ses, err := d.create(proto.CreateParams{Project: "pr", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(ses.ID) })
	pID := ses.Panes[0].ID

	// Pane without AgentSession fails cleanly.
	rawBad, _ := json.Marshal(proto.ForkParams{Pane: pID})
	if _, err := d.call(nil, "pane.fork", rawBad); err == nil {
		t.Fatal("expected pane.fork on pane without agent session to fail")
	}

	// Set a forkable agent session in a worktree (ProcessDir).
	pn := d.findPane(pID)
	pn.mu.Lock()
	pn.info.ProcessDir = worktreeDir
	pn.info.Agent = "claude"
	pn.info.AgentSession = &proto.AgentSession{Agent: "claude", ID: "sess-abc-123"}
	pn.info.AgentSessionID = "sess-abc-123"
	pn.mu.Unlock()

	rawOK, _ := json.Marshal(proto.ForkParams{Pane: pID})
	res, err := d.call(nil, "pane.fork", rawOK)
	if err != nil {
		t.Fatalf("pane.fork failed: %v", err)
	}
	ns, ok := res.(proto.Session)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	if len(ns.Panes) != 2 {
		t.Fatalf("expected 2 panes after fork, got %d", len(ns.Panes))
	}
	forked := ns.PaneByID(ns.Focus)
	if forked == nil || forked.ID == pID {
		t.Fatalf("expected focus on newly forked pane, got %+v", forked)
	}
	if want := "claude --resume sess-abc-123 --fork-session"; strings.Join(forked.Command, " ") != want {
		t.Errorf("forked command = %q, want %q", strings.Join(forked.Command, " "), want)
	}
	if forked.Dir != worktreeDir {
		t.Errorf("forked dir = %q, want %q", forked.Dir, worktreeDir)
	}
	if forked.AgentSession != nil {
		t.Errorf("expected new forked pane to start without parent's AgentSession, got %+v", forked.AgentSession)
	}
	if ns.Layout == nil || !ns.Layout.Row {
		t.Errorf("expected default right split (Row layout), got %+v", ns.Layout)
	}
}
