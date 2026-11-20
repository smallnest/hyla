package main

import (
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

const (
	pluginID = "hyla.timeline"
	slotID   = "timeline"

	fastPollMS = 3000
	idlePollMS = 30000
)

type paneCandidate struct {
	ID     string            `json:"id"`
	Label  string            `json:"label"`
	Agent  string            `json:"agent,omitempty"`
	Status hylaplugin.Status `json:"status,omitempty"`
	Dir    string            `json:"dir,omitempty"`
}

type snapshotRequest struct {
	TargetPane string `json:"targetPane,omitempty"`
}

type snapshotResponse struct {
	Locale     string               `json:"locale"`
	SessionID  string               `json:"sessionId,omitempty"`
	PaneID     string               `json:"paneId,omitempty"`
	TargetPane string               `json:"targetPane,omitempty"`
	Target     *paneCandidate       `json:"target,omitempty"`
	Candidates []paneCandidate      `json:"candidates"`
	Timeline   *hylaplugin.Timeline `json:"timeline,omitempty"`
	PollMS     int                  `json:"pollMs"`
	Error      string               `json:"error,omitempty"`
}

func isContentPane(p hylaplugin.Pane) bool {
	return p.Plugin == "" && p.URL == "" && p.File == "" && p.Timeline == "" && !p.Subagents && !p.Hongguo
}

func resolveTargetPane(sess *hylaplugin.Session, preferredID string) *hylaplugin.Pane {
	if sess == nil {
		return nil
	}
	if preferredID != "" {
		for i := range sess.Panes {
			if sess.Panes[i].ID == preferredID && isContentPane(sess.Panes[i]) {
				return &sess.Panes[i]
			}
		}
	}
	for i := range sess.Panes {
		if isContentPane(sess.Panes[i]) {
			return &sess.Panes[i]
		}
	}
	return nil
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

func candidateForPane(p hylaplugin.Pane) paneCandidate {
	label := p.Agent
	if label == "" {
		label = p.Shell
	}
	if label == "" && p.Dir != "" {
		label = filepath.Base(p.Dir)
	}
	if label == "" {
		label = p.ID
	} else if p.Dir != "" {
		base := filepath.Base(p.Dir)
		if base != "" && base != "." && base != "/" && base != label {
			label = label + " · " + base
		}
	}
	return paneCandidate{
		ID:     p.ID,
		Label:  label,
		Agent:  p.Agent,
		Status: p.Status,
		Dir:    p.Dir,
	}
}

func timelineRunning(tl *hylaplugin.Timeline) bool {
	if tl == nil || len(tl.Turns) == 0 {
		return false
	}
	last := tl.Turns[len(tl.Turns)-1]
	return last.End == 0 || last.State == hylaplugin.StepRunning || last.State == hylaplugin.StepWaiting
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

func localizedTimelineTitle(locale string) string {
	switch normalizeLocale(locale) {
	case "zh-Hans":
		return "时间线"
	case "ja":
		return "タイムライン"
	default:
		return "Timeline"
	}
}

func localizedNoSessionMsg(locale string) string {
	switch normalizeLocale(locale) {
	case "zh-Hans":
		return "没有活动的会话"
	case "ja":
		return "アクティブなセッションがありません"
	default:
		return "No active session"
	}
}

func localizedNoPaneMsg(locale string) string {
	switch normalizeLocale(locale) {
	case "zh-Hans":
		return "没有可用于时间线的窗格"
	case "ja":
		return "タイムラインを表示できるペインがありません"
	default:
		return "No pane available for timeline"
	}
}

func newPlugin() *hylaplugin.Plugin {
	var mu sync.RWMutex
	locale := "en"

	setLocale := func(loc string) string {
		norm := normalizeLocale(loc)
		mu.Lock()
		locale = norm
		mu.Unlock()
		return norm
	}

	resolveLocale := func(reqLoc string) string {
		if strings.TrimSpace(reqLoc) != "" {
			return setLocale(reqLoc)
		}
		mu.RLock()
		defer mu.RUnlock()
		if locale == "" {
			return "en"
		}
		return locale
	}

	return &hylaplugin.Plugin{
		Name:          "Timeline",
		Version:       "1.0.0",
		Capabilities:  []string{"ui.webview"},
		Subscriptions: []string{"pane.status", "session.state", "locale.changed"},

		OnInitialize: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.InitializeParams) error {
			if strings.TrimSpace(p.Locale) != "" {
				setLocale(p.Locale)
			}
			return nil
		},

		OnAction: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.ActionInvokeParams) (hylaplugin.ActionInvokeResult, error) {
			if p.ActionID != "" && p.ActionID != "open" && p.ActionID != pluginID+".open" {
				return hylaplugin.ActionInvokeResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown action: "+p.ActionID)
			}
			loc := resolveLocale(p.Locale)
			st, err := host.GetState(ctx)
			if err != nil {
				return hylaplugin.ActionInvokeResult{}, err
			}
			sess := findSession(st, p.SessionID, p.PaneID)
			if sess == nil {
				return hylaplugin.ActionInvokeResult{OK: false, Message: localizedNoSessionMsg(loc)}, nil
			}
			focusID := p.PaneID
			if focusID == "" {
				focusID = sess.Focus
			}
			if focusID == "" {
				focusID = st.FocusedPane
			}
			var target *hylaplugin.Pane
			for i := range sess.Panes {
				pn := &sess.Panes[i]
				if pn.ID != focusID {
					continue
				}
				if pn.Plugin == pluginID {
					target = resolveTargetPane(sess, pn.PluginArgs["targetPane"])
				} else if pn.Timeline != "" {
					target = resolveTargetPane(sess, pn.Timeline)
				} else {
					target = pn
				}
				break
			}
			if target == nil {
				target = resolveTargetPane(sess, focusID)
			}
			if target == nil {
				return hylaplugin.ActionInvokeResult{OK: false, Message: localizedNoPaneMsg(loc)}, nil
			}
			title := localizedTimelineTitle(loc)
			if target.Agent != "" {
				title = target.Agent
			}
			if err := host.OpenPane(ctx, hylaplugin.UIOpenPaneParams{
				Session:       sess.ID,
				Split:         target.ID,
				Plugin:        pluginID,
				View:          slotID,
				Title:         title,
				Args:          map[string]string{"targetPane": target.ID},
				Direction:     "right",
				FocusExisting: true,
			}); err != nil {
				return hylaplugin.ActionInvokeResult{}, err
			}
			return hylaplugin.ActionInvokeResult{OK: true}, nil
		},

		OnWebViewRPC: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.WebViewRPCParams) (hylaplugin.WebViewRPCResult, error) {
			if p.Method != "timeline.snapshot" {
				return hylaplugin.WebViewRPCResult{}, hylaplugin.NewError(hylaplugin.ErrCodeMethodNotFound, "unknown method: "+p.Method)
			}
			loc := resolveLocale(p.Locale)
			var req snapshotRequest
			if len(p.Params) > 0 {
				_ = json.Unmarshal(p.Params, &req)
			}

			st, err := host.GetState(ctx)
			if err != nil {
				return hylaplugin.WebViewRPCResult{}, err
			}
			sess := findSession(st, p.SessionID, p.PaneID)

			preferred := strings.TrimSpace(req.TargetPane)
			if preferred == "" && sess != nil && p.PaneID != "" {
				for i := range sess.Panes {
					if sess.Panes[i].ID != p.PaneID {
						continue
					}
					if tp := strings.TrimSpace(sess.Panes[i].PluginArgs["targetPane"]); tp != "" {
						preferred = tp
					} else if tp := strings.TrimSpace(sess.Panes[i].Timeline); tp != "" {
						preferred = tp
					}
					break
				}
			}
			if preferred == "" && p.Args != nil {
				preferred = strings.TrimSpace(p.Args["targetPane"])
			}

			candidates := make([]paneCandidate, 0)
			if sess != nil {
				for _, pn := range sess.Panes {
					if isContentPane(pn) {
						candidates = append(candidates, candidateForPane(pn))
					}
				}
			}
			target := resolveTargetPane(sess, preferred)

			resp := snapshotResponse{
				Locale:     loc,
				PaneID:     p.PaneID,
				Candidates: candidates,
				PollMS:     idlePollMS,
			}
			if sess != nil {
				resp.SessionID = sess.ID
			}
			if target != nil {
				c := candidateForPane(*target)
				resp.TargetPane = target.ID
				resp.Target = &c
				if target.Status != "" && target.Status != hylaplugin.StatusIdle {
					resp.PollMS = fastPollMS
				}
				tlRes, tlErr := host.GetTimeline(ctx, resp.SessionID, target.ID)
				if tlErr != nil {
					resp.Error = tlErr.Error()
					resp.PollMS = fastPollMS
				} else if tlRes != nil {
					tl := tlRes.Timeline
					resp.Timeline = &tl
					if timelineRunning(&tl) {
						resp.PollMS = fastPollMS
					}
				}
			}
			raw, err := json.Marshal(resp)
			if err != nil {
				return hylaplugin.WebViewRPCResult{}, err
			}
			return hylaplugin.WebViewRPCResult{Data: raw}, nil
		},

		OnNotify: func(ctx context.Context, host *hylaplugin.Host, ev hylaplugin.EventNotifyParams) {
			if ev.Topic == "locale.changed" {
				var data struct {
					Locale string `json:"locale"`
				}
				if json.Unmarshal(ev.Data, &data) == nil && strings.TrimSpace(data.Locale) != "" {
					setLocale(data.Locale)
				}
				return
			}
			if ev.Topic != "pane.status" && ev.Topic != "session.state" {
				return
			}
			emitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			_ = host.WebViewEmit(emitCtx, slotID, "timeline.dirty", json.RawMessage(ev.Data))
		},
	}
}

func main() {
	if err := hylaplugin.Run(newPlugin()); err != nil {
		log.Fatal(err)
	}
}
