package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Bookmark represents a custom web game URL saved by the user.
type Bookmark struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	AddedAt int64  `json:"addedAt"`
}

// GameSettings holds user preferences synchronized from Hyla plugin settings.
type GameSettings struct {
	AutoPause   bool   `json:"autoPause"`
	DefaultGame string `json:"defaultGame"`
}

// StoreState is the persisted state saved to state.json.
type StoreState struct {
	HighScores  map[string]int `json:"highScores"`
	GamesPlayed int            `json:"gamesPlayed"`
	Bookmarks   []Bookmark     `json:"bookmarks"`
	Settings    GameSettings   `json:"settings"`
}

// Store persists webgames high scores, bookmarks, and settings atomically.
type Store struct {
	mu      sync.Mutex
	baseDir string
	state   StoreState
}

// NewStore opens or initializes state.json under baseDir.
func NewStore(baseDir string) (*Store, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, errors.New("baseDir is required")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}
	st := &Store{
		baseDir: baseDir,
		state: StoreState{
			HighScores: map[string]int{},
			Settings: GameSettings{
				AutoPause:   true,
				DefaultGame: "2048",
			},
		},
	}
	if b, err := os.ReadFile(st.path()); err == nil && len(b) > 0 {
		var loaded StoreState
		if json.Unmarshal(b, &loaded) == nil {
			if loaded.HighScores == nil {
				loaded.HighScores = map[string]int{}
			}
			if loaded.Settings.DefaultGame == "" {
				loaded.Settings.DefaultGame = "2048"
			}
			st.state = loaded
		}
	}
	return st, nil
}

func (s *Store) path() string { return filepath.Join(s.baseDir, "state.json") }

// Snapshot returns a deep copy of the current state.
func (s *Store) Snapshot() StoreState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Store) snapshotLocked() StoreState {
	hs := make(map[string]int, len(s.state.HighScores))
	for k, v := range s.state.HighScores {
		hs[k] = v
	}
	return StoreState{
		HighScores:  hs,
		GamesPlayed: s.state.GamesPlayed,
		Bookmarks:   slices.Clone(s.state.Bookmarks),
		Settings:    s.state.Settings,
	}
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

// RecordScore increments GamesPlayed and updates the high score for game if score beats the record.
func (s *Store) RecordScore(game string, score int) (StoreState, error) {
	game = strings.TrimSpace(game)
	if game == "" {
		return StoreState{}, errors.New("game name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.HighScores == nil {
		s.state.HighScores = map[string]int{}
	}
	s.state.GamesPlayed++
	if score > s.state.HighScores[game] {
		s.state.HighScores[game] = score
	}
	if err := s.saveLocked(); err != nil {
		return StoreState{}, err
	}
	return s.snapshotLocked(), nil
}

// AddBookmark saves or updates a custom web game URL bookmark.
func (s *Store) AddBookmark(name, rawURL string, now time.Time) (StoreState, error) {
	name = strings.TrimSpace(name)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return StoreState{}, errors.New("url is required")
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	if name == "" {
		name = rawURL
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Bookmarks {
		if s.state.Bookmarks[i].URL == rawURL {
			s.state.Bookmarks[i].Name = name
			if err := s.saveLocked(); err != nil {
				return StoreState{}, err
			}
			return s.snapshotLocked(), nil
		}
	}
	id := "bm-" + strings.ReplaceAll(now.UTC().Format("20060102150405.000"), ".", "")
	s.state.Bookmarks = append(s.state.Bookmarks, Bookmark{
		ID:      id,
		Name:    name,
		URL:     rawURL,
		AddedAt: now.UnixMilli(),
	})
	if err := s.saveLocked(); err != nil {
		return StoreState{}, err
	}
	return s.snapshotLocked(), nil
}

// RemoveBookmark deletes a custom bookmark by id.
func (s *Store) RemoveBookmark(id string) (StoreState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Bookmarks = slices.DeleteFunc(s.state.Bookmarks, func(b Bookmark) bool {
		return b.ID == id
	})
	if err := s.saveLocked(); err != nil {
		return StoreState{}, err
	}
	return s.snapshotLocked(), nil
}

// UpdateSettings updates plugin settings and persists them.
func (s *Store) UpdateSettings(cfg GameSettings) (StoreState, error) {
	if strings.TrimSpace(cfg.DefaultGame) == "" {
		cfg.DefaultGame = "2048"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Settings = cfg
	if err := s.saveLocked(); err != nil {
		return StoreState{}, err
	}
	return s.snapshotLocked(), nil
}
