package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

const maxRecentChannels = 60

type RecentEntry struct {
	ChannelID string `json:"channelId"`
	LineIdx   int    `json:"lineIdx"`
	WatchedAt int64  `json:"watchedAt"`
}

type PersistedState struct {
	Version       int           `json:"version"`
	Sources       []Source      `json:"sources"`
	Favorites     []string      `json:"favorites"`
	Recent        []RecentEntry `json:"recent"`
	LastChannelID string        `json:"lastChannelId,omitempty"`
}

type StorageInfo struct {
	LogosCount int   `json:"logosCount"`
	LogosBytes int64 `json:"logosBytes"`
}

type Store struct {
	mu       sync.RWMutex
	dir      string
	st       PersistedState
	channels []Channel
	groups   []string
	byID     map[string]Channel
}

func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	st := &Store{
		dir:  dataDir,
		byID: make(map[string]Channel),
	}
	for _, sub := range []string{st.sourcesDir(), st.logosDir()} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return nil, err
		}
	}
	if err := st.load(); err != nil {
		return nil, err
	}
	st.rebuildMergedLocked()
	return st, nil
}

func (s *Store) statePath() string  { return filepath.Join(s.dir, "state.json") }
func (s *Store) sourcesDir() string { return filepath.Join(s.dir, "sources") }
func (s *Store) logosDir() string   { return filepath.Join(s.dir, "logos") }
func (s *Store) sourceCachePath(srcID string) string {
	return filepath.Join(s.sourcesDir(), filepath.Base(srcID)+".json")
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			s.st = PersistedState{Version: 1}
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
	s.st = ps
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

func (s *Store) saveStateLocked() error {
	return writeAtomicJSON(s.statePath(), s.st)
}

func (s *Store) rebuildMergedLocked() {
	perSource := make(map[string][]RawChannel, len(s.st.Sources))
	for _, src := range s.st.Sources {
		if !src.Enabled {
			continue
		}
		if b, err := os.ReadFile(s.sourceCachePath(src.ID)); err == nil {
			var raws []RawChannel
			if json.Unmarshal(b, &raws) == nil {
				perSource[src.ID] = raws
			}
		}
	}
	chs, grps := MergeChannels(s.st.Sources, perSource)
	favSet := make(map[string]bool, len(s.st.Favorites))
	for _, id := range s.st.Favorites {
		favSet[id] = true
	}
	byID := make(map[string]Channel, len(chs))
	for i := range chs {
		chs[i].Favorite = favSet[chs[i].ID]
		byID[chs[i].ID] = chs[i]
	}
	s.channels = chs
	s.groups = grps
	s.byID = byID
}

func (s *Store) Snapshot() (sources []Source, channels []Channel, groups []string, favorites []string, recent []RecentEntry, lastCh string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources = slices.Clone(s.st.Sources)
	channels = make([]Channel, len(s.channels))
	for i, ch := range s.channels {
		cp := ch
		cp.Streams = slices.Clone(ch.Streams)
		channels[i] = cp
	}
	groups = slices.Clone(s.groups)
	favorites = slices.Clone(s.st.Favorites)
	recent = slices.Clone(s.st.Recent)
	lastCh = s.st.LastChannelID
	return
}

func (s *Store) UpsertSource(src Source, raws []RawChannel) (Source, error) {
	now := time.Now().UnixMilli()
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i := range s.st.Sources {
		if s.st.Sources[i].ID == src.ID {
			idx = i
			break
		}
	}
	if src.AddedAt == 0 {
		if idx >= 0 && s.st.Sources[idx].AddedAt > 0 {
			src.AddedAt = s.st.Sources[idx].AddedAt
		} else {
			src.AddedAt = now
		}
	}
	src.LastFetchedAt = now
	src.ChannelCount = len(raws)
	src.LastError = ""

	if err := writeAtomicJSON(s.sourceCachePath(src.ID), raws); err != nil {
		return Source{}, err
	}
	if idx >= 0 {
		src.Enabled = s.st.Sources[idx].Enabled
		if s.st.Sources[idx].Name != "" && src.Name == "" {
			src.Name = s.st.Sources[idx].Name
		}
		s.st.Sources[idx] = src
	} else {
		s.st.Sources = append(s.st.Sources, src)
	}
	if err := s.saveStateLocked(); err != nil {
		return Source{}, err
	}
	s.rebuildMergedLocked()
	return src, nil
}

func (s *Store) MarkSourceError(sourceID, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.st.Sources {
		if s.st.Sources[i].ID == sourceID {
			s.st.Sources[i].LastError = errMsg
			s.st.Sources[i].LastFetchedAt = time.Now().UnixMilli()
			return s.saveStateLocked()
		}
	}
	return fmt.Errorf("source not found")
}

func (s *Store) ToggleSource(sourceID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i := range s.st.Sources {
		if s.st.Sources[i].ID == sourceID {
			s.st.Sources[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("source not found")
	}
	if err := s.saveStateLocked(); err != nil {
		return err
	}
	s.rebuildMergedLocked()
	return nil
}

func (s *Store) RemoveSource(sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Source, 0, len(s.st.Sources))
	for _, src := range s.st.Sources {
		if src.ID != sourceID {
			next = append(next, src)
		}
	}
	s.st.Sources = next
	_ = os.Remove(s.sourceCachePath(sourceID))
	if err := s.saveStateLocked(); err != nil {
		return err
	}
	s.rebuildMergedLocked()
	return nil
}

func (s *Store) ToggleFavorite(channelID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[channelID]; !ok {
		return false, fmt.Errorf("channel not found")
	}
	isFav := false
	idx := slices.Index(s.st.Favorites, channelID)
	if idx >= 0 {
		s.st.Favorites = append(s.st.Favorites[:idx], s.st.Favorites[idx+1:]...)
		isFav = false
	} else {
		s.st.Favorites = append([]string{channelID}, s.st.Favorites...)
		isFav = true
	}
	if err := s.saveStateLocked(); err != nil {
		return false, err
	}
	s.rebuildMergedLocked()
	return isFav, nil
}

func (s *Store) RecordWatch(channelID string, lineIdx int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[channelID]; !ok {
		return fmt.Errorf("channel not found")
	}
	now := time.Now().UnixMilli()
	next := []RecentEntry{{ChannelID: channelID, LineIdx: lineIdx, WatchedAt: now}}
	for _, r := range s.st.Recent {
		if r.ChannelID != channelID {
			next = append(next, r)
		}
		if len(next) >= maxRecentChannels {
			break
		}
	}
	s.st.Recent = next
	s.st.LastChannelID = channelID
	return s.saveStateLocked()
}

func (s *Store) GetChannel(channelID string) (Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ch, ok := s.byID[channelID]
	if !ok {
		return Channel{}, false
	}
	ch.Streams = slices.Clone(ch.Streams)
	return ch, true
}

func (s *Store) StorageUsage() StorageInfo {
	var info StorageInfo
	entries, err := os.ReadDir(s.logosDir())
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if fi, err := e.Info(); err == nil {
				info.LogosCount++
				info.LogosBytes += fi.Size()
			}
		}
	}
	return info
}

func (s *Store) ClearLogoCache() (StorageInfo, error) {
	_ = os.RemoveAll(s.logosDir())
	if err := os.MkdirAll(s.logosDir(), 0o755); err != nil {
		return StorageInfo{}, err
	}
	return s.StorageUsage(), nil
}

func (s *Store) epgPath() string { return filepath.Join(s.dir, "epg.json") }
