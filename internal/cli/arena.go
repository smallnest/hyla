package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"hyla/internal/client"
	"hyla/internal/proto"
)

func arenaCmd(c *client.Client, st proto.State, args []string) error {
	if len(args) == 0 {
		return usageErr("hyla arena <start|ls|status|test|pick|close> [args…]")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "start":
		f, err := parse(rest,
			map[string]string{"agents": "agents", "a": "agents", "d": "dir", "dir": "dir", "base": "base", "test": "test", "n": "name", "name": "name"},
			map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		prompt := strings.Join(append(f.pos, f.rest...), " ")
		if strings.TrimSpace(prompt) == "" {
			return usageErr("hyla arena start [--agents claude,codex,agy] [--test cmd] <prompt…>")
		}
		var agents []string
		if raw := strings.TrimSpace(f.vals["agents"]); raw != "" {
			for _, part := range strings.Split(raw, ",") {
				if p := strings.TrimSpace(part); p != "" {
					agents = append(agents, p)
				}
			}
		}
		dir := f.vals["dir"]
		if dir == "" {
			dir, _ = os.Getwd()
		}
		var ar proto.Arena
		if err := c.Call("arena.start", proto.ArenaStartParams{
			Prompt:  prompt,
			Agents:  agents,
			Dir:     dir,
			Base:    f.vals["base"],
			TestCmd: f.vals["test"],
			Name:    f.vals["name"],
		}, &ar); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(ar)
		}
		printArena(ar)
		return nil

	case "ls", "list":
		f, err := parse(rest, nil, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		var list []proto.Arena
		if err := c.Call("arena.list", nil, &list); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(list)
		}
		if len(list) == 0 {
			fmt.Println("no arenas")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tAGENTS\tWINNER\tNAME")
		for _, a := range list {
			var ags []string
			for _, ct := range a.Contestants {
				ags = append(ags, ct.Agent)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.ID, a.Status, strings.Join(ags, ","), a.Winner, a.Name)
		}
		return w.Flush()

	case "status":
		f, err := parse(rest, nil, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) < 1 {
			return usageErr("hyla arena status <arena-id>")
		}
		var ar proto.Arena
		if err := c.Call("arena.status", proto.ArenaStatusParams{ID: f.pos[0]}, &ar); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(ar)
		}
		printArena(ar)
		return nil

	case "test":
		f, err := parse(rest, map[string]string{"cmd": "cmd"}, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) < 1 {
			return usageErr("hyla arena test <arena-id> [-- cmd…]")
		}
		cmdStr := f.vals["cmd"]
		if cmdStr == "" && len(f.rest) > 0 {
			cmdStr = strings.Join(f.rest, " ")
		}
		if cmdStr == "" && len(f.pos) > 1 {
			cmdStr = strings.Join(f.pos[1:], " ")
		}
		var ar proto.Arena
		if err := c.Call("arena.test", proto.ArenaTestParams{ID: f.pos[0], Cmd: cmdStr}, &ar); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(ar)
		}
		printArena(ar)
		return nil

	case "pick":
		f, err := parse(rest, nil, map[string]string{"keep-others": "keep", "no-merge": "nomerge", "json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) < 2 {
			return usageErr("hyla arena pick <arena-id> <agent|session> [--keep-others] [--no-merge]")
		}
		var ar proto.Arena
		if err := c.Call("arena.pick", proto.ArenaPickParams{
			ID:         f.pos[0],
			Winner:     f.pos[1],
			KeepOthers: f.set["keep"],
			NoMerge:    f.set["nomerge"],
		}, &ar); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(ar)
		}
		printArena(ar)
		return nil

	case "close":
		f, err := parse(rest, nil, map[string]string{"force": "force"})
		if err != nil {
			return err
		}
		if len(f.pos) < 1 {
			return usageErr("hyla arena close <arena-id> [--force]")
		}
		return c.Call("arena.close", proto.ArenaCloseParams{ID: f.pos[0], Force: f.set["force"]}, nil)

	default:
		return usageErr("unknown arena command %q", sub)
	}
}

func printArena(a proto.Arena) {
	fmt.Printf("Arena %s (%s) base=%s\n", a.ID, a.Status, a.Base)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tSESSION\tSTATUS\tFILES\t+LINES\t-LINES\tCOMMITS\tCHECK")
	for _, c := range a.Contestants {
		chk := c.CheckStatus
		if chk == "" {
			chk = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t+%d\t-%d\t%d\t%s\n",
			c.Agent, c.SessionID, c.Status, c.Files, c.Added, c.Removed, c.Commits, chk)
	}
	_ = w.Flush()
}

func reviewCmd(c *client.Client, st proto.State, args []string) error {
	f, err := parse(args,
		map[string]string{"reviewer": "reviewer", "r": "reviewer"},
		map[string]string{"branch": "branch", "turn": "turn", "down": "down", "right": "right", "json": "json", "f": "focus", "focus": "focus"})
	if err != nil {
		return err
	}
	t, err := findOrCurrent(st, f.pos)
	if err != nil {
		return err
	}
	scope := "working"
	if f.set["branch"] {
		scope = "branch"
	} else if f.set["turn"] {
		scope = "turn"
	}
	dir := "right"
	if f.set["down"] {
		dir = "down"
	}
	var res proto.CrossReviewResult
	if err := c.Call("pane.crossReview", proto.CrossReviewParams{
		Pane:      t.ref(),
		Reviewer:  f.vals["reviewer"],
		Scope:     scope,
		Direction: dir,
	}, &res); err != nil {
		return err
	}
	if f.set["focus"] {
		_ = c.Call("select", proto.FocusParams{Session: res.Session.ID, Pane: res.PaneID}, nil)
	}
	if f.set["json"] {
		return printJSON(res)
	}
	fmt.Printf("%s (%s)\n", res.PaneID, res.Reviewer)
	return nil
}
