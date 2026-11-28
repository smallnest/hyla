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
	"strings"
	"time"

	"hyla/internal/proto"
)

var errOpenCodeLegacyCLI = errors.New("opencode cli lacks api subcommand")

// forkCommand returns the direct CLI argv that forks a's session in a new
// terminal pane, or nil when the agent is unsupported, the ID is invalid,
// or the agent requires a pre-fork step (opencode v2).
func forkCommand(a *proto.AgentSession) []string {
	if a == nil || !proto.ValidAgentSessionID(a.ID) {
		return nil
	}
	switch a.Agent {
	case "claude":
		return []string{"claude", "--resume", a.ID, "--fork-session"}
	case "codex":
		return []string{"codex", "fork", a.ID}
	case "pi":
		return []string{"pi", "--fork", a.ID}
	case "agy", "antigravity":
		return []string{"agy", "--resume", a.ID}
	}
	return nil
}

// agyConversationFromPath extracts an Antigravity CLI conversation ID from an
// open file path, returning its ID and priority (2 for presence lock, 1 for
// conversation SQLite DB).
func agyConversationFromPath(path string) (string, int) {
	p := filepath.ToSlash(path)
	if strings.Contains(p, "/presence/") && strings.HasSuffix(p, ".lock") {
		id := strings.TrimSuffix(filepath.Base(p), ".lock")
		if proto.ValidAgentSessionID(id) {
			return id, 2
		}
	}
	if strings.Contains(p, "/conversations/") && strings.HasSuffix(p, ".db") {
		id := strings.TrimSuffix(filepath.Base(p), ".db")
		if proto.ValidAgentSessionID(id) {
			return id, 1
		}
	}
	return "", 0
}

func agyPromptReady(screen string) bool {
	if strings.Contains(screen, "Signing in...") || strings.Contains(screen, "Do you trust the contents") {
		return false
	}
	lines := strings.Split(screen, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "────────") {
			continue
		}
		for j := i + 1; j < len(lines) && j <= i+3; j++ {
			if strings.TrimSpace(lines[j]) != ">" {
				continue
			}
			for k := j + 1; k < len(lines) && k <= j+3; k++ {
				if strings.Contains(lines[k], "────────") {
					return true
				}
			}
		}
	}
	return false
}

// startAgyFork sends `/fork` to a newly resumed Antigravity CLI pane once its
// prompt box is ready. Overridden in tests.
var startAgyFork = triggerAgyFork

func triggerAgyFork(pn *pane) {
	if pn == nil {
		return
	}
	go func() {
		deadline := time.Now().Add(20 * time.Second)
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for time.Now().Before(deadline) {
			<-ticker.C
			pn.mu.Lock()
			if pn.closed || pn.info.Status == proto.Exited {
				pn.mu.Unlock()
				return
			}
			agent := pn.info.Agent
			screen := pn.screen()
			proc := pn.proc
			pn.mu.Unlock()
			if proc == nil || (agent != "agy" && agent != "antigravity") || !agyPromptReady(screen) {
				continue
			}
			time.Sleep(300 * time.Millisecond)
			pn.mu.Lock()
			if pn.closed || pn.info.Status == proto.Exited || pn.proc == nil {
				pn.mu.Unlock()
				return
			}
			proc = pn.proc
			pn.mu.Unlock()
			_, _ = proc.Write([]byte("/fork"))
			time.Sleep(150 * time.Millisecond)
			_, _ = proc.Write([]byte("\r"))
			return
		}
	}()
}

// runOpenCodeFork forks an OpenCode session via its background server API
// and returns the new child session ID. Overridden in tests.
var runOpenCodeFork = defaultOpenCodeFork

func findOpenCodeExe() (string, error) {
	if exe, err := exec.LookPath("opencode"); err == nil {
		return exe, nil
	}
	candidate := filepath.Join(proto.Home(), ".opencode", "bin", "opencode")
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate, nil
	}
	return "", errors.New("opencode executable not found")
}

func isOpenCodeLegacyOutput(combined string) bool {
	lower := strings.ToLower(combined)
	if strings.Contains(lower, "unknown command") || strings.Contains(lower, "unknown subcommand") || strings.Contains(lower, "unexpected argument") {
		return true
	}
	// OpenCode 1.x uses yargs, which prints top-level help and exits 1 when
	// given an unknown subcommand like "api", without an "unknown command" line.
	return strings.Contains(lower, "commands:") &&
		strings.Contains(lower, "options:") &&
		strings.Contains(lower, "--fork") &&
		!strings.Contains(lower, "opencode api")
}

func defaultOpenCodeFork(ctx context.Context, id, dir string) (string, error) {
	exe, err := findOpenCodeExe()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, exe, "api", "session.fork", "--param", "sessionID="+id)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		combined := strings.TrimSpace(stderr.String() + "\n" + stdout.String())
		if isOpenCodeLegacyOutput(combined) {
			return "", errOpenCodeLegacyCLI
		}
		if msg := extractJSONErrorMessage(stdout.Bytes(), stderr.Bytes()); msg != "" {
			return "", fmt.Errorf("opencode fork failed: %s", msg)
		}
		if combined != "" {
			return "", fmt.Errorf("opencode fork failed: %s", combined)
		}
		return "", fmt.Errorf("opencode fork failed: %w", err)
	}
	var payload struct {
		ID   string `json:"id"`
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(stdout.Bytes())))
	if err := dec.Decode(&payload); err != nil {
		return "", fmt.Errorf("opencode fork returned invalid JSON: %w", err)
	}
	newID := payload.Data.ID
	if newID == "" {
		newID = payload.ID
	}
	if !proto.ValidAgentSessionID(newID) {
		return "", fmt.Errorf("opencode fork returned invalid session id %q", newID)
	}
	return newID, nil
}

func extractJSONErrorMessage(sources ...[]byte) string {
	for _, src := range sources {
		trimmed := bytes.TrimSpace(src)
		if len(trimmed) == 0 {
			continue
		}
		var obj struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if json.NewDecoder(bytes.NewReader(trimmed)).Decode(&obj) == nil {
			if obj.Message != "" {
				return obj.Message
			}
			if obj.Error != "" {
				return obj.Error
			}
		}
	}
	return ""
}

// resolveForkCommand resolves the command argv that opens a forked copy of
// the agent session in a new pane.
func resolveForkCommand(ctx context.Context, a *proto.AgentSession, dir string) ([]string, error) {
	if a == nil || !proto.ValidAgentSessionID(a.ID) || !proto.CanForkAgent(a.Agent) {
		return nil, errors.New("agent session is not forkable")
	}
	if a.Agent == "opencode" {
		newID, err := runOpenCodeFork(ctx, a.ID, dir)
		if errors.Is(err, errOpenCodeLegacyCLI) {
			return []string{"opencode", "--session", a.ID, "--fork"}, nil
		}
		if err != nil {
			return nil, err
		}
		return []string{"opencode", "--session", newID}, nil
	}
	cmd := forkCommand(a)
	if len(cmd) == 0 {
		return nil, fmt.Errorf("agent %s does not support session fork", a.Agent)
	}
	return cmd, nil
}

// forkPane forks the pane's active agent session into a new split pane.
func (d *Daemon) forkPane(p proto.ForkParams) (proto.Session, error) {
	d.mu.Lock()
	at := d.findPaneLocked(p.Pane)
	if at == nil {
		if s := d.findSessionLocked(p.Pane); s != nil {
			at = focusedLocked(s)
		}
	}
	if at == nil {
		d.mu.Unlock()
		return proto.Session{}, fmt.Errorf("no session or pane %q", p.Pane)
	}
	at.mu.Lock()
	atID := at.info.ID
	workDir := at.info.WorkDir()
	canFork := at.info.CanForkSession()
	var as proto.AgentSession
	if at.info.AgentSession != nil {
		as = *at.info.AgentSession
	}
	at.mu.Unlock()
	d.mu.Unlock()

	if !canFork {
		return proto.Session{}, fmt.Errorf("pane %s has no forkable agent session", atID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd, err := resolveForkCommand(ctx, &as, workDir)
	if err != nil {
		return proto.Session{}, err
	}

	direction := p.Direction
	if direction == "" {
		direction = "right"
	}
	ns, err := d.split(proto.CreateParams{
		Split:     atID,
		Direction: direction,
		Dir:       workDir,
		Command:   cmd,
		Cols:      p.Cols,
		Rows:      p.Rows,
	})
	if err != nil {
		return ns, err
	}
	if as.Agent == "agy" || as.Agent == "antigravity" {
		if forked := d.findPane(ns.Focus); forked != nil {
			startAgyFork(forked)
		}
	}
	return ns, nil
}
