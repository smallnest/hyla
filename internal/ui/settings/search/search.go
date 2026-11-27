// Package search finds the Settings window's settings, by what they say
// and by the words they are looked for by.
package search

import (
	"runtime"
	"slices"
	"strings"

	"hyla/internal/actions"
	"hyla/locales"
)

// Entry is a setting the search finds: its page, its label as its row
// shows it, and words that find it besides, as its key in the
// configuration file.
type Entry struct {
	Page, Label, Words string
}

// settingsIndex is the settings the search finds, labelled in the
// language the app shows, as their rows are.
func Index() []Entry {
	entries := []Entry{
		{"general", locales.SetNewSessionsStartIn(), "new-session-directory folder current project home tab"},
		{"general", locales.SetSessionName(), "session-name title sidebar focused lead folder pane"},
		{"general", locales.SetSidebarRows(), "sidebar-rows compact detailed status message branch two lines"},
		{"general", locales.SetFocusFollowsMouse(), "focus-follows-mouse hover pointer niri hyprland sloppy"},
		{"general", locales.SetAskBeforeClosing(), "confirm-close quit kill"},
		{"general", locales.SetQuitEndsSessions(), "quit-ends-sessions quit exit stop kill end clean cleanly leave window close background process"},
		{"general", locales.SetRestoreSessions(), "restore-sessions reboot persist"},
		{"general", locales.SetResumeAgents(), "resume-agents claude codex pi opencode conversation restore reboot"},
		{"general", locales.SetResumePrograms(), "resume-programs osc 7577 nvim lazygit htop program restore reboot"},
		{"general", locales.SetResumeAllow(), "resume-programs-allow programs allow list nvim lazygit path"},
		{"general", locales.SetGitStatus(), "git-status branch changes diff repository worktree"},
		{"general", locales.SetDiffLayout(), "diff-layout diff unified split side by side changes"},
		{"general", locales.SetOpenAtLogin(), "startup launch login items"},
		{"general", locales.SetVersion(), "updates check for updates software update about"},
		{"general", locales.SetAutoCheck(), "updates automatic background daily"},
		{"general", locales.SetAutoInstall(), "updates automatic install download"},

		{"appearance", locales.SetAppearance(), "window-theme dark light mode system"},
		{"appearance", locales.SetSessionSwitch(), "session-switch animation switch session transition dissolve slide push cover zoom none 切换 动画 溶解 滑动 推移 覆盖 缩放"},
		{"appearance", locales.SetLanguage(), "language locale translation chinese english 语言 中文"},
		{"appearance", locales.SetTranslucent(), "window-translucent vibrancy blur material transparency"},
		{"appearance", locales.SetWindowOpacity(), "window-opacity opacity translucent transparency see-through dim 透明度 不透明度"},
		{"appearance", locales.SetWindowBlur(), "window-blur blur frosted vibrancy material transparency 模糊 毛玻璃"},
		{"appearance", locales.SetWindowMaterial(), "window-material material sidebar popover plastic glass liquid dock blur 窗口材质 材质 塑料"},
		{"appearance", locales.SetSidebarImage(), "sidebar-image background picture photo image left panel 左边栏 侧栏 背景图 图片"},
		{"appearance", locales.SetSidebarImageOpacity(), "sidebar-image-opacity opacity translucent fade picture image 背景图 不透明度 透明度"},
		{"appearance", locales.SetPaneImage(), "pane-image terminal panes background picture photo image wallpaper 终端 背景图 图片 壁纸"},
		{"appearance", locales.SetPaneImageOpacity(), "pane-image-opacity opacity translucent fade picture image terminal 终端 背景图 不透明度 透明度"},
		{"appearance", locales.SetPaneImageFit(), "pane-image-fit fill fit stretch center tile cover contain wallpaper picture terminal 终端 背景图 填充 适合 拉伸 居中 拼贴"},
		{"appearance", locales.SetPet(), "pet frog hyla mascot sprite companion agent status drag offset position 小精灵 青蛙 宠物"},
		{"appearance", locales.SetBackground(), "background wash petal mist meadow indigo plain dock gradient color 靛蓝"},

		{"terminal", locales.SetFont(), "font-family typeface jetbrains mono nerd"},
		{"terminal", locales.SetFontFallback(), "font-fallback fonts chinese cjk han emoji glyphs missing"},
		{"terminal", locales.SetSize(), "font-size zoom bigger smaller text"},
		{"terminal", locales.SetCellWidth(), "cell-width horizontal spacing glyph " + locales.SetCellWidthDetail()},
		{"terminal", locales.SetLineHeight(), "line-height spacing leading cell"},
		{"terminal", locales.SetWeight(), "font-weight bold light"},
		{"terminal", locales.SetLigatures(), "font-ligatures calt arrows"},
		{"terminal", locales.SetFontFeatures(), "font-feature opentype stylistic sets ss01 cv01 alternates"},
		{"terminal", locales.SetThicken(), "font-thicken stroke smoothing"},
		{"terminal", locales.SetLightWindows(), "theme colors color scheme ghostty palette"},
		{"terminal", locales.SetDarkWindows(), "theme colors color scheme ghostty palette"},
		{"terminal", locales.SetDimUnfocused(), "dim-unfocused-terminals panes fade inactive split opacity veil blur"},
		{"terminal", locales.SetPaneGlass(), "pane-opacity pane-blur terminal panes opacity blur glass frosted see-through 终端 透明 模糊"},
		{"terminal", locales.SetPaneOpacity(), "pane-opacity desktop window background translucent terminal panes 0% 终端 不透明度 透明"},
		{"terminal", locales.SetPaneBlur(), "pane-blur plain transparency clear frosted terminal panes 终端 模糊 毛玻璃 普通透明"},
		{"terminal", locales.SetPaneMaterial(), "pane-material material plastic glass clear liquid terminal panes 终端 材质 玻璃 塑料"},
		{"terminal", locales.SetPaneGlass(), "pane-opacity pane-blur translucent terminal panes opacity blur glass frosted see-through 终端 透明 模糊"},
		{"terminal", locales.SetPaneOpacity(), "pane-opacity opacity translucent transparency see-through desktop window background terminal panes 终端 不透明度 透明"},
		{"terminal", locales.SetPaneBlur(), "pane-blur blur frosted plain transparency terminal panes 终端 模糊 毛玻璃 普通透明"},
		{"terminal", locales.SetShape(), "cursor-style cursor block bar underline caret"},
		{"terminal", locales.SetBlink(), "cursor-style-blink cursor caret"},
		{"terminal", locales.SetOptionMeta(), "macos-option-as-alt alt meta emacs readline keyboard"},
		{"terminal", locales.SetCopyOnSelect(), "copy-on-select clipboard selection"},
		{"terminal", locales.SetRightClick(), "right-click-action context menu paste mouse"},
		{"terminal", locales.SetClipboardRead(), "clipboard-read osc 52 paste privacy"},
		{"terminal", locales.SetWheelScroll(), "mouse-scroll-multiplier discrete scroll speed wheel notch rows"},
		{"terminal", locales.SetTrackpadScroll(), "mouse-scroll-multiplier precision scroll speed trackpad touchpad"},

		{"shell", locales.SetShell(), "command fish zsh bash login"},
		{"shell", locales.SetScrollback(), "scrollback-limit history buffer lines memory"},
		{"shell", locales.SetEnvironment(), "env variables path"},

		{"browser", locales.SetBlockAds(), "browser-block-ads browser-filter-list ad blocker adblock trackers filter lists easylist easyprivacy peter lowe fanboy cookie annoyances social china privacy web"},

		{"notifications", locales.SetNotifyWaiting(), "notify-on-waiting attention blocked permission"},
		{"notifications", locales.SetNotifyDone(), "notify-on-done done complete"},
		{"notifications", locales.SetNotifyFailed(), "notify-on-error error"},
		{"notifications", locales.SetPlaySound(), "notify-sound audio"},
		{"notifications", locales.SetDockBadge(), "dock-badge count"},
		{"notifications", locales.SetDockBounce(), "dock-bounce attention"},

		{"feishu", locales.SetFeishuYolo(), "feishu-yolo yolo permission prompts sandbox skip dangerously auto approve claude codex unsafe dangerous"},
		{"feishu", locales.SetFeishu(), "feishu lark bot chat app secret feishu-app feishu-allow feishu-agent feishu-run feishu-dir feishu-cards feishu-markdown feishu-open-button feishu-host feishu-timeout feishu-stream"},
		{"agents", locales.SetRemoteControlRow(), "remote-control hyla do focus browser automation"},
		{"agents", locales.SetCliRow(), "cli command line tool install path"},
		{"agents", "Claude Code", "skill agents install"},
		{"agents", "Codex, OpenCode, Pi", "skill agents install"},
		{"agents", locales.SetProgramStatus(), "osc 7501 report progress"},
		{"plugins", locales.PagePlugins(), "plugins extensions packs permissions hylapkg reload install market marketplace store"},

		{"hosts", locales.SetHosts(), "host hosts remote connection p2p server ssh tailcat beacontower beacon relay wsl machine linux devbox phone qr password token connect disconnect status 连接 断开 状态 烽火台"},
		{"hosts", locales.SetHostAdd(), "add host address ssh wsl tailcat beacontower beacon"},
		{"hosts", locales.SetHostRemoteControl(), "remote-control host hyla do focus"},
		{"hosts", locales.SetBeaconTowerTitle(), "beacontower beacon relay pair code token e2ee wss tls 烽火台 中继"},
		{"hosts", locales.SetBeaconTowerListen(), "beacontower listener provide service relay token port beacontower:// hyla beacontower enable"},
		{"hosts", locales.SetBeaconTowerCode(), "beacontower pair code paircode copy secret share other machines"},
		// The group's title, above its switch.
		{"hosts", locales.SetTailcatTitle(), "tailcat pair code relay derp paircode tailcat-relay tailcat-port"},
		{"hosts", locales.SetTailcatListen(), "tailcat listener provide service host relay port tailcat:// hyla hosts tailcat enable"},
		{"hosts", locales.SetTailcatCode(), "tailcat pair code paircode copy secret share other machines"},

		{"advanced", locales.SetConfigFile(), "configuration ghostty edit file dotfiles"},
		{"advanced", "hylad", "daemon background service restart sessions"},
		{"advanced", locales.SetLog(), "hylad log debug"},
		{"advanced", locales.SetFolder(), "data directory finder"},
		{"advanced", locales.SetReset(), "defaults reset"},
	}
	if runtime.GOOS == "darwin" {
		entries = append(entries, Entry{"general", locales.SetAppIcon(), "macos-show-app-icon dock menu bar menubar tray status item hide"})
	}
	return entries
}

// searchEntries are the index and the actions, which the keyboard page
// lists, with the shortcuts they have.
func Entries(shortcuts map[string]string, title func(string) string) []Entry {
	out := Index()
	for _, act := range actions.List() {
		keys := actions.Keys(act, shortcuts)
		out = append(out, Entry{"keyboard", strings.TrimSuffix(title(act.ID), "…"),
			act.ID + " shortcut keybind " + act.Menu + " " + strings.ToLower(keys) + " " + actions.Glyphs(keys) + " " + ActionWords[act.ID]})
	}
	return out
}

// actionWords find actions besides their titles: what one looks for an
// action by when its title doesn't say it.
// ActionWords find actions besides their titles.
var ActionWords = map[string]string{
	"settings.open":        "preferences options config configure",
	"cli.install":          "hyla command line shell path terminal",
	"app.quitAll":          "exit quit kill stop end all sessions",
	"session.new":          "create add tab terminal shell",
	"session.newHome":      "create add tab terminal shell home directory ~",
	"session.newWorktree":  "create add worktree branch checkout agent isolate parallel",
	"conversation.resume":  "history agent claude codex pi opencode continue reopen restore past chat",
	"project.open":         "folder directory repo repository workspace add",
	"palette.open":         "switch jump search find command palette",
	"session.attention":    "waiting blocked attention notification needs input agent",
	"session.next":         "switch tab forward",
	"session.previous":     "switch tab back backward",
	"session.rename":       "name title label",
	"pane.restart":         "rerun relaunch reload reset shell",
	"pane.close":           "pane remove kill exit",
	"session.close":        "remove kill exit end tab",
	"find.open":            "search look up text grep",
	"find.next":            "search again",
	"find.previous":        "search again back",
	"pane.split.right":     "vertical horizontal side by side new pane",
	"pane.split.down":      "horizontal vertical below new pane",
	"pane.fork":            "fork branch clone split agent session claude codex pi opencode 分叉 分支 会话",
	"pane.browser":         "browser web page url preview localhost open",
	"pane.zoom":            "maximize fullscreen focus expand",
	"pane.next":            "switch focus forward",
	"pane.previous":        "switch focus back backward",
	"pane.left":            "focus move navigate",
	"pane.right":           "focus move navigate",
	"pane.up":              "focus move navigate above",
	"pane.down":            "focus move navigate below",
	"sidebar.toggle":       "show hide sidebar sessions list",
	"side.toggle":          "side panel toggle show hide close",
	"changes.toggle":       "git diff branch panel changes",
	"files.toggle":         "file tree browser folder explore panel",
	"subagents.open":       "subagent subagents delegated parallel agent pane open",
	"hongguo.open":         "hongguo drama short video break player 红果 短剧 摸鱼 刷剧",
	"pane.chat":            "chat conversation transcript messages prompt agent talk",
	"text.bigger":          "zoom in font size larger increase text",
	"text.smaller":         "zoom out font size decrease text",
	"text.reset":           "zoom reset font size default text",
	"quickTerminal.toggle": "quake doom drop down dropdown hotkey global visor guake iterm console",
	"pet.toggle":           "frog hyla mascot sprite show hide pet companion 小青蛙 精灵 宠物",
}

// PageWords find Settings' pages in the palette besides their titles.
var PageWords = map[string]string{
	"general":       "preferences startup",
	"appearance":    "theme color dark light font look",
	"terminal":      "font cursor scrollback ghostty",
	"shell":         "env environment variables path command",
	"browser":       "web ad block adblock trackers privacy",
	"notifications": "alerts sound badge dock notify",
	"keyboard":      "shortcuts keybindings keys hotkeys",
	"feishu":        "lark bot chat message im app secret",
	"plugins":       "plugin extension pack theme wash pet agent hook",
	"sync":          "cloud backup machine device config settings",
	"agents":        "claude codex ai skill cli remote control",
	"hosts":         "remote ssh tailcat server machine wsl",
	"advanced":      "config file daemon hylad log reset",
}

// Matches reports whether every word of a query is in a text.
func Matches(text string, terms []string) bool {
	text = strings.ToLower(text)
	for _, t := range terms {
		if !strings.Contains(text, t) {
			return false
		}
	}
	return len(terms) > 0
}

// Page is a page of the Settings window, as Results knows it.
type Page struct {
	ID    string
	Title string
}

// PluginEntry is a plugin setting, page, or slot indexed by Settings Search.
type PluginEntry struct {
	Page     string
	Label    string
	Words    string
	PluginID string // non-empty when matching reveals a plugin drawer in Settings › Plugins
}

// Results returns what a query finds, by page: the labels of the
// settings it finds, an empty list for a page whose title it is, and
// nothing for the others. It returns nil for an empty query.
func Results(query string, shortcuts map[string]string, pages []Page, title func(string) string) map[string][]string {
	out, _ := ResultsWithPlugins(query, shortcuts, pages, nil, title)
	return out
}

// ResultsWithPlugins extends Results with plugin settings, standalone plugin
// pages, and plugin UI slots. It returns matched labels per page and matched
// plugin IDs per page so callers can reveal a plugin's drawer on match.
func ResultsWithPlugins(query string, shortcuts map[string]string, pages []Page, plugins []PluginEntry, title func(string) string) (map[string][]string, map[string][]string) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, nil
	}
	out := map[string][]string{}
	pids := map[string][]string{}
	for _, p := range pages {
		if Matches(p.Title, terms) {
			out[p.ID] = []string{}
		}
	}
	for _, e := range Entries(shortcuts, title) {
		if Matches(e.Label+" "+e.Words, terms) {
			out[e.Page] = append(out[e.Page], e.Label)
		}
	}
	for _, pe := range plugins {
		if Matches(pe.Label+" "+pe.Words, terms) {
			if pe.Label != "" {
				out[pe.Page] = append(out[pe.Page], pe.Label)
			} else if out[pe.Page] == nil {
				out[pe.Page] = []string{}
			}
			if pe.PluginID != "" && !slices.Contains(pids[pe.Page], pe.PluginID) {
				pids[pe.Page] = append(pids[pe.Page], pe.PluginID)
			}
		}
	}
	return out, pids
}
