// Package settings is hyla's settings: one file, settings.json in hyla's
// directory, that the app writes from its Settings window and hylad reads
// as it starts panes, so that the shell, its environment and the
// scrollback apply to new panes at once.
package settings

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"hyla/internal/adblock"
	"hyla/internal/proto"
)

// Settings are the user's choices. The zero value of a field is never a
// choice of its own: Load fills defaults in.
type Settings struct {
	// General
	NewSessionDir     string `json:"newSessionDir"`     // "current", "project" or "home"
	SessionName       string `json:"sessionName"`       // what names an unnamed session: proto.NameFocused, NameLead or NameFolder
	SidebarRows       string `json:"sidebarRows"`       // "detailed", two lines a session, or "compact", one
	FocusFollowsMouse bool   `json:"focusFollowsMouse"` // a pane the pointer enters takes the focus
	ConfirmClose      bool   `json:"confirmClose"`      // before closing a pane a program runs in
	RestoreSessions   bool   `json:"restoreSessions"`   // after hylad restarts, as after a reboot
	ResumeAgents      bool   `json:"resumeAgents"`      // as it restores sessions: the agents' own
	QuitEndsSessions  bool   `json:"quitEndsSessions"`  // quitting ends this machine's sessions, as app.quitAll
	SessionHistory    bool   `json:"sessionHistory"`    // the sidebar lists the agents' ended conversations
	QuitConfirm       bool   `json:"quitConfirm"`       // tell, the first time, that quitting keeps sessions
	AppIcon           string `json:"appIcon"`           // macOS: AppIcons; elsewhere the icon is always in the notification area

	// Appearance
	Appearance    string  `json:"appearance"`    // "system", "light" or "dark"
	Language      string  `json:"language"`      // "system", or one of Languages
	Background    string  `json:"background"`    // a wash, see Backgrounds
	Vibrancy      bool    `json:"vibrancy"`      // the window's material under the wash
	WindowOpacity float32 `json:"windowOpacity"` // how much of the wash covers the window, 0 to 1
	WindowBlur    bool    `json:"windowBlur"`    // blur the desktop behind the wash
	// WindowMaterial is the window's material under the wash: "sidebar",
	// "window", "popover", "glass" or "glass-clear".
	WindowMaterial string `json:"windowMaterial"`
	// SidebarImage is a picture of this machine shown behind the sidebar,
	// as tall as 80% of it, against its left edge: an image file's path,
	// "" for none. SidebarImageOpacity is how much of it shows, 0 to 1.
	SidebarImage        string  `json:"sidebarImage"`
	SidebarImageOpacity float32 `json:"sidebarImageOpacity"`
	// PaneImage is a picture of this machine shown in the terminals'
	// cards, under their text, in place of the window's background: an
	// image file's path, "" for none. PaneImageOpacity is how much of it
	// shows, 0 to 1. PaneImageFit is how it fills the card, as PaneImageFits.
	PaneImage        string  `json:"paneImage"`
	PaneImageOpacity float32 `json:"paneImageOpacity"`
	PaneImageFit     string  `json:"paneImageFit"`
	PaneOpacity      float32 `json:"paneOpacity"` // how much of the window's background shows in a terminal, 0 to 1
	PaneBlur         bool    `json:"paneBlur"`    // blur what shows through the terminals
	// PaneMaterial is the terminal's glass where PaneBlur is on: "glass"
	// (Liquid Glass) or "glass-clear".
	PaneMaterial string `json:"paneMaterial"`
	// SessionSwitch is how a session shown in place of another comes in,
	// as SessionSwitches.
	SessionSwitch string  `json:"sessionSwitch"`
	Pet           bool    `json:"pet"`       // the frog on the window's top edge
	PetOffset     float32 `json:"petOffset"` // where it stands, 0 to 1 across the window
	PetSet        string  `json:"petSet"`    // who: "hyla", a Codex pet's directory name, or "codex" for the active one
	PetSize       float32 `json:"petSize"`   // how tall a Codex pet shows, against the Codex app's frame

	// Terminal
	FontFamily string `json:"fontFamily"` // "" the font hyla carries; "monospace" the system's
	// FontFallback is the families after FontFamily, which draw what it
	// lacks: Chinese in a Chinese font, as "PingFang SC, Apple Color Emoji".
	// It is kept as written, so that "" asks for the system's choice.
	FontFallback string  `json:"fontFallback,omitempty"`
	FontSize     float32 `json:"fontSize"`
	LineHeight   float32 `json:"lineHeight"`
	// CellWidth is the width of terminal cells, a multiple of the font's:
	// 1 keeps its grid, and other values space characters out or bring
	// them together without scaling them, as Ghostty's adjust-cell-width.
	CellWidth  float32 `json:"cellWidth"`
	FontWeight int     `json:"fontWeight"`
	Ligatures  bool    `json:"ligatures"`
	// FontFeatures turn the font's OpenType features on and off, as
	// Ghostty's font-feature: "ss01", "-calt", "cv05=2". They apply
	// after Ligatures, so that they can turn one back on.
	FontFeatures []string `json:"fontFeatures,omitempty"`
	Thicken      bool     `json:"thicken"`
	MinContrast  float32  `json:"minContrast"`           // a WCAG ratio text keeps with its background
	DimUnfocused bool     `json:"dimUnfocusedTerminals"` // the terminals the user is not in fade
	Cursor       string   `json:"cursor"`                // "block", "bar" or "underline"
	CursorBlink  bool     `json:"cursorBlink"`
	LightTheme   string   `json:"lightTheme"` // "hyla", or a Ghostty theme's name
	DarkTheme    string   `json:"darkTheme"`
	OptionAsAlt  bool     `json:"optionAsAlt"`
	CopyOnSelect bool     `json:"copyOnSelect"`
	Clipboard    string   `json:"clipboard"`  // programs reading it: "ask", "allow" or "deny"
	RightClick   string   `json:"rightClick"` // "menu", "paste" or "copyOrPaste"
	// ScrollPrecision scales a touchpad's scrolling, and ScrollDiscrete is
	// the rows a notch of a mouse wheel scrolls, as Ghostty's
	// mouse-scroll-multiplier = precision:…,discrete:….
	ScrollPrecision float32 `json:"scrollPrecision"`
	ScrollDiscrete  float32 `json:"scrollDiscrete"`

	// Shell
	Shell      string   `json:"shell"` // "" the account's login shell
	Env        []string `json:"env"`   // "KEY=value", added to every pane's
	Scrollback int      `json:"scrollback"`

	// Feishu's cards
	FeishuCards      bool `json:"feishuCards"`      // a turn's card updates as the agent works
	FeishuMarkdown   bool `json:"feishuMarkdown"`   // cards' text as Markdown
	FeishuOpenButton bool `json:"feishuOpenButton"` // a card's button opens its session in Hyla
	FeishuYolo       bool `json:"feishuYolo"`       // every agent from Feishu runs in its yolo mode, as feishu-yolo

	// Notifications
	NotifyWaiting bool `json:"notifyWaiting"` // a session needs you
	NotifyDone    bool `json:"notifyDone"`    // an agent finished
	NotifyFailed  bool `json:"notifyFailed"`  // a program reported it failed
	NotifySound   bool `json:"notifySound"`
	DockBadge     bool `json:"dockBadge"`
	BounceDock    bool `json:"bounceDock"`

	// Agents
	RemoteControl bool `json:"remoteControl"` // hyla focus and hyla do drive the app
	// Feishu drives the sessions from Feishu chats (package feishu).
	Feishu *Feishu `json:"feishu,omitempty"`

	// Git
	GitStatus  bool   `json:"gitStatus"`  // each pane's branch and changes, which hylad follows
	DiffLayout string `json:"diffLayout"` // the changes panel's diffs: "auto", "unified" or "split"
	// WorktreeBase is what a session's own worktree branches from:
	// "default", the repository's default branch, or "current", the
	// branch of the session the user is in. WorktreeDir is where its
	// worktrees go, "" for .hyla/worktrees in the repository.
	WorktreeBase string `json:"worktreeBase"`
	WorktreeDir  string `json:"worktreeDir"`

	// Browser
	BlockAds bool     `json:"blockAds"` // in browser panes, with AdLists
	AdLists  []string `json:"adLists"`  // the filter lists: adblock.Catalog's IDs, or URLs

	// Keyboard: accelerators by action, over the defaults; "" unbinds.
	Shortcuts map[string]string `json:"shortcuts,omitempty"`

	// Listen are hylad's listeners, for clients on other machines
	// (tailcat://:7473), admitted by the pair code of the key that makes
	// one.
	Listen []string `json:"listen,omitempty"`
	// Tailcat is hylad's tailcat listener, which serves HylaPocket: off,
	// it starts none, and the app shows no pair code. TailcatRelay is the
	// DERP map where tailcat finds relays: "" for
	// remote.DefaultDERPMapURL, Tailscale's public ones.
	Tailcat      bool   `json:"tailcat"`
	TailcatRelay string `json:"tailcatRelay,omitempty"`
	// TailcatRotate makes hylad start its tailcat listener with the key
	// again, whose pair code changed: this machine's, as `hyla tailcat
	// rotate` sets it.
	TailcatRotate bool `json:"tailcatRotate,omitempty"`
}

// DefaultWindowOpacity is how much of the window's background the wash
// covers while translucent.
const DefaultWindowOpacity = 0.82

// DefaultSidebarImageOpacity is how much of the sidebar's picture shows:
// faintly, as a veil under the sessions.
const DefaultSidebarImageOpacity = 0.35

// DefaultPaneOpacity is how much of the window's background shows in a
// terminal while the window is translucent.
const DefaultPaneOpacity = 0.82

// DefaultPaneImageOpacity is how much of a terminal's picture shows:
// faintly, as a veil under the text.
const DefaultPaneImageOpacity = 0.6

// PaneImageFits are how a terminal's picture fills its card, as a macOS
// desktop picture's options: "fill" as wide as the card, from its top;
// "fit" all of it, its shape kept; "stretch" to the card's edges;
// "center" its own size at the card's middle; "tile" its own size from
// there.
var PaneImageFits = []string{"fill", "fit", "stretch", "center", "tile"}

// DefaultPaneImageFit is how a terminal's picture fills its card: as
// large as the card covers, its top at the card's.
const DefaultPaneImageFit = "fill"

// SessionSwitches are how a session shown in place of another comes in, as
// a slide transition does: "dissolve" fades it in as the one it leaves
// blurs and fades away over it; "slide" brings it in from the right;
// "push" slides it in from the right, pushing the one it leaves out to the
// left; "cover" brings it in from the right, over the one it leaves, which
// stays; "zoom" grows it into place from a little smaller, fading in; and
// "none" shows it at once.
var SessionSwitches = []string{"dissolve", "slide", "push", "cover", "zoom", "none"}

// DefaultSessionSwitch is how a session shown in place of another comes
// in: fading in, lifting a little, springing, as the one it leaves blurs
// and fades away over it.
const DefaultSessionSwitch = "dissolve"

// PaneBlurRadius is how much a terminal blurs what shows through it, in
// DIPs, with terminal blur on: a frost that hides what is under it, where
// the window's own blur, about 10 DIPs, hardly shows in a card.
const PaneBlurRadius = 16

// Default returns the settings of a new install.
func Default() Settings {
	return Settings{
		NewSessionDir:       "current",
		SessionName:         proto.NameFocused,
		SidebarRows:         "detailed",
		ConfirmClose:        true,
		RestoreSessions:     true,
		ResumeAgents:        true,
		QuitConfirm:         true,
		AppIcon:             "dock-and-menubar",
		Appearance:          "system",
		Language:            "system",
		Background:          "rex",
		Vibrancy:            true,
		WindowOpacity:       DefaultWindowOpacity,
		WindowBlur:          true,
		WindowMaterial:      "sidebar",
		SidebarImageOpacity: DefaultSidebarImageOpacity,
		PaneImageOpacity:    DefaultPaneImageOpacity,
		PaneImageFit:        DefaultPaneImageFit,
		PaneOpacity:         DefaultPaneOpacity,
		PaneMaterial:        "glass",
		Pet:                 true,
		PetOffset:           0.5,
		PetSet:              PetSetHyla,
		PetSize:             DefaultPetSize,
		DimUnfocused:        true,
		FontSize:            13,
		LineHeight:          1.22,
		CellWidth:           1,
		SessionSwitch:       DefaultSessionSwitch,
		FontWeight:          400,
		Ligatures:           true,
		MinContrast:         3,
		Cursor:              "block",
		CursorBlink:         true,
		LightTheme:          "hyla",
		DarkTheme:           "hyla",
		Clipboard:           "ask",
		RightClick:          "menu",
		ScrollPrecision:     1,
		ScrollDiscrete:      3,
		Scrollback:          4,
		NotifyWaiting:       true,
		NotifyDone:          true,
		NotifyFailed:        true,
		DockBadge:           true,
		RemoteControl:       true,
		FeishuCards:         true,
		FeishuMarkdown:      true,
		FeishuOpenButton:    true,
		FeishuYolo:          true,
		GitStatus:           true,
		DiffLayout:          "auto",
		WorktreeBase:        "default",
		BlockAds:            true,
		AdLists:             slices.Clone(adblock.DefaultLists),
	}
}

// MinScrollMultiplier and MaxScrollMultiplier bound ScrollPrecision and
// ScrollDiscrete, as Ghostty bounds mouse-scroll-multiplier.
const (
	MinScrollMultiplier = 0.01
	MaxScrollMultiplier = 10000
)

// Languages are the languages the app is written in, as the catalogs of
// hyla/locales name them.
var Languages = []string{"en", "ja", "zh-Hans"}

// AppIcons are where the app's icon shows on macOS: the Dock and the menu
// bar, the Dock alone, or the menu bar alone.
var AppIcons = []string{"dock-and-menubar", "dock", "menubar"}

// Backgrounds are the washes the window can show.
var Backgrounds = []struct{ ID, Name string }{
	{"rex", "Petal"},
	{"mist", "Mist"},
	{"meadow", "Meadow"},
	{"dock", "Dock"},
	{"ember", "Ember"},
	{"tidal", "Tidal"},
	{"umbra", "Umbra"},
	{"static", "Static"},
	{"plain", "Plain"},
}

// Path returns the settings' file, the configuration (ConfigPath).
func Path() string { return ConfigPath() }

// legacyPath is where hyla kept its settings before the configuration
// file, as JSON, which the first Load moves into it.
func legacyPath() string { return filepath.Join(proto.Dir(), "settings.json") }

var (
	mu       sync.Mutex
	cache    *Settings
	mtime    int64
	problems []string
)

// Load returns the settings, the defaults where the file says nothing.
// It reads the file again when it changed, so that a long-running reader,
// as hylad, sees the app's changes, and the user's.
func Load() Settings {
	mu.Lock()
	defer mu.Unlock()
	migrate()
	// The file the links lead to reads as write's file does: a
	// configuration kept in a dotfiles repository changes there.
	path := Target()
	st, err := os.Stat(path)
	if err != nil {
		cache, problems = nil, nil
		return Default()
	}
	if cache != nil && st.ModTime().UnixNano() == mtime {
		return clone(*cache)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Default()
	}
	s, probs := parse(string(b))
	cache, mtime, problems = &s, st.ModTime().UnixNano(), probs
	return clone(s)
}

// Reload forgets the last Load: the settings are read again, as a
// test's HYLA_HOME takes another configuration. It is no key.
func Reload() {
	mu.Lock()
	defer mu.Unlock()
	cache = nil
}

// Problems returns what the file got wrong, as of the last Load: keys
// hyla does not know, and values it cannot take, which keep their
// defaults.
func Problems() []string {
	mu.Lock()
	defer mu.Unlock()
	return slices.Clone(problems)
}

// migrate moves the settings of the JSON file hyla kept before into the
// configuration, once; mu is held.
func migrate() {
	if _, err := os.Stat(Path()); err == nil {
		return
	}
	b, err := os.ReadFile(legacyPath())
	if err != nil {
		return
	}
	s := Default()
	if json.Unmarshal(b, &s) != nil {
		return
	}
	s.normalize()
	if write(s) == nil {
		os.Rename(legacyPath(), legacyPath()+".migrated")
	}
}

// Save writes the settings into the configuration, keeping what the user
// wrote around them (see format).
func Save(s Settings) error {
	mu.Lock()
	defer mu.Unlock()
	s.normalize()
	if err := write(s); err != nil {
		return err
	}
	c := clone(s)
	cache, problems = &c, nil
	if st, err := os.Stat(Path()); err == nil {
		mtime = st.ModTime().UnixNano()
	}
	return nil
}

// write writes the configuration; mu is held. A configuration that is a
// symlink, as one kept in a dotfiles repository is, stays one: the file it
// points to is what changes.
func write(s Settings) error {
	path := Target()
	old, _ := os.ReadFile(path)
	data := format(s, string(old))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Target returns the file the configuration is in: Path, or the file it
// is a symlink to, as one kept in a dotfiles repository.
func Target() string { return target(Path()) }

// target returns the file a path's symlinks lead to, even one not there
// yet, else the path itself.
func target(path string) string {
	for range 40 {
		dest, err := os.Readlink(path)
		if err != nil {
			return path
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(path), dest)
		}
		path = dest
	}
	return path
}

// Clone returns a copy that shares nothing with s.
func (s Settings) Clone() Settings { return clone(s) }

// ScrollbackBytes is a terminal's terminal.Options.Scrollback for the
// scrollback-limit, the same in hylad's copy of a pane and the app's: -1
// keeps none, as 0 would take the terminal's default.
func (s Settings) ScrollbackBytes() int {
	if s.Scrollback == 0 {
		return -1
	}
	return s.Scrollback << 20
}

func clone(s Settings) Settings {
	s.Env = append([]string(nil), s.Env...)
	s.FontFeatures = slices.Clone(s.FontFeatures)
	s.AdLists = slices.Clone(s.AdLists)
	s.Listen = slices.Clone(s.Listen)
	if s.Feishu != nil {
		f := *s.Feishu
		f.Allow = append([]string(nil), f.Allow...)
		f.Host = slices.Clone(f.Host)
		if f.Agents != nil {
			f.Agents = maps.Clone(f.Agents)
		}
		s.Feishu = &f
	}
	if s.Shortcuts != nil {
		m := make(map[string]string, len(s.Shortcuts))
		for k, v := range s.Shortcuts {
			m[k] = v
		}
		s.Shortcuts = m
	}
	return s
}

// ParseFontFeatures reads OpenType features as font-feature takes them,
// as Ghostty's: tags of four characters, comma-separated, each turned on
// as ss01 or +ss01, off as -calt or calt=0, or set as cv05=2. It returns
// them as written, without spaces, or an error naming those it can't
// read; nil for none.
func ParseFontFeatures(list string) ([]string, error) {
	var out, bad []string
	for _, f := range strings.Split(list, ",") {
		f = strings.Join(strings.Fields(f), "")
		if f == "" {
			continue
		}
		if !validFeature(f) {
			bad = append(bad, strconv.Quote(f))
			continue
		}
		out = append(out, f)
	}
	if bad != nil {
		return out, fmt.Errorf("%s: write a tag of four characters, as ss01, -calt or cv05=2", strings.Join(bad, ", "))
	}
	return out, nil
}

// ParseFontFallback reads font-fallback as font-family takes a list:
// families separated by commas, in the order they are tried. It returns
// them as written, without the spaces around them, or nil for none.
func ParseFontFallback(list string) []string {
	var out []string
	for _, f := range strings.Split(list, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// FallbackFamilies returns font-fallback as the family list of the font
// that takes it, after family.
func (s Settings) FallbackFamilies(family string) string {
	if f := s.FontFallback; strings.TrimSpace(f) != "" {
		return family + ", " + f
	}
	return family
}

func validFeature(f string) bool {
	tag, value, set := strings.Cut(f, "=")
	if rest, ok := strings.CutPrefix(tag, "-"); ok {
		if set {
			return false // -calt=1 says two things
		}
		tag = rest
	} else {
		tag = strings.TrimPrefix(tag, "+")
	}
	if set {
		if _, err := strconv.ParseUint(value, 10, 32); err != nil {
			return false
		}
	}
	return len(tag) == 4 && !strings.ContainsFunc(tag, func(r rune) bool { return r <= ' ' || r > '~' })
}

// normalize keeps values in range, as a file edited by hand may not.
func (s *Settings) normalize() {
	d := Default()
	oneOf := func(v *string, def string, ok ...string) {
		for _, o := range ok {
			if *v == o {
				return
			}
		}
		*v = def
	}
	oneOf(&s.NewSessionDir, d.NewSessionDir, "current", "project", "home")
	oneOf(&s.SidebarRows, d.SidebarRows, "detailed", "compact")
	oneOf(&s.Appearance, d.Appearance, "system", "light", "dark")
	oneOf(&s.Language, d.Language, append([]string{"system"}, Languages...)...)
	oneOf(&s.Cursor, d.Cursor, "block", "bar", "underline")
	oneOf(&s.Clipboard, d.Clipboard, "ask", "allow", "deny")
	oneOf(&s.RightClick, d.RightClick, "menu", "paste", "copyOrPaste")
	oneOf(&s.DiffLayout, d.DiffLayout, "auto", "unified", "split")
	oneOf(&s.WorktreeBase, d.WorktreeBase, "default", "current")
	if s.PetOffset < 0 || s.PetOffset > 1 {
		s.PetOffset = d.PetOffset
	}
	if !validPetName(s.PetSet) {
		s.PetSet = d.PetSet
	}
	if s.PetSize < 0.5 || s.PetSize > 3 {
		s.PetSize = d.PetSize
	}
	if s.WindowOpacity < 0 || s.WindowOpacity > 1 {
		s.WindowOpacity = d.WindowOpacity
	}
	if s.PaneOpacity < 0 || s.PaneOpacity > 1 {
		s.PaneOpacity = d.PaneOpacity
	}
	if s.SidebarImageOpacity < 0 || s.SidebarImageOpacity > 1 {
		s.SidebarImageOpacity = d.SidebarImageOpacity
	}
	if s.PaneImageOpacity < 0 || s.PaneImageOpacity > 1 {
		s.PaneImageOpacity = d.PaneImageOpacity
	}
	oneOf(&s.PaneImageFit, d.PaneImageFit, PaneImageFits...)
	oneOf(&s.SessionSwitch, d.SessionSwitch, SessionSwitches...)
	oneOf(&s.AppIcon, d.AppIcon, AppIcons...)
	known := false
	for _, b := range Backgrounds {
		known = known || b.ID == s.Background
	}
	if !known {
		s.Background = d.Background
	}
	if s.FontSize < 8 || s.FontSize > 36 {
		s.FontSize = d.FontSize
	}
	if s.LineHeight < 0.8 || s.LineHeight > 2.5 {
		s.LineHeight = d.LineHeight
	}
	if !(s.CellWidth >= 0.8 && s.CellWidth <= 2) {
		s.CellWidth = d.CellWidth
	}
	if s.FontWeight < 100 || s.FontWeight > 900 {
		s.FontWeight = d.FontWeight
	}
	if s.ScrollPrecision < MinScrollMultiplier || s.ScrollPrecision > MaxScrollMultiplier {
		s.ScrollPrecision = d.ScrollPrecision
	}
	if s.ScrollDiscrete < MinScrollMultiplier || s.ScrollDiscrete > MaxScrollMultiplier {
		s.ScrollDiscrete = d.ScrollDiscrete
	}
	if s.Scrollback < 0 || s.Scrollback > 256 {
		s.Scrollback = d.Scrollback
	}
	normalizeFeishu(s.Feishu)
	if s.TailcatRelay != "" && !ValidDERPMapURL(s.TailcatRelay) {
		s.TailcatRelay = d.TailcatRelay
	}
	if s.LightTheme == "" {
		s.LightTheme = d.LightTheme
	}
	if s.DarkTheme == "" {
		s.DarkTheme = d.DarkTheme
	}
	// 2.x, which called Hyla Kero, named its pet and its theme kero.
	if s.PetSet == "kero" {
		s.PetSet = PetSetHyla
	}
	if s.LightTheme == "kero" {
		s.LightTheme = d.LightTheme
	}
	if s.DarkTheme == "kero" {
		s.DarkTheme = d.DarkTheme
	}
}
