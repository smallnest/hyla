package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"hyla/internal/proto"
)

func makeWorktreeCommit(t *testing.T, repo, branch, file, content string) string {
	t.Helper()
	ctx := context.Background()
	wt := filepath.Join(t.TempDir(), branch)
	if _, err := execGit(ctx, repo, "worktree", "add", "-b", branch, wt, "main"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = execGit(context.Background(), repo, "worktree", "remove", "--force", wt)
	})
	if err := os.WriteFile(filepath.Join(wt, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := execGit(ctx, wt, "add", file); err != nil {
		t.Fatal(err)
	}
	if _, err := execGit(ctx, wt, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "commit "+branch); err != nil {
		t.Fatal(err)
	}
	return wt
}

func TestMergeQueueTrainAndIsolation(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)

	makeWorktreeCommit(t, repo, "feat-good", "good.txt", "ok\n")
	makeWorktreeCommit(t, repo, "feat-bad", "bad.txt", "broken\n")

	laneGood, err := d.mqAdd(proto.MergeQueueAddParams{Repo: repo, Branch: "feat-good", Base: "main"})
	if err != nil {
		t.Fatalf("mqAdd good failed: %v", err)
	}
	laneBad, err := d.mqAdd(proto.MergeQueueAddParams{Repo: repo, Branch: "feat-bad", Base: "main"})
	if err != nil {
		t.Fatalf("mqAdd bad failed: %v", err)
	}

	res, err := d.mqRun(nil, proto.MergeQueueRunParams{
		Repo:       repo,
		Train:      4,
		MaxBounces: 2,
		CheckCmd:   "test ! -f bad.txt",
	})
	if err != nil {
		t.Fatalf("mqRun failed: %v", err)
	}
	if !slices.Equal(res.Landed, []string{laneGood.ID}) {
		t.Errorf("Landed = %v, want [%s]", res.Landed, laneGood.ID)
	}
	if !slices.Equal(res.Bounced, []string{laneBad.ID}) {
		t.Errorf("Bounced = %v, want [%s]", res.Bounced, laneBad.ID)
	}
	if _, err := os.Stat(filepath.Join(repo, "good.txt")); err != nil {
		t.Errorf("expected good.txt on main: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "bad.txt")); err == nil {
		t.Errorf("bad.txt should not be on main")
	}

	// Anti-cheat guard blocks t.Skip until approved.
	makeWorktreeCommit(t, repo, "feat-cheat", "foo_test.go", "package main\nimport \"testing\"\nfunc TestX(t *testing.T) { t.Skip(\"x\") }\n")
	laneCheat, err := d.mqAdd(proto.MergeQueueAddParams{Repo: repo, Branch: "feat-cheat", Base: "main"})
	if err != nil {
		t.Fatalf("mqAdd cheat failed: %v", err)
	}
	res2, err := d.mqRun(nil, proto.MergeQueueRunParams{Repo: repo, CheckCmd: "true"})
	if err != nil {
		t.Fatalf("mqRun cheat failed: %v", err)
	}
	if !slices.Equal(res2.NeedsReview, []string{laneCheat.ID}) {
		t.Fatalf("NeedsReview = %v, want [%s]", res2.NeedsReview, laneCheat.ID)
	}

	approved, err := d.mqApprove(laneCheat.ID)
	if err != nil || !approved.GuardApproved || approved.Status != "queued" {
		t.Fatalf("mqApprove = (%+v, %v)", approved, err)
	}
	res3, err := d.mqRun(nil, proto.MergeQueueRunParams{Repo: repo, CheckCmd: "true"})
	if err != nil {
		t.Fatalf("mqRun after approve failed: %v", err)
	}
	if !slices.Equal(res3.Landed, []string{laneCheat.ID}) {
		t.Errorf("Landed after approve = %v, want [%s]", res3.Landed, laneCheat.ID)
	}
}
