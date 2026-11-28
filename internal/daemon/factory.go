package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"hyla/internal/git"
	"hyla/internal/proto"
)

func (d *Daemon) findFactoryRunLocked(ref string) *proto.FactoryRun {
	ref = strings.TrimSpace(ref)
	for _, r := range d.factoryRuns {
		if r.ID == ref {
			return r
		}
	}
	var match *proto.FactoryRun
	for _, r := range d.factoryRuns {
		if strings.HasPrefix(r.ID, ref) {
			if match != nil {
				return nil
			}
			match = r
		}
	}
	return match
}

func cloneFactoryRun(r *proto.FactoryRun) proto.FactoryRun {
	out := *r
	out.Tasks = append([]proto.FactoryTask(nil), r.Tasks...)
	return out
}

func defaultFetchIssue(ctx context.Context, repo string, issue int) (string, string, error) {
	cmd := exec.CommandContext(ctx, "gh", "issue", "view", strconv.Itoa(issue), "--json", "title,body")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("gh issue view #%d: %v (%s)", issue, err, strings.TrimSpace(string(out)))
	}
	var payload struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", "", fmt.Errorf("decode gh issue #%d: %w", issue, err)
	}
	return strings.TrimSpace(payload.Title), strings.TrimSpace(payload.Body), nil
}

func (d *Daemon) factoryStart(p proto.FactoryStartParams) (proto.FactoryRun, error) {
	if len(p.Issues) == 0 && len(p.Tasks) == 0 {
		return proto.FactoryRun{}, errors.New("factory.start: at least one issue or task prompt is required")
	}

	repo := strings.TrimSpace(p.Repo)
	if repo == "" {
		repo = proto.Home()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := git.Open(ctx, repo)
	if err != nil {
		return proto.FactoryRun{}, fmt.Errorf("factory.start: %w", err)
	}
	if m := r.Main(); m != "" {
		repo = m
	} else {
		repo = r.Root
	}

	base := strings.TrimSpace(p.Base)
	if base == "" {
		if st, err := r.Status(ctx); err == nil && st.Branch != "" {
			base = st.Branch
		} else if def := r.DefaultBranch(); def != "" {
			base = def
		} else {
			base = "HEAD"
		}
	}

	agent := strings.TrimSpace(strings.ToLower(p.Agent))
	if agent == "" {
		agent = "claude"
	}
	maxParallel := p.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 2
	}
	maxMinutes := p.MaxMinutes
	if maxMinutes <= 0 && p.MaxDurationMs <= 0 {
		maxMinutes = 45
	}
	maxOutputKB := p.MaxOutputKB
	if maxOutputKB <= 0 {
		maxOutputKB = 4096
	}
	maxRepeatErrors := p.MaxRepeatErrors
	if maxRepeatErrors <= 0 {
		maxRepeatErrors = 5
	}

	fetchFn := defaultFetchIssue
	if d.fetchIssue != nil {
		fetchFn = d.fetchIssue
	}

	runID := "fac_" + newID()
	shortRun := strings.TrimPrefix(runID, "fac_")
	if len(shortRun) > 6 {
		shortRun = shortRun[:6]
	}

	var tasks []proto.FactoryTask
	for _, num := range p.Issues {
		title, body, err := fetchFn(ctx, repo, num)
		if err != nil {
			title = fmt.Sprintf("Issue #%d", num)
			body = fmt.Sprintf("Implement GitHub issue #%d in %s.", num, repo)
		}
		prompt := title
		if body != "" {
			prompt = fmt.Sprintf("Issue #%d: %s\n\n%s", num, title, body)
		}
		tasks = append(tasks, proto.FactoryTask{
			ID:     "ft_" + newID(),
			Issue:  num,
			Title:  title,
			Prompt: prompt,
			Branch: fmt.Sprintf("factory/%s-issue-%d", shortRun, num),
			Status: "queued",
		})
	}
	for i, raw := range p.Tasks {
		prompt := strings.TrimSpace(raw)
		if prompt == "" {
			continue
		}
		title := prompt
		if len(title) > 64 {
			title = title[:61] + "..."
		}
		tasks = append(tasks, proto.FactoryTask{
			ID:     "ft_" + newID(),
			Title:  title,
			Prompt: prompt,
			Branch: fmt.Sprintf("factory/%s-task-%d", shortRun, i+1),
			Status: "queued",
		})
	}
	if len(tasks) == 0 {
		return proto.FactoryRun{}, errors.New("factory.start: no valid tasks provided")
	}

	run := &proto.FactoryRun{
		ID:              runID,
		Repo:            repo,
		Base:            base,
		Agent:           agent,
		CheckCmd:        strings.TrimSpace(p.CheckCmd),
		FlowPath:        strings.TrimSpace(p.FlowPath),
		FeishuChat:      strings.TrimSpace(p.FeishuChat),
		BrowserPane:     strings.TrimSpace(p.BrowserPane),
		MaxParallel:     maxParallel,
		MaxMinutes:      maxMinutes,
		MaxOutputKB:     maxOutputKB,
		MaxRepeatErrors: maxRepeatErrors,
		AutoEnqueue:     p.AutoEnqueue,
		Status:          "running",
		CreatedAt:       time.Now().UnixMilli(),
		Tasks:           tasks,
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	d.mu.Lock()
	if d.factoryCancels == nil {
		d.factoryCancels = make(map[string]context.CancelFunc)
	}
	d.factoryCancels[runID] = runCancel
	d.factoryRuns = append(d.factoryRuns, run)
	snap := cloneFactoryRun(run)
	d.mu.Unlock()
	d.changed()

	maxDur := time.Duration(maxMinutes) * time.Minute
	if p.MaxDurationMs > 0 {
		maxDur = time.Duration(p.MaxDurationMs) * time.Millisecond
	}

	go d.executeFactoryRun(runCtx, run, maxDur)
	return snap, nil
}

func (d *Daemon) executeFactoryRun(ctx context.Context, run *proto.FactoryRun, maxDur time.Duration) {
	sem := make(chan struct{}, run.MaxParallel)
	var wg sync.WaitGroup

	for i := range run.Tasks {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				d.mu.Lock()
				if run.Tasks[idx].Status == "queued" {
					run.Tasks[idx].Status = "stopped"
					run.Tasks[idx].EndedAt = time.Now().UnixMilli()
				}
				d.mu.Unlock()
				d.changed()
				return
			}
			d.runFactoryTask(ctx, run, idx, maxDur)
		}()
	}

	wg.Wait()

	d.mu.Lock()
	if run.Status == "running" {
		if ctx.Err() != nil {
			run.Status = "stopped"
		} else {
			run.Status = "done"
		}
		run.EndedAt = time.Now().UnixMilli()
	}
	if d.factoryCancels != nil {
		delete(d.factoryCancels, run.ID)
	}
	feishuChat := run.FeishuChat
	d.mu.Unlock()
	d.changed()

	if feishuChat != "" {
		_, _ = d.factoryDigest(proto.FactoryDigestParams{ID: run.ID, FeishuChat: feishuChat})
	}
}

func (d *Daemon) runFactoryTask(ctx context.Context, run *proto.FactoryRun, idx int, maxDur time.Duration) {
	d.mu.Lock()
	task := run.Tasks[idx]
	repo := run.Repo
	base := run.Base
	agent := run.Agent
	checkCmd := run.CheckCmd
	flowPath := run.FlowPath
	autoEnqueue := run.AutoEnqueue
	maxOutputKB := run.MaxOutputKB
	maxRepeatErrors := run.MaxRepeatErrors
	d.mu.Unlock()

	started := time.Now()
	sess, err := d.create(proto.CreateParams{
		Dir:      repo,
		Base:     base,
		Worktree: task.Branch,
		Name:     task.Title,
		Command:  agentPromptCommand(agent, task.Prompt),
	})
	if err != nil {
		d.mu.Lock()
		run.Tasks[idx].Status = "failed"
		run.Tasks[idx].Error = err.Error()
		run.Tasks[idx].StartedAt = started.UnixMilli()
		run.Tasks[idx].EndedAt = time.Now().UnixMilli()
		d.mu.Unlock()
		d.changed()
		return
	}

	var wtPath string
	if sess.Worktree != nil {
		wtPath = sess.Worktree.Path
	}
	d.mu.Lock()
	run.Tasks[idx].Status = "running"
	run.Tasks[idx].SessionID = sess.ID
	run.Tasks[idx].PaneID = sess.Pane.ID
	run.Tasks[idx].Worktree = wtPath
	run.Tasks[idx].StartedAt = started.UnixMilli()
	d.mu.Unlock()
	d.changed()

	tripReason := d.monitorFactoryPane(ctx, started, sess.Pane.ID, maxDur, maxOutputKB, maxRepeatErrors)
	if tripReason != "" {
		_ = d.closeSession(proto.CloseParams{ID: sess.ID})
		d.mu.Lock()
		if ctx.Err() != nil && tripReason == "stopped" {
			run.Tasks[idx].Status = "stopped"
		} else {
			run.Tasks[idx].Status = "tripped"
			run.Tasks[idx].TripReason = tripReason
		}
		run.Tasks[idx].EndedAt = time.Now().UnixMilli()
		d.mu.Unlock()
		d.changed()
		return
	}

	// Run optional acceptance check command in the task worktree.
	if checkCmd != "" && wtPath != "" {
		chkCtx, chkCancel := context.WithTimeout(ctx, 5*time.Minute)
		cmd := shellCmd(chkCtx, wtPath, checkCmd)
		out, err := cmd.CombinedOutput()
		chkCancel()
		if err != nil {
			d.mu.Lock()
			run.Tasks[idx].Status = "failed"
			run.Tasks[idx].Error = tailString(out, 1500)
			if run.Tasks[idx].Error == "" {
				run.Tasks[idx].Error = err.Error()
			}
			run.Tasks[idx].EndedAt = time.Now().UnixMilli()
			d.mu.Unlock()
			d.changed()
			return
		}
	}

	var laneID string
	if autoEnqueue {
		lane, err := d.mqAdd(proto.MergeQueueAddParams{
			Target:   sess.ID,
			Repo:     repo,
			Branch:   task.Branch,
			Base:     base,
			CheckCmd: checkCmd,
			FlowPath: flowPath,
		})
		if err == nil {
			laneID = lane.ID
		}
	}

	d.mu.Lock()
	run.Tasks[idx].Status = "verified"
	run.Tasks[idx].LaneID = laneID
	run.Tasks[idx].EndedAt = time.Now().UnixMilli()
	d.mu.Unlock()
	d.changed()
}

func (d *Daemon) monitorFactoryPane(ctx context.Context, started time.Time, paneID string, maxDur time.Duration, maxOutputKB, maxRepeatErrors int) string {
	poll := 250 * time.Millisecond
	if maxDur > 0 && maxDur < 2*time.Second {
		poll = 40 * time.Millisecond
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	wasAwaiting := false
	for {
		select {
		case <-ctx.Done():
			return "stopped"
		case <-ticker.C:
		}

		pn := d.findPane(paneID)
		if pn == nil {
			return ""
		}
		var closed, awaiting, cmdDone bool
		var histBytes int
		pn.mu.Lock()
		closed = pn.closed
		awaiting = pn.awaiting
		cmdDone = pn.info.CommandExit != nil
		if pn.term != nil {
			histBytes = len(pn.term.History())
		}
		pn.mu.Unlock()

		if awaiting {
			wasAwaiting = true
		}
		if closed || cmdDone || (wasAwaiting && !awaiting) {
			return ""
		}

		if maxDur > 0 && time.Since(started) > maxDur {
			return fmt.Sprintf("wall-clock limit exceeded (%s)", maxDur.Round(time.Millisecond))
		}
		if maxOutputKB > 0 && histBytes > maxOutputKB*1024 {
			return fmt.Sprintf("output runaway (%d KB > %d KB)", histBytes/1024, maxOutputKB)
		}
		if maxRepeatErrors > 0 {
			text := pn.readText(true, 100)
			if dup := repeatedErrorLine(text, maxRepeatErrors); dup != "" {
				return fmt.Sprintf("repeated error loop (%dx): %s", maxRepeatErrors, dup)
			}
		}
	}
}

func repeatedErrorLine(text string, threshold int) string {
	if threshold <= 1 {
		return ""
	}
	counts := make(map[string]int)
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if len(line) < 6 {
			continue
		}
		low := strings.ToLower(line)
		if strings.Contains(low, "error:") || strings.Contains(low, "fatal:") ||
			strings.Contains(low, "panic:") || strings.Contains(low, "fail\t") ||
			strings.HasPrefix(low, "fail:") {
			counts[line]++
			if counts[line] >= threshold {
				if len(line) > 120 {
					line = line[:117] + "..."
				}
				return line
			}
		}
	}
	return ""
}

func (d *Daemon) factoryList() []proto.FactoryRun {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]proto.FactoryRun, 0, len(d.factoryRuns))
	for _, r := range d.factoryRuns {
		out = append(out, cloneFactoryRun(r))
	}
	return out
}

func (d *Daemon) factoryStatus(id string) (proto.FactoryRun, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := d.findFactoryRunLocked(id)
	if r == nil {
		return proto.FactoryRun{}, fmt.Errorf("no factory run %q", id)
	}
	return cloneFactoryRun(r), nil
}

func (d *Daemon) factoryStop(id string) error {
	d.mu.Lock()
	r := d.findFactoryRunLocked(id)
	if r == nil {
		d.mu.Unlock()
		return fmt.Errorf("no factory run %q", id)
	}
	var cancel context.CancelFunc
	if d.factoryCancels != nil {
		cancel = d.factoryCancels[r.ID]
	}
	r.Status = "stopped"
	r.EndedAt = time.Now().UnixMilli()
	for i := range r.Tasks {
		if r.Tasks[i].Status == "queued" || r.Tasks[i].Status == "running" {
			r.Tasks[i].Status = "stopped"
			r.Tasks[i].EndedAt = r.EndedAt
		}
	}
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	d.changed()
	return nil
}

func (d *Daemon) factoryDigest(p proto.FactoryDigestParams) (proto.FactoryDigestResult, error) {
	d.mu.Lock()
	r := d.findFactoryRunLocked(p.ID)
	if r == nil {
		d.mu.Unlock()
		return proto.FactoryDigestResult{}, fmt.Errorf("no factory run %q", p.ID)
	}
	snap := cloneFactoryRun(r)
	d.mu.Unlock()

	var verified, failed, tripped, running, stopped int
	for _, t := range snap.Tasks {
		switch t.Status {
		case "verified":
			verified++
		case "failed":
			failed++
		case "tripped":
			tripped++
		case "running", "queued":
			running++
		case "stopped":
			stopped++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Hyla Factory Morning Digest (%s)\n", snap.ID)
	fmt.Fprintf(&b, "- Repo: `%s` (base `%s`, agent `%s`, status `%s`)\n", snap.Repo, snap.Base, snap.Agent, snap.Status)
	fmt.Fprintf(&b, "- Summary: **%d verified**, %d failed, %d circuit-breaker tripped, %d stopped, %d pending\n\n",
		verified, failed, tripped, stopped, running)
	for _, t := range snap.Tasks {
		extra := ""
		if t.LaneID != "" {
			extra = fmt.Sprintf(" -> MQ `%s`", t.LaneID)
		} else if t.TripReason != "" {
			extra = fmt.Sprintf(" (tripped: %s)", t.TripReason)
		} else if t.Error != "" {
			extra = fmt.Sprintf(" (error: %s)", strings.SplitN(t.Error, "\n", 2)[0])
		}
		fmt.Fprintf(&b, "- [%s] `%s` (%s)%s\n", t.Status, t.Title, t.Branch, extra)
	}
	md := b.String()

	sent := false
	chatID := strings.TrimSpace(p.FeishuChat)
	if chatID == "" {
		chatID = snap.FeishuChat
	}
	if chatID != "" {
		if bot := d.feishuBot(); bot != nil {
			title := fmt.Sprintf("Hyla Factory Digest: %d/%d verified", verified, len(snap.Tasks))
			if _, err := bot.SendApprovalCard(chatID, title, md, snap.ID, ""); err == nil {
				sent = true
			}
		}
	}
	return proto.FactoryDigestResult{
		RunID:    snap.ID,
		Markdown: md,
		Sent:     sent,
	}, nil
}
