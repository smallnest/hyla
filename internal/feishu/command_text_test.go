package feishu

import "testing"

// A Claude Code answer drops the agent's own reports to the terminal,
// which its JSON's lines do not hold, and its warning of no input.
func TestCommandTextClaude(t *testing.T) {
	raw := `[claude-code:unrecognized_model] {"model":"group/smart-split[1m]","query_source":"sdk"}
Warning: no stdin data received in 3s, proceeding without it. If piping from a slow command, redirect stdin explicitly: < /dev/null to skip, or wait longer.
{"type":"result","result":"Hi! What do you need help with?","session_id":"x"}
`
	if got := commandText("claude", raw); got != "Hi! What do you need help with?" {
		t.Errorf("the answer %q", got)
	}
}

// A Codex turn's events give its messages as the answer, and name the
// session a chat continues.
func TestCommandTextCodex(t *testing.T) {
	raw := `{"type":"thread.started","thread_id":"0199-th-1"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"thinking"}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Hello."}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"What next?"}}
{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":2}}
`
	if got := commandText("codex", raw); got != "Hello.\n\nWhat next?" {
		t.Errorf("the answer %q", got)
	}
	if sid := conversation("codex", raw, "x"); sid != "0199-th-1" {
		t.Errorf("codex continues %q", sid)
	}
	// A turn that failed says why, rather than its events.
	failed := `{"type":"turn.started"}
{"type":"turn.failed","error":{"message":"invalid session id"}}
`
	if got := commandText("codex", failed); got != "invalid session id" {
		t.Errorf("the failure %q", got)
	}
}

// An OpenCode turn's events give its answer, the session a chat
// continues, and what the turn used.
func TestCommandTextOpenCode(t *testing.T) {
	raw := `{"type":"step_start","timestamp":1,"sessionID":"ses_1","part":{"id":"p1","type":"step-start"}}
{"type":"text","timestamp":2,"sessionID":"ses_1","part":{"id":"p2","type":"text","text":"Hello."}}
{"type":"step_finish","timestamp":3,"sessionID":"ses_1","part":{"id":"p3","type":"step-finish","reason":"stop","tokens":{"input":21500,"output":469,"reasoning":0,"cache":{"read":39900,"write":0}}}}`
	if got := commandText("opencode", raw); got != "Hello." {
		t.Errorf("the answer %q", got)
	}
	if sid := conversation("opencode", raw, "x"); sid != "ses_1" {
		t.Errorf("opencode continues %q", sid)
	}
}

// A Pi turn's events give its answer, and what it used.
func TestCommandTextPi(t *testing.T) {
	raw := `{"type":"message_start","message":{"role":"user","content":[{"type":"text","text":"hi"}]}}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Hello."},{"type":"toolCall","id":"t1","name":"bash","arguments":{}}]}}
{"type":"agent_end"}`
	if got := commandText("pi", raw); got != "Hello." {
		t.Errorf("the answer %q", got)
	}
}

// A Gemini turn's JSON object gives its answer.
func TestCommandTextGemini(t *testing.T) {
	raw := `{"response":"Hello.","stats":{"models":{"gemini-2.5-pro":{"tokens":{"input":1000,"candidates":200,"cached":400}}}}}`
	if got := commandText("gemini", raw); got != "Hello." {
		t.Errorf("the answer %q", got)
	}
}
