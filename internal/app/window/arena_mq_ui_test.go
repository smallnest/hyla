package window

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/settings/search"
	"hyla/locales"
)

func drainTestMain(t *testing.T, tt *ui.Tester, mainQ chan func(), pred func() bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		for {
			select {
			case fn := <-mainQ:
				fn()
			default:
				goto framed
			}
		}
	framed:
		tt.Frame()
		if pred() {
			return
		}
		select {
		case fn := <-mainQ:
			fn()
		case <-time.After(10 * time.Millisecond):
		case <-deadline:
			t.Fatalf("timed out waiting for UI condition; texts=%q", tt.Texts())
		}
	}
}

func TestArenaAndMQMenusAndPalette(t *testing.T) {
	useUpdaterPlugin()
	a := testApp(t)
	m := a.Menu()
	for _, id := range []string{"arena.open", "mq.open"} {
		if m.ItemByID(id) == nil {
			t.Errorf("no %s in the app menus", id)
		}
		if search.ActionWords[id] == "" {
			t.Errorf("no search keywords for %s", id)
		}
	}
}

func TestArenaDialogFlow(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	mainQ := make(chan func(), 32)
	a.mainHook = func(fn func()) { mainQ <- fn }

	// Target contestant session so Focus selects a valid session.
	a.St.Sessions = append(a.St.Sessions, proto.Session{
		ID:    "arena-s-claude",
		Name:  "arena-claude",
		Panes: []proto.Pane{{ID: "p-claude", Dir: "/tmp/wt-claude"}},
		Focus: "p-claude",
	})

	var mu sync.Mutex
	var calls []string
	var startParams proto.ArenaStartParams
	var pickParams proto.ArenaPickParams
	var arenas []proto.Arena

	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, method)
		switch method {
		case "arena.list":
			return slices.Clone(arenas), nil
		case "arena.start":
			if err := json.Unmarshal(raw, &startParams); err != nil {
				return nil, err
			}
			ar := proto.Arena{
				ID:     "ar_ui_1",
				Prompt: startParams.Prompt,
				Base:   "main",
				Status: "running",
				Contestants: []proto.ArenaContestant{
					{Agent: "claude", SessionID: "arena-s-claude", Status: proto.Working, Files: 2, Added: 20, Removed: 4, Commits: 1},
					{Agent: "codex", SessionID: "arena-s-codex", Status: proto.Idle, Files: 1, Added: 10, Removed: 2, Commits: 1},
				},
			}
			arenas = []proto.Arena{ar}
			return ar, nil
		case "arena.test":
			arenas[0].Contestants[0].CheckStatus = "passed"
			arenas[0].Contestants[1].CheckStatus = "failed"
			return arenas[0], nil
		case "arena.pick":
			if err := json.Unmarshal(raw, &pickParams); err != nil {
				return nil, err
			}
			arenas[0].Status = "picked"
			arenas[0].Winner = pickParams.Winner
			return arenas[0], nil
		case "focus", "report":
			return nil, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.Local().Client = cl

	tt := ui.NewTester(a.view, 1180, 760)
	a.run("arena.open", nil)
	drainTestMain(t, tt, mainQ, func() bool {
		return a.ArenaAsk != nil && a.ArenaAsk.Loaded && a.ArenaAsk.Tab == "new"
	})

	tt.Type("Add rate limiting")
	tt.Frame()
	if err := tt.Click(locales.ArenaStart()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return a.ArenaAsk.Tab == "list" && tt.HasText("Add rate limiting")
	})

	if startParams.Prompt != "Add rate limiting" || len(startParams.Agents) == 0 {
		t.Fatalf("unexpected arena.start params: %+v", startParams)
	}

	if err := tt.Click(locales.ArenaRunTests()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return tt.HasText("passed")
	})

	if err := tt.Click(locales.ArenaPickWinner()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return tt.HasText("picked")
	})
	if pickParams.ID != "ar_ui_1" || pickParams.Winner != "claude" {
		t.Fatalf("unexpected arena.pick params: %+v", pickParams)
	}

	if err := tt.Click(locales.ArenaFocus()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ArenaAsk != nil {
		t.Fatal("expected arena dialog to close after Focus")
	}
	if a.Selected != "arena-s-claude" {
		t.Fatalf("Selected session = %q, want arena-s-claude", a.Selected)
	}
}

func TestMergeQueueDialogFlow(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	mainQ := make(chan func(), 32)
	a.mainHook = func(fn func()) { mainQ <- fn }

	var mu sync.Mutex
	lanes := []proto.MergeLane{
		{
			ID:        "mq_1",
			Branch:    "feat/login",
			Base:      "main",
			SessionID: "s1",
			CheckCmd:  "go test ./...",
			Status:    "needs_review",
			Bounces:   1,
			Findings: []proto.AntiCheatFinding{
				{Kind: "deleted_test", File: "auth_test.go", Reason: "removed test assertions"},
			},
		},
	}
	var approvedID string
	var addParams proto.MergeQueueAddParams
	var runCalled bool

	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "mq.list":
			return slices.Clone(lanes), nil
		case "mq.approve":
			var p proto.MergeQueueLaneParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			approvedID = p.ID
			lanes[0].Status = "queued"
			lanes[0].GuardApproved = true
			return lanes[0], nil
		case "mq.add":
			if err := json.Unmarshal(raw, &addParams); err != nil {
				return nil, err
			}
			nl := proto.MergeLane{
				ID:       "mq_2",
				Branch:   "feat/metrics",
				Base:     "main",
				CheckCmd: addParams.CheckCmd,
				Status:   "queued",
			}
			lanes = append(lanes, nl)
			return nl, nil
		case "mq.run":
			runCalled = true
			lanes[0].Status = "landed"
			lanes[1].Status = "landed"
			return proto.MergeQueueRunResult{
				Landed: []string{"mq_1", "mq_2"},
				Lanes:  slices.Clone(lanes),
			}, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.Local().Client = cl

	tt := ui.NewTester(a.view, 1200, 760)
	a.run("mq.open", nil)
	drainTestMain(t, tt, mainQ, func() bool {
		return a.MQAsk != nil && a.MQAsk.Loaded && tt.HasText("feat/login")
	})

	if !tt.HasText(locales.MqGuardWarning()) {
		t.Fatalf("expected anti-cheat guard warning in texts: %q", tt.Texts())
	}

	if err := tt.Click(locales.MqApprove()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return approvedID == "mq_1" && tt.HasText("queued")
	})

	if err := tt.Click(locales.MqEnqueue()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return tt.HasText("feat/metrics")
	})
	if addParams.Target != "s1" {
		t.Fatalf("unexpected mq.add target: %+v", addParams)
	}

	if err := tt.Click(locales.MqRunQueue()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return runCalled && tt.HasText("landed")
	})
}

func TestBrowserToolbarRecordAndReplay(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	mainQ := make(chan func(), 32)
	a.mainHook = func(fn func()) { mainQ <- fn }

	workDir := t.TempDir()
	flowDir := filepath.Join(workDir, ".hyla", "flows")
	if err := os.MkdirAll(flowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	savedFlow := filepath.Join(flowDir, "checkout.json")
	if err := os.WriteFile(savedFlow, []byte(`{"name":"checkout","steps":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &a.St.Sessions[0]
	s.Panes[1] = proto.Pane{ID: "b", Dir: workDir, URL: proto.BlankURL, Status: proto.Idle, Progress: -1}

	var mu sync.Mutex
	var methods []string
	var replayParams proto.BrowserReplayParams

	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		methods = append(methods, method)
		switch method {
		case "browser.record.start":
			return proto.BrowserRecordStatus{Pane: "b", Recording: true, Name: "flow-1"}, nil
		case "browser.record.stop":
			return proto.BrowserRecordStatus{Pane: "b", Recording: false, Name: "flow-1", Steps: 2, SavedPath: savedFlow}, nil
		case "browser.replay":
			if err := json.Unmarshal(raw, &replayParams); err != nil {
				return nil, err
			}
			return proto.BrowserReplayResult{
				FlowName:          "checkout",
				Passed:            true,
				DurationMs:        42,
				BaselineDiffRatio: 0,
			}, nil
		case "pane.page":
			return nil, nil
		default:
			return nil, errors.New("unexpected method " + method)
		}
	})
	defer stop()
	a.Local().Client = cl

	tt := ui.NewTester(a.view, 1180, 720)
	tt.Frame()
	tt.Frame()

	if err := tt.Click(locales.BrowserRecordStart()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return a.Webs["b"] != nil && a.Webs["b"].Recording
	})

	if err := tt.Click(locales.BrowserRecordStop()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return a.Webs["b"] != nil && !a.Webs["b"].Recording && a.Webs["b"].LastFlowPath == savedFlow
	})

	if err := tt.Click(locales.BrowserReplay()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ReplayAsk == nil || !a.ReplayAsk.Open {
		t.Fatal("expected ReplayAsk dialog to open")
	}

	if err := tt.Click(locales.BrowserReplayRun()); err != nil {
		t.Fatal(err)
	}
	drainTestMain(t, tt, mainQ, func() bool {
		return tt.HasText("Flow passed in 42ms (diff 0.00%)") || tt.HasText("42ms")
	})

	if replayParams.Pane != "b" || replayParams.FlowPath != savedFlow {
		t.Fatalf("unexpected replay params: %+v", replayParams)
	}
}
