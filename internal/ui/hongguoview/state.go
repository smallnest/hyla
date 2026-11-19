package hongguoview

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"hyla/internal/app/hongguo"
	"hyla/internal/proto"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

const (
	episodeChunkSize   = 50
	progressSaveEvery  = 3 * time.Second
	seekGuardDuration  = 350 * time.Millisecond
	endResumeMarginSec = 5.0
)

// Mode selects between the drama browser list and the active video player.
type Mode int

const (
	ModeList Mode = iota
	ModePlayer
)

// App defines the host window callbacks required by hongguoview.
type App interface {
	HongguoService() *hongguo.Service
	HongguoState(paneID string) *State
	PaneRef(id string) *ui.Handle
	FocusOf(s *proto.Session) string
	SetFocus(s *proto.Session, pane string)
	RunMain(fn func())
	AfterFrame(fn func())
	OpenVideo(localPath, streamURL string) (*ui.VideoPlayer, error)
}

// State holds all persistent UI state for one Hongguo drama pane.
type State struct {
	PaneID string
	Mode   Mode

	ShowHistory  bool
	ShowEpisodes bool

	SearchInput   string
	ActiveKeyword string
	ActiveChannel string
	Items         []hongguo.Drama
	FeedCursor    string
	SearchCursor  string
	HasMore       bool
	FeedLoaded    bool
	FeedLoading   bool
	FeedErr       string
	FeedCanReset  bool
	ListScroll    ui.ScrollState

	Series           hongguo.Drama
	Episodes         []hongguo.Episode
	EpisodeChunk     int
	SeriesLoading    bool
	SeriesErr        string
	CurrentEp        *hongguo.Episode
	PlayLoading      bool
	PlayStage        string
	PlayErr          string
	PlayCanReset     bool
	Definitions      []string
	ActiveDef        string
	Player           *ui.VideoPlayer
	ResumeAt         float64
	SeekPos          float64
	Seeking          bool
	SeekGuardUntil   time.Time
	EndedHandled     bool
	PrefetchedID     string
	AutoPausedReason string

	Volume    float64
	Muted     bool
	Rate      float64
	AutoPause bool

	LastSavedPos float64
	LastSavedAt  time.Time

	SearchIn ui.Handle

	closed     bool
	feedGen    int
	seriesGen  int
	playGen    int
	cancelPlay context.CancelFunc
}

// NewState initializes a Hongguo pane state seeded from saved preferences.
func NewState(paneID string, svc *hongguo.Service) *State {
	prefs := svc.Prefs()
	return &State{
		PaneID:        paneID,
		ActiveChannel: hongguo.ChannelAll,
		ShowEpisodes:  true,
		Volume:        prefs.Volume,
		Muted:         prefs.Muted,
		Rate:          prefs.Rate,
		AutoPause:     prefs.AutoPause,
	}
}

// Pane draws the Hongguo short drama pane inside the current card body.
func Pane(x App, c *ui.Context, pal *theme.Palette, s *proto.Session, p *proto.Pane) {
	if p == nil {
		return
	}
	svc := x.HongguoService()
	st := x.HongguoState(p.ID)
	if !st.FeedLoaded && !st.FeedLoading && st.FeedErr == "" {
		st.loadFeed(x, svc, false)
	}

	ref := x.PaneRef(p.ID)
	root := ui.Column(c.Key("hongguo-" + p.ID)).Grow(1).MinHeight(0).Focusable().Bind(ref)
	if (root.Pressed() || root.FocusWithin()) && s != nil && x.FocusOf(s) != p.ID {
		x.SetFocus(s, p.ID)
	}
	width := ref.Bounds(c).W
	if width <= 0 {
		width = 480
	}

	root.Children(func() {
		if st.Mode == ModePlayer {
			st.handlePlayerShortcuts(x, c, svc, root)
			st.renderPlayer(x, c, pal, svc, width)
		} else {
			st.renderList(x, c, pal, svc, width)
		}
	})
}

// PauseForAgent pauses playback when the coding agent finishes work and AutoPause is enabled.
func (st *State) PauseForAgent(svc *hongguo.Service, reason string) {
	if !st.AutoPause || st.Player == nil || !st.Player.Playing() {
		return
	}
	st.Player.Pause()
	st.saveCurrentProgress(svc, false)
	st.AutoPausedReason = reason
}

// Close cancels pending playback jobs, saves progress, and destroys any active player.
func (st *State) Close(svc *hongguo.Service) {
	st.closed = true
	if st.cancelPlay != nil {
		st.cancelPlay()
		st.cancelPlay = nil
	}
	st.saveCurrentProgress(svc, false)
	st.closePlayer()
}

func (st *State) closePlayer() {
	if st.Player != nil {
		st.Player.Close()
		st.Player = nil
	}
}

func (st *State) savePrefs(svc *hongguo.Service) {
	svc.SavePrefs(hongguo.Prefs{
		Volume:    st.Volume,
		Muted:     st.Muted,
		Rate:      st.Rate,
		AutoPause: st.AutoPause,
	})
}

func (st *State) saveCurrentProgress(svc *hongguo.Service, forceZero bool) {
	if svc == nil || st.Series.ID == "" || st.CurrentEp == nil {
		return
	}
	pos := st.SeekPos
	if st.Player != nil {
		p := st.Player.Position().Seconds()
		if !math.IsNaN(p) && !math.IsInf(p, 0) && p >= 0 {
			pos = p
		}
	}
	if !forceZero && pos <= 0 {
		return
	}
	st.LastSavedPos = pos
	st.LastSavedAt = time.Now()
	svc.SaveProgress(hongguo.HistoryEntry{
		SeriesID:  st.Series.ID,
		Title:     st.Series.Title,
		CoverURL:  st.Series.CoverURL,
		EpisodeID: st.CurrentEp.ID,
		Number:    st.CurrentEp.Number,
		Position:  pos,
		UpdatedAt: time.Now().UnixMilli(),
	})
}

func (st *State) loadFeed(x App, svc *hongguo.Service, force bool) {
	st.loadChannel(x, svc, st.ActiveChannel, force)
}

func (st *State) loadChannel(x App, svc *hongguo.Service, channel string, force bool) {
	if channel == "" {
		channel = hongguo.ChannelAll
	}
	prevCh := st.ActiveChannel
	wasSearch := st.ActiveKeyword != ""
	st.feedGen++
	gen := st.feedGen
	st.ActiveChannel = channel
	st.FeedLoading = true
	st.FeedErr = ""
	st.FeedCanReset = false
	st.ActiveKeyword = ""
	st.FeedCursor = ""
	st.SearchCursor = ""
	st.HasMore = false
	st.ListScroll.Y = 0
	if force || wasSearch || prevCh != channel {
		st.Items = nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		res, err := svc.Catalog(ctx, channel, "", nil, force)
		x.RunMain(func() {
			if st.closed || st.feedGen != gen {
				return
			}
			st.FeedLoading = false
			st.FeedLoaded = true
			if err != nil {
				st.FeedErr, st.FeedCanReset = svc.ErrorMessage(err)
				return
			}
			st.Items = res.Items
			st.FeedCursor = res.Cursor
			st.HasMore = res.HasMore
		})
	}()
}

func (st *State) runSearch(x App, svc *hongguo.Service, keyword string) {
	keyword = trimKeyword(keyword)
	if keyword == "" {
		st.SearchInput = ""
		st.loadFeed(x, svc, false)
		return
	}
	st.feedGen++
	gen := st.feedGen
	st.ActiveKeyword = keyword
	st.FeedLoading = true
	st.FeedErr = ""
	st.FeedCanReset = false
	st.Items = nil
	st.FeedCursor = ""
	st.SearchCursor = ""
	st.HasMore = false
	st.ListScroll.Y = 0

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		res, err := svc.Search(ctx, keyword, "")
		x.RunMain(func() {
			if st.closed || st.feedGen != gen {
				return
			}
			st.FeedLoading = false
			st.FeedLoaded = true
			if err != nil {
				st.FeedErr, st.FeedCanReset = svc.ErrorMessage(err)
				return
			}
			st.Items = res.Items
			st.SearchCursor = res.Cursor
			st.HasMore = res.HasMore
		})
	}()
}

func (st *State) loadMore(x App, svc *hongguo.Service) {
	if !st.HasMore || st.FeedLoading {
		return
	}
	st.feedGen++
	gen := st.feedGen
	keyword := st.ActiveKeyword
	channel := st.ActiveChannel
	feedCur := st.FeedCursor
	searchCur := st.SearchCursor
	var seen map[string]struct{}
	if keyword == "" {
		seen = make(map[string]struct{}, len(st.Items))
		for _, it := range st.Items {
			seen[it.ID] = struct{}{}
		}
	}
	st.FeedLoading = true
	st.FeedErr = ""
	st.FeedCanReset = false

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if keyword != "" {
			res, err := svc.Search(ctx, keyword, searchCur)
			x.RunMain(func() {
				if st.closed || st.feedGen != gen {
					return
				}
				st.FeedLoading = false
				if err != nil {
					st.FeedErr, st.FeedCanReset = svc.ErrorMessage(err)
					return
				}
				st.appendUniqueItems(res.Items)
				st.SearchCursor = res.Cursor
				st.HasMore = res.HasMore
			})
			return
		}
		res, err := svc.Catalog(ctx, channel, feedCur, seen, false)
		x.RunMain(func() {
			if st.closed || st.feedGen != gen {
				return
			}
			st.FeedLoading = false
			if err != nil {
				st.FeedErr, st.FeedCanReset = svc.ErrorMessage(err)
				return
			}
			st.appendUniqueItems(res.Items)
			st.FeedCursor = res.Cursor
			st.HasMore = res.HasMore
		})
	}()
}

func (st *State) appendUniqueItems(items []hongguo.Drama) {
	seen := make(map[string]bool, len(st.Items))
	for _, it := range st.Items {
		seen[it.ID] = true
	}
	for _, it := range items {
		if it.ID != "" && !seen[it.ID] {
			seen[it.ID] = true
			st.Items = append(st.Items, it)
		}
	}
}

func (st *State) openSeries(x App, svc *hongguo.Service, drama hongguo.Drama, episodeID string, resumePos float64) {
	st.saveCurrentProgress(svc, false)
	st.closePlayer()
	if st.cancelPlay != nil {
		st.cancelPlay()
		st.cancelPlay = nil
	}

	st.seriesGen++
	gen := st.seriesGen
	st.playGen++

	st.Mode = ModePlayer
	st.Series = drama
	st.Episodes = nil
	st.EpisodeChunk = 0
	st.CurrentEp = nil
	st.Definitions = nil
	st.ActiveDef = ""
	st.SeriesLoading = true
	st.SeriesErr = ""
	st.PlayLoading = false
	st.PlayStage = ""
	st.PlayErr = ""
	st.PlayCanReset = false
	st.AutoPausedReason = ""
	st.SeekPos = 0
	st.ResumeAt = 0
	st.EndedHandled = false
	st.PrefetchedID = ""

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		detail, err := svc.Series(ctx, drama.ID)
		x.RunMain(func() {
			if st.closed || st.seriesGen != gen || st.Mode != ModePlayer {
				return
			}
			st.SeriesLoading = false
			if err != nil {
				st.SeriesErr, st.PlayCanReset = svc.ErrorMessage(err)
				return
			}
			if detail.Series.Title != "" || detail.Series.CoverURL != "" {
				if detail.Series.Title == "" {
					detail.Series.Title = st.Series.Title
				}
				if detail.Series.CoverURL == "" {
					detail.Series.CoverURL = st.Series.CoverURL
				}
				st.Series = detail.Series
			}
			st.Episodes = detail.Episodes
			if len(st.Episodes) == 0 {
				st.SeriesErr = locales.HongguoNoEpisodes()
				return
			}

			var target *hongguo.Episode
			startPos := resumePos
			if episodeID != "" {
				for i := range st.Episodes {
					if st.Episodes[i].ID == episodeID && st.Episodes[i].Playable {
						target = &st.Episodes[i]
						break
					}
				}
			}
			if target == nil {
				if saved, ok := svc.HistoryOf(drama.ID); ok && saved.EpisodeID != "" {
					for i := range st.Episodes {
						if st.Episodes[i].ID == saved.EpisodeID && st.Episodes[i].Playable {
							target = &st.Episodes[i]
							if startPos <= 0 {
								startPos = saved.Position
							}
							break
						}
					}
				}
			}
			if target == nil {
				for i := range st.Episodes {
					if st.Episodes[i].Playable {
						target = &st.Episodes[i]
						startPos = 0
						break
					}
				}
			}
			if target == nil {
				st.SeriesErr = locales.HongguoNoEpisodes()
				return
			}
			st.playEpisode(x, svc, *target, startPos, "")
		})
	}()
}

func (st *State) playEpisode(x App, svc *hongguo.Service, ep hongguo.Episode, resumeAt float64, preferredDef string) {
	if !ep.Playable || ep.ID == "" {
		return
	}
	if st.cancelPlay != nil {
		st.cancelPlay()
		st.cancelPlay = nil
	}
	st.closePlayer()

	st.playGen++
	gen := st.playGen
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	st.cancelPlay = cancel

	epCopy := ep
	st.CurrentEp = &epCopy
	if idx := slices.IndexFunc(st.Episodes, func(it hongguo.Episode) bool { return it.ID == ep.ID }); idx >= 0 {
		st.EpisodeChunk = idx / episodeChunkSize
	}
	st.PlayLoading = true
	st.PlayStage = "loading"
	st.PlayErr = ""
	st.PlayCanReset = false
	st.AutoPausedReason = ""
	if resumeAt <= 0 {
		resumeAt = svc.ProgressOf(st.Series.ID, ep.ID)
	}
	st.ResumeAt = resumeAt
	st.SeekPos = resumeAt
	st.EndedHandled = false
	st.PrefetchedID = ""
	st.saveCurrentProgress(svc, true)

	go func() {
		media, err := svc.ResolvePlay(ctx, ep.ID, preferredDef, func(stage string) {
			x.RunMain(func() {
				if st.closed || st.playGen != gen {
					return
				}
				st.PlayStage = stage
			})
		})
		x.RunMain(func() {
			if st.closed || st.playGen != gen {
				return
			}
			st.PlayLoading = false
			st.PlayStage = ""
			if err != nil {
				st.PlayErr, st.PlayCanReset = svc.ErrorMessage(err)
				return
			}
			st.Definitions = media.Definitions
			st.ActiveDef = media.Definition
			player, openErr := x.OpenVideo(media.LocalPath, media.StreamURL)
			if openErr != nil {
				st.PlayErr = openErr.Error()
				return
			}
			player.SetVolume(st.Volume)
			player.SetMuted(st.Muted)
			player.SetRate(st.Rate)
			player.Play()
			st.Player = player
		})
	}()
}

func (st *State) backToList(svc *hongguo.Service) {
	st.saveCurrentProgress(svc, false)
	if st.cancelPlay != nil {
		st.cancelPlay()
		st.cancelPlay = nil
	}
	st.closePlayer()
	st.seriesGen++
	st.playGen++
	st.Mode = ModeList
	st.AutoPausedReason = ""
	st.PlayErr = ""
	st.SeriesErr = ""
}

func (st *State) togglePlayPause(x App, svc *hongguo.Service) {
	if st.Player == nil {
		if st.CurrentEp != nil && !st.PlayLoading {
			st.playEpisode(x, svc, *st.CurrentEp, st.SeekPos, st.ActiveDef)
		}
		return
	}
	if st.Player.Playing() {
		st.Player.Pause()
		st.saveCurrentProgress(svc, false)
		return
	}
	st.AutoPausedReason = ""
	if st.Player.Ended() {
		st.EndedHandled = false
		st.SeekPos = 0
	}
	st.Player.Play()
}

func (st *State) neighborPlayableEpisode(delta int) (hongguo.Episode, bool) {
	if st.CurrentEp == nil || len(st.Episodes) == 0 {
		return hongguo.Episode{}, false
	}
	idx := slices.IndexFunc(st.Episodes, func(ep hongguo.Episode) bool {
		return ep.ID == st.CurrentEp.ID
	})
	if idx < 0 {
		return hongguo.Episode{}, false
	}
	for i := idx + delta; i >= 0 && i < len(st.Episodes); i += delta {
		if st.Episodes[i].Playable {
			return st.Episodes[i], true
		}
	}
	return hongguo.Episode{}, false
}

func (st *State) stepEpisode(x App, svc *hongguo.Service, delta int) bool {
	ep, ok := st.neighborPlayableEpisode(delta)
	if !ok {
		return false
	}
	st.playEpisode(x, svc, ep, 0, st.ActiveDef)
	return true
}

func (st *State) seekBy(deltaSec float64) {
	if st.Player == nil {
		return
	}
	cur := st.Player.Position().Seconds()
	dur := st.Player.Duration().Seconds()
	next := cur + deltaSec
	if next < 0 {
		next = 0
	}
	if dur > 0 && next > dur {
		next = dur
	}
	st.SeekPos = next
	st.SeekGuardUntil = time.Now().Add(seekGuardDuration)
	st.EndedHandled = false
	st.Player.Seek(time.Duration(next * float64(time.Second)))
}

func (st *State) adjustVolume(svc *hongguo.Service, delta float64) {
	st.Volume = min(max(math.Round((st.Volume+delta)*100)/100, 0), 1)
	st.Muted = st.Volume == 0
	if st.Player != nil {
		st.Player.SetVolume(st.Volume)
		st.Player.SetMuted(st.Muted)
	}
	st.savePrefs(svc)
}

func (st *State) toggleMute(svc *hongguo.Service) {
	st.Muted = !st.Muted
	if st.Player != nil {
		st.Player.SetMuted(st.Muted)
	}
	st.savePrefs(svc)
}

func (st *State) applyRate(svc *hongguo.Service, rate float64) {
	st.Rate = rate
	if st.Player != nil {
		st.Player.SetRate(rate)
	}
	st.savePrefs(svc)
}

func (st *State) maybePrefetchNext(svc *hongguo.Service, pos, dur time.Duration) {
	next, ok := st.neighborPlayableEpisode(1)
	if !ok || st.PrefetchedID == next.ID {
		return
	}
	threshold := 10 * time.Second
	if dur > 0 && dur*3/10 < threshold {
		threshold = dur * 3 / 10
	}
	if pos < threshold {
		return
	}
	st.PrefetchedID = next.ID
	go func(epID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		svc.Prefetch(ctx, epID)
	}(next.ID)
}

func (st *State) handlePlayerShortcuts(x App, c *ui.Context, svc *hongguo.Service, root ui.Element) {
	if st.SearchIn.Focused(c) {
		return
	}
	if root.Shortcut(0, ui.KeySpace) {
		st.togglePlayPause(x, svc)
	}
	if root.Shortcut(0, ui.KeyLeft) {
		st.seekBy(-5)
	}
	if root.Shortcut(0, ui.KeyRight) {
		st.seekBy(5)
	}
	if root.Shortcut(0, ui.KeyUp) {
		st.adjustVolume(svc, 0.05)
	}
	if root.Shortcut(0, ui.KeyDown) {
		st.adjustVolume(svc, -0.05)
	}
	if root.Shortcut(0, ui.KeyM) {
		st.toggleMute(svc)
	}
	if root.Shortcut(0, ui.KeyN) {
		st.stepEpisode(x, svc, 1)
	}
	if root.Shortcut(0, ui.KeyP) {
		st.stepEpisode(x, svc, -1)
	}
	if root.Shortcut(0, ui.KeyEscape) {
		st.backToList(svc)
	}
}

func formatSeconds(sec float64) string {
	if sec < 0 || math.IsNaN(sec) || math.IsInf(sec, 0) {
		sec = 0
	}
	total := int(sec + 0.5)
	m, s := total/60, total%60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func trimKeyword(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
