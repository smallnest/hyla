package settings

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/adblock"
	"hyla/internal/buildinfo"
	"hyla/internal/cli"
	"hyla/internal/client"
	"hyla/internal/platform"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/shell"
	"hyla/internal/ui/confirm"
	"hyla/internal/ui/paints"
	"hyla/internal/ui/settings/draft"
	"hyla/internal/ui/settings/kit"
	"hyla/internal/ui/settings/search"
	"hyla/internal/ui/theme"
	"hyla/internal/ui/wash"

	"hyla/internal/ui/controls"
	"hyla/internal/ui/glyph"
	"hyla/internal/ui/style"
	"hyla/locales"
)

// settingsPage is a page of the Settings window.
type settingsPage struct {
	id           string
	title        func() string
	icon         *ui.SVG
	experimental bool // tagged so beside its title
}

var settingsPages = []settingsPage{
	{"general", locales.PageGeneral, glyph.Of("f7-gear-alt"), false},
	{"appearance", locales.PageAppearance, glyph.Of("f7-paintbrush"), false},
	{"terminal", locales.PageTerminal, glyph.Terminal, false},
	{"shell", locales.PageShell, glyph.Of("f7-chevron-left-slash-chevron-right"), false},
	{"browser", locales.PageBrowser, glyph.Globe, false},
	{"notifications", locales.PageNotifications, glyph.Of("f7-bell"), false},
	{"keyboard", locales.PageKeyboard, glyph.Of("f7-keyboard"), false},
	{"feishu", locales.PageFeishu, glyph.Of("f7-chat-bubble-2"), false},
	{"agents", locales.PageAgents, glyph.Of("f7-sparkles"), false},
	{"plugins", locales.PagePlugins, glyph.Plugins, false},
	{"sync", locales.PageSync, glyph.Sync, false},
	{"hosts", locales.PageHosts, glyph.Host, true},
	{"advanced", locales.PageAdvanced, glyph.Of("f7-slider-horizontal-3"), false},
}

// PluginFieldDraft holds a single editable field's current buffer and baseline
// value so Settings › Plugins keeps one stable address per input control.
type PluginFieldDraft struct {
	Value string
	init  string
}

// State is what the Settings window keeps besides the settings.
type State struct {
	Page       string
	Recording  string // the action whose shortcut is being typed
	RecordRef  ui.Handle
	Filter     string // the keyboard page's search
	Query      string // the sidebar's search, of every page
	SearchRef  ui.Handle
	Revealed   string // the query and page the search last scrolled to
	Draft      map[string]string
	EnvDraft   string
	ShellDraft string
	// FeaturesDraft is font-feature as the field has it, while
	// FeaturesEditing.
	FeaturesDraft   string
	FeaturesEditing bool
	ShellEditing    bool
	// ResumeAllowDraft is resume-programs-allow as the field has it,
	// while ResumeAllowEditing.
	ResumeAllowDraft   string
	ResumeAllowEditing bool
	// ProxyDraft is http-proxy as the field has it, while ProxyEditing.
	ProxyDraft   string
	ProxyEditing bool
	LoginOpen    bool // open at login, as the system says
	PetKnob      bool // the pet's size slider was held, last frame
	Updates      updatePrefs
	CLIPath      string
	Service      string // hylad, as last asked
	Notice       string // a line under the keyboard page, as of a conflict
	Toast        string
	Menus        map[string]*controls.SearchMenu // the search menus', by key
	Problems     []string                        // what the configuration file got wrong
	Scrolls      map[string]*ui.ScrollState      // each page's, for its scroll edge
	Confirm      *confirm.Ask                    // a question, asked in this window

	// The forms the root keeps keep their drafts here.
	BeaconTower draft.BeaconTowerDraft // the BeaconTower form
	BTHost      draft.BTHostDraft      // the machines reached by a BeaconTower pair code
	Tailcat     draft.TailcatDraft     // the Tailcat form
	P2P         draft.P2PDraft         // the P2P form
	TCHost      draft.TCHostDraft      // the machines reached by a pair code
	Remote      draft.HostDraft        // the other machines' form
	Feishu      draft.FeishuDraft      // the Feishu page's form

	PluginDetail   string
	PluginLogsID   string
	PluginLogsText string
	PluginDrafts   map[string]*PluginFieldDraft
	NavRevealed    string
}

// checkUpdates asks the updater for a newer app, which none does where
// the app has no updater, as in a test.
func checkUpdates() {
	defer func() { recover() }() // no updater: the panic of mygo's CheckForUpdates
	updater.CheckForUpdates()
}

// SetPage sets the Settings window's active page, clearing any active search
// filter when navigating explicitly to a non-empty page.
func SetPage(ss *State, page string) {
	if page != "" {
		ss.Page = page
		ss.Query = ""
		ss.Revealed = ""
	}
	if ss.Page == "" {
		ss.Page = "general"
	}
}

// Open shows the Settings window, on a page, making it when there is
// none, and calling w with the content to show when it makes one.
func Open(x App, page string, w func(content any)) {
	ss := x.SettingsState()
	SetPage(ss, page)
	if x.SettingsWindow() != nil {
		x.SettingsWindow().Show()
		x.SettingsWindow().Focus()
		x.SettingsWindow().Invalidate()
		return
	}
	if x.RootWindow() == nil {
		w(View(x))
		return
	}
	ss.LoginOpen = mygo.App.OpenAtLogin()
	refreshUpdates(x)
	invalidate := func() {
		if x.SettingsWindow() != nil {
			x.SettingsWindow().Invalidate()
		}
	}
	offUpdates := updater.OnChange(func() {
		refreshUpdates(x)
		invalidate()
	})
	ss.Draft = map[string]string{}
	refreshAgents(x)
	refreshService(x)
	w(View(x))
	// The window closes as it is made, in a test: no further window.
	if win := x.SettingsWindow(); win != nil {
		win.OnClosed(func() {
			offUpdates()
			x.SetSettingsWindow(nil)
			x.StartRecording("") // the menus take their shortcuts back
			ss.Query = ""
		})
	}
}

// updatePrefs are the updater's, as last read: the page reads these, not
// the updater, which needs the running app.
type updatePrefs struct {
	Enabled     bool // this copy of Hyla can update itself
	Automatic   bool // checks in the background
	Download    bool // and installs what it finds without asking
	LastChecked time.Time
}

func refreshUpdates(x App) {

	var (
		ss = x.SettingsState()
		s  = x.Settings()
	)
	var _ = ss
	var _ = s
	x.SettingsState().Updates = updatePrefs{
		Enabled:     mygo.Updater.Enabled(),
		Automatic:   updater.AutomaticChecks(),
		Download:    updater.AutomaticDownloads(),
		LastChecked: updater.LastCheck(),
	}
}

func refreshAgents(x App) {

	var (
		ss = x.SettingsState()
		s  = x.Settings()
	)
	var _ = ss
	var _ = s
	x.SettingsState().CLIPath = x.CLIPath()
}

// refreshService asks hylad how it is, off the main thread.
func refreshService(x App) {
	s := x.Settings()
	var _ = s
	go func() {
		line := locales.SvcNotRunning()
		if c, err := client.Connect(true); err == nil {
			var st proto.State
			c.Call("state", nil, &st)
			panes := 0
			for _, s := range st.Sessions {
				panes += len(s.Panes)
			}
			up := time.Since(time.UnixMilli(c.Welcome.Started)).Round(time.Minute)
			line = locales.SvcRunning(strconv.Itoa(c.Welcome.PID), durationWords(up), len(st.Sessions), panes)
			if buildinfo.Stale(c.Welcome.App, c.Welcome.Build) {
				line += locales.SvcStale()
			}
			c.Close()
		}
		x.Main(func() {
			x.SettingsState().Service = line
			if x.SettingsWindow() != nil {
				x.SettingsWindow().Invalidate()
			}
		})
	}()
}

func durationWords(d time.Duration) string {
	switch {
	case d < time.Minute:
		return locales.DurUnderMinute()
	case d < time.Hour:
		return locales.DurMinutes(int(d.Minutes()))
	case d < 48*time.Hour:
		return locales.DurHours(int(math.Round(d.Hours())))
	}
	return locales.DurDays(int(d.Hours() / 24))
}

// View is the window's content: the sidebar of pages, the search and the
// page shown.
func View(x App) func(c *ui.Context) { return func(c *ui.Context) { view(x, c) } }

func allSettingsPages(x App) (builtin, plugin, all []settingsPage) {
	builtin = settingsPages
	if x != nil {
		if pa := x.PluginsView(); pa != nil {
			for _, pp := range pa.SettingsPages() {
				ic := pp.Icon
				if ic == nil {
					ic = glyph.Plugins
				}
				title := pp.Title
				plugin = append(plugin, settingsPage{
					id:    pp.ID,
					title: func() string { return title },
					icon:  ic,
				})
			}
		}
	}
	all = make([]settingsPage, 0, len(builtin)+len(plugin))
	all = append(all, builtin...)
	all = append(all, plugin...)
	return builtin, plugin, all
}

func view(x App, c *ui.Context) {
	ss, s := x.SettingsState(), x.Settings()
	pal := theme.For(c)
	c.SetTheme(x.Borderless(c, pal))
	if ss.Toast != "" {
		c.Toast(ss.Toast)
		ss.Toast = ""
	}
	builtinPages, pluginPages, allPages := allSettingsPages(x)
	if !slices.ContainsFunc(allPages, func(p settingsPage) bool { return p.id == ss.Page }) {
		if strings.HasPrefix(ss.Page, "plugin:") {
			ss.Page = "plugins"
		} else {
			ss.Page = "general"
		}
		x.StartRecording("")
	}
	found, pluginHits := searchSettings(x, allPages)
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { ss.SearchRef.Focus(c) })
	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	theme.Backdrop(c, pal, row, *s)
	row.Children(func() {
		// Under the traffic lights on macOS; elsewhere the search starts
		// the sidebar, centered on barMid as the page's title and the
		// window controls are.
		headerH := x.HeaderHeight()
		top := x.TopInset()
		if platform.OwnControls {
			top = x.BarMid() - (style.ControlH+2)/2
		}
		ui.Column(c).Width(212).Shrink(0).Padding(top, 10, 12, 10).Gap(2).DragWindow().Children(func() {
			settingsSearch(x, c, pal)
			prevPage := ss.Page
			ui.Scroll(c.Key("nav")).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					shownBuiltin := false
					for _, p := range builtinPages {
						if found == nil || found[p.id] != nil {
							settingsNav(x, c, pal, p)
							shownBuiltin = true
						}
					}
					shownPlugin := false
					for _, p := range pluginPages {
						if found == nil || found[p.id] != nil {
							if !shownPlugin && shownBuiltin {
								ui.Box(c.Key("nav-plg-sep")).Height(1).Margin(6, 10).Shrink(0).Background(pal.Divider)
							}
							settingsNav(x, c, pal, p)
							shownPlugin = true
						}
					}
					if found != nil && len(found) == 0 {
						ui.Text(c, locales.CommonNoResults()).FontSize(13).TextColor(pal.TextFaint).Padding(8, 10)
					}
				})
			})
			if ss.Page != prevPage && found != nil {
				applySearchSelection(ss, found, pluginHits)
			}
			link := ui.ButtonBase(c.Key("open-config")).Shrink(0).Padding(6, 10).Gap(6).Radius(7).AlignItems(ui.Center).Label(locales.SettingsOpenConfigLabel())
			fg := pal.TextMuted
			if link.Hovered() {
				fg = pal.Text
				link.Background(pal.RowHover)
			}
			link.TextColor(fg).Tooltip(proto.ShortDir(settings.Path())).Children(func() {
				ui.Icon(c, glyph.Of("f7-chevron-left-slash-chevron-right")).Size(13, 13)
				ui.Text(c, locales.SettingsOpenConfig()).FontSize(12.5).SingleLine()
			})
			if link.Clicked() {
				x.OpenConfig()
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Padding(x.CardGap(), x.CardGap(), x.CardGap(), 0).Children(func() {
			card := style.Raised(ui.Column(c).Grow(1).MinHeight(0).Radius(14).Background(pal.Card), pal).Clip()
			card.Children(func() {
				page := allPages[0]
				for _, p := range allPages {
					if p.id == ss.Page {
						page = p
					}
				}
				// The page scrolls under its header, which macOS 27's hard
				// scroll edge frosts it under, as System Settings does.
				// The edge shows, as AppKit's, once content is under the bar,
				// or while the pointer is over it.
				if ss.Scrolls == nil {
					ss.Scrolls = map[string]*ui.ScrollState{}
				}
				sc := ss.Scrolls[page.id]
				if sc == nil {
					sc = &ui.ScrollState{}
					ss.Scrolls[page.id] = sc
				}
				box := ui.Box(c.Key("page")).Grow(1).MinHeight(0)
				_, py, over := box.PointerPosition()
				edgeOn := sc.Y > 0.5 || over && py < headerH
				box.Children(func() {
					ui.Scroll(c.Key("page-"+page.id)).Fill().Padding(headerH+16, 24, 28, 24).ScrollbarInsets(headerH, 0, 0).TrackScroll(sc).Children(func() {
						ui.Column(c).Gap(22).MaxWidth(620).Children(func() {
							if strings.HasPrefix(page.id, "plugin:") {
								pluginSettingsPage(x, c, pal, page.id)
							} else {
								switch page.id {
								case "general":
									generalPage(x, c, pal)
								case "appearance":
									appearancePage(x, c, pal)
								case "terminal":
									terminalPage(x, c, pal)
								case "shell":
									shellPage(x, c, pal)
								case "browser":
									browserPage(x, c, pal)
								case "notifications":
									notificationsPage(x, c, pal)
								case "keyboard":
									keyboardPage(x, c, pal)
								case "feishu":
									feishuPage(x, c, pal)
								case "agents":
									agentsPage(x, c, pal)
								case "plugins":
									pluginsPage(x, c, pal)
								case "sync":
									page := SyncPage
									if page == nil {
										page = func(x App, c *ui.Context, pal *theme.Palette) { x.Sync(c, pal) }
									}
									page(x, c, pal)
								case "hosts":
									bt := BeaconTowerPage
									if bt == nil {
										bt = func(x App, c *ui.Context, pal *theme.Palette) { x.BeaconTower(c, pal) }
									}
									tailcat := TailcatPage
									if tailcat == nil {
										tailcat = func(x App, c *ui.Context, pal *theme.Palette) { x.Tailcat(c, pal) }
									}
									p2p := P2PPage
									if p2p == nil {
										p2p = func(x App, c *ui.Context, pal *theme.Palette) { x.P2P(c, pal) }
									}
									bt(x, c, pal)
									tailcat(x, c, pal)
									p2p(x, c, pal)
								case "advanced":
									advancedPage(x, c, pal)
								}
								if page.id != "plugins" {
									if pa := x.PluginsView(); pa != nil {
										pa.RenderSettingsMount(c, pal, "settings."+page.id, "")
									}
								}
							}
						})
					})
					edge := ui.Box(c.Key("edge")).Absolute().Top(0).Left(0).Right(0).Height(headerH).PassThrough().
						Material(glass.ScrollEdge{Hard: true, Background: pal.Card})
					edge.Opacity(edge.Animate("on", b2f(edgeOn), 160*time.Millisecond))
					ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(headerH).AlignItems(ui.Center).Padding(0, 10, 0, 24).DragWindow().Children(func() {
						ui.Text(c, page.title()).FontSize(15).FontWeight(650).TextColor(pal.Text)
						if page.experimental {
							ui.Row(c.Key("experimental")).Height(18).Margin(0, 0, 0, 8).Padding(0, 6).Radius(5).AlignItems(ui.Center).Shrink(0).
								Background(pal.Attention.Alpha(0.14)).Children(func() {
								ui.Text(c, locales.PageExperimental()).FontSize(11.5).FontWeight(600).TextColor(pal.Attention).SingleLine()
							})
						}
						ui.Spacer(c)
						x.WindowControls(c, pal)
					})
				})
			})
		})
	})
	if ss.Confirm != nil {
		confirm.Dialog(c, pal, &ss.Confirm)
	}
	// The controls edit *s: what changed applies, and saves.
	if !reflect.DeepEqual(*s, x.SettingsApplied()) {
		x.ApplySettings(*s)
	}
}

// searchSettings applies the sidebar's search: it shows a page with
// finds, when the page shown has none, and sets what the pages highlight.
// It returns what search.ResultsWithPlugins does.
func searchSettings(x App, allPages []settingsPage) (map[string][]string, map[string][]string) {
	ss, s := x.SettingsState(), x.Settings()
	if strings.TrimSpace(ss.Query) == "" {
		kit.Hits, kit.Reveal = nil, ""
		ss.Revealed = ""
		return nil, nil
	}
	var extras []search.PluginEntry
	if pa := x.PluginsView(); pa != nil {
		extras = pa.SearchEntries()
	}
	spages := make([]search.Page, 0, len(allPages))
	for _, p := range allPages {
		spages = append(spages, search.Page{ID: p.id, Title: p.title()})
	}
	found, pluginHits := search.ResultsWithPlugins(ss.Query, s.Shortcuts, spages, extras, x.ActionTitle)
	kit.Hits, kit.Reveal = nil, ""
	if found == nil {
		ss.Revealed = ""
		return nil, nil
	}
	if found[ss.Page] == nil {
		for _, p := range allPages {
			if found[p.id] != nil {
				ss.Page = p.id
				x.StartRecording("")
				break
			}
		}
	}
	applySearchSelection(ss, found, pluginHits)
	return found, pluginHits
}

func applySearchSelection(ss *State, found map[string][]string, pluginHits map[string][]string) {
	labels := found[ss.Page]
	kit.Hits = map[string]bool{}
	for _, l := range labels {
		kit.Hits[l] = true
	}
	// The first find comes into view once for each query and page, after
	// which the page scrolls as the user likes.
	if at := ss.Query + "\x00" + ss.Page; ss.Revealed != at {
		if len(labels) > 0 {
			ss.Revealed, kit.Reveal = at, labels[0]
		} else {
			ss.Revealed = at
		}
		if ss.Page == "plugins" {
			if pids := pluginHits["plugins"]; len(pids) > 0 && !slices.Contains(pids, ss.PluginDetail) {
				ss.PluginDetail = pids[0]
			}
		}
	}
}

// settingsSearch is the sidebar's search field, which finds settings in
// every page by their labels, their keys in the configuration file, and
// words for them.
func settingsSearch(x App, c *ui.Context, pal *theme.Palette) {
	ss := x.SettingsState()
	var in ui.Element
	frame := ui.Row(c.Key("search")).Cursor(ui.CursorText).Margin(0, 0, 10, 0)
	frame.Children(func() {
		ui.Icon(c, glyph.Search).Size(13, 13).TextColor(pal.TextFaint).Shrink(0)
		in = ui.TextInputBase(c.Key("in"), &ss.Query).Bind(&ss.SearchRef).Grow(1).MinWidth(0).AlignSelf(ui.Stretch).FontSize(13).
			Placeholder(locales.CommonSearch()).Label(locales.SettingsSearchLabel())
		in.OnShortcut(0, ui.KeyEscape, func() { ss.Query = "" })
		if ss.Query != "" {
			x := ui.ButtonBase(c.Key("clear")).Size(16, 16).Radius(8).Center().Shrink(0).Label(locales.SettingsClearSearch()).
				Background(pal.TextFaint.Alpha(0.55)).TextColor(pal.Card)
			if x.Hovered() {
				x.Background(pal.TextMuted)
			}
			x.Children(func() { ui.Icon(c, glyph.Close).Size(8, 8) })
			if x.Clicked() {
				ss.Query = ""
				ss.SearchRef.Focus(c)
			}
		}
	})
	controls.Field(c, frame, in.Focused(), pal)
}

func settingsNav(x App, c *ui.Context, pal *theme.Palette, p settingsPage) {
	ss := x.SettingsState()
	sel := ss.Page == p.id
	item := ui.Row(c.Key(p.id)).Height(32).Padding(0, 10).Gap(10).AlignItems(ui.Center).Radius(8)
	if pal.Dark {
		item.Border(1, ui.Transparent) // room for the chosen item's edge
	}
	bg := ui.Transparent
	switch {
	case sel:
		bg = pal.RowSelected
		style.Chosen(item, pal)
	case item.Hovered():
		bg = pal.RowHover
	}
	item.Background(item.AnimateColor("bg", bg, ui.Animation{Duration: 110 * time.Millisecond}))
	// A quick tap can press and release between frames; keep its click too.
	if (item.Pressed() || item.Clicked()) && !sel {
		ss.Page = p.id
		x.StartRecording("")
	}
	if sel && ss.NavRevealed != ss.Page {
		ss.NavRevealed = ss.Page
		item.ScrollIntoView()
	}
	item.Children(func() {
		ic := ui.Icon(c, p.icon).Size(15, 15).TextColor(pal.TextMuted)
		if sel {
			ic.TextColor(pal.Accent)
		}
		ui.Text(c, p.title()).FontSize(13.5).TextColor(pal.Text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
	})
}

func generalPage(x App, c *ui.Context, pal *theme.Palette) {
	ss, s := x.SettingsState(), x.Settings()
	kit.Group(c, pal, locales.SetSessions(), locales.SetSessionsDetail(x.Tip(strings.TrimSuffix(x.ActionTitle("app.quitAll"), "…"), "app.quitAll")), func() {
		kit.Row(c, pal, locales.SetNewSessionsStartIn(), "", func() {
			kit.Segmented(c, "newdir", &s.NewSessionDir, []string{"current", "project", "home"}, []string{locales.SetDirCurrent(), locales.SetDirProject(), locales.SetDirHome()})
		})
		kit.Row(c, pal, locales.SetSessionName(), locales.SetSessionNameDetail(), func() {
			kit.Segmented(c, "sessionname", &s.SessionName, []string{proto.NameFocused, proto.NameLead, proto.NameFolder},
				[]string{locales.SetNameFocused(), locales.SetNameLead(), locales.SetNameFolder()})
		})
		kit.Row(c, pal, locales.SetSidebarRows(), locales.SetSidebarRowsDetail(), func() {
			kit.Segmented(c, "sidebar-rows", &s.SidebarRows, []string{"detailed", "compact"}, []string{locales.SetRowsDetailed(), locales.SetRowsCompact()})
		})
		kit.Row(c, pal, locales.SetFocusFollowsMouse(), locales.SetFocusFollowsMouseDetail(), func() {
			style.Toggle(c.Key("focusmouse"), &s.FocusFollowsMouse, pal)
		})
		kit.Row(c, pal, locales.SetAskBeforeClosing(), locales.SetAskBeforeClosingDetail(), func() {
			style.Toggle(c.Key("confirm"), &s.ConfirmClose, pal)
		})
		kit.Row(c, pal, locales.SetQuitEndsSessions(), locales.SetQuitEndsSessionsDetail(), func() {
			style.Toggle(c.Key("quit-ends"), &s.QuitEndsSessions, pal)
		})
		kit.Row(c, pal, locales.SetRestoreSessions(), locales.SetRestoreSessionsDetail(), func() {
			style.Toggle(c.Key("restore"), &s.RestoreSessions, pal)
		})
		if s.RestoreSessions {
			kit.Row(c, pal, locales.SetResumeAgents(), locales.SetResumeAgentsDetail(), func() {
				style.Toggle(c.Key("resume"), &s.ResumeAgents, pal)
			})
			kit.Row(c, pal, locales.SetResumePrograms(), locales.SetResumeProgramsDetail(), func() {
				style.Toggle(c.Key("resume-programs"), &s.ResumePrograms, pal)
			})
			if s.ResumePrograms {
				kit.Row(c, pal, locales.SetResumeAllow(), locales.SetResumeAllowDetail(), func() {
					if !ss.ResumeAllowEditing {
						ss.ResumeAllowDraft = strings.Join(s.ResumeCommands, ", ")
					}
					in := controls.TextField(c, "resume-allow", &ss.ResumeAllowDraft, "nvim, lazygit, htop", 220, pal)
					ss.ResumeAllowEditing = in.Focused()
					set := func() {
						var list []string
						for _, a := range strings.Split(ss.ResumeAllowDraft, ",") {
							if a = strings.TrimSpace(a); a != "" && !slices.Contains(list, a) {
								list = append(list, a)
							}
						}
						if !slices.Equal(list, s.ResumeCommands) {
							s.ResumeCommands = list
						}
					}
					in.OnSubmit(set)
					if !ss.ResumeAllowEditing {
						set()
					}
				})
			}
		}
	})
	kit.Group(c, pal, locales.SetGit(), locales.SetGitDetail(), func() {
		kit.Row(c, pal, locales.SetGitStatus(), locales.SetGitStatusDetail(), func() {
			style.Toggle(c.Key("git-status"), &s.GitStatus, pal)
		})
		kit.Row(c, pal, locales.SetDiffLayout(), locales.SetDiffLayoutDetail(), func() {
			kit.Segmented(c, "diff-layout", &s.DiffLayout, []string{"auto", "unified", "split"},
				[]string{locales.SetDiffAuto(), locales.ChangesUnified(), locales.ChangesSplit()})
		})
	})
	// Windows and Linux always show the icon, in the notification area.
	if runtime.GOOS == "darwin" {
		detail := ""
		if s.AppIcon != "dock" {
			detail = locales.SetAppIconDetail()
		}
		kit.Group(c, pal, "", "", func() {
			kit.Row(c, pal, locales.SetAppIcon(), detail, func() {
				controls.Popup(c, "app-icon", &s.AppIcon, settings.AppIcons, appIconName, 180, pal)
			})
		})
	}
	kit.Group(c, pal, locales.SetStartup(), "", func() {
		kit.Row(c, pal, locales.SetOpenAtLogin(), "", func() {
			style.Toggle(c.Key("login"), &ss.LoginOpen, pal).OnChange(func() {
				if err := mygo.App.SetOpenAtLogin(ss.LoginOpen); err != nil {
					ss.LoginOpen = !ss.LoginOpen
					ss.Toast = locales.SetLoginFailed(err.Error())
				}
			})
		})
	})
	updatesGroup(x, c, pal)
}

// browserPage is Settings › Browser: what browser panes block.
func browserPage(x App, c *ui.Context, pal *theme.Palette) {
	s := x.Settings()
	lists := ""
	if s.BlockAds {
		lists = locales.SetAdListsNote()
	}
	kit.Group(c, pal, locales.SetAdBlocking(), lists, func() {
		kit.Row(c, pal, locales.SetBlockAds(), locales.SetBlockAdsDetail(), func() {
			style.Toggle(c.Key("browser-block-ads"), &s.BlockAds, pal)
		})
		if s.BlockAds {
			adListRows(c, pal, s)
		}
	})
}

// adListRows are the filter lists to block with: those Hyla offers, and
// those of other URLs the configuration names.
func adListRows(c *ui.Context, pal *theme.Palette, s *settings.Settings) {
	sources := slices.Clone(adblock.Catalog)
	for _, id := range s.AdLists {
		if src, ok := adblock.Find(id); ok && !slices.ContainsFunc(sources, func(c adblock.Source) bool { return c.ID == id }) {
			sources = append(sources, src)
		}
	}
	for _, src := range sources {
		on := slices.Contains(s.AdLists, src.ID)
		kit.Row(c, pal, src.Name, adListDetail(src), func() {
			style.Toggle(c.Key("adlist-"+src.ID), &on, pal).OnChange(func() {
				// In the order of the rows, whatever order the file had.
				var lists []string
				for _, other := range sources {
					if other.ID == src.ID && on || other.ID != src.ID && slices.Contains(s.AdLists, other.ID) {
						lists = append(lists, other.ID)
					}
				}
				if lists == nil {
					lists = []string{}
				}
				s.AdLists = lists
			})
		})
	}
}

// adListDetail says what a filter list blocks, or where it is.
func adListDetail(src adblock.Source) string {
	switch src.ID {
	case "easylist":
		return locales.SetAdListEasylist()
	case "easyprivacy":
		return locales.SetAdListEasyprivacy()
	case "peter-lowe":
		return locales.SetAdListPeterLowe()
	case "easylist-china":
		return locales.SetAdListEasylistChina()
	case "fanboy-cookie":
		return locales.SetAdListFanboyCookie()
	case "fanboy-social":
		return locales.SetAdListFanboySocial()
	case "fanboy-annoyance":
		return locales.SetAdListFanboyAnnoyance()
	}
	return src.URL
}

func updatesGroup(x App, c *ui.Context, pal *theme.Palette) {
	ss := x.SettingsState()
	u := &ss.Updates
	version := "Hyla " + mygo.App.Version()
	switch {
	case !u.Enabled:
		version += locales.SetVersionNoUpdate()
	case u.LastChecked.IsZero():
		version += locales.SetVersionNotChecked()
	default:
		version += locales.SetVersionLastChecked(durationWords(time.Since(u.LastChecked)))
	}
	kit.Group(c, pal, locales.SetUpdates(), "", func() {
		kit.Row(c, pal, locales.SetVersion(), version, func() {
			if style.Button(c, "check-updates", locales.SetCheckForUpdates(), "", false, pal) {
				checkUpdates()
			}
		})
		kit.Row(c, pal, locales.SetAutoCheck(), locales.SetAutoCheckDetail(), func() {
			style.Toggle(c.Key("auto-check"), &u.Automatic, pal).OnChange(func() { updater.SetAutomaticChecks(u.Automatic) })
		})
		kit.Row(c, pal, locales.SetAutoInstall(), locales.SetAutoInstallDetail(), func() {
			style.Toggle(c.Key("auto-download"), &u.Download, pal).OnChange(func() { updater.SetAutomaticDownloads(u.Download) })
		})
	})
}

// materialName names a window material as Settings offers it.
func materialName(id string) string {
	switch id {
	case "window":
		return locales.SetMaterialWindow()
	case "popover":
		return locales.SetMaterialPopover()
	case "plastic":
		return locales.SetMaterialPlastic()
	case "glass":
		return locales.SetMaterialGlass()
	case "glass-clear":
		return locales.SetMaterialGlassClear()
	}
	return locales.SetMaterialSidebar()
}

func appearancePage(x App, c *ui.Context, pal *theme.Palette) {
	s := x.Settings()
	kit.Group(c, pal, "", "", func() {
		kit.Row(c, pal, locales.SetAppearance(), "", func() {
			kit.Segmented(c, "appearance", &s.Appearance, []string{"system", "light", "dark"}, []string{locales.SetThemeSystem(), locales.SetThemeLight(), locales.SetThemeDark()})
		})
		kit.Row(c, pal, locales.SetLanguage(), "", func() {
			controls.Popup(c, "language", &s.Language, append([]string{"system"}, settings.Languages...), languageName, 180, pal)
		})
		kit.Row(c, pal, locales.SetTranslucent(), locales.SetTranslucentDetail(), func() {
			style.Toggle(c.Key("vibrancy"), &s.Vibrancy, pal)
		})
		kit.Row(c, pal, locales.SetSessionSwitch(), locales.SetSessionSwitchDetail(), func() {
			controls.Popup(c, "session-switch", &s.SessionSwitch, settings.SessionSwitches, sessionSwitchName, 160, pal)
		})
		kit.Row(c, pal, locales.SetWindowOpacity(), locales.SetWindowOpacityDetail(), func() {
			v := float64(s.WindowOpacity)
			controls.Slider(c, "window-opacity", &v, 0, 1, 160, pal).OnChange(func() {
				s.WindowOpacity = float32(int(v*100+0.5)) / 100
			})
			ui.Textf(c, "%.0f%%", s.WindowOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
		kit.Row(c, pal, locales.SetWindowBlur(), locales.SetWindowBlurDetail(), func() {
			style.Toggle(c.Key("window-blur"), &s.WindowBlur, pal)
		})
		kit.Row(c, pal, locales.SetWindowMaterial(), locales.SetWindowMaterialDetail(), func() {
			controls.Popup(c, "window-material", &s.WindowMaterial, []string{"sidebar", "window", "popover", "plastic", "glass", "glass-clear"}, materialName, 200, pal)
		})
		detail := locales.SetSidebarImageDetail()
		if s.SidebarImage != "" {
			detail = filepath.Base(s.SidebarImage)
		}
		kit.Row(c, pal, locales.SetSidebarImage(), detail, func() {
			if style.Button(c, "sidebar-image", locales.SetSidebarImageChoose(), "", false, pal) {
				x.PickSidebarImage()
			}
			if s.SidebarImage != "" && style.Button(c, "sidebar-image-clear", locales.CommonRemove(), "", false, pal) {
				s.SidebarImage = ""
			}
		})
		if s.SidebarImage != "" {
			kit.Row(c, pal, locales.SetSidebarImageOpacity(), locales.SetSidebarImageOpacityDetail(), func() {
				v := float64(s.SidebarImageOpacity)
				controls.Slider(c, "sidebar-image-opacity", &v, 0, 1, 160, pal).OnChange(func() {
					s.SidebarImageOpacity = float32(int(v*100+0.5)) / 100
				})
				ui.Textf(c, "%.0f%%", s.SidebarImageOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
			})
		}
	})
	kit.Group(c, pal, locales.SetPetGroup(), "", func() {
		kit.Row(c, pal, locales.SetPet(), locales.SetPetDetail(), func() {
			style.Toggle(c.Key("pet"), &s.Pet, pal)
		})
		kit.Row(c, pal, locales.SetPetWhich(), x.PetGallery(), func() {
			x.PetChooser(c, pal)
		})
		kit.Row(c, pal, locales.SetPetSize(), locales.SetPetSizeDetail(), func() {
			v := float64(s.PetSize)
			controls.Slider(c, "pet-size", &v, 0.5, 3, 160, pal).OnChange(func() {
				// The pet grows as the knob drags; the settings keep its
				// size when the knob goes.
				s.PetSize = float32(int(v*10+0.5)) / 10
				x.PetGrow()
			})
			ui.Textf(c, "%.1f×", s.PetSize).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
	})
	ui.Column(c).Gap(8).Children(func() {
		// The washes under their own title, and Plain, which is the
		// window's material alone, with the material's under foot.
		kit.GroupTitle(c, pal, locales.SetBackground())
		ui.Row(c).Gap(12).Wrap().Children(func() {
			for _, b := range settings.Backgrounds {
				if b.ID == "plain" {
					continue
				}
				backgroundTile(x, c, pal, b.ID, backgroundName(b.ID, b.Name))
			}
			for _, pw := range wash.PluginList() {
				backgroundTile(x, c, pal, pw.ID, pw.Name)
			}
		})
		ui.Row(c).Gap(12).Children(func() {
			backgroundTile(x, c, pal, "plain", backgroundName("plain", "Plain"))
		})
	})
}

// languageName names a language in itself, as "简体中文", so that whoever
// reads it finds theirs; "system" in the language the app shows.
// sessionSwitchName names how a session shown in place of another comes
// in, as a slide transition's name does.
func sessionSwitchName(id string) string {
	switch id {
	case "none":
		return locales.SwitchNone()
	case "slide":
		return locales.SwitchSlide()
	case "push":
		return locales.SwitchPush()
	case "cover":
		return locales.SwitchCover()
	case "zoom":
		return locales.SwitchZoom()
	}
	return locales.SwitchDissolve()
}

// paneFitName names how a terminal's picture fills its card, as a macOS
// desktop picture's options do.
func paneFitName(id string) string {
	switch id {
	case "fit":
		return locales.PaneFit()
	case "stretch":
		return locales.PaneStretch()
	case "center":
		return locales.PaneCenter()
	case "tile":
		return locales.PaneTile()
	}
	return locales.PaneFill()
}

func languageName(tag string) string {
	if tag == "system" {
		return locales.SetLanguageSystem()
	}
	return i18n.Autonym(tag)
}

// appIconName names a place the app's icon shows (settings.AppIcons).
func appIconName(id string) string {
	switch id {
	case "dock":
		return locales.SetAppIconDock()
	case "menubar":
		return locales.SetAppIconMenuBar()
	}
	return locales.SetAppIconBoth()
}

// localeTag is the tag of a language setting for App.SetLocale: "" follows
// the system's languages.
func localeTag(language string) string {
	if language == "system" {
		return ""
	}
	return language
}

// backgroundName is a wash's name in the language the app shows.
func backgroundName(id, name string) string {
	if t, ok := map[string]func() string{"rex": locales.BgPetal, "mist": locales.BgMist, "meadow": locales.BgMeadow, "plain": locales.BgPlain, "dock": locales.BgDock,
		"ember": locales.BgEmber, "tidal": locales.BgTidal, "umbra": locales.BgUmbra, "static": locales.BgStatic, "indigo": locales.BgIndigo}[id]; ok {
		return t()
	}
	return name
}

// backgroundTile previews a wash, choosing it on a click.
func backgroundTile(x App, c *ui.Context, pal *theme.Palette, id, name string) {
	s := x.Settings()
	sel := s.Background == id
	tile := ui.ButtonBase(c.Key("bg-" + id)).Column().Width(104).Gap(8).AlignItems(ui.Stretch).Label(name)
	if tile.Clicked() {
		s.Background = id
		if id == "dock" {
			// The Dock's is Liquid Glass under a thin veil: choose it, and
			// let the veil show where the wash covered nothing at all.
			s.WindowMaterial = "glass"
			if s.WindowOpacity < 0.2 {
				s.WindowOpacity = 0.45
			}
		}
	}
	tile.Children(func() {
		sw := style.Raised(ui.Box(c).Size(104, 68).Radius(12), pal).Clip()
		base, glow := wash.Washes(id, pal.Dark)
		sw.LinearGradient(base)
		sw.Children(func() {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(glow)
			if scene := wash.Table[id].Scene; scene != nil {
				// The tile is little: the scene at its size, as the window
				// has it.
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Draw(func(p *ui.Painter, r ui.Rect) { scene(p, r, pal.Dark, 1) })
			}
			// A card on it, as the window shows them.
			ui.Box(c).Absolute().Top(12).Left(26).Right(8).Bottom(-6).Radius(7).Background(pal.Card)
			ui.Box(c).Absolute().Top(20).Left(8).Width(12).Height(4).Radius(2).Background(pal.TextFaint.Alpha(0.5))
			if sel {
				ui.Box(c).Absolute().Top(6).Right(6).Size(18, 18).Radius(9).Center().Background(pal.Accent).Children(func() {
					ui.Box(c).Size(10, 10).Draw(func(p *ui.Painter, r ui.Rect) {
						ck := (&ui.Path{}).MoveTo(r.X+1, r.Y+5.2).LineTo(r.X+4, r.Y+8).LineTo(r.X+9, r.Y+2)
						p.StrokePath(ck, 1.6, ui.Hex("#ffffff"))
					})
				})
			}
		})
		lbl := ui.Text(c, name).FontSize(12.5).TextColor(pal.TextMuted).TextAlign(ui.Center)
		if sel {
			lbl.TextColor(pal.Text).FontWeight(600)
		}
	})
}

// fontChoices are the fonts the picker offers: hyla's own first, then
// the system's, then common coding fonts the user may have installed.
var fontChoices = []string{"JetBrains Mono Nerd Font", "System", "SF Mono", "Menlo", "Monaco", "JetBrains Mono", "Fira Code", "Cascadia Code",
	"Iosevka", "Berkeley Mono", "Geist Mono", "IBM Plex Mono", "Source Code Pro", "Hack", "Maple Mono"}

// fontOptions are the families the font menu offers: hyla's own, the
// system's, the common coding fonts, and every monospaced family the
// machine has, so that a font installed with the system is there to pick.
// Fonts installed while hyla runs show after a restart, as MyGo lists
// them once. The system's is named in the language the app shows now.
func fontOptions() []string {
	return append([]string{fontChoices[0], locales.SetFontSystem()}, otherFonts()...)
}

// otherFonts are the families of fontOptions after hyla's own and the
// system's, listed once.
var otherFonts = sync.OnceValue(func() []string {
	var opts []string
	seen := map[string]bool{fontChoices[0]: true, bundledFont: true}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		opts = append(opts, name)
	}
	for _, name := range fontChoices[2:] {
		add(name)
	}
	for _, f := range ui.FontFamilies() {
		if f.Monospaced {
			add(f.Name)
		}
	}
	return opts
})

func terminalPage(x App, c *ui.Context, pal *theme.Palette) {
	ss, s := x.SettingsState(), x.Settings()
	terminalPreview(x, c, pal)
	kit.Group(c, pal, locales.SetText(), "", func() {
		kit.Row(c, pal, locales.SetFont(), locales.SetFontDetail(), func() {
			// The picker's names for the settings' values: "" is hyla's
			// font, "monospace" the system's.
			family := s.FontFamily
			switch family {
			case "":
				family = "JetBrains Mono Nerd Font"
			case "monospace":
				family = locales.SetFontSystem()
			}
			opts := fontOptions()
			if !slices.Contains(opts, family) {
				opts = append([]string{family}, opts...)
			}
			controls.SearchPopup(c, &ss.Menus, "font", family, opts, 240, locales.SetUseFont, pal, func(v string) {
				switch v {
				case "JetBrains Mono Nerd Font":
					v = ""
				case locales.SetFontSystem():
					v = "monospace"
				}
				s.FontFamily = v
			})
		})
		kit.Group(c, pal, "", locales.SetFontFallbackDetail(), func() {
			fontFallbackField(x, c, pal)
		})
		kit.Row(c, pal, locales.SetSize(), locales.SetSizeDetail(x.Keys("text.bigger"), x.Keys("text.smaller")), func() {
			sizes := []float32{10, 11, 12, 13, 14, 15, 16, 18, 20, 22, 24, 28}
			if !slices.Contains(sizes, s.FontSize) {
				sizes = append(sizes, s.FontSize)
				slices.Sort(sizes)
			}
			controls.Popup(c, "size", &s.FontSize, sizes, func(v float32) string { return strconv.FormatFloat(float64(v), 'f', -1, 32) + " pt" }, 90, pal)
		})
		kit.Row(c, pal, locales.SetLineHeight(), "", func() {
			v := float64(s.LineHeight)
			controls.Slider(c, "lh", &v, 1, 2, 160, pal).OnChange(func() { s.LineHeight = float32(int(v*100+0.5)) / 100 })
			ui.Textf(c, "%.2f×", s.LineHeight).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
		kit.Row(c, pal, locales.SetWeight(), "", func() {
			controls.Popup(c, "weight", &s.FontWeight, []int{100, 200, 300, 400, 500, 600}, func(w int) string {
				return map[int]string{
					100: locales.SetWeightThin(), 200: locales.SetWeightExtraLight(),
					300: locales.SetWeightLight(), 400: locales.SetWeightRegular(),
					500: locales.SetWeightMedium(), 600: locales.SetWeightSemibold(),
				}[w]
			}, 140, pal)
		})
		kit.Row(c, pal, locales.SetLigatures(), locales.SetLigaturesDetail(), func() {
			style.Toggle(c.Key("liga"), &s.Ligatures, pal)
		})
		kit.Row(c, pal, locales.SetFontFeatures(), locales.SetFontFeaturesDetail(), func() {
			fontFeaturesField(x, c, pal)
		})
		kit.Row(c, pal, locales.SetThicken(), locales.SetThickenDetail(), func() {
			style.Toggle(c.Key("thicken"), &s.Thicken, pal)
		})
	})
	themes := theme.TerminalThemes()
	kit.Group(c, pal, locales.SetColors(), locales.SetColorsDetail(), func() {
		kit.Row(c, pal, locales.SetLightWindows(), "", func() {
			controls.SearchPopup(c, &ss.Menus, "light", s.LightTheme, themes, 240, nil, pal, func(v string) { s.LightTheme = v })
		})
		kit.Row(c, pal, locales.SetDarkWindows(), "", func() {
			controls.SearchPopup(c, &ss.Menus, "dark", s.DarkTheme, themes, 240, nil, pal, func(v string) { s.DarkTheme = v })
		})
		kit.Row(c, pal, locales.SetDimUnfocused(), locales.SetDimUnfocusedDetail(), func() {
			style.Toggle(c.Key("dim-unfocused"), &s.DimUnfocused, pal)
		})
	})
	// As the window's translucency: an opacity, and a blur or none.
	kit.Group(c, pal, locales.SetPaneGlass(), "", func() {
		kit.Row(c, pal, locales.SetPaneOpacity(), locales.SetPaneOpacityDetail(), func() {
			v := float64(s.PaneOpacity)
			controls.Slider(c, "pane-opacity", &v, 0, 1, 160, pal).OnChange(func() {
				s.PaneOpacity = float32(int(v*100+0.5)) / 100
			})
			ui.Textf(c, "%.0f%%", s.PaneOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
		kit.Row(c, pal, locales.SetPaneBlur(), locales.SetPaneBlurDetail(), func() {
			style.Toggle(c.Key("pane-blur"), &s.PaneBlur, pal)
		})
		kit.Row(c, pal, locales.SetPaneMaterial(), locales.SetPaneMaterialDetail(), func() {
			controls.Popup(c, "pane-material", &s.PaneMaterial, []string{"glass", "plastic", "glass-clear"}, materialName, 200, pal)
		})
		detail := locales.SetPaneImageDetail()
		if s.PaneImage != "" {
			detail = filepath.Base(s.PaneImage)
		}
		kit.Row(c, pal, locales.SetPaneImage(), detail, func() {
			if style.Button(c, "pane-image", locales.SetSidebarImageChoose(), "", false, pal) {
				x.PickPaneImage()
			}
			if s.PaneImage != "" && style.Button(c, "pane-image-clear", locales.CommonRemove(), "", false, pal) {
				s.PaneImage = ""
			}
		})
		if s.PaneImage != "" {
			kit.Row(c, pal, locales.SetPaneImageOpacity(), locales.SetPaneImageOpacityDetail(), func() {
				v := float64(s.PaneImageOpacity)
				controls.Slider(c, "pane-image-opacity", &v, 0, 1, 160, pal).OnChange(func() {
					s.PaneImageOpacity = float32(int(v*100+0.5)) / 100
				})
				ui.Textf(c, "%.0f%%", s.PaneImageOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
			})
			kit.Row(c, pal, locales.SetPaneImageFit(), locales.SetPaneImageFitDetail(), func() {
				controls.Popup(c, "pane-image-fit", &s.PaneImageFit, settings.PaneImageFits, paneFitName, 200, pal)
			})
		}
	})
	kit.Group(c, pal, locales.SetCursor(), "", func() {
		kit.Row(c, pal, locales.SetShape(), locales.SetShapeDetail(), func() {
			kit.Segmented(c, "cursor", &s.Cursor, []string{"block", "bar", "underline"}, []string{locales.SetCursorBlock(), locales.SetCursorBar(), locales.SetCursorUnderline()})
		})
		kit.Row(c, pal, locales.SetBlink(), "", func() { style.Toggle(c.Key("blink"), &s.CursorBlink, pal) })
	})
	kit.Group(c, pal, locales.SetInput(), "", func() {
		kit.Row(c, pal, locales.SetOptionMeta(), locales.SetOptionMetaDetail(), func() {
			style.Toggle(c.Key("optalt"), &s.OptionAsAlt, pal)
		})
		kit.Row(c, pal, locales.SetCopyOnSelect(), locales.SetCopyOnSelectDetail(), func() {
			style.Toggle(c.Key("cos"), &s.CopyOnSelect, pal)
		})
		kit.Row(c, pal, locales.SetRightClick(), "", func() {
			controls.Popup(c, "rc", &s.RightClick, []string{"menu", "paste", "copyOrPaste"}, func(id string) string {
				return map[string]string{"menu": locales.SetRightClickMenu(), "paste": locales.SetRightClickPaste(), "copyOrPaste": locales.SetRightClickCopyOrPaste()}[id]
			}, 170, pal)
		})
		kit.Row(c, pal, locales.SetClipboardRead(), locales.SetClipboardReadDetail(), func() {
			kit.Segmented(c, "clip", &s.Clipboard, []string{"ask", "allow", "deny"}, []string{locales.SetAsk(), locales.SetAllow(), locales.SetDeny()})
		})
	})
	// mouse-scroll-multiplier's discrete: and precision:.
	kit.Group(c, pal, locales.SetScrolling(), "", func() {
		kit.Row(c, pal, locales.SetWheelScroll(), locales.SetWheelScrollDetail(), func() {
			rows := []float32{1, 2, 3, 4, 5, 6, 8, 10}
			if !slices.Contains(rows, s.ScrollDiscrete) {
				rows = append(rows, s.ScrollDiscrete)
				slices.Sort(rows)
			}
			controls.Popup(c, "wheel", &s.ScrollDiscrete, rows, func(n float32) string {
				if n == float32(int(n)) {
					return locales.SetScrollRows(int(n))
				}
				return strconv.FormatFloat(float64(n), 'f', -1, 32)
			}, 110, pal)
		})
		kit.Row(c, pal, locales.SetTrackpadScroll(), locales.SetTrackpadScrollDetail(), func() {
			v := float64(s.ScrollPrecision)
			controls.Slider(c, "trackpad", &v, 0.25, 3, 160, pal).OnChange(func() { s.ScrollPrecision = float32(int(v*20+0.5)) / 20 })
			ui.Textf(c, "%.2f×", s.ScrollPrecision).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
	})
}

// terminalPreview shows text as the terminals would draw it: the font,
// its size and line height, and the colors of the window's appearance.
func terminalPreview(x App, c *ui.Context, pal *theme.Palette) {
	s := x.Settings()
	light, dark := x.TermThemes()
	th := light
	if pal.Dark {
		th = dark
	}
	features := x.UIFeatures(x.FontFeatures())
	line := func(spans ...ui.Span) {
		// The preview draws in the UI's font, and takes the terminals' size,
		// weight and features: another family shapes the lines with its own
		// advances, and the preview keeps no cell width of its own.
		for i := range spans {
			spans[i].Size, spans[i].Weight, spans[i].Features = s.FontSize, s.FontWeight, features
		}
		ui.RichText(c, spans...).LineHeight(s.LineHeight).SingleLine()
	}
	box := style.Raised(ui.Column(c).Radius(12).Padding(16, 18).Gap(0).Background(th.Background), pal)
	box.Children(func() {
		// Nerd Fonts' folder and branch icons: fonts without them show
		// boxes, which is worth knowing before picking one.
		line(ui.Span{Text: "\uf07b ~/dev/hyla ", Color: th.Palette[4]}, ui.Span{Text: "\ue0a0 main ", Color: th.Palette[5]}, ui.Span{Text: "❯ ", Color: th.Palette[2]}, ui.Span{Text: "git status", Color: th.Foreground})
		line(ui.Span{Text: "modified:   ", Color: th.Foreground}, ui.Span{Text: "settings_view.go", Color: th.Palette[1]})
		line(ui.Span{Text: "func ", Color: th.Palette[5]}, ui.Span{Text: "main", Color: th.Palette[4]}, ui.Span{Text: "() { x := a != b && c >= d => ok }", Color: th.Foreground})
		ui.Row(c).Gap(4).Margin(10, 0, 0, 0).Children(func() {
			for i := range 16 {
				ui.Box(c).Size(16, 10).Radius(3).Background(th.Palette[i])
			}
		})
	})
}

// fontFallbackField shows font-fallback as the families in the order they
// are tried, each removed by its x, and above them a search menu of the
// families installed, which adds one: a font may lack Chinese, emoji or
// icons, which a fallback of the user's draws. It returns the button that
// adds one.
func fontFallbackField(x App, c *ui.Context, pal *theme.Palette) {
	ss := x.SettingsState()
	cur := x.FallbackFonts()
	for i, f := range cur {
		family, at := f, i
		ui.Row(c.Key("fb-"+family)).Padding(8, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%d", at+1).FontSize(12).TextColor(pal.TextFaint).Width(14).FontFeatures("tnum").Shrink(0)
			ui.Text(c, family).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1).Grow(1)
			paints.IconButton(c, "fb-x-"+family, glyph.Close, locales.CommonRemove(), pal, func() {
				x.RemoveFallback(at)
			})
		})
	}
	label := locales.SetFallbackAdd()
	if len(cur) > 0 {
		label = locales.SetFallbackAddMore()
	}
	ui.Row(c).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		controls.SearchPopup(c, &ss.Menus, "fallback", label, fallbackOptions(), 240, locales.SetUseFont, pal, func(v string) {
			x.AddFallback(v)
		})
	})
}

// fallbackOptions are the families a fallback may be: the font menu's
// after hyla's own and the system's, then every other family the machine
// has: first those whose names name Chinese, Japanese or Korean, then the
// rest, each listed once. MyGo lists the machine's families once, so one
// installed while hyla runs shows after a restart.
func fallbackOptions() []string {
	var out, rest []string
	seen := map[string]bool{fontChoices[0]: true, bundledFont: true}
	add := func(list *[]string, name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		*list = append(*list, name)
	}
	for _, name := range fontChoices[2:] {
		add(&out, name)
	}
	for _, f := range ui.FontFamilies() {
		if f.Monospaced {
			add(&out, f.Name)
		}
	}
	for _, f := range ui.FontFamilies() {
		if cjkFamily(f.Name) {
			add(&out, f.Name)
		} else {
			add(&rest, f.Name)
		}
	}
	return append(out, rest...)
}

// cjkFamily tells, by the name, that a family draws Chinese, Japanese or
// Korean, as the fallback a terminal's font lacks them wants.
func cjkFamily(name string) bool {
	for _, s := range []string{"CJK", "Hei", "Song", "Kai", "Ming", "Gothic", "Mincho", "Myeongjo", "YaHei", "Gulim",
		"PingFang", "Hiragino", "Noto Sans SC", "Noto Sans TC", "Noto Sans JP", "Noto Sans KR", "Noto Serif SC", "Noto Serif TC", "Noto Serif JP", "Noto Serif KR",
		"Source Han", "WenQuanYi", "Droid Sans Fallback", "Sarasa", "LXGW", "Hong Kong", "Macau", "Zhuyin"} {
		if strings.Contains(name, s) {
			return true
		}
	}
	return false
}

// fontFeaturesField edits font-feature as a comma-separated list. What
// it can read applies as it is submitted or loses the focus; a tag it
// can't is left in the field, marked, for the user to fix.
func fontFeaturesField(x App, c *ui.Context, pal *theme.Palette) {
	ss, s := x.SettingsState(), x.Settings()
	if !ss.FeaturesEditing {
		ss.FeaturesDraft = strings.Join(s.FontFeatures, ", ")
	}
	in := controls.TextField(c, "features", &ss.FeaturesDraft, "ss01, cv05=2, -calt", 200, pal).Font("monospace")
	ss.FeaturesEditing = in.Focused()
	_, err := settings.ParseFontFeatures(ss.FeaturesDraft)
	if err != nil {
		in.TextColor(pal.Danger)
	}
	keep := func() {
		if fs, err := settings.ParseFontFeatures(ss.FeaturesDraft); err == nil && !slices.Equal(fs, s.FontFeatures) {
			s.FontFeatures = fs
		}
	}
	in.OnSubmit(keep)
	if !ss.FeaturesEditing {
		keep()
	}
}

func shellPage(x App, c *ui.Context, pal *theme.Palette) {
	ss, s := x.SettingsState(), x.Settings()
	login := shell.Login()
	note := locales.SetShellGroupDetail()
	if x.HostCount() > 1 {
		note += " " + locales.SetShellThisComputer()
	}
	kit.Group(c, pal, locales.SetShellGroup(), note, func() {
		kit.Row(c, pal, locales.SetShell(), locales.SetShellDetail(proto.ShortDir(login)), func() {
			if !ss.ShellEditing {
				ss.ShellDraft = s.Shell
			}
			in := controls.TextField(c, "shell", &ss.ShellDraft, proto.ShortDir(login), 220, pal)
			ss.ShellEditing = in.Focused()
			// Kept as it is submitted, or as the field loses the focus.
			in.OnSubmit(func() { s.Shell = strings.TrimSpace(ss.ShellDraft) })
			if !ss.ShellEditing && strings.TrimSpace(ss.ShellDraft) != s.Shell {
				s.Shell = strings.TrimSpace(ss.ShellDraft)
			}
		})
		kit.Row(c, pal, locales.SetScrollback(), locales.SetScrollbackDetail(), func() {
			sizes := []int{0, 1, 4, 16, 64}
			if !slices.Contains(sizes, s.Scrollback) {
				sizes = append(sizes, s.Scrollback)
			}
			controls.Popup(c, "sb", &s.Scrollback, sizes, func(mb int) string {
				if mb == 0 {
					return locales.CommonNone()
				}
				return strconv.Itoa(mb) + " MB"
			}, 130, pal)
		})
	})
	kit.Group(c, pal, locales.SetEnvironment(), locales.SetEnvironmentDetail(), func() {
		for i, kv := range s.Env {
			k, v, _ := strings.Cut(kv, "=")
			ui.Row(c.Key("env"+strconv.Itoa(i))).Padding(8, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, k).Font("monospace").FontSize(12.5).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, v).Font("monospace").FontSize(12.5).TextColor(pal.TextMuted).Grow(1).SingleLine().Ellipsis("…").MinWidth(0)
				idx := i
				paints.IconButton(c, "rm", glyph.Close, locales.CommonRemove(), pal, func() {
					s.Env = slices.Delete(slices.Clone(s.Env), idx, idx+1)
				})
			})
		}
		ui.Row(c).Padding(8, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			addEnv := func() {
				if kv := strings.TrimSpace(ss.EnvDraft); strings.Contains(kv, "=") {
					s.Env = append(slices.Clone(s.Env), kv)
					ss.EnvDraft = ""
				}
			}
			controls.TextField(c, "env-new", &ss.EnvDraft, "NAME=value", 0, pal).Font("monospace").OnSubmit(addEnv)
			if style.FieldButton(c, "env-add", locales.CommonAdd(), false, pal) {
				addEnv()
			}
		})
	})
}

func notificationsPage(x App, c *ui.Context, pal *theme.Palette) {
	ss, s := x.SettingsState(), x.Settings()
	note := ""
	if !mygo.NotificationsSupported() {
		note = locales.SetNotifyUnpackaged()
	}
	kit.Group(c, pal, locales.SetNotifyWhen(), note, func() {
		kit.Row(c, pal, locales.SetNotifyWaiting(), locales.SetNotifyWaitingDetail(), func() {
			style.Toggle(c.Key("nw"), &s.NotifyWaiting, pal)
		})
		kit.Row(c, pal, locales.SetNotifyDone(), locales.SetNotifyDoneDetail(), func() {
			style.Toggle(c.Key("nd"), &s.NotifyDone, pal)
		})
		kit.Row(c, pal, locales.SetNotifyFailed(), locales.SetNotifyFailedDetail(), func() {
			style.Toggle(c.Key("nf"), &s.NotifyFailed, pal)
		})
	})
	kit.Group(c, pal, locales.SetHow(), "", func() {
		kit.Row(c, pal, locales.SetPlaySound(), "", func() { style.Toggle(c.Key("ns"), &s.NotifySound, pal) })
		kit.Row(c, pal, locales.SetDockBadge(), locales.SetDockBadgeDetail(), func() { style.Toggle(c.Key("nb"), &s.DockBadge, pal) })
		kit.Row(c, pal, locales.SetDockBounce(), locales.SetDockBounceDetail(), func() { style.Toggle(c.Key("nbo"), &s.BounceDock, pal) })
	})
	if style.Button(c, "test-notif", locales.SetTestNotification(), "", false, pal) {
		n := mygo.NewNotification(mygo.NotificationOptions{Title: "Hyla", Body: locales.SetTestNotificationBody(), Silent: !s.NotifySound})
		go func() {
			if err := n.Show(); err != nil {
				x.Main(func() {
					ss.Toast = locales.SetNotifyError(err.Error())
					if x.SettingsWindow() != nil {
						x.SettingsWindow().Invalidate()
					}
				})
			}
		}()
	}
}

func agentsPage(x App, c *ui.Context, pal *theme.Palette) {
	ss := x.SettingsState()
	s := x.Settings()
	kit.Group(c, pal, locales.SetRemoteControl(), "", func() {
		kit.Row(c, pal, locales.SetRemoteControlRow(), locales.SetRemoteControlDetail(), func() {
			style.Toggle(c.Key("remote"), &s.RemoteControl, pal)
		})
	})
	codeBlock(c, pal, "hyla do pane.split.down\nhyla do session.attention\nhyla actions        # what hyla do runs")
	kit.Group(c, pal, locales.SetCli(), locales.SetCliDetail(), func() {
		detail := locales.SetCliNotInstalled()
		if ss.CLIPath != "" {
			detail = ss.CLIPath
		}
		kit.Row(c, pal, locales.SetCliRow(), detail, func() {
			label := locales.CommonInstall()
			if ss.CLIPath != "" {
				label = locales.CommonReinstall()
			}
			if style.Button(c, "cli", label, "", ss.CLIPath == "", pal) {
				x.InstallCLI()
				time.AfterFunc(1500*time.Millisecond, func() { x.Main(x.RefreshAgents) })
			}
		})
	})
	kit.Group(c, pal, locales.SetSkills(), locales.SetSkillsDetail(), func() {
		for _, st := range cli.SkillStates() {
			name := map[string]string{"claude": "Claude Code", "agents": "Codex, OpenCode, Pi"}[st.Agent]
			detail := map[string]string{"current": locales.SetSkillInstalled(), "outdated": locales.SetSkillOutdated(), "missing": locales.SetSkillMissing()}[st.State] + " · " + proto.ShortDir(st.Dir)
			kit.Row(c, pal, name, detail, func() {
				agent := st.Agent
				if st.State != "missing" && style.Button(c, "rm-"+agent, locales.CommonRemove(), "", false, pal) {
					if err := cli.RemoveSkill(agent); err != nil {
						c.Toast(err.Error())
					}
				}
				if st.State != "current" {
					label := locales.CommonInstall()
					if st.State == "outdated" {
						label = locales.CommonUpdate()
					}
					if style.Button(c, "in-"+agent, label, "", true, pal) {
						if err := cli.InstallSkill(agent); err != nil {
							c.Toast(err.Error())
						}
					}
				}
			})
		}
	})
	kit.Group(c, pal, locales.SetProgramStatus(), locales.SetProgramStatusDetail(), nil)
	codeBlock(c, pal, "hyla report working \"Deploying\" --progress 40\nhyla report blocked \"Approve the migration?\" --kind permission\nhyla report done \"Deployed\"")
}

func codeBlock(c *ui.Context, pal *theme.Palette, code string) {
	ui.Text(c, code).Font("monospace").FontSize(12).LineHeight(1.55).TextColor(pal.Text).
		Padding(12, 16).Radius(10).Background(pal.Chip).Selectable()
}

func advancedPage(x App, c *ui.Context, pal *theme.Palette) {
	ss, s := x.SettingsState(), x.Settings()
	kit.Group(c, pal, locales.SetConfiguration(), locales.SetConfigurationDetail(), func() {
		kit.Row(c, pal, locales.SetConfigFile(), proto.ShortDir(settings.Path()), func() {
			if style.Button(c, "reveal-config", locales.CommonShowIn(platform.FileManager()), "", false, pal) {
				ensureConfig(x)
				platform.RevealPath(settings.Path())
			}
			if style.Button(c, "edit-config", locales.CommonOpen(), "", true, pal) {
				x.OpenConfig()
			}
		})
		for i, p := range ss.Problems {
			ui.Row(c.Key("problem"+strconv.Itoa(i))).Padding(0, 0, 8, 0).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(pal.Attention).Shrink(0)
				ui.Text(c, p).FontSize(12.5).TextColor(pal.Attention).Grow(1).MinWidth(0)
			})
		}
	})
	kit.Group(c, pal, locales.SetProxy(), locales.SetProxyDetail(), func() {
		kit.Row(c, pal, locales.SetProxyTitle(), "", func() {
			if !ss.ProxyEditing {
				ss.ProxyDraft = s.HTTPProxy
			}
			in := controls.TextField(c, "proxy", &ss.ProxyDraft, "http://127.0.0.1:7890", 260, pal).Font("monospace")
			ss.ProxyEditing = in.Focused()
			// Kept as it is submitted, or as the field loses the focus.
			in.OnSubmit(func() { s.HTTPProxy = strings.TrimSpace(ss.ProxyDraft) })
			if !ss.ProxyEditing && strings.TrimSpace(ss.ProxyDraft) != s.HTTPProxy {
				s.HTTPProxy = strings.TrimSpace(ss.ProxyDraft)
			}
		})
	})
	kit.Group(c, pal, locales.SetBackgroundService(), locales.SetBackgroundServiceDetail(), func() {
		kit.Row(c, pal, "hylad", ss.Service, func() {
			if style.Button(c, "svc-restart", locales.CommonRestart(), "", false, pal) {
				ss.Confirm = &confirm.Ask{
					Open: true, Title: locales.ConfirmRestartServiceTitle(),
					Detail: locales.RestartServiceDetail(),
					Action: locales.CommonRestart(), Destroy: true,
					Run: func() { x.RestartDaemon(); time.AfterFunc(2*time.Second, func() { x.Main(x.RefreshService) }) },
				}
			}
		})
		kit.Row(c, pal, locales.SetLog(), proto.ShortDir(filepath.Join(proto.Dir(), "hylad.log")), func() {
			if style.Button(c, "log", locales.CommonShow(), "", false, pal) {
				platform.RevealPath(filepath.Join(proto.Dir(), "hylad.log"))
			}
		})
	})
	kit.Group(c, pal, locales.SetData(), locales.SetDataDetail(), func() {
		kit.Row(c, pal, locales.SetFolder(), proto.ShortDir(proto.Dir()), func() {
			if style.Button(c, "reveal", locales.CommonShowIn(platform.FileManager()), "", false, pal) {
				platform.OpenPath(proto.Dir())
			}
		})
		kit.Row(c, pal, locales.SetReset(), locales.SetResetDetail(), func() {
			if style.Button(c, "reset", locales.SetResetButton(), "", false, pal) {
				ss.Confirm = &confirm.Ask{
					Open: true, Title: locales.SetResetTitle(), Detail: locales.SetResetConfirmDetail(), Action: locales.SetResetAction(), Destroy: true,
					Run: func() { x.ApplySettings(settings.Default()) },
				}
			}
		})
	})
}

// ensureConfig writes the configuration file, with its header, when
// there is none yet, so that there is a file to open.
func ensureConfig(x App) {
	if _, err := os.Stat(settings.Path()); err != nil {
		settings.Save(*x.Settings())
	}
}

// openConfig opens the configuration in the user's text editor.
func openConfig(x App) {
	ensureConfig(x)
	platform.OpenTextFile(settings.Path())
}

// --- keyboard ---

func keyboardPage(x App, c *ui.Context, pal *theme.Palette) {
	ss, s := x.SettingsState(), x.Settings()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		controls.TextField(c, "kb-search", &ss.Filter, locales.SetKeyboardSearch(), 0, pal)
		if len(s.Shortcuts) > 0 && style.Button(c, "kb-reset", locales.SetRestoreDefaults(), "", false, pal) {
			s.Shortcuts = nil
			ss.Notice = ""
		}
	})
	if ss.Notice != "" {
		ui.Text(c, ss.Notice).FontSize(12).TextColor(pal.Attention).Padding(0, 4)
	}
	q := strings.ToLower(strings.TrimSpace(ss.Filter))
	allActs := actions.List()
	menus := []string{"Hyla", "Session", "Edit", "Pane", "View", "Plugins"}
	for _, menu := range menus {
		var acts []actions.Action
		for _, act := range allActs {
			keys := actions.Glyphs(actions.Keys(act, s.Shortcuts))
			found := q == "" || strings.Contains(strings.ToLower(x.ActionTitle(act.ID)), q) || strings.Contains(strings.ToLower(act.Title), q) || strings.Contains(strings.ToLower(keys), q) || strings.Contains(act.ID, q)
			if kit.Hits != nil {
				found = found && kit.Hits[strings.TrimSuffix(x.ActionTitle(act.ID), "…")]
			}
			inGroup := (act.PluginID == "" && act.Menu == menu) || (menu == "Plugins" && act.PluginID != "")
			if inGroup && found {
				acts = append(acts, act)
			}
		}
		if len(acts) == 0 {
			continue
		}
		var title string
		if menu == "Plugins" {
			title = locales.MenuPlugins()
		} else {
			title = map[string]func() string{"Hyla": locales.MenuApp, "Session": locales.MenuSession, "Edit": locales.MenuEdit, "Pane": locales.MenuPane, "View": locales.MenuView}[menu]()
		}
		ui.Column(c.Key("kb-grp-" + menu)).Gap(8).Children(func() {
			kit.Group(c, pal, title, "", func() {
				for _, act := range acts {
					shortcutRow(x, c, pal, act)
				}
			})
		})
	}
	ui.Text(c, locales.SetKeyboardHelp()).FontSize(12).TextColor(pal.TextFaint).Padding(0, 4)
}

func shortcutRow(x App, c *ui.Context, pal *theme.Palette, act actions.Action) {
	ss, s := x.SettingsState(), x.Settings()
	keys := actions.Keys(act, s.Shortcuts)
	_, custom := s.Shortcuts[act.ID]
	ui.Row(c.Key(act.ID)).Padding(7, 0).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, strings.TrimSuffix(x.ActionTitle(act.ID), "…")).FontSize(13.5).TextColor(pal.Text).SingleLine()
			if act.Global {
				ui.Text(c, locales.SetShortcutGlobal()).FontSize(11.5).TextColor(pal.TextFaint).SingleLine()
			}
		})
		if custom && keys != act.Keys {
			b := ui.ButtonBase(c.Key("undo")).Size(22, 22).Radius(6).Center().TextColor(pal.TextFaint).Label(locales.SetUseDefault(actions.Glyphs(act.Keys))).Tooltip(locales.SetDefaultKeys(actions.Glyphs(act.Keys)))
			if b.Hovered() {
				b.Background(pal.RowHover).TextColor(pal.Text)
			}
			b.Children(func() { ui.Icon(c, glyph.Return).Size(12, 12) })
			if b.Clicked() {
				delete(s.Shortcuts, act.ID)
				if len(s.Shortcuts) == 0 {
					s.Shortcuts = nil
				}
			}
		}
		recording := ss.Recording == act.ID
		chip := ui.ButtonBase(c.Key("keys")).Height(26).MinWidth(84).Padding(0, 10).Radius(7).Center().Label(locales.SetShortcutFor(strings.TrimSuffix(x.ActionTitle(act.ID), "…")))
		bg := pal.Control
		switch {
		case recording:
			bg = pal.Accent.Alpha(0.18)
		case chip.Hovered():
			bg = pal.ControlHover
		}
		style.Face(chip, pal, bg)
		if chip.Clicked() && !recording {
			x.StartRecording(act.ID)
			ss.RecordRef.Focus()
		}
		if recording {
			chip.Bind(&ss.RecordRef).HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputKeyDown {
					return ev.Kind == ui.InputText
				}
				return recordKey(x, act, ev.Mods, ev.Key)
			})
			if !ss.RecordRef.Focused(c) && !chip.Pressed() {
				// Focus went elsewhere: the shortcut stays.
				c.After(200 * time.Millisecond)
			}
		}
		chip.Children(func() {
			switch {
			case recording:
				ui.Text(c, locales.SetTypeShortcut()).FontSize(12.5).TextColor(pal.Accent).SingleLine()
			case keys == "":
				ui.Text(c, locales.CommonNone()).FontSize(12.5).TextColor(pal.TextFaint).SingleLine()
			default:
				ui.Text(c, actions.Glyphs(keys)).FontSize(13).TextColor(pal.Text).SingleLine().FontFeatures("tnum")
			}
		})
	})
}

// setRecording starts or ends typing a shortcut. While one is typed, the
// menus give up their key equivalents, which macOS gives keys to first:
// else typing one bound already, as ⌘D, would split a pane, not record.
func setRecording(x App, id string) {
	ss := x.SettingsState()
	if ss.Recording == id {
		return
	}
	was := ss.Recording != ""
	ss.Recording = id
	if was != (id != "") && x.RootWindow() != nil {
		mygo.App.SetMenu(x.Menu())
		x.ApplyGlobalShortcuts()
	}
}

// shortcutNeeds tells what a shortcut needs, typed without it.
func shortcutNeeds() string {
	if runtime.GOOS == "darwin" {
		return locales.SetShortcutNeedsMac()
	}
	return locales.SetShortcutNeedsPc()
}

// recordKey takes a key typed for a shortcut, reporting whether it took it.
func recordKey(x App, act actions.Action, mods ui.Modifiers, key ui.Key) bool {
	ss := x.SettingsState()
	switch {
	case key == ui.KeyEscape && mods == 0:
		x.StartRecording("")
		return true
	case (key == ui.KeyBackspace || key == ui.KeyDelete) && mods == 0:
		setShortcut(x, act.ID, "")
		x.StartRecording("")
		return true
	}
	name, ok := keyNames[key]
	if !ok {
		return false // a modifier alone, or a key menus can't take
	}
	if mods&(ui.Super|ui.Ctrl|ui.Alt) == 0 && !strings.HasPrefix(name, "F") {
		ss.Notice = shortcutNeeds()
		return true
	}
	var parts []string
	if mods&ui.Super != 0 && runtime.GOOS == "darwin" {
		parts = append(parts, "CmdOrCtrl")
	} else if mods&ui.Super != 0 {
		parts = append(parts, "Super")
	}
	if mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	accel := strings.Join(append(parts, name), "+")
	setShortcut(x, act.ID, accel)
	x.StartRecording("")
	return true
}

// setShortcut binds an action, taking the keys from any other that had
// them.
func setShortcut(x App, id, accel string) {
	ss, s := x.SettingsState(), x.Settings()
	sc := map[string]string{}
	for k, v := range s.Shortcuts {
		sc[k] = v
	}
	ss.Notice = ""
	if accel != "" {
		for _, other := range actions.List() {
			if other.ID != id && strings.EqualFold(actions.Keys(other, sc), accel) {
				sc[other.ID] = ""
				ss.Notice = locales.SetShortcutTaken(actions.Glyphs(accel), strings.TrimSuffix(x.ActionTitle(other.ID), "…"))
			}
		}
	}
	act, _ := actions.Find(id)
	if accel == act.Keys {
		delete(sc, id)
	} else {
		sc[id] = accel
	}
	if len(sc) == 0 {
		sc = nil
	}
	s.Shortcuts = sc
}

// keyNames are the keys a shortcut takes, by their accelerator names.
var keyNames = func() map[ui.Key]string {
	m := map[ui.Key]string{
		ui.KeyEnter: "Enter", ui.KeyTab: "Tab", ui.KeySpace: "Space", ui.KeyBackspace: "Backspace", ui.KeyDelete: "Delete",
		ui.KeyEscape: "Escape", ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown",
		ui.KeyLeft: "Left", ui.KeyRight: "Right", ui.KeyUp: "Up", ui.KeyDown: "Down",
		ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".", ui.KeySlash: "/", ui.KeySemicolon: ";",
		ui.KeyQuote: "'", ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]", ui.KeyBackslash: "\\", ui.KeyBackquote: "`",
	}
	fs := []ui.Key{ui.KeyF1, ui.KeyF2, ui.KeyF3, ui.KeyF4, ui.KeyF5, ui.KeyF6, ui.KeyF7, ui.KeyF8, ui.KeyF9, ui.KeyF10, ui.KeyF11, ui.KeyF12}
	for i, k := range fs {
		m[k] = "F" + strconv.Itoa(i+1)
	}
	digits := []ui.Key{ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
	for i, k := range digits {
		m[k] = strconv.Itoa(i)
	}
	letters := []ui.Key{ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH, ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM,
		ui.KeyN, ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR, ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ}
	for i, k := range letters {
		m[k] = string(rune('A' + i))
	}
	return m
}()

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// searchPagesFor are the window's pages, as the search knows them.
func searchPagesFor(x App) []search.Page {
	_, _, all := allSettingsPages(x)
	pages := make([]search.Page, 0, len(all))
	for _, p := range all {
		pages = append(pages, search.Page{ID: p.id, Title: p.title()})
	}
	return pages
}

// --- the pages the root keeps ---

// pageFunc is a page that reads beyond the window's state, as the host
// forms and the Sync and Feishu pages do: it is the app's.
type pageFunc func(x App, c *ui.Context, pal *theme.Palette)

// OnPages are the pages the app draws in place of the window's own.
var (
	SyncPage        pageFunc
	BeaconTowerPage pageFunc
	TailcatPage     pageFunc
	P2PPage         pageFunc
)

func feishuPage(x App, c *ui.Context, pal *theme.Palette) { Feishu(x, c, pal) }

// bundledFont is the name of the font hyla brings.
const bundledFont = "JetBrains Mono Nerd Font"

// LocaleTag is the locale a language names.
func LocaleTag(language string) string { return localeTag(language) }

// KeyNames are the keys a shortcut takes, by their accelerator names.
func KeyNames() map[ui.Key]string { return keyNames }
