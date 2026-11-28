package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"hyla/internal/git"
	"hyla/internal/proto"
)

func agentPromptCommand(agent, prompt string) []string {
	switch agent {
	case "claude":
		return []string{"claude", prompt}
	case "codex":
		return []string{"codex", prompt}
	case "opencode":
		return []string{"opencode", "--prompt", prompt}
	case "pi":
		return []string{"pi", prompt}
	case "agy", "antigravity":
		return []string{"agy", "-i", prompt}
	default:
		return []string{agent, prompt}
	}
}

func (d *Daemon) arenasPath() string { return filepath.Join(d.dir, "arenas.json") }

func (d *Daemon) loadArenas() {
	data, err := os.ReadFile(d.arenasPath())
	if err != nil {
		return
	}
	var list []*proto.Arena
	if json.Unmarshal(data, &list) != nil {
		return
	}
	d.mu.Lock()
	d.arenas = list
	d.mu.Unlock()
}

func (d *Daemon) saveArenas() {
	d.mu.Lock()
	data, err := json.MarshalIndent(d.arenas, "", "  ")
	d.mu.Unlock()
	if err == nil {
		writeAtomic(d.arenasPath(), data)
	}
}

func detectInstalledAgents() []string {
	var found []string
	for _, ag := range []string{"claude", "codex", "opencode", "pi", "agy"} {
		if _, err := exec.LookPath(ag); err == nil {
			found = append(found, ag)
		}
	}
	return found
}

func (d *Daemon) findArenaLocked(ref string) *proto.Arena {
	for _, a := range d.arenas {
		if a.ID == ref || (a.Name != "" && a.Name == ref) {
			return a
		}
	}
	var match *proto.Arena
	for _, a := range d.arenas {
		if strings.HasPrefix(a.ID, ref) {
			if match != nil {
				return nil
			}
			match = a
		}
	}
	return match
}

func (d *Daemon) arenaStart(p proto.ArenaStartParams) (proto.Arena, error) {
	prompt := strings.TrimSpace(p.Prompt)
	if prompt == "" {
		return proto.Arena{}, errors.New("arena.start: prompt is required")
	}
	agents := make([]string, 0, len(p.Agents))
	for _, a := range p.Agents {
		a = strings.TrimSpace(strings.ToLower(a))
		if a != "" {
			agents = append(agents, a)
		}
	}
	if len(agents) == 0 {
		agents = detectInstalledAgents()
		if len(agents) == 0 {
			return proto.Arena{}, errors.New("arena.start: no supported coding agents found (specify agents)")
		}
	}

	dir := strings.TrimSpace(p.Dir)
	if dir == "" {
		dir = proto.Home()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := git.Open(ctx, dir)
	if err != nil {
		return proto.Arena{}, fmt.Errorf("arena.start: %w", err)
	}
	repo := r.Main()
	if repo == "" {
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

	id := "ar_" + newID()
	now := time.Now().UnixMilli()
	ar := &proto.Arena{
		ID:        id,
		Name:      strings.TrimSpace(p.Name),
		Prompt:    prompt,
		Repo:      repo,
		Base:      base,
		TestCmd:   strings.TrimSpace(p.TestCmd),
		Status:    "running",
		CreatedAt: now,
	}

	var created []string
	cleanupCreated := func() {
		for _, sid := range created {
			_ = d.closeSession(proto.CloseParams{ID: sid, RemoveWorktree: true, Force: true})
		}
	}

	seenAgent := map[string]int{}
	for _, agent := range agents {
		seenAgent[agent]++
		suffix := agent
		if seenAgent[agent] > 1 {
			suffix = fmt.Sprintf("%s-%d", agent, seenAgent[agent])
		}
		branch := fmt.Sprintf("arena/%s/%s", id, suffix)
		sname := fmt.Sprintf("arena:%s:%s", id, suffix)
		if ar.Name != "" {
			sname = fmt.Sprintf("%s:%s", ar.Name, suffix)
		}
		ses, err := d.create(proto.CreateParams{
			Dir:      repo,
			Worktree: branch,
			Base:     base,
			Name:     sname,
			Command:  agentPromptCommand(agent, prompt),
		})
		if err != nil {
			cleanupCreated()
			return proto.Arena{}, fmt.Errorf("arena.start (%s): %w", agent, err)
		}
		created = append(created, ses.ID)
		paneID := ""
		wtPath := ""
		if len(ses.Panes) > 0 {
			paneID = ses.Panes[0].ID
		}
		if ses.Worktree != nil {
			wtPath = ses.Worktree.Path
		}
		ar.Contestants = append(ar.Contestants, proto.ArenaContestant{
			Agent:     agent,
			SessionID: ses.ID,
			PaneID:    paneID,
			Branch:    branch,
			Worktree:  wtPath,
			Status:    proto.Working,
			StartedAt: now,
		})
	}

	d.mu.Lock()
	d.arenas = slices.Insert(d.arenas, 0, ar)
	d.refreshArenaLocked(ar)
	out := cloneArena(ar)
	d.mu.Unlock()
	d.saveArenas()
	return out, nil
}

func cloneArena(a *proto.Arena) proto.Arena {
	cp := *a
	cp.Contestants = slices.Clone(a.Contestants)
	return cp
}

func (d *Daemon) refreshArenaLocked(a *proto.Arena) {
	for i := range a.Contestants {
		c := &a.Contestants[i]
		if s := d.findSessionLocked(c.SessionID); s != nil {
			info := d.sessionInfo(s)
			c.Status = info.Status
		} else if c.Status == "" || c.Status == proto.Working || c.Status == proto.Idle || c.Status == proto.Waiting {
			c.Status = proto.Exited
		}
	}
}

func refreshArenaGit(a *proto.Arena) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for i := range a.Contestants {
		c := &a.Contestants[i]
		if c.Worktree == "" {
			continue
		}
		if _, err := os.Stat(c.Worktree); err != nil {
			continue
		}
		if out, err := execGit(ctx, c.Worktree, "status", "--porcelain"); err == nil {
			c.Files = countNonEmptyLines(out)
		}
		if a.Base != "" {
			if out, err := execGit(ctx, c.Worktree, "diff", "--numstat", a.Base); err == nil {
				c.Added, c.Removed = sumNumstat(out)
			}
			if out, err := execGit(ctx, c.Worktree, "rev-list", "--count", a.Base+"..HEAD"); err == nil {
				c.Commits, _ = strconv.Atoi(strings.TrimSpace(string(out)))
			}
		}
	}
}

func countNonEmptyLines(b []byte) int {
	n := 0
	for _, l := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			n++
		}
	}
	return n
}

func sumNumstat(b []byte) (added, removed int) {
	for _, l := range bytes.Split(b, []byte("\n")) {
		f := strings.Fields(string(l))
		if len(f) < 2 {
			continue
		}
		a, _ := strconv.Atoi(f[0])
		r, _ := strconv.Atoi(f[1])
		added += a
		removed += r
	}
	return added, removed
}

func execGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-c", "core.quotepath=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("git %s: %s", args[0], msg)
	}
	return stdout.Bytes(), nil
}

func shellCmd(ctx context.Context, dir, script string) *exec.Cmd {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", script)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", script)
	}
	cmd.Dir = dir
	return cmd
}

func tailString(b []byte, maxBytes int) string {
	b = bytes.TrimSpace(b)
	if len(b) <= maxBytes {
		return string(b)
	}
	return string(b[len(b)-maxBytes:])
}

func (d *Daemon) arenaList() []proto.Arena {
	d.mu.Lock()
	out := make([]proto.Arena, len(d.arenas))
	for i, a := range d.arenas {
		d.refreshArenaLocked(a)
		out[i] = cloneArena(a)
	}
	d.mu.Unlock()
	for i := range out {
		refreshArenaGit(&out[i])
	}
	return out
}

func (d *Daemon) arenaStatus(id string) (proto.Arena, error) {
	d.mu.Lock()
	a := d.findArenaLocked(id)
	if a == nil {
		d.mu.Unlock()
		return proto.Arena{}, fmt.Errorf("no arena %q", id)
	}
	d.refreshArenaLocked(a)
	out := cloneArena(a)
	d.mu.Unlock()
	refreshArenaGit(&out)
	return out, nil
}

func (d *Daemon) arenaTest(p proto.ArenaTestParams) (proto.Arena, error) {
	d.mu.Lock()
	a := d.findArenaLocked(p.ID)
	if a == nil {
		d.mu.Unlock()
		return proto.Arena{}, fmt.Errorf("no arena %q", p.ID)
	}
	cmdStr := strings.TrimSpace(p.Cmd)
	if cmdStr == "" {
		cmdStr = a.TestCmd
	}
	if cmdStr == "" {
		d.mu.Unlock()
		return proto.Arena{}, errors.New("arena.test: test command is required")
	}
	a.TestCmd = cmdStr
	contestants := slices.Clone(a.Contestants)
	d.mu.Unlock()

	type checkRes struct {
		idx      int
		status   string
		exitCode int
		durMs    int64
		output   string
	}
	results := make([]checkRes, len(contestants))
	var wg sync.WaitGroup
	for i, c := range contestants {
		wg.Add(1)
		go func(idx int, wt string) {
			defer wg.Done()
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			cmd := shellCmd(ctx, wt, cmdStr)
			out, err := cmd.CombinedOutput()
			durMs := time.Since(start).Milliseconds()
			if err == nil {
				results[idx] = checkRes{idx: idx, status: "passed", exitCode: 0, durMs: durMs, output: tailString(out, 4096)}
				return
			}
			code := 1
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				code = ee.ExitCode()
			}
			outStr := tailString(out, 4096)
			if outStr == "" {
				outStr = err.Error()
			}
			results[idx] = checkRes{idx: idx, status: "failed", exitCode: code, durMs: durMs, output: outStr}
		}(i, c.Worktree)
	}
	wg.Wait()

	d.mu.Lock()
	a = d.findArenaLocked(p.ID)
	if a == nil {
		d.mu.Unlock()
		return proto.Arena{}, fmt.Errorf("no arena %q", p.ID)
	}
	for _, r := range results {
		if r.idx < len(a.Contestants) {
			a.Contestants[r.idx].CheckStatus = r.status
			a.Contestants[r.idx].CheckExitCode = r.exitCode
			a.Contestants[r.idx].CheckDurationMs = r.durMs
			a.Contestants[r.idx].CheckOutput = r.output
		}
	}
	d.refreshArenaLocked(a)
	out := cloneArena(a)
	d.mu.Unlock()
	refreshArenaGit(&out)
	d.saveArenas()
	return out, nil
}

func findWinnerContestant(cs []proto.ArenaContestant, ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	for i, c := range cs {
		if c.Agent == ref || c.SessionID == ref || c.Branch == ref {
			return i, nil
		}
	}
	match := -1
	for i, c := range cs {
		if strings.HasPrefix(c.SessionID, ref) || strings.HasPrefix(c.Agent, ref) {
			if match >= 0 {
				return -1, fmt.Errorf("ambiguous winner %q", ref)
			}
			match = i
		}
	}
	if match < 0 {
		return -1, fmt.Errorf("no contestant %q in arena", ref)
	}
	return match, nil
}

func (d *Daemon) arenaPick(p proto.ArenaPickParams) (proto.Arena, error) {
	d.mu.Lock()
	a := d.findArenaLocked(p.ID)
	if a == nil {
		d.mu.Unlock()
		return proto.Arena{}, fmt.Errorf("no arena %q", p.ID)
	}
	snap := cloneArena(a)
	d.mu.Unlock()

	idx, err := findWinnerContestant(snap.Contestants, p.Winner)
	if err != nil {
		return proto.Arena{}, err
	}
	winner := snap.Contestants[idx]

	if !p.NoMerge {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if stOut, err := execGit(ctx, winner.Worktree, "status", "--porcelain"); err == nil && countNonEmptyLines(stOut) > 0 {
			return proto.Arena{}, fmt.Errorf("winner worktree (%s) has uncommitted changes; commit them or pass --no-merge", winner.Agent)
		}
		if snap.Base != "" && snap.Base != "HEAD" {
			curOut, err := execGit(ctx, snap.Repo, "rev-parse", "--abbrev-ref", "HEAD")
			if err == nil && strings.TrimSpace(string(curOut)) != snap.Base {
				if _, err := execGit(ctx, snap.Repo, "checkout", snap.Base); err != nil {
					return proto.Arena{}, err
				}
			}
		}
		if _, err := execGit(ctx, snap.Repo, "merge", "--no-edit", winner.Branch); err != nil {
			_, _ = execGit(ctx, snap.Repo, "merge", "--abort")
			return proto.Arena{}, err
		}
	}

	if !p.KeepOthers {
		for i, c := range snap.Contestants {
			if i == idx {
				continue
			}
			_ = d.closeSession(proto.CloseParams{ID: c.SessionID, RemoveWorktree: true, Force: true})
		}
		if !p.NoMerge {
			_ = d.closeSession(proto.CloseParams{ID: winner.SessionID, RemoveWorktree: true, Force: true})
		}
	}

	d.mu.Lock()
	a = d.findArenaLocked(p.ID)
	if a == nil {
		d.mu.Unlock()
		return proto.Arena{}, fmt.Errorf("no arena %q", p.ID)
	}
	a.Status = "picked"
	a.Winner = winner.Agent
	d.refreshArenaLocked(a)
	out := cloneArena(a)
	d.mu.Unlock()
	d.saveArenas()
	return out, nil
}

func (d *Daemon) arenaClose(p proto.ArenaCloseParams) error {
	d.mu.Lock()
	a := d.findArenaLocked(p.ID)
	if a == nil {
		d.mu.Unlock()
		return fmt.Errorf("no arena %q", p.ID)
	}
	contestants := slices.Clone(a.Contestants)
	d.mu.Unlock()

	for _, c := range contestants {
		if err := d.closeSession(proto.CloseParams{ID: c.SessionID, RemoveWorktree: true, Force: p.Force}); err != nil {
			if !strings.Contains(err.Error(), "no session") {
				return err
			}
		}
	}

	d.mu.Lock()
	if a := d.findArenaLocked(p.ID); a != nil {
		a.Status = "closed"
	}
	d.mu.Unlock()
	d.saveArenas()
	return nil
}
