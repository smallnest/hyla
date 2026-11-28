package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"hyla/internal/proto"
)

func TestExtractHookWritePath(t *testing.T) {
	tests := []struct {
		tool string
		raw  string
		want string
	}{
		{"Read", `{"file_path":"internal/auth/token.go"}`, ""},
		{"Grep", `{"path":"internal/auth"}`, ""},
		{"Write", `{"file_path":"internal/auth/token.go"}`, "internal/auth/token.go"},
		{"edit_file", `{"target_file":"cmd/hyla/main.go"}`, "cmd/hyla/main.go"},
		{"NotebookEdit", `{"notebook_path":"notebooks/demo.ipynb"}`, "notebooks/demo.ipynb"},
	}
	for _, tc := range tests {
		got := extractHookWritePath(tc.tool, json.RawMessage(tc.raw))
		if got != tc.want {
			t.Errorf("extractHookWritePath(%q, %s) = %q, want %q", tc.tool, tc.raw, got, tc.want)
		}
	}
}

func TestPreToolUseDecision(t *testing.T) {
	// No conflict emits nothing.
	if _, ok := preToolUseDecision(proto.PathClaimCheckResult{Allowed: true}); ok {
		t.Fatal("expected no output when no conflicts")
	}

	// Warn emits additionalContext without denying.
	warnBytes, ok := preToolUseDecision(proto.PathClaimCheckResult{
		Allowed:   true,
		Conflicts: []proto.PathClaim{{Pattern: "internal/auth/*", Mode: "warn"}},
		Message:   `claude claimed "internal/auth/*" [warn]`,
	})
	if !ok || !strings.Contains(string(warnBytes), "additionalContext") || strings.Contains(string(warnBytes), `"deny"`) {
		t.Fatalf("unexpected warn payload: %s", warnBytes)
	}

	// Block emits permissionDecision=deny.
	denyBytes, ok := preToolUseDecision(proto.PathClaimCheckResult{
		Allowed:   false,
		Conflicts: []proto.PathClaim{{Pattern: "internal/auth/*", Mode: "block"}},
		Message:   `codex claimed "internal/auth/*" [block]`,
	})
	if !ok || !strings.Contains(string(denyBytes), `"permissionDecision":"deny"`) {
		t.Fatalf("unexpected deny payload: %s", denyBytes)
	}
}
