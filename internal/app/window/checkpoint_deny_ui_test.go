package window

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/changesview"
	"hyla/internal/ui/settings/search"
	"hyla/locales"
)

func TestCheckpointAndDenyTeachUI(t *testing.T) {
	useUpdaterPlugin()
	a := testApp(t)
	m := a.Menu()
	if m.ItemByID("checkpoint.open") == nil {
		t.Fatal("missing checkpoint.open menu item")
	}
	if search.ActionWords["checkpoint.open"] == "" {
		t.Fatal("missing search keywords for checkpoint.open")
	}

	a.Local().Conn = host.StateReady
	mainQ := make(chan func(), 32)
	a.mainHook = func(fn func()) { mainQ <- fn }

	a.St.Sessions = append(a.St.Sessions, proto.Session{
		ID:    "sess-cp-1",
		Name:  "demo",
		Panes: []proto.Pane{{ID: "pane-cp-1", Dir: "/tmp/demo-repo"}},
		Focus: "pane-cp-1",
	})
	a.Selected = "sess-cp-1"

	var mu sync.Mutex
	var calls []string
	var ruleReq proto.RuleAddParams
	cps := []proto.Checkpoint{
		{
			ID:     "cp_turn_1",
			PaneID: "pane-cp-1",
			Agent:  "claude",
			Root:   "/tmp/demo-repo",
			Tree:   "abcdef1234567890",
			Auto:   true,
			At:     time.Now().UnixMilli(),
		},
	}

	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, method)
		switch method {
		case "checkpoint.list":
			return append([]proto.Checkpoint(nil), cps...), nil
		case "checkpoint.create":
			var p proto.CheckpointCreateParams
			_ = json.Unmarshal(raw, &p)
			cp := proto.Checkpoint{
				ID:    "cp_manual_2",
				Root:  "/tmp/demo-repo",
				Tree:  "1122334455667788",
				Label: p.Label,
				At:    time.Now().UnixMilli(),
			}
			cps = append([]proto.Checkpoint{cp}, cps...)
			return cp, nil
		case "checkpoint.restore":
			return proto.CheckpointRestoreResult{Restored: cps[0], SafetyBackupID: "cp_backup_3"}, nil
		case "rule.add":
			if err := json.Unmarshal(raw, &ruleReq); err != nil {
				return nil, err
			}
			return proto.RuleAddResult{Path: "/tmp/demo-repo/.hyla/rules.md", Added: true}, nil
		case "focus", "report":
			return nil, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.Local().Client = cl

	tt := ui.NewTester(a.view, 1180, 760)
	a.run("checkpoint.open", nil)
	drainTestMain(t, tt, mainQ, func() bool {
		return a.CheckpointAsk != nil && a.CheckpointAsk.Loaded && tt.HasText("cp_turn_1")
	})

	// Type a checkpoint label and save.
	a.CheckpointAsk.Label = "before refactor"
	tt.Frame()
	if err := tt.Click(locales.CpSave()); err != nil {
		t.Fatalf("click save checkpoint: %v", err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return tt.HasText("before refactor")
	})

	// Restore first listed checkpoint.
	if err := tt.Click(locales.CpRestore()); err != nil {
		t.Fatalf("click restore: %v", err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(strings.Join(calls, ","), "checkpoint.restore")
	})
	a.CheckpointAsk = nil

	// Exercise Deny & Teach modal flow.
	reverted := false
	a.Side.Changes.Deny = &changesview.DenyAsk{
		Open:     true,
		PaneID:   "pane-cp-1",
		Dir:      "/tmp/demo-repo",
		File:     "internal/db/query.go",
		Reason:   "Never build SQL with string concatenation",
		Dest:     "hyla",
		OnRevert: func() { reverted = true },
	}
	tt.Frame()
	if !tt.HasText(locales.ChangesDenyTitle()) {
		t.Fatalf("expected Deny & Teach modal rendered; texts=%q", tt.Texts())
	}
	if err := tt.Click(locales.ChangesDenyConfirm()); err != nil {
		t.Fatalf("click confirm deny rule: %v", err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return a.Side.Changes.Deny == nil
	})
	if !reverted {
		t.Fatal("expected hunk OnRevert callback invoked")
	}
	if ruleReq.Dest != "hyla" || ruleReq.Reason != "Never build SQL with string concatenation" {
		t.Fatalf("unexpected rule.add params: %+v", ruleReq)
	}
}
