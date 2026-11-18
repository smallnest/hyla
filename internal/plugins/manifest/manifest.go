package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const ManifestFileName = "hyla-plugin.toml"

// Manifest represents a parsed hyla-plugin.toml plugin declaration.
type Manifest struct {
	APIVersion  int               `toml:"api_version" json:"apiVersion"`
	ID          string            `toml:"id" json:"id"`
	Name        string            `toml:"name" json:"name"`
	Version     string            `toml:"version" json:"version"`
	Description string            `toml:"description" json:"description,omitempty"`
	Author      string            `toml:"author" json:"author,omitempty"`
	Homepage    string            `toml:"homepage" json:"homepage,omitempty"`
	Engines     EnginesSpec       `toml:"engines" json:"engines,omitempty"`
	Permissions PermissionsSpec   `toml:"permissions" json:"permissions,omitempty"`
	Daemon      *HostProcessSpec  `toml:"daemon" json:"daemon,omitempty"`
	App         *HostProcessSpec  `toml:"app" json:"app,omitempty"`
	Agents      []AgentSpec       `toml:"agents" json:"agents,omitempty"`
	Actions     []ActionSpec      `toml:"actions" json:"actions,omitempty"`
	UI          UISpec            `toml:"ui" json:"ui,omitempty"`
	Themes      []ThemeSpec       `toml:"themes" json:"themes,omitempty"`
	Washes      []WashSpec        `toml:"washes" json:"washes,omitempty"`
	Pets        []PetSpec         `toml:"pets" json:"pets,omitempty"`
	Hooks       []HookSpec        `toml:"hooks" json:"hooks,omitempty"`
	Skills      []SkillSpec       `toml:"skills" json:"skills,omitempty"`
	Settings    []SettingSpec     `toml:"settings" json:"settings,omitempty"`
	Env         map[string]string `toml:"env" json:"env,omitempty"`
}

type EnginesSpec struct {
	Hyla string `toml:"hyla" json:"hyla,omitempty"`
}

type PermissionsSpec struct {
	Scopes []string `toml:"scopes" json:"scopes,omitempty"`
}

// HostProcessSpec configures an out-of-process JSON-RPC host binary/script
// managed by hylad ([daemon]) or the desktop app ([app]).
type HostProcessSpec struct {
	Command             []string `toml:"command" json:"command,omitempty"`
	Transport           string   `toml:"transport" json:"transport,omitempty"` // "stdio" (default) | "socket"
	AutoStart           bool     `toml:"auto_start" json:"autoStart,omitempty"`
	KeepAlive           bool     `toml:"keep_alive" json:"keepAlive,omitempty"`
	MinRestartMS        int      `toml:"min_restart_ms" json:"minRestartMs,omitempty"`
	MaxCrashesPerMinute int      `toml:"max_crashes_per_minute" json:"maxCrashesPerMinute,omitempty"`
}

type AgentSpec struct {
	ID            string   `toml:"id" json:"id"`
	Name          string   `toml:"name" json:"name"`
	Detect        string   `toml:"detect" json:"detect,omitempty"`
	Mark          string   `toml:"mark" json:"mark,omitempty"`
	MarkFullColor bool     `toml:"mark_full_color" json:"markFullColor,omitempty"`
	Timeline      bool     `toml:"timeline" json:"timeline,omitempty"`
	Subagents     bool     `toml:"subagents" json:"subagents,omitempty"`
	Chat          bool     `toml:"chat" json:"chat,omitempty"`
	Processes     []string `toml:"processes" json:"processes,omitempty"`
	ResumeArgv    []string `toml:"resume_argv" json:"resumeArgv,omitempty"`
}

type ActionSpec struct {
	ID       string `toml:"id" json:"id"`
	Title    string `toml:"title" json:"title"`
	Menu     string `toml:"menu" json:"menu,omitempty"`
	Shortcut string `toml:"shortcut" json:"shortcut,omitempty"`
	Palette  bool   `toml:"palette" json:"palette,omitempty"`
	Remote   bool   `toml:"remote" json:"remote,omitempty"`
	When     string `toml:"when" json:"when,omitempty"`
	Target   string `toml:"target" json:"target,omitempty"` // "app" | "daemon" | "open_pane"
	View     string `toml:"view" json:"view,omitempty"`
}

type UISpec struct {
	Slots []SlotSpec `toml:"slots" json:"slots,omitempty"`
}

// Known UI slot mount points supported by [[ui.slots]].
const (
	MountInfoCard              = "info.card"
	MountSideTab               = "side.tab"
	MountPane                  = "pane"
	MountSidebarMenu           = "sidebar.menu"
	MountFilesMenu             = "files.menu"
	MountPaneMenu              = "pane.menu"
	MountPalette               = "palette"
	MountSettingsPlugins       = "settings.plugins"
	MountSettingsPage          = "settings.page"
	MountSettingsGeneral       = "settings.general"
	MountSettingsAppearance    = "settings.appearance"
	MountSettingsTerminal      = "settings.terminal"
	MountSettingsShell         = "settings.shell"
	MountSettingsBrowser       = "settings.browser"
	MountSettingsNotifications = "settings.notifications"
	MountSettingsKeyboard      = "settings.keyboard"
	MountSettingsFeishu        = "settings.feishu"
	MountSettingsAgents        = "settings.agents"
	MountSettingsSync          = "settings.sync"
	MountSettingsHosts         = "settings.hosts"
	MountSettingsAdvanced      = "settings.advanced"
)

// BuiltInSettingsMount reports whether mount targets a built-in Settings page
// section (e.g. "settings.general" -> "general"), excluding "settings.page"
// and "settings.plugins".
func BuiltInSettingsMount(mount string) (string, bool) {
	switch mount {
	case MountSettingsGeneral,
		MountSettingsAppearance,
		MountSettingsTerminal,
		MountSettingsShell,
		MountSettingsBrowser,
		MountSettingsNotifications,
		MountSettingsKeyboard,
		MountSettingsFeishu,
		MountSettingsAgents,
		MountSettingsSync,
		MountSettingsHosts,
		MountSettingsAdvanced:
		return strings.TrimPrefix(mount, "settings."), true
	default:
		return "", false
	}
}

type SlotSpec struct {
	ID      string `toml:"id" json:"id"`
	Mount   string `toml:"mount" json:"mount"` // "info.card", "side.tab", "pane", "sidebar.menu", "files.menu", "pane.menu", "palette", "settings.plugins", "settings.page", "settings.<page>"
	Title   string `toml:"title" json:"title"`
	Icon    string `toml:"icon" json:"icon,omitempty"`
	Mode    string `toml:"mode" json:"mode,omitempty"` // "declarative" | "webview"
	WebRoot string `toml:"web_root" json:"webRoot,omitempty"`
	Entry   string `toml:"entry" json:"entry,omitempty"`
	Host    string `toml:"host" json:"host,omitempty"` // "app" | "daemon"
	Order   int    `toml:"order" json:"order,omitempty"`
	When    string `toml:"when" json:"when,omitempty"`
	Action  string `toml:"action" json:"action,omitempty"`
}

type ThemeSpec struct {
	Name string `toml:"name" json:"name"`
	File string `toml:"file" json:"file"`
}

type WashSpec struct {
	ID   string `toml:"id" json:"id"`
	Name string `toml:"name" json:"name"`
	File string `toml:"file" json:"file"`
}

type PetSpec struct {
	ID  string `toml:"id" json:"id"`
	Dir string `toml:"dir" json:"dir"`
}

type HookSpec struct {
	Agent  string `toml:"agent" json:"agent"`
	Event  string `toml:"event" json:"event"`
	Script string `toml:"script" json:"script"`
}

type SkillSpec struct {
	Agent string `toml:"agent" json:"agent"`
	Name  string `toml:"name" json:"name"`
	Dir   string `toml:"dir" json:"dir"`
}

type SettingSpec struct {
	Key         string   `toml:"key" json:"key"`
	Title       string   `toml:"title" json:"title"`
	Description string   `toml:"description" json:"description,omitempty"`
	Type        string   `toml:"type" json:"type"` // "string", "secret", "boolean", "integer", "number", "select"
	Default     any      `toml:"default" json:"default,omitempty"`
	Options     []string `toml:"options" json:"options,omitempty"`
	Required    bool     `toml:"required" json:"required,omitempty"`
	Placeholder string   `toml:"placeholder" json:"placeholder,omitempty"`
}

// Parse decodes and validates TOML manifest bytes.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	md, err := toml.Decode(string(data), &m)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", ManifestFileName, err)
	}
	if len(md.Undecoded()) > 0 {
		// Ignore nothing blindly? Wait: only fail on clearly invalid schema, or keep forward compat?
		// Keep forward compat for unknown keys in minor revisions, validate known invariants strictly.
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadFile reads and parses hyla-plugin.toml inside pluginDir.
func LoadFile(pluginDir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(pluginDir, ManifestFileName))
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Validate checks manifest invariants, identifiers, permission scopes, and asset paths.
func (m *Manifest) Validate() error {
	if m.APIVersion != APIVersion {
		return fmt.Errorf("unsupported plugin api_version %d (supported: %d)", m.APIVersion, APIVersion)
	}
	if err := ValidateID(m.ID); err != nil {
		return err
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("plugin %q: name must not be empty", m.ID)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("plugin %q: version must not be empty", m.ID)
	}
	for _, s := range m.Permissions.Scopes {
		if !IsKnownPermission(s) {
			return fmt.Errorf("plugin %q: unknown permission scope %q", m.ID, s)
		}
	}
	if err := validateProcessSpec(m.ID, "daemon", m.Daemon); err != nil {
		return err
	}
	if err := validateProcessSpec(m.ID, "app", m.App); err != nil {
		return err
	}

	seenAgents := make(map[string]bool)
	for _, a := range m.Agents {
		if strings.TrimSpace(a.ID) == "" || strings.ContainsAny(a.ID, " /\\") {
			return fmt.Errorf("plugin %q: invalid agent id %q", m.ID, a.ID)
		}
		if seenAgents[a.ID] {
			return fmt.Errorf("plugin %q: duplicate agent id %q", m.ID, a.ID)
		}
		seenAgents[a.ID] = true
		if a.Detect != "" {
			if _, err := ValidateRelPath(a.Detect); err != nil {
				return fmt.Errorf("plugin %q agent %q detect: %w", m.ID, a.ID, err)
			}
		}
		if a.Mark != "" {
			if _, err := ValidateRelPath(a.Mark); err != nil {
				return fmt.Errorf("plugin %q agent %q mark: %w", m.ID, a.ID, err)
			}
		}
	}

	seenActions := make(map[string]bool)
	for i := range m.Actions {
		a := &m.Actions[i]
		if strings.TrimSpace(a.ID) == "" {
			return fmt.Errorf("plugin %q: action missing id", m.ID)
		}
		qid := QualifyActionID(m.ID, a.ID)
		if seenActions[qid] {
			return fmt.Errorf("plugin %q: duplicate action id %q", m.ID, qid)
		}
		seenActions[qid] = true
		if a.Target != "" && a.Target != "app" && a.Target != "daemon" && a.Target != "open_pane" {
			return fmt.Errorf("plugin %q action %q: invalid target %q", m.ID, a.ID, a.Target)
		}
	}

	seenSlots := make(map[string]bool)
	for i := range m.UI.Slots {
		s := &m.UI.Slots[i]
		if strings.TrimSpace(s.ID) == "" {
			return fmt.Errorf("plugin %q: ui slot missing id", m.ID)
		}
		if seenSlots[s.ID] {
			return fmt.Errorf("plugin %q: duplicate ui slot id %q", m.ID, s.ID)
		}
		seenSlots[s.ID] = true
		switch s.Mount {
		case MountInfoCard, MountSideTab, MountPane, MountSidebarMenu, MountFilesMenu, MountPaneMenu, MountPalette, MountSettingsPlugins, MountSettingsPage:
		default:
			if _, ok := BuiltInSettingsMount(s.Mount); !ok {
				return fmt.Errorf("plugin %q slot %q: unknown mount %q", m.ID, s.ID, s.Mount)
			}
		}
		if s.Mode != "" && s.Mode != "declarative" && s.Mode != "webview" {
			return fmt.Errorf("plugin %q slot %q: invalid mode %q", m.ID, s.ID, s.Mode)
		}
		if s.Host != "" && s.Host != "app" && s.Host != "daemon" {
			return fmt.Errorf("plugin %q slot %q: invalid host %q", m.ID, s.ID, s.Host)
		}
		if s.Icon != "" {
			if _, err := ValidateRelPath(s.Icon); err != nil {
				return fmt.Errorf("plugin %q slot %q icon: %w", m.ID, s.ID, err)
			}
		}
		if s.Mode == "webview" {
			if s.Mount != MountPane {
				return fmt.Errorf("plugin %q slot %q: mode %q only supports mount %q", m.ID, s.ID, s.Mode, MountPane)
			}
			if !m.HasPermission(PermUIWebView) {
				return fmt.Errorf("plugin %q slot %q uses webview mode without declaring %q permission", m.ID, s.ID, PermUIWebView)
			}
			if s.WebRoot != "" {
				if _, err := ValidateRelPath(s.WebRoot); err != nil {
					return fmt.Errorf("plugin %q slot %q web_root: %w", m.ID, s.ID, err)
				}
			}
			if s.Entry != "" {
				if _, err := ValidateRelPath(s.Entry); err != nil {
					return fmt.Errorf("plugin %q slot %q entry: %w", m.ID, s.ID, err)
				}
			}
		}
	}

	for _, th := range m.Themes {
		if strings.TrimSpace(th.Name) == "" {
			return fmt.Errorf("plugin %q: theme missing name", m.ID)
		}
		if _, err := ValidateRelPath(th.File); err != nil {
			return fmt.Errorf("plugin %q theme %q: %w", m.ID, th.Name, err)
		}
	}
	for _, w := range m.Washes {
		if err := ValidateToken("wash id", w.ID); err != nil {
			return fmt.Errorf("plugin %q: %w", m.ID, err)
		}
		if strings.TrimSpace(w.Name) == "" {
			return fmt.Errorf("plugin %q wash %q: missing name", m.ID, w.ID)
		}
		if _, err := ValidateRelPath(w.File); err != nil {
			return fmt.Errorf("plugin %q wash %q: %w", m.ID, w.ID, err)
		}
	}
	for _, p := range m.Pets {
		if err := ValidateToken("pet id", p.ID); err != nil {
			return fmt.Errorf("plugin %q: %w", m.ID, err)
		}
		if _, err := ValidateRelPath(p.Dir); err != nil {
			return fmt.Errorf("plugin %q pet %q: %w", m.ID, p.ID, err)
		}
	}
	for _, h := range m.Hooks {
		if err := ValidateToken("hook agent", h.Agent); err != nil {
			return fmt.Errorf("plugin %q: %w", m.ID, err)
		}
		if err := ValidateToken("hook event", h.Event); err != nil {
			return fmt.Errorf("plugin %q: %w", m.ID, err)
		}
		if _, err := ValidateRelPath(h.Script); err != nil {
			return fmt.Errorf("plugin %q hook %q: %w", m.ID, h.Event, err)
		}
	}
	for _, sk := range m.Skills {
		if err := ValidateToken("skill agent", sk.Agent); err != nil {
			return fmt.Errorf("plugin %q: %w", m.ID, err)
		}
		if err := ValidateToken("skill name", sk.Name); err != nil {
			return fmt.Errorf("plugin %q: %w", m.ID, err)
		}
		if _, err := ValidateRelPath(sk.Dir); err != nil {
			return fmt.Errorf("plugin %q skill %q: %w", m.ID, sk.Name, err)
		}
	}
	seenSettings := make(map[string]bool)
	for _, st := range m.Settings {
		if strings.TrimSpace(st.Key) == "" || strings.TrimSpace(st.Title) == "" {
			return fmt.Errorf("plugin %q: setting missing key or title", m.ID)
		}
		if seenSettings[st.Key] {
			return fmt.Errorf("plugin %q: duplicate setting key %q", m.ID, st.Key)
		}
		seenSettings[st.Key] = true
		switch st.Type {
		case "", "string", "secret", "boolean", "integer", "number", "select":
		default:
			return fmt.Errorf("plugin %q setting %q: invalid type %q", m.ID, st.Key, st.Type)
		}
	}
	return nil
}

func validateProcessSpec(pluginID, host string, spec *HostProcessSpec) error {
	if spec == nil {
		return nil
	}
	if len(spec.Command) == 0 || strings.TrimSpace(spec.Command[0]) == "" {
		return fmt.Errorf("plugin %q [%s]: command must not be empty", pluginID, host)
	}
	switch spec.Transport {
	case "", "stdio", "socket":
	default:
		return fmt.Errorf("plugin %q [%s]: invalid transport %q", pluginID, host, spec.Transport)
	}
	if strings.Contains(spec.Command[0], `\`) {
		return fmt.Errorf("plugin %q [%s]: command path %q must use forward slashes", pluginID, host, spec.Command[0])
	}
	// If command looks relative with a path separator, require it not to escape root.
	if strings.ContainsRune(spec.Command[0], '/') && !filepath.IsAbs(spec.Command[0]) {
		if _, err := ValidateRelPath(spec.Command[0]); err != nil {
			return fmt.Errorf("plugin %q [%s] command[0]: %w", pluginID, host, err)
		}
	}
	return nil
}

// QualifyActionID qualifies actionID under pluginID unless it already uses
// the pluginID prefix or "plugin." namespace.
func QualifyActionID(pluginID, actionID string) string {
	if strings.HasPrefix(actionID, pluginID+".") || strings.HasPrefix(actionID, "plugin.") {
		return actionID
	}
	return pluginID + "." + actionID
}

// HasPermission reports whether the manifest declared scope.
func (m *Manifest) HasPermission(scope string) bool {
	return slices.Contains(m.Permissions.Scopes, scope)
}

// CompatibleWith checks Engines.Hyla against currentVersion (semver minimal check, e.g. ">=3.0.0").
func (m *Manifest) CompatibleWith(currentVersion string) error {
	req := strings.TrimSpace(m.Engines.Hyla)
	if req == "" || currentVersion == "" {
		return nil
	}
	cur, ok := parseSemver(currentVersion)
	if !ok {
		return nil
	}
	for _, clause := range strings.Fields(strings.ReplaceAll(req, ",", " ")) {
		op := "="
		verStr := clause
		for _, prefix := range []string{">=", "<=", ">", "<", "=", "^"} {
			if strings.HasPrefix(clause, prefix) {
				op = prefix
				verStr = strings.TrimPrefix(clause, prefix)
				break
			}
		}
		target, ok := parseSemver(verStr)
		if !ok {
			continue
		}
		cmp := compareSemver(cur, target)
		switch op {
		case ">=":
			if cmp < 0 {
				return fmt.Errorf("plugin %q requires Hyla %s (running %s)", m.ID, req, currentVersion)
			}
		case ">":
			if cmp <= 0 {
				return fmt.Errorf("plugin %q requires Hyla %s (running %s)", m.ID, req, currentVersion)
			}
		case "<=":
			if cmp > 0 {
				return fmt.Errorf("plugin %q requires Hyla %s (running %s)", m.ID, req, currentVersion)
			}
		case "<":
			if cmp >= 0 {
				return fmt.Errorf("plugin %q requires Hyla %s (running %s)", m.ID, req, currentVersion)
			}
		case "^":
			if cmp < 0 || cur[0] != target[0] {
				return fmt.Errorf("plugin %q requires Hyla %s (running %s)", m.ID, req, currentVersion)
			}
		default:
			if cmp != 0 {
				return fmt.Errorf("plugin %q requires Hyla %s (running %s)", m.ID, req, currentVersion)
			}
		}
	}
	return nil
}

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i := 0; i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

func compareSemver(a, b [3]int) int {
	for i := range 3 {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
