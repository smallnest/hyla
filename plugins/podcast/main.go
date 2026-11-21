package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

const (
	pluginID = "hyla.podcast"
	slotID   = "podcast"
)

type JobStatus struct {
	ID       string              `json:"id"`
	Kind     string              `json:"kind"` // "download" | "translate" | "transcribe" | "guide" | "refresh"
	TargetID string              `json:"targetId,omitempty"`
	State    string              `json:"state"` // "running" | "done" | "error"
	Progress int                 `json:"progress"`
	Message  string              `json:"message,omitempty"`
	Error    string              `json:"error,omitempty"`
	Segments []TranscriptSegment `json:"segments,omitempty"`
	Guide    *AIGuide            `json:"guide,omitempty"`
	Episode  *Episode            `json:"episode,omitempty"`
	Storage  *StorageInfo        `json:"storage,omitempty"`
}

type podcastService struct {
	mu           sync.RWMutex
	host         *hylaplugin.Host
	store        *Store
	media        *MediaServer
	locale       string
	cfg          AIConfig
	workingPanes map[string]bool
	jobs         map[string]JobStatus
	jobSeq       int
	bgCtx        context.Context
	bgCancel     context.CancelFunc

	// Overridable base URLs for unit tests.
	itunesBaseURL     string
	googleTransBase   string
	dictionaryAPIBase string
}

func newPodcastService() *podcastService {
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &podcastService{
		locale:       "en",
		cfg:          buildAIConfig(nil, nil, nil),
		workingPanes: map[string]bool{},
		jobs:         map[string]JobStatus{},
		bgCtx:        bgCtx,
		bgCancel:     bgCancel,
	}
}

func (s *podcastService) shutdown() {
	s.bgCancel()
	s.mu.Lock()
	ms := s.media
	s.media = nil
	s.mu.Unlock()
	if ms != nil {
		ms.Close()
	}
}

func (s *podcastService) initRuntime(ctx context.Context, host *hylaplugin.Host, dataDir, locale string, settings map[string]any) error {
	if strings.TrimSpace(locale) != "" {
		s.setLocale(locale)
	}
	if strings.TrimSpace(dataDir) == "" {
		var err error
		dataDir, err = os.MkdirTemp("", "hyla-podcast-data-*")
		if err != nil {
			return err
		}
	}
	store, err := NewStore(dataDir)
	if err != nil {
		return err
	}
	ms, err := NewMediaServer(store, func() string {
		return s.getConfig().HTTPProxy
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	oldMedia := s.media
	s.host = host
	s.store = store
	s.media = ms
	s.cfg = buildAIConfig(settings, nil, nil)
	s.mu.Unlock()
	if oldMedia != nil {
		oldMedia.Close()
	}

	// Best-effort initial settings & secrets fetch; do not fail initialize if secrets permission is ungranted.
	go func() {
		rctx, cancel := context.WithTimeout(s.bgCtx, 5*time.Second)
		defer cancel()
		s.reloadSettings(rctx)
	}()
	return nil
}

func (s *podcastService) ensureStore() (*Store, *MediaServer, error) {
	s.mu.RLock()
	st, ms := s.store, s.media
	s.mu.RUnlock()
	if st != nil {
		return st, ms, nil
	}
	if err := s.initRuntime(context.Background(), s.host, "", s.locale, nil); err != nil {
		return nil, nil, err
	}
	s.mu.RLock()
	st, ms = s.store, s.media
	s.mu.RUnlock()
	return st, ms, nil
}

func (s *podcastService) setLocale(loc string) string {
	norm := normalizeLocale(loc)
	s.mu.Lock()
	s.locale = norm
	s.mu.Unlock()
	return norm
}

func (s *podcastService) resolveLocale(reqLoc string) string {
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

func (s *podcastService) getConfig() AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *podcastService) reloadSettings(ctx context.Context) {
	s.mu.RLock()
	h := s.host
	s.mu.RUnlock()
	if h == nil {
		return
	}
	res, err := h.GetSettings(ctx, true)
	if err != nil {
		res, err = h.GetSettings(ctx, false)
	}
	if err != nil || res == nil {
		return
	}
	cfg := buildAIConfig(res.Settings, res.Secrets, res.SecretsSet)
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

func (s *podcastService) httpClient(timeout time.Duration) *http.Client {
	return httpClient(s.getConfig().HTTPProxy, timeout)
}

func (s *podcastService) emitEvent(event string, data any) {
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

func (s *podcastService) startJob(kind, targetID, msg string) JobStatus {
	s.mu.Lock()
	s.jobSeq++
	id := fmt.Sprintf("job-%d", s.jobSeq)
	job := JobStatus{
		ID:       id,
		Kind:     kind,
		TargetID: targetID,
		State:    "running",
		Progress: 0,
		Message:  msg,
	}
	s.jobs[id] = job
	s.mu.Unlock()
	s.emitEvent("podcast.job", job)
	return job
}

func (s *podcastService) updateJob(job JobStatus) {
	s.mu.Lock()
	if job.State == "done" || job.State == "error" {
		delete(s.jobs, job.ID)
	} else {
		s.jobs[job.ID] = job
	}
	s.mu.Unlock()
	s.emitEvent("podcast.job", job)
}

func (s *podcastService) activeJobsList() []JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]JobStatus, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

func (s *podcastService) enrichFeed(ms *MediaServer, f Feed) Feed {
	if ms != nil && f.ArtworkURL != "" {
		f.CoverURL = ms.RegisterCover(f.ArtworkURL)
	}
	return f
}

func (s *podcastService) enrichEpisode(ms *MediaServer, ep Episode) Episode {
	if ms != nil {
		ep.PlayURL = ms.PlayURL(ep.ID)
		if ep.ImageURL != "" {
			ep.CoverURL = ms.RegisterCover(ep.ImageURL)
		}
	}
	return ep
}

func (s *podcastService) enrichDiscover(ms *MediaServer, list []DiscoverItem) []DiscoverItem {
	out := make([]DiscoverItem, len(list))
	copy(out, list)
	for i := range out {
		if ms != nil && out[i].ArtworkURL != "" {
			out[i].CoverURL = ms.RegisterCover(out[i].ArtworkURL)
		}
	}
	return out
}

func (s *podcastService) handleAction(ctx context.Context, host *hylaplugin.Host, p hylaplugin.ActionInvokeParams) (hylaplugin.ActionInvokeResult, error) {
	if p.ActionID != "" && p.ActionID != "open" && p.ActionID != pluginID+".open" {
		return hylaplugin.ActionInvokeResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown action: "+p.ActionID)
	}
	loc := s.resolveLocale(p.Locale)
	st, err := host.GetState(ctx)
	if err != nil {
		return hylaplugin.ActionInvokeResult{}, err
	}
	sess := findSession(st, p.SessionID, p.PaneID)
	var sessID, splitID string
	if sess != nil {
		sessID = sess.ID
		splitID = p.PaneID
		if splitID == "" {
			splitID = sess.Focus
		}
		if splitID == "" {
			splitID = st.FocusedPane
		}
	}
	if err := host.OpenPane(ctx, hylaplugin.UIOpenPaneParams{
		Session:       sessID,
		Split:         splitID,
		Plugin:        pluginID,
		View:          slotID,
		Title:         localizedPodcastTitle(loc),
		Direction:     "right",
		FocusExisting: true,
	}); err != nil {
		return hylaplugin.ActionInvokeResult{}, err
	}
	return hylaplugin.ActionInvokeResult{OK: true}, nil
}

func (s *podcastService) handleWebRPC(ctx context.Context, host *hylaplugin.Host, p hylaplugin.WebViewRPCParams) (hylaplugin.WebViewRPCResult, error) {
	loc := s.resolveLocale(p.Locale)
	st, ms, err := s.ensureStore()
	if err != nil {
		return hylaplugin.WebViewRPCResult{}, err
	}

	var data any
	switch p.Method {
	case "podcast.bootstrap":
		data = s.rpcBootstrap(ctx, st, ms, loc)

	case "podcast.search":
		var req struct {
			Query   string `json:"query"`
			Country string `json:"country"`
		}
		_ = json.Unmarshal(p.Params, &req)
		results, rerr := SearchITunes(ctx, s.httpClient(20*time.Second), s.itunesBaseURL, req.Query, req.Country)
		if rerr != nil {
			return hylaplugin.WebViewRPCResult{}, rerr
		}
		for i := range results {
			if results[i].ArtworkURL != "" && ms != nil {
				results[i].CoverURL = ms.RegisterCover(results[i].ArtworkURL)
			}
		}
		data = map[string]any{"results": results}

	case "podcast.subscribe":
		var req struct {
			Input      string `json:"input"`
			DiscoverID string `json:"discoverId"`
			Category   string `json:"category"`
		}
		_ = json.Unmarshal(p.Params, &req)
		feed, eps, serr := s.subscribe(ctx, st, req.Input, req.DiscoverID, req.Category)
		if serr != nil {
			return hylaplugin.WebViewRPCResult{}, serr
		}
		outEps := make([]Episode, len(eps))
		for i, ep := range eps {
			outEps[i] = s.enrichEpisode(ms, ep)
		}
		data = map[string]any{
			"feed":     s.enrichFeed(ms, feed),
			"episodes": outEps,
		}

	case "podcast.unsubscribe":
		var req struct {
			FeedID string `json:"feedId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		if rerr := st.RemoveFeed(req.FeedID); rerr != nil {
			return hylaplugin.WebViewRPCResult{}, rerr
		}
		data = s.rpcBootstrap(ctx, st, ms, loc)

	case "podcast.refreshFeeds":
		var req struct {
			FeedID string `json:"feedId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		if req.FeedID != "" {
			f, ok := st.GetFeed(req.FeedID)
			if !ok {
				return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "feed not found")
			}
			updated, eps, rerr := s.refreshSingleFeed(ctx, st, f)
			if rerr != nil {
				return hylaplugin.WebViewRPCResult{}, rerr
			}
			outEps := make([]Episode, len(eps))
			for i, ep := range eps {
				outEps[i] = s.enrichEpisode(ms, ep)
			}
			data = map[string]any{
				"feed":     s.enrichFeed(ms, updated),
				"episodes": outEps,
			}
		} else {
			job := s.startJob("refresh", "", "Refreshing feeds…")
			go s.runRefreshAllJob(job, st)
			data = map[string]any{"job": job}
		}

	case "podcast.importOPML":
		var req struct {
			XML string `json:"xml"`
		}
		_ = json.Unmarshal(p.Params, &req)
		entries, perr := ParseOPML([]byte(req.XML))
		if perr != nil {
			return hylaplugin.WebViewRPCResult{}, perr
		}
		added := 0
		var failed []string
		for _, ent := range entries {
			if _, _, serr := s.subscribe(ctx, st, ent.FeedURL, "", ""); serr != nil {
				failed = append(failed, ent.FeedURL)
			} else {
				added++
			}
		}
		data = map[string]any{
			"added":     added,
			"total":     len(entries),
			"failed":    failed,
			"bootstrap": s.rpcBootstrap(ctx, st, ms, loc),
		}

	case "podcast.exportOPML":
		feeds, _, _, _, _ := st.Snapshot()
		data = map[string]any{"opml": ExportOPML(feeds)}

	case "podcast.episodeDetail":
		var req struct {
			EpisodeID string `json:"episodeId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, det, derr := s.loadEpisodeDetail(ctx, st, req.EpisodeID)
		if derr != nil {
			return hylaplugin.WebViewRPCResult{}, derr
		}
		data = map[string]any{
			"episode": s.enrichEpisode(ms, ep),
			"detail":  det,
		}

	case "podcast.saveProgress":
		var req struct {
			EpisodeID      string  `json:"episodeId"`
			PositionSec    float64 `json:"positionSec"`
			DurationSec    float64 `json:"durationSec"`
			Completed      bool    `json:"completed"`
			PlayedDeltaSec float64 `json:"playedDeltaSec"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, stats, perr := st.SaveProgress(req.EpisodeID, req.PositionSec, req.DurationSec, req.Completed, req.PlayedDeltaSec)
		if perr != nil {
			return hylaplugin.WebViewRPCResult{}, perr
		}
		data = map[string]any{
			"episode": s.enrichEpisode(ms, ep),
			"stats":   stats,
		}

	case "podcast.toggleFavorite":
		var req struct {
			EpisodeID string `json:"episodeId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, terr := st.ToggleFavorite(req.EpisodeID)
		if terr != nil {
			return hylaplugin.WebViewRPCResult{}, terr
		}
		data = map[string]any{"episode": s.enrichEpisode(ms, ep)}

	case "podcast.queueSet":
		var req struct {
			Queue []string `json:"queue"`
		}
		_ = json.Unmarshal(p.Params, &req)
		q, qerr := st.SetQueue(req.Queue)
		if qerr != nil {
			return hylaplugin.WebViewRPCResult{}, qerr
		}
		data = map[string]any{"queue": q}

	case "podcast.downloadEpisode":
		var req struct {
			EpisodeID string `json:"episodeId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, ok := st.GetEpisode(req.EpisodeID)
		if !ok {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
		}
		job := s.startJob("download", ep.ID, "Downloading episode…")
		go s.runDownloadJob(job, st, ms, ep)
		data = map[string]any{"job": job}

	case "podcast.deleteDownload":
		var req struct {
			EpisodeID string `json:"episodeId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, ok := st.GetEpisode(req.EpisodeID)
		if !ok {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
		}
		if ep.DownloadedFile != "" {
			_ = os.Remove(st.DownloadPath(ep.DownloadedFile))
		}
		updated, derr := st.SetDownload(ep.ID, "", 0)
		if derr != nil {
			return hylaplugin.WebViewRPCResult{}, derr
		}
		data = map[string]any{
			"episode": s.enrichEpisode(ms, updated),
			"storage": st.StorageUsage(),
		}

	case "podcast.clearCache":
		info, cerr := st.ClearDownloadsAndCovers()
		if cerr != nil {
			return hylaplugin.WebViewRPCResult{}, cerr
		}
		data = map[string]any{
			"storage":   info,
			"bootstrap": s.rpcBootstrap(ctx, st, ms, loc),
		}

	case "podcast.importTranscript":
		var req struct {
			EpisodeID string `json:"episodeId"`
			Content   string `json:"content"`
			Format    string `json:"format"`
		}
		_ = json.Unmarshal(p.Params, &req)
		segs := ParseTranscriptAuto([]byte(req.Content), req.Format, "")
		if len(segs) == 0 {
			return hylaplugin.WebViewRPCResult{}, fmt.Errorf("no transcript segments found in pasted text")
		}
		det, uerr := st.UpdateEpisodeDetail(req.EpisodeID, func(ep *Episode, det *EpisodeDetail) {
			det.Segments = segs
			det.TranscriptSource = "imported"
		})
		if uerr != nil {
			return hylaplugin.WebViewRPCResult{}, uerr
		}
		data = map[string]any{"detail": det}

	case "podcast.translateTranscript":
		var req struct {
			EpisodeID  string `json:"episodeId"`
			TargetLang string `json:"targetLang"`
			Force      bool   `json:"force"`
		}
		_ = json.Unmarshal(p.Params, &req)
		if _, ok := st.GetEpisode(req.EpisodeID); !ok {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
		}
		job := s.startJob("translate", req.EpisodeID, "Translating transcript…")
		go s.runTranslateJob(job, st, req.EpisodeID, req.TargetLang, req.Force)
		data = map[string]any{"job": job}

	case "podcast.transcribeEpisode":
		var req struct {
			EpisodeID string `json:"episodeId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, ok := st.GetEpisode(req.EpisodeID)
		if !ok {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
		}
		job := s.startJob("transcribe", ep.ID, "Transcribing audio…")
		go s.runTranscribeJob(job, st, ep)
		data = map[string]any{"job": job}

	case "podcast.exportTranscript":
		var req struct {
			EpisodeID string `json:"episodeId"`
			Format    string `json:"format"`
			Mode      string `json:"mode"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, ok := st.GetEpisode(req.EpisodeID)
		if !ok {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
		}
		det, derr := st.LoadDetail(ep.ID)
		if derr != nil {
			return hylaplugin.WebViewRPCResult{}, derr
		}
		data = map[string]any{
			"content": ExportTranscript(ep, det.Segments, req.Format, req.Mode),
		}

	case "podcast.aiGuide":
		var req struct {
			EpisodeID string `json:"episodeId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		ep, ok := st.GetEpisode(req.EpisodeID)
		if !ok {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
		}
		job := s.startJob("guide", ep.ID, "Generating AI guide…")
		go s.runGuideJob(job, st, ep, loc)
		data = map[string]any{"job": job}

	case "podcast.aiAsk":
		var req struct {
			EpisodeID string `json:"episodeId"`
			Question  string `json:"question"`
		}
		_ = json.Unmarshal(p.Params, &req)
		if strings.TrimSpace(req.Question) == "" {
			return hylaplugin.WebViewRPCResult{}, fmt.Errorf("question must not be empty")
		}
		ep, ok := st.GetEpisode(req.EpisodeID)
		if !ok {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
		}
		det, derr := st.LoadDetail(ep.ID)
		if derr != nil {
			return hylaplugin.WebViewRPCResult{}, derr
		}
		ans, aerr := AskEpisode(ctx, s.httpClient(90*time.Second), s.getConfig(), ep, det.Segments, det.Chat, req.Question, loc)
		if aerr != nil {
			return hylaplugin.WebViewRPCResult{}, aerr
		}
		now := time.Now().UnixMilli()
		updatedDet, uerr := st.UpdateEpisodeDetail(ep.ID, func(_ *Episode, d *EpisodeDetail) {
			d.Chat = append(d.Chat,
				ChatMessage{Role: "user", Content: strings.TrimSpace(req.Question), CreatedAt: now},
				ChatMessage{Role: "assistant", Content: ans, CreatedAt: time.Now().UnixMilli()},
			)
		})
		if uerr != nil {
			return hylaplugin.WebViewRPCResult{}, uerr
		}
		data = map[string]any{"chat": updatedDet.Chat}

	case "podcast.aiClearChat":
		var req struct {
			EpisodeID string `json:"episodeId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		updatedDet, uerr := st.UpdateEpisodeDetail(req.EpisodeID, func(_ *Episode, d *EpisodeDetail) {
			d.Chat = nil
		})
		if uerr != nil {
			return hylaplugin.WebViewRPCResult{}, uerr
		}
		data = map[string]any{"chat": updatedDet.Chat}

	case "podcast.wordLookup":
		var req struct {
			Word       string `json:"word"`
			Sentence   string `json:"sentence"`
			TargetLang string `json:"targetLang"`
		}
		_ = json.Unmarshal(p.Params, &req)
		res, werr := LookupWord(ctx, s.httpClient(15*time.Second), s.getConfig(), s.dictionaryAPIBase, s.googleTransBase, req.Word, req.Sentence, req.TargetLang)
		if werr != nil {
			return hylaplugin.WebViewRPCResult{}, werr
		}
		data = res

	case "podcast.vocabAdd":
		var req VocabEntry
		_ = json.Unmarshal(p.Params, &req)
		list, verr := st.AddVocab(req)
		if verr != nil {
			return hylaplugin.WebViewRPCResult{}, verr
		}
		data = map[string]any{"vocab": list}

	case "podcast.vocabDelete":
		var req struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(p.Params, &req)
		list, verr := st.DeleteVocab(req.ID)
		if verr != nil {
			return hylaplugin.WebViewRPCResult{}, verr
		}
		data = map[string]any{"vocab": list}

	case "podcast.vocabReview":
		var req struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(p.Params, &req)
		list, verr := st.ReviewVocab(req.ID)
		if verr != nil {
			return hylaplugin.WebViewRPCResult{}, verr
		}
		data = map[string]any{"vocab": list}

	case "podcast.vocabExportCSV":
		_, _, vocab, _, _ := st.Snapshot()
		data = map[string]any{"csv": exportVocabCSV(vocab)}

	case "podcast.saveSettings":
		var req struct {
			AutoPauseOnAgent *bool   `json:"autoPauseOnAgent"`
			AIProvider       *string `json:"aiProvider"`
			AIBaseURL        *string `json:"aiBaseUrl"`
			AIModel          *string `json:"aiModel"`
			AIAPIKey         *string `json:"aiApiKey"`
			TranslateEngine  *string `json:"translateEngine"`
			ASRBaseURL       *string `json:"asrBaseUrl"`
			ASRModel         *string `json:"asrModel"`
			ASRAPIKey        *string `json:"asrApiKey"`
			HTTPProxy        *string `json:"httpProxy"`
		}
		_ = json.Unmarshal(p.Params, &req)
		settingsMap := map[string]any{}
		secretsMap := map[string]string{}
		if req.AutoPauseOnAgent != nil {
			settingsMap["auto_pause_on_agent"] = *req.AutoPauseOnAgent
		}
		if req.AIProvider != nil {
			settingsMap["ai_provider"] = strings.TrimSpace(*req.AIProvider)
		}
		if req.AIBaseURL != nil {
			settingsMap["ai_base_url"] = strings.TrimSpace(*req.AIBaseURL)
		}
		if req.AIModel != nil {
			settingsMap["ai_model"] = strings.TrimSpace(*req.AIModel)
		}
		if req.TranslateEngine != nil {
			settingsMap["translate_engine"] = strings.TrimSpace(*req.TranslateEngine)
		}
		if req.ASRBaseURL != nil {
			settingsMap["asr_base_url"] = strings.TrimSpace(*req.ASRBaseURL)
		}
		if req.ASRModel != nil {
			settingsMap["asr_model"] = strings.TrimSpace(*req.ASRModel)
		}
		if req.HTTPProxy != nil {
			settingsMap["http_proxy"] = strings.TrimSpace(*req.HTTPProxy)
		}
		if req.AIAPIKey != nil && strings.TrimSpace(*req.AIAPIKey) != "" {
			secretsMap["ai_api_key"] = strings.TrimSpace(*req.AIAPIKey)
		}
		if req.ASRAPIKey != nil && strings.TrimSpace(*req.ASRAPIKey) != "" {
			secretsMap["asr_api_key"] = strings.TrimSpace(*req.ASRAPIKey)
		}
		if len(settingsMap) > 0 || len(secretsMap) > 0 {
			if serr := host.SetSettings(ctx, settingsMap, secretsMap); serr != nil {
				return hylaplugin.WebViewRPCResult{}, serr
			}
			s.reloadSettings(ctx)
		}
		data = map[string]any{"settings": s.getConfig()}

	default:
		return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown method: "+p.Method)
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return hylaplugin.WebViewRPCResult{}, err
	}
	return hylaplugin.WebViewRPCResult{Data: raw}, nil
}

func (s *podcastService) rpcBootstrap(ctx context.Context, st *Store, ms *MediaServer, loc string) map[string]any {
	feeds, eps, vocab, stats, queue := st.Snapshot()
	outFeeds := make([]Feed, len(feeds))
	for i, f := range feeds {
		outFeeds[i] = s.enrichFeed(ms, f)
	}
	outEps := make([]Episode, len(eps))
	for i, ep := range eps {
		outEps[i] = s.enrichEpisode(ms, ep)
	}
	disc := s.enrichDiscover(ms, curatedDiscoverList())
	return map[string]any{
		"locale":       loc,
		"mediaBaseUrl": ms.BaseURL(),
		"feeds":        outFeeds,
		"episodes":     outEps,
		"queue":        queue,
		"vocab":        vocab,
		"stats":        stats,
		"discover":     disc,
		"settings":     s.getConfig(),
		"activeJobs":   s.activeJobsList(),
		"storage":      st.StorageUsage(),
	}
}

func (s *podcastService) subscribe(ctx context.Context, st *Store, rawInput, discoverID, category string) (Feed, []Episode, error) {
	input := strings.TrimSpace(rawInput)
	var discItem *DiscoverItem
	if discoverID != "" {
		for _, item := range curatedDiscoverList() {
			if item.ID == discoverID {
				cp := item
				discItem = &cp
				break
			}
		}
		if discItem == nil {
			return Feed{}, nil, fmt.Errorf("discover entry %q not found", discoverID)
		}
		if input == "" {
			input = discItem.FeedURL
		}
		if category == "" {
			category = discItem.Category
		}
	}
	if input == "" {
		return Feed{}, nil, fmt.Errorf("subscription URL is empty")
	}

	client := s.httpClient(30 * time.Second)
	feed, eps, err := ResolveAndFetchInput(ctx, client, s.itunesBaseURL, input)
	if err != nil && discItem != nil && discItem.ITunesID != "" {
		if fbURL, art, lerr := LookupITunesFeed(ctx, client, s.itunesBaseURL, discItem.ITunesID); lerr == nil {
			if rf, reps, rerr := fetchAndParseFeed(ctx, client, fbURL, "apple"); rerr == nil {
				feed, eps, err = rf, reps, nil
				if feed.ArtworkURL == "" && art != "" {
					feed.ArtworkURL = art
				}
			}
		}
	}
	if err != nil {
		return Feed{}, nil, err
	}
	if category != "" {
		feed.Category = category
	}
	if discItem != nil && feed.ArtworkURL == "" && discItem.ArtworkURL != "" {
		feed.ArtworkURL = discItem.ArtworkURL
	}
	return st.UpsertFeedAndEpisodes(feed, eps)
}

func (s *podcastService) refreshSingleFeed(ctx context.Context, st *Store, f Feed) (Feed, []Episode, error) {
	if f.SourceType == "direct" || strings.HasPrefix(f.FeedURL, "direct://") {
		return f, nil, nil
	}
	client := s.httpClient(30 * time.Second)
	fetched, eps, err := ResolveAndFetchInput(ctx, client, s.itunesBaseURL, f.FeedURL)
	if err != nil {
		return Feed{}, nil, err
	}
	fetched.ID = f.ID
	fetched.SubscribedAt = f.SubscribedAt
	if fetched.Category == "" {
		fetched.Category = f.Category
	}
	if fetched.SourceType == "" {
		fetched.SourceType = f.SourceType
	}
	return st.UpsertFeedAndEpisodes(fetched, eps)
}

func (s *podcastService) loadEpisodeDetail(ctx context.Context, st *Store, epID string) (Episode, EpisodeDetail, error) {
	ep, ok := st.GetEpisode(epID)
	if !ok {
		return Episode{}, EpisodeDetail{}, hylaplugin.NewError(hylaplugin.ErrCodeTargetNotFound, "episode not found")
	}
	det, err := st.LoadDetail(epID)
	if err != nil {
		return Episode{}, EpisodeDetail{}, err
	}

	needSave := false
	var fetchedSegs []TranscriptSegment
	var fetchedSource string
	var fetchedChapters []Chapter

	if len(det.Segments) == 0 && ep.TranscriptURL != "" {
		tctx, cancel := context.WithTimeout(ctx, 12*time.Second)
		b, _, terr := fetchBytes(tctx, s.httpClient(12*time.Second), ep.TranscriptURL, 8<<20)
		cancel()
		if terr == nil {
			if segs := ParseTranscriptAuto(b, ep.TranscriptType, ep.TranscriptURL); len(segs) > 0 {
				fetchedSegs = segs
				fetchedSource = "rss"
				needSave = true
			}
		} else {
			log.Printf("fetch transcript %s: %v", ep.TranscriptURL, terr)
		}
	}

	if len(ep.Chapters) == 0 && len(det.Chapters) == 0 && ep.ChaptersURL != "" {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		b, _, cerr := fetchBytes(cctx, s.httpClient(10*time.Second), ep.ChaptersURL, 2<<20)
		cancel()
		if cerr == nil {
			if ch, perr := ParsePodcastChaptersJSON(b); perr == nil && len(ch) > 0 {
				fetchedChapters = ch
				needSave = true
			}
		}
	}

	if needSave {
		det, err = st.UpdateEpisodeDetail(epID, func(_ *Episode, d *EpisodeDetail) {
			if len(fetchedSegs) > 0 {
				d.Segments = fetchedSegs
				d.TranscriptSource = fetchedSource
			}
			if len(fetchedChapters) > 0 {
				d.Chapters = fetchedChapters
			}
		})
		if err != nil {
			return Episode{}, EpisodeDetail{}, err
		}
		if refreshed, ok := st.GetEpisode(epID); ok {
			ep = refreshed
		}
	}
	if len(det.Chapters) == 0 && len(ep.Chapters) > 0 {
		det.Chapters = ep.Chapters
	}
	return ep, det, nil
}

func (s *podcastService) runRefreshAllJob(job JobStatus, st *Store) {
	feeds, _, _, _, _ := st.Snapshot()
	if len(feeds) == 0 {
		job.State = "done"
		job.Progress = 100
		s.updateJob(job)
		return
	}
	ctx, cancel := context.WithTimeout(s.bgCtx, 3*time.Minute)
	defer cancel()

	for i, f := range feeds {
		if ctx.Err() != nil {
			job.State = "error"
			job.Error = ctx.Err().Error()
			s.updateJob(job)
			return
		}
		job.Message = f.Title
		job.Progress = (i * 100) / len(feeds)
		s.updateJob(job)
		if _, _, err := s.refreshSingleFeed(ctx, st, f); err != nil {
			log.Printf("refresh feed %s: %v", f.Title, err)
		}
	}
	job.State = "done"
	job.Progress = 100
	job.Message = ""
	s.updateJob(job)
}

func (s *podcastService) runDownloadJob(job JobStatus, st *Store, ms *MediaServer, ep Episode) {
	ctx, cancel := context.WithTimeout(s.bgCtx, 30*time.Minute)
	defer cancel()

	fileName, size, err := s.downloadAudioTo(ctx, st.downloadsDir(), ep, func(pct int) {
		job.Progress = pct
		s.updateJob(job)
	})
	if err != nil {
		job.State = "error"
		job.Error = err.Error()
		s.updateJob(job)
		return
	}

	updated, serr := st.SetDownload(ep.ID, fileName, size)
	if serr != nil {
		job.State = "error"
		job.Error = serr.Error()
		s.updateJob(job)
		return
	}
	enriched := s.enrichEpisode(ms, updated)
	storage := st.StorageUsage()
	job.State = "done"
	job.Progress = 100
	job.Episode = &enriched
	job.Storage = &storage
	s.updateJob(job)
}

func (s *podcastService) downloadAudioTo(ctx context.Context, dir string, ep Episode, onPct func(int)) (string, int64, error) {
	u, err := validateHTTPURL(ep.AudioURL)
	if err != nil {
		return "", 0, err
	}
	ext := strings.ToLower(path.Ext(u.Path))
	if ext == "" || len(ext) > 6 {
		ext = ".mp3"
	}
	finalName := ep.ID + ext
	finalPath := filepath.Join(dir, finalName)

	req, err := newHTTPRequest(ctx, http.MethodGet, ep.AudioURL, nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := s.httpClient(0).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("download HTTP %s", resp.Status)
	}

	tmp, err := os.CreateTemp(dir, ".dl-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32<<10)
	lastEmit := time.Now()
	lastPct := -1

	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				return "", 0, werr
			}
			written += int64(n)
			if total > 0 && onPct != nil {
				pct := int((written * 100) / total)
				if pct > 99 {
					pct = 99
				}
				if pct != lastPct && time.Since(lastEmit) >= 250*time.Millisecond {
					lastPct = pct
					lastEmit = time.Now()
					onPct(pct)
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", 0, rerr
		}
	}
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", 0, err
	}
	cleanup = false
	return finalName, written, nil
}

func (s *podcastService) runTranslateJob(job JobStatus, st *Store, epID, targetLang string, force bool) {
	ctx, cancel := context.WithTimeout(s.bgCtx, 10*time.Minute)
	defer cancel()

	det, err := st.LoadDetail(epID)
	if err != nil {
		job.State = "error"
		job.Error = err.Error()
		s.updateJob(job)
		return
	}
	if len(det.Segments) == 0 {
		job.State = "error"
		job.Error = "no transcript segments to translate"
		s.updateJob(job)
		return
	}

	cfg := s.getConfig()
	batchCount := 0
	lastPersist := time.Now()
	translated, err := TranslateSegmentsProgressive(ctx, s.httpClient(60*time.Second), cfg, s.googleTransBase, det.Segments, targetLang, force, func(done, total int, partial []TranscriptSegment) {
		batchCount++
		if total > 0 {
			job.Progress = (done * 100) / total
			job.Message = fmt.Sprintf("Translating %d/%d…", done, total)
		}
		if partial != nil && (batchCount == 1 || done == total || batchCount%3 == 0 || time.Since(lastPersist) >= 2*time.Second) {
			if _, serr := st.UpdateEpisodeDetail(epID, func(_ *Episode, d *EpisodeDetail) {
				d.Segments = partial
			}); serr == nil {
				job.Segments = partial
				lastPersist = time.Now()
			}
		} else {
			job.Segments = nil
		}
		s.updateJob(job)
	})
	if err != nil {
		job.Segments = nil
		job.State = "error"
		job.Error = err.Error()
		s.updateJob(job)
		return
	}

	updated, uerr := st.UpdateEpisodeDetail(epID, func(_ *Episode, d *EpisodeDetail) {
		d.Segments = translated
	})
	if uerr != nil {
		job.Segments = nil
		job.State = "error"
		job.Error = uerr.Error()
		s.updateJob(job)
		return
	}
	job.State = "done"
	job.Progress = 100
	job.Message = ""
	job.Segments = updated.Segments
	s.updateJob(job)
}

func (s *podcastService) runTranscribeJob(job JobStatus, st *Store, ep Episode) {
	ctx, cancel := context.WithTimeout(s.bgCtx, 25*time.Minute)
	defer cancel()

	audioPath := ""
	if ep.DownloadedFile != "" {
		cand := st.DownloadPath(ep.DownloadedFile)
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			audioPath = cand
		}
	}
	if audioPath == "" {
		tmpDir, err := os.MkdirTemp("", "hyla-podcast-tr-*")
		if err != nil {
			job.State = "error"
			job.Error = err.Error()
			s.updateJob(job)
			return
		}
		defer os.RemoveAll(tmpDir)
		job.Message = "Downloading audio for transcription…"
		s.updateJob(job)
		fn, _, err := s.downloadAudioTo(ctx, tmpDir, ep, func(pct int) {
			job.Progress = pct / 4
			s.updateJob(job)
		})
		if err != nil {
			job.State = "error"
			job.Error = err.Error()
			s.updateJob(job)
			return
		}
		audioPath = filepath.Join(tmpDir, fn)
	}

	segs, err := TranscribeWithCloudASR(ctx, s.httpClient(5*time.Minute), s.getConfig(), audioPath, func(pct int, msg string) {
		job.Progress = pct
		job.Message = msg
		s.updateJob(job)
	})
	if err != nil {
		job.State = "error"
		job.Error = err.Error()
		s.updateJob(job)
		return
	}

	updated, uerr := st.UpdateEpisodeDetail(ep.ID, func(_ *Episode, d *EpisodeDetail) {
		d.Segments = segs
		d.TranscriptSource = "cloud_asr"
	})
	if uerr != nil {
		job.State = "error"
		job.Error = uerr.Error()
		s.updateJob(job)
		return
	}
	job.State = "done"
	job.Progress = 100
	job.Segments = updated.Segments
	s.updateJob(job)
}

func (s *podcastService) runGuideJob(job JobStatus, st *Store, ep Episode, locale string) {
	ctx, cancel := context.WithTimeout(s.bgCtx, 2*time.Minute)
	defer cancel()

	det, err := st.LoadDetail(ep.ID)
	if err != nil {
		job.State = "error"
		job.Error = err.Error()
		s.updateJob(job)
		return
	}
	guide, err := GenerateGuide(ctx, s.httpClient(90*time.Second), s.getConfig(), ep, det.Segments, locale)
	if err != nil {
		job.State = "error"
		job.Error = err.Error()
		s.updateJob(job)
		return
	}
	updated, uerr := st.UpdateEpisodeDetail(ep.ID, func(_ *Episode, d *EpisodeDetail) {
		d.Guide = guide
	})
	if uerr != nil {
		job.State = "error"
		job.Error = uerr.Error()
		s.updateJob(job)
		return
	}
	job.State = "done"
	job.Progress = 100
	job.Guide = updated.Guide
	s.updateJob(job)
}

func (s *podcastService) handleNotify(ctx context.Context, host *hylaplugin.Host, ev hylaplugin.EventNotifyParams) {
	switch ev.Topic {
	case "locale.changed":
		var data struct {
			Locale string `json:"locale"`
		}
		if json.Unmarshal(ev.Data, &data) == nil && strings.TrimSpace(data.Locale) != "" {
			s.setLocale(data.Locale)
		}
	case "settings.changed":
		s.reloadSettings(ctx)
		s.emitEvent("podcast.settingsChanged", map[string]any{"settings": s.getConfig()})
	case "pane.status":
		var data struct {
			SessionID string `json:"sessionId"`
			PaneID    string `json:"paneId"`
			Status    string `json:"status"`
			Agent     string `json:"agent"`
		}
		if json.Unmarshal(ev.Data, &data) != nil || data.PaneID == "" {
			return
		}
		isWorking := data.Status == string(hylaplugin.StatusWorking)
		s.mu.Lock()
		wasWorking := s.workingPanes[data.PaneID]
		if isWorking {
			s.workingPanes[data.PaneID] = true
		} else {
			delete(s.workingPanes, data.PaneID)
		}
		autoPause := s.cfg.AutoPauseOnAgent
		s.mu.Unlock()

		if autoPause && wasWorking && !isWorking {
			s.emitEvent("podcast.autoPause", map[string]any{
				"sessionId": data.SessionID,
				"paneId":    data.PaneID,
				"agent":     data.Agent,
				"status":    data.Status,
			})
		}
	}
}

func exportVocabCSV(vocab []VocabEntry) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"Word", "Phonetic", "Meaning", "Sentence", "Translation", "Episode", "Timestamp", "Reviews"})
	for _, v := range vocab {
		_ = w.Write([]string{
			v.Word,
			v.Phonetic,
			v.Meaning,
			v.Sentence,
			v.Translation,
			v.EpisodeTitle,
			formatClockMMSS(v.TimestampSec),
			fmt.Sprintf("%d", v.ReviewCount),
		})
	}
	w.Flush()
	return b.String()
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

func localizedPodcastTitle(locale string) string {
	switch normalizeLocale(locale) {
	case "zh-Hans":
		return "播客"
	case "ja":
		return "ポッドキャスト"
	default:
		return "Podcast"
	}
}

func newPlugin() *hylaplugin.Plugin {
	svc := newPodcastService()
	return &hylaplugin.Plugin{
		Name:          "Podcast",
		Version:       "1.0.0",
		Capabilities:  []string{"ui.webview"},
		Subscriptions: []string{"pane.status", "session.state", "locale.changed", "settings.changed"},
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

// Keep unused net/url import avoidance unnecessary — validate query encoding cleanly.
var _ = url.QueryEscape

func main() {
	if err := hylaplugin.Run(newPlugin()); err != nil {
		log.Fatal(err)
	}
}
