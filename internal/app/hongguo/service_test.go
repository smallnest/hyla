package hongguo

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

type fakeAPI struct {
	discoverFn    func(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error)
	searchFn      func(ctx context.Context, query string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error)
	shortFilterFn func(ctx context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error)
	shortRankFn   func(ctx context.Context, q sdk.ShortRankQuery) (*model.ShortRanking, error)
	seriesFn      func(ctx context.Context, seriesID string) (*model.Series, error)
	batchFn       func(ctx context.Context, seriesIDs []string) ([]model.SeriesDetail, error)
	episodesFn    func(ctx context.Context, seriesID string, opts sdk.EpisodeOptions) (*sdk.Page[model.Episode], error)
	playbackFn    func(ctx context.Context, episodeID string, opts sdk.PlaybackOptions) (*model.Playback, error)
	resetFn       func(ctx context.Context) error
	resetCalls    atomic.Int32
	searchCalls   atomic.Int32
}

func (f *fakeAPI) Discover(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
	if f.discoverFn != nil {
		return f.discoverFn(ctx, opts)
	}
	return &sdk.Page[model.SeriesSummary]{}, nil
}

func (f *fakeAPI) Search(ctx context.Context, query string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error) {
	f.searchCalls.Add(1)
	if f.searchFn != nil {
		return f.searchFn(ctx, query, opts)
	}
	return &sdk.Page[model.SeriesSummary]{}, nil
}

func (f *fakeAPI) ShortFilter(ctx context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error) {
	if f.shortFilterFn != nil {
		return f.shortFilterFn(ctx, q)
	}
	return &model.ShortFilterPage{}, nil
}

func (f *fakeAPI) ShortRank(ctx context.Context, q sdk.ShortRankQuery) (*model.ShortRanking, error) {
	if f.shortRankFn != nil {
		return f.shortRankFn(ctx, q)
	}
	return &model.ShortRanking{}, nil
}

func (f *fakeAPI) GetSeries(ctx context.Context, seriesID string) (*model.Series, error) {
	if f.seriesFn != nil {
		return f.seriesFn(ctx, seriesID)
	}
	return nil, nil
}

func (f *fakeAPI) GetSeriesBatch(ctx context.Context, seriesIDs []string) ([]model.SeriesDetail, error) {
	if f.batchFn != nil {
		return f.batchFn(ctx, seriesIDs)
	}
	return nil, nil
}

func (f *fakeAPI) ListEpisodes(ctx context.Context, seriesID string, opts sdk.EpisodeOptions) (*sdk.Page[model.Episode], error) {
	if f.episodesFn != nil {
		return f.episodesFn(ctx, seriesID, opts)
	}
	return &sdk.Page[model.Episode]{}, nil
}

func (f *fakeAPI) ResolvePlayback(ctx context.Context, episodeID string, opts sdk.PlaybackOptions) (*model.Playback, error) {
	if f.playbackFn != nil {
		return f.playbackFn(ctx, episodeID, opts)
	}
	return nil, sdk.ErrContentUnavailable
}

func (f *fakeAPI) ResetDevice(ctx context.Context) error {
	f.resetCalls.Add(1)
	if f.resetFn != nil {
		return f.resetFn(ctx)
	}
	return nil
}

func TestFeedDedupeCacheAndFallbackSearch(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir)
	var discoverCalls atomic.Int32
	api := &fakeAPI{
		discoverFn: func(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			discoverCalls.Add(1)
			if opts.Cursor == "" {
				var items []model.SeriesSummary
				for i := range 12 {
					items = append(items, model.SeriesSummary{
						ID:       string(rune('a'+i)) + "100",
						Title:    "Drama " + string(rune('A'+i)),
						CoverURL: "https://example.com/cover.jpg",
					})
				}
				return &sdk.Page[model.SeriesSummary]{Items: items, Cursor: "p2", HasMore: true}, nil
			}
			// Duplicate first item + one item without cover.
			return &sdk.Page[model.SeriesSummary]{
				Items: []model.SeriesSummary{
					{ID: "a100", Title: "Drama A", CoverURL: "https://example.com/cover.jpg"},
					{ID: "nocover1", Title: "No Cover"},
				},
				HasMore: false,
			}, nil
		},
		searchFn: func(ctx context.Context, query string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error) {
			return &sdk.Page[model.SeriesSummary]{
				Items: []model.SeriesSummary{
					{ID: "search-" + query, Title: "Search " + query, CoverURL: "https://example.com/s.jpg"},
				},
			}, nil
		},
	}
	svc.SetAPI(api)

	got, err := svc.Feed(context.Background(), false)
	if err != nil {
		t.Fatalf("Feed error: %v", err)
	}
	// Dramas without covers are kept rather than discarded (12 + nocover1 = 13).
	if len(got) != 13 {
		t.Fatalf("len(Feed) = %d, want 13", len(got))
	}
	foundNoCover := false
	for _, d := range got {
		if d.ID == "nocover1" {
			foundNoCover = true
		}
	}
	if !foundNoCover {
		t.Fatalf("expected nocover1 to be preserved in Feed")
	}

	// Second call within TTL uses cache.
	_, err = svc.Feed(context.Background(), false)
	if err != nil {
		t.Fatalf("cached Feed error: %v", err)
	}
	if discoverCalls.Load() != 2 {
		t.Fatalf("discoverCalls = %d, want 2 (cached on second Feed)", discoverCalls.Load())
	}

	// When Discover returns empty, Feed falls back to fallbackSearchTerms.
	api2 := &fakeAPI{
		discoverFn: func(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			return &sdk.Page[model.SeriesSummary]{}, nil
		},
		searchFn: api.searchFn,
	}
	svc2 := NewService(t.TempDir())
	svc2.SetAPI(api2)
	fb, err := svc2.Feed(context.Background(), false)
	if err != nil {
		t.Fatalf("fallback Feed error: %v", err)
	}
	if len(fb) != 3 {
		t.Fatalf("len(fallback Feed) = %d, want 3", len(fb))
	}
}

func TestCatalogInfinitePaginationChannelsAndFallback(t *testing.T) {
	// 1. ChannelAll paginates unbounded past 48 items with zero duplicates and cover backfill.
	svc := NewService(t.TempDir())
	var sfCalls, batchCalls atomic.Int32
	api := &fakeAPI{
		shortFilterFn: func(ctx context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error) {
			sfCalls.Add(1)
			start := 0
			if q.Cursor != "" {
				var err error
				start, err = strconv.Atoi(q.Cursor)
				if err != nil {
					t.Fatalf("unexpected cursor %q", q.Cursor)
				}
			}
			var items []model.SeriesSummary
			// Overlap first item with previous batch to verify deduplication.
			if start > 0 {
				prevID := "sf-" + strconv.Itoa(start-1)
				items = append(items, model.SeriesSummary{ID: prevID, Title: "Dup " + prevID})
			}
			end := min(start+15, 65)
			for i := start; i < end; i++ {
				cover := "https://example.com/sf-" + strconv.Itoa(i) + ".jpg"
				if i == 2 {
					cover = ""
				}
				items = append(items, model.SeriesSummary{
					ID:           "sf-" + strconv.Itoa(i),
					Title:        "Short " + strconv.Itoa(i),
					CoverURL:     cover,
					EpisodeCount: 20 + i,
				})
			}
			hasMore := end < 65
			nextCur := ""
			if hasMore {
				nextCur = strconv.Itoa(end)
			}
			return &model.ShortFilterPage{Items: items, Cursor: nextCur, HasMore: hasMore}, nil
		},
		discoverFn: func(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			switch opts.Channel {
			case ChannelLive:
				return &sdk.Page[model.SeriesSummary]{
					Items: []model.SeriesSummary{{ID: "live-1", Title: "Live Drama", CoverURL: "https://example.com/live.jpg"}},
				}, nil
			case ChannelRecommend:
				return &sdk.Page[model.SeriesSummary]{
					Items: []model.SeriesSummary{{ID: "rec-1", Title: "Rec Drama", CoverURL: "https://example.com/rec.jpg"}},
				}, nil
			default:
				return &sdk.Page[model.SeriesSummary]{}, nil
			}
		},
		shortRankFn: func(ctx context.Context, q sdk.ShortRankQuery) (*model.ShortRanking, error) {
			return &model.ShortRanking{
				Items: []model.RankItem{
					{SeriesID: "rank-1", Title: "Rank #1", Score: "999万", EpisodeCount: 50, CoverURL: "https://example.com/r1.jpg"},
					{ActorID: "actor-only"},
				},
			}, nil
		},
		batchFn: func(ctx context.Context, ids []string) ([]model.SeriesDetail, error) {
			batchCalls.Add(1)
			out := make([]model.SeriesDetail, 0, len(ids))
			for _, id := range ids {
				out = append(out, model.SeriesDetail{
					Series: &model.Series{SeriesSummary: model.SeriesSummary{ID: id, CoverURL: "https://example.com/bf-" + id + ".jpg"}},
				})
			}
			return out, nil
		},
	}
	svc.SetAPI(api)

	var all []Drama
	seen := make(map[string]struct{})
	cursor := ""
	for {
		page, err := svc.Catalog(context.Background(), ChannelAll, cursor, seen, false)
		if err != nil {
			t.Fatalf("Catalog(all, %q) error: %v", cursor, err)
		}
		for _, d := range page.Items {
			if _, dup := seen[d.ID]; dup {
				t.Fatalf("duplicate drama %q across catalog pages", d.ID)
			}
			seen[d.ID] = struct{}{}
			all = append(all, d)
		}
		if !page.HasMore || page.Cursor == "" {
			break
		}
		cursor = page.Cursor
	}
	if len(all) != 65 {
		t.Fatalf("total paginated ChannelAll items = %d, want 65", len(all))
	}
	if batchCalls.Load() == 0 || all[2].CoverURL != "https://example.com/bf-sf-2.jpg" {
		t.Fatalf("expected cover backfill for sf-2, got %q (batchCalls=%d)", all[2].CoverURL, batchCalls.Load())
	}

	// Page 1 of ChannelAll is cached per channel.
	beforeCalls := sfCalls.Load()
	cached, err := svc.Catalog(context.Background(), ChannelAll, "", nil, false)
	if err != nil || len(cached.Items) == 0 || sfCalls.Load() != beforeCalls {
		t.Fatalf("expected cached first page for ChannelAll (calls %d -> %d, err=%v)", beforeCalls, sfCalls.Load(), err)
	}

	// Separate channels cache and paginate independently.
	live, err := svc.Catalog(context.Background(), ChannelLive, "", nil, false)
	if err != nil || len(live.Items) != 1 || live.Items[0].ID != "live-1" {
		t.Fatalf("ChannelLive = %+v, err=%v", live, err)
	}
	rank, err := svc.Catalog(context.Background(), ChannelRank, "", nil, false)
	if err != nil || len(rank.Items) != 1 || rank.Items[0].ID != "rank-1" || rank.Items[0].Description != "999万" {
		t.Fatalf("ChannelRank = %+v, err=%v", rank, err)
	}

	// 2. ChannelAll falls back to Discover with "discover:" cursor when ShortFilter errors.
	svcFB := NewService(t.TempDir())
	svcFB.SetAPI(&fakeAPI{
		shortFilterFn: func(context.Context, sdk.ShortFilterQuery) (*model.ShortFilterPage, error) {
			return nil, errors.New("filter down")
		},
		discoverFn: func(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			if opts.Channel != ChannelRecommend {
				t.Fatalf("fallback discover channel = %q, want %q", opts.Channel, ChannelRecommend)
			}
			if opts.Cursor == "" {
				var p1 []model.SeriesSummary
				for i := range 24 {
					p1 = append(p1, model.SeriesSummary{
						ID:       "disc-" + strconv.Itoa(i),
						Title:    "Disc " + strconv.Itoa(i),
						CoverURL: "https://example.com/d.jpg",
					})
				}
				return &sdk.Page[model.SeriesSummary]{Items: p1, Cursor: "dc2", HasMore: true}, nil
			}
			if opts.Cursor != "dc2" {
				t.Fatalf("unexpected discover fallback cursor %q", opts.Cursor)
			}
			return &sdk.Page[model.SeriesSummary]{
				Items: []model.SeriesSummary{{ID: "disc-24", Title: "Disc 24", CoverURL: "https://example.com/d.jpg"}},
			}, nil
		},
	})
	p1, err := svcFB.Catalog(context.Background(), ChannelAll, "", nil, false)
	if err != nil {
		t.Fatalf("fallback Catalog page 1 error: %v", err)
	}
	if len(p1.Items) != 24 || p1.Cursor != "discover:dc2" || !p1.HasMore {
		t.Fatalf("unexpected fallback page 1: items=%d cursor=%q hasMore=%v", len(p1.Items), p1.Cursor, p1.HasMore)
	}
	p2, err := svcFB.Catalog(context.Background(), ChannelAll, p1.Cursor, nil, false)
	if err != nil {
		t.Fatalf("fallback Catalog page 2 error: %v", err)
	}
	if len(p2.Items) != 1 || p2.Items[0].ID != "disc-24" || p2.HasMore {
		t.Fatalf("unexpected fallback page 2: %+v", p2)
	}
}

func TestSearchResetAndCoverBackfill(t *testing.T) {
	svc := NewService(t.TempDir())
	var attempt atomic.Int32
	api := &fakeAPI{
		searchFn: func(ctx context.Context, query string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error) {
			if attempt.Add(1) == 1 {
				return nil, sdk.ErrAnonymousLimited
			}
			if opts.Cursor != "c1" {
				t.Fatalf("cursor = %q, want c1", opts.Cursor)
			}
			return &sdk.Page[model.SeriesSummary]{
				Items: []model.SeriesSummary{
					{ID: "s1", Title: "First"},
					{ID: "s1", Title: "Duplicate"},
					{ID: "s2", Title: "Second", CoverURL: "https://example.com/2.jpg"},
				},
				Cursor:  "c2",
				HasMore: true,
			}, nil
		},
		batchFn: func(ctx context.Context, seriesIDs []string) ([]model.SeriesDetail, error) {
			if len(seriesIDs) != 1 || seriesIDs[0] != "s1" {
				t.Fatalf("batch IDs = %v, want [s1]", seriesIDs)
			}
			return []model.SeriesDetail{{
				Series: &model.Series{
					SeriesSummary: model.SeriesSummary{
						ID:           "s1",
						CoverURL:     "https://example.com/backfill1.jpg",
						EpisodeCount: 40,
					},
				},
			}}, nil
		},
	}
	svc.SetAPI(api)

	res, err := svc.Search(context.Background(), "重生", "c1")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if api.resetCalls.Load() != 1 {
		t.Fatalf("resetCalls = %d, want 1", api.resetCalls.Load())
	}
	if len(res.Items) != 2 || res.Cursor != "c2" || !res.HasMore {
		t.Fatalf("unexpected Search result: %+v", res)
	}
	if res.Items[0].CoverURL != "https://example.com/backfill1.jpg" || res.Items[0].EpisodeCount != 40 {
		t.Fatalf("backfill failed: %+v", res.Items[0])
	}
}

func TestResolvePlayProtectedSingleFlightAndClearStream(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir)
	var decryptCalls atomic.Int32
	var stages []string
	var stageMu sync.Mutex

	api := &fakeAPI{
		playbackFn: func(ctx context.Context, episodeID string, opts sdk.PlaybackOptions) (*model.Playback, error) {
			if episodeID == "clear1" {
				return &model.Playback{
					EpisodeID:       "clear1",
					DurationSeconds: 95,
					Protected:       false,
					Streams: []model.Stream{
						{Definition: "720p", URL: "https://cdn.example.com/720.mp4", Height: 720, Bitrate: 1000},
						{Definition: "1080p", URL: "https://cdn.example.com/1080.mp4", Height: 1080, Bitrate: 2000},
					},
				}, nil
			}
			return &model.Playback{
				EpisodeID:       "ep1",
				DurationSeconds: 120,
				Protected:       true,
				KeyMaterial:     "opaque",
				Streams: []model.Stream{
					{Definition: "720p", URL: "https://cdn.example.com/720.enc", Height: 720, Bitrate: 1000, Protected: true},
					{Definition: "1080p", URL: "https://cdn.example.com/1080.enc", Height: 1080, Bitrate: 2000, Protected: true},
				},
			}, sdk.ErrDRMProtected
		},
	}
	svc.SetAPI(api)
	svc.DeriveKey = func(pb *model.Playback) (model.ContentKey, error) {
		return model.ContentKey{Hex: "00112233445566778899aabbccddeeff"}, nil
	}
	svc.Decrypt = func(ctx context.Context, keyHex, srcURL, backupURL, outPath string) error {
		decryptCalls.Add(1)
		time.Sleep(60 * time.Millisecond)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
			return err
		}
		return os.WriteFile(outPath, []byte("mp4data"), 0o600)
	}

	// Concurrent calls for the same protected episode/definition must share one decrypt job.
	var wg sync.WaitGroup
	results := make([]PlayMedia, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = svc.ResolvePlay(context.Background(), "ep1", "", func(st string) {
				stageMu.Lock()
				stages = append(stages, st)
				stageMu.Unlock()
			})
		}(i)
	}
	wg.Wait()

	for i := range 2 {
		if errs[i] != nil {
			t.Fatalf("ResolvePlay[%d] error: %v", i, errs[i])
		}
		if results[i].Definition != "1080p" || results[i].LocalPath == "" {
			t.Fatalf("ResolvePlay[%d] = %+v", i, results[i])
		}
	}
	if decryptCalls.Load() != 1 {
		t.Fatalf("decryptCalls = %d, want 1 (single-flight)", decryptCalls.Load())
	}
	if len(stages) == 0 || stages[0] != "decrypting" {
		t.Fatalf("stages = %v, want decrypting", stages)
	}

	// Unencrypted stream returns StreamURL directly and respects preferred definition.
	clearRes, err := svc.ResolvePlay(context.Background(), "clear1", "720p", nil)
	if err != nil {
		t.Fatalf("clear ResolvePlay error: %v", err)
	}
	if clearRes.StreamURL != "https://cdn.example.com/720.mp4" || clearRes.LocalPath != "" || clearRes.Definition != "720p" {
		t.Fatalf("clearRes = %+v", clearRes)
	}

	// Error mapping checks.
	if msg, reset := svc.ErrorMessage(ErrFFmpegMissing); msg == "" || reset {
		t.Fatalf("ErrorMessage(ErrFFmpegMissing) = (%q, %v)", msg, reset)
	}
	if msg, reset := svc.ErrorMessage(sdk.ErrAnonymousLimited); msg == "" || !reset {
		t.Fatalf("ErrorMessage(ErrAnonymousLimited) = (%q, %v)", msg, reset)
	}
}

func TestPruneMedia(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	names := []string{"ep1.mp4", "ep2.mp4", "ep3.mp4", "ep4.mp4", "ep5.part.mp4"}
	for i, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(time.Duration(i) * time.Minute)
		_ = os.Chtimes(p, mt, mt)
	}
	PruneMedia(dir, 2)
	if _, err := os.Stat(filepath.Join(dir, "ep1.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ep1.mp4 pruned, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ep2.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ep2.mp4 pruned, err=%v", err)
	}
	for _, keep := range []string{"ep3.mp4", "ep4.mp4", "ep5.part.mp4"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Fatalf("expected %s kept, err=%v", keep, err)
		}
	}
}

func TestHistoryAndPrefsPersistence(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir)

	svc.SavePrefs(Prefs{Volume: 1.5, Muted: true, Rate: 99, AutoPause: false})
	svc2 := NewService(dir)
	p := svc2.Prefs()
	if p.Volume != 1.0 || !p.Muted || p.Rate != 1.0 || p.AutoPause {
		t.Fatalf("clamped prefs = %+v", p)
	}

	for i := range 25 {
		svc.SaveProgress(HistoryEntry{
			SeriesID:  string(rune('a'+i/10)) + string(rune('0'+i%10)),
			Title:     "Drama",
			EpisodeID: "ep1",
			Number:    1,
			Position:  float64(i + 1),
			UpdatedAt: int64(1000 + i),
		})
	}
	hist := NewService(dir).History()
	if len(hist) != maxHistoryEntries {
		t.Fatalf("len(history) = %d, want %d", len(hist), maxHistoryEntries)
	}
	if hist[0].Position != 25 {
		t.Fatalf("newest entry position = %v, want 25", hist[0].Position)
	}
	if pos := NewService(dir).ProgressOf(hist[0].SeriesID, "ep1"); pos != 25 {
		t.Fatalf("ProgressOf = %v, want 25", pos)
	}
	svc.RemoveHistory(hist[0].SeriesID)
	if len(NewService(dir).History()) != maxHistoryEntries-1 {
		t.Fatalf("len after remove = %d", len(NewService(dir).History()))
	}
	svc.ClearHistory()
	if len(NewService(dir).History()) != 0 {
		t.Fatalf("expected cleared history")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCoverCacheFetchAndDiskCache(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 16, 24))
	for y := range 24 {
		for x := range 16 {
			img.Set(x, y, color.RGBA{R: 220, G: 40, B: 60, A: 255})
		}
	}
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	jpgBytes := buf.Bytes()

	var hits atomic.Int32
	tr := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(bytes.NewReader(jpgBytes)),
		}, nil
	})

	cacheDir := t.TempDir()
	coverURL := "https://example.com/poster.jpg"
	cc := NewCoverCache(cacheDir)
	cc.httpClient = &http.Client{Transport: tr}
	done := make(chan struct{}, 1)
	if bm := cc.Get(coverURL, func() { done <- struct{}{} }); bm != nil {
		t.Fatal("expected nil on initial async fetch")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cover fetch timed out")
	}
	bm := cc.Get(coverURL, nil)
	if bm == nil {
		t.Fatal("expected cached bitmap after ready callback")
	}
	w, h := bm.Size()
	if w != 16 || h != 24 {
		t.Fatalf("bitmap size = %dx%d, want 16x24", w, h)
	}

	// Second CoverCache sharing cacheDir reads from disk without HTTP hit.
	cc2 := NewCoverCache(cacheDir)
	cc2.httpClient = &http.Client{Transport: tr}
	done2 := make(chan struct{}, 1)
	_ = cc2.Get(coverURL, func() { done2 <- struct{}{} })
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("disk cache load timed out")
	}
	if hits.Load() != 1 {
		t.Fatalf("HTTP hits = %d, want 1 (second load from disk cache)", hits.Load())
	}
}
