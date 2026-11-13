package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"hyla/internal/actions"
	"hyla/internal/adblock"
	"hyla/internal/proto"
	"hyla/internal/remote"
)

// hyla's configuration is a file of lines `key = value`, as Ghostty's:
// `#` starts a comment, and keys that hold lists, as env and keybind,
// repeat. The keys are Ghostty's where they mean what Ghostty's do. A
// file holds what the user changed; the rest is the defaults. The
// Settings window edits it in place, keeping its comments and order.

// ConfigPath returns the configuration file: $HYLA_CONFIG, else
// $XDG_CONFIG_HOME/hyla/config, else ~/.config/hyla/config; hyla-dev in
// place of hyla for a development build (proto.Name).
//
// The file of 2.x, which called Hyla Kero, is this one: the first to ask
// for it moves it here, as proto.Dir moves the directory.
func ConfigPath() string {
	if p := os.Getenv("HYLA_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	name := proto.Name()
	p := filepath.Join(base, name, "config")
	old := filepath.Join(base, strings.Replace(name, "hyla", "kero", 1), "config")
	if _, err := os.Stat(p); err != nil {
		if _, err := os.Stat(old); err == nil {
			if os.MkdirAll(filepath.Dir(p), 0o755) == nil && os.Rename(old, p) == nil {
				return p
			}
			return old
		}
	}
	return p
}

// key is a key of the configuration: how it reads and writes a setting.
type key struct {
	name string
	doc  string
	list bool // repeats, one value a line
	get  func(*Settings) []string
	set  func(*Settings, []string) error
	// check, for a key related to others, tells what is wrong with s once
	// all of the file's keys are in it: listen needs tailcat before it,
	// whatever order the file writes them in.
	check func(*Settings) error
}

// float32Key is numberKey for a setting kept as a float32, which reads
// and writes its shortest form, not the double's.
func float32Key(name, doc string, f func(*Settings) *float32) key {
	return key{name: name, doc: doc,
		get: func(s *Settings) []string { return []string{strconv.FormatFloat(float64(*f(s)), 'g', -1, 32)} },
		set: func(s *Settings, v []string) error {
			n, err := strconv.ParseFloat(v[len(v)-1], 32)
			if err != nil {
				return fmt.Errorf("%q is not a number", v[len(v)-1])
			}
			*f(s) = float32(n)
			return nil
		}}
}

func boolKey(name, doc string, f func(*Settings) *bool) key {
	return key{name: name, doc: doc,
		get: func(s *Settings) []string { return []string{strconv.FormatBool(*f(s))} },
		set: func(s *Settings, v []string) error {
			b, err := strconv.ParseBool(v[len(v)-1])
			if err != nil {
				return fmt.Errorf("%q is not true or false", v[len(v)-1])
			}
			*f(s) = b
			return nil
		}}
}

func choiceKey(name, doc string, f func(*Settings) *string, choices ...string) key {
	return key{name: name, doc: doc + " (" + strings.Join(choices, ", ") + ")",
		get: func(s *Settings) []string { return []string{*f(s)} },
		set: func(s *Settings, v []string) error {
			c := v[len(v)-1]
			if !slices.Contains(choices, c) {
				return fmt.Errorf("%q is none of %s", c, strings.Join(choices, ", "))
			}
			*f(s) = c
			return nil
		}}
}

func stringKey(name, doc string, f func(*Settings) *string) key {
	return key{name: name, doc: doc,
		get: func(s *Settings) []string { return []string{*f(s)} },
		set: func(s *Settings, v []string) error { *f(s) = v[len(v)-1]; return nil }}
}

func numberKey(name, doc string, get func(*Settings) float64, set func(*Settings, float64)) key {
	return key{name: name, doc: doc,
		get: func(s *Settings) []string { return []string{strconv.FormatFloat(get(s), 'f', -1, 64)} },
		set: func(s *Settings, v []string) error {
			n, err := strconv.ParseFloat(v[len(v)-1], 64)
			if err != nil {
				return fmt.Errorf("%q is not a number", v[len(v)-1])
			}
			set(s, n)
			return nil
		}}
}

// mapChoice maps the values of a choice in the file to those of Settings.
func mapChoice(name, doc string, f func(*Settings) *string, file, ours []string) key {
	return key{name: name, doc: doc + " (" + strings.Join(file, ", ") + ")",
		get: func(s *Settings) []string { return []string{file[max(slices.Index(ours, *f(s)), 0)]} },
		set: func(s *Settings, v []string) error {
			i := slices.Index(file, v[len(v)-1])
			if i < 0 {
				return fmt.Errorf("%q is none of %s", v[len(v)-1], strings.Join(file, ", "))
			}
			*f(s) = ours[i]
			return nil
		}}
}

// keys are the configuration's keys, in the order a new file and
// `hyla config show` list them.
var keys = append([]key{
	mapChoice("new-session-directory", "where a new session starts", func(s *Settings) *string { return &s.NewSessionDir },
		[]string{"current", "project", "home"}, []string{"current", "project", "home"}),
	choiceKey("session-name", "what names a session you or an agent didn't: the program in the pane you're in, else in the one that most needs you; the program in the one that most needs you; or the first pane's folder",
		func(s *Settings) *string { return &s.SessionName }, proto.NameFocused, proto.NameLead, proto.NameFolder),
	choiceKey("sidebar-rows", "how the sidebar shows a session: detailed, with its status message, else its branch and changes, on a second line; or compact, on one line",
		func(s *Settings) *string { return &s.SidebarRows }, "detailed", "compact"),
	boolKey("focus-follows-mouse", "focus the pane the pointer moves into", func(s *Settings) *bool { return &s.FocusFollowsMouse }),
	boolKey("confirm-close", "ask before closing a pane a program runs in", func(s *Settings) *bool { return &s.ConfirmClose }),
	boolKey("restore-sessions", "start sessions again after hylad restarts, as after a reboot", func(s *Settings) *bool { return &s.RestoreSessions }),
	boolKey("resume-agents", "as sessions start again, resume the conversation of the coding agent each pane ran", func(s *Settings) *bool { return &s.ResumeAgents }),
	boolKey("quit-ends-sessions", "quitting ends this machine's sessions and its hylad, as Quit and End All Sessions does; other machines' keep running", func(s *Settings) *bool { return &s.QuitEndsSessions }),
	boolKey("session-history", "list the coding agents' ended conversations in the sidebar, to resume or archive", func(s *Settings) *bool { return &s.SessionHistory }),
	choiceKey("macos-show-app-icon", "where Hyla's icon shows; closing the window keeps Hyla running in the menu bar while it shows there. Windows and Linux always show it in the notification area",
		func(s *Settings) *string { return &s.AppIcon }, AppIcons...),

	{name: "window-opacity", doc: "how much of the window's background the wash covers while translucent, 0 to 1",
		get: func(s *Settings) []string {
			return []string{strconv.FormatFloat(float64(s.WindowOpacity), 'g', -1, 32)}
		},
		set: func(s *Settings, v []string) error {
			n, err := strconv.ParseFloat(v[len(v)-1], 32)
			if err != nil {
				return fmt.Errorf("%q is not a number", v[len(v)-1])
			}
			s.WindowOpacity = float32(n)
			return nil
		}},
	boolKey("window-blur", "blur the desktop behind the translucent background; off, the wash is plain transparency", func(s *Settings) *bool { return &s.WindowBlur }),
	choiceKey("window-material", "the window's material behind the wash while blurred: sidebar, window, popover, plastic (a frosted content background), glass (Liquid Glass, as the Dock's) or glass-clear", func(s *Settings) *string { return &s.WindowMaterial }, "sidebar", "window", "popover", "plastic", "glass", "glass-clear"),
	stringKey("sidebar-image", "a picture behind the sidebar, as tall as 80% of it and against its left edge: an image file's path, empty for none", func(s *Settings) *string { return &s.SidebarImage }),
	float32Key("sidebar-image-opacity", "how much of the sidebar's picture shows, 0 to 1", func(s *Settings) *float32 { return &s.SidebarImageOpacity }),
	stringKey("pane-image", "a picture in the terminals' cards, under their text, in place of the window's background: an image file's path, empty for none", func(s *Settings) *string { return &s.PaneImage }),
	float32Key("pane-image-opacity", "how much of the terminals' picture shows, 0 to 1", func(s *Settings) *float32 { return &s.PaneImageOpacity }),
	choiceKey("pane-image-fit", "how the terminals' picture fills a card: fill, as large as the card covers, from its top; fit, all of it, its shape kept; stretch, to the card's edges; center, its own size at the card's middle; or tile", func(s *Settings) *string { return &s.PaneImageFit }, PaneImageFits...),
	choiceKey("session-switch", "how a session shown in place of another comes in: dissolve, fading in as the one it leaves blurs and fades away; slide, coming in from the right; push, sliding in from the right as the one it leaves goes out to the left; cover, coming in from the right over the one it leaves, which stays; zoom, growing into place; or none, showing at once", func(s *Settings) *string { return &s.SessionSwitch }, SessionSwitches...),
	float32Key("pane-opacity", "how much of the window's background shows in a terminal while the window is translucent, 0 to 1, as window-opacity; 0 shows the desktop alone, and the terminal's text stays opaque", func(s *Settings) *float32 { return &s.PaneOpacity }),
	boolKey("pane-blur", "blur what shows through the terminals; off, they are plain transparency, and 0% opacity shows the desktop alone", func(s *Settings) *bool { return &s.PaneBlur }),
	choiceKey("pane-material", "the terminal's glass while Terminal blur is on: glass (Liquid Glass), plastic (a frosted content background) or glass-clear", func(s *Settings) *string { return &s.PaneMaterial }, "glass", "plastic", "glass-clear"),
	boolKey("pet", "Hyla, the tree frog on the window's top edge, who follows your sessions", func(s *Settings) *bool { return &s.Pet }),
	float32Key("pet-offset", "where Hyla stands across the window, 0 to 1; drag it there", func(s *Settings) *float32 { return &s.PetOffset }),
	float32Key("pet-size", "how tall a Codex pet shows, against the Codex app's 82 DIPs, 0.5 to 3", func(s *Settings) *float32 { return &s.PetSize }),
	stringKey("pet-set", `the pet on the window's edge: hyla, codex (the Codex app's active pet), or a Codex pet's directory name in ~/.codex/pets, or in hyla's own`, func(s *Settings) *string { return &s.PetSet }),
	choiceKey("window-theme", "the app's appearance", func(s *Settings) *string { return &s.Appearance }, "system", "light", "dark"),
	choiceKey("language", "the app's language; system follows the system's", func(s *Settings) *string { return &s.Language }, append([]string{"system"}, Languages...)...),
	mapChoice("background", "the window's wash", func(s *Settings) *string { return &s.Background },
		[]string{"petal", "mist", "meadow", "plain", "dock"}, []string{"rex", "mist", "meadow", "plain", "dock"}),
	boolKey("window-translucent", "let the desktop show through the window", func(s *Settings) *bool { return &s.Vibrancy }),

	stringKey("font-family", "the terminals' font: empty for JetBrains Mono Nerd Font, which Hyla carries; monospace for the system's", func(s *Settings) *string { return &s.FontFamily }),
	stringKey("font-fallback", `the fonts of what font-family lacks, in the order they are tried, as "PingFang SC"; empty for the system's choice`, func(s *Settings) *string { return &s.FontFallback }),
	numberKey("font-size", "in points", func(s *Settings) float64 { return float64(s.FontSize) }, func(s *Settings, n float64) { s.FontSize = float32(n) }),
	numberKey("font-weight", "100 to 900", func(s *Settings) float64 { return float64(s.FontWeight) }, func(s *Settings, n float64) { s.FontWeight = int(n) }),
	numberKey("line-height", "a multiple of the font's", func(s *Settings) float64 { return float64(s.LineHeight) }, func(s *Settings, n float64) { s.LineHeight = float32(n) }),
	numberKey("cell-width", "the width of terminal cells, a multiple of the font's, from 0.8 to 2; the text keeps its size", func(s *Settings) float64 { return float64(s.CellWidth) }, func(s *Settings, n float64) { s.CellWidth = float32(n) }),
	boolKey("font-ligatures", "join characters as the font draws them", func(s *Settings) *bool { return &s.Ligatures }),
	{name: "font-feature", doc: "the font's OpenType features, as Ghostty's: ss01 turns one on, -calt turns one off, cv05=2 picks an alternate; comma-separated or repeated, and applied after font-ligatures", list: true,
		get: func(s *Settings) []string { return slices.Clone(s.FontFeatures) },
		set: func(s *Settings, v []string) error {
			s.FontFeatures = nil
			var bad []string
			for _, l := range v {
				if strings.TrimSpace(l) == "" {
					s.FontFeatures = nil // an empty value clears those before it
					continue
				}
				fs, err := ParseFontFeatures(l)
				s.FontFeatures = append(s.FontFeatures, fs...)
				if err != nil {
					bad = append(bad, err.Error())
				}
			}
			if bad != nil {
				return fmt.Errorf("%s", strings.Join(bad, "; "))
			}
			return nil
		}},
	boolKey("font-thicken", "a heavier stroke, as Ghostty's", func(s *Settings) *bool { return &s.Thicken }),
	numberKey("minimum-contrast", "the least contrast text keeps with its background, 1 (none) to 21, as Ghostty's", func(s *Settings) float64 { return float64(s.MinContrast) }, func(s *Settings, n float64) { s.MinContrast = float32(min(max(n, 1), 21)) }),
	boolKey("dim-unfocused-terminals", "fade the terminals you're not in, when a session has several panes", func(s *Settings) *bool { return &s.DimUnfocused }),
	{name: "theme", doc: "terminal colors: hyla, a Ghostty theme, or light:<theme>,dark:<theme>",
		get: func(s *Settings) []string {
			if s.LightTheme == s.DarkTheme {
				return []string{s.LightTheme}
			}
			return []string{"light:" + s.LightTheme + ",dark:" + s.DarkTheme}
		},
		set: func(s *Settings, v []string) error {
			val := v[len(v)-1]
			if !strings.Contains(val, ":") {
				s.LightTheme, s.DarkTheme = val, val
				return nil
			}
			for _, part := range strings.Split(val, ",") {
				k, t, ok := strings.Cut(strings.TrimSpace(part), ":")
				switch {
				case ok && k == "light":
					s.LightTheme = strings.TrimSpace(t)
				case ok && k == "dark":
					s.DarkTheme = strings.TrimSpace(t)
				default:
					return fmt.Errorf("%q: write light:<theme>,dark:<theme>", val)
				}
			}
			return nil
		}},
	choiceKey("cursor-style", "programs may change it", func(s *Settings) *string { return &s.Cursor }, "block", "bar", "underline"),
	boolKey("cursor-style-blink", "", func(s *Settings) *bool { return &s.CursorBlink }),
	boolKey("macos-option-as-alt", "Option sends Alt (Meta) to programs", func(s *Settings) *bool { return &s.OptionAsAlt }),
	boolKey("copy-on-select", "what you select goes to the clipboard", func(s *Settings) *bool { return &s.CopyOnSelect }),
	choiceKey("clipboard-read", "programs reading the clipboard (OSC 52)", func(s *Settings) *string { return &s.Clipboard }, "ask", "allow", "deny"),
	mapChoice("right-click-action", "", func(s *Settings) *string { return &s.RightClick },
		[]string{"context-menu", "paste", "copy-or-paste"}, []string{"menu", "paste", "copyOrPaste"}),

	{name: "mouse-scroll-multiplier", doc: "how far scrolling goes, as Ghostty's: precision:<n> scales a touchpad's, discrete:<n> is the rows a notch of a mouse wheel scrolls; both comma-separated, or one number for both",
		get: func(s *Settings) []string {
			f := func(n float32) string { return strconv.FormatFloat(float64(n), 'f', -1, 32) }
			return []string{"precision:" + f(s.ScrollPrecision) + ",discrete:" + f(s.ScrollDiscrete)}
		},
		set: func(s *Settings, v []string) error {
			val := v[len(v)-1]
			n := func(t string) (float32, error) {
				f, err := strconv.ParseFloat(strings.TrimSpace(t), 32)
				if err != nil {
					return 0, fmt.Errorf("%q is not a number", strings.TrimSpace(t))
				}
				return float32(min(max(f, MinScrollMultiplier), MaxScrollMultiplier)), nil
			}
			if !strings.Contains(val, ":") {
				f, err := n(val)
				if err != nil {
					return err
				}
				s.ScrollPrecision, s.ScrollDiscrete = f, f
				return nil
			}
			for _, part := range strings.Split(val, ",") {
				k, t, _ := strings.Cut(strings.TrimSpace(part), ":")
				var dst *float32
				switch k {
				case "precision":
					dst = &s.ScrollPrecision
				case "discrete":
					dst = &s.ScrollDiscrete
				default:
					return fmt.Errorf("%q: write precision:<n>,discrete:<n>", val)
				}
				f, err := n(t)
				if err != nil {
					return err
				}
				*dst = f
			}
			return nil
		}},

	stringKey("command", "the shell of new panes: empty for your login shell", func(s *Settings) *string { return &s.Shell }),
	{name: "env", doc: "added to every pane's environment, as NAME=value; repeat it", list: true,
		get: func(s *Settings) []string { return slices.Clone(s.Env) },
		set: func(s *Settings, v []string) error {
			s.Env = nil
			for _, kv := range v {
				if !strings.Contains(kv, "=") {
					return fmt.Errorf("%q is not NAME=value", kv)
				}
				s.Env = append(s.Env, kv)
			}
			return nil
		}},
	numberKey("scrollback-limit", "the output each pane keeps, in megabytes; 0 for none", func(s *Settings) float64 { return float64(s.Scrollback) }, func(s *Settings, n float64) { s.Scrollback = int(n) }),

	boolKey("notify-on-waiting", "when a session needs you", func(s *Settings) *bool { return &s.NotifyWaiting }),
	boolKey("notify-on-done", "when an agent finishes", func(s *Settings) *bool { return &s.NotifyDone }),
	boolKey("notify-on-error", "when a program reports it failed", func(s *Settings) *bool { return &s.NotifyFailed }),
	boolKey("notify-sound", "", func(s *Settings) *bool { return &s.NotifySound }),
	boolKey("dock-badge", "the number of sessions that need you", func(s *Settings) *bool { return &s.DockBadge }),
	boolKey("dock-bounce", "once, when a session comes to need you", func(s *Settings) *bool { return &s.BounceDock }),

	boolKey("remote-control", "hyla do, hyla focus and hyla browser drive the app", func(s *Settings) *bool { return &s.RemoteControl }),
	{name: "host", doc: "another machine whose sessions the app shows, as name=address: [user@]host or ssh://… over SSH, wsl://distro, or tailcat://… with the pair code of its listener; add remote-control after it to let its programs drive the app, and color=red, orange, amber, green, teal, blue, purple or pink to mark its sessions; repeat it", list: true,
		get: func(s *Settings) []string {
			var out []string
			for _, h := range s.Hosts {
				v := h.Name + "=" + h.Addr
				if h.RemoteControl {
					v += " remote-control"
				}
				if h.Color != "" {
					v += " color=" + h.Color
				}
				out = append(out, v)
			}
			return out
		},
		set: func(s *Settings, v []string) error {
			s.Hosts = nil
			var bad []string
			for _, line := range v {
				h, err := ParseHost(line)
				if err == nil && s.Host(h.Name) != nil {
					err = fmt.Errorf("%q is named twice", h.Name)
				}
				if err != nil {
					bad = append(bad, err.Error())
					continue
				}
				s.Hosts = append(s.Hosts, h)
			}
			if bad != nil {
				return fmt.Errorf("%s", strings.Join(bad, "; "))
			}
			return nil
		}},
	{name: "listen", doc: "let Hyla on other machines reach this one's sessions: tailcat://:7473 through a relay with a pair code (tailcat = true), which also serves HylaPocket on a phone; repeat it", list: true,
		get: func(s *Settings) []string { return slices.Clone(s.Listen) },
		set: func(s *Settings, v []string) error {
			s.Listen = nil
			var bad []string
			for _, l := range v {
				if _, err := remote.ParseListen(l); err != nil {
					bad = append(bad, err.Error())
					continue
				}
				s.Listen = append(s.Listen, strings.TrimSpace(l))
			}
			if bad != nil {
				return fmt.Errorf("%s", strings.Join(bad, "; "))
			}
			return nil
		},
		check: func(s *Settings) error {
			var bad []string
			for _, l := range s.Listen {
				if spec, err := remote.ParseListen(l); err == nil && spec.Kind == "tailcat" && !s.Tailcat {
					bad = append(bad, fmt.Sprintf("%s: Tailcat is off: turn it on with tailcat = true", l))
				}
			}
			if bad != nil {
				return fmt.Errorf("%s", strings.Join(bad, "; "))
			}
			return nil
		}},
	boolKey("tailcat", "give Hyla other ways to reach its hosts: a relay with a pair code. Off, Hyla lists none of those hosts and connects to none, and hylad starts no tailcat listener", func(s *Settings) *bool { return &s.Tailcat }),
	{name: "tailcat-host", doc: "a host Hyla reaches over tailcat, as name=tailcat:// plus the pair code of its tailcat listener (tailcat = true); repeat it", list: true,
		get: func(s *Settings) []string {
			out := make([]string, len(s.TailcatHosts))
			for i, h := range s.TailcatHosts {
				out[i] = h.Name + "=" + h.Addr
			}
			return out
		},
		set: func(s *Settings, v []string) error {
			s.TailcatHosts = nil
			var bad []string
			for _, line := range v {
				name, addr, ok := strings.Cut(strings.TrimSpace(line), "=")
				name, addr = strings.TrimSpace(name), strings.TrimSpace(addr)
				var err error
				switch {
				case !ok:
					err = fmt.Errorf("%q: write name=tailcat://paircode", line)
				case !hostName(name):
					err = fmt.Errorf("%q: a name is letters, digits, dots, dashes and underscores", name)
				case !validTailcatAddr(addr):
					err = fmt.Errorf("%q: %q is not a tailcat address", line, addr)
				case s.TailcatNameTaken(name, s.TailcatHostByName(name)):
					err = fmt.Errorf("%q is named twice", name)
				}
				if err != nil {
					bad = append(bad, err.Error())
					continue
				}
				// A host named here asks for no switch of its own: the
				// file's tailcat line says, and the app's switch sets it.
				s.TailcatHosts = append(s.TailcatHosts, TailcatHost{Name: name, Addr: addr})
			}
			if bad != nil {
				return fmt.Errorf("%s", strings.Join(bad, "; "))
			}
			return nil
		}},
	{name: "tailcat-port", doc: "the port of the tailcat listeners of the hosts Hyla adds, which their pair codes do not have: the default, 7473; set it to the Listener port of a host that names another",
		get: func(s *Settings) []string { return []string{s.TailcatPort} },
		set: func(s *Settings, v []string) error {
			if p := strings.TrimSpace(v[len(v)-1]); p != "" && !ValidTailcatPort(p) {
				return fmt.Errorf("%q is not a port", p)
			}
			s.TailcatPort = strings.TrimSpace(v[len(v)-1])
			return nil
		}},
	{name: "tailcat-relay", doc: "where tailcat finds its relays, a DERP map as https://tailcat.dev/derpmap.json: Tailscale's public, rate-limited relays unless you run your own, as derper does; the host and the other machines set the same map",
		get: func(s *Settings) []string { return []string{s.TailcatRelay} },
		set: func(s *Settings, v []string) error {
			if u := strings.TrimSpace(v[len(v)-1]); u != "" && !ValidDERPMapURL(u) {
				return fmt.Errorf("%q is not an https URL of a DERP map", u)
			}
			s.TailcatRelay = strings.TrimSpace(v[len(v)-1])
			return nil
		}},
	boolKey("tailcat-rotate", "start hylad's tailcat listener with the key again, whose pair code changed: hyla tailcat rotate sets it", func(s *Settings) *bool { return &s.TailcatRotate }),
	boolKey("remote-updates", "let Hyla on another machine that connects update the Hyla app here to its version, which restarts it", func(s *Settings) *bool { return &s.RemoteUpdates }),

	boolKey("git-status", "show the git branch and changes of each session", func(s *Settings) *bool { return &s.GitStatus }),
	choiceKey("worktree-base", "what a session's own git worktree branches from: the repository's default branch, or the branch of the session you're in", func(s *Settings) *string { return &s.WorktreeBase }, "default", "current"),
	stringKey("worktree-dir", "where sessions' own git worktrees go, each under its repository's name; empty for .hyla/worktrees in the repository", func(s *Settings) *string { return &s.WorktreeDir }),
	choiceKey("diff-layout", "the changes panel's diffs: unified while the panel is narrow and side by side once it's wide, or always one", func(s *Settings) *string { return &s.DiffLayout }, "auto", "unified", "split"),
	boolKey("browser-block-ads", "block ads and trackers in browser panes, with the browser-filter-list lists", func(s *Settings) *bool { return &s.BlockAds }),
	{name: "browser-filter-list", doc: "a filter list browser-block-ads blocks with: " + strings.Join(adListIDs(), ", ") + ", or a list's https URL; repeat it, and leave it empty for none", list: true,
		get: func(s *Settings) []string {
			if len(s.AdLists) == 0 {
				return []string{""} // none, rather than the defaults
			}
			return slices.Clone(s.AdLists)
		},
		set: func(s *Settings, v []string) error {
			s.AdLists = []string{}
			var bad []string
			for _, id := range v {
				switch _, ok := adblock.Find(id); {
				case id == "":
					s.AdLists = s.AdLists[:0] // an empty value clears the lists before it
				case !ok:
					bad = append(bad, fmt.Sprintf("%q is no list", id))
				case !slices.Contains(s.AdLists, id):
					s.AdLists = append(s.AdLists, id)
				}
			}
			if bad != nil {
				return fmt.Errorf("%s", strings.Join(bad, "; "))
			}
			return nil
		}},
	{name: "keybind", doc: "<keys>=<action>, over the defaults, as cmd+shift+d=pane.split.down; <keys>=unbind frees keys; see hyla actions", list: true,
		get: func(s *Settings) []string {
			var out []string
			for _, a := range actions.All {
				k, ok := s.Shortcuts[a.ID]
				if !ok {
					continue
				}
				if k == "" {
					if a.Keys != "" {
						out = append(out, configKeys(a.Keys)+"=unbind")
					}
					continue
				}
				out = append(out, configKeys(k)+"="+a.ID)
			}
			return out
		},
		set: func(s *Settings, v []string) error {
			s.Shortcuts = nil
			sc := map[string]string{}
			var bad []string
			for _, b := range v {
				i := strings.LastIndex(b, "=")
				if i <= 0 {
					bad = append(bad, fmt.Sprintf("%q is not <keys>=<action>", b))
					continue
				}
				// Ghostty marks a global shortcut global:; hyla's actions
				// know which are.
				keys, id := accelerator(strings.TrimPrefix(strings.TrimSpace(b[:i]), "global:")), strings.TrimSpace(b[i+1:])
				if id == "unbind" {
					for _, a := range actions.All {
						if strings.EqualFold(actions.Keys(a, sc), keys) {
							sc[a.ID] = ""
						}
					}
					continue
				}
				if _, ok := actions.Find(id); !ok {
					bad = append(bad, fmt.Sprintf("no action %q", id))
					continue
				}
				sc[id] = keys
			}
			if len(sc) > 0 {
				s.Shortcuts = sc
			}
			if bad != nil {
				return fmt.Errorf("%s", strings.Join(bad, "; "))
			}
			return nil
		}},
}, append(feishuKeys(), feishuCardKeys()...)...)

func adListIDs() []string {
	var ids []string
	for _, s := range adblock.Catalog {
		ids = append(ids, s.ID)
	}
	return ids
}

func findKey(name string) *key {
	for i := range keys {
		if keys[i].name == name {
			return &keys[i]
		}
	}
	return nil
}

// configKeys writes an accelerator as the file does: "CmdOrCtrl+Shift+D"
// as "cmd+shift+d".
func configKeys(accel string) string {
	parts := strings.Split(accel, "+")
	if strings.HasSuffix(accel, "++") {
		parts = append(parts[:len(parts)-2], "plus")
	}
	for i, p := range parts {
		switch strings.ToLower(p) {
		case "cmdorctrl", "commandorcontrol", "command":
			parts[i] = "cmd"
		case "control":
			parts[i] = "ctrl"
		case "option":
			parts[i] = "alt"
		default:
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, "+")
}

// accelerator reads keys as the file writes them: "cmd+shift+d" as
// "CmdOrCtrl+Shift+D".
func accelerator(keys string) string {
	keys = strings.TrimSpace(keys)
	parts := strings.Split(keys, "+")
	if strings.HasSuffix(keys, "++") {
		parts = append(parts[:len(parts)-2], "plus")
	}
	names := map[string]string{
		"cmd": "CmdOrCtrl", "super": "CmdOrCtrl", "command": "CmdOrCtrl", "ctrl": "Ctrl", "control": "Ctrl",
		"alt": "Alt", "opt": "Alt", "option": "Alt", "shift": "Shift",
		"enter": "Enter", "return": "Enter", "tab": "Tab", "space": "Space", "backspace": "Backspace", "delete": "Delete",
		"escape": "Escape", "esc": "Escape", "up": "Up", "down": "Down", "left": "Left", "right": "Right",
		"home": "Home", "end": "End", "pageup": "PageUp", "pagedown": "PageDown", "plus": "Plus",
	}
	for i, p := range parts {
		l := strings.ToLower(strings.TrimSpace(p))
		if n, ok := names[l]; ok {
			parts[i] = n
		} else {
			parts[i] = strings.ToUpper(l)
		}
	}
	return strings.Join(parts, "+")
}

// line is a line of a configuration file.
type line struct {
	text       string
	key, value string // for a line setting a key
}

func parseLines(data string) []line {
	var out []line
	for _, raw := range strings.Split(strings.TrimRight(data, "\n"), "\n") {
		l := line{text: raw}
		t := strings.TrimSpace(raw)
		if t != "" && !strings.HasPrefix(t, "#") {
			if k, v, ok := strings.Cut(t, "="); ok {
				l.key = strings.TrimSpace(k)
				l.value = strings.TrimSpace(v)
				if len(l.value) >= 2 && l.value[0] == '"' && l.value[len(l.value)-1] == '"' {
					l.value = l.value[1 : len(l.value)-1]
				}
			} else {
				l.key = t // a key without a value: a problem
			}
		}
		if raw != "" || len(out) > 0 {
			out = append(out, l)
		}
	}
	if len(data) == 0 {
		return nil
	}
	return out
}

// parse reads a configuration over the defaults, with what it got wrong.
func parse(data string) (Settings, []string) {
	s := Default()
	var problems []string
	values := map[string][]string{}
	var order []string
	for n, l := range parseLines(data) {
		if l.key == "" {
			continue
		}
		if findKey(l.key) == nil {
			// feishu-card-body is gone: a card is patched as the card it
			// is, and never as a body alone. A file that still names the
			// switch keeps working, with no problem to report.
			if l.key == "feishu-card-body" {
				continue
			}
			problems = append(problems, fmt.Sprintf("line %d: no setting %q", n+1, l.key))
			continue
		}
		if _, seen := values[l.key]; !seen {
			order = append(order, l.key)
		}
		values[l.key] = append(values[l.key], l.value)
	}
	for _, name := range order {
		k := findKey(name)
		v := values[name]
		if name == "background" {
			// Dusk is gone: its wash was too close to Petal's. A file that
			// still names it gets Petal, without a problem to report.
			for i, val := range v {
				if val == "dusk" {
					v[i] = "petal"
				}
			}
		}
		if !k.list && v[len(v)-1] == "" && name != "font-family" && name != "font-fallback" && name != "command" {
			continue // a key left empty keeps its default
		}
		if err := k.set(&s, v); err != nil {
			problems = append(problems, name+": "+err.Error())
		}
	}
	// A file from before the switch that named a tailcat host asked for
	// Tailcat; one that names the switch has said which way it wants it,
	// whichever its lines' order.
	if _, named := values["tailcat"]; !named {
		for _, h := range s.TailcatHosts {
			s.Tailcat = s.Tailcat || !h.Added
		}
	}
	// After every key, as a file may write listen before tailcat.
	for _, name := range order {
		if k := findKey(name); k.list && k.check != nil {
			if err := k.check(&s); err != nil {
				problems = append(problems, name+": "+err.Error())
			}
		}
	}
	s.normalize()
	return s, problems
}

func quote(v string) string {
	if v == "" || strings.ContainsAny(v, "#") || strings.TrimSpace(v) != v {
		return strconv.Quote(v)
	}
	return v
}

// format writes s into a configuration, over the file's lines: a key the
// file sets takes its new values where it is; one the file leaves out is
// written at the end when it differs from its default. Comments, blank
// lines and keys hyla does not know stay as they are.
func format(s Settings, data string) string {
	def := Default()
	lines := parseLines(data)
	written := map[string]bool{}
	var out []string
	emit := func(k *key) {
		for _, v := range k.get(&s) {
			out = append(out, k.name+" = "+quote(v))
		}
		written[k.name] = true
	}
	for _, l := range lines {
		k := findKey(l.key)
		switch {
		case k == nil:
			out = append(out, l.text)
		case written[k.name]:
			// A list's other lines: its values went at its first.
		default:
			emit(k)
		}
	}
	if len(lines) == 0 {
		out = append(out, header...)
	}
	for i := range keys {
		k := &keys[i]
		if written[k.name] {
			continue
		}
		if slices.Equal(k.get(&s), k.get(&def)) {
			// A file that names tailcat hosts and says nothing of the
			// switch asked for Tailcat, as one from before the switch did:
			// off is written, that the file keeps it off.
			if k.name != "tailcat" || len(s.TailcatHosts) == 0 {
				continue
			}
		}
		emit(k)
	}
	return strings.Join(out, "\n") + "\n"
}

var header = []string{
	"# Hyla's configuration, as Ghostty's: one setting a line, `key = value`.",
	"# It holds what differs from the defaults. Hyla's Settings window edits",
	"# it in place, keeping your comments; what you save here applies at once.",
	"# `hyla config show` lists every key, with its default.",
	"",
}

// Reference writes every key with its value in s, and what it does, as
// `hyla config show` prints it.
func Reference(s Settings) string {
	var b strings.Builder
	for _, k := range keys {
		if k.doc != "" {
			b.WriteString("# " + k.doc + "\n")
		}
		vals := k.get(&s)
		if len(vals) == 0 {
			b.WriteString("# " + k.name + " =\n")
		}
		for _, v := range vals {
			b.WriteString(k.name + " = " + quote(v) + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
