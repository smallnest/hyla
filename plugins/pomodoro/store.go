package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Phase string

const (
	PhasePomodoro   Phase = "pomodoro"
	PhaseShortBreak Phase = "short_break"
	PhaseLongBreak  Phase = "long_break"

	maxHistoryRecords = 500
	recentReportLimit = 30
)

type Settings struct {
	PomodoroMinutes      int    `json:"pomodoroMinutes"`
	ShortBreakMinutes    int    `json:"shortBreakMinutes"`
	LongBreakMinutes     int    `json:"longBreakMinutes"`
	LongBreakInterval    int    `json:"longBreakInterval"`
	AutoStartBreaks      bool   `json:"autoStartBreaks"`
	AutoStartPomodoros   bool   `json:"autoStartPomodoros"`
	AlarmSound           string `json:"alarmSound"`
	AlarmVolume          int    `json:"alarmVolume"`
	AlarmRepeat          int    `json:"alarmRepeat"`
	AmbientSound         string `json:"ambientSound"`
	AmbientVolume        int    `json:"ambientVolume"`
	DesktopNotifications bool   `json:"desktopNotifications"`
	DailyGoalPomos       int    `json:"dailyGoalPomos"`
}

func DefaultSettings() Settings {
	return Settings{
		PomodoroMinutes:      25,
		ShortBreakMinutes:    5,
		LongBreakMinutes:     15,
		LongBreakInterval:    4,
		AutoStartBreaks:      false,
		AutoStartPomodoros:   false,
		AlarmSound:           "bell",
		AlarmVolume:          70,
		AlarmRepeat:          2,
		AmbientSound:         "none",
		AmbientVolume:        55,
		DesktopNotifications: true,
		DailyGoalPomos:       8,
	}
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func (s Settings) Normalize() Settings {
	def := DefaultSettings()
	if s.PomodoroMinutes <= 0 {
		s.PomodoroMinutes = def.PomodoroMinutes
	}
	if s.ShortBreakMinutes <= 0 {
		s.ShortBreakMinutes = def.ShortBreakMinutes
	}
	if s.LongBreakMinutes <= 0 {
		s.LongBreakMinutes = def.LongBreakMinutes
	}
	if s.LongBreakInterval <= 0 {
		s.LongBreakInterval = def.LongBreakInterval
	}
	if s.AlarmRepeat <= 0 {
		s.AlarmRepeat = def.AlarmRepeat
	}
	if s.DailyGoalPomos <= 0 {
		s.DailyGoalPomos = def.DailyGoalPomos
	}

	s.PomodoroMinutes = clampInt(s.PomodoroMinutes, 1, 180)
	s.ShortBreakMinutes = clampInt(s.ShortBreakMinutes, 1, 60)
	s.LongBreakMinutes = clampInt(s.LongBreakMinutes, 1, 90)
	s.LongBreakInterval = clampInt(s.LongBreakInterval, 1, 12)
	s.AlarmVolume = clampInt(s.AlarmVolume, 0, 100)
	s.AlarmRepeat = clampInt(s.AlarmRepeat, 1, 5)
	s.AmbientVolume = clampInt(s.AmbientVolume, 0, 100)
	s.DailyGoalPomos = clampInt(s.DailyGoalPomos, 1, 48)

	switch strings.ToLower(strings.TrimSpace(s.AlarmSound)) {
	case "bell", "bowl", "marimba", "digital", "none":
		s.AlarmSound = strings.ToLower(strings.TrimSpace(s.AlarmSound))
	default:
		s.AlarmSound = def.AlarmSound
	}
	switch strings.ToLower(strings.TrimSpace(s.AmbientSound)) {
	case "none", "white", "tick", "rain", "brown":
		s.AmbientSound = strings.ToLower(strings.TrimSpace(s.AmbientSound))
	default:
		s.AmbientSound = def.AmbientSound
	}
	return s
}

func (s Settings) DurationSec(phase Phase) int {
	switch phase {
	case PhaseShortBreak:
		return s.ShortBreakMinutes * 60
	case PhaseLongBreak:
		return s.LongBreakMinutes * 60
	default:
		return s.PomodoroMinutes * 60
	}
}

func SettingsFromMap(raw map[string]any, base Settings) Settings {
	out := base.Normalize()
	if len(raw) == 0 {
		return out
	}
	if v, ok := asInt(raw["pomodoro_minutes"]); ok {
		out.PomodoroMinutes = v
	}
	if v, ok := asInt(raw["short_break_minutes"]); ok {
		out.ShortBreakMinutes = v
	}
	if v, ok := asInt(raw["long_break_minutes"]); ok {
		out.LongBreakMinutes = v
	}
	if v, ok := asInt(raw["long_break_interval"]); ok {
		out.LongBreakInterval = v
	}
	if v, ok := asBool(raw["auto_start_breaks"]); ok {
		out.AutoStartBreaks = v
	}
	if v, ok := asBool(raw["auto_start_pomodoros"]); ok {
		out.AutoStartPomodoros = v
	}
	if v, ok := asString(raw["alarm_sound"]); ok {
		out.AlarmSound = v
	}
	if v, ok := asInt(raw["alarm_volume"]); ok {
		out.AlarmVolume = v
	}
	if v, ok := asInt(raw["alarm_repeat"]); ok {
		out.AlarmRepeat = v
	}
	if v, ok := asString(raw["ambient_sound"]); ok {
		out.AmbientSound = v
	}
	if v, ok := asInt(raw["ambient_volume"]); ok {
		out.AmbientVolume = v
	}
	if v, ok := asBool(raw["desktop_notifications"]); ok {
		out.DesktopNotifications = v
	}
	if v, ok := asInt(raw["daily_goal_pomos"]); ok {
		out.DailyGoalPomos = v
	}
	return out.Normalize()
}

func SettingsToMap(s Settings) map[string]any {
	s = s.Normalize()
	return map[string]any{
		"pomodoro_minutes":      s.PomodoroMinutes,
		"short_break_minutes":   s.ShortBreakMinutes,
		"long_break_minutes":    s.LongBreakMinutes,
		"long_break_interval":   s.LongBreakInterval,
		"auto_start_breaks":     s.AutoStartBreaks,
		"auto_start_pomodoros":  s.AutoStartPomodoros,
		"alarm_sound":           s.AlarmSound,
		"alarm_volume":          s.AlarmVolume,
		"alarm_repeat":          s.AlarmRepeat,
		"ambient_sound":         s.AmbientSound,
		"ambient_volume":        s.AmbientVolume,
		"desktop_notifications": s.DesktopNotifications,
		"daily_goal_pomos":      s.DailyGoalPomos,
	}
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case int32:
		return int(x), true
	case float64:
		return int(math.Round(x)), true
	case float32:
		return int(math.Round(float64(x))), true
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return int(i), true
		}
		if f, err := x.Float64(); err == nil {
			return int(math.Round(f)), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			return i, true
		}
	}
	return 0, false
}

func asBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	}
	return false, false
}

func asString(v any) (string, bool) {
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

type Task struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Note      string `json:"note,omitempty"`
	EstPomos  int    `json:"estPomos"`
	DonePomos int    `json:"donePomos"`
	Completed bool   `json:"completed"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type SessionRecord struct {
	ID          string `json:"id"`
	Phase       Phase  `json:"phase"`
	DurationMin int    `json:"durationMin"`
	TaskID      string `json:"taskId,omitempty"`
	TaskTitle   string `json:"taskTitle,omitempty"`
	CompletedAt int64  `json:"completedAt"`
	Day         string `json:"day"`
}

type TimerState struct {
	Phase         Phase  `json:"phase"`
	Running       bool   `json:"running"`
	TotalSec      int    `json:"totalSec"`
	RemainingSec  int    `json:"remainingSec"`
	EndAtMs       int64  `json:"endAtMs,omitempty"`
	CycleCount    int    `json:"cycleCount"`
	SessionNumber int    `json:"sessionNumber"`
	LastResetDay  string `json:"lastResetDay,omitempty"`
}

type CompletionEvent struct {
	CompletedPhase Phase  `json:"completedPhase"`
	NextPhase      Phase  `json:"nextPhase"`
	DurationMin    int    `json:"durationMin"`
	TaskID         string `json:"taskId,omitempty"`
	TaskTitle      string `json:"taskTitle,omitempty"`
	AutoStarted    bool   `json:"autoStarted"`
	AlarmSound     string `json:"alarmSound"`
	AlarmVolume    int    `json:"alarmVolume"`
	AlarmRepeat    int    `json:"alarmRepeat"`
}

type DayStat struct {
	Day     string `json:"day"`
	Pomos   int    `json:"pomos"`
	Minutes int    `json:"minutes"`
}

type ReportSummary struct {
	TodayPomos   int             `json:"todayPomos"`
	TodayMinutes int             `json:"todayMinutes"`
	TotalPomos   int             `json:"totalPomos"`
	TotalMinutes int             `json:"totalMinutes"`
	StreakDays   int             `json:"streakDays"`
	DailyGoal    int             `json:"dailyGoal"`
	Last7Days    []DayStat       `json:"last7Days"`
	Recent       []SessionRecord `json:"recent"`
}

type Snapshot struct {
	Timer        TimerState    `json:"timer"`
	Tasks        []Task        `json:"tasks"`
	ActiveTaskID string        `json:"activeTaskId,omitempty"`
	Settings     Settings      `json:"settings"`
	Report       ReportSummary `json:"report"`
	ServerNowMs  int64         `json:"serverNowMs"`
}

type PersistedState struct {
	Version      int             `json:"version"`
	Settings     Settings        `json:"settings"`
	Timer        TimerState      `json:"timer"`
	Tasks        []Task          `json:"tasks"`
	ActiveTaskID string          `json:"activeTaskId,omitempty"`
	History      []SessionRecord `json:"history"`
	NextSeq      int64           `json:"nextSeq"`
}

type Store struct {
	mu  sync.RWMutex
	dir string
	st  PersistedState
}

func NewStore(dataDir string) (*Store, error) {
	return newStoreAt(dataDir, time.Now())
}

func newStoreAt(dataDir string, now time.Time) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dataDir}
	if err := s.load(now); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) statePath() string {
	return filepath.Join(s.dir, "state.json")
}

func validPhase(p Phase) bool {
	return p == PhasePomodoro || p == PhaseShortBreak || p == PhaseLongBreak
}

func dayKey(t time.Time) string {
	return t.Local().Format("2006-01-02")
}

func (s *Store) load(now time.Time) error {
	raw, err := os.ReadFile(s.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			def := DefaultSettings()
			s.st = PersistedState{
				Version:  1,
				Settings: def,
				Timer: TimerState{
					Phase:         PhasePomodoro,
					Running:       false,
					TotalSec:      def.DurationSec(PhasePomodoro),
					RemainingSec:  def.DurationSec(PhasePomodoro),
					SessionNumber: 1,
					LastResetDay:  dayKey(now),
				},
				Tasks:   []Task{},
				History: []SessionRecord{},
			}
			return nil
		}
		return err
	}

	var ps PersistedState
	if err := json.Unmarshal(raw, &ps); err != nil {
		return fmt.Errorf("decode %s: %w", s.statePath(), err)
	}
	if ps.Version == 0 {
		ps.Version = 1
	}
	ps.Settings = ps.Settings.Normalize()
	if !validPhase(ps.Timer.Phase) {
		ps.Timer.Phase = PhasePomodoro
	}
	if ps.Timer.TotalSec <= 0 {
		ps.Timer.TotalSec = ps.Settings.DurationSec(ps.Timer.Phase)
	}
	if ps.Timer.SessionNumber <= 0 {
		ps.Timer.SessionNumber = 1
	}
	if ps.Timer.CycleCount < 0 {
		ps.Timer.CycleCount = 0
	}
	if ps.Timer.LastResetDay == "" {
		ps.Timer.LastResetDay = dayKey(now)
	}
	if ps.Tasks == nil {
		ps.Tasks = []Task{}
	}
	if ps.History == nil {
		ps.History = []SessionRecord{}
	}

	// Reconcile a timer saved while running: keep remaining time if still in the
	// future, or pause cleanly at the phase duration if it expired while closed.
	if ps.Timer.Running {
		nowMs := now.UnixMilli()
		if ps.Timer.EndAtMs > nowMs {
			rem := int((ps.Timer.EndAtMs - nowMs + 999) / 1000)
			if rem > ps.Timer.TotalSec {
				rem = ps.Timer.TotalSec
			}
			ps.Timer.RemainingSec = rem
		} else {
			ps.Timer.Running = false
			ps.Timer.EndAtMs = 0
			ps.Timer.TotalSec = ps.Settings.DurationSec(ps.Timer.Phase)
			ps.Timer.RemainingSec = ps.Timer.TotalSec
		}
	} else {
		if ps.Timer.RemainingSec <= 0 || ps.Timer.RemainingSec > ps.Timer.TotalSec {
			ps.Timer.RemainingSec = ps.Timer.TotalSec
		}
		ps.Timer.EndAtMs = 0
	}

	s.st = ps
	s.ensureActiveTaskLocked()
	return nil
}

func writeAtomicJSON(target string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func (s *Store) saveLocked() error {
	return writeAtomicJSON(s.statePath(), s.st)
}

func (s *Store) nextIDLocked(prefix string, now time.Time) string {
	s.st.NextSeq++
	return fmt.Sprintf("%s-%d-%d", prefix, now.UnixMilli(), s.st.NextSeq)
}

func (s *Store) maybeRollDayLocked(now time.Time) {
	today := dayKey(now)
	if s.st.Timer.LastResetDay == today {
		return
	}
	if !s.st.Timer.Running {
		s.st.Timer.CycleCount = 0
		s.st.Timer.SessionNumber = 1
		s.st.Timer.LastResetDay = today
	}
}

func (s *Store) ensureActiveTaskLocked() {
	if s.st.ActiveTaskID != "" {
		for _, t := range s.st.Tasks {
			if t.ID == s.st.ActiveTaskID {
				return
			}
		}
		s.st.ActiveTaskID = ""
	}
	for _, t := range s.st.Tasks {
		if !t.Completed {
			s.st.ActiveTaskID = t.ID
			return
		}
	}
	if len(s.st.Tasks) > 0 {
		s.st.ActiveTaskID = s.st.Tasks[0].ID
	}
}

func (s *Store) liveRemainingLocked(now time.Time) int {
	if !s.st.Timer.Running {
		return s.st.Timer.RemainingSec
	}
	diffMs := s.st.Timer.EndAtMs - now.UnixMilli()
	if diffMs <= 0 {
		return 0
	}
	rem := int((diffMs + 999) / 1000)
	if rem > s.st.Timer.TotalSec {
		return s.st.Timer.TotalSec
	}
	return rem
}

func (s *Store) snapshotLocked(now time.Time) Snapshot {
	timer := s.st.Timer
	timer.RemainingSec = s.liveRemainingLocked(now)
	tasks := slices.Clone(s.st.Tasks)
	if tasks == nil {
		tasks = []Task{}
	}
	return Snapshot{
		Timer:        timer,
		Tasks:        tasks,
		ActiveTaskID: s.st.ActiveTaskID,
		Settings:     s.st.Settings,
		Report:       s.buildReportLocked(now),
		ServerNowMs:  now.UnixMilli(),
	}
}

func (s *Store) buildReportLocked(now time.Time) ReportSummary {
	today := dayKey(now)
	byDay := make(map[string]DayStat)
	var totalPomos, totalMinutes int
	for _, rec := range s.st.History {
		if rec.Phase != PhasePomodoro {
			continue
		}
		totalPomos++
		totalMinutes += rec.DurationMin
		ds := byDay[rec.Day]
		ds.Day = rec.Day
		ds.Pomos++
		ds.Minutes += rec.DurationMin
		byDay[rec.Day] = ds
	}

	todayStat := byDay[today]

	// Last 7 days ending today, oldest -> newest.
	localToday := time.Date(now.Local().Year(), now.Local().Month(), now.Local().Day(), 0, 0, 0, 0, now.Local().Location())
	last7 := make([]DayStat, 7)
	for i := 0; i < 7; i++ {
		d := localToday.AddDate(0, 0, -(6 - i)).Format("2006-01-02")
		if st, ok := byDay[d]; ok {
			last7[i] = st
		} else {
			last7[i] = DayStat{Day: d}
		}
	}

	// Streak counts consecutive days with at least one completed pomodoro,
	// starting from today if today has one, otherwise yesterday.
	streak := 0
	cursor := localToday
	if byDay[cursor.Format("2006-01-02")].Pomos == 0 {
		cursor = cursor.AddDate(0, 0, -1)
	}
	for byDay[cursor.Format("2006-01-02")].Pomos > 0 {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}

	// Recent records newest first (focus sessions only, up to recentReportLimit).
	recent := make([]SessionRecord, 0, recentReportLimit)
	for i := len(s.st.History) - 1; i >= 0 && len(recent) < recentReportLimit; i-- {
		rec := s.st.History[i]
		if rec.Phase == PhasePomodoro {
			recent = append(recent, rec)
		}
	}

	return ReportSummary{
		TodayPomos:   todayStat.Pomos,
		TodayMinutes: todayStat.Minutes,
		TotalPomos:   totalPomos,
		TotalMinutes: totalMinutes,
		StreakDays:   streak,
		DailyGoal:    s.st.Settings.DailyGoalPomos,
		Last7Days:    last7,
		Recent:       recent,
	}
}

func (s *Store) Snapshot(now time.Time) Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked(now)
}

func (s *Store) Start(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maybeRollDayLocked(now)
	if !s.st.Timer.Running {
		if s.st.Timer.RemainingSec <= 0 {
			dur := s.st.Settings.DurationSec(s.st.Timer.Phase)
			s.st.Timer.TotalSec = dur
			s.st.Timer.RemainingSec = dur
		}
		s.st.Timer.Running = true
		s.st.Timer.EndAtMs = now.UnixMilli() + int64(s.st.Timer.RemainingSec)*1000
		if err := s.saveLocked(); err != nil {
			return Snapshot{}, err
		}
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) Pause(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st.Timer.Running {
		s.st.Timer.RemainingSec = s.liveRemainingLocked(now)
		s.st.Timer.Running = false
		s.st.Timer.EndAtMs = 0
		if err := s.saveLocked(); err != nil {
			return Snapshot{}, err
		}
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) Toggle(now time.Time) (Snapshot, error) {
	s.mu.RLock()
	running := s.st.Timer.Running
	s.mu.RUnlock()
	if running {
		return s.Pause(now)
	}
	return s.Start(now)
}

func (s *Store) Reset(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dur := s.st.Settings.DurationSec(s.st.Timer.Phase)
	s.st.Timer.Running = false
	s.st.Timer.EndAtMs = 0
	s.st.Timer.TotalSec = dur
	s.st.Timer.RemainingSec = dur
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) SetPhase(phase Phase, now time.Time) (Snapshot, error) {
	if !validPhase(phase) {
		return Snapshot{}, fmt.Errorf("invalid phase %q", phase)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maybeRollDayLocked(now)
	dur := s.st.Settings.DurationSec(phase)
	s.st.Timer.Phase = phase
	s.st.Timer.Running = false
	s.st.Timer.EndAtMs = 0
	s.st.Timer.TotalSec = dur
	s.st.Timer.RemainingSec = dur
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) SetDuration(seconds int, now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st.Timer.Running || s.st.Timer.RemainingSec != s.st.Timer.TotalSec {
		return Snapshot{}, fmt.Errorf("timer duration can only be customized before starting")
	}
	seconds = clampInt(seconds, 10, 180*60)
	s.st.Timer.TotalSec = seconds
	s.st.Timer.RemainingSec = seconds
	s.st.Timer.EndAtMs = 0
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) Skip(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maybeRollDayLocked(now)
	var next Phase
	if s.st.Timer.Phase == PhasePomodoro {
		if (s.st.Timer.CycleCount + 1) >= s.st.Settings.LongBreakInterval {
			next = PhaseLongBreak
		} else {
			next = PhaseShortBreak
		}
	} else {
		next = PhasePomodoro
	}
	dur := s.st.Settings.DurationSec(next)
	s.st.Timer.Phase = next
	s.st.Timer.Running = false
	s.st.Timer.EndAtMs = 0
	s.st.Timer.TotalSec = dur
	s.st.Timer.RemainingSec = dur
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) Tick(now time.Time) (*CompletionEvent, Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.st.Timer.Running || now.UnixMilli() < s.st.Timer.EndAtMs {
		return nil, s.snapshotLocked(now), nil
	}

	completedPhase := s.st.Timer.Phase
	durationMin := s.st.Timer.TotalSec / 60
	if durationMin < 1 {
		durationMin = 1
	}

	var taskID, taskTitle string
	var nextPhase Phase
	var autoStart bool

	if completedPhase == PhasePomodoro {
		s.st.Timer.CycleCount++
		s.st.Timer.SessionNumber++
		s.st.Timer.LastResetDay = dayKey(now)

		if s.st.ActiveTaskID != "" {
			for i := range s.st.Tasks {
				if s.st.Tasks[i].ID == s.st.ActiveTaskID && !s.st.Tasks[i].Completed {
					s.st.Tasks[i].DonePomos++
					s.st.Tasks[i].UpdatedAt = now.UnixMilli()
					taskID = s.st.Tasks[i].ID
					taskTitle = s.st.Tasks[i].Title
					break
				}
			}
		}

		s.st.History = append(s.st.History, SessionRecord{
			ID:          s.nextIDLocked("rec", now),
			Phase:       PhasePomodoro,
			DurationMin: durationMin,
			TaskID:      taskID,
			TaskTitle:   taskTitle,
			CompletedAt: now.UnixMilli(),
			Day:         dayKey(now),
		})
		if len(s.st.History) > maxHistoryRecords {
			s.st.History = slices.Clone(s.st.History[len(s.st.History)-maxHistoryRecords:])
		}

		if s.st.Timer.CycleCount >= s.st.Settings.LongBreakInterval {
			nextPhase = PhaseLongBreak
			s.st.Timer.CycleCount = 0
		} else {
			nextPhase = PhaseShortBreak
		}
		autoStart = s.st.Settings.AutoStartBreaks
	} else {
		nextPhase = PhasePomodoro
		autoStart = s.st.Settings.AutoStartPomodoros
	}

	nextDur := s.st.Settings.DurationSec(nextPhase)
	s.st.Timer.Phase = nextPhase
	s.st.Timer.TotalSec = nextDur
	s.st.Timer.RemainingSec = nextDur
	s.st.Timer.Running = autoStart
	if autoStart {
		s.st.Timer.EndAtMs = now.UnixMilli() + int64(nextDur)*1000
	} else {
		s.st.Timer.EndAtMs = 0
	}

	if err := s.saveLocked(); err != nil {
		return nil, Snapshot{}, err
	}

	ev := &CompletionEvent{
		CompletedPhase: completedPhase,
		NextPhase:      nextPhase,
		DurationMin:    durationMin,
		TaskID:         taskID,
		TaskTitle:      taskTitle,
		AutoStarted:    autoStart,
		AlarmSound:     s.st.Settings.AlarmSound,
		AlarmVolume:    s.st.Settings.AlarmVolume,
		AlarmRepeat:    s.st.Settings.AlarmRepeat,
	}
	return ev, s.snapshotLocked(now), nil
}

func (s *Store) AddTask(title, note string, estPomos int, now time.Time) (Snapshot, Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Snapshot{}, Task{}, fmt.Errorf("task title cannot be empty")
	}
	note = strings.TrimSpace(note)
	estPomos = clampInt(estPomos, 1, 99)

	s.mu.Lock()
	defer s.mu.Unlock()
	ts := now.UnixMilli()
	task := Task{
		ID:        s.nextIDLocked("task", now),
		Title:     title,
		Note:      note,
		EstPomos:  estPomos,
		CreatedAt: ts,
		UpdatedAt: ts,
	}
	s.st.Tasks = append(s.st.Tasks, task)
	if s.st.ActiveTaskID == "" {
		s.st.ActiveTaskID = task.ID
	}
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, Task{}, err
	}
	return s.snapshotLocked(now), task, nil
}

func (s *Store) UpdateTask(id, title, note string, estPomos int, now time.Time) (Snapshot, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Snapshot{}, fmt.Errorf("task title cannot be empty")
	}
	note = strings.TrimSpace(note)
	estPomos = clampInt(estPomos, 1, 99)

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.st.Tasks {
		if s.st.Tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Snapshot{}, fmt.Errorf("task %q not found", id)
	}
	s.st.Tasks[idx].Title = title
	s.st.Tasks[idx].Note = note
	s.st.Tasks[idx].EstPomos = estPomos
	s.st.Tasks[idx].UpdatedAt = now.UnixMilli()
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) DeleteTask(id string, now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.st.Tasks {
		if s.st.Tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Snapshot{}, fmt.Errorf("task %q not found", id)
	}
	s.st.Tasks = slices.Delete(s.st.Tasks, idx, idx+1)
	if s.st.ActiveTaskID == id {
		s.st.ActiveTaskID = ""
		s.ensureActiveTaskLocked()
	}
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) ToggleTask(id string, now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.st.Tasks {
		if s.st.Tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Snapshot{}, fmt.Errorf("task %q not found", id)
	}
	s.st.Tasks[idx].Completed = !s.st.Tasks[idx].Completed
	s.st.Tasks[idx].UpdatedAt = now.UnixMilli()
	if s.st.Tasks[idx].Completed && s.st.ActiveTaskID == id {
		s.st.ActiveTaskID = ""
		for _, t := range s.st.Tasks {
			if !t.Completed {
				s.st.ActiveTaskID = t.ID
				break
			}
		}
		if s.st.ActiveTaskID == "" {
			s.st.ActiveTaskID = id
		}
	}
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) SelectTask(id string, now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		s.st.ActiveTaskID = ""
		if err := s.saveLocked(); err != nil {
			return Snapshot{}, err
		}
		return s.snapshotLocked(now), nil
	}
	found := false
	for _, t := range s.st.Tasks {
		if t.ID == id {
			found = true
			break
		}
	}
	if !found {
		return Snapshot{}, fmt.Errorf("task %q not found", id)
	}
	s.st.ActiveTaskID = id
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) MoveTask(id string, delta int, now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.st.Tasks {
		if s.st.Tasks[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Snapshot{}, fmt.Errorf("task %q not found", id)
	}
	target := idx + delta
	if target < 0 {
		target = 0
	}
	if target >= len(s.st.Tasks) {
		target = len(s.st.Tasks) - 1
	}
	if target != idx {
		item := s.st.Tasks[idx]
		s.st.Tasks = slices.Delete(s.st.Tasks, idx, idx+1)
		s.st.Tasks = slices.Insert(s.st.Tasks, target, item)
		if err := s.saveLocked(); err != nil {
			return Snapshot{}, err
		}
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) ClearFinishedTasks(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Task, 0, len(s.st.Tasks))
	for _, t := range s.st.Tasks {
		if !t.Completed {
			kept = append(kept, t)
		}
	}
	s.st.Tasks = kept
	s.ensureActiveTaskLocked()
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) ClearActPomos(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.st.Tasks {
		s.st.Tasks[i].DonePomos = 0
		s.st.Tasks[i].UpdatedAt = now.UnixMilli()
	}
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) ClearAllTasks(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Tasks = []Task{}
	s.st.ActiveTaskID = ""
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) UpdateSettings(next Settings, now time.Time) (Snapshot, error) {
	next = next.Normalize()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Settings = next
	if !s.st.Timer.Running && s.st.Timer.RemainingSec == s.st.Timer.TotalSec {
		dur := next.DurationSec(s.st.Timer.Phase)
		s.st.Timer.TotalSec = dur
		s.st.Timer.RemainingSec = dur
	}
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}

func (s *Store) ClearHistory(now time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.History = []SessionRecord{}
	if err := s.saveLocked(); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotLocked(now), nil
}
