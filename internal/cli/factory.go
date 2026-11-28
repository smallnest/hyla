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

func factoryCmd(c *client.Client, _ proto.State, args []string) error {
	if len(args) == 0 {
		return usageErr("hyla factory <start|ls|status|stop|digest> [args…]")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "start":
		f, err := parse(rest,
			map[string]string{
				"repo":              "repo",
				"base":              "base",
				"agent":             "agent",
				"issues":            "issues",
				"task":              "task",
				"cmd":               "cmd",
				"flow":              "flow",
				"feishu-chat":       "chat",
				"max-parallel":      "parallel",
				"max-minutes":       "minutes",
				"max-output-kb":     "outkb",
				"max-repeat-errors": "repeats",
			},
			map[string]string{"auto-enqueue": "enqueue", "json": "json"})
		if err != nil {
			return err
		}
		repo := f.vals["repo"]
		if repo == "" {
			repo, _ = os.Getwd()
		}
		var issues []int
		if raw := strings.TrimSpace(f.vals["issues"]); raw != "" {
			for _, part := range strings.Split(raw, ",") {
				n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(part, "#")))
				if err != nil || n <= 0 {
					return usageErr("invalid issue number %q", part)
				}
				issues = append(issues, n)
			}
		}
		var tasks []string
		if f.vals["task"] != "" {
			tasks = append(tasks, f.vals["task"])
		}
		if len(f.pos) > 0 {
			tasks = append(tasks, strings.Join(f.pos, " "))
		}
		maxParallel, _ := strconv.Atoi(f.vals["parallel"])
		maxMinutes, _ := strconv.Atoi(f.vals["minutes"])
		maxOutputKB, _ := strconv.Atoi(f.vals["outkb"])
		maxRepeatErrors, _ := strconv.Atoi(f.vals["repeats"])

		var run proto.FactoryRun
		if err := c.Call("factory.start", proto.FactoryStartParams{
			Repo:            repo,
			Base:            f.vals["base"],
			Agent:           f.vals["agent"],
			Issues:          issues,
			Tasks:           tasks,
			CheckCmd:        f.vals["cmd"],
			FlowPath:        f.vals["flow"],
			FeishuChat:      f.vals["chat"],
			MaxParallel:     maxParallel,
			MaxMinutes:      maxMinutes,
			MaxOutputKB:     maxOutputKB,
			MaxRepeatErrors: maxRepeatErrors,
			AutoEnqueue:     f.set["enqueue"],
		}, &run); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(run)
		}
		fmt.Printf("%s\t%s\t%d tasks (%s)\n", run.ID, run.Status, len(run.Tasks), run.Agent)
		return nil

	case "ls", "list":
		f, err := parse(rest, nil, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		var runs []proto.FactoryRun
		if err := c.Call("factory.list", nil, &runs); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(runs)
		}
		if len(runs) == 0 {
			fmt.Println("no factory runs")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tAGENT\tTASKS\tREPO")
		for _, r := range runs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", r.ID, r.Status, r.Agent, len(r.Tasks), r.Repo)
		}
		return w.Flush()

	case "status":
		f, err := parse(rest, nil, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) == 0 {
			return usageErr("hyla factory status <run-id>")
		}
		var run proto.FactoryRun
		if err := c.Call("factory.status", proto.FactoryStatusParams{ID: f.pos[0]}, &run); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(run)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintf(w, "Run %s (%s) repo=%s base=%s agent=%s\n", run.ID, run.Status, run.Repo, run.Base, run.Agent)
		fmt.Fprintln(w, "TASK\tSTATUS\tBRANCH\tLANE\tDETAIL")
		for _, tk := range run.Tasks {
			detail := tk.TripReason
			if detail == "" {
				detail = tk.Error
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", tk.Title, tk.Status, tk.Branch, tk.LaneID, detail)
		}
		return w.Flush()

	case "stop":
		if len(rest) == 0 {
			return usageErr("hyla factory stop <run-id>")
		}
		return c.Call("factory.stop", proto.FactoryStopParams{ID: rest[0]}, nil)

	case "digest":
		f, err := parse(rest, map[string]string{"feishu-chat": "chat"}, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) == 0 {
			return usageErr("hyla factory digest <run-id> [--feishu-chat id]")
		}
		var res proto.FactoryDigestResult
		if err := c.Call("factory.digest", proto.FactoryDigestParams{ID: f.pos[0], FeishuChat: f.vals["chat"]}, &res); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(res)
		}
		fmt.Print(res.Markdown)
		return nil

	default:
		return usageErr("unknown factory command %q", sub)
	}
}
