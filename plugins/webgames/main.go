package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

const (
	pluginID = "hyla.webgames"
	slotID   = "webgames"
)

type webGamesService struct {
	mu           sync.RWMutex
	host         *hylaplugin.Host
	store        *Store
	locale       string
	workingPanes map[string]bool
}

func newWebGamesService() *webGamesService {
	return &webGamesService{
		locale:       "en",
		workingPanes: map[string]bool{},
	}
}

func (s *webGamesService) initRuntime(ctx context.Context, host *hylaplugin.Host, dataDir, locale string, rawSettings map[string]any) error {
	if strings.TrimSpace(locale) != "" {
		s.locale = normalizeLocale(locale)
	}
	if strings.TrimSpace(dataDir) == "" {
		dir, err := os.MkdirTemp("", "hyla-webgames-data-*")
		if err != nil {
			return err
		}
		dataDir = dir
	}
	st, err := NewStore(dataDir)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.host = host
	s.store = st
	s.mu.Unlock()
	if len(rawSettings) > 0 {
		s.applySettingsMap(rawSettings)
	}
	return nil
}

func (s *webGamesService) ensureStore() (*Store, error) {
	s.mu.RLock()
	st := s.store
	h := s.host
	loc := s.locale
	s.mu.RUnlock()
	if st != nil {
		return st, nil
	}
	if err := s.initRuntime(context.Background(), h, "", loc, nil); err != nil {
		return nil, err
	}
	s.mu.RLock()
	st = s.store
	s.mu.RUnlock()
	return st, nil
}

func (s *webGamesService) applySettingsMap(raw map[string]any) {
	st, err := s.ensureStore()
	if err != nil {
		return
	}
	cur := st.Snapshot().Settings
	if v, ok := raw["auto_pause"].(bool); ok {
		cur.AutoPause = v
	}
	if v, ok := raw["default_game"].(string); ok && strings.TrimSpace(v) != "" {
		cur.DefaultGame = strings.TrimSpace(v)
	}
	_, _ = st.UpdateSettings(cur)
}

func (s *webGamesService) handleAction(ctx context.Context, host *hylaplugin.Host, p hylaplugin.ActionInvokeParams) (hylaplugin.ActionInvokeResult, error) {
	stRes, err := host.GetState(ctx)
	if err != nil {
		return hylaplugin.ActionInvokeResult{}, err
	}
	var sessID, splitID string
	if sess := findSession(stRes, p.SessionID, p.PaneID); sess != nil {
		sessID = sess.ID
		splitID = p.PaneID
		if splitID == "" {
			splitID = sess.Focus
		}
		if splitID == "" {
			splitID = stRes.FocusedPane
		}
	}
	s.mu.RLock()
	loc := s.locale
	s.mu.RUnlock()
	if err := host.OpenPane(ctx, hylaplugin.UIOpenPaneParams{
		Session:       sessID,
		Split:         splitID,
		Plugin:        pluginID,
		View:          slotID,
		Title:         localizedTitle(loc),
		Direction:     "right",
		FocusExisting: true,
	}); err != nil {
		return hylaplugin.ActionInvokeResult{}, err
	}
	return hylaplugin.ActionInvokeResult{OK: true}, nil
}

func (s *webGamesService) handleWebRPC(ctx context.Context, host *hylaplugin.Host, p hylaplugin.WebViewRPCParams) (hylaplugin.WebViewRPCResult, error) {
	if host != nil {
		s.mu.Lock()
		if s.host == nil {
			s.host = host
		}
		s.mu.Unlock()
	}
	st, err := s.ensureStore()
	if err != nil {
		return hylaplugin.WebViewRPCResult{}, err
	}

	var snap StoreState
	switch p.Method {
	case "webgames.bootstrap", "webgames.state":
		snap = st.Snapshot()
	case "webgames.score":
		var req struct {
			Game  string `json:"game"`
			Score int    `json:"score"`
		}
		if err := json.Unmarshal(p.Params, &req); err != nil {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, err.Error())
		}
		snap, err = st.RecordScore(req.Game, req.Score)
	case "webgames.bookmark.add":
		var req struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}
		if err := json.Unmarshal(p.Params, &req); err != nil {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, err.Error())
		}
		snap, err = st.AddBookmark(req.Name, req.URL, time.Now())
	case "webgames.bookmark.remove":
		var req struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(p.Params, &req); err != nil {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, err.Error())
		}
		snap, err = st.RemoveBookmark(req.ID)
	case "webgames.settings":
		var req GameSettings
		if err := json.Unmarshal(p.Params, &req); err != nil {
			return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, err.Error())
		}
		snap, err = st.UpdateSettings(req)
	default:
		return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown method: "+p.Method)
	}
	if err != nil {
		return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeInvalidParams, err.Error())
	}

	s.mu.RLock()
	loc := s.locale
	s.mu.RUnlock()
	if strings.TrimSpace(p.Locale) != "" {
		loc = normalizeLocale(p.Locale)
	}
	raw, err := json.Marshal(map[string]any{
		"locale": loc,
		"state":  snap,
	})
	if err != nil {
		return hylaplugin.WebViewRPCResult{}, err
	}
	return hylaplugin.WebViewRPCResult{Data: raw}, nil
}

func (s *webGamesService) handleNotify(ctx context.Context, host *hylaplugin.Host, ev hylaplugin.EventNotifyParams) {
	switch ev.Topic {
	case "locale.changed":
		var data struct {
			Locale string `json:"locale"`
		}
		if json.Unmarshal(ev.Data, &data) == nil && strings.TrimSpace(data.Locale) != "" {
			s.mu.Lock()
			s.locale = normalizeLocale(data.Locale)
			s.mu.Unlock()
		}
	case "settings.changed":
		var raw map[string]any
		if json.Unmarshal(ev.Data, &raw) == nil {
			s.applySettingsMap(raw)
		} else if host != nil {
			if cfg, err := host.GetSettings(ctx, false); err == nil && cfg != nil {
				s.applySettingsMap(cfg.Settings)
			}
		}
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
		if s.shouldAutoPause(data.PaneID, data.Status) {
			s.emitAutoPause(data.SessionID, data.PaneID, data.Agent, data.Status)
		}
	}
}

func (s *webGamesService) shouldAutoPause(paneID, status string) bool {
	isWorking := status == string(hylaplugin.StatusWorking)
	s.mu.Lock()
	wasWorking := s.workingPanes[paneID]
	if isWorking {
		s.workingPanes[paneID] = true
	} else {
		delete(s.workingPanes, paneID)
	}
	st := s.store
	s.mu.Unlock()

	autoPause := true
	if st != nil {
		autoPause = st.Snapshot().Settings.AutoPause
	}
	return autoPause && wasWorking && !isWorking
}

func (s *webGamesService) emitAutoPause(sessionID, paneID, agent, status string) {
	s.mu.RLock()
	h := s.host
	s.mu.RUnlock()
	if h == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = h.WebViewEmit(ctx, slotID, "webgames.autoPause", map[string]any{
		"sessionId": sessionID,
		"paneId":    paneID,
		"agent":     agent,
		"status":    status,
	})
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
	default:
		return "en"
	}
}

func localizedTitle(loc string) string {
	switch normalizeLocale(loc) {
	case "zh-Hans":
		return "网页游戏"
	case "ja":
		return "ウェブゲーム"
	default:
		return "Web Games"
	}
}

func newPlugin() *hylaplugin.Plugin {
	svc := newWebGamesService()
	return &hylaplugin.Plugin{
		Name:          "Web Games",
		Version:       "1.0.0",
		Capabilities:  []string{"ui.webview"},
		Subscriptions: []string{"pane.status", "session.state", "locale.changed", "settings.changed"},
		OnInitialize: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.InitializeParams) error {
			return svc.initRuntime(ctx, host, p.DataDir, p.Locale, p.Settings)
		},
		OnAction:     svc.handleAction,
		OnWebViewRPC: svc.handleWebRPC,
		OnNotify:     svc.handleNotify,
	}
}

func main() {
	if err := hylaplugin.Run(newPlugin()); err != nil {
		log.Fatal(err)
	}
}
