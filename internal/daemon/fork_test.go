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
		{"unsupported agent vim", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "vim", ID: "sess-1"}}, false},
		{"claude", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "claude", ID: "13b997c3-f8b3-4b3e-bb8a-ca1b5d1b9787"}}, true},
		{"codex", &proto.Pane{ID: "p1", Status: proto.Working, AgentSession: &proto.AgentSession{Agent: "codex", ID: "019a2b3c-4d5e"}}, true},
		{"pi", &proto.Pane{ID: "p1", Status: proto.Waiting, AgentSession: &proto.AgentSession{Agent: "pi", ID: "0f3c2a1b-8e9d"}}, true},
		{"opencode", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "opencode", ID: "ses_ea7b9b964ffemfrVYxK9yw55Gz"}}, true},
		{"agy", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "agy", ID: "f6999341-108b-450b-9087-3ae193332eed"}}, true},
		{"antigravity", &proto.Pane{ID: "p1", Status: proto.Idle, AgentSession: &proto.AgentSession{Agent: "antigravity", ID: "f6999341-108b-450b-9087-3ae193332eed"}}, true},
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
		{"agy", "f6999341-108b", []string{"agy", "--resume", "f6999341-108b"}},
		{"antigravity", "f6999341-108b", []string{"agy", "--resume", "f6999341-108b"}},
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

func TestIsOpenCodeLegacyOutput(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name: "opencode 1.x yargs help dump",
			output: `Commands:
  opencode [project]  start opencode tui
  opencode session    manage sessions

Options:
  -s, --session  session id to continue
      --fork     fork the session when continuing`,
			want: true,
		},
		{
			name:   "explicit unknown command",
			output: "error: unknown command: api",
			want:   true,
		},
		{
			name:   "explicit unknown subcommand",
			output: "Unknown subcommand 'api'",
			want:   true,
		},
		{
			name:   "explicit unexpected argument",
			output: "error: unexpected argument 'api' found",
			want:   true,
		},
		{
			name: "opencode v2 help with api subcommand",
			output: `Commands:
  opencode api   call server API
Options:
  --fork`,
			want: false,
		},
		{
			name:   "json error from server",
			output: `{"error":"session not found"}`,
			want:   false,
		},
		{
			name:   "connection refused",
			output: "connection refused",
			want:   false,
		},
	}
	for _, tc := range cases {
		if got := isOpenCodeLegacyOutput(tc.output); got != tc.want {
			t.Errorf("%s: isOpenCodeLegacyOutput = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAgyConversationFromPath(t *testing.T) {
	cases := []struct {
		path     string
		wantID   string
		wantPrio int
	}{
		{"/Users/u/.gemini/antigravity-cli/presence/2859d64d-df68-4d7c-ac4c-1e1cb49256ad.lock", "2859d64d-df68-4d7c-ac4c-1e1cb49256ad", 2},
		{"/Users/u/.gemini/antigravity-cli/conversations/2859d64d-df68-4d7c-ac4c-1e1cb49256ad.db", "2859d64d-df68-4d7c-ac4c-1e1cb49256ad", 1},
		{"/Users/u/.gemini/antigravity-cli/conversation_summaries.db", "", 0},
		{"/Users/u/.gemini/antigravity-cli/conversations/2859d64d-df68-4d7c-ac4c-1e1cb49256ad.db-wal", "", 0},
		{"/Users/u/.gemini/antigravity-cli/conversations/2859d64d-df68-4d7c-ac4c-1e1cb49256ad.db-shm", "", 0},
		{"/Users/u/.gemini/antigravity-cli/presence/bad;id.lock", "", 0},
	}
	for _, tc := range cases {
		gotID, gotPrio := agyConversationFromPath(tc.path)
		if gotID != tc.wantID || gotPrio != tc.wantPrio {
			t.Errorf("agyConversationFromPath(%q) = (%q, %d), want (%q, %d)", tc.path, gotID, gotPrio, tc.wantID, tc.wantPrio)
		}
	}
}

func TestAgyPromptReady(t *testing.T) {
	cases := []struct {
		name   string
		screen string
		want   bool
	}{
		{
			name:   "signing in screen",
			screen: "Welcome to the Antigravity CLI.\nSigning in...\n────────\n>\n────────",
			want:   false,
		},
		{
			name:   "trust prompt",
			screen: "Do you trust the contents of this project?\n> Yes, I trust this folder",
			want:   false,
		},
		{
			name:   "ready prompt box",
			screen: "Antigravity CLI 1.5.18\n────────────────────────\n>\n────────────────────────",
			want:   true,
		},
		{
			name:   "ready prompt with advisory banner above",
			screen: "This conversation is already open in another instance. Use /fork.\n────────────────────────\n>\n────────────────────────",
			want:   true,
		},
	}
	for _, tc := range cases {
		if got := agyPromptReady(tc.screen); got != tc.want {
			t.Errorf("%s: agyPromptReady = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPaneForkAgyRPC(t *testing.T) {
	orig := startAgyFork
	t.Cleanup(func() { startAgyFork = orig })

	var triggeredPaneID string
	startAgyFork = func(pn *pane) {
		if pn != nil {
			triggeredPaneID = pn.info.ID
		}
	}

	d := testDaemon(t)
	dir := d.projects[0].Dir
	ses, err := d.create(proto.CreateParams{Project: "pr", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(ses.ID) })
	pID := ses.Panes[0].ID

	pn := d.findPane(pID)
	pn.mu.Lock()
	pn.info.Agent = "agy"
	pn.info.AgentSession = &proto.AgentSession{Agent: "agy", ID: "2859d64d-df68-4d7c-ac4c-1e1cb49256ad"}
	pn.info.AgentSessionID = "2859d64d-df68-4d7c-ac4c-1e1cb49256ad"
	pn.mu.Unlock()

	raw, _ := json.Marshal(proto.ForkParams{Pane: pID})
	res, err := d.call(nil, "pane.fork", raw)
	if err != nil {
		t.Fatalf("pane.fork agy failed: %v", err)
	}
	ns := res.(proto.Session)
	forked := ns.PaneByID(ns.Focus)
	if forked == nil || forked.ID == pID {
		t.Fatalf("expected focus on newly forked pane, got %+v", forked)
	}
	if want := "agy --resume 2859d64d-df68-4d7c-ac4c-1e1cb49256ad"; strings.Join(forked.Command, " ") != want {
		t.Errorf("forked command = %q, want %q", strings.Join(forked.Command, " "), want)
	}
	if triggeredPaneID != forked.ID {
		t.Errorf("startAgyFork triggered on %q, want %q", triggeredPaneID, forked.ID)
	}
}
