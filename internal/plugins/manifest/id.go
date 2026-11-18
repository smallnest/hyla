// Package manifest defines the Hyla plugin manifest (hyla-plugin.toml),
// plugin registry (plugins.json), isolated storage layout, permission scopes,
// and safe .hylapkg packaging and extraction.
package manifest

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const APIVersion = 1

// idPattern matches DNS-safe reverse-domain plugin identifiers (no underscores).
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9](\.[a-z0-9][a-z0-9-]{0,61}[a-z0-9])+$`)

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidateToken verifies that an asset identifier (agent, event, skill, wash, pet)
// contains only safe identifier characters and no path traversal or shell metacharacters.
func ValidateToken(kind, val string) error {
	if val == "" {
		return fmt.Errorf("%s must not be empty", kind)
	}
	if strings.Contains(val, "..") || !tokenPattern.MatchString(val) {
		return fmt.Errorf("%s %q contains invalid characters (allowed: A-Z, a-z, 0-9, '.', '_', '-'; no '..')", kind, val)
	}
	return nil
}

// Permission scopes enforced across Daemon Host and App Host RPC gateways.
const (
	PermSessionRead         = "session.read"
	PermPaneRead            = "pane.read"
	PermPaneWrite           = "pane.write"
	PermPaneFocus           = "pane.focus"
	PermSessionCreate       = "session.create"
	PermAgentProvider       = "agent.provider"
	PermTimelineRead        = "timeline.read"
	PermUISlot              = "ui.slot"
	PermUIWebView           = "ui.webview"
	PermNetwork             = "network"
	PermSecrets             = "secrets"
	PermNotifications       = "notifications"
	PermFilesystemWorkspace = "filesystem.workspace"
)

// PermissionInfo documents a known plugin permission scope.
type PermissionInfo struct {
	Scope       string
	HighRisk    bool
	Description string
}

// KnownPermissions lists all valid permission scopes supported by Hyla.
var KnownPermissions = []PermissionInfo{
	{Scope: PermSessionRead, Description: "Read workspace projects, sessions, and panes."},
	{Scope: PermPaneRead, Description: "Read pane terminal output, screen text, and environment."},
	{Scope: PermPaneWrite, HighRisk: true, Description: "Send input text or keys into terminal panes."},
	{Scope: PermPaneFocus, Description: "Focus sessions or switch active panes."},
	{Scope: PermSessionCreate, HighRisk: true, Description: "Create new sessions or split panes."},
	{Scope: PermAgentProvider, Description: "Register custom coding agents and resume commands."},
	{Scope: PermTimelineRead, Description: "Access agent timelines, chats, and subagent records."},
	{Scope: PermUISlot, Description: "Render native UI cards, side tabs, custom panes, menus, palette actions, and Settings extensions."},
	{Scope: PermUIWebView, Description: "Load sandboxed WebView panes via hylaplugin://."},
	{Scope: PermNetwork, Description: "Perform outbound network requests (declared by plugins)."},
	{Scope: PermSecrets, HighRisk: true, Description: "Store and read plugin secrets in credential storage."},
	{Scope: PermNotifications, Description: "Show toast or OS notifications in the app."},
	{Scope: PermFilesystemWorkspace, HighRisk: true, Description: "Access workspace files referenced by panes."},
}

var knownPermSet = func() map[string]PermissionInfo {
	m := make(map[string]PermissionInfo, len(KnownPermissions))
	for _, p := range KnownPermissions {
		m[p.Scope] = p
	}
	return m
}()

// IsKnownPermission reports whether scope is one of the recognized permission scopes.
func IsKnownPermission(scope string) bool {
	_, ok := knownPermSet[scope]
	return ok
}

// DescribePermission returns metadata about a known permission scope.
func DescribePermission(scope string) (PermissionInfo, bool) {
	info, ok := knownPermSet[scope]
	return info, ok
}

// ValidateID checks that id is a valid DNS-safe reverse-domain plugin identifier.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("plugin id must not be empty")
	}
	if len(id) > 128 {
		return fmt.Errorf("plugin id %q exceeds 128 characters", id)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("plugin id %q must use reverse-domain DNS-safe format (lowercase a-z, 0-9, '-', dot-separated, at least 2 segments)", id)
	}
	return nil
}

// ValidateRelPath verifies a manifest-relative path is clean, non-empty,
// relative, slash-safe, and never escapes its root with "..".
func ValidateRelPath(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("relative path must not be empty")
	}
	if strings.ContainsRune(rel, 0) {
		return "", fmt.Errorf("relative path contains NUL byte")
	}
	if strings.Contains(rel, `\`) {
		return "", fmt.Errorf("relative path %q must use forward slashes", rel)
	}
	if strings.HasPrefix(rel, "/") || (len(rel) >= 2 && rel[1] == ':') {
		return "", fmt.Errorf("path %q must be relative", rel)
	}
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path %q escapes plugin root", rel)
	}
	return cleaned, nil
}

// ResolveWithin joins root and a validated relative path and confirms the result
// stays inside root (using filepath.EvalSymlinks where components exist).
func ResolveWithin(root, rel string) (string, error) {
	cleaned, err := ValidateRelPath(rel)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolvedRoot, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolvedRoot
	}
	joined := filepath.Join(absRoot, filepath.FromSlash(cleaned))
	relCheck, err := filepath.Rel(absRoot, joined)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes plugin root %q", rel, root)
	}
	if resolvedJoined, err := filepath.EvalSymlinks(joined); err == nil {
		relReal, err := filepath.Rel(absRoot, resolvedJoined)
		if err != nil || relReal == ".." || strings.HasPrefix(relReal, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("symlink %q escapes plugin root %q", rel, root)
		}
		return resolvedJoined, nil
	}
	return joined, nil
}
