package daemon

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"hyla/internal/proto"
)

func canonicalAgent(a string) string {
	a = strings.ToLower(strings.TrimSpace(a))
	if a == "antigravity" {
		return "agy"
	}
	return a
}

// pickReviewerAgent picks an installed coding agent different from current.
func pickReviewerAgent(current string, lookPath func(string) (string, error)) (string, error) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	cur := canonicalAgent(current)
	for _, cand := range []string{"claude", "codex", "agy", "opencode", "pi"} {
		if cand == cur {
			continue
		}
		if _, err := lookPath(cand); err == nil {
			return cand, nil
		}
	}
	return "", errors.New("no alternative coding agent found on PATH for cross-review")
}

func buildCrossReviewPrompt(sourceAgent, scope string) string {
	diffCmd := "git diff HEAD"
	switch scope {
	case "branch":
		diffCmd = "git diff $(git merge-base HEAD main 2>/dev/null || git merge-base HEAD master 2>/dev/null || echo HEAD~1)...HEAD"
	case "turn":
		diffCmd = "git diff"
	}
	by := "the current implementation"
	if sourceAgent != "" {
		by = fmt.Sprintf("the work produced by %s", sourceAgent)
	}
	return fmt.Sprintf(
		"Perform a strict, read-only adversarial code review of %s in this repository. "+
			"Do NOT edit files or create commits. Inspect the changes with `%s` (and `git status`), "+
			"check edge cases, bugs, security issues, race conditions, regressions, and missing tests. "+
			"Cite exact file paths and line numbers for every finding.",
		by, diffCmd,
	)
}

func (d *Daemon) crossReviewPane(p proto.CrossReviewParams) (proto.CrossReviewResult, error) {
	d.mu.Lock()
	at := d.findPaneLocked(p.Pane)
	if at == nil {
		if s := d.findSessionLocked(p.Pane); s != nil {
			at = focusedLocked(s)
		}
	}
	if at == nil {
		d.mu.Unlock()
		return proto.CrossReviewResult{}, fmt.Errorf("no session or pane %q", p.Pane)
	}
	at.mu.Lock()
	atID := at.info.ID
	workDir := at.info.WorkDir()
	currentAgent := at.info.Agent
	shown := at.info.Shown()
	at.mu.Unlock()
	d.mu.Unlock()

	if shown {
		return proto.CrossReviewResult{}, fmt.Errorf("pane %s is not a terminal pane", atID)
	}

	reviewer := strings.TrimSpace(strings.ToLower(p.Reviewer))
	if reviewer == "" {
		var err error
		reviewer, err = pickReviewerAgent(currentAgent, exec.LookPath)
		if err != nil {
			return proto.CrossReviewResult{}, err
		}
	}

	direction := p.Direction
	if direction == "" {
		direction = "right"
	}
	prompt := buildCrossReviewPrompt(currentAgent, p.Scope)
	ns, err := d.split(proto.CreateParams{
		Split:     atID,
		Direction: direction,
		Dir:       workDir,
		Command:   agentPromptCommand(reviewer, prompt),
		Cols:      p.Cols,
		Rows:      p.Rows,
	})
	if err != nil {
		return proto.CrossReviewResult{}, err
	}
	return proto.CrossReviewResult{
		Session:  ns,
		Reviewer: reviewer,
		PaneID:   ns.Focus,
	}, nil
}
