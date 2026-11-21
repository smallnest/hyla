package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

const (
	pluginID = "hyla.iptv"
	slotID   = "iptv"
)

type JobStatus struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`  // "refresh"
	State    string `json:"state"` // "running" | "done" | "error"
	Progress int    `json:"progress"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

type iptvService struct {
	mu           sync.RWMutex
	host         *hylaplugin.Host
	store        *Store
	netc         *netClients
	media        *MediaServer
	epg          *epgService
	presets      *presetList
	locale       string
	cfg          NetConfig
	workingPanes map[string]bool
	jobs         map[string]JobStatus
	jobSeq       int
	bgCtx        context.Context
	bgCancel     context.CancelFunc
}

func newIPTVService() *iptvService {
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &iptvService{
		locale:       "en",
		cfg:          defaultNetConfig(),
		netc:         newNetClients(),
		workingPanes: map[string]bool{},
		jobs:         map[string]JobStatus{},
		bgCtx:        bgCtx,
		bgCancel:     bgCancel,
	}
}

func (s *iptvService) shutdown() {
	s.bgCancel()
	s.mu.Lock()
	ms, es := s.media, s.epg
	s.media, s.epg = nil, nil
	s.mu.Unlock()
	if es != nil {
		es.Close()
	}
	if ms != nil {
		ms.Close()
	}
	if s.netc != nil {
		s.netc.closeIdle()
	}
}

func (s *iptvService) initRuntime(ctx context.Context, host *hylaplugin.Host, dataDir, locale string, settings map[string]any) error {
	if strings.TrimSpace(locale) != "" {
		s.setLocale(locale)
	}
	if strings.TrimSpace(dataDir) == "" {
		var err error
		dataDir, err = os.MkdirTemp("", "hyla-iptv-data-*")
		if err != nil {
			return err
		}
	}
	store, err := NewStore(dataDir)
	if err != nil {
		return err
	}
	ms, err := NewMediaServer(store, s.netc, func() NetConfig {
		return s.getConfig()
	})
	if err != nil {
		return err
	}

	es := newEPGService(s.bgCtx, store, s.netc, func() NetConfig {
		return s.getConfig()
	})

	presets := newPresetList(dataDir, s.netc, func() NetConfig {
		return s.getConfig()
	})
	// A test reads the file it gives: no network, no wait for it.
	presets.offline = isTestBinary()

	s.mu.Lock()
	oldMedia, oldEPG := s.media, s.epg
	s.presets = presets
	s.host = host
	s.store = store
	s.media = ms
	s.epg = es
	s.cfg = buildNetConfig(settings)
	s.mu.Unlock()
	if oldEPG != nil {
		oldEPG.Close()
	}
	if oldMedia != nil {
		oldMedia.Close()
	}

	presets.start()
	go func() {
		rctx, cancel := context.WithTimeout(s.bgCtx, 5*time.Second)
		defer cancel()
		s.reloadSettings(rctx)
		// The guide loads beside the pane: a saved guide shows at once, and
		// a stale one fetches in the background.
		sources, channels, _, _, _, _ := store.Snapshot()
		s.epgFor().start(func() []Channel { return channels }, epgURLsOf(sources))
	}()
	return nil
}

func (s *iptvService) epgFor() *epgService {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.epg
}

// startGuide keeps the guide current after a source change.
func (s *iptvService) startGuide(st *Store) {
	sources, channels, _, _, _, _ := st.Snapshot()
	if es := s.epgFor(); es != nil {
		es.start(func() []Channel { return channels }, epgURLsOf(sources))
	}
}

// presetSources returns the presets of iptv.md. The first call waits a
// little for the file, so that an open pane shows the current sources.
func (s *iptvService) presetSources(ctx context.Context) []PresetSource {
	s.mu.RLock()
	pl := s.presets
	s.mu.RUnlock()
	if pl == nil {
		return fallbackPresets()
	}
	return pl.waitForFirst(ctx, presetsWaitOnOpen)
}

func (s *iptvService) ensureStore() (*Store, *MediaServer, error) {
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

func (s *iptvService) setLocale(loc string) string {
	norm := normalizeLocale(loc)
	s.mu.Lock()
	s.locale = norm
	s.mu.Unlock()
	return norm
}

func (s *iptvService) resolveLocale(reqLoc string) string {
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

func (s *iptvService) getConfig() NetConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *iptvService) reloadSettings(ctx context.Context) {
	s.mu.RLock()
	h := s.host
	s.mu.RUnlock()
	if h == nil {
		return
	}
	res, err := h.GetSettings(ctx, false)
	if err != nil || res == nil {
		return
	}
	cfg := buildNetConfig(res.Settings)
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

func (s *iptvService) emitEvent(event string, data any) {
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

func (s *iptvService) startJob(kind, msg string) JobStatus {
	s.mu.Lock()
	s.jobSeq++
	job := JobStatus{
		ID:      fmt.Sprintf("job-%d", s.jobSeq),
		Kind:    kind,
		State:   "running",
		Message: msg,
	}
	s.jobs[job.ID] = job
	s.mu.Unlock()
	s.emitEvent("iptv.job", job)
	return job
}

func (s *iptvService) updateJob(job JobStatus) {
	s.mu.Lock()
	if job.State == "done" || job.State == "error" {
		delete(s.jobs, job.ID)
	} else {
		s.jobs[job.ID] = job
	}
	s.mu.Unlock()
	s.emitEvent("iptv.job", job)
}

func (s *iptvService) activeJobsList() []JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]JobStatus, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}

func (s *iptvService) enrichChannel(ms *MediaServer, ch Channel) Channel {
	if ms != nil {
		if ch.LogoURL != "" {
			ch.LogoProxyURL = ms.RegisterLogo(ch.LogoURL)
		}
		for i := range ch.Streams {
			ch.Streams[i].PlayURL = ms.PlayURL(ch.ID, i, ch.Streams[i].URL)
		}
	}
	return ch
}

func (s *iptvService) rpcBootstrap(ctx context.Context, st *Store, ms *MediaServer, loc string) map[string]any {
	if ctx == nil {
		ctx = context.Background()
	}
	sources, channels, groups, favorites, recent, lastCh := st.Snapshot()
	outCh := make([]Channel, len(channels))
	for i, ch := range channels {
		outCh[i] = s.enrichChannel(ms, ch)
	}
	// The presets come from iptv.md; the first call may wait for it.
	presets := s.presetSources(ctx)
	out := map[string]any{
		"locale":        loc,
		"sources":       sources,
		"channels":      outCh,
		"groups":        groups,
		"favorites":     favorites,
		"recent":        recent,
		"lastChannelId": lastCh,
		"presets":       presets,
		"settings":      s.getConfig(),
		"storage":       st.StorageUsage(),
		"activeJobs":    s.activeJobsList(),
	}
	// A pane that waits for the guide calls bootstrap again after a
	// fetch, and it hears of a fetch that ends later. Set this before
	// the work that follows: a fetch can end while it runs.
	if es := s.epgFor(); es != nil {
		es.setOnReady(func() {
			go func() {
				progs, _ := es.programGuide()
				s.emitEvent("iptv.epg", map[string]any{"programs": progs, "epg": es.status()})
			}()
		})
	}
	if es := s.epgFor(); es != nil {
		// The guide keeps the programmes of the channels that it last
		// matched. A source that the user adds or removes changes the
		// channels, so match the guide again with the new ones.
		es.rematchIfStale(channels)
		programs, _ := es.programGuide()
		out["programs"] = programs
		out["epg"] = es.status() // holds the time of the last read
	}
	return out
}

func (s *iptvService) handleAction(ctx context.Context, host *hylaplugin.Host, p hylaplugin.ActionInvokeParams) (hylaplugin.ActionInvokeResult, error) {
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
		Title:         localizedIPTVTitle(loc),
		Direction:     "right",
		FocusExisting: true,
	}); err != nil {
		return hylaplugin.ActionInvokeResult{}, err
	}
	return hylaplugin.ActionInvokeResult{OK: true}, nil
}

func (s *iptvService) handleWebRPC(ctx context.Context, host *hylaplugin.Host, p hylaplugin.WebViewRPCParams) (hylaplugin.WebViewRPCResult, error) {
	loc := s.resolveLocale(p.Locale)
	st, ms, err := s.ensureStore()
	if err != nil {
		return hylaplugin.WebViewRPCResult{}, err
	}

	var data any
	switch p.Method {
	case "iptv.bootstrap":
		data = s.rpcBootstrap(ctx, st, ms, loc)

	case "iptv.addSources":
		var req struct {
			Input string `json:"input"`
			Name  string `json:"name"`
		}
		_ = json.Unmarshal(p.Params, &req)
		cfg := s.getConfig()
		s.mu.RLock()
		if s.epg == nil {
			s.mu.RUnlock()
			return hylaplugin.WebViewRPCResult{}, fmt.Errorf("guide service is not ready")
		}
		s.mu.RUnlock()
		urls, rawPlaylist := ParseInputLines(req.Input)
		var addedSources []Source
		var warnings []string

		if rawPlaylist != "" {
			src, chs, rerr := ResolveAndFetchSource(ctx, s.netc, cfg, rawPlaylist, req.Name)
			if rerr != nil {
				return hylaplugin.WebViewRPCResult{}, rerr
			}
			saved, uerr := st.UpsertSource(src, chs)
			if uerr != nil {
				return hylaplugin.WebViewRPCResult{}, uerr
			}
			addedSources = append(addedSources, saved)
		} else {
			if len(urls) == 0 {
				return hylaplugin.WebViewRPCResult{}, fmt.Errorf("please enter at least one M3U URL or paste M3U playlist content")
			}
			for _, u := range urls {
				hint := ""
				if len(urls) == 1 {
					hint = req.Name
				}
				src, chs, rerr := ResolveAndFetchSource(ctx, s.netc, cfg, u, hint)
				if rerr != nil {
					warnings = append(warnings, fmt.Sprintf("%s: %v", u, rerr))
					continue
				}
				saved, uerr := st.UpsertSource(src, chs)
				if uerr != nil {
					warnings = append(warnings, fmt.Sprintf("%s: %v", u, uerr))
					continue
				}
				addedSources = append(addedSources, saved)
			}
			if len(addedSources) == 0 && len(warnings) > 0 {
				return hylaplugin.WebViewRPCResult{}, fmt.Errorf("%s", strings.Join(warnings, "; "))
			}
		}

		s.startGuide(st)
		data = map[string]any{
			"added":     len(addedSources),
			"warnings":  warnings,
			"bootstrap": s.rpcBootstrap(ctx, st, ms, loc),
		}

	case "iptv.toggleSource":
		var req struct {
			SourceID string `json:"sourceId"`
			Enabled  bool   `json:"enabled"`
		}
		_ = json.Unmarshal(p.Params, &req)
		if terr := st.ToggleSource(req.SourceID, req.Enabled); terr != nil {
			return hylaplugin.WebViewRPCResult{}, terr
		}
		s.startGuide(st)
		data = map[string]any{"bootstrap": s.rpcBootstrap(ctx, st, ms, loc)}

	case "iptv.removeSource":
		var req struct {
			SourceID string `json:"sourceId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		if rerr := st.RemoveSource(req.SourceID); rerr != nil {
			return hylaplugin.WebViewRPCResult{}, rerr
		}
		s.startGuide(st)
		data = map[string]any{"bootstrap": s.rpcBootstrap(ctx, st, ms, loc)}

	case "iptv.refreshSources":
		job := s.startJob("refresh", "Refreshing sources…")
		go s.runRefreshJob(job, st)
		data = map[string]any{"job": job}

	case "iptv.toggleFavorite":
		var req struct {
			ChannelID string `json:"channelId"`
		}
		_ = json.Unmarshal(p.Params, &req)
		fav, terr := st.ToggleFavorite(req.ChannelID)
		if terr != nil {
			return hylaplugin.WebViewRPCResult{}, terr
		}
		_, _, _, favs, _, _ := st.Snapshot()
		data = map[string]any{
			"channelId": req.ChannelID,
			"favorite":  fav,
			"favorites": favs,
		}

	case "iptv.recordWatch":
		var req struct {
			ChannelID string `json:"channelId"`
			LineIdx   int    `json:"lineIdx"`
		}
		_ = json.Unmarshal(p.Params, &req)
		if werr := st.RecordWatch(req.ChannelID, req.LineIdx); werr != nil {
			return hylaplugin.WebViewRPCResult{}, werr
		}
		_, _, _, _, recent, lastCh := st.Snapshot()
		data = map[string]any{
			"recent":        recent,
			"lastChannelId": lastCh,
		}

	case "iptv.saveSettings":
		var req NetConfig
		if uerr := json.Unmarshal(p.Params, &req); uerr != nil {
			return hylaplugin.WebViewRPCResult{}, uerr
		}
		switch req.ProxyMode {
		case "auto", "direct", "system", "custom":
		default:
			req.ProxyMode = "auto"
		}
		if host != nil {
			if serr := host.SetSettings(ctx, map[string]any{
				"proxy_mode":         req.ProxyMode,
				"http_proxy":         strings.TrimSpace(req.HTTPProxy),
				"auto_failover":      req.AutoFailover,
				"auto_mute_on_agent": req.AutoMuteOnAgent,
				"user_agent":         strings.TrimSpace(req.UserAgent),
			}, nil); serr != nil {
				return hylaplugin.WebViewRPCResult{}, serr
			}
		}
		s.mu.Lock()
		s.cfg = req
		s.mu.Unlock()
		data = map[string]any{"settings": req}

	case "iptv.clearLogoCache":
		info, cerr := st.ClearLogoCache()
		if cerr != nil {
			return hylaplugin.WebViewRPCResult{}, cerr
		}
		data = map[string]any{"storage": info}

	case "iptv.pictureInPicture":
		// The window moves the video: on macOS, WKWebView moves only the
		// video it lets itself move, not one a page asks for.
		if host == nil {
			return hylaplugin.WebViewRPCResult{}, fmt.Errorf("picture in picture needs the window")
		}
		if err := host.WebViewPictureInPicture(ctx, p.SlotID); err != nil {
			return hylaplugin.WebViewRPCResult{}, err
		}
		data = map[string]any{"ok": true}

	case "iptv.refreshEPG":
		es := s.epgFor()
		if es == nil {
			return hylaplugin.WebViewRPCResult{}, fmt.Errorf("guide service is not ready")
		}
		sources, channels, _, _, _, _ := st.Snapshot()
		keep := func() []Channel { return channels }
		urls := epgURLsOf(sources)
		if len(urls) == 0 {
			urls = es.guideURLs()
		}
		es.refresh(keep, urls)
		data = map[string]any{"epg": es.status()}

	case "iptv.programs":
		es := s.epgFor()
		if es == nil {
			return hylaplugin.WebViewRPCResult{}, fmt.Errorf("guide service is not ready")
		}
		programs, updatedAt := es.programGuide()
		data = map[string]any{"programs": programs, "updatedAt": updatedAt, "epg": es.status()}

	case "iptv.exportM3U":
		_, channels, _, _, _, _ := st.Snapshot()
		data = map[string]any{"m3u": ExportM3U(channels)}

	default:
		return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown method: "+p.Method)
	}

	raw, merr := json.Marshal(data)
	if merr != nil {
		return hylaplugin.WebViewRPCResult{}, merr
	}
	return hylaplugin.WebViewRPCResult{Data: raw}, nil
}

func (s *iptvService) runRefreshJob(job JobStatus, st *Store) {
	ctx, cancel := context.WithTimeout(s.bgCtx, 3*time.Minute)
	defer cancel()

	sources, _, _, _, _, _ := st.Snapshot()
	if len(sources) == 0 {
		job.State = "done"
		job.Progress = 100
		s.updateJob(job)
		return
	}
	cfg := s.getConfig()
	for i, src := range sources {
		if ctx.Err() != nil {
			job.State = "error"
			job.Error = ctx.Err().Error()
			s.updateJob(job)
			return
		}
		job.Message = src.Name
		job.Progress = (i * 100) / len(sources)
		s.updateJob(job)
		if src.InputURL == "" {
			continue
		}
		fresh, chs, err := ResolveAndFetchSource(ctx, s.netc, cfg, src.InputURL, src.Name)
		if err != nil {
			log.Printf("iptv refresh %s: %v", src.Name, err)
			_ = st.MarkSourceError(src.ID, err.Error())
			continue
		}
		fresh.ID = src.ID
		fresh.AddedAt = src.AddedAt
		fresh.Enabled = src.Enabled
		_, _ = st.UpsertSource(fresh, chs)
	}
	job.State = "done"
	job.Progress = 100
	job.Message = ""
	s.updateJob(job)
}

func (s *iptvService) handleNotify(ctx context.Context, host *hylaplugin.Host, ev hylaplugin.EventNotifyParams) {
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
		s.emitEvent("iptv.settingsChanged", map[string]any{"settings": s.getConfig()})
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
		autoMute := s.cfg.AutoMuteOnAgent
		s.mu.Unlock()

		if autoMute && wasWorking && !isWorking {
			s.emitEvent("iptv.autoMute", map[string]any{
				"sessionId": data.SessionID,
				"paneId":    data.PaneID,
				"agent":     data.Agent,
			})
		}
	}
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

func localizedIPTVTitle(locale string) string {
	switch normalizeLocale(locale) {
	case "zh-Hans":
		return "IPTV 直播"
	default:
		return "IPTV"
	}
}

func newPlugin() *hylaplugin.Plugin {
	svc := newIPTVService()
	return &hylaplugin.Plugin{
		Name:          "IPTV",
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

func main() {
	if err := hylaplugin.Run(newPlugin()); err != nil {
		log.Fatal(err)
	}
}
