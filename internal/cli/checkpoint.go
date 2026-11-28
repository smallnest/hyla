package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"hyla/internal/client"
	"hyla/internal/proto"
)

func checkpointCmd(c *client.Client, st proto.State, args []string) error {
	if len(args) == 0 {
		return usageErr("hyla checkpoint <ls|create|restore|fork> [args…]")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "ls", "list":
		f, err := parse(rest, map[string]string{"target": "target", "limit": "limit"}, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		target := f.vals["target"]
		if target == "" && len(f.pos) > 0 {
			target = f.pos[0]
		}
		limit, _ := strconv.Atoi(f.vals["limit"])
		var cps []proto.Checkpoint
		if err := c.Call("checkpoint.list", proto.CheckpointListParams{Target: target, Limit: limit}, &cps); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(cps)
		}
		if len(cps) == 0 {
			fmt.Println("no checkpoints recorded")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tKIND\tTREE\tTIME\tLABEL")
		for _, cp := range cps {
			kind := "manual"
			if cp.Auto {
				kind = "auto"
			}
			shortTree := cp.Tree
			if len(shortTree) > 8 {
				shortTree = shortTree[:8]
			}
			label := cp.Label
			if label == "" {
				label = cp.Root
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", cp.ID, kind, shortTree, time.UnixMilli(cp.At).Format("15:04:05"), label)
		}
		return w.Flush()

	case "create", "save":
		f, err := parse(rest, map[string]string{"target": "target", "label": "label"}, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		target := f.vals["target"]
		if target == "" && len(f.pos) > 0 {
			target = f.pos[0]
		}
		if target == "" {
			target = current()
		}
		if target == "" {
			if t, err := findOrCurrent(st, nil); err == nil {
				target = t.ref()
			} else if wd, werr := os.Getwd(); werr == nil {
				target = wd
			}
		}
		var cp proto.Checkpoint
		if err := c.Call("checkpoint.create", proto.CheckpointCreateParams{Target: target, Label: f.vals["label"]}, &cp); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(cp)
		}
		fmt.Printf("%s\t%s\t%s\n", cp.ID, cp.Tree[:min(8, len(cp.Tree))], cp.Root)
		return nil

	case "restore":
		f, err := parse(rest, nil, map[string]string{"force": "force", "json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) == 0 {
			return usageErr("hyla checkpoint restore <checkpoint-id>")
		}
		var res proto.CheckpointRestoreResult
		if err := c.Call("checkpoint.restore", proto.CheckpointRestoreParams{ID: f.pos[0], Force: f.set["force"]}, &res); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(res)
		}
		fmt.Printf("restored %s (safety backup: %s)\n", res.Restored.ID, res.SafetyBackupID)
		return nil

	case "fork":
		f, err := parse(rest, map[string]string{"branch": "branch"}, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) == 0 {
			return usageErr("hyla checkpoint fork <checkpoint-id> [--branch name]")
		}
		var res proto.CheckpointForkResult
		if err := c.Call("checkpoint.fork", proto.CheckpointForkParams{ID: f.pos[0], Branch: f.vals["branch"]}, &res); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(res)
		}
		fmt.Printf("forked session %s on branch %s\n", res.Session.ID, res.Branch)
		return nil

	default:
		return usageErr("unknown checkpoint command %q", sub)
	}
}

func ruleCmd(c *client.Client, _ proto.State, args []string) error {
	if len(args) == 0 {
		return usageErr("hyla rule add <reason> [--dir path] [--file relpath] [--dest agents|hyla]")
	}
	sub, rest := args[0], args[1:]
	if sub != "add" {
		return usageErr("unknown rule command %q", sub)
	}
	f, err := parse(rest,
		map[string]string{"dir": "dir", "target": "target", "file": "file", "snippet": "snippet", "dest": "dest"},
		map[string]string{"json": "json"})
	if err != nil {
		return err
	}
	reason := strings.TrimSpace(strings.Join(f.pos, " "))
	if reason == "" {
		return usageErr("hyla rule add <reason>")
	}
	dir := f.vals["dir"]
	if dir == "" && f.vals["target"] == "" {
		dir, _ = os.Getwd()
	}
	var res proto.RuleAddResult
	if err := c.Call("rule.add", proto.RuleAddParams{
		Target:  f.vals["target"],
		Dir:     dir,
		File:    f.vals["file"],
		Reason:  reason,
		Snippet: f.vals["snippet"],
		Dest:    f.vals["dest"],
	}, &res); err != nil {
		return err
	}
	if f.set["json"] {
		return printJSON(res)
	}
	if res.Added {
		fmt.Printf("added rule to %s: %s\n", res.Path, res.Content)
	} else {
		fmt.Printf("rule already present in %s\n", res.Path)
	}
	return nil
}
