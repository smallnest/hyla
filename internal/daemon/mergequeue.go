package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"hyla/internal/git"
	"hyla/internal/proto"
)

func (d *Daemon) mergeQueuePath() string { return filepath.Join(d.dir, "mergequeue.json") }

func (d *Daemon) loadMergeQueue() {
	data, err := os.ReadFile(d.mergeQueuePath())
	if err != nil {
		return
	}
	var list []*proto.MergeLane
	if json.Unmarshal(data, &list) != nil {
		return
	}
	d.mu.Lock()
	d.mergeLanes = list
	d.mu.Unlock()
}

func (d *Daemon) saveMergeQueue() {
	d.mu.Lock()
	data, err := json.MarshalIndent(d.mergeLanes, "", "  ")
	d.mu.Unlock()
	if err == nil {
		writeAtomic(d.mergeQueuePath(), data)
	}
}

func (d *Daemon) findLaneLocked(ref string) *proto.MergeLane {
	for _, l := range d.mergeLanes {
		if l.ID == ref || l.Branch == ref {
			return l
		}
	}
	var match *proto.MergeLane
	for _, l := range d.mergeLanes {
		if strings.HasPrefix(l.ID, ref) {
			if match != nil {
				return nil
			}
			match = l
		}
	}
	return match
}

func (d *Daemon) mqAdd(p proto.MergeQueueAddParams) (proto.MergeLane, error) {
	var lane proto.MergeLane
	now := time.Now().UnixMilli()
	lane.ID = "mq_" + newID()
	lane.Status = "queued"
	lane.CheckCmd = strings.TrimSpace(p.CheckCmd)
	lane.FlowPath = strings.TrimSpace(p.FlowPath)
	lane.CreatedAt = now
	lane.UpdatedAt = now

	if p.Target != "" {
		d.mu.Lock()
		s := d.findSessionLocked(p.Target)
		if s != nil {
			lane.SessionID = s.id
			if len(s.panes) > 0 {
				s.panes[0].mu.Lock()
				lane.PaneID = s.panes[0].info.ID
				s.panes[0].mu.Unlock()
			}
			if s.worktree != nil {
				lane.Worktree = s.worktree.Path
				lane.Branch = s.worktree.Branch
				lane.Base = s.worktree.Base
				lane.Repo = s.worktree.Main
			}
		}
		d.mu.Unlock()
		if s == nil {
			return proto.MergeLane{}, fmt.Errorf("no session or pane %q", p.Target)
		}
	}

	if p.Repo != "" {
		lane.Repo = p.Repo
	}
	if p.Branch != "" {
		lane.Branch = p.Branch
	}
	if p.Base != "" {
		lane.Base = p.Base
	}
	if lane.Repo == "" {
		return proto.MergeLane{}, errors.New("mq.add: repo or worktree session is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r, err := git.Open(ctx, lane.Repo)
	if err != nil {
		return proto.MergeLane{}, fmt.Errorf("mq.add: %w", err)
	}
	if m := r.Main(); m != "" {
		lane.Repo = m
	} else {
		lane.Repo = r.Root
	}
	if lane.Branch == "" {
		return proto.MergeLane{}, errors.New("mq.add: branch is required")
	}
	if lane.Base == "" {
		if def := r.DefaultBranch(); def != "" {
			lane.Base = def
		} else {
			lane.Base = "main"
		}
	}

	d.mu.Lock()
	d.mergeLanes = append(d.mergeLanes, &lane)
	out := lane
	d.mu.Unlock()
	d.saveMergeQueue()
	return out, nil
}

func (d *Daemon) mqList(p proto.MergeQueueListParams) []proto.MergeLane {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]proto.MergeLane, 0, len(d.mergeLanes))
	for _, l := range d.mergeLanes {
		if p.Repo != "" && l.Repo != p.Repo {
			continue
		}
		cp := *l
		cp.Findings = slices.Clone(l.Findings)
		out = append(out, cp)
	}
	return out
}

func (d *Daemon) mqBounceLane(laneID, msg string, maxBounces int) string {
	d.mu.Lock()
	l := d.findLaneLocked(laneID)
	if l == nil {
		d.mu.Unlock()
		return ""
	}
	l.Bounces++
	l.LastError = msg
	l.UpdatedAt = time.Now().UnixMilli()
	if l.Bounces >= maxBounces {
		l.Status = "needs_review"
	} else {
		l.Status = "bounced"
	}
	status := l.Status
	paneID := l.PaneID
	var pn *pane
	if paneID != "" {
		pn = d.findPaneLocked(paneID)
	}
	d.mu.Unlock()
	if pn != nil {
		note := fmt.Sprintf("\n# [hyla mq] lane %s (%s) bounced (%d/%d): %s\n", l.ID, l.Branch, l.Bounces, maxBounces, msg)
		pn.input([]byte(note))
	}
	return status
}

func (d *Daemon) mqFastForwardBase(ctx context.Context, repo, base, sha string) error {
	curOut, err := execGit(ctx, repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil && strings.TrimSpace(string(curOut)) == base {
		_, err := execGit(ctx, repo, "merge", "--ff-only", sha)
		return err
	}
	_, err = execGit(ctx, repo, "update-ref", "refs/heads/"+base, sha)
	return err
}

func (d *Daemon) mqRunCheck(ctx context.Context, wt, cmdStr string) error {
	if strings.TrimSpace(cmdStr) == "" {
		return nil
	}
	cmd := shellCmd(ctx, wt, cmdStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := tailString(out, 2048)
		if tail != "" {
			return fmt.Errorf("%v\n%s", err, tail)
		}
		return err
	}
	return nil
}

func (d *Daemon) mqRun(c *controlConn, p proto.MergeQueueRunParams) (proto.MergeQueueRunResult, error) {
	trainSize := p.Train
	if trainSize <= 0 {
		trainSize = 4
	}
	maxBounces := p.MaxBounces
	if maxBounces <= 0 {
		maxBounces = 2
	}

	d.mu.Lock()
	var queued []*proto.MergeLane
	targetRepo := strings.TrimSpace(p.Repo)
	for _, l := range d.mergeLanes {
		if l.Status != "queued" {
			continue
		}
		if targetRepo == "" {
			targetRepo = l.Repo
		}
		if l.Repo != targetRepo {
			continue
		}
		cp := *l
		queued = append(queued, &cp)
		if len(queued) >= trainSize {
			break
		}
	}
	d.mu.Unlock()

	if len(queued) == 0 {
		return proto.MergeQueueRunResult{Lanes: d.mqList(proto.MergeQueueListParams{Repo: targetRepo})}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var res proto.MergeQueueRunResult
	var eligible []*proto.MergeLane

	// 1. Anti-cheat diff inspection before merging into train.
	for _, l := range queued {
		if l.GuardApproved {
			eligible = append(eligible, l)
			continue
		}
		diffOut, err := execGit(ctx, l.Repo, "diff", l.Base+"..."+l.Branch)
		if err == nil {
			if findings := InspectDiffForCheating(string(diffOut)); len(findings) > 0 {
				d.mu.Lock()
				if cur := d.findLaneLocked(l.ID); cur != nil {
					cur.Status = "needs_review"
					cur.Findings = findings
					cur.LastError = fmt.Sprintf("anti-cheat guard flagged %d finding(s)", len(findings))
					cur.UpdatedAt = time.Now().UnixMilli()
				}
				d.mu.Unlock()
				res.NeedsReview = append(res.NeedsReview, l.ID)
				if p.FeishuChat != "" {
					if bot := d.feishuBot(); bot != nil {
						var lines []string
						for _, f := range findings {
							lines = append(lines, fmt.Sprintf("- **%s** `%s:%d`: %s", f.Kind, f.File, f.Line, f.Reason))
						}
						_, _ = bot.SendApprovalCard(p.FeishuChat,
							"Merge Guard: "+l.Branch,
							strings.Join(lines, "\n"),
							l.ID, l.Screenshot)
					}
				}
				continue
			}
		}
		eligible = append(eligible, l)
	}

	if len(eligible) == 0 {
		d.saveMergeQueue()
		res.Lanes = d.mqList(proto.MergeQueueListParams{Repo: targetRepo})
		return res, nil
	}

	base := eligible[0].Base
	wt := filepath.Join(targetRepo, ".hyla", "mq-integrate-"+newID())
	if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
		return proto.MergeQueueRunResult{}, err
	}
	if _, err := execGit(ctx, targetRepo, "worktree", "add", "--detach", wt, base); err != nil {
		return proto.MergeQueueRunResult{}, err
	}
	defer func() {
		_, _ = execGit(context.Background(), targetRepo, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}()

	var merged []*proto.MergeLane
	for _, l := range eligible {
		if _, err := execGit(ctx, wt, "merge", "--no-edit", l.Branch); err != nil {
			_, _ = execGit(ctx, wt, "merge", "--abort")
			st := d.mqBounceLane(l.ID, err.Error(), maxBounces)
			if st == "needs_review" {
				res.NeedsReview = append(res.NeedsReview, l.ID)
			} else {
				res.Bounced = append(res.Bounced, l.ID)
			}
			continue
		}
		merged = append(merged, l)
	}

	if len(merged) == 0 {
		d.saveMergeQueue()
		res.Lanes = d.mqList(proto.MergeQueueListParams{Repo: targetRepo})
		return res, nil
	}

	checkCmd := strings.TrimSpace(p.CheckCmd)
	if checkCmd == "" {
		checkCmd = merged[0].CheckCmd
	}

	checkErr := d.mqRunCheck(ctx, wt, checkCmd)
	if checkErr == nil {
		if err := d.mqReplayFlowIfSet(ctx, c, p, merged); err != nil {
			checkErr = err
		}
	}

	if checkErr == nil {
		shaOut, err := execGit(ctx, wt, "rev-parse", "HEAD")
		if err != nil {
			return proto.MergeQueueRunResult{}, err
		}
		sha := strings.TrimSpace(string(shaOut))
		if err := d.mqFastForwardBase(ctx, targetRepo, base, sha); err != nil {
			return proto.MergeQueueRunResult{}, err
		}
		for _, l := range merged {
			d.markLaneLanded(l.ID, p.Cleanup)
			res.Landed = append(res.Landed, l.ID)
		}
		d.notifyLandedFeishu(p.FeishuChat, merged)
	} else {
		// Isolate culprit(s) if train had >1 lane; otherwise bounce the single lane.
		if len(merged) == 1 {
			st := d.mqBounceLane(merged[0].ID, checkErr.Error(), maxBounces)
			if st == "needs_review" {
				res.NeedsReview = append(res.NeedsReview, merged[0].ID)
			} else {
				res.Bounced = append(res.Bounced, merged[0].ID)
			}
		} else {
			for _, l := range merged {
				_, _ = execGit(ctx, wt, "reset", "--hard", base)
				if _, err := execGit(ctx, wt, "merge", "--no-edit", l.Branch); err != nil {
					_, _ = execGit(ctx, wt, "merge", "--abort")
					st := d.mqBounceLane(l.ID, err.Error(), maxBounces)
					if st == "needs_review" {
						res.NeedsReview = append(res.NeedsReview, l.ID)
					} else {
						res.Bounced = append(res.Bounced, l.ID)
					}
					continue
				}
				laneCmd := checkCmd
				if l.CheckCmd != "" {
					laneCmd = l.CheckCmd
				}
				if err := d.mqRunCheck(ctx, wt, laneCmd); err != nil {
					st := d.mqBounceLane(l.ID, err.Error(), maxBounces)
					if st == "needs_review" {
						res.NeedsReview = append(res.NeedsReview, l.ID)
					} else {
						res.Bounced = append(res.Bounced, l.ID)
					}
					continue
				}
				shaOut, err := execGit(ctx, wt, "rev-parse", "HEAD")
				if err != nil {
					return proto.MergeQueueRunResult{}, err
				}
				if err := d.mqFastForwardBase(ctx, targetRepo, base, strings.TrimSpace(string(shaOut))); err != nil {
					return proto.MergeQueueRunResult{}, err
				}
				d.markLaneLanded(l.ID, p.Cleanup)
				res.Landed = append(res.Landed, l.ID)
			}
		}
	}

	d.saveMergeQueue()
	res.Lanes = d.mqList(proto.MergeQueueListParams{Repo: targetRepo})
	return res, nil
}

func (d *Daemon) mqReplayFlowIfSet(ctx context.Context, c *controlConn, p proto.MergeQueueRunParams, merged []*proto.MergeLane) error {
	if p.BrowserPane == "" || c == nil {
		return nil
	}
	d.mu.Lock()
	pn := d.findPaneLocked(p.BrowserPane)
	d.mu.Unlock()
	if pn == nil {
		return nil
	}
	for _, l := range merged {
		if l.FlowPath == "" {
			continue
		}
		rep, err := d.browserReplay(c, pn, proto.BrowserReplayParams{
			Pane:       p.BrowserPane,
			FlowPath:   l.FlowPath,
			FeishuChat: p.FeishuChat,
		})
		if err != nil {
			return err
		}
		if rep.Screenshot != "" {
			d.mu.Lock()
			if cur := d.findLaneLocked(l.ID); cur != nil {
				cur.Screenshot = rep.Screenshot
			}
			d.mu.Unlock()
		}
		if !rep.Passed {
			return fmt.Errorf("browser flow %s failed", l.FlowPath)
		}
	}
	_ = ctx
	return nil
}

func (d *Daemon) markLaneLanded(id string, cleanup bool) {
	d.mu.Lock()
	l := d.findLaneLocked(id)
	var sid string
	if l != nil {
		l.Status = "landed"
		l.LastError = ""
		l.UpdatedAt = time.Now().UnixMilli()
		sid = l.SessionID
	}
	d.mu.Unlock()
	if cleanup && sid != "" {
		_ = d.closeSession(proto.CloseParams{ID: sid, RemoveWorktree: true, Force: true})
	}
}

func (d *Daemon) notifyLandedFeishu(chatID string, landed []*proto.MergeLane) {
	if chatID == "" || len(landed) == 0 {
		return
	}
	bot := d.feishuBot()
	if bot == nil {
		return
	}
	var branches []string
	var shot string
	for _, l := range landed {
		branches = append(branches, "`"+l.Branch+"`")
		if shot == "" && l.Screenshot != "" {
			shot = l.Screenshot
		}
	}
	_, _ = bot.SendApprovalCard(chatID, "Merge Train Landed",
		fmt.Sprintf("Landed %d lane(s) into `%s`: %s", len(landed), landed[0].Base, strings.Join(branches, ", ")),
		"", shot)
}

func (d *Daemon) mqApprove(id string) (proto.MergeLane, error) {
	d.mu.Lock()
	l := d.findLaneLocked(id)
	if l == nil {
		d.mu.Unlock()
		return proto.MergeLane{}, fmt.Errorf("no merge lane %q", id)
	}
	l.GuardApproved = true
	l.Status = "queued"
	l.LastError = ""
	l.UpdatedAt = time.Now().UnixMilli()
	out := *l
	out.Findings = slices.Clone(l.Findings)
	d.mu.Unlock()
	d.saveMergeQueue()
	return out, nil
}

func (d *Daemon) mqReject(id, reason string) (proto.MergeLane, error) {
	d.mu.Lock()
	l := d.findLaneLocked(id)
	if l == nil {
		d.mu.Unlock()
		return proto.MergeLane{}, fmt.Errorf("no merge lane %q", id)
	}
	l.Status = "rejected"
	l.LastError = reason
	l.UpdatedAt = time.Now().UnixMilli()
	out := *l
	out.Findings = slices.Clone(l.Findings)
	d.mu.Unlock()
	d.saveMergeQueue()
	return out, nil
}

func (d *Daemon) mqRemove(id string) error {
	d.mu.Lock()
	before := len(d.mergeLanes)
	d.mergeLanes = slices.DeleteFunc(d.mergeLanes, func(l *proto.MergeLane) bool { return l.ID == id })
	removed := len(d.mergeLanes) < before
	d.mu.Unlock()
	if !removed {
		return fmt.Errorf("no merge lane %q", id)
	}
	d.saveMergeQueue()
	return nil
}

func (d *Daemon) mqClear(repo string) int {
	d.mu.Lock()
	before := len(d.mergeLanes)
	d.mergeLanes = slices.DeleteFunc(d.mergeLanes, func(l *proto.MergeLane) bool {
		if repo != "" && l.Repo != repo {
			return false
		}
		return l.Status == "landed" || l.Status == "rejected"
	})
	removed := before - len(d.mergeLanes)
	d.mu.Unlock()
	if removed > 0 {
		d.saveMergeQueue()
	}
	return removed
}
