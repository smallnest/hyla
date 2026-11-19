package hongguo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"

	"hyla/internal/proto"
	"hyla/locales"
)

const (
	feedCacheTTL      = 15 * time.Minute
	catalogInitTarget = 24
	catalogMoreTarget = 18
	catalogMaxHops    = 3
	mediaKeepFiles    = 40

	ChannelAll       = "all"
	ChannelRecommend = "推荐"
	ChannelLive      = "真人剧"
	ChannelComic     = "漫剧"
	ChannelRank      = "rank"
)

var (
	fallbackSearchTerms = []string{"总裁", "重生", "逆袭"}
	safeIDRegex         = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)
	safeDefRegex        = regexp.MustCompile(`[^0-9A-Za-z_-]+`)
)

// Drama summarizes a short drama series for lists and headers.
type Drama struct {
	ID           string
	Title        string
	Description  string
	CoverURL     string
	EpisodeCount int
}

// Episode describes a single episode of a drama.
type Episode struct {
	ID       string
	SeriesID string
	Number   int
	Title    string
	CoverURL string
	Vertical bool
	Playable bool
	Duration time.Duration
}

// SeriesDetail bundles drama metadata with its episodes.
type SeriesDetail struct {
	Series   Drama
	Episodes []Episode
}

// CatalogPage holds one paginated batch of dramas from a Hongguo browse channel.
type CatalogPage struct {
	Items   []Drama
	Cursor  string
	HasMore bool
}

// SearchResult holds one page of drama search results.
type SearchResult struct {
	Items   []Drama
	Cursor  string
	HasMore bool
}

// PlayMedia describes a resolved playable episode stream or decrypted local MP4.
type PlayMedia struct {
	EpisodeID   string
	LocalPath   string
	StreamURL   string
	Definition  string
	Definitions []string
	DurationSec int
	Protected   bool
	ExpiresAt   int64
}

// API abstracts the openhongguo SDK client for offline testing.
type API interface {
	Discover(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error)
	Search(ctx context.Context, query string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error)
	ShortFilter(ctx context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error)
	ShortRank(ctx context.Context, q sdk.ShortRankQuery) (*model.ShortRanking, error)
	GetSeries(ctx context.Context, seriesID string) (*model.Series, error)
	GetSeriesBatch(ctx context.Context, seriesIDs []string) ([]model.SeriesDetail, error)
	ListEpisodes(ctx context.Context, seriesID string, opts sdk.EpisodeOptions) (*sdk.Page[model.Episode], error)
	ResolvePlayback(ctx context.Context, episodeID string, opts sdk.PlaybackOptions) (*model.Playback, error)
	ResetDevice(ctx context.Context) error
}

type cachedCatalog struct {
	page CatalogPage
	at   time.Time
}

type mediaJob struct {
	done chan struct{}
	err  error
}

// Service manages openhongguo catalog, playback resolution, decryption, covers, and watch state.
type Service struct {
	dir    string
	covers *CoverCache

	// Injectable hooks for tests; nil uses production defaults.
	DeriveKey func(*model.Playback) (model.ContentKey, error)
	Decrypt   func(ctx context.Context, keyHex, srcURL, backupURL, outPath string) error

	mu           sync.Mutex
	api          API
	catalogCache map[string]cachedCatalog
	dramaMeta    map[string]Drama
	searchCovers map[string]string
	mediaJobs    map[string]*mediaJob

	storeLoaded bool
	history     []HistoryEntry
	prefs       Prefs
}

// NewService creates a lazy Hongguo service rooted at dataDir (or proto.Dir()/hongguo when empty).
// No network requests or profile creation occur during construction.
func NewService(dataDir string) *Service {
	if dataDir == "" {
		dataDir = filepath.Join(proto.Dir(), "hongguo")
	}
	return &Service{
		dir:          dataDir,
		covers:       NewCoverCache(filepath.Join(dataDir, "covers")),
		catalogCache: make(map[string]cachedCatalog),
		dramaMeta:    make(map[string]Drama),
		searchCovers: make(map[string]string),
		mediaJobs:    make(map[string]*mediaJob),
	}
}

// Covers returns the poster bitmap cache.
func (s *Service) Covers() *CoverCache { return s.covers }

// SetAPI replaces the underlying SDK API implementation (primarily for tests).
func (s *Service) SetAPI(api API) {
	s.mu.Lock()
	s.api = api
	s.mu.Unlock()
}

func (s *Service) client() (API, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.api != nil {
		return s.api, nil
	}
	profilePath := filepath.Join(s.dir, "profile.json")
	cl, err := adapters.NewClient(profilePath, sdk.Options{
		RetryAttempts: 2,
	})
	if err != nil {
		return nil, err
	}
	s.api = cl
	return cl, nil
}

// Feed returns the first page of the recommended drama feed, cached for 15 minutes unless force is true.
func (s *Service) Feed(ctx context.Context, force bool) ([]Drama, error) {
	page, err := s.Catalog(ctx, ChannelRecommend, "", nil, force)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// Catalog fetches one paginated batch for a browse channel (ChannelAll, ChannelRecommend,
// ChannelLive, ChannelComic, or ChannelRank), supporting unbounded cursor pagination,
// duplicate skipping via seen, cover backfill, and first-page caching per channel.
func (s *Service) Catalog(ctx context.Context, channel, cursor string, seen map[string]struct{}, force bool) (CatalogPage, error) {
	if channel == "" {
		channel = ChannelAll
	}
	if cursor == "" && !force {
		s.mu.Lock()
		if cached, ok := s.catalogCache[channel]; ok && len(cached.page.Items) > 0 && time.Since(cached.at) < feedCacheTTL {
			out := CatalogPage{
				Items:   append([]Drama(nil), cached.page.Items...),
				Cursor:  cached.page.Cursor,
				HasMore: cached.page.HasMore,
			}
			s.mu.Unlock()
			return out, nil
		}
		s.mu.Unlock()
	}

	api, err := s.client()
	if err != nil {
		return CatalogPage{}, err
	}

	skip := make(map[string]bool, len(seen))
	for id := range seen {
		if id != "" {
			skip[id] = true
		}
	}
	target := catalogInitTarget
	if cursor != "" {
		target = catalogMoreTarget
	}

	var (
		rows    []Drama
		nextCur string
		hasMore bool
	)

	switch channel {
	case ChannelAll:
		if after, ok := strings.CutPrefix(cursor, "discover:"); ok {
			rows, nextCur, hasMore, err = s.discoverBatch(ctx, api, ChannelRecommend, after, skip, target)
			if nextCur != "" {
				nextCur = "discover:" + nextCur
			}
		} else {
			rows, nextCur, hasMore, err = s.shortFilterBatch(ctx, api, cursor, skip, target)
			if cursor == "" && (err != nil || len(rows) == 0) {
				dRows, dCur, dMore, dErr := s.discoverBatch(ctx, api, ChannelRecommend, "", skip, target)
				if dErr == nil && len(dRows) > 0 {
					rows, hasMore, err = dRows, dMore, nil
					if dCur != "" {
						nextCur = "discover:" + dCur
					} else {
						nextCur = ""
					}
				} else if err == nil {
					err = dErr
				}
			}
			if cursor == "" && len(rows) == 0 {
				fbRows, fbErr := s.searchFallback(ctx, api, skip)
				if len(fbRows) > 0 {
					rows, nextCur, hasMore, err = fbRows, "", false, nil
				} else if err == nil {
					err = fbErr
				}
			}
		}
	case ChannelRank:
		rows, nextCur, hasMore, err = s.shortRankBatch(ctx, api, cursor, skip, target)
	case ChannelRecommend, ChannelLive, ChannelComic:
		rows, nextCur, hasMore, err = s.discoverBatch(ctx, api, channel, cursor, skip, target)
		if cursor == "" && channel == ChannelRecommend && len(rows) == 0 {
			fbRows, fbErr := s.searchFallback(ctx, api, skip)
			if len(fbRows) > 0 {
				rows, nextCur, hasMore, err = fbRows, "", false, nil
			} else if err == nil {
				err = fbErr
			}
		}
	default:
		return CatalogPage{}, fmt.Errorf("unknown channel %q", channel)
	}

	if len(rows) == 0 {
		if err != nil {
			return CatalogPage{}, err
		}
		if cursor == "" {
			return CatalogPage{}, sdk.ErrAnonymousLimited
		}
		return CatalogPage{}, nil
	}

	s.backfillCovers(ctx, api, rows)
	s.recordDramas(rows)

	page := CatalogPage{
		Items:   rows,
		Cursor:  nextCur,
		HasMore: hasMore && nextCur != "",
	}
	if cursor == "" {
		s.mu.Lock()
		s.catalogCache[channel] = cachedCatalog{
			page: CatalogPage{
				Items:   append([]Drama(nil), page.Items...),
				Cursor:  page.Cursor,
				HasMore: page.HasMore,
			},
			at: time.Now(),
		}
		s.mu.Unlock()
	}
	return page, nil
}

type rawCatalogBatch struct {
	items   []Drama
	cursor  string
	hasMore bool
}

func callWithDeviceReset[T any](ctx context.Context, api API, fn func() (T, error)) (T, error) {
	res, err := fn()
	if errors.Is(err, sdk.ErrAnonymousLimited) || errors.Is(err, sdk.ErrDeviceRejected) {
		if rerr := api.ResetDevice(ctx); rerr == nil {
			res, err = fn()
		}
	}
	return res, err
}

func collectCatalogBatch(ctx context.Context, api API, cursor string, skip map[string]bool, target int, fetch func(cur string) (rawCatalogBatch, error)) ([]Drama, string, bool, error) {
	var rows []Drama
	cur := cursor
	for range catalogMaxHops {
		batch, err := callWithDeviceReset(ctx, api, func() (rawCatalogBatch, error) {
			return fetch(cur)
		})
		if err != nil {
			if len(rows) > 0 {
				return rows, "", false, nil
			}
			return nil, "", false, err
		}
		for _, d := range batch.items {
			if d.ID == "" || skip[d.ID] {
				continue
			}
			skip[d.ID] = true
			rows = append(rows, d)
		}
		cur = batch.cursor
		if !batch.hasMore || cur == "" {
			return rows, "", false, nil
		}
		if len(rows) >= target {
			return rows, cur, true, nil
		}
	}
	return rows, cur, cur != "", nil
}

func dramasFromSummaries(items []model.SeriesSummary) []Drama {
	out := make([]Drama, 0, len(items))
	for _, it := range items {
		if it.ID == "" {
			continue
		}
		out = append(out, Drama{
			ID:           it.ID,
			Title:        it.Title,
			Description:  it.Description,
			CoverURL:     it.CoverURL,
			EpisodeCount: it.EpisodeCount,
		})
	}
	return out
}

func (s *Service) shortFilterBatch(ctx context.Context, api API, cursor string, skip map[string]bool, target int) ([]Drama, string, bool, error) {
	return collectCatalogBatch(ctx, api, cursor, skip, target, func(cur string) (rawCatalogBatch, error) {
		p, err := api.ShortFilter(ctx, sdk.ShortFilterQuery{Cursor: cur})
		if err != nil {
			return rawCatalogBatch{}, err
		}
		if p == nil {
			return rawCatalogBatch{}, nil
		}
		return rawCatalogBatch{
			items:   dramasFromSummaries(p.Items),
			cursor:  p.Cursor,
			hasMore: p.HasMore,
		}, nil
	})
}

func (s *Service) discoverBatch(ctx context.Context, api API, channel, cursor string, skip map[string]bool, target int) ([]Drama, string, bool, error) {
	return collectCatalogBatch(ctx, api, cursor, skip, target, func(cur string) (rawCatalogBatch, error) {
		p, err := api.Discover(ctx, sdk.DiscoverOptions{Channel: channel, Cursor: cur})
		if err != nil {
			return rawCatalogBatch{}, err
		}
		if p == nil {
			return rawCatalogBatch{}, nil
		}
		return rawCatalogBatch{
			items:   dramasFromSummaries(p.Items),
			cursor:  p.Cursor,
			hasMore: p.HasMore,
		}, nil
	})
}

func (s *Service) shortRankBatch(ctx context.Context, api API, cursor string, skip map[string]bool, target int) ([]Drama, string, bool, error) {
	return collectCatalogBatch(ctx, api, cursor, skip, target, func(cur string) (rawCatalogBatch, error) {
		p, err := api.ShortRank(ctx, sdk.ShortRankQuery{Content: "all", Cursor: cur})
		if err != nil {
			return rawCatalogBatch{}, err
		}
		if p == nil {
			return rawCatalogBatch{}, nil
		}
		items := make([]Drama, 0, len(p.Items))
		for _, it := range p.Items {
			if it.SeriesID == "" {
				continue
			}
			items = append(items, Drama{
				ID:           it.SeriesID,
				Title:        it.Title,
				Description:  it.Score,
				CoverURL:     it.CoverURL,
				EpisodeCount: it.EpisodeCount,
			})
		}
		return rawCatalogBatch{
			items:   items,
			cursor:  p.Cursor,
			hasMore: p.HasMore,
		}, nil
	})
}

func (s *Service) searchFallback(ctx context.Context, api API, skip map[string]bool) ([]Drama, error) {
	var rows []Drama
	var firstErr error
	for _, term := range fallbackSearchTerms {
		page, err := callWithDeviceReset(ctx, api, func() (*sdk.Page[model.SeriesSummary], error) {
			return api.Search(ctx, term, sdk.SearchOptions{})
		})
		if err != nil {
			if !errors.Is(err, sdk.ErrAnonymousLimited) && len(rows) == 0 && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if page == nil {
			continue
		}
		for _, it := range dramasFromSummaries(page.Items) {
			if skip[it.ID] {
				continue
			}
			skip[it.ID] = true
			rows = append(rows, it)
		}
	}
	if len(rows) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return rows, nil
}

func (s *Service) recordDramas(rows []Drama) {
	s.mu.Lock()
	for _, d := range rows {
		if d.ID == "" {
			continue
		}
		cur := s.dramaMeta[d.ID]
		cur.ID = d.ID
		if d.Title != "" {
			cur.Title = d.Title
		}
		if d.Description != "" {
			cur.Description = d.Description
		}
		if d.CoverURL != "" {
			cur.CoverURL = d.CoverURL
			s.searchCovers[d.ID] = d.CoverURL
		}
		if d.EpisodeCount > 0 {
			cur.EpisodeCount = d.EpisodeCount
		}
		s.dramaMeta[d.ID] = cur
	}
	s.mu.Unlock()
}

func (s *Service) backfillCovers(ctx context.Context, api API, rows []Drama) {
	if len(rows) == 0 {
		return
	}
	var missing []string
	seenMissing := make(map[string]bool)

	s.mu.Lock()
	for i := range rows {
		id := rows[i].ID
		if id == "" {
			continue
		}
		meta := s.dramaMeta[id]
		if rows[i].CoverURL == "" {
			if u := s.searchCovers[id]; u != "" {
				rows[i].CoverURL = u
			} else if meta.CoverURL != "" {
				rows[i].CoverURL = meta.CoverURL
			}
		}
		if rows[i].EpisodeCount == 0 && meta.EpisodeCount > 0 {
			rows[i].EpisodeCount = meta.EpisodeCount
		}
		if rows[i].CoverURL == "" && !seenMissing[id] && len(missing) < 20 {
			seenMissing[id] = true
			missing = append(missing, id)
		}
	}
	s.mu.Unlock()

	if len(missing) == 0 {
		return
	}
	details, err := api.GetSeriesBatch(ctx, missing)
	if err != nil {
		return
	}
	found := make(map[string]Drama, len(details))
	for _, d := range details {
		if d.Series == nil || d.Series.ID == "" {
			continue
		}
		found[d.Series.ID] = Drama{
			ID:           d.Series.ID,
			Title:        d.Series.Title,
			Description:  d.Series.Description,
			CoverURL:     d.Series.CoverURL,
			EpisodeCount: d.Series.EpisodeCount,
		}
	}
	for i := range rows {
		d, ok := found[rows[i].ID]
		if !ok {
			continue
		}
		if rows[i].CoverURL == "" && d.CoverURL != "" {
			rows[i].CoverURL = d.CoverURL
		}
		if rows[i].EpisodeCount == 0 && d.EpisodeCount > 0 {
			rows[i].EpisodeCount = d.EpisodeCount
		}
		if rows[i].Title == "" && d.Title != "" {
			rows[i].Title = d.Title
		}
		if rows[i].Description == "" && d.Description != "" {
			rows[i].Description = d.Description
		}
	}
}

// Search searches short dramas by keyword and cursor, backfilling missing poster URLs.
func (s *Service) Search(ctx context.Context, query, cursor string) (SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResult{}, nil
	}
	api, err := s.client()
	if err != nil {
		return SearchResult{}, err
	}

	page, err := callWithDeviceReset(ctx, api, func() (*sdk.Page[model.SeriesSummary], error) {
		return api.Search(ctx, query, sdk.SearchOptions{Cursor: cursor})
	})
	if err != nil {
		return SearchResult{}, err
	}
	if page == nil {
		return SearchResult{}, nil
	}

	seen := make(map[string]bool, len(page.Items))
	rows := make([]Drama, 0, len(page.Items))
	for _, it := range dramasFromSummaries(page.Items) {
		if seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		rows = append(rows, it)
	}

	s.backfillCovers(ctx, api, rows)
	s.recordDramas(rows)

	return SearchResult{
		Items:   rows,
		Cursor:  page.Cursor,
		HasMore: page.HasMore,
	}, nil
}

// Series fetches drama metadata and episode list for seriesID.
func (s *Service) Series(ctx context.Context, seriesID string) (SeriesDetail, error) {
	api, err := s.client()
	if err != nil {
		return SeriesDetail{}, err
	}

	s.mu.Lock()
	drama := s.dramaMeta[seriesID]
	if drama.CoverURL == "" {
		drama.CoverURL = s.searchCovers[seriesID]
	}
	s.mu.Unlock()
	if drama.ID == "" {
		drama.ID = seriesID
	}

	if info, serr := api.GetSeries(ctx, seriesID); serr == nil && info != nil {
		if info.Title != "" {
			drama.Title = info.Title
		}
		if info.Description != "" {
			drama.Description = info.Description
		}
		if info.CoverURL != "" {
			drama.CoverURL = info.CoverURL
		}
		if info.EpisodeCount > 0 {
			drama.EpisodeCount = info.EpisodeCount
		}
	}

	epPage, err := api.ListEpisodes(ctx, seriesID, sdk.EpisodeOptions{})
	if errors.Is(err, sdk.ErrAnonymousLimited) || errors.Is(err, sdk.ErrDeviceRejected) {
		if rerr := api.ResetDevice(ctx); rerr == nil {
			epPage, err = api.ListEpisodes(ctx, seriesID, sdk.EpisodeOptions{})
		}
	}
	if err != nil {
		return SeriesDetail{}, err
	}

	var eps []Episode
	if epPage != nil {
		eps = make([]Episode, 0, len(epPage.Items))
		for _, ep := range epPage.Items {
			eps = append(eps, Episode{
				ID:       ep.ID,
				SeriesID: seriesID,
				Number:   ep.Number,
				Title:    ep.Title,
				CoverURL: ep.CoverURL,
				Vertical: ep.Vertical,
				Playable: ep.Playable,
				Duration: ep.Duration,
			})
		}
	}
	if drama.EpisodeCount == 0 && len(eps) > 0 {
		drama.EpisodeCount = len(eps)
	}
	if drama.CoverURL != "" {
		s.mu.Lock()
		s.searchCovers[seriesID] = drama.CoverURL
		s.mu.Unlock()
	}

	return SeriesDetail{Series: drama, Episodes: eps}, nil
}

// ResolvePlay resolves an episode's playback streams and decrypts protected MP4s locally.
func (s *Service) ResolvePlay(ctx context.Context, episodeID, preferredDef string, onStage func(stage string)) (PlayMedia, error) {
	if episodeID == "" {
		return PlayMedia{}, errors.New("missing episode id")
	}
	api, err := s.client()
	if err != nil {
		return PlayMedia{}, err
	}

	pb, err := s.resolvePlaybackWithReset(ctx, api, episodeID)
	if err != nil {
		return PlayMedia{}, err
	}

	ordered := rankStreams(pb.Streams)
	if len(ordered) == 0 {
		return PlayMedia{}, sdk.ErrContentUnavailable
	}

	defs := collectDefinitions(ordered)
	chosen := pickStream(ordered, preferredDef)
	def := streamLabel(chosen)

	res := PlayMedia{
		EpisodeID:   episodeID,
		Definition:  def,
		Definitions: defs,
		DurationSec: pb.DurationSeconds,
		Protected:   pb.Protected,
		ExpiresAt:   pb.ExpiresAt,
	}
	if pb.EpisodeID != "" {
		res.EpisodeID = pb.EpisodeID
	}

	if !pb.Protected {
		res.StreamURL = chosen.URL
		return res, nil
	}

	if onStage != nil {
		onStage("decrypting")
	}

	derive := s.DeriveKey
	if derive == nil {
		derive = sdk.ContentKey
	}
	key, err := derive(pb)
	if err != nil {
		return PlayMedia{}, err
	}

	epSafe := sanitizeID(res.EpisodeID)
	if epSafe == "" {
		return PlayMedia{}, errors.New("invalid episode id for cache")
	}
	defSafe := safeDefRegex.ReplaceAllString(def, "_")
	defSafe = strings.Trim(defSafe, "_")
	if defSafe == "" {
		defSafe = "default"
	}
	mediaDir := filepath.Join(s.dir, "media")
	outPath := filepath.Join(mediaDir, fmt.Sprintf("%s_%s.mp4", epSafe, defSafe))

	if err := s.runDecryptJob(ctx, outPath, key.Hex, chosen.URL, chosen.BackupURL); err != nil {
		return PlayMedia{}, err
	}
	go PruneMedia(mediaDir, mediaKeepFiles)

	res.LocalPath = outPath
	return res, nil
}

// Prefetch quietly warms the local media cache for an upcoming episode.
func (s *Service) Prefetch(ctx context.Context, episodeID string) {
	if episodeID == "" {
		return
	}
	_, _ = s.ResolvePlay(ctx, episodeID, "", nil)
}

// ResetDevice resets the anonymous SDK device profile.
func (s *Service) ResetDevice(ctx context.Context) error {
	api, err := s.client()
	if err != nil {
		return err
	}
	return api.ResetDevice(ctx)
}

// ErrorMessage maps SDK/decryption errors to localized user-facing text and whether resetting the device might help.
func (s *Service) ErrorMessage(err error) (msg string, canReset bool) {
	if err == nil {
		return "", false
	}
	switch {
	case errors.Is(err, ErrFFmpegMissing):
		return locales.HongguoFfmpegMissing(), false
	case errors.Is(err, sdk.ErrAnonymousLimited):
		return locales.HongguoErrorAnonymous(), true
	case errors.Is(err, sdk.ErrDeviceRejected):
		return locales.HongguoErrorDevice(), true
	case errors.Is(err, sdk.ErrRateLimited):
		return locales.HongguoErrorRateLimit(), false
	case errors.Is(err, sdk.ErrTransport):
		return locales.HongguoErrorNetwork(), false
	case errors.Is(err, sdk.ErrContentUnavailable):
		return locales.HongguoErrorUnavailable(), false
	default:
		return err.Error(), false
	}
}

func (s *Service) resolvePlaybackWithReset(ctx context.Context, api API, episodeID string) (*model.Playback, error) {
	call := func() (*model.Playback, error) {
		pb, err := api.ResolvePlayback(ctx, episodeID, sdk.PlaybackOptions{})
		if err != nil && !errors.Is(err, sdk.ErrDRMProtected) {
			return nil, err
		}
		if pb == nil || len(pb.Streams) == 0 {
			return nil, sdk.ErrAnonymousLimited
		}
		return pb, nil
	}

	pb, err := call()
	if errors.Is(err, sdk.ErrAnonymousLimited) || errors.Is(err, sdk.ErrDeviceRejected) {
		if rerr := api.ResetDevice(ctx); rerr == nil {
			pb, err = call()
		}
	}
	return pb, err
}

func (s *Service) runDecryptJob(ctx context.Context, outPath, keyHex, srcURL, backupURL string) error {
	s.mu.Lock()
	job, running := s.mediaJobs[outPath]
	if !running {
		job = &mediaJob{done: make(chan struct{})}
		s.mediaJobs[outPath] = job
		decryptFn := s.Decrypt
		go func() {
			var err error
			if decryptFn != nil {
				err = decryptFn(context.Background(), keyHex, srcURL, backupURL, outPath)
			} else {
				var bin string
				bin, err = FindFFmpeg()
				if err == nil {
					err = DecryptMP4(context.Background(), bin, keyHex, srcURL, backupURL, outPath)
				}
			}
			job.err = err
			close(job.done)
			s.mu.Lock()
			if s.mediaJobs[outPath] == job {
				delete(s.mediaJobs, outPath)
			}
			s.mu.Unlock()
		}()
	}
	s.mu.Unlock()

	select {
	case <-job.done:
		return job.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func sanitizeID(id string) string {
	if !safeIDRegex.MatchString(id) {
		return ""
	}
	return id
}

func rankStreams(streams []model.Stream) []model.Stream {
	var nonDash, dashOnly []model.Stream
	for _, st := range streams {
		if strings.TrimSpace(st.URL) == "" {
			continue
		}
		if st.Dash {
			dashOnly = append(dashOnly, st)
			continue
		}
		nonDash = append(nonDash, st)
	}
	usable := nonDash
	if len(usable) == 0 {
		usable = dashOnly
	}
	sort.SliceStable(usable, func(i, j int) bool {
		hi, hj := streamHeightRank(usable[i]), streamHeightRank(usable[j])
		if hi != hj {
			return hi > hj
		}
		return usable[i].Bitrate > usable[j].Bitrate
	})
	return usable
}

func streamHeightRank(st model.Stream) int {
	if st.Height > 0 {
		return st.Height
	}
	digits := 0
	for _, r := range st.Definition {
		if r < '0' || r > '9' {
			break
		}
		digits = digits*10 + int(r-'0')
	}
	if digits > 0 {
		return digits
	}
	return st.Width
}

func streamLabel(st model.Stream) string {
	if st.Definition != "" {
		return st.Definition
	}
	if st.Quality != "" {
		return st.Quality
	}
	return "default"
}

func collectDefinitions(streams []model.Stream) []string {
	seen := make(map[string]bool, len(streams))
	var out []string
	for _, st := range streams {
		lbl := streamLabel(st)
		if !seen[lbl] {
			seen[lbl] = true
			out = append(out, lbl)
		}
	}
	return out
}

func pickStream(streams []model.Stream, preferredDef string) model.Stream {
	if preferredDef != "" {
		for _, st := range streams {
			if strings.EqualFold(streamLabel(st), preferredDef) {
				return st
			}
		}
	}
	for _, st := range streams {
		if !st.HLS && !st.Dash {
			return st
		}
	}
	return streams[0]
}
