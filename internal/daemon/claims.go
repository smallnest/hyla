package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"hyla/internal/git"
	"hyla/internal/proto"
)

func canonicalRepoRoot(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if r, err := git.Open(ctx, dir); err == nil {
		if m := r.Main(); m != "" {
			return filepath.Clean(m)
		}
		return filepath.Clean(r.Root)
	}
	return filepath.Clean(dir)
}

func (d *Daemon) pruneExpiredClaimsLocked(now int64) bool {
	if len(d.pathClaims) == 0 {
		return false
	}
	kept := d.pathClaims[:0]
	changed := false
	for _, c := range d.pathClaims {
		if c.ExpiresAt > 0 && c.ExpiresAt <= now {
			changed = true
			continue
		}
		kept = append(kept, c)
	}
	if changed {
		d.pathClaims = kept
	}
	return changed
}

func (d *Daemon) claimAdd(p proto.PathClaimParams) (proto.PathClaim, error) {
	pat := strings.TrimSpace(p.Pattern)
	if pat == "" {
		return proto.PathClaim{}, errors.New("claim.add: pattern is required")
	}
	mode := strings.ToLower(strings.TrimSpace(p.Mode))
	switch mode {
	case "", "warn":
		mode = "warn"
	case "block":
	default:
		return proto.PathClaim{}, fmt.Errorf("claim.add: invalid mode %q (want warn or block)", p.Mode)
	}

	var sid, pid, agent, paneDir string
	if p.Target != "" {
		pn := d.findPane(p.Target)
		if pn == nil {
			return proto.PathClaim{}, fmt.Errorf("no pane or session %q", p.Target)
		}
		pn.mu.Lock()
		sid = pn.sid
		pid = pn.info.ID
		agent = pn.info.Agent
		paneDir = pn.info.WorkDir()
		pn.mu.Unlock()
	}

	repo := strings.TrimSpace(p.Repo)
	if repo == "" {
		repo = paneDir
	}
	if repo == "" {
		return proto.PathClaim{}, errors.New("claim.add: repo or target is required")
	}
	repo = canonicalRepoRoot(repo)

	now := time.Now().UnixMilli()
	claim := proto.PathClaim{
		ID:        "clm_" + newID(),
		Repo:      repo,
		Pattern:   normalizeClaimPattern(repo, pat),
		Mode:      mode,
		SessionID: sid,
		PaneID:    pid,
		Agent:     agent,
		Reason:    strings.TrimSpace(p.Reason),
		CreatedAt: now,
	}
	if p.TTLSeconds > 0 {
		claim.ExpiresAt = now + p.TTLSeconds*1000
	}

	d.mu.Lock()
	d.pruneExpiredClaimsLocked(now)
	d.pathClaims = append(d.pathClaims, &claim)
	d.mu.Unlock()
	d.changed()
	return claim, nil
}

func (d *Daemon) claimRelease(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("claim.release: id is required")
	}
	now := time.Now().UnixMilli()
	d.mu.Lock()
	d.pruneExpiredClaimsLocked(now)
	idx := -1
	for i, c := range d.pathClaims {
		if c.ID == id {
			idx = i
			break
		}
		if strings.HasPrefix(c.ID, id) {
			if idx != -1 {
				d.mu.Unlock()
				return fmt.Errorf("ambiguous claim id %q", id)
			}
			idx = i
		}
	}
	if idx == -1 {
		d.mu.Unlock()
		return fmt.Errorf("no claim %q", id)
	}
	d.pathClaims = append(d.pathClaims[:idx], d.pathClaims[idx+1:]...)
	d.mu.Unlock()
	d.changed()
	return nil
}

func (d *Daemon) claimList(repo string) []proto.PathClaim {
	repo = canonicalRepoRoot(repo)
	now := time.Now().UnixMilli()
	d.mu.Lock()
	pruned := d.pruneExpiredClaimsLocked(now)
	out := make([]proto.PathClaim, 0, len(d.pathClaims))
	for _, c := range d.pathClaims {
		if repo != "" && c.Repo != repo {
			continue
		}
		out = append(out, *c)
	}
	d.mu.Unlock()
	if pruned {
		d.changed()
	}
	return out
}

func (d *Daemon) claimCheck(p proto.PathClaimCheckParams) proto.PathClaimCheckResult {
	repo := strings.TrimSpace(p.Repo)
	if repo == "" && p.Pane != "" {
		if pn := d.findPane(p.Pane); pn != nil {
			pn.mu.Lock()
			repo = pn.info.WorkDir()
			pn.mu.Unlock()
		}
	}
	if repo != "" {
		repo = canonicalRepoRoot(repo)
	}
	rel := normalizeRelPath(repo, p.Path)

	now := time.Now().UnixMilli()
	d.mu.Lock()
	pruned := d.pruneExpiredClaimsLocked(now)
	var conflicts []proto.PathClaim
	allowed := true
	for _, c := range d.pathClaims {
		if repo != "" && c.Repo != repo {
			continue
		}
		if p.Pane != "" && c.PaneID == p.Pane {
			continue
		}
		if claimMatches(c.Pattern, rel) {
			conflicts = append(conflicts, *c)
			if c.Mode == "block" {
				allowed = false
			}
		}
	}
	d.mu.Unlock()
	if pruned {
		d.changed()
	}

	res := proto.PathClaimCheckResult{
		Allowed:   allowed,
		Conflicts: conflicts,
	}
	if len(conflicts) > 0 {
		var parts []string
		for _, c := range conflicts {
			owner := c.PaneID
			if c.Agent != "" {
				owner = c.Agent
				if c.PaneID != "" {
					owner += " (" + c.PaneID + ")"
				}
			}
			msg := fmt.Sprintf("%s claimed %q [%s]", owner, c.Pattern, c.Mode)
			if c.Reason != "" {
				msg += ": " + c.Reason
			}
			parts = append(parts, msg)
		}
		res.Message = strings.Join(parts, "; ")
	}
	return res
}

func normalizeClaimPattern(repo, pat string) string {
	pat = strings.TrimSpace(pat)
	if repo != "" && filepath.IsAbs(pat) {
		if rel, err := filepath.Rel(repo, pat); err == nil && !strings.HasPrefix(rel, "..") {
			pat = rel
		}
	}
	pat = filepath.ToSlash(filepath.Clean(pat))
	pat = strings.TrimPrefix(pat, "./")
	pat = strings.TrimPrefix(pat, "/")
	return pat
}

func normalizeRelPath(repo, target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if repo != "" {
		if rel, err := filepath.Rel(repo, target); err == nil && !strings.HasPrefix(rel, "..") {
			target = rel
		}
	}
	target = filepath.ToSlash(filepath.Clean(target))
	target = strings.TrimPrefix(target, "./")
	target = strings.TrimPrefix(target, "/")
	return target
}

func claimMatches(pattern, rel string) bool {
	if pattern == "" || rel == "" {
		return false
	}
	if pattern == "*" || pattern == "**" || pattern == "." {
		return true
	}
	p := strings.TrimSuffix(pattern, "/")
	if p == rel {
		return true
	}
	if strings.HasSuffix(pattern, "/") && strings.HasPrefix(rel, pattern) {
		return true
	}
	if strings.HasPrefix(rel, p+"/") {
		return true
	}
	if strings.HasSuffix(p, "/**") {
		prefix := strings.TrimSuffix(p, "/**")
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	if ok, _ := path.Match(p, rel); ok {
		return true
	}
	if !strings.Contains(p, "/") {
		if ok, _ := path.Match(p, path.Base(rel)); ok {
			return true
		}
	}
	return false
}

func (d *Daemon) clashCheck(p proto.ClashCheckParams) (proto.ClashCheckResult, error) {
	started := time.Now()
	repo := strings.TrimSpace(p.Repo)
	if repo == "" {
		return proto.ClashCheckResult{}, errors.New("clash.check: repo is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	r, err := git.Open(ctx, repo)
	if err != nil {
		return proto.ClashCheckResult{}, fmt.Errorf("clash.check: %w", err)
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

	var branches []string
	for _, b := range p.Branches {
		if b = strings.TrimSpace(b); b != "" {
			branches = append(branches, b)
		}
	}
	if len(branches) == 0 {
		d.mu.Lock()
		for _, s := range d.sessions {
			if s.worktree != nil && filepath.Clean(s.worktree.Main) == filepath.Clean(repo) && s.worktree.Branch != "" && s.worktree.Branch != base {
				branches = append(branches, s.worktree.Branch)
			}
		}
		d.mu.Unlock()
	}

	res := proto.ClashCheckResult{
		Passed:   true,
		Base:     base,
		Branches: branches,
	}
	if len(branches) < 2 && p.CheckCmd == "" {
		res.DurationMs = time.Since(started).Milliseconds()
		return res, nil
	}

	tempWT := filepath.Join(os.TempDir(), "hyla-clash-"+newID())
	if _, err := execGit(ctx, repo, "worktree", "add", "--detach", tempWT, base); err != nil {
		return res, fmt.Errorf("clash.check: create trial worktree: %w", err)
	}
	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanCancel()
		_, _ = execGit(cleanCtx, repo, "worktree", "remove", "--force", tempWT)
	}()

	for _, br := range branches {
		if _, err := execGit(ctx, tempWT, "-c", "user.name=Hyla", "-c", "user.email=hyla@local", "merge", "--no-ff", "--no-edit", br); err != nil {
			res.Passed = false
			if out, derr := execGit(ctx, tempWT, "diff", "--name-only", "--diff-filter=U"); derr == nil {
				for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
					if f := strings.TrimSpace(line); f != "" {
						res.ConflictFiles = append(res.ConflictFiles, f)
					}
				}
			}
			_, _ = execGit(ctx, tempWT, "merge", "--abort")
			res.CheckError = fmt.Sprintf("merge conflict on %s: %v", br, err)
			res.DurationMs = time.Since(started).Milliseconds()
			return res, nil
		}
	}

	if cmdStr := strings.TrimSpace(p.CheckCmd); cmdStr != "" {
		cmd := shellCmd(ctx, tempWT, cmdStr)
		if out, err := cmd.CombinedOutput(); err != nil {
			res.Passed = false
			res.CheckError = tailString(out, 2000)
			if res.CheckError == "" {
				res.CheckError = err.Error()
			}
		}
	}
	res.DurationMs = time.Since(started).Milliseconds()
	return res, nil
}
