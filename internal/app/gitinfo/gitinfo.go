// Package gitinfo says a pane's or a session's worktree in words: its
// branch, how far it is from its upstream, and what changed.
package gitinfo

import (
	"strconv"
	"strings"

	"hyla/internal/proto"
	"hyla/locales"
)

// BranchName is a worktree's branch, or its commit while detached.
func BranchName(g *proto.GitInfo) string {
	if g.Branch != "" {
		return g.Branch
	}
	if len(g.Head) >= 7 {
		return g.Head[:7]
	}
	return g.Head
}

// State names an operation in progress.
func State(st string) string {
	if say, ok := map[string]func() string{
		"merge": locales.GitMerging, "rebase": locales.GitRebasing, "cherry-pick": locales.GitCherryPicking, "revert": locales.GitReverting,
	}[st]; ok {
		return say()
	}
	return ""
}

// Summary says a worktree's status in a line: its branch, how far it
// is from its upstream, and what changed, as "fix-login ↑2 · 3 files +40 −2".
func Summary(g *proto.GitInfo) string {
	var b strings.Builder
	b.WriteString(BranchName(g))
	if g.Ahead > 0 {
		b.WriteString(" ↑" + strconv.Itoa(g.Ahead))
	}
	if g.Behind > 0 {
		b.WriteString(" ↓" + strconv.Itoa(g.Behind))
	}
	if st := State(g.State); st != "" {
		b.WriteString(" · " + st)
	}
	if g.Files > 0 {
		b.WriteString(" · " + locales.ChangesFiles(g.Files))
		if g.Added > 0 {
			b.WriteString(" +" + strconv.Itoa(g.Added))
		}
		if g.Removed > 0 {
			b.WriteString(" −" + strconv.Itoa(g.Removed))
		}
	}
	return b.String()
}

// Session is the worktree a session's row tells of: its lead pane's,
// else the one of the pane the user was in last, else any of its panes'.
func Session(s *proto.Session) *proto.GitInfo {
	if s.Git != nil {
		return s.Git
	}
	if p := s.FocusPane(); p != nil && p.Git != nil {
		return p.Git
	}
	for i := range s.Panes {
		if s.Panes[i].Git != nil {
			return s.Panes[i].Git
		}
	}
	return nil
}

// OffBranch is the branch to show by a session's name in the sidebar: one
// other than its repository's default, as an agent's in a worktree of its
// own; "" for the default, which every other session is on.
func OffBranch(g *proto.GitInfo) string {
	if g == nil || g.Default == "" || g.Branch == g.Default {
		return ""
	}
	return BranchName(g)
}
