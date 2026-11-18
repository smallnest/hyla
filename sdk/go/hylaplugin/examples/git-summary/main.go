package main

import (
	"context"
	"fmt"
	"log"

	"github.com/smallnest/hyla/sdk/go/hylaplugin"
)

func main() {
	plg := &hylaplugin.Plugin{
		Name:    "git-summary",
		Version: "0.1.0",
		OnUIRender: func(ctx context.Context, host *hylaplugin.Host, p hylaplugin.UIRenderParams) (hylaplugin.UIRenderResult, error) {
			st, err := host.GetState(ctx)
			if err != nil {
				return hylaplugin.UIRenderResult{}, err
			}
			return hylaplugin.UIRenderResult{
				Title: "Git Summary",
				Root: hylaplugin.Column([]*hylaplugin.UINode{
					hylaplugin.Text(fmt.Sprintf("Active sessions: %d", len(st.State.Sessions))),
					hylaplugin.Text(fmt.Sprintf("Projects: %d", len(st.State.Projects)), hylaplugin.WithProp("tone", "muted")),
				}, hylaplugin.WithStyle(hylaplugin.UIStyle{Gap: 6})),
			}, nil
		},
	}
	if err := hylaplugin.Run(plg); err != nil {
		log.Fatal(err)
	}
}
