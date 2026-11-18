package proto

import "testing"

func TestPluginAgentRegistryAndPaneMethods(t *testing.T) {
	t.Cleanup(ClearPluginAgents)

	if AgentDisplayName("claude") != "Claude Code" {
		t.Fatalf("built-in display name = %q", AgentDisplayName("claude"))
	}
	if !StartsTimeline("claude") || StartsTimeline("aider") {
		t.Fatal("unexpected built-in StartsTimeline behavior")
	}

	SetPluginAgents("org.example.aider", []AgentMeta{{
		ID:       "aider",
		Name:     "Aider",
		Timeline: true,
		Chat:     true,
	}})

	if got := AgentDisplayName("aider"); got != "Aider" {
		t.Fatalf("AgentDisplayName(aider) = %q, want Aider", got)
	}
	if !StartsTimeline("aider") {
		t.Fatal("expected plugin agent aider to start timeline")
	}
	if m, ok := AgentProvider("aider"); !ok || m.PluginID != "org.example.aider" || !m.Chat {
		t.Fatalf("unexpected AgentProvider(aider): %+v, ok=%v", m, ok)
	}
	if names := AllAgentNames(); names["aider"] != "Aider" || names["claude"] != "Claude Code" {
		t.Fatalf("AllAgentNames missing entries: %+v", names)
	}

	p := &Pane{
		ID:          "p1",
		Plugin:      "org.example.aider",
		PluginView:  "panel",
		PluginTitle: "Aider Graph",
	}
	if !p.PluginPane() || !p.Shown() {
		t.Fatal("expected PluginPane and Shown to be true")
	}
	if p.PaneName() != "Aider Graph" {
		t.Fatalf("PaneName() = %q, want Aider Graph", p.PaneName())
	}
	p.PluginTitle = ""
	if p.PaneName() != "panel" {
		t.Fatalf("PaneName() fallback = %q, want panel", p.PaneName())
	}
	p.FeishuAnswer = "/tmp/old"
	if p.ChannelAnswerDir() != "/tmp/old" {
		t.Fatalf("ChannelAnswerDir legacy = %q", p.ChannelAnswerDir())
	}
	p.AnswerDir = "/tmp/new"
	if p.ChannelAnswerDir() != "/tmp/new" {
		t.Fatalf("ChannelAnswerDir answerDir = %q", p.ChannelAnswerDir())
	}

	RemovePluginAgents("org.example.aider")
	if StartsTimeline("aider") {
		t.Fatal("expected aider timeline support cleared after RemovePluginAgents")
	}
}
