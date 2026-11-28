package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hyla/internal/proto"
)

func TestRuleAddAppendsAndDeduplicates(t *testing.T) {
	d := testDaemon(t)
	repo := initTestGitRepo(t)

	res1, err := d.ruleAdd(proto.RuleAddParams{
		Dir:     repo,
		File:    "internal/db/query.go",
		Reason:  "Never bypass parameterized SQL queries",
		Snippet: "db.Exec(\"SELECT * FROM users WHERE id=\" + id)",
		Dest:    "agents",
	})
	if err != nil {
		t.Fatalf("ruleAdd agents: %v", err)
	}
	if !res1.Added || filepath.Base(res1.Path) != "AGENTS.md" {
		t.Fatalf("unexpected ruleAdd result: %+v", res1)
	}
	body, err := os.ReadFile(res1.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "## Hyla Guardrails") || !strings.Contains(string(body), "Never bypass parameterized SQL queries") {
		t.Fatalf("unexpected AGENTS.md contents:\n%s", body)
	}

	// Calling again with identical rule deduplicates cleanly.
	resDup, err := d.ruleAdd(proto.RuleAddParams{
		Dir:     repo,
		File:    "internal/db/query.go",
		Reason:  "Never bypass parameterized SQL queries",
		Snippet: "db.Exec(\"SELECT * FROM users WHERE id=\" + id)",
		Dest:    "agents",
	})
	if err != nil {
		t.Fatalf("ruleAdd dup: %v", err)
	}
	if resDup.Added {
		t.Fatalf("expected deduplicated Added=false, got true")
	}

	// Writing to .hyla/rules.md works too.
	resHyla, err := d.ruleAdd(proto.RuleAddParams{
		Dir:    repo,
		Reason: "Keep public HTTP handlers backwards compatible",
		Dest:   "hyla",
	})
	if err != nil {
		t.Fatalf("ruleAdd hyla: %v", err)
	}
	if !resHyla.Added || !strings.HasSuffix(resHyla.Path, filepath.Join(".hyla", "rules.md")) {
		t.Fatalf("unexpected hyla rules result: %+v", resHyla)
	}
}
