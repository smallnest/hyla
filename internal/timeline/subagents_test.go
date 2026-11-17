package timeline

import (
	"os"
	"path/filepath"
	"testing"
)

// A timeline's subagents are the steps its agent gave its work to, in the
// turns that gave it, the ones that run first and the newest before.
func TestSubagents(t *testing.T) {
	turns := []Step{
		{Kind: Turn, Title: "Fix the login bug", Children: []Step{
			{Kind: Exchange, Children: []Step{
				{Kind: Tool, Title: "Read", Start: 1, End: 2},
				{Kind: Subagent, Title: "Explore the tests", Detail: "Explore", Start: 3, End: 9, State: Done},
			}},
		}},
		{Kind: Turn, Title: "Run the suite", Children: []Step{
			{Kind: Subagent, Title: "Run the tests", Detail: "general-purpose", Start: 12, State: Running},
			{Kind: Subagent, Title: "Check the docs", Detail: "Explore", Start: 11, End: 14, State: Done},
		}},
	}
	subs := Subagents(turns)
	if len(subs) != 3 {
		t.Fatalf("subagents %+v", subs)
	}
	if subs[0].Task != "Run the tests" || subs[0].State != Running || subs[0].Turn != "Run the suite" {
		t.Errorf("first %+v", subs[0])
	}
	if subs[1].Task != "Check the docs" || subs[1].Type != "Explore" || subs[1].Turn != "Run the suite" {
		t.Errorf("second %+v", subs[1])
	}
	if subs[2].Task != "Explore the tests" || subs[2].Turn != "Fix the login bug" || subs[2].End != 9 {
		t.Errorf("third %+v", subs[2])
	}
}

// A subagent's own transcript says the step it runs now: the tool it
// called that no result ended, else what it wrote last.
func TestSubagentNow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-a.jsonl")
	lines := `{"type":"user","isSidechain":true,"message":{"role":"user","content":"Look at the store's tests"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"a.go"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Grep","input":{"pattern":"func Test"}}]}}
`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	if now := SubagentNow(path); now != "Grep" {
		t.Fatalf("now %q", now)
	}
	// With every result in, the last words are what it does.
	if err := os.WriteFile(path, []byte(lines+`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"ok"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"The store's tests pass."}]}}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if now := SubagentNow(path); now != "The store's tests pass." {
		t.Fatalf("now %q", now)
	}
	if now := SubagentNow(filepath.Join(dir, "none.jsonl")); now != "" {
		t.Fatalf("missing file now %q", now)
	}
}
