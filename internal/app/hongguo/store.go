package hongguo

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const maxHistoryEntries = 20

// ValidSpeeds lists the playback rates offered by the player UI.
var ValidSpeeds = []float64{0.5, 0.75, 1, 1.25, 1.5, 2}

// HistoryEntry records the last watched episode and playback timestamp of a drama.
type HistoryEntry struct {
	SeriesID  string  `json:"seriesId"`
	Title     string  `json:"title"`
	CoverURL  string  `json:"coverUrl,omitempty"`
	EpisodeID string  `json:"episodeId"`
	Number    int     `json:"number"`
	Position  float64 `json:"position"`
	UpdatedAt int64   `json:"updatedAt"`
}

// Prefs stores user playback preferences across sessions.
type Prefs struct {
	Volume    float64 `json:"volume"`
	Muted     bool    `json:"muted,omitempty"`
	Rate      float64 `json:"rate"`
	AutoPause bool    `json:"autoPause"`
}

func defaultPrefs() Prefs {
	return Prefs{Volume: 1.0, Muted: false, Rate: 1.0, AutoPause: true}
}

func clampPrefs(p Prefs) Prefs {
	if math.IsNaN(p.Volume) || math.IsInf(p.Volume, 0) {
		p.Volume = 1.0
	}
	p.Volume = min(max(p.Volume, 0), 1)
	if !slices.Contains(ValidSpeeds, p.Rate) {
		p.Rate = 1.0
	}
	return p
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Service) historyPath() string { return filepath.Join(s.dir, "history.json") }
func (s *Service) prefsPath() string   { return filepath.Join(s.dir, "prefs.json") }

func (s *Service) loadStoreLocked() {
	if s.storeLoaded {
		return
	}
	s.storeLoaded = true
	s.prefs = defaultPrefs()
	if b, err := os.ReadFile(s.prefsPath()); err == nil {
		var p Prefs
		if json.Unmarshal(b, &p) == nil {
			s.prefs = clampPrefs(p)
		}
	}
	if b, err := os.ReadFile(s.historyPath()); err == nil {
		var list []HistoryEntry
		if json.Unmarshal(b, &list) == nil {
			s.history = sortAndTrimHistory(list)
		}
	}
}

func sortAndTrimHistory(list []HistoryEntry) []HistoryEntry {
	seen := make(map[string]bool, len(list))
	slices.SortStableFunc(list, func(a, b HistoryEntry) int {
		if a.UpdatedAt > b.UpdatedAt {
			return -1
		}
		if a.UpdatedAt < b.UpdatedAt {
			return 1
		}
		return 0
	})
	var out []HistoryEntry
	for _, it := range list {
		if it.SeriesID == "" || seen[it.SeriesID] {
			continue
		}
		seen[it.SeriesID] = true
		out = append(out, it)
		if len(out) >= maxHistoryEntries {
			break
		}
	}
	return out
}

func (s *Service) saveHistoryLocked() {
	b, err := json.MarshalIndent(s.history, "", "  ")
	if err != nil {
		return
	}
	_ = writeAtomic(s.historyPath(), b)
}

// Prefs returns a copy of the saved player preferences.
func (s *Service) Prefs() Prefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadStoreLocked()
	return s.prefs
}

// SavePrefs persists updated player preferences.
func (s *Service) SavePrefs(p Prefs) {
	p = clampPrefs(p)
	s.mu.Lock()
	s.loadStoreLocked()
	s.prefs = p
	s.mu.Unlock()
	if b, err := json.MarshalIndent(p, "", "  "); err == nil {
		_ = writeAtomic(s.prefsPath(), b)
	}
}

// History returns a copy of the watch history, ordered newest first.
func (s *Service) History() []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadStoreLocked()
	out := make([]HistoryEntry, len(s.history))
	copy(out, s.history)
	return out
}

// ProgressOf returns the saved position (seconds) for the given series and episode, if any.
func (s *Service) ProgressOf(seriesID, episodeID string) float64 {
	if seriesID == "" || episodeID == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadStoreLocked()
	for _, it := range s.history {
		if it.SeriesID == seriesID && it.EpisodeID == episodeID {
			return it.Position
		}
	}
	return 0
}

// HistoryOf returns the saved history entry for seriesID, if any.
func (s *Service) HistoryOf(seriesID string) (HistoryEntry, bool) {
	if seriesID == "" {
		return HistoryEntry{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadStoreLocked()
	for _, it := range s.history {
		if it.SeriesID == seriesID {
			return it, true
		}
	}
	return HistoryEntry{}, false
}

// SaveProgress updates watch history for entry and persists it to disk.
func (s *Service) SaveProgress(entry HistoryEntry) {
	if entry.SeriesID == "" || entry.EpisodeID == "" {
		return
	}
	if entry.Position < 0 || math.IsNaN(entry.Position) || math.IsInf(entry.Position, 0) {
		entry.Position = 0
	}
	if entry.UpdatedAt == 0 {
		entry.UpdatedAt = time.Now().UnixMilli()
	}
	s.mu.Lock()
	s.loadStoreLocked()
	var updated []HistoryEntry
	updated = append(updated, entry)
	for _, it := range s.history {
		if it.SeriesID != entry.SeriesID {
			updated = append(updated, it)
		}
	}
	s.history = sortAndTrimHistory(updated)
	s.saveHistoryLocked()
	s.mu.Unlock()
}

// RemoveHistory removes a single series from watch history.
func (s *Service) RemoveHistory(seriesID string) {
	if seriesID == "" {
		return
	}
	s.mu.Lock()
	s.loadStoreLocked()
	var out []HistoryEntry
	for _, it := range s.history {
		if it.SeriesID != seriesID {
			out = append(out, it)
		}
	}
	s.history = out
	s.saveHistoryLocked()
	s.mu.Unlock()
}

// ClearHistory deletes all saved watch history entries.
func (s *Service) ClearHistory() {
	s.mu.Lock()
	s.loadStoreLocked()
	s.history = nil
	s.saveHistoryLocked()
	s.mu.Unlock()
}
