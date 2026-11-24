package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func TestStoreTimerTransitionsAndLongBreak(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 25, 9, 0, 0, 0, time.Local)
	st, err := newStoreAt(dir, t0)
	if err != nil {
		t.Fatalf("newStoreAt: %v", err)
	}

	cfg := DefaultSettings()
	cfg.LongBreakInterval = 2
	cfg.AutoStartBreaks = true
	cfg.AutoStartPomodoros = false
	snap, err := st.UpdateSettings(cfg, t0)
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if snap.Timer.TotalSec != 25*60 || snap.Timer.RemainingSec != 25*60 {
		t.Fatalf("unexpected initial timer: %+v", snap.Timer)
	}

	// Add two tasks and verify first becomes active.
	snap, task1, err := st.AddTask("Write specs", "Focus deeply", 2, t0)
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if snap.ActiveTaskID != task1.ID {
		t.Fatalf("expected activeTaskId %q, got %q", task1.ID, snap.ActiveTaskID)
	}

	// Start first pomodoro, pause halfway, resume, and complete.
	snap, err = st.Start(t0)
	if err != nil || !snap.Timer.Running {
		t.Fatalf("Start failed: %v %+v", err, snap.Timer)
	}
	snap, err = st.Pause(t0.Add(10 * time.Minute))
	if err != nil || snap.Timer.Running || snap.Timer.RemainingSec != 15*60 {
		t.Fatalf("Pause failed: %v %+v", err, snap.Timer)
	}
	snap, err = st.Start(t0.Add(11 * time.Minute))
	if err != nil || !snap.Timer.Running {
		t.Fatalf("Resume failed: %v", err)
	}

	// Tick before expiration -> no completion.
	ev, _, err := st.Tick(t0.Add(25 * time.Minute))
	if err != nil || ev != nil {
		t.Fatalf("expected no completion yet, got ev=%+v err=%v", ev, err)
	}

	// Tick at expiration -> switches to short break (auto-started) and increments task DonePomos.
	t1 := t0.Add(26 * time.Minute)
	ev, snap, err = st.Tick(t1)
	if err != nil || ev == nil {
		t.Fatalf("expected completion event, got ev=%+v err=%v", ev, err)
	}
	if ev.CompletedPhase != PhasePomodoro || ev.NextPhase != PhaseShortBreak || !ev.AutoStarted {
		t.Fatalf("unexpected completion event: %+v", ev)
	}
	if snap.Timer.Phase != PhaseShortBreak || !snap.Timer.Running || snap.Timer.CycleCount != 1 || snap.Timer.SessionNumber != 2 {
		t.Fatalf("unexpected timer after 1st pomo: %+v", snap.Timer)
	}
	if len(snap.Tasks) != 1 || snap.Tasks[0].DonePomos != 1 {
		t.Fatalf("expected task DonePomos=1, got %+v", snap.Tasks)
	}

	// Complete short break -> switches to pomodoro (paused because AutoStartPomodoros=false).
	t2 := t1.Add(5 * time.Minute)
	ev, snap, err = st.Tick(t2)
	if err != nil || ev == nil || ev.CompletedPhase != PhaseShortBreak || ev.NextPhase != PhasePomodoro || ev.AutoStarted {
		t.Fatalf("unexpected break completion: ev=%+v err=%v", ev, err)
	}
	if snap.Timer.Phase != PhasePomodoro || snap.Timer.Running {
		t.Fatalf("expected paused pomodoro after break, got %+v", snap.Timer)
	}

	// Complete 2nd pomodoro -> triggers LongBreak (LongBreakInterval=2) and resets CycleCount=0.
	_, err = st.Start(t2)
	if err != nil {
		t.Fatal(err)
	}
	t3 := t2.Add(25 * time.Minute)
	ev, snap, err = st.Tick(t3)
	if err != nil || ev == nil || ev.NextPhase != PhaseLongBreak {
		t.Fatalf("expected LongBreak after 2nd pomo, got ev=%+v err=%v", ev, err)
	}
	if snap.Timer.CycleCount != 0 || snap.Report.TodayPomos != 2 || snap.Report.TodayMinutes != 50 || snap.Report.StreakDays != 1 {
		t.Fatalf("unexpected cycle/report after 2nd pomo: timer=%+v report=%+v", snap.Timer, snap.Report)
	}

	// Skip from long break returns to paused pomodoro without adding history.
	snap, err = st.Skip(t3.Add(time.Minute))
	if err != nil || snap.Timer.Phase != PhasePomodoro || snap.Timer.Running || snap.Report.TodayPomos != 2 {
		t.Fatalf("unexpected skip result: %+v", snap)
	}
}

func TestStorePersistenceAndTaskOperations(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.Local)
	st, err := newStoreAt(dir, t0)
	if err != nil {
		t.Fatal(err)
	}

	_, taskA, err := st.AddTask("Task A", "", 1, t0)
	if err != nil {
		t.Fatal(err)
	}
	_, taskB, err := st.AddTask("Task B", "notes", 3, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := func() error {
		_, err := st.UpdateTask(taskA.ID, "Task A+", "updated", 2, t0)
		return err
	}(); err != nil {
		t.Fatal(err)
	}
	snap, err := st.MoveTask(taskB.ID, -1, t0)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Tasks[0].ID != taskB.ID || snap.Tasks[1].ID != taskA.ID {
		t.Fatalf("unexpected move order: %+v", snap.Tasks)
	}

	// Start timer and reload from disk while still running.
	if _, err := st.Start(t0); err != nil {
		t.Fatal(err)
	}
	stReloaded, err := newStoreAt(dir, t0.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	snap2 := stReloaded.Snapshot(t0.Add(5 * time.Minute))
	if !snap2.Timer.Running || snap2.Timer.RemainingSec != 20*60 {
		t.Fatalf("expected running timer with 20m left, got %+v", snap2.Timer)
	}

	// Reload after timer would have long expired -> pauses cleanly at phase duration.
	stExpired, err := newStoreAt(dir, t0.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	snap3 := stExpired.Snapshot(t0.Add(2 * time.Hour))
	if snap3.Timer.Running || snap3.Timer.RemainingSec != 25*60 {
		t.Fatalf("expected expired running timer to reset cleanly on reload, got %+v", snap3.Timer)
	}

	// Toggle complete moves activeTaskId to next uncompleted task.
	if _, err := stExpired.SelectTask(taskB.ID, t0); err != nil {
		t.Fatal(err)
	}
	snap4, err := stExpired.ToggleTask(taskB.ID, t0)
	if err != nil {
		t.Fatal(err)
	}
	if snap4.ActiveTaskID != taskA.ID {
		t.Fatalf("expected activeTaskId to advance to %q, got %q", taskA.ID, snap4.ActiveTaskID)
	}
	snap4, err = stExpired.ClearFinishedTasks(t0)
	if err != nil || len(snap4.Tasks) != 1 {
		t.Fatalf("ClearFinishedTasks failed: len=%d err=%v", len(snap4.Tasks), err)
	}
}

func TestSettingsRoundTripAndClamping(t *testing.T) {
	raw := map[string]any{
		"pomodoro_minutes":      float64(45),
		"short_break_minutes":   float64(10),
		"long_break_minutes":    float64(20),
		"long_break_interval":   float64(3),
		"auto_start_breaks":     true,
		"auto_start_pomodoros":  true,
		"alarm_sound":           "bowl",
		"alarm_volume":          float64(150), // clamped to 100
		"alarm_repeat":          float64(4),
		"ambient_sound":         "rain",
		"ambient_volume":        float64(50),
		"desktop_notifications": false,
		"daily_goal_pomos":      float64(6),
	}
	s := SettingsFromMap(raw, DefaultSettings())
	if s.PomodoroMinutes != 45 || s.AlarmVolume != 100 || s.AlarmSound != "bowl" || s.AmbientSound != "rain" || s.DesktopNotifications {
		t.Fatalf("unexpected SettingsFromMap result: %+v", s)
	}
	m := SettingsToMap(s)
	s2 := SettingsFromMap(m, DefaultSettings())
	if s2 != s {
		t.Fatalf("roundtrip mismatch: got %+v want %+v", s2, s)
	}
}

func TestPluginHarnessEndToEnd(t *testing.T) {
	plg := newPlugin()
	h := hylaplugin.NewTestHarness(plg)
	defer h.Close()

	var (
		mu          sync.Mutex
		openedPanes []hylaplugin.UIOpenPaneParams
		notified    []hylaplugin.UINotifyParams
		emitted     []hylaplugin.WebViewEmitParams
		savedCfg    map[string]any
	)

	h.RegisterHostHandler("host.getState", func(ctx context.Context, raw json.RawMessage) (any, error) {
		return hylaplugin.HostGetStateResult{
			ActiveSession: "sess-1",
			FocusedPane:   "pane-1",
			State: hylaplugin.State{
				Sessions: []hylaplugin.Session{
					{ID: "sess-1", Name: "Dev", Focus: "pane-1"},
				},
			},
		}, nil
	})
	h.RegisterHostHandler("ui.openPane", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p hylaplugin.UIOpenPaneParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		openedPanes = append(openedPanes, p)
		mu.Unlock()
		return map[string]any{}, nil
	})
	h.RegisterHostHandler("plugin.getSettings", func(ctx context.Context, raw json.RawMessage) (any, error) {
		return hylaplugin.PluginGetSettingsResult{Settings: map[string]any{}}, nil
	})
	h.RegisterHostHandler("plugin.setSettings", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p hylaplugin.PluginSetSettingsParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		savedCfg = p.Settings
		mu.Unlock()
		return map[string]any{}, nil
	})
	h.RegisterHostHandler("ui.notify", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p hylaplugin.UINotifyParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		notified = append(notified, p)
		mu.Unlock()
		return map[string]any{}, nil
	})
	h.RegisterHostHandler("webview.emit", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p hylaplugin.WebViewEmitParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		emitted = append(emitted, p)
		mu.Unlock()
		return map[string]any{}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var initRes hylaplugin.InitializeResult
	if err := h.Call(ctx, "initialize", hylaplugin.InitializeParams{
		HylaVersion: "3.23.2",
		Host:        "app",
		PluginID:    pluginID,
		DataDir:     t.TempDir(),
		Locale:      "zh-CN",
	}, &initRes); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	var actRes hylaplugin.ActionInvokeResult
	if err := h.Call(ctx, "action.invoke", hylaplugin.ActionInvokeParams{
		ActionID: "open",
		Locale:   "zh-CN",
	}, &actRes); err != nil {
		t.Fatalf("action.invoke: %v", err)
	}
	if !actRes.OK {
		t.Fatalf("expected action ok, got %+v", actRes)
	}
	mu.Lock()
	if len(openedPanes) != 1 || openedPanes[0].Title != "番茄钟" || openedPanes[0].View != slotID {
		t.Fatalf("unexpected opened panes: %+v", openedPanes)
	}
	mu.Unlock()

	callRPC := func(method string, params any) Snapshot {
		t.Helper()
		rawParams, _ := json.Marshal(params)
		var out hylaplugin.WebViewRPCResult
		if err := h.Call(ctx, "webview.rpc", hylaplugin.WebViewRPCParams{
			SlotID:    slotID,
			Method:    method,
			Params:    rawParams,
			SessionID: "sess-1",
		}, &out); err != nil {
			t.Fatalf("%s failed: %v", method, err)
		}
		var payload struct {
			Locale   string   `json:"locale"`
			Snapshot Snapshot `json:"snapshot"`
		}
		if err := json.Unmarshal(out.Data, &payload); err != nil {
			t.Fatalf("decode %s: %v", method, err)
		}
		return payload.Snapshot
	}

	snap := callRPC("pomodoro.bootstrap", nil)
	if snap.Timer.TotalSec != 25*60 {
		t.Fatalf("unexpected bootstrap timer: %+v", snap.Timer)
	}

	snap = callRPC("pomodoro.task.add", map[string]any{"title": "Ship Pomodoro", "estPomos": 2})
	if len(snap.Tasks) != 1 || snap.Tasks[0].Title != "Ship Pomodoro" {
		t.Fatalf("unexpected tasks: %+v", snap.Tasks)
	}

	nextCfg := DefaultSettings()
	nextCfg.PomodoroMinutes = 30
	snap = callRPC("pomodoro.settings.update", nextCfg)
	if snap.Settings.PomodoroMinutes != 30 || snap.Timer.TotalSec != 30*60 {
		t.Fatalf("settings update not reflected: %+v", snap)
	}
	mu.Lock()
	if savedCfg == nil || savedCfg["pomodoro_minutes"] != float64(30) {
		t.Fatalf("expected plugin.setSettings call, got %+v", savedCfg)
	}
	if len(notified) != 0 && len(emitted) == 0 {
		t.Fatalf("unexpected notification/emit state")
	}
	mu.Unlock()

	// Ensure web assets referenced by manifest exist.
	for _, rel := range []string{"hyla-plugin.toml", "web/index.html", "web/style.css", "web/app.js"} {
		if _, err := os.Stat(filepath.Join(".", rel)); err != nil {
			t.Fatalf("missing required file %s: %v", rel, err)
		}
	}
}
