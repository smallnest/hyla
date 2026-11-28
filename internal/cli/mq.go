package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"hyla/internal/client"
	"hyla/internal/proto"
)

func mqCmd(c *client.Client, st proto.State, args []string) error {
	if len(args) == 0 {
		return usageErr("hyla mq <add|ls|run|approve|reject|remove|clear> [args…]")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		f, err := parse(rest,
			map[string]string{"repo": "repo", "branch": "branch", "base": "base", "check": "check", "flow": "flow"},
			map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		targetRef := ""
		if len(f.pos) > 0 {
			t, err := find(st, f.pos[0])
			if err != nil {
				return err
			}
			targetRef = t.s.ID
		} else if f.vals["repo"] == "" {
			if t, err := findOrCurrent(st, nil); err == nil {
				targetRef = t.s.ID
			}
		}
		var lane proto.MergeLane
		if err := c.Call("mq.add", proto.MergeQueueAddParams{
			Target:   targetRef,
			Repo:     f.vals["repo"],
			Branch:   f.vals["branch"],
			Base:     f.vals["base"],
			CheckCmd: f.vals["check"],
			FlowPath: f.vals["flow"],
		}, &lane); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(lane)
		}
		fmt.Printf("%s\t%s\t%s -> %s\n", lane.ID, lane.Status, lane.Branch, lane.Base)
		return nil

	case "ls", "list":
		f, err := parse(rest, map[string]string{"repo": "repo"}, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		var lanes []proto.MergeLane
		if err := c.Call("mq.list", proto.MergeQueueListParams{Repo: f.vals["repo"]}, &lanes); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(lanes)
		}
		if len(lanes) == 0 {
			fmt.Println("merge queue is empty")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tBRANCH\tBASE\tBOUNCES\tFINDINGS")
		for _, l := range lanes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\n", l.ID, l.Status, l.Branch, l.Base, l.Bounces, len(l.Findings))
		}
		return w.Flush()

	case "run":
		f, err := parse(rest,
			map[string]string{"repo": "repo", "train": "train", "max-bounces": "bounces", "check": "check", "feishu-chat": "chat", "browser": "browser"},
			map[string]string{"cleanup": "cleanup", "json": "json"})
		if err != nil {
			return err
		}
		train, _ := strconv.Atoi(f.vals["train"])
		bounces, _ := strconv.Atoi(f.vals["bounces"])
		var res proto.MergeQueueRunResult
		if err := c.Call("mq.run", proto.MergeQueueRunParams{
			Repo:        f.vals["repo"],
			Train:       train,
			MaxBounces:  bounces,
			CheckCmd:    f.vals["check"],
			Cleanup:     f.set["cleanup"],
			FeishuChat:  f.vals["chat"],
			BrowserPane: f.vals["browser"],
		}, &res); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(res)
		}
		fmt.Printf("landed=%d bounced=%d needs_review=%d\n", len(res.Landed), len(res.Bounced), len(res.NeedsReview))
		for _, l := range res.Lanes {
			if l.LastError != "" {
				fmt.Printf("  %s (%s): %s\n", l.ID, l.Status, l.LastError)
			}
		}
		return nil

	case "approve":
		if len(rest) < 1 {
			return usageErr("hyla mq approve <lane-id>")
		}
		var lane proto.MergeLane
		if err := c.Call("mq.approve", proto.MergeQueueLaneParams{ID: rest[0]}, &lane); err != nil {
			return err
		}
		fmt.Printf("%s approved (%s)\n", lane.ID, lane.Status)
		return nil

	case "reject":
		if len(rest) < 1 {
			return usageErr("hyla mq reject <lane-id> [reason…]")
		}
		reason := strings.Join(rest[1:], " ")
		var lane proto.MergeLane
		if err := c.Call("mq.reject", proto.MergeQueueLaneParams{ID: rest[0], Reason: reason}, &lane); err != nil {
			return err
		}
		fmt.Printf("%s rejected\n", lane.ID)
		return nil

	case "remove", "rm":
		if len(rest) < 1 {
			return usageErr("hyla mq remove <lane-id>")
		}
		return c.Call("mq.remove", proto.MergeQueueLaneParams{ID: rest[0]}, nil)

	case "clear":
		f, err := parse(rest, map[string]string{"repo": "repo"}, nil)
		if err != nil {
			return err
		}
		var n int
		if err := c.Call("mq.clear", proto.MergeQueueListParams{Repo: f.vals["repo"]}, &n); err != nil {
			return err
		}
		fmt.Printf("cleared %d lane(s)\n", n)
		return nil

	default:
		return usageErr("unknown mq command %q", sub)
	}
}
