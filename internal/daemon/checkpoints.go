package daemon

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"hyla/internal/git"
	"hyla/internal/proto"
)

const maxStoredCheckpoints = 300

func (d *Daemon) recordTurnCheckpoint(p *pane, root, tree string, at int64) {
	if p == nil || root == "" || tree == "" {
		return
	}
	root = canonicalRepoRoot(root)
	p.mu.Lock()
	sid := p.sid
	pid := p.info.ID
	agent := p.info.Agent
	agentSID := p.info.AgentSessionID
	p.mu.Unlock()

	d.mu.Lock()
	for i := len(d.checkpoints) - 1; i >= 0; i-- {
		last := d.checkpoints[i]
		if last.PaneID == pid {
			if last.Root == root && last.Tree == tree {
				d.mu.Unlock()
				return
			}
			break
		}
	}
	cp := &proto.Checkpoint{
		ID:             "cp_" + newID(),
		SessionID:      sid,
		PaneID:         pid,
		Agent:          agent,
		AgentSessionID: agentSID,
		Root:           root,
		Tree:           tree,
		Auto:           true,
		At:             at,
	}
	d.checkpoints = append(d.checkpoints, cp)
	if len(d.checkpoints) > maxStoredCheckpoints {
		d.checkpoints = d.checkpoints[len(d.checkpoints)-maxStoredCheckpoints:]
	}
	d.mu.Unlock()
	d.changed()
}

func (d *Daemon) findCheckpointLocked(ref string) (*proto.Checkpoint, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, errors.New("checkpoint id is required")
	}
	for _, cp := range d.checkpoints {
		if cp.ID == ref {
			return cp, nil
		}
	}
	var match *proto.Checkpoint
	for _, cp := range d.checkpoints {
		if strings.HasPrefix(cp.ID, ref) {
			if match != nil {
				return nil, fmt.Errorf("ambiguous checkpoint id %q", ref)
			}
			match = cp
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no checkpoint %q", ref)
	}
	return match, nil
}

func (d *Daemon) checkpointList(p proto.CheckpointListParams) []proto.Checkpoint {
	target := strings.TrimSpace(p.Target)
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	var targetSID, targetPID, targetRoot string
	if target != "" {
		if pn := d.findPane(target); pn != nil {
			pn.mu.Lock()
			targetSID = pn.sid
			targetPID = pn.info.ID
			pn.mu.Unlock()
		} else {
			targetRoot = canonicalRepoRoot(target)
		}
	}

	d.mu.Lock()
	out := make([]proto.Checkpoint, 0, len(d.checkpoints))
	for _, cp := range d.checkpoints {
		if target != "" {
			if targetPID != "" {
				if cp.PaneID != targetPID && cp.SessionID != targetSID {
					continue
				}
			} else if canonicalRepoRoot(cp.Root) != targetRoot && cp.SessionID != target && cp.PaneID != target {
				continue
			}
		}
		out = append(out, *cp)
	}
	d.mu.Unlock()

	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (d *Daemon) checkpointCreate(p proto.CheckpointCreateParams) (proto.Checkpoint, error) {
	var sid, pid, agent, agentSID, dir string
	target := strings.TrimSpace(p.Target)
	if target != "" {
		if pn := d.findPane(target); pn != nil {
			pn.mu.Lock()
			sid = pn.sid
			pid = pn.info.ID
			agent = pn.info.Agent
			agentSID = pn.info.AgentSessionID
			dir = pn.info.WorkDir()
			pn.mu.Unlock()
		} else {
			dir = target
		}
	}
	if dir == "" {
		return proto.Checkpoint{}, errors.New("checkpoint.create: target pane or directory is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r, err := git.Open(ctx, dir)
	if err != nil {
		return proto.Checkpoint{}, fmt.Errorf("checkpoint.create: %w", err)
	}
	tree, err := r.Snapshot(ctx)
	if err != nil {
		return proto.Checkpoint{}, fmt.Errorf("checkpoint.create: %w", err)
	}
	cp := proto.Checkpoint{
		ID:             "cp_" + newID(),
		SessionID:      sid,
		PaneID:         pid,
		Agent:          agent,
		AgentSessionID: agentSID,
		Root:           r.Root,
		Tree:           tree,
		Label:          strings.TrimSpace(p.Label),
		Auto:           false,
		At:             time.Now().UnixMilli(),
	}
	d.mu.Lock()
	d.checkpoints = append(d.checkpoints, &cp)
	if len(d.checkpoints) > maxStoredCheckpoints {
		d.checkpoints = d.checkpoints[len(d.checkpoints)-maxStoredCheckpoints:]
	}
	d.mu.Unlock()
	d.changed()
	return cp, nil
}

func (d *Daemon) checkpointRestore(p proto.CheckpointRestoreParams) (proto.CheckpointRestoreResult, error) {
	d.mu.Lock()
	cp, err := d.findCheckpointLocked(p.ID)
	var copyCP proto.Checkpoint
	if cp != nil {
		copyCP = *cp
	}
	d.mu.Unlock()
	if err != nil {
		return proto.CheckpointRestoreResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := git.Open(ctx, copyCP.Root)
	if err != nil {
		return proto.CheckpointRestoreResult{}, fmt.Errorf("checkpoint.restore: %w", err)
	}

	// Capture a safety backup before overwriting the working tree.
	var safetyID string
	if preTree, err := r.Snapshot(ctx); err == nil && preTree != copyCP.Tree {
		backup := proto.Checkpoint{
			ID:        "cp_" + newID(),
			SessionID: copyCP.SessionID,
			PaneID:    copyCP.PaneID,
			Agent:     copyCP.Agent,
			Root:      r.Root,
			Tree:      preTree,
			Label:     "pre-restore safety backup",
			Auto:      false,
			At:        time.Now().UnixMilli(),
		}
		d.mu.Lock()
		d.checkpoints = append(d.checkpoints, &backup)
		d.mu.Unlock()
		safetyID = backup.ID
	}

	if err := r.RestoreTree(ctx, copyCP.Tree); err != nil {
		return proto.CheckpointRestoreResult{}, fmt.Errorf("checkpoint.restore: %w", err)
	}
	d.changed()
	return proto.CheckpointRestoreResult{
		Restored:       copyCP,
		SafetyBackupID: safetyID,
	}, nil
}

func (d *Daemon) checkpointFork(p proto.CheckpointForkParams) (proto.CheckpointForkResult, error) {
	d.mu.Lock()
	cp, err := d.findCheckpointLocked(p.ID)
	var copyCP proto.Checkpoint
	if cp != nil {
		copyCP = *cp
	}
	d.mu.Unlock()
	if err != nil {
		return proto.CheckpointForkResult{}, err
	}

	branch := strings.TrimSpace(p.Branch)
	if branch == "" {
		short := strings.TrimPrefix(copyCP.ID, "cp_")
		if len(short) > 6 {
			short = short[:6]
		}
		branch = "fork-" + short
	}

	sess, err := d.create(proto.CreateParams{
		Dir:      copyCP.Root,
		Worktree: branch,
		Base:     "HEAD",
		Name:     branch,
	})
	if err != nil {
		return proto.CheckpointForkResult{}, fmt.Errorf("checkpoint.fork: %w", err)
	}
	if sess.Worktree != nil && sess.Worktree.Path != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if r, err := git.Open(ctx, sess.Worktree.Path); err == nil {
			if err := r.RestoreTree(ctx, copyCP.Tree); err != nil {
				return proto.CheckpointForkResult{}, fmt.Errorf("checkpoint.fork restore tree: %w", err)
			}
		}
	}
	return proto.CheckpointForkResult{
		Session:    sess,
		Checkpoint: copyCP,
		Branch:     branch,
	}, nil
}
