package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	"hyla/internal/proto"
)

func TestFactoryRunCircuitBreakerAndDigest(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)
	d.fetchIssue = func(_ context.Context, _ string, issue int) (string, string, error) {
		if issue == 101 {
			return "Exit cleanly", "finish task", nil
		}
		return "Runaway loop", "loop", nil
	}

	// 1. Runaway wall-clock circuit breaker trips a hung task cleanly.
	run, err := d.factoryStart(proto.FactoryStartParams{
		Repo:            repo,
		Base:            "main",
		Agent:           "sleep",
		Tasks:           []string{"5"},
		MaxParallel:     1,
		MaxDurationMs:   150,
		MaxRepeatErrors: 3,
	})
	if err != nil {
		t.Fatalf("factoryStart runaway: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	var final proto.FactoryRun
	for time.Now().Before(deadline) {
		st, err := d.factoryStatus(run.ID)
		if err == nil && st.Status == "done" {
			final = st
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if final.Status != "done" || len(final.Tasks) != 1 || final.Tasks[0].Status != "tripped" {
		t.Fatalf("expected tripped circuit-breaker task, got %+v", final)
	}
	t.Cleanup(func() {
		for _, tk := range final.Tasks {
			if tk.SessionID != "" {
				_ = d.closeSession(proto.CloseParams{ID: tk.SessionID, RemoveWorktree: true, Force: true})
			}
		}
	})

	// 2. Clean issue task runs check command, auto-enqueues into MQ, and renders digest.
	run2, err := d.factoryStart(proto.FactoryStartParams{
		Repo:        repo,
		Base:        "main",
		Agent:       "true",
		Issues:      []int{101},
		CheckCmd:    "true",
		AutoEnqueue: true,
	})
	if err != nil {
		t.Fatalf("factoryStart issue: %v", err)
	}
	deadline = time.Now().Add(8 * time.Second)
	var final2 proto.FactoryRun
	for time.Now().Before(deadline) {
		st, err := d.factoryStatus(run2.ID)
		if err == nil && st.Status == "done" {
			final2 = st
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Cleanup(func() {
		for _, tk := range final2.Tasks {
			if tk.SessionID != "" {
				_ = d.closeSession(proto.CloseParams{ID: tk.SessionID, RemoveWorktree: true, Force: true})
			}
		}
	})
	if final2.Status != "done" || len(final2.Tasks) != 1 || final2.Tasks[0].Status != "verified" || final2.Tasks[0].LaneID == "" {
		t.Fatalf("expected verified + auto-enqueued task, got %+v", final2)
	}

	dig, err := d.factoryDigest(proto.FactoryDigestParams{ID: run2.ID})
	if err != nil {
		t.Fatalf("factoryDigest: %v", err)
	}
	if !strings.Contains(dig.Markdown, "1 verified") || !strings.Contains(dig.Markdown, final2.Tasks[0].LaneID) {
		t.Fatalf("unexpected digest:\n%s", dig.Markdown)
	}
}
