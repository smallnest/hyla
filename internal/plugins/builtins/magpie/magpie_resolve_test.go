package magpieplugin

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hyla/internal/app/magpie"
	"hyla/internal/plugins/rpc"
	"hyla/internal/proto"
)

func TestResolveAgentSessionIDAndBadge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"name":"magpie"}`))
		case "/v1/magpie/usage":
			_, _ = w.Write([]byte(`{"calls":3,"input":100,"output":50,"cost":1.25}`))
		case "/v1/magpie/quotas":
			_, _ = w.Write([]byte(`{}`))
		case "/v1/magpie/usage/requests":
			_, _ = w.Write([]byte(`{"total":1,"rows":[{"session":"agent-conv-xyz","status":200,"served":"anthropic","ms":120,"cost":0.42}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("MAGPIE_URL", srv.URL)

	cHost, cPlugin := net.Pipe()
	defer cHost.Close()
	defer cPlugin.Close()

	hostPeer := rpc.NewPeer(cHost, cHost, cHost)
	hostPeer.Handle("host.getState", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return rpc.HostGetStateResult{
			State: proto.State{
				Sessions: []proto.Session{{
					ID:    "hyla-sess-1",
					Focus: "pane-1",
					Panes: []proto.Pane{{
						ID:             "pane-1",
						AgentSessionID: "agent-conv-xyz",
					}},
				}},
			},
		}, nil
	})
	pluginPeer := rpc.NewPeer(cPlugin, cPlugin, cPlugin)
	defer pluginPeer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hostPeer.Start(ctx)
	go runServer(ctx, pluginPeer)

	var renderRes rpc.UIRenderResult
	if err := hostPeer.Call(ctx, "ui.render", rpc.UIRenderParams{
		SlotID:    "magpie",
		SessionID: "hyla-sess-1",
		PaneID:    "pane-1",
	}, &renderRes); err != nil {
		t.Fatal(err)
	}
	if renderRes.Badge != magpie.Cost(1.25) {
		t.Fatalf("Badge=%q, want %q", renderRes.Badge, magpie.Cost(1.25))
	}
	raw, _ := json.Marshal(renderRes.Root)
	if !strings.Contains(string(raw), "This conversation") || !strings.Contains(string(raw), "0.4200") {
		t.Fatalf("expected resolved AgentSessionID conversation usage in rendered tree, got: %s", raw)
	}

	// Fallback when HostGetStateResult has empty AgentSessionID -> pane.getEnv resolves it.
	cHost2, cPlugin2 := net.Pipe()
	defer cHost2.Close()
	defer cPlugin2.Close()
	hostPeer2 := rpc.NewPeer(cHost2, cHost2, cHost2)
	hostPeer2.Handle("host.getState", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		return rpc.HostGetStateResult{
			ActiveSession: "hyla-sess-1",
			FocusedPane:   "pane-1",
			State: proto.State{
				Sessions: []proto.Session{{
					ID:    "hyla-sess-1",
					Focus: "pane-1",
					Panes: []proto.Pane{{ID: "pane-1", Agent: "claude"}},
				}},
			},
		}, nil
	})
	hostPeer2.Handle("pane.getEnv", func(ctx context.Context, method string, params json.RawMessage) (any, error) {
		var p rpc.PaneGetEnvParams
		_ = json.Unmarshal(params, &p)
		if p.Pane == "pane-1" {
			return rpc.PaneGetEnvResult{AgentSessionID: "agent-conv-xyz"}, nil
		}
		return rpc.PaneGetEnvResult{}, nil
	})
	pluginPeer2 := rpc.NewPeer(cPlugin2, cPlugin2, cPlugin2)
	defer pluginPeer2.Close()
	hostPeer2.Start(ctx)
	go runServer(ctx, pluginPeer2)

	var renderRes2 rpc.UIRenderResult
	if err := hostPeer2.Call(ctx, "ui.render", rpc.UIRenderParams{SlotID: "magpie"}, &renderRes2); err != nil {
		t.Fatal(err)
	}
	raw2, _ := json.Marshal(renderRes2.Root)
	if !strings.Contains(string(raw2), "This conversation") || !strings.Contains(string(raw2), "0.4200") {
		t.Fatalf("expected pane.getEnv fallback to resolve session usage, got: %s", raw2)
	}
}
