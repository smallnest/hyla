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

func claimCmd(c *client.Client, st proto.State, args []string) error {
	if len(args) == 0 {
		return usageErr("hyla claim <add|ls|release|check> [args…]")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		f, err := parse(rest,
			map[string]string{"target": "target", "session": "target", "s": "target", "repo": "repo", "mode": "mode", "reason": "reason", "ttl": "ttl"},
			map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) == 0 {
			return usageErr("hyla claim add <pattern> [--target pane] [--mode warn|block]")
		}
		targetRef := f.vals["target"]
		if targetRef == "" {
			targetRef = current()
		}
		if targetRef == "" {
			if t, err := findOrCurrent(st, nil); err == nil {
				targetRef = t.ref()
			}
		}
		var ttl int64
		if f.vals["ttl"] != "" {
			v, err := strconv.ParseInt(f.vals["ttl"], 10, 64)
			if err != nil || v < 0 {
				return usageErr("invalid --ttl %q", f.vals["ttl"])
			}
			ttl = v
		}
		var out proto.PathClaim
		if err := c.Call("claim.add", proto.PathClaimParams{
			Target:     targetRef,
			Repo:       f.vals["repo"],
			Pattern:    f.pos[0],
			Mode:       f.vals["mode"],
			Reason:     f.vals["reason"],
			TTLSeconds: ttl,
		}, &out); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(out)
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", out.ID, out.Mode, out.Pattern, out.Repo)
		return nil

	case "ls", "list":
		f, err := parse(rest, map[string]string{"repo": "repo"}, map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		var claims []proto.PathClaim
		if err := c.Call("claim.list", proto.PathClaimListParams{Repo: f.vals["repo"]}, &claims); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(claims)
		}
		if len(claims) == 0 {
			fmt.Println("no active path claims")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tMODE\tPATTERN\tPANE\tAGENT\tREASON")
		for _, cl := range claims {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", cl.ID, cl.Mode, cl.Pattern, cl.PaneID, cl.Agent, cl.Reason)
		}
		return w.Flush()

	case "release", "rm":
		if len(rest) < 1 {
			return usageErr("hyla claim release <claim-id>")
		}
		return c.Call("claim.release", proto.PathClaimReleaseParams{ID: rest[0]}, nil)

	case "check":
		f, err := parse(rest,
			map[string]string{"pane": "pane", "repo": "repo", "tool": "tool"},
			map[string]string{"json": "json"})
		if err != nil {
			return err
		}
		if len(f.pos) == 0 {
			return usageErr("hyla claim check <path> [--pane id] [--repo dir]")
		}
		paneID := f.vals["pane"]
		if paneID == "" {
			paneID = current()
		}
		var res proto.PathClaimCheckResult
		if err := c.Call("claim.check", proto.PathClaimCheckParams{
			Pane: paneID,
			Repo: f.vals["repo"],
			Path: f.pos[0],
			Tool: f.vals["tool"],
		}, &res); err != nil {
			return err
		}
		if f.set["json"] {
			return printJSON(res)
		}
		if len(res.Conflicts) == 0 {
			fmt.Println("ok (no conflicts)")
			return nil
		}
		state := "warning"
		if !res.Allowed {
			state = "blocked"
		}
		fmt.Printf("%s: %s\n", state, res.Message)
		return nil

	default:
		return usageErr("unknown claim command %q", sub)
	}
}

func clashCmd(c *client.Client, _ proto.State, args []string) error {
	if len(args) == 0 {
		return usageErr("hyla clash check [--repo dir] [--base branch] [--branches a,b] [--cmd cmd]")
	}
	sub, rest := args[0], args[1:]
	if sub != "check" {
		return usageErr("unknown clash command %q", sub)
	}
	f, err := parse(rest,
		map[string]string{"repo": "repo", "base": "base", "branches": "branches", "cmd": "cmd"},
		map[string]string{"json": "json"})
	if err != nil {
		return err
	}
	repo := f.vals["repo"]
	if repo == "" {
		repo, _ = os.Getwd()
	}
	var branches []string
	if b := strings.TrimSpace(f.vals["branches"]); b != "" {
		for _, part := range strings.Split(b, ",") {
			if part = strings.TrimSpace(part); part != "" {
				branches = append(branches, part)
			}
		}
	}
	var res proto.ClashCheckResult
	if err := c.Call("clash.check", proto.ClashCheckParams{
		Repo:     repo,
		Base:     f.vals["base"],
		Branches: branches,
		CheckCmd: f.vals["cmd"],
	}, &res); err != nil {
		return err
	}
	if f.set["json"] {
		return printJSON(res)
	}
	if res.Passed {
		fmt.Printf("ok: trial merge clean (%d branches on %s, %dms)\n", len(res.Branches), res.Base, res.DurationMs)
		return nil
	}
	if len(res.ConflictFiles) > 0 {
		fmt.Printf("clash: conflicts in %s\n", strings.Join(res.ConflictFiles, ", "))
	}
	if res.CheckError != "" {
		fmt.Printf("error: %s\n", res.CheckError)
	}
	return fmt.Errorf("clash check failed")
}
