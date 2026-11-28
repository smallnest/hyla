package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hyla/internal/proto"
)

func TestPathClaimsAndExpiry(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)

	s1, err := d.create(proto.CreateParams{Dir: repo, Command: []string{"cat"}})
	if err != nil {
		t.Fatalf("create s1: %v", err)
	}
	s2, err := d.create(proto.CreateParams{Dir: repo, Command: []string{"cat"}})
	if err != nil {
		t.Fatalf("create s2: %v", err)
	}

	c1, err := d.claimAdd(proto.PathClaimParams{
		Target:  s1.Pane.ID,
		Repo:    repo,
		Pattern: "internal/auth/*",
		Mode:    "block",
		Reason:  "migrating auth tokens",
	})
	if err != nil {
		t.Fatalf("claimAdd: %v", err)
	}
	if c1.Mode != "block" || c1.PaneID != s1.Pane.ID {
		t.Fatalf("unexpected claim: %+v", c1)
	}

	// Self pane should be allowed.
	selfChk := d.claimCheck(proto.PathClaimCheckParams{
		Pane: s1.Pane.ID,
		Repo: repo,
		Path: filepath.Join(repo, "internal/auth/token.go"),
	})
	if !selfChk.Allowed || len(selfChk.Conflicts) != 0 {
		t.Fatalf("expected self pane allowed with no conflicts, got %+v", selfChk)
	}

	// Other pane writing under internal/auth/* must be blocked.
	otherChk := d.claimCheck(proto.PathClaimCheckParams{
		Pane: s2.Pane.ID,
		Repo: repo,
		Path: filepath.Join(repo, "internal/auth/token.go"),
	})
	if otherChk.Allowed || len(otherChk.Conflicts) != 1 {
		t.Fatalf("expected blocked conflict for s2, got %+v", otherChk)
	}

	// Short TTL claim expires naturally.
	_, err = d.claimAdd(proto.PathClaimParams{
		Target:     s1.Pane.ID,
		Repo:       repo,
		Pattern:    "docs/**",
		Mode:       "warn",
		TTLSeconds: 1,
	})
	if err != nil {
		t.Fatalf("claimAdd ttl: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	list := d.claimList(repo)
	if len(list) != 1 || list[0].ID != c1.ID {
		t.Fatalf("expected expired claim pruned, got %+v", list)
	}

	if err := d.claimRelease(c1.ID[:8]); err != nil {
		t.Fatalf("claimRelease prefix: %v", err)
	}
	if len(d.claimList(repo)) != 0 {
		t.Fatalf("expected empty claims after release")
	}
}

func TestClashCheckDetectsConflictAndCleanMerge(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)
	ctx := context.Background()

	commitBranch := func(branch, file, content string) {
		wt, err := d.addWorktree(repo, branch, "main")
		if err != nil {
			t.Fatalf("addWorktree %s: %v", branch, err)
		}
		if err := os.WriteFile(filepath.Join(wt.Path, file), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := execGit(ctx, wt.Path, "add", file); err != nil {
			t.Fatal(err)
		}
		if _, err := execGit(ctx, wt.Path, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "commit "+branch); err != nil {
			t.Fatal(err)
		}
	}

	commitBranch("feat-a", "a.txt", "alpha\n")
	commitBranch("feat-b", "b.txt", "beta\n")

	okRes, err := d.clashCheck(proto.ClashCheckParams{
		Repo:     repo,
		Base:     "main",
		Branches: []string{"feat-a", "feat-b"},
		CheckCmd: "test -f a.txt && test -f b.txt",
	})
	if err != nil {
		t.Fatalf("clashCheck clean: %v", err)
	}
	if !okRes.Passed || len(okRes.ConflictFiles) != 0 {
		t.Fatalf("expected clean clash pass, got %+v", okRes)
	}

	// Now create two branches that both modify README.md incompatibly.
	commitBranch("feat-c1", "README.md", "version 1\n")
	commitBranch("feat-c2", "README.md", "version 2\n")

	badRes, err := d.clashCheck(proto.ClashCheckParams{
		Repo:     repo,
		Base:     "main",
		Branches: []string{"feat-c1", "feat-c2"},
	})
	if err != nil {
		t.Fatalf("clashCheck conflict: %v", err)
	}
	if badRes.Passed || len(badRes.ConflictFiles) == 0 || badRes.ConflictFiles[0] != "README.md" {
		t.Fatalf("expected README.md conflict, got %+v", badRes)
	}
}
