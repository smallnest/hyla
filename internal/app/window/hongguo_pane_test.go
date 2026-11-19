package window

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	"github.com/egoist/mygo/ui"

	"hyla/internal/app/hongguo"
	"hyla/internal/app/host"
	"hyla/internal/proto"
	"hyla/internal/ui/hongguoview"
	"hyla/locales"
)

type mockHongguoAPI struct {
	discoverFn    func(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error)
	searchFn      func(ctx context.Context, query string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error)
	shortFilterFn func(ctx context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error)
	shortRankFn   func(ctx context.Context, q sdk.ShortRankQuery) (*model.ShortRanking, error)
	seriesFn      func(ctx context.Context, seriesID string) (*model.Series, error)
	batchFn       func(ctx context.Context, seriesIDs []string) ([]model.SeriesDetail, error)
	episodesFn    func(ctx context.Context, seriesID string, opts sdk.EpisodeOptions) (*sdk.Page[model.Episode], error)
	playbackFn    func(ctx context.Context, episodeID string, opts sdk.PlaybackOptions) (*model.Playback, error)
}

func (m *mockHongguoAPI) Discover(ctx context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
	if m.discoverFn != nil {
		return m.discoverFn(ctx, opts)
	}
	return &sdk.Page[model.SeriesSummary]{}, nil
}

func (m *mockHongguoAPI) Search(ctx context.Context, query string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, opts)
	}
	return &sdk.Page[model.SeriesSummary]{}, nil
}

func (m *mockHongguoAPI) ShortFilter(ctx context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error) {
	if m.shortFilterFn != nil {
		return m.shortFilterFn(ctx, q)
	}
	return &model.ShortFilterPage{}, nil
}

func (m *mockHongguoAPI) ShortRank(ctx context.Context, q sdk.ShortRankQuery) (*model.ShortRanking, error) {
	if m.shortRankFn != nil {
		return m.shortRankFn(ctx, q)
	}
	return &model.ShortRanking{}, nil
}

func (m *mockHongguoAPI) GetSeries(ctx context.Context, seriesID string) (*model.Series, error) {
	if m.seriesFn != nil {
		return m.seriesFn(ctx, seriesID)
	}
	return nil, nil
}

func (m *mockHongguoAPI) GetSeriesBatch(ctx context.Context, seriesIDs []string) ([]model.SeriesDetail, error) {
	if m.batchFn != nil {
		return m.batchFn(ctx, seriesIDs)
	}
	return nil, nil
}

func (m *mockHongguoAPI) ListEpisodes(ctx context.Context, seriesID string, opts sdk.EpisodeOptions) (*sdk.Page[model.Episode], error) {
	if m.episodesFn != nil {
		return m.episodesFn(ctx, seriesID, opts)
	}
	return &sdk.Page[model.Episode]{}, nil
}

func (m *mockHongguoAPI) ResolvePlayback(ctx context.Context, episodeID string, opts sdk.PlaybackOptions) (*model.Playback, error) {
	if m.playbackFn != nil {
		return m.playbackFn(ctx, episodeID, opts)
	}
	return nil, sdk.ErrContentUnavailable
}

func (m *mockHongguoAPI) ResetDevice(context.Context) error { return nil }

type testVideoDecoder struct {
	mu    sync.Mutex
	index int
	total int
	img   *image.RGBA
}

func newTestVideoDecoder() *testVideoDecoder {
	img := image.NewRGBA(image.Rect(0, 0, 32, 48))
	for y := range 48 {
		for x := range 32 {
			img.Set(x, y, color.RGBA{R: 220, G: 60, B: 80, A: 255})
		}
	}
	return &testVideoDecoder{total: 20, img: img}
}

func (d *testVideoDecoder) Next() (ui.VideoFrame, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.index >= d.total {
		return ui.VideoFrame{}, io.EOF
	}
	d.index++
	return ui.VideoFrame{Image: d.img, Duration: 200 * time.Millisecond}, nil
}

func (d *testVideoDecoder) Rewind() error {
	d.mu.Lock()
	d.index = 0
	d.mu.Unlock()
	return nil
}

func (d *testVideoDecoder) Duration() time.Duration { return 30 * time.Second }

func drainMainAndFrame(t *testing.T, tt *ui.Tester, mainQ chan func(), until func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for {
			select {
			case fn := <-mainQ:
				fn()
			default:
				goto drained
			}
		}
	drained:
		tt.Frame()
		if until() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for Hongguo UI condition")
}

func TestPaneMenuHongguo(t *testing.T) {
	useUpdaterPlugin()
	a := testApp(t)
	m := a.Menu()
	if m.ItemByID("hongguo.open") == nil {
		t.Fatal("no hongguo.open in the menus")
	}
}

func TestHongguoOpenSplits(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	got := make(chan proto.CreateParams, 2)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "split":
			var p proto.CreateParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			got <- p
			return proto.Session{ID: "s1"}, nil
		case "focus":
			return nil, nil
		}
		return nil, errors.New("unexpected method " + method)
	})
	a.Local().Client = cl
	defer stop()

	a.run("hongguo.open", nil)
	select {
	case p := <-got:
		if p.Split != "a" || !p.Hongguo || p.Direction != "right" {
			t.Fatalf("Hongguo split %+v, want a hongguo pane beside a", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Hongguo split no pane")
	}
}

func TestHongguoOpenGoesToItsPane(t *testing.T) {
	a := testApp(t)
	a.Local().Conn = host.StateReady
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "hg", Hongguo: true, Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "hg", "right", 0)
	s.Focus = "a"
	split := make(chan struct{}, 1)
	cl, stop := projectClient(t, func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "split":
			split <- struct{}{}
			return proto.Session{ID: "s1"}, nil
		case "focus":
			return nil, nil
		}
		return nil, errors.New("unexpected method " + method)
	})
	a.Local().Client = cl
	defer stop()

	a.run("hongguo.open", nil)
	select {
	case <-split:
		t.Fatal("Hongguo split a second pane")
	case <-time.After(50 * time.Millisecond):
	}
	if got := a.Focus["s1"]; got != "hg" {
		t.Errorf("Focus = %q, want hg", got)
	}
}

func TestHongguoPaneListAndSearch(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "hg", Hongguo: true, Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "hg", "right", 0)
	s.Focus = "hg"

	mainQ := make(chan func(), 64)
	a.hongguoMain = func(fn func()) { mainQ <- fn }

	svc := hongguo.NewService(t.TempDir())
	var page1, page2 []model.SeriesSummary
	for i := 1; i <= 25; i++ {
		page1 = append(page1, model.SeriesSummary{
			ID:           "d" + strconv.Itoa(i),
			Title:        "Drama " + strconv.Itoa(i),
			Description:  "Short drama description " + strconv.Itoa(i),
			EpisodeCount: 30,
		})
	}
	for i := 26; i <= 45; i++ {
		page2 = append(page2, model.SeriesSummary{
			ID:           "d" + strconv.Itoa(i),
			Title:        "Drama " + strconv.Itoa(i),
			Description:  "Short drama description " + strconv.Itoa(i),
			EpisodeCount: 30,
		})
	}
	svc.SetAPI(&mockHongguoAPI{
		shortFilterFn: func(_ context.Context, q sdk.ShortFilterQuery) (*model.ShortFilterPage, error) {
			if q.Cursor == "c2" {
				return &model.ShortFilterPage{Items: page2, HasMore: false}, nil
			}
			return &model.ShortFilterPage{Items: page1, Cursor: "c2", HasMore: true}, nil
		},
		discoverFn: func(_ context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			if opts.Channel == hongguo.ChannelRecommend {
				return &sdk.Page[model.SeriesSummary]{
					Items: []model.SeriesSummary{{ID: "r1", Title: "RecDrama 1", EpisodeCount: 24}},
				}, nil
			}
			return &sdk.Page[model.SeriesSummary]{}, nil
		},
		searchFn: func(_ context.Context, q string, opts sdk.SearchOptions) (*sdk.Page[model.SeriesSummary], error) {
			if opts.Cursor == "s2" {
				return &sdk.Page[model.SeriesSummary]{
					Items: []model.SeriesSummary{{ID: "sr2", Title: "Result " + q + " 2", EpisodeCount: 20}},
				}, nil
			}
			return &sdk.Page[model.SeriesSummary]{
				Items:   []model.SeriesSummary{{ID: "sr1", Title: "Result " + q, EpisodeCount: 20}},
				Cursor:  "s2",
				HasMore: true,
			}, nil
		},
	})
	a.HongguoSvc = svc

	tt := ui.NewTester(a.view, 1280, 720)
	drainMainAndFrame(t, tt, mainQ, func() bool {
		_, ok := tt.Find("Drama 1")
		return ok
	})

	// All 25 items of page 1 load continuously without 12-item slicing (rows beyond viewport are virtualized until scrolled).
	st := a.HongguoState("hg")
	if len(st.Items) != 25 {
		t.Fatalf("len(st.Items) = %d, want 25", len(st.Items))
	}
	if _, ok := tt.Find("Drama 12"); !ok {
		t.Fatalf("expected Drama 12 on initial virtualized grid; texts %q", tt.Texts())
	}
	if _, ok := tt.Find(locales.HongguoLoadedCount("25")); !ok {
		t.Fatalf("expected loaded count for 25 dramas; texts %q", tt.Texts())
	}

	// Scrolling to the bottom reveals previously virtualized rows and triggers auto-load of page 2 (Drama 26..45).
	searchBox, ok := tt.Find(locales.HongguoSearchPlaceholder())
	if !ok {
		t.Fatalf("missing search input")
	}
	if st.ListScroll.MaxY <= 0 {
		t.Fatalf("expected scrollable grid with MaxY > 0, got %v", st.ListScroll.MaxY)
	}
	tt.Scroll(searchBox.X+20, 400, 0, st.ListScroll.MaxY)
	drainMainAndFrame(t, tt, mainQ, func() bool {
		return len(st.Items) == 45
	})
	if _, ok := tt.Find("Drama 25"); !ok {
		t.Fatalf("expected Drama 25 after scrolling into view; texts %q", tt.Texts())
	}
	if _, ok := tt.Find("Drama 26"); !ok {
		t.Fatalf("expected Drama 26 after scroll pagination; texts %q", tt.Texts())
	}

	// Switch to Recommended tab.
	if err := tt.Click(locales.HongguoTabRecommend()); err != nil {
		t.Fatal(err)
	}
	drainMainAndFrame(t, tt, mainQ, func() bool {
		_, ok := tt.Find("RecDrama 1")
		return ok
	})

	// Server-side search via search input and Load More button click.
	searchBox, ok = tt.Find(locales.HongguoSearchPlaceholder())
	if !ok {
		t.Fatalf("missing search input")
	}
	tt.ClickAt(searchBox.X+searchBox.W/2, searchBox.Y+searchBox.H/2)
	tt.Type("Reverse")
	tt.Key(0, ui.KeyEnter)
	drainMainAndFrame(t, tt, mainQ, func() bool {
		_, ok := tt.Find("Result Reverse")
		return ok
	})
	if err := tt.Click(locales.HongguoLoadMore()); err != nil {
		t.Fatal(err)
	}
	drainMainAndFrame(t, tt, mainQ, func() bool {
		_, ok := tt.Find("Result Reverse 2")
		return ok
	})

	// Toggle History drawer.
	if _, ok := tt.Find(locales.HongguoHistory()); !ok {
		t.Fatalf("missing history button")
	}
	if err := tt.Click(locales.HongguoHistory()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find(locales.HongguoHistoryEmpty()); !ok {
		t.Fatalf("expected empty history text; texts %q", tt.Texts())
	}
}

func TestHongguoPanePlayerAndAutoPause(t *testing.T) {
	a := testApp(t)
	s := a.session("s1")
	s.Panes = append(s.Panes, proto.Pane{ID: "hg", Hongguo: true, Dir: "/tmp", Status: proto.Idle, Progress: -1})
	s.Layout = s.Layout.Split("a", "hg", "right", 0)
	s.Focus = "hg"

	mainQ := make(chan func(), 64)
	a.hongguoMain = func(fn func()) { mainQ <- fn }
	a.hongguoOpenVideo = func(localPath, streamURL string) (*ui.VideoPlayer, error) {
		return ui.NewVideoPlayer(newTestVideoDecoder())
	}

	svc := hongguo.NewService(t.TempDir())
	svc.SetAPI(&mockHongguoAPI{
		shortFilterFn: func(context.Context, sdk.ShortFilterQuery) (*model.ShortFilterPage, error) {
			return &model.ShortFilterPage{
				Items: []model.SeriesSummary{
					{ID: "d1", Title: "Alpha Drama", Description: "Desc", EpisodeCount: 2},
				},
			}, nil
		},
		discoverFn: func(context.Context, sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			return &sdk.Page[model.SeriesSummary]{
				Items: []model.SeriesSummary{
					{ID: "d1", Title: "Alpha Drama", Description: "Desc", EpisodeCount: 2},
				},
			}, nil
		},
		seriesFn: func(context.Context, string) (*model.Series, error) {
			return &model.Series{
				SeriesSummary: model.SeriesSummary{ID: "d1", Title: "Alpha Drama", EpisodeCount: 2},
			}, nil
		},
		episodesFn: func(context.Context, string, sdk.EpisodeOptions) (*sdk.Page[model.Episode], error) {
			return &sdk.Page[model.Episode]{
				Items: []model.Episode{
					{ID: "ep1", SeriesID: "d1", Number: 1, Playable: true, Duration: 30 * time.Second},
					{ID: "ep2", SeriesID: "d1", Number: 2, Playable: true, Duration: 30 * time.Second},
				},
			}, nil
		},
		playbackFn: func(_ context.Context, epID string, _ sdk.PlaybackOptions) (*model.Playback, error) {
			return &model.Playback{
				EpisodeID:       epID,
				DurationSeconds: 30,
				Protected:       false,
				Streams: []model.Stream{
					{Definition: "720p", URL: "https://example.com/" + epID + ".mp4", Height: 720},
				},
			}, nil
		},
	})
	a.HongguoSvc = svc

	tt := ui.NewTester(a.view, 1280, 720)
	drainMainAndFrame(t, tt, mainQ, func() bool {
		_, ok := tt.Find(locales.HongguoWatchNow())
		return ok
	})

	if err := tt.Click(locales.HongguoWatchNow()); err != nil {
		t.Fatal(err)
	}

	st := a.HongguoState("hg")
	drainMainAndFrame(t, tt, mainQ, func() bool {
		return st.Mode == hongguoview.ModePlayer && st.Player != nil && st.Player.Playing()
	})
	if st.CurrentEp == nil || st.CurrentEp.ID != "ep1" {
		t.Fatalf("CurrentEp = %+v, want ep1", st.CurrentEp)
	}

	// Switch to episode 2 from the episode grid.
	if _, ok := tt.Find("2"); !ok {
		t.Fatalf("missing episode 2 button; texts %q", tt.Texts())
	}
	if err := tt.Click("2"); err != nil {
		t.Fatal(err)
	}
	drainMainAndFrame(t, tt, mainQ, func() bool {
		return st.CurrentEp != nil && st.CurrentEp.ID == "ep2" && st.Player != nil && st.Player.Playing()
	})

	// Agent transitions Working -> Idle: auto-pause should pause playback and show notice.
	makeState := func(status proto.Status) proto.State {
		return proto.State{
			Projects: []proto.Project{{ID: "p1", Name: "demo", Dir: "/tmp"}},
			Sessions: []proto.Session{{
				ID: "s1", Project: "p1", Name: "demo",
				Panes: []proto.Pane{
					{ID: "a", Shell: "fish", Dir: "/tmp", Status: status, Progress: -1},
					{ID: "hg", Hongguo: true, Dir: "/tmp", Status: proto.Idle, Progress: -1},
				},
				Layout: s.Layout,
				Focus:  "hg",
			}},
		}
	}
	a.TakeStateOf(a.Local(), makeState(proto.Working))
	if !st.Player.Playing() {
		t.Fatal("expected playback to continue while agent is Working")
	}
	a.TakeStateOf(a.Local(), makeState(proto.Idle))
	if st.Player.Playing() {
		t.Fatal("expected playback to auto-pause when agent transitions Working -> Idle")
	}
	tt.Frame()
	if _, ok := tt.Find(locales.HongguoAutoPausedNotice()); !ok {
		t.Fatalf("expected auto-pause notice in UI; texts %q", tt.Texts())
	}

	// DropHongguo cleans up player and state.
	a.DropHongguo("hg")
	if a.Hongguo["hg"] != nil {
		t.Fatal("expected DropHongguo to remove pane state")
	}
	if len(svc.History()) == 0 {
		t.Fatal("expected watch history saved for watched episode")
	}
}
