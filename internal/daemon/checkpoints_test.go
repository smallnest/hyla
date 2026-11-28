package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"hyla/internal/proto"
)

func TestCheckpointsCreateRestoreForkAndAutoRecord(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)

	s, err := d.create(proto.CreateParams{Dir: repo, Command: []string{"cat"}})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	cp1, err := d.checkpointCreate(proto.CheckpointCreateParams{
		Target: s.Pane.ID,
		Label:  "initial clean state",
	})
	if err != nil {
		t.Fatalf("checkpointCreate: %v", err)
	}

	// Mutate README.md and create an untracked file.
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("broken edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "scratch.tmp"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Restore back to cp1: README.md reverts and scratch.tmp is removed, while a safety backup is kept.
	res, err := d.checkpointRestore(proto.CheckpointRestoreParams{ID: cp1.ID[:8]})
	if err != nil {
		t.Fatalf("checkpointRestore: %v", err)
	}
	if res.SafetyBackupID == "" {
		t.Fatalf("expected non-empty safety backup id")
	}
	got, err := os.ReadFile(filepath.Join(repo, "README.md"))
	if err != nil || string(got) != "hello\n" {
		t.Fatalf("expected restored README.md = hello, got %q (err=%v)", got, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "scratch.tmp")); !os.IsNotExist(err) {
		t.Fatalf("expected scratch.tmp cleaned on restore, stat err=%v", err)
	}

	// Fork from the safety backup into an isolated worktree session; scratch.tmp must appear in the fork!
	forkRes, err := d.checkpointFork(proto.CheckpointForkParams{
		ID:     res.SafetyBackupID,
		Branch: "fork-experiment",
	})
	if err != nil {
		t.Fatalf("checkpointFork: %v", err)
	}
	if forkRes.Session.Worktree == nil || forkRes.Branch != "fork-experiment" {
		t.Fatalf("unexpected fork session: %+v", forkRes)
	}
	t.Cleanup(func() {
		_ = d.closeSession(proto.CloseParams{ID: forkRes.Session.ID, RemoveWorktree: true, Force: true})
	})
	forkedScratch, err := os.ReadFile(filepath.Join(forkRes.Session.Worktree.Path, "scratch.tmp"))
	if err != nil || string(forkedScratch) != "untracked\n" {
		t.Fatalf("expected scratch.tmp in forked worktree, got %q (err=%v)", forkedScratch, err)
	}

	// Test gitFollow.snapshot auto-recording into checkpoints.
	pn := d.findPane(s.Pane.ID)
	if pn == nil {
		t.Fatal("missing pane")
	}
	d.git = &gitFollow{d: d}
	d.git.snapshot(pn, repo, time.Now())
	list := d.checkpointList(proto.CheckpointListParams{Target: s.Pane.ID})
	foundAuto := false
	for _, c := range list {
		if c.Auto {
			foundAuto = true
			break
		}
	}
	if !foundAuto {
		t.Fatalf("expected auto turn checkpoint from gitFollow.snapshot, list=%+v", list)
	}
}
