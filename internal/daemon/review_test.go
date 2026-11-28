package daemon

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"hyla/internal/proto"
)

func TestPickReviewerAgent(t *testing.T) {
	fakeLook := func(avail ...string) func(string) (string, error) {
		set := map[string]bool{}
		for _, a := range avail {
			set[a] = true
		}
		return func(name string) (string, error) {
			if set[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		}
	}

	got, err := pickReviewerAgent("claude", fakeLook("claude", "codex", "agy"))
	if err != nil || got != "codex" {
		t.Fatalf("pickReviewerAgent(claude) = (%q, %v), want (codex, nil)", got, err)
	}

	got, err = pickReviewerAgent("antigravity", fakeLook("agy", "opencode"))
	if err != nil || got != "opencode" {
		t.Fatalf("pickReviewerAgent(antigravity) = (%q, %v), want (opencode, nil)", got, err)
	}

	if _, err := pickReviewerAgent("claude", fakeLook("claude")); err == nil {
		t.Fatal("expected error when no alternative agent is installed")
	}
}

func TestBuildCrossReviewPrompt(t *testing.T) {
	pBranch := buildCrossReviewPrompt("claude", "branch")
	if !strings.Contains(pBranch, "Do NOT edit files") || !strings.Contains(pBranch, "merge-base") {
		t.Errorf("unexpected branch review prompt: %q", pBranch)
	}
	pTurn := buildCrossReviewPrompt("codex", "turn")
	if !strings.Contains(pTurn, "codex") || !strings.Contains(pTurn, "git diff") {
		t.Errorf("unexpected turn review prompt: %q", pTurn)
	}
}

func TestCrossReviewPaneRPC(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)
	ses, err := d.create(proto.CreateParams{Project: "pr", Dir: repo})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.removeSession(ses.ID) })
	pID := ses.Panes[0].ID

	pn := d.findPane(pID)
	pn.mu.Lock()
	pn.info.Agent = "claude"
	pn.mu.Unlock()

	raw, _ := json.Marshal(proto.CrossReviewParams{
		Pane:     pID,
		Reviewer: "echo",
		Scope:    "branch",
	})
	resAny, err := d.call(nil, "pane.crossReview", raw)
	if err != nil {
		t.Fatalf("pane.crossReview failed: %v", err)
	}
	res, ok := resAny.(proto.CrossReviewResult)
	if !ok {
		t.Fatalf("unexpected result type %T", resAny)
	}
	if res.Reviewer != "echo" || res.PaneID == "" || res.PaneID == pID {
		t.Fatalf("unexpected cross-review result: %+v", res)
	}
	reviewPane := res.Session.PaneByID(res.PaneID)
	if reviewPane == nil || len(reviewPane.Command) < 2 || reviewPane.Command[0] != "echo" {
		t.Fatalf("unexpected review pane command: %+v", reviewPane)
	}
}
