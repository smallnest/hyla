package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

const (
	pluginID     = "hyla.pomodoro"
	slotID       = "pomodoro"
	tickInterval = 250 * time.Millisecond
)

type pomodoroService struct {
	mu            sync.RWMutex
	host          *hylaplugin.Host
	store         *Store
	locale        string
	lastSessionID string
	bgCtx         context.Context
	bgCancel      context.CancelFunc
	tickerOnce    sync.Once
}

func newPomodoroService() *pomodoroService {
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &pomodoroService{
		locale:   "en",
		bgCtx:    bgCtx,
		bgCancel: bgCancel,
	}
}

func (s *pomodoroService) shutdown() {
	s.bgCancel()
}

func (s *pomodoroService) initRuntime(ctx context.Context, host *hylaplugin.Host, dataDir, locale string, rawSettings map[string]any) error {
	if strings.TrimSpace(locale) != "" {
		s.setLocale(locale)
	}
	if strings.TrimSpace(dataDir) == "" {
		var err error
		dataDir, err = os.MkdirTemp("", "hyla-pomodoro-data-*")
		if err != nil {
			return err
		}
	}
	st, err := NewStore(dataDir)
	if err != nil {
		return err
	}
	if len(rawSettings) > 0 {
		cur := st.Snapshot(time.Now()).Settings
		if _, uerr := st.UpdateSettings(SettingsFromMap(rawSettings, cur), time.Now()); uerr != nil {
			return uerr
		}
	}

	s.mu.Lock()
	s.host = host
	s.store = st
	s.mu.Unlock()

	s.startTicker()

	go func() {
		rctx, cancel := context.WithTimeout(s.bgCtx, 5*time.Second)
		defer cancel()
		s.reloadSettings(rctx)
	}()
	return nil
}

func (s *pomodoroService) ensureStore() (*Store, error) {
	s.mu.RLock()
	st := s.store
	h := s.host
	loc := s.locale
	s.mu.RUnlock()
	if st != nil {
		return st, nil
	}
	if err := s.initRuntime(context.Background(), h, "", loc, nil); err != nil {
		return nil, err
	}
	s.mu.RLock()
	st = s.store
	s.mu.RUnlock()
	return st, nil
}

func (s *pomodoroService) startTicker() {
	s.tickerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(tickInterval)
			defer ticker.Stop()
			for {
				select {
				case <-s.bgCtx.Done():
					return
				case now := <-ticker.C:
					s.onTick(now)
				}
			}
		}()
	})
}

func (s *pomodoroService) onTick(now time.Time) {
	s.mu.RLock()
	st := s.store
	s.mu.RUnlock()
	if st == nil {
		return
	}
	ev, snap, err := st.Tick(now)
	if err != nil {
		log.Printf("pomodoro tick error: %v", err)
		return
	}
	if ev == nil {
		return
	}

	s.emitState(snap)
	s.emitEvent("pomodoro.phaseCompleted", map[string]any{
		"event":    ev,
		"snapshot": snap,
		"locale":   s.resolveLocale(""),
	})

	if snap.Settings.DesktopNotifications {
		s.sendCompletionNotification(ev)
	}
}

func (s *pomodoroService) sendCompletionNotification(ev *CompletionEvent) {
	s.mu.RLock()
	h := s.host
	sessID := s.lastSessionID
	s.mu.RUnlock()
	if h == nil || ev == nil {
		return
	}

	ctx, cancel := context.WithTimeout(s.bgCtx, 3*time.Second)
	defer cancel()

	if sessID == "" {
		if stRes, err := h.GetState(ctx); err == nil {
			if sess := findSession(stRes, "", ""); sess != nil {
				sessID = sess.ID
				s.rememberSession(sessID)
			}
		}
	}

	loc := s.resolveLocale("")
	title, body := localizedCompletionMessage(loc, ev)
	_ = h.NotifyUI(ctx, hylaplugin.UINotifyParams{
		Title:     title,
		Body:      body,
		Kind:      "success",
		SessionID: sessID,
	})
}

func (s *pomodoroService) rememberSession(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	s.lastSessionID = sessionID
	s.mu.Unlock()
}

func (s *pomodoroService) setLocale(loc string) string {
	norm := normalizeLocale(loc)
	s.mu.Lock()
	s.locale = norm
	s.mu.Unlock()
	return norm
}

func (s *pomodoroService) resolveLocale(reqLoc string) string {
	if strings.TrimSpace(reqLoc) != "" {
		return s.setLocale(reqLoc)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.locale == "" {
		return "en"
	}
	return s.locale
}

func (s *pomodoroService) reloadSettings(ctx context.Context) {
	s.mu.RLock()
	h := s.host
	st := s.store
	s.mu.RUnlock()
	if h == nil || st == nil {
		return
	}
	res, err := h.GetSettings(ctx, false)
	if err != nil || res == nil {
		return
	}
	cur := st.Snapshot(time.Now()).Settings
	next := SettingsFromMap(res.Settings, cur)
	if snap, uerr := st.UpdateSettings(next, time.Now()); uerr == nil {
		s.emitState(snap)
	}
}

func (s *pomodoroService) emitEvent(event string, data any) {
	s.mu.RLock()
	h := s.host
	s.mu.RUnlock()
	if h == nil {
		return
	}
	ctx, cancel := context.WithTimeout(s.bgCtx, 3*time.Second)
	defer cancel()
	_ = h.WebViewEmit(ctx, slotID, event, data)
}

func (s *pomodoroService) emitState(snap Snapshot) {
	s.emitEvent("pomodoro.state", map[string]any{
		"snapshot": snap,
		"locale":   s.resolveLocale(""),
	})
}

func (s *pomodoroService) handleAction(ctx context.Context, host *hylaplugin.Host, p hylaplugin.ActionInvokeParams) (hylaplugin.ActionInvokeResult, error) {
	if p.ActionID != "" && p.ActionID != "open" && p.ActionID != pluginID+".open" {
		return hylaplugin.ActionInvokeResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown action: "+p.ActionID)
	}
	loc := s.resolveLocale(p.Locale)
	stRes, err := host.GetState(ctx)
	if err != nil {
		return hylaplugin.ActionInvokeResult{}, err
	}
	sess := findSession(stRes, p.SessionID, p.PaneID)
	var sessID, splitID string
	if sess != nil {
		sessID = sess.ID
		s.rememberSession(sessID)
		splitID = p.PaneID
		if splitID == "" {
			splitID = sess.Focus
		}
		if splitID == "" {
			splitID = stRes.FocusedPane
		}
	}
	if err := host.OpenPane(ctx, hylaplugin.UIOpenPaneParams{
		Session:       sessID,
		Split:         splitID,
		Plugin:        pluginID,
		View:          slotID,
		Title:         localizedPomodoroTitle(loc),
		Direction:     "right",
		FocusExisting: true,
	}); err != nil {
		return hylaplugin.ActionInvokeResult{}, err
	}
	return hylaplugin.ActionInvokeResult{OK: true}, nil
}

func (s *pomodoroService) handleWebRPC(ctx context.Context, host *hylaplugin.Host, p hylaplugin.WebViewRPCParams) (hylaplugin.WebViewRPCResult, error) {
	if host != nil {
		s.mu.Lock()
		if s.host == nil {
			s.host = host
		}
		s.mu.Unlock()
	}
	s.rememberSession(p.SessionID)

	st, err := s.ensureStore()
	if err != nil {
		return hylaplugin.WebViewRPCResult{}, err
	}
	loc := s.resolveLocale(p.Locale)
	now := time.Now()

	var snap Snapshot
	mutated := true

	switch p.Method {
	case "pomodoro.bootstrap":
		mutated = false
		snap = st.Snapshot(now)

	case "pomodoro.start":
		snap, err = st.Start(now)

	case "pomodoro.pause":
		snap, err = st.Pause(now)

	case "pomodoro.toggle":
		snap, err = st.Toggle(now)

	case "pomodoro.reset":
		snap, err = st.Reset(now)

	case "pomodoro.skip":
		snap, err = st.Skip(now)

	case "pomodoro.setPhase":
		var req struct {
			Phase Phase `json:"phase"`
		}
		if len(p.Params) > 0 {
			if jerr := json.Unmarshal(p.Params, &req); jerr != nil {
				return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, jerr.Error())
			}
		}
		snap, err = st.SetPhase(req.Phase, now)

	case "pomodoro.task.add":
		var req struct {
			Title    string `json:"title"`
			Note     string `json:"note"`
			EstPomos int    `json:"estPomos"`
		}
		if len(p.Params) > 0 {
			if jerr := json.Unmarshal(p.Params, &req); jerr != nil {
				return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, jerr.Error())
			}
		}
		snap, _, err = st.AddTask(req.Title, req.Note, req.EstPomos, now)

	case "pomodoro.task.update":
		var req struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Note     string `json:"note"`
			EstPomos int    `json:"estPomos"`
		}
		if len(p.Params) > 0 {
			if jerr := json.Unmarshal(p.Params, &req); jerr != nil {
				return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, jerr.Error())
			}
		}
		snap, err = st.UpdateTask(req.ID, req.Title, req.Note, req.EstPomos, now)

	case "pomodoro.task.delete":
		var req struct {
			ID string `json:"id"`
		}
		if len(p.Params) > 0 {
			_ = json.Unmarshal(p.Params, &req)
		}
		snap, err = st.DeleteTask(req.ID, now)

	case "pomodoro.task.toggle":
		var req struct {
			ID string `json:"id"`
		}
		if len(p.Params) > 0 {
			_ = json.Unmarshal(p.Params, &req)
		}
		snap, err = st.ToggleTask(req.ID, now)

	case "pomodoro.task.select":
		var req struct {
			ID string `json:"id"`
		}
		if len(p.Params) > 0 {
			_ = json.Unmarshal(p.Params, &req)
		}
		snap, err = st.SelectTask(req.ID, now)

	case "pomodoro.task.move":
		var req struct {
			ID    string `json:"id"`
			Delta int    `json:"delta"`
		}
		if len(p.Params) > 0 {
			_ = json.Unmarshal(p.Params, &req)
		}
		snap, err = st.MoveTask(req.ID, req.Delta, now)

	case "pomodoro.task.clearFinished":
		snap, err = st.ClearFinishedTasks(now)

	case "pomodoro.task.clearAct":
		snap, err = st.ClearActPomos(now)

	case "pomodoro.task.clearAll":
		snap, err = st.ClearAllTasks(now)

	case "pomodoro.settings.update":
		var req Settings
		if len(p.Params) > 0 {
			if jerr := json.Unmarshal(p.Params, &req); jerr != nil {
				return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, jerr.Error())
			}
		}
		req = req.Normalize()
		snap, err = st.UpdateSettings(req, now)
		if err == nil && host != nil {
			_ = host.SetSettings(ctx, SettingsToMap(req), nil)
		}

	case "pomodoro.history.clear":
		snap, err = st.ClearHistory(now)

	default:
		return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown method: "+p.Method)
	}

	if err != nil {
		return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, err.Error())
	}

	if mutated {
		s.emitState(snap)
	}

	payload := map[string]any{
		"locale":   loc,
		"snapshot": snap,
	}
	raw, merr := json.Marshal(payload)
	if merr != nil {
		return hylaplugin.WebViewRPCResult{}, merr
	}
	return hylaplugin.WebViewRPCResult{Data: raw}, nil
}

func (s *pomodoroService) handleNotify(ctx context.Context, host *hylaplugin.Host, ev hylaplugin.EventNotifyParams) {
	switch ev.Topic {
	case "locale.changed":
		var data struct {
			Locale string `json:"locale"`
		}
		if json.Unmarshal(ev.Data, &data) == nil && strings.TrimSpace(data.Locale) != "" {
			s.setLocale(data.Locale)
			if s.store != nil {
				s.emitState(s.store.Snapshot(time.Now()))
			}
		}
	case "settings.changed":
		s.reloadSettings(ctx)
	}
}

func findSession(res *hylaplugin.HostGetStateResult, sessionID, paneID string) *hylaplugin.Session {
	if res == nil || len(res.State.Sessions) == 0 {
		return nil
	}
	sessions := res.State.Sessions
	if sessionID != "" {
		for i := range sessions {
			if sessions[i].ID == sessionID {
				return &sessions[i]
			}
		}
	}
	if paneID != "" {
		for i := range sessions {
			for j := range sessions[i].Panes {
				if sessions[i].Panes[j].ID == paneID {
					return &sessions[i]
				}
			}
		}
	}
	if res.ActiveSession != "" {
		for i := range sessions {
			if sessions[i].ID == res.ActiveSession {
				return &sessions[i]
			}
		}
	}
	return &sessions[0]
}

func normalizeLocale(loc string) string {
	l := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(loc, "_", "-")))
	switch {
	case l == "zh-hans" || l == "zh-cn" || l == "zh-sg" || l == "zh" || strings.HasPrefix(l, "zh-hans"):
		return "zh-Hans"
	case l == "ja" || strings.HasPrefix(l, "ja-"):
		return "ja"
	case l == "en" || strings.HasPrefix(l, "en-"):
		return "en"
	default:
		if strings.TrimSpace(loc) != "" {
			return strings.TrimSpace(loc)
		}
		return "en"
	}
}

func localizedPomodoroTitle(locale string) string {
	switch normalizeLocale(locale) {
	case "zh-Hans":
		return "番茄钟"
	case "ja":
		return "ポモドーロ"
	default:
		return "Pomodoro"
	}
}

func localizedCompletionMessage(locale string, ev *CompletionEvent) (string, string) {
	loc := normalizeLocale(locale)
	taskNote := ""
	if ev.TaskTitle != "" {
		taskNote = ev.TaskTitle
	}
	switch ev.CompletedPhase {
	case PhasePomodoro:
		switch loc {
		case "zh-Hans":
			title := fmt.Sprintf("专注完成（%d 分钟）", ev.DurationMin)
			if ev.NextPhase == PhaseLongBreak {
				if taskNote != "" {
					return title, fmt.Sprintf("已完成「%s」，休息一下进入长休息吧。", taskNote)
				}
				return title, "连续专注达成，开始长休息放松一下吧。"
			}
			if taskNote != "" {
				return title, fmt.Sprintf("已完成「%s」，喝口水短休片刻。", taskNote)
			}
			return title, "太棒了，现在进入短休息时间。"
		case "ja":
			title := fmt.Sprintf("集中セッション完了（%d分）", ev.DurationMin)
			if ev.NextPhase == PhaseLongBreak {
				if taskNote != "" {
					return title, fmt.Sprintf("「%s」完了。長い休憩でリフレッシュしましょう。", taskNote)
				}
				return title, "サイクル達成です。長い休憩を取りましょう。"
			}
			if taskNote != "" {
				return title, fmt.Sprintf("「%s」完了。短い休憩を取りましょう。", taskNote)
			}
			return title, "お疲れさまでした。短い休憩の時間です。"
		default:
			title := fmt.Sprintf("Focus complete (%dm)", ev.DurationMin)
			if ev.NextPhase == PhaseLongBreak {
				if taskNote != "" {
					return title, fmt.Sprintf("%s — time for a long break.", taskNote)
				}
				return title, "Cycle complete — take a well-earned long break."
			}
			if taskNote != "" {
				return title, fmt.Sprintf("%s — time for a short break.", taskNote)
			}
			return title, "Great focus — time for a short break."
		}
	default:
		switch loc {
		case "zh-Hans":
			return "休息结束", "准备好开始新一轮专注了吗？"
		case "ja":
			return "休憩終了", "次の集中セッションを始めましょう。"
		default:
			return "Break complete", "Ready for your next focus session?"
		}
	}
}

func newPlugin() *hylaplugin.Plugin {
	svc := newPomodoroService()
	return &hylaplugin.Plugin{
		Name:          "Pomodoro",
		Version:       "1.0.0",
		Capabilities:  []string{"ui.webview"},
		Subscriptions: []string{"locale.changed", "settings.changed"},
		OnInitialize: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.InitializeParams) error {
			return svc.initRuntime(ctx, host, p.DataDir, p.Locale, p.Settings)
		},
		OnAction:     svc.handleAction,
		OnWebViewRPC: svc.handleWebRPC,
		OnNotify:     svc.handleNotify,
		OnShutdown: func(context.Context) {
			svc.shutdown()
		},
	}
}

func main() {
	if err := hylaplugin.Run(newPlugin()); err != nil {
		log.Fatal(err)
	}
}
