package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxEpisodesPerFeed = 200

type Chapter struct {
	StartSec float64 `json:"startSec"`
	Title    string  `json:"title"`
	URL      string  `json:"url,omitempty"`
	Img      string  `json:"img,omitempty"`
}

type TranscriptSegment struct {
	ID          int     `json:"id"`
	StartSec    float64 `json:"startSec"`
	EndSec      float64 `json:"endSec"`
	Text        string  `json:"text"`
	Translation string  `json:"translation,omitempty"`
}

type GuideTimelineItem struct {
	TimeSec float64 `json:"timeSec"`
	Title   string  `json:"title"`
	Summary string  `json:"summary,omitempty"`
}

type GuideQuote struct {
	TimeSec float64 `json:"timeSec"`
	Text    string  `json:"text"`
	Note    string  `json:"note,omitempty"`
}

type AIGuide struct {
	OneLiner    string              `json:"oneLiner"`
	Takeaways   []string            `json:"takeaways"`
	Timeline    []GuideTimelineItem `json:"timeline"`
	Quotes      []GuideQuote        `json:"quotes"`
	Model       string              `json:"model,omitempty"`
	GeneratedAt int64               `json:"generatedAt"`
}

type ChatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"createdAt"`
}

type Feed struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Author        string `json:"author,omitempty"`
	Description   string `json:"description,omitempty"`
	FeedURL       string `json:"feedUrl"`
	SiteURL       string `json:"siteUrl,omitempty"`
	ArtworkURL    string `json:"artworkUrl,omitempty"`
	CoverURL      string `json:"coverUrl,omitempty"`
	SourceType    string `json:"sourceType,omitempty"`
	Language      string `json:"language,omitempty"`
	Category      string `json:"category,omitempty"`
	SubscribedAt  int64  `json:"subscribedAt"`
	LastFetchedAt int64  `json:"lastFetchedAt"`
	EpisodeCount  int    `json:"episodeCount,omitempty"`
}

type Episode struct {
	ID              string    `json:"id"`
	FeedID          string    `json:"feedId"`
	FeedTitle       string    `json:"feedTitle,omitempty"`
	GUID            string    `json:"guid,omitempty"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary,omitempty"`
	Description     string    `json:"description,omitempty"`
	AudioURL        string    `json:"audioUrl"`
	AudioType       string    `json:"audioType,omitempty"`
	AudioBytes      int64     `json:"audioBytes,omitempty"`
	LinkURL         string    `json:"linkUrl,omitempty"`
	ImageURL        string    `json:"imageUrl,omitempty"`
	CoverURL        string    `json:"coverUrl,omitempty"`
	PlayURL         string    `json:"playUrl,omitempty"`
	DurationSec     float64   `json:"durationSec,omitempty"`
	PublishedAt     int64     `json:"publishedAt,omitempty"`
	TranscriptURL   string    `json:"transcriptUrl,omitempty"`
	TranscriptType  string    `json:"transcriptType,omitempty"`
	ChaptersURL     string    `json:"chaptersUrl,omitempty"`
	Chapters        []Chapter `json:"chapters,omitempty"`
	PositionSec     float64   `json:"positionSec,omitempty"`
	Completed       bool      `json:"completed,omitempty"`
	Favorited       bool      `json:"favorited,omitempty"`
	DownloadedFile  string    `json:"downloadedFile,omitempty"`
	DownloadedBytes int64     `json:"downloadedBytes,omitempty"`
	HasTranscript   bool      `json:"hasTranscript,omitempty"`
	HasTranslation  bool      `json:"hasTranslation,omitempty"`
	HasGuide        bool      `json:"hasGuide,omitempty"`
	PlayCount       int       `json:"playCount,omitempty"`
	LastPlayedAt    int64     `json:"lastPlayedAt,omitempty"`
}

type EpisodeDetail struct {
	EpisodeID        string              `json:"episodeId"`
	Segments         []TranscriptSegment `json:"segments,omitempty"`
	TranscriptSource string              `json:"transcriptSource,omitempty"`
	Chapters         []Chapter           `json:"chapters,omitempty"`
	Guide            *AIGuide            `json:"guide,omitempty"`
	Chat             []ChatMessage       `json:"chat,omitempty"`
}

type VocabEntry struct {
	ID             string  `json:"id"`
	Word           string  `json:"word"`
	Phonetic       string  `json:"phonetic,omitempty"`
	Meaning        string  `json:"meaning"`
	Sentence       string  `json:"sentence,omitempty"`
	Translation    string  `json:"translation,omitempty"`
	EpisodeID      string  `json:"episodeId,omitempty"`
	EpisodeTitle   string  `json:"episodeTitle,omitempty"`
	TimestampSec   float64 `json:"timestampSec,omitempty"`
	CreatedAt      int64   `json:"createdAt"`
	ReviewCount    int     `json:"reviewCount,omitempty"`
	LastReviewedAt int64   `json:"lastReviewedAt,omitempty"`
}

type ListeningStats struct {
	TotalSec  float64            `json:"totalSec"`
	HourlySec [24]float64        `json:"hourlySec"`
	DailySec  map[string]float64 `json:"dailySec"`
	ShowSec   map[string]float64 `json:"showSec"`
}

type StorageInfo struct {
	DownloadsCount int   `json:"downloadsCount"`
	DownloadsBytes int64 `json:"downloadsBytes"`
	CoversBytes    int64 `json:"coversBytes"`
	CacheBytes     int64 `json:"cacheBytes"`
}

type persistedState struct {
	Version  int            `json:"version"`
	Feeds    []Feed         `json:"feeds"`
	Episodes []Episode      `json:"episodes"`
	Vocab    []VocabEntry   `json:"vocab"`
	Stats    ListeningStats `json:"stats"`
	Queue    []string       `json:"queue"`
}

type Store struct {
	mu      sync.RWMutex
	baseDir string
	state   persistedState
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func makeFeedID(feedURL string) string {
	return "fd-" + shortHash(strings.TrimSpace(feedURL))
}

func makeEpisodeID(feedID, guidOrAudio string) string {
	return "ep-" + shortHash(feedID+"\x00"+strings.TrimSpace(guidOrAudio))
}

func makeVocabID(word, episodeID string) string {
	return "vb-" + shortHash(strings.ToLower(strings.TrimSpace(word))+"\x00"+episodeID)
}

func NewStore(dataDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("empty data directory")
	}
	for _, sub := range []string{"", "cache", "downloads", "covers"} {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	s := &Store{
		baseDir: dataDir,
		state: persistedState{
			Version: 1,
			Stats: ListeningStats{
				DailySec: map[string]float64{},
				ShowSec:  map[string]float64{},
			},
		},
	}
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) statePath() string     { return filepath.Join(s.baseDir, "state.json") }
func (s *Store) cacheDir() string      { return filepath.Join(s.baseDir, "cache") }
func (s *Store) downloadsDir() string  { return filepath.Join(s.baseDir, "downloads") }
func (s *Store) coversDir() string     { return filepath.Join(s.baseDir, "covers") }

func (s *Store) cachePath(episodeID string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", "..", "_").Replace(episodeID)
	return filepath.Join(s.cacheDir(), safe+".json")
}

func (s *Store) DownloadPath(fileName string) string {
	if fileName == "" {
		return ""
	}
	return filepath.Join(s.downloadsDir(), filepath.Base(fileName))
}

func (s *Store) loadLocked() error {
	b, err := os.ReadFile(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var st persistedState
	if err := json.Unmarshal(b, &st); err != nil {
		return err
	}
	if st.Version == 0 {
		st.Version = 1
	}
	if st.Stats.DailySec == nil {
		st.Stats.DailySec = map[string]float64{}
	}
	if st.Stats.ShowSec == nil {
		st.Stats.ShowSec = map[string]float64{}
	}
	s.state = st
	return nil
}

func writeAtomicJSON(dst string, v any) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func (s *Store) saveLocked() error {
	return writeAtomicJSON(s.statePath(), s.state)
}

func (s *Store) Snapshot() ([]Feed, []Episode, []VocabEntry, ListeningStats, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	feeds := slices.Clone(s.state.Feeds)
	counts := map[string]int{}
	for _, ep := range s.state.Episodes {
		counts[ep.FeedID]++
	}
	for i := range feeds {
		feeds[i].EpisodeCount = counts[feeds[i].ID]
	}

	eps := make([]Episode, len(s.state.Episodes))
	copy(eps, s.state.Episodes)
	for i := range eps {
		eps[i].Chapters = slices.Clone(eps[i].Chapters)
	}
	vocab := slices.Clone(s.state.Vocab)
	stats := s.state.Stats
	stats.DailySec = mapsCloneFloat(stats.DailySec)
	stats.ShowSec = mapsCloneFloat(stats.ShowSec)
	queue := slices.Clone(s.state.Queue)
	return feeds, eps, vocab, stats, queue
}

func mapsCloneFloat(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (s *Store) GetFeed(feedID string) (Feed, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.state.Feeds {
		if f.ID == feedID {
			return f, true
		}
	}
	return Feed{}, false
}

func (s *Store) GetEpisode(epID string) (Episode, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ep := range s.state.Episodes {
		if ep.ID == epID {
			ep.Chapters = slices.Clone(ep.Chapters)
			return ep, true
		}
	}
	return Episode{}, false
}

func (s *Store) UpsertFeedAndEpisodes(feed Feed, incoming []Episode) (Feed, []Episode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if feed.ID == "" {
		feed.ID = makeFeedID(feed.FeedURL)
	}
	if feed.SubscribedAt == 0 {
		feed.SubscribedAt = now
	}
	feed.LastFetchedAt = now

	feedIdx := -1
	for i := range s.state.Feeds {
		if s.state.Feeds[i].ID == feed.ID || (feed.FeedURL != "" && s.state.Feeds[i].FeedURL == feed.FeedURL) {
			feedIdx = i
			feed.ID = s.state.Feeds[i].ID
			feed.SubscribedAt = s.state.Feeds[i].SubscribedAt
			if feed.Category == "" {
				feed.Category = s.state.Feeds[i].Category
			}
			if feed.SourceType == "" {
				feed.SourceType = s.state.Feeds[i].SourceType
			}
			if feed.ArtworkURL == "" {
				feed.ArtworkURL = s.state.Feeds[i].ArtworkURL
			}
			break
		}
	}
	if feedIdx >= 0 {
		s.state.Feeds[feedIdx] = feed
	} else {
		s.state.Feeds = append(s.state.Feeds, feed)
	}

	existingByID := make(map[string]Episode)
	otherEpisodes := make([]Episode, 0, len(s.state.Episodes))
	for _, ep := range s.state.Episodes {
		if ep.FeedID == feed.ID {
			existingByID[ep.ID] = ep
		} else {
			otherEpisodes = append(otherEpisodes, ep)
		}
	}

	mergedByID := make(map[string]Episode, len(existingByID)+len(incoming))
	for id, ep := range existingByID {
		mergedByID[id] = ep
	}
	for _, ep := range incoming {
		ep.FeedID = feed.ID
		if ep.FeedTitle == "" {
			ep.FeedTitle = feed.Title
		}
		if ep.ImageURL == "" {
			ep.ImageURL = feed.ArtworkURL
		}
		if ep.ID == "" {
			key := ep.GUID
			if key == "" {
				key = ep.AudioURL
			}
			if key == "" {
				key = ep.Title
			}
			ep.ID = makeEpisodeID(feed.ID, key)
		}
		if prev, ok := existingByID[ep.ID]; ok {
			ep.PositionSec = prev.PositionSec
			ep.Completed = prev.Completed
			ep.Favorited = prev.Favorited
			ep.DownloadedFile = prev.DownloadedFile
			ep.DownloadedBytes = prev.DownloadedBytes
			ep.HasTranscript = prev.HasTranscript
			ep.HasTranslation = prev.HasTranslation
			ep.HasGuide = prev.HasGuide
			ep.PlayCount = prev.PlayCount
			ep.LastPlayedAt = prev.LastPlayedAt
			if len(ep.Chapters) == 0 && len(prev.Chapters) > 0 {
				ep.Chapters = prev.Chapters
			}
			if ep.DurationSec <= 0 && prev.DurationSec > 0 {
				ep.DurationSec = prev.DurationSec
			}
		}
		mergedByID[ep.ID] = ep
	}

	feedEpisodes := make([]Episode, 0, len(mergedByID))
	for _, ep := range mergedByID {
		feedEpisodes = append(feedEpisodes, ep)
	}
	sort.SliceStable(feedEpisodes, func(i, j int) bool {
		return feedEpisodes[i].PublishedAt > feedEpisodes[j].PublishedAt
	})

	if len(feedEpisodes) > maxEpisodesPerFeed {
		kept := make([]Episode, 0, maxEpisodesPerFeed+16)
		for i, ep := range feedEpisodes {
			if i < maxEpisodesPerFeed || ep.Favorited || ep.DownloadedFile != "" || ep.PositionSec > 0 || ep.HasTranscript || ep.HasGuide {
				kept = append(kept, ep)
			}
		}
		feedEpisodes = kept
	}

	s.state.Episodes = append(otherEpisodes, feedEpisodes...)
	sort.SliceStable(s.state.Episodes, func(i, j int) bool {
		return s.state.Episodes[i].PublishedAt > s.state.Episodes[j].PublishedAt
	})
	feed.EpisodeCount = len(feedEpisodes)
	if err := s.saveLocked(); err != nil {
		return Feed{}, nil, err
	}
	return feed, feedEpisodes, nil
}

func (s *Store) RemoveFeed(feedID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	feeds := s.state.Feeds[:0]
	for _, f := range s.state.Feeds {
		if f.ID != feedID {
			feeds = append(feeds, f)
		}
	}
	s.state.Feeds = feeds

	removedIDs := map[string]bool{}
	eps := s.state.Episodes[:0]
	for _, ep := range s.state.Episodes {
		if ep.FeedID == feedID {
			removedIDs[ep.ID] = true
			if ep.DownloadedFile != "" {
				_ = os.Remove(s.DownloadPath(ep.DownloadedFile))
			}
			_ = os.Remove(s.cachePath(ep.ID))
		} else {
			eps = append(eps, ep)
		}
	}
	s.state.Episodes = eps

	queue := s.state.Queue[:0]
	for _, qid := range s.state.Queue {
		if !removedIDs[qid] {
			queue = append(queue, qid)
		}
	}
	s.state.Queue = queue
	return s.saveLocked()
}

func (s *Store) SaveProgress(epID string, positionSec, durationSec float64, completed bool, playedDeltaSec float64) (Episode, ListeningStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i := range s.state.Episodes {
		if s.state.Episodes[i].ID == epID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Episode{}, ListeningStats{}, fmt.Errorf("episode %q not found", epID)
	}

	ep := &s.state.Episodes[idx]
	if positionSec >= 0 {
		ep.PositionSec = positionSec
	}
	if durationSec > 0 && (ep.DurationSec <= 0 || durationSec > ep.DurationSec) {
		ep.DurationSec = durationSec
	}
	if completed {
		ep.Completed = true
	}
	now := time.Now()
	if playedDeltaSec > 0 && playedDeltaSec <= 120 {
		if ep.PlayCount == 0 {
			ep.PlayCount = 1
		}
		ep.LastPlayedAt = now.UnixMilli()
		s.state.Stats.TotalSec += playedDeltaSec
		hr := now.Hour()
		if hr >= 0 && hr < 24 {
			s.state.Stats.HourlySec[hr] += playedDeltaSec
		}
		dayKey := now.Format("2006-01-02")
		if s.state.Stats.DailySec == nil {
			s.state.Stats.DailySec = map[string]float64{}
		}
		s.state.Stats.DailySec[dayKey] += playedDeltaSec
		if s.state.Stats.ShowSec == nil {
			s.state.Stats.ShowSec = map[string]float64{}
		}
		showName := ep.FeedTitle
		if showName == "" {
			showName = ep.FeedID
		}
		s.state.Stats.ShowSec[showName] += playedDeltaSec
	} else if positionSec > 0 && ep.LastPlayedAt == 0 {
		ep.LastPlayedAt = now.UnixMilli()
	}

	if err := s.saveLocked(); err != nil {
		return Episode{}, ListeningStats{}, err
	}
	stats := s.state.Stats
	stats.DailySec = mapsCloneFloat(stats.DailySec)
	stats.ShowSec = mapsCloneFloat(stats.ShowSec)
	return *ep, stats, nil
}

func (s *Store) ToggleFavorite(epID string) (Episode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Episodes {
		if s.state.Episodes[i].ID == epID {
			s.state.Episodes[i].Favorited = !s.state.Episodes[i].Favorited
			if err := s.saveLocked(); err != nil {
				return Episode{}, err
			}
			return s.state.Episodes[i], nil
		}
	}
	return Episode{}, fmt.Errorf("episode %q not found", epID)
}

func (s *Store) SetDownload(epID, fileName string, sizeBytes int64) (Episode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Episodes {
		if s.state.Episodes[i].ID == epID {
			s.state.Episodes[i].DownloadedFile = fileName
			s.state.Episodes[i].DownloadedBytes = sizeBytes
			if err := s.saveLocked(); err != nil {
				return Episode{}, err
			}
			return s.state.Episodes[i], nil
		}
	}
	return Episode{}, fmt.Errorf("episode %q not found", epID)
}

func (s *Store) SetQueue(queue []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	known := make(map[string]bool, len(s.state.Episodes))
	for _, ep := range s.state.Episodes {
		known[ep.ID] = true
	}
	seen := make(map[string]bool, len(queue))
	clean := make([]string, 0, len(queue))
	for _, id := range queue {
		id = strings.TrimSpace(id)
		if id != "" && known[id] && !seen[id] {
			seen[id] = true
			clean = append(clean, id)
		}
	}
	s.state.Queue = clean
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return slices.Clone(s.state.Queue), nil
}

func (s *Store) UpdateEpisodeDetail(epID string, fn func(ep *Episode, detail *EpisodeDetail)) (EpisodeDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	epIdx := -1
	for i := range s.state.Episodes {
		if s.state.Episodes[i].ID == epID {
			epIdx = i
			break
		}
	}
	if epIdx < 0 {
		return EpisodeDetail{}, fmt.Errorf("episode %q not found", epID)
	}

	det, err := s.loadDetailLocked(epID)
	if err != nil {
		return EpisodeDetail{}, err
	}
	fn(&s.state.Episodes[epIdx], &det)

	hasTr := len(det.Segments) > 0
	hasTrans := false
	for _, seg := range det.Segments {
		if strings.TrimSpace(seg.Translation) != "" {
			hasTrans = true
			break
		}
	}
	s.state.Episodes[epIdx].HasTranscript = hasTr
	s.state.Episodes[epIdx].HasTranslation = hasTrans
	s.state.Episodes[epIdx].HasGuide = det.Guide != nil
	if len(det.Chapters) > 0 {
		s.state.Episodes[epIdx].Chapters = det.Chapters
	}

	if err := writeAtomicJSON(s.cachePath(epID), det); err != nil {
		return EpisodeDetail{}, err
	}
	if err := s.saveLocked(); err != nil {
		return EpisodeDetail{}, err
	}
	return det, nil
}

func (s *Store) LoadDetail(epID string) (EpisodeDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadDetailLocked(epID)
}

func (s *Store) loadDetailLocked(epID string) (EpisodeDetail, error) {
	det := EpisodeDetail{EpisodeID: epID}
	b, err := os.ReadFile(s.cachePath(epID))
	if errors.Is(err, os.ErrNotExist) {
		return det, nil
	}
	if err != nil {
		return det, err
	}
	if err := json.Unmarshal(b, &det); err != nil {
		return EpisodeDetail{EpisodeID: epID}, nil
	}
	det.EpisodeID = epID
	return det, nil
}

func (s *Store) AddVocab(v VocabEntry) ([]VocabEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v.Word = strings.TrimSpace(v.Word)
	if v.Word == "" {
		return nil, fmt.Errorf("word must not be empty")
	}
	if v.ID == "" {
		v.ID = makeVocabID(v.Word, v.EpisodeID)
	}
	if v.CreatedAt == 0 {
		v.CreatedAt = time.Now().UnixMilli()
	}

	replaced := false
	for i := range s.state.Vocab {
		if s.state.Vocab[i].ID == v.ID {
			v.CreatedAt = s.state.Vocab[i].CreatedAt
			v.ReviewCount = s.state.Vocab[i].ReviewCount
			v.LastReviewedAt = s.state.Vocab[i].LastReviewedAt
			s.state.Vocab[i] = v
			replaced = true
			break
		}
	}
	if !replaced {
		s.state.Vocab = append([]VocabEntry{v}, s.state.Vocab...)
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return slices.Clone(s.state.Vocab), nil
}

func (s *Store) DeleteVocab(id string) ([]VocabEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.state.Vocab[:0]
	for _, v := range s.state.Vocab {
		if v.ID != id {
			out = append(out, v)
		}
	}
	s.state.Vocab = out
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return slices.Clone(s.state.Vocab), nil
}

func (s *Store) ReviewVocab(id string) ([]VocabEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Vocab {
		if s.state.Vocab[i].ID == id {
			s.state.Vocab[i].ReviewCount++
			s.state.Vocab[i].LastReviewedAt = time.Now().UnixMilli()
			break
		}
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return slices.Clone(s.state.Vocab), nil
}

func (s *Store) StorageUsage() StorageInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var info StorageInfo
	info.DownloadsCount, info.DownloadsBytes = dirStats(s.downloadsDir())
	_, info.CoversBytes = dirStats(s.coversDir())
	_, info.CacheBytes = dirStats(s.cacheDir())
	return info
}

func (s *Store) ClearDownloadsAndCovers() (StorageInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, dir := range []string{s.downloadsDir(), s.coversDir()} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
	for i := range s.state.Episodes {
		s.state.Episodes[i].DownloadedFile = ""
		s.state.Episodes[i].DownloadedBytes = 0
	}
	if err := s.saveLocked(); err != nil {
		return StorageInfo{}, err
	}
	var info StorageInfo
	_, info.CacheBytes = dirStats(s.cacheDir())
	return info, nil
}

func dirStats(dir string) (int, int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	count := 0
	var total int64
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			count++
			total += fi.Size()
		}
	}
	return count, total
}
