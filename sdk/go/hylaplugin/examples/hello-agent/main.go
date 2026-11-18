package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func main() {
	clicks := 0
	plg := &hylaplugin.Plugin{
		Name:    "hello-agent",
		Version: "0.1.0",
		OnDetect: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.AgentDetectParams) (hylaplugin.AgentDetectResult, error) {
			if strings.Contains(p.Screen, "Approve?") {
				return hylaplugin.AgentDetectResult{
					State:    hylaplugin.StatusWaiting,
					WaitKind: "permission",
					Message:  "Waiting for approval",
				}, nil
			}
			return hylaplugin.AgentDetectResult{State: hylaplugin.StatusIdle}, nil
		},
		OnReadTimeline: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.AgentReadTimelineParams) (hylaplugin.AgentReadTimelineResult, error) {
			return hylaplugin.AgentReadTimelineResult{
				Timeline: hylaplugin.Timeline{
					Agent: p.Agent,
					Turns: []hylaplugin.TimelineStep{{
						Kind:  "turn",
						Title: "Hello from plugin agent!",
						State: hylaplugin.StepDone,
					}},
				},
			}, nil
		},
		OnUIRender: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIRenderParams) (hylaplugin.UIRenderResult, error) {
			return hylaplugin.UIRenderResult{
				Title: "Hello Agent",
				Badge: fmt.Sprintf("%d clicks", clicks),
				Root: hylaplugin.Column([]*hylaplugin.UINode{
					hylaplugin.Text("Session: "+p.SessionID, hylaplugin.WithProp("tone", "muted")),
					hylaplugin.Button("ping", "Ping Agent", "primary", hylaplugin.WithOnClick("ping")),
				}, hylaplugin.WithStyle(hylaplugin.UIStyle{Gap: 8})),
			}, nil
		},
		OnUIEvent: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIEventParams) (hylaplugin.UIEventResult, error) {
			clicks++
			return hylaplugin.UIEventResult{
				Handled:    true,
				Invalidate: true,
			}, nil
		},
	}
	if err := hylaplugin.Run(plg); err != nil {
		log.Fatal(err)
	}
}
