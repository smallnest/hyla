package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"hyla/internal/proto"
)

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")
	return dir
}

func TestArenaLifecycle(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)

	ar, err := d.arenaStart(proto.ArenaStartParams{
		Prompt:  "implement feature",
		Agents:  []string{"echo", "printf"},
		Dir:     repo,
		TestCmd: "true",
		Name:    "auth-race",
	})
	if err != nil {
		t.Fatalf("arenaStart failed: %v", err)
	}
	t.Cleanup(func() {
		_ = d.arenaClose(proto.ArenaCloseParams{ID: ar.ID, Force: true})
	})

	if ar.Status != "running" || ar.Base != "main" {
		t.Fatalf("unexpected arena state: %+v", ar)
	}
	if len(ar.Contestants) != 2 {
		t.Fatalf("expected 2 contestants, got %d", len(ar.Contestants))
	}

	// Commit a change in contestant 0's worktree.
	wt0 := ar.Contestants[0].Worktree
	if err := os.WriteFile(filepath.Join(wt0, "feature.txt"), []byte("winner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := execGit(ctx, wt0, "add", "feature.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := execGit(ctx, wt0, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "add feature"); err != nil {
		t.Fatal(err)
	}

	list := d.arenaList()
	if len(list) != 1 || list[0].ID != ar.ID {
		t.Fatalf("arenaList = %+v", list)
	}

	status, err := d.arenaStatus(ar.ID)
	if err != nil {
		t.Fatalf("arenaStatus failed: %v", err)
	}
	if status.Contestants[0].Commits != 1 || status.Contestants[0].Added != 1 {
		t.Errorf("expected 1 commit and 1 added line, got %+v", status.Contestants[0])
	}

	tested, err := d.arenaTest(proto.ArenaTestParams{ID: ar.ID, Cmd: "test -f feature.txt"})
	if err != nil {
		t.Fatalf("arenaTest failed: %v", err)
	}
	if tested.Contestants[0].CheckStatus != "passed" {
		t.Errorf("contestant 0 check = %q (%s), want passed", tested.Contestants[0].CheckStatus, tested.Contestants[0].CheckOutput)
	}
	if tested.Contestants[1].CheckStatus != "failed" {
		t.Errorf("contestant 1 check = %q, want failed", tested.Contestants[1].CheckStatus)
	}

	picked, err := d.arenaPick(proto.ArenaPickParams{ID: ar.ID, Winner: "echo"})
	if err != nil {
		t.Fatalf("arenaPick failed: %v", err)
	}
	if picked.Status != "picked" || picked.Winner != "echo" {
		t.Fatalf("unexpected picked arena: %+v", picked)
	}
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); err != nil {
		t.Errorf("expected feature.txt merged into main repo: %v", err)
	}
}
