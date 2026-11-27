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
	}
	return nil
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
		lower := strings.ToLower(combined)
		if strings.Contains(lower, "unknown command") || strings.Contains(lower, "unknown subcommand") || strings.Contains(lower, "unexpected argument") {
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
	return d.split(proto.CreateParams{
		Split:     atID,
		Direction: direction,
		Dir:       workDir,
		Command:   cmd,
		Cols:      p.Cols,
		Rows:      p.Rows,
	})
}
