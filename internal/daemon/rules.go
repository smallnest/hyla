package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hyla/internal/proto"
)

func (d *Daemon) ruleAdd(p proto.RuleAddParams) (proto.RuleAddResult, error) {
	reason := strings.TrimSpace(strings.Join(strings.Fields(p.Reason), " "))
	if reason == "" {
		return proto.RuleAddResult{}, errors.New("rule.add: reason is required")
	}

	dir := strings.TrimSpace(p.Dir)
	if dir == "" && p.Target != "" {
		if pn := d.findPane(p.Target); pn != nil {
			pn.mu.Lock()
			dir = pn.info.WorkDir()
			pn.mu.Unlock()
		} else {
			dir = p.Target
		}
	}
	if dir == "" {
		return proto.RuleAddResult{}, errors.New("rule.add: dir or target is required")
	}
	root := canonicalRepoRoot(dir)

	dest := strings.ToLower(strings.TrimSpace(p.Dest))
	var path string
	switch dest {
	case "", "agents":
		path = filepath.Join(root, "AGENTS.md")
	case "hyla":
		path = filepath.Join(root, ".hyla", "rules.md")
	default:
		return proto.RuleAddResult{}, fmt.Errorf("rule.add: invalid dest %q (want agents or hyla)", p.Dest)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return proto.RuleAddResult{}, fmt.Errorf("rule.add: %w", err)
	}

	existingBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return proto.RuleAddResult{}, fmt.Errorf("rule.add: %w", err)
	}
	existing := string(existingBytes)

	fileNote := strings.TrimSpace(p.File)
	if fileNote != "" {
		fileNote = normalizeRelPath(root, fileNote)
	}
	var suffix string
	if fileNote != "" {
		suffix = fmt.Sprintf("(%s) %s", fileNote, reason)
	} else {
		suffix = reason
	}
	if snip := strings.TrimSpace(strings.Join(strings.Fields(p.Snippet), " ")); snip != "" {
		if len(snip) > 120 {
			snip = snip[:117] + "..."
		}
		suffix += fmt.Sprintf(" [hunk: `%s`]", snip)
	}

	// Deduplicate regardless of date stamp so repeated denials do not bloat the file.
	for _, line := range strings.Split(existing, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), suffix) {
			return proto.RuleAddResult{
				Path:    path,
				Added:   false,
				Content: strings.TrimSpace(line),
			}, nil
		}
	}

	entry := fmt.Sprintf("- [%s] %s", time.Now().Format("2006-01-02"), suffix)
	var b strings.Builder
	b.WriteString(strings.TrimRight(existing, "\r\n"))
	if existing == "" {
		b.WriteString("# Rules\n\n## Hyla Guardrails\n\n")
	} else if !strings.Contains(existing, "## Hyla Guardrails") {
		b.WriteString("\n\n## Hyla Guardrails\n\n")
	} else {
		b.WriteByte('\n')
	}
	b.WriteString(entry)
	b.WriteByte('\n')

	writeAtomic(path, []byte(b.String()))
	return proto.RuleAddResult{
		Path:    path,
		Added:   true,
		Content: entry,
	}, nil
}
