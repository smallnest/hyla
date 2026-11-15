package main

import (
	"hyla/internal/platform"
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
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"hyla/internal/actions"
	"hyla/internal/adblock"
	"hyla/internal/buildinfo"
	"hyla/internal/cli"
	"hyla/internal/client"
	"hyla/internal/proto"
	"hyla/internal/settings"
	"hyla/internal/shell"
	"hyla/internal/ui/paints"
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
	{"sync", locales.PageSync, glyph.Sync, false},
	{"hosts", locales.PageHosts, glyph.Host, true},
	{"advanced", locales.PageAdvanced, glyph.Of("f7-slider-horizontal-3"), false},
}

// settingsState is what the Settings window keeps besides the settings.
type settingsState struct {
	page       string
	recording  string // the action whose shortcut is being typed
	recordRef  ui.Handle
	filter     string // the keyboard page's search
	query      string // the sidebar's search, of every page
	searchRef  ui.Handle
	revealed   string // the query and page the search last scrolled to
	draft      map[string]string
	envDraft   string
	shellDraft string
	// featuresDraft is font-feature as the field has it, while
	// featuresEditing.
	featuresDraft   string
	featuresEditing bool
	shellEditing    bool
	loginOpen       bool // open at login, as the system says
	petKnob         bool // the pet's size slider was held, last frame
	updates         updatePrefs
	cliPath         string
	service         string // hylad, as last asked
	notice          string // a line under the keyboard page, as of a conflict
	toast           string
	menus           map[string]*controls.SearchMenu // the search menus', by key
	problems        []string                        // what the configuration file got wrong
	scrolls         map[string]*ui.ScrollState      // each page's, for its scroll edge
	tailcat         tailcatDraft                    // the Tailcat group's
	remote          hostDraft                       // the other machines' form
	tcHost          tcHostDraft                     // the Tailcat machines' form
	feishu          feishuDraft                     // the Feishu page's form
	confirm         *confirmation                   // a question, asked in this window
}

// checkUpdates asks the updater for a newer app, which none does where
// the app has no updater, as in a test.
func (a *App) checkUpdates() {
	defer func() { recover() }() // no updater: the panic of mygo's CheckForUpdates
	updater.CheckForUpdates()
}

// openSettings shows the Settings window, on a page.
func (a *App) openSettings(page string) {
	if page != "" {
		a.Ss.page = page
	}
	if a.Ss.page == "" {
		a.Ss.page = "general"
	}
	if a.SetWin != nil {
		a.SetWin.Show()
		a.SetWin.Focus()
		return
	}
	a.Ss.loginOpen = mygo.App.OpenAtLogin()
	a.refreshUpdates()
	offUpdates := updater.OnChange(func() {
		a.refreshUpdates()
		if a.SetWin != nil {
			a.SetWin.Invalidate()
		}
	})
	a.Ss.draft = map[string]string{}
	a.Ss.feishu = feishuDraft{}
	a.refreshAgents()
	a.refreshService()
	a.SetWin = mygo.NewWindow(platform.Frame(mygo.WindowOptions{
		Title: locales.SettingsTitle(), Width: 820, Height: 620, MinWidth: 700, MinHeight: 480,
		StateKey: "settings",
		Vibrancy: vibrancyFor(a.settingsIn),
		Content:  ui.View(a.settingsView),
	}, topBar, barMid))
	a.SetWin.OnClosed(func() {
		offUpdates()
		a.SetWin = nil
		a.setRecording("") // the menus take their shortcuts back
		a.Ss.query = ""
	})
}

// updatePrefs are the updater's, as last read: the page reads these, not
// the updater, which needs the running app.
type updatePrefs struct {
	enabled     bool // this copy of Hyla can update itself
	automatic   bool // checks in the background
	download    bool // and installs what it finds without asking
	lastChecked time.Time
}

func (a *App) refreshUpdates() {
	a.Ss.updates = updatePrefs{
		enabled:     mygo.Updater.Enabled(),
		automatic:   updater.AutomaticChecks(),
		download:    updater.AutomaticDownloads(),
		lastChecked: updater.LastCheck(),
	}
}

func (a *App) refreshAgents() {
	a.Ss.cliPath = cliInstalled()
}

// refreshService asks hylad how it is, off the main thread.
func (a *App) refreshService() {
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
		a.update(func() {
			a.Ss.service = line
			if a.SetWin != nil {
				a.SetWin.Invalidate()
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

func (a *App) settingsView(c *ui.Context) {
	pal := theme.For(c)
	c.SetTheme(borderless(c, pal))
	if a.Ss.toast != "" {
		c.Toast(a.Ss.toast)
		a.Ss.toast = ""
	}
	found := a.searchSettings()
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { a.Ss.searchRef.Focus(c) })
	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	theme.Backdrop(c, pal, row, a.settingsIn)
	row.Children(func() {
		// Under the traffic lights on macOS; elsewhere the search starts
		// the sidebar, centered on barMid as the page's title and the
		// window controls are.
		top := float32(topBar)
		if platform.OwnControls {
			top = barMid - (style.ControlH+2)/2
		}
		ui.Column(c).Width(212).Shrink(0).Padding(top, 10, 12, 10).Gap(2).DragWindow().Children(func() {
			a.settingsSearch(c, pal)
			for _, p := range settingsPages {
				if found == nil || found[p.id] != nil {
					a.settingsNav(c, pal, p)
				}
			}
			if found != nil && len(found) == 0 {
				ui.Text(c, locales.CommonNoResults()).FontSize(13).TextColor(pal.TextFaint).Padding(8, 10)
			}
			ui.Spacer(c)
			link := ui.ButtonBase(c.Key("open-config")).Padding(6, 10).Gap(6).Radius(7).AlignItems(ui.Center).Label(locales.SettingsOpenConfigLabel())
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
				a.openConfig()
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Padding(cardGap, cardGap, cardGap, 0).Children(func() {
			card := style.Raised(ui.Column(c).Grow(1).MinHeight(0).Radius(14).Background(pal.Card), pal).Clip()
			card.Children(func() {
				page := settingsPages[0]
				for _, p := range settingsPages {
					if p.id == a.Ss.page {
						page = p
					}
				}
				// The page scrolls under its header, which macOS 27's hard
				// scroll edge frosts it under, as System Settings does.
				// The edge shows, as AppKit's, once content is under the bar,
				// or while the pointer is over it.
				if a.Ss.scrolls == nil {
					a.Ss.scrolls = map[string]*ui.ScrollState{}
				}
				sc := a.Ss.scrolls[page.id]
				if sc == nil {
					sc = &ui.ScrollState{}
					a.Ss.scrolls[page.id] = sc
				}
				box := ui.Box(c.Key("page")).Grow(1).MinHeight(0)
				_, py, over := box.PointerPosition()
				edgeOn := sc.Y > 0.5 || over && py < headerH
				box.Children(func() {
					ui.Scroll(c.Key("page-"+page.id)).Fill().Padding(headerH+16, 24, 28, 24).ScrollbarInsets(headerH, 0, 0).TrackScroll(sc).Children(func() {
						ui.Column(c).Gap(22).MaxWidth(620).Children(func() {
							switch page.id {
							case "general":
								a.generalPage(c, pal)
							case "appearance":
								a.appearancePage(c, pal)
							case "terminal":
								a.terminalPage(c, pal)
							case "shell":
								a.shellPage(c, pal)
							case "browser":
								a.browserPage(c, pal)
							case "notifications":
								a.notificationsPage(c, pal)
							case "keyboard":
								a.keyboardPage(c, pal)
							case "feishu":
								a.feishuPage(c, pal)
							case "agents":
								a.agentsPage(c, pal)
							case "sync":
								a.syncPage(c, pal)
							case "hosts":
								a.tailcatGroup(c, pal)
								a.p2pGroup(c, pal)
							case "advanced":
								a.advancedPage(c, pal)
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
						windowControls(c, pal, a.SetWin)
					})
				})
			})
		})
	})
	if a.Ss.confirm != nil {
		confirmDialog(c, pal, &a.Ss.confirm)
	}
	// The controls edit a.settingsIn: what changed applies, and saves.
	if !reflect.DeepEqual(a.settingsIn, a.Applied) {
		a.applySettings(a.settingsIn)
	}
}

// searchSettings applies the sidebar's search: it shows a page with
// finds, when the page shown has none, and sets what the pages highlight.
// It returns what searchResults does.
func (a *App) searchSettings() map[string][]string {
	found := searchResults(a.Ss.query, a.settingsIn.Shortcuts)
	searchHits, searchReveal = nil, ""
	if found == nil {
		a.Ss.revealed = ""
		return nil
	}
	if found[a.Ss.page] == nil {
		for _, p := range settingsPages {
			if found[p.id] != nil {
				a.Ss.page = p.id
				a.setRecording("")
				break
			}
		}
	}
	labels := found[a.Ss.page]
	searchHits = map[string]bool{}
	for _, l := range labels {
		searchHits[l] = true
	}
	// The first find comes into view once for each query and page, after
	// which the page scrolls as the user likes.
	if at := a.Ss.query + "\x00" + a.Ss.page; len(labels) > 0 && a.Ss.revealed != at {
		a.Ss.revealed, searchReveal = at, labels[0]
	}
	return found
}

// settingsSearch is the sidebar's search field, which finds settings in
// every page by their labels, their keys in the configuration file, and
// words for them.
func (a *App) settingsSearch(c *ui.Context, pal *theme.Palette) {
	var in ui.Element
	frame := ui.Row(c.Key("search")).Cursor(ui.CursorText).Margin(0, 0, 10, 0)
	frame.Children(func() {
		ui.Icon(c, glyph.Search).Size(13, 13).TextColor(pal.TextFaint).Shrink(0)
		in = ui.TextInputBase(c.Key("in"), &a.Ss.query).Bind(&a.Ss.searchRef).Grow(1).MinWidth(0).AlignSelf(ui.Stretch).FontSize(13).
			Placeholder(locales.CommonSearch()).Label(locales.SettingsSearchLabel())
		in.OnShortcut(0, ui.KeyEscape, func() { a.Ss.query = "" })
		if a.Ss.query != "" {
			x := ui.ButtonBase(c.Key("clear")).Size(16, 16).Radius(8).Center().Shrink(0).Label(locales.SettingsClearSearch()).
				Background(pal.TextFaint.Alpha(0.55)).TextColor(pal.Card)
			if x.Hovered() {
				x.Background(pal.TextMuted)
			}
			x.Children(func() { ui.Icon(c, glyph.Close).Size(8, 8) })
			if x.Clicked() {
				a.Ss.query = ""
				a.Ss.searchRef.Focus(c)
			}
		}
	})
	controls.Field(c, frame, in.Focused(), pal)
}

func (a *App) settingsNav(c *ui.Context, pal *theme.Palette, p settingsPage) {
	sel := a.Ss.page == p.id
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
		a.Ss.page = p.id
		a.setRecording("")
	}
	item.Children(func() {
		ic := ui.Icon(c, p.icon).Size(15, 15).TextColor(pal.TextMuted)
		if sel {
			ic.TextColor(pal.Accent)
		}
		ui.Text(c, p.title()).FontSize(13.5).TextColor(pal.Text).SingleLine()
	})
}

// groupTitle is a group's title, which the search highlights as it does
// a setting.
func groupTitle(c *ui.Context, pal *theme.Palette, title string) {
	row := ui.Row(c.Key("title-"+title)).Padding(0, 4)
	row.Children(func() {
		searchHighlight(c, row, title, pal, 2, 3)
		ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(pal.TextMuted)
	})
}

// group is a titled set of settings, on a soft panel.
func settingsGroup(c *ui.Context, pal *theme.Palette, title, note string, fn func()) {
	ui.Column(c).Gap(8).Children(func() {
		if title != "" {
			groupTitle(c, pal, title)
		}
		if fn != nil { // a group of a note alone has no panel
			groupRows = 0
			ui.Column(c).Radius(12).Background(pal.Chip).Padding(4, 16).Children(fn)
		}
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(pal.TextFaint).LineHeight(1.4).Padding(0, 4)
		}
	})
}

// groupRows counts the rows of the group being built, so that a divider
// goes between them, as in macOS's Settings.
var groupRows int

// setting is a row of a group: a label, what it does, and its control.
func settingRow(c *ui.Context, pal *theme.Palette, label, detail string, control func()) {
	if groupRows++; groupRows > 1 {
		ui.Box(c).Height(1).Shrink(0).Background(pal.Text.Alpha(0.07))
	}
	row := ui.Row(c.Key("row-"+label)).Padding(11, 0).Gap(16).AlignItems(ui.Center)
	row.Children(func() {
		searchHighlight(c, row, label, pal, 10, -2)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, label).FontSize(13.5).TextColor(pal.Text)
			if detail != "" {
				ui.Text(c, detail).FontSize(12).TextColor(pal.TextMuted).LineHeight(1.35)
			}
		})
		ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(control)
	})
}

// segmented chooses one of ids into value, as macOS's segmented control
// shows it: a soft track, the choice a lighter fill raised a hair, thin
// separators between the others, and labels of one weight.
func segmented(c *ui.Context, key string, value *string, ids, labels []string) {
	pal := theme.For(c)
	i := max(slices.Index(ids, *value), 0)
	track, thumb := ui.RGBA(0, 0, 0, 0.06), ui.Hex("#ffffff")
	if pal.Dark {
		track, thumb = ui.RGBA(255, 255, 255, 0.07), ui.RGBA(255, 255, 255, 0.2)
	}
	seg := ui.SegmentedBase(c.Key(key), &i, len(labels))
	seg.Track.Height(24).Padding(2).Radius(7).Background(track).AlignItems(ui.Center).Children(func() {
		for n, label := range labels {
			if n > 0 {
				// A separator, where neither segment beside it is chosen.
				sep := ui.Box(c).Width(1).Height(12).Shrink(0)
				if n != i && n-1 != i {
					sep.Background(pal.Text.Alpha(0.14))
				}
			}
			s := seg.Segment(n).FillHeight().Padding(0, 12).Radius(5).Center()
			if n == i {
				s.Background(thumb)
				if pal.Dark {
					s.Shadow(0, 1, 1, 0, ui.RGBA(0, 0, 0, 0.25))
				} else {
					s.Shadow(0, 0.5, 1.5, 0, ui.RGBA(0, 0, 0, 0.16)).Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.04))
				}
			}
			s.Children(func() {
				ui.Text(c, label).FontSize(12.5).SingleLine().TextColor(pal.Text)
			})
		}
	})
	// The choice lands after this runs: take it then.
	seg.Track.OnChange(func() { *value = ids[i] })
}

func (a *App) generalPage(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
	settingsGroup(c, pal, locales.SetSessions(), locales.SetSessionsDetail(a.tip(strings.TrimSuffix(actionTitle("app.quitAll"), "…"), "app.quitAll")), func() {
		settingRow(c, pal, locales.SetNewSessionsStartIn(), "", func() {
			segmented(c, "newdir", &s.NewSessionDir, []string{"current", "project", "home"}, []string{locales.SetDirCurrent(), locales.SetDirProject(), locales.SetDirHome()})
		})
		settingRow(c, pal, locales.SetSessionName(), locales.SetSessionNameDetail(), func() {
			segmented(c, "sessionname", &s.SessionName, []string{proto.NameFocused, proto.NameLead, proto.NameFolder},
				[]string{locales.SetNameFocused(), locales.SetNameLead(), locales.SetNameFolder()})
		})
		settingRow(c, pal, locales.SetSidebarRows(), locales.SetSidebarRowsDetail(), func() {
			segmented(c, "sidebar-rows", &s.SidebarRows, []string{"detailed", "compact"}, []string{locales.SetRowsDetailed(), locales.SetRowsCompact()})
		})
		settingRow(c, pal, locales.SetFocusFollowsMouse(), locales.SetFocusFollowsMouseDetail(), func() {
			style.Toggle(c.Key("focusmouse"), &s.FocusFollowsMouse, pal)
		})
		settingRow(c, pal, locales.SetAskBeforeClosing(), locales.SetAskBeforeClosingDetail(), func() {
			style.Toggle(c.Key("confirm"), &s.ConfirmClose, pal)
		})
		settingRow(c, pal, locales.SetQuitEndsSessions(), locales.SetQuitEndsSessionsDetail(), func() {
			style.Toggle(c.Key("quit-ends"), &s.QuitEndsSessions, pal)
		})
		settingRow(c, pal, locales.SetRestoreSessions(), locales.SetRestoreSessionsDetail(), func() {
			style.Toggle(c.Key("restore"), &s.RestoreSessions, pal)
		})
		if s.RestoreSessions {
			settingRow(c, pal, locales.SetResumeAgents(), locales.SetResumeAgentsDetail(), func() {
				style.Toggle(c.Key("resume"), &s.ResumeAgents, pal)
			})
		}
	})
	settingsGroup(c, pal, locales.SetGit(), locales.SetGitDetail(), func() {
		settingRow(c, pal, locales.SetGitStatus(), locales.SetGitStatusDetail(), func() {
			style.Toggle(c.Key("git-status"), &s.GitStatus, pal)
		})
		settingRow(c, pal, locales.SetDiffLayout(), locales.SetDiffLayoutDetail(), func() {
			segmented(c, "diff-layout", &s.DiffLayout, []string{"auto", "unified", "split"},
				[]string{locales.SetDiffAuto(), locales.ChangesUnified(), locales.ChangesSplit()})
		})
	})
	// Windows and Linux always show the icon, in the notification area.
	if runtime.GOOS == "darwin" {
		detail := ""
		if s.AppIcon != "dock" {
			detail = locales.SetAppIconDetail()
		}
		settingsGroup(c, pal, "", "", func() {
			settingRow(c, pal, locales.SetAppIcon(), detail, func() {
				controls.Popup(c, "app-icon", &s.AppIcon, settings.AppIcons, appIconName, 180, pal)
			})
		})
	}
	settingsGroup(c, pal, locales.SetStartup(), "", func() {
		settingRow(c, pal, locales.SetOpenAtLogin(), "", func() {
			style.Toggle(c.Key("login"), &a.Ss.loginOpen, pal).OnChange(func() {
				if err := mygo.App.SetOpenAtLogin(a.Ss.loginOpen); err != nil {
					a.Ss.loginOpen = !a.Ss.loginOpen
					a.Ss.toast = locales.SetLoginFailed(err.Error())
				}
			})
		})
	})
	a.updatesGroup(c, pal)
}

// browserPage is Settings › Browser: what browser panes block.
func (a *App) browserPage(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
	lists := ""
	if s.BlockAds {
		lists = locales.SetAdListsNote()
	}
	settingsGroup(c, pal, locales.SetAdBlocking(), lists, func() {
		settingRow(c, pal, locales.SetBlockAds(), locales.SetBlockAdsDetail(), func() {
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
		settingRow(c, pal, src.Name, adListDetail(src), func() {
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

func (a *App) updatesGroup(c *ui.Context, pal *theme.Palette) {
	u := &a.Ss.updates
	version := "Hyla " + mygo.App.Version()
	switch {
	case !u.enabled:
		version += locales.SetVersionNoUpdate()
	case u.lastChecked.IsZero():
		version += locales.SetVersionNotChecked()
	default:
		version += locales.SetVersionLastChecked(durationWords(time.Since(u.lastChecked)))
	}
	settingsGroup(c, pal, locales.SetUpdates(), "", func() {
		settingRow(c, pal, locales.SetVersion(), version, func() {
			if style.Button(c, "check-updates", locales.SetCheckForUpdates(), "", false, pal) {
				a.checkUpdates()
			}
		})
		settingRow(c, pal, locales.SetAutoCheck(), locales.SetAutoCheckDetail(), func() {
			style.Toggle(c.Key("auto-check"), &u.automatic, pal).OnChange(func() { updater.SetAutomaticChecks(u.automatic) })
		})
		settingRow(c, pal, locales.SetAutoInstall(), locales.SetAutoInstallDetail(), func() {
			style.Toggle(c.Key("auto-download"), &u.download, pal).OnChange(func() { updater.SetAutomaticDownloads(u.download) })
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

func (a *App) appearancePage(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
	settingsGroup(c, pal, "", "", func() {
		settingRow(c, pal, locales.SetAppearance(), "", func() {
			segmented(c, "appearance", &s.Appearance, []string{"system", "light", "dark"}, []string{locales.SetThemeSystem(), locales.SetThemeLight(), locales.SetThemeDark()})
		})
		settingRow(c, pal, locales.SetLanguage(), "", func() {
			controls.Popup(c, "language", &s.Language, append([]string{"system"}, settings.Languages...), languageName, 180, pal)
		})
		settingRow(c, pal, locales.SetTranslucent(), locales.SetTranslucentDetail(), func() {
			style.Toggle(c.Key("vibrancy"), &s.Vibrancy, pal)
		})
		settingRow(c, pal, locales.SetSessionSwitch(), locales.SetSessionSwitchDetail(), func() {
			controls.Popup(c, "session-switch", &s.SessionSwitch, settings.SessionSwitches, sessionSwitchName, 160, pal)
		})
		settingRow(c, pal, locales.SetWindowOpacity(), locales.SetWindowOpacityDetail(), func() {
			v := float64(s.WindowOpacity)
			controls.Slider(c, "window-opacity", &v, 0, 1, 160, pal).OnChange(func() {
				s.WindowOpacity = float32(int(v*100+0.5)) / 100
			})
			ui.Textf(c, "%.0f%%", s.WindowOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
		settingRow(c, pal, locales.SetWindowBlur(), locales.SetWindowBlurDetail(), func() {
			style.Toggle(c.Key("window-blur"), &s.WindowBlur, pal)
		})
		settingRow(c, pal, locales.SetWindowMaterial(), locales.SetWindowMaterialDetail(), func() {
			controls.Popup(c, "window-material", &s.WindowMaterial, []string{"sidebar", "window", "popover", "plastic", "glass", "glass-clear"}, materialName, 200, pal)
		})
		detail := locales.SetSidebarImageDetail()
		if s.SidebarImage != "" {
			detail = filepath.Base(s.SidebarImage)
		}
		settingRow(c, pal, locales.SetSidebarImage(), detail, func() {
			if style.Button(c, "sidebar-image", locales.SetSidebarImageChoose(), "", false, pal) {
				a.pickSidebarImage()
			}
			if s.SidebarImage != "" && style.Button(c, "sidebar-image-clear", locales.CommonRemove(), "", false, pal) {
				s.SidebarImage = ""
			}
		})
		if s.SidebarImage != "" {
			settingRow(c, pal, locales.SetSidebarImageOpacity(), locales.SetSidebarImageOpacityDetail(), func() {
				v := float64(s.SidebarImageOpacity)
				controls.Slider(c, "sidebar-image-opacity", &v, 0, 1, 160, pal).OnChange(func() {
					s.SidebarImageOpacity = float32(int(v*100+0.5)) / 100
				})
				ui.Textf(c, "%.0f%%", s.SidebarImageOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
			})
		}
		detail = locales.SetPaneImageDetail()
		if s.PaneImage != "" {
			detail = filepath.Base(s.PaneImage)
		}
		settingRow(c, pal, locales.SetPaneImage(), detail, func() {
			if style.Button(c, "pane-image", locales.SetSidebarImageChoose(), "", false, pal) {
				a.pickPaneImage()
			}
			if s.PaneImage != "" && style.Button(c, "pane-image-clear", locales.CommonRemove(), "", false, pal) {
				s.PaneImage = ""
			}
		})
		if s.PaneImage != "" {
			settingRow(c, pal, locales.SetPaneImageOpacity(), locales.SetPaneImageOpacityDetail(), func() {
				v := float64(s.PaneImageOpacity)
				controls.Slider(c, "pane-image-opacity", &v, 0, 1, 160, pal).OnChange(func() {
					s.PaneImageOpacity = float32(int(v*100+0.5)) / 100
				})
				ui.Textf(c, "%.0f%%", s.PaneImageOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
			})
			settingRow(c, pal, locales.SetPaneImageFit(), locales.SetPaneImageFitDetail(), func() {
				controls.Popup(c, "pane-image-fit", &s.PaneImageFit, settings.PaneImageFits, paneFitName, 200, pal)
			})
		}
	})
	settingsGroup(c, pal, locales.SetPetGroup(), "", func() {
		settingRow(c, pal, locales.SetPet(), locales.SetPetDetail(), func() {
			style.Toggle(c.Key("pet"), &s.Pet, pal)
		})
		settingRow(c, pal, locales.SetPetWhich(), a.petGallery(), func() {
			a.petChooser(c, pal)
		})
		settingRow(c, pal, locales.SetPetSize(), locales.SetPetSizeDetail(), func() {
			v := float64(s.PetSize)
			controls.Slider(c, "pet-size", &v, 0.5, 3, 160, pal).OnChange(func() {
				// The pet grows as the knob drags; the settings keep its
				// size when the knob goes.
				s.PetSize = float32(int(v*10+0.5)) / 10
				a.petGrow()
			})
			ui.Textf(c, "%.1f×", s.PetSize).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
	})
	ui.Column(c).Gap(8).Children(func() {
		// The washes under their own title, and Plain, which is the
		// window's material alone, with the material's under foot.
		groupTitle(c, pal, locales.SetBackground())
		ui.Row(c).Gap(12).Wrap().Children(func() {
			for _, b := range settings.Backgrounds {
				if b.ID == "plain" {
					continue
				}
				a.backgroundTile(c, pal, b.ID, backgroundName(b.ID, b.Name))
			}
		})
		ui.Row(c).Gap(12).Children(func() {
			a.backgroundTile(c, pal, "plain", backgroundName("plain", "Plain"))
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
		"ember": locales.BgEmber, "tidal": locales.BgTidal, "umbra": locales.BgUmbra, "static": locales.BgStatic}[id]; ok {
		return t()
	}
	return name
}

// backgroundTile previews a wash, choosing it on a click.
func (a *App) backgroundTile(c *ui.Context, pal *theme.Palette, id, name string) {
	sel := a.settingsIn.Background == id
	tile := ui.ButtonBase(c.Key("bg-" + id)).Column().Width(104).Gap(8).AlignItems(ui.Stretch).Label(name)
	if tile.Clicked() {
		a.settingsIn.Background = id
		if id == "dock" {
			// The Dock's is Liquid Glass under a thin veil: choose it, and
			// let the veil show where the wash covered nothing at all.
			a.settingsIn.WindowMaterial = "glass"
			if a.settingsIn.WindowOpacity < 0.2 {
				a.settingsIn.WindowOpacity = 0.45
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

func (a *App) terminalPage(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
	a.terminalPreview(c, pal)
	settingsGroup(c, pal, locales.SetText(), "", func() {
		settingRow(c, pal, locales.SetFont(), locales.SetFontDetail(), func() {
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
			controls.SearchPopup(c, &a.Ss.menus, "font", family, opts, 240, locales.SetUseFont, pal, func(v string) {
				switch v {
				case "JetBrains Mono Nerd Font":
					v = ""
				case locales.SetFontSystem():
					v = "monospace"
				}
				a.settingsIn.FontFamily = v
			})
		})
		settingsGroup(c, pal, "", locales.SetFontFallbackDetail(), func() {
			a.fontFallbackField(c, pal)
		})
		settingRow(c, pal, locales.SetSize(), locales.SetSizeDetail(a.keys("text.bigger"), a.keys("text.smaller")), func() {
			sizes := []float32{10, 11, 12, 13, 14, 15, 16, 18, 20, 22, 24, 28}
			if !slices.Contains(sizes, s.FontSize) {
				sizes = append(sizes, s.FontSize)
				slices.Sort(sizes)
			}
			controls.Popup(c, "size", &s.FontSize, sizes, func(v float32) string { return strconv.FormatFloat(float64(v), 'f', -1, 32) + " pt" }, 90, pal)
		})
		settingRow(c, pal, locales.SetLineHeight(), "", func() {
			v := float64(s.LineHeight)
			controls.Slider(c, "lh", &v, 1, 2, 160, pal).OnChange(func() { a.settingsIn.LineHeight = float32(int(v*100+0.5)) / 100 })
			ui.Textf(c, "%.2f×", s.LineHeight).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
		settingRow(c, pal, locales.SetWeight(), "", func() {
			controls.Popup(c, "weight", &s.FontWeight, []int{100, 200, 300, 400, 500, 600}, func(w int) string {
				return map[int]string{
					100: locales.SetWeightThin(), 200: locales.SetWeightExtraLight(),
					300: locales.SetWeightLight(), 400: locales.SetWeightRegular(),
					500: locales.SetWeightMedium(), 600: locales.SetWeightSemibold(),
				}[w]
			}, 140, pal)
		})
		settingRow(c, pal, locales.SetLigatures(), locales.SetLigaturesDetail(), func() {
			style.Toggle(c.Key("liga"), &s.Ligatures, pal)
		})
		settingRow(c, pal, locales.SetFontFeatures(), locales.SetFontFeaturesDetail(), func() {
			a.fontFeaturesField(c, pal)
		})
		settingRow(c, pal, locales.SetThicken(), locales.SetThickenDetail(), func() {
			style.Toggle(c.Key("thicken"), &s.Thicken, pal)
		})
	})
	themes := append([]string{"hyla"}, terminal.GhosttyThemes()...)
	settingsGroup(c, pal, locales.SetColors(), locales.SetColorsDetail(), func() {
		settingRow(c, pal, locales.SetLightWindows(), "", func() {
			controls.SearchPopup(c, &a.Ss.menus, "light", s.LightTheme, themes, 240, nil, pal, func(v string) { a.settingsIn.LightTheme = v })
		})
		settingRow(c, pal, locales.SetDarkWindows(), "", func() {
			controls.SearchPopup(c, &a.Ss.menus, "dark", s.DarkTheme, themes, 240, nil, pal, func(v string) { a.settingsIn.DarkTheme = v })
		})
		settingRow(c, pal, locales.SetDimUnfocused(), locales.SetDimUnfocusedDetail(), func() {
			style.Toggle(c.Key("dim-unfocused"), &s.DimUnfocused, pal)
		})
	})
	// As the window's translucency: an opacity, and a blur or none.
	settingsGroup(c, pal, locales.SetPaneGlass(), "", func() {
		settingRow(c, pal, locales.SetPaneOpacity(), locales.SetPaneOpacityDetail(), func() {
			v := float64(s.PaneOpacity)
			controls.Slider(c, "pane-opacity", &v, 0, 1, 160, pal).OnChange(func() {
				s.PaneOpacity = float32(int(v*100+0.5)) / 100
			})
			ui.Textf(c, "%.0f%%", s.PaneOpacity*100).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
		settingRow(c, pal, locales.SetPaneBlur(), locales.SetPaneBlurDetail(), func() {
			style.Toggle(c.Key("pane-blur"), &s.PaneBlur, pal)
		})
		settingRow(c, pal, locales.SetPaneMaterial(), locales.SetPaneMaterialDetail(), func() {
			controls.Popup(c, "pane-material", &s.PaneMaterial, []string{"glass", "plastic", "glass-clear"}, materialName, 200, pal)
		})
	})
	settingsGroup(c, pal, locales.SetCursor(), "", func() {
		settingRow(c, pal, locales.SetShape(), locales.SetShapeDetail(), func() {
			segmented(c, "cursor", &s.Cursor, []string{"block", "bar", "underline"}, []string{locales.SetCursorBlock(), locales.SetCursorBar(), locales.SetCursorUnderline()})
		})
		settingRow(c, pal, locales.SetBlink(), "", func() { style.Toggle(c.Key("blink"), &s.CursorBlink, pal) })
	})
	settingsGroup(c, pal, locales.SetInput(), "", func() {
		settingRow(c, pal, locales.SetOptionMeta(), locales.SetOptionMetaDetail(), func() {
			style.Toggle(c.Key("optalt"), &s.OptionAsAlt, pal)
		})
		settingRow(c, pal, locales.SetCopyOnSelect(), locales.SetCopyOnSelectDetail(), func() {
			style.Toggle(c.Key("cos"), &s.CopyOnSelect, pal)
		})
		settingRow(c, pal, locales.SetRightClick(), "", func() {
			controls.Popup(c, "rc", &s.RightClick, []string{"menu", "paste", "copyOrPaste"}, func(id string) string {
				return map[string]string{"menu": locales.SetRightClickMenu(), "paste": locales.SetRightClickPaste(), "copyOrPaste": locales.SetRightClickCopyOrPaste()}[id]
			}, 170, pal)
		})
		settingRow(c, pal, locales.SetClipboardRead(), locales.SetClipboardReadDetail(), func() {
			segmented(c, "clip", &s.Clipboard, []string{"ask", "allow", "deny"}, []string{locales.SetAsk(), locales.SetAllow(), locales.SetDeny()})
		})
	})
	// mouse-scroll-multiplier's discrete: and precision:.
	settingsGroup(c, pal, locales.SetScrolling(), "", func() {
		settingRow(c, pal, locales.SetWheelScroll(), locales.SetWheelScrollDetail(), func() {
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
		settingRow(c, pal, locales.SetTrackpadScroll(), locales.SetTrackpadScrollDetail(), func() {
			v := float64(s.ScrollPrecision)
			controls.Slider(c, "trackpad", &v, 0.25, 3, 160, pal).OnChange(func() { a.settingsIn.ScrollPrecision = float32(int(v*20+0.5)) / 20 })
			ui.Textf(c, "%.2f×", s.ScrollPrecision).FontSize(12.5).TextColor(pal.TextMuted).Width(40).FontFeatures("tnum")
		})
	})
}

// terminalPreview shows text as the terminals would draw it: the font,
// its size and line height, and the colors of the window's appearance.
func (a *App) terminalPreview(c *ui.Context, pal *theme.Palette) {
	light, dark := a.termThemes()
	th := light
	if pal.Dark {
		th = dark
	}
	features := uiFeatures(a.fontFeatures())
	line := func(spans ...ui.Span) {
		// The preview draws in the UI's font, and takes the terminals' size,
		// weight and features: another family shapes the lines with its own
		// advances, and the preview keeps no cell width of its own.
		for i := range spans {
			spans[i].Size, spans[i].Weight, spans[i].Features = a.settingsIn.FontSize, a.settingsIn.FontWeight, features
		}
		ui.RichText(c, spans...).LineHeight(a.settingsIn.LineHeight).SingleLine()
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
func (a *App) fontFallbackField(c *ui.Context, pal *theme.Palette) {
	cur := a.fallbackFonts()
	for i, f := range cur {
		family, at := f, i
		ui.Row(c.Key("fb-"+family)).Padding(8, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%d", at+1).FontSize(12).TextColor(pal.TextFaint).Width(14).FontFeatures("tnum").Shrink(0)
			ui.Text(c, family).FontSize(13).TextColor(pal.Text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1).Grow(1)
			paints.IconButton(c, "fb-x-"+family, glyph.Close, locales.CommonRemove(), pal, func() {
				a.settingsIn.FontFallback = strings.Join(slices.Delete(slices.Clone(cur), at, at+1), ", ")
			})
		})
	}
	label := locales.SetFallbackAdd()
	if len(cur) > 0 {
		label = locales.SetFallbackAddMore()
	}
	ui.Row(c).Padding(8, 0).AlignItems(ui.Center).Children(func() {
		controls.SearchPopup(c, &a.Ss.menus, "fallback", label, fallbackOptions(), 240, locales.SetUseFont, pal, func(v string) {
			a.addFallback(v, -1)
		})
	})
}

// fallbackOptions are the families a fallback may be: the font menu's
// after hyla's own and the system's, then every other family the machine
// has: first those whose names name Chinese, Japanese or Korean, then the
// rest, each listed once. MyGo lists the machine's families once, so one
// installed while hyla runs shows after a restart.
var fallbackOptions = sync.OnceValue(func() []string {
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
})

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
func (a *App) fontFeaturesField(c *ui.Context, pal *theme.Palette) {
	if !a.Ss.featuresEditing {
		a.Ss.featuresDraft = strings.Join(a.settingsIn.FontFeatures, ", ")
	}
	in := controls.TextField(c, "features", &a.Ss.featuresDraft, "ss01, cv05=2, -calt", 200, pal).Font("monospace")
	a.Ss.featuresEditing = in.Focused()
	_, err := settings.ParseFontFeatures(a.Ss.featuresDraft)
	if err != nil {
		in.TextColor(pal.Danger)
	}
	keep := func() {
		if fs, err := settings.ParseFontFeatures(a.Ss.featuresDraft); err == nil && !slices.Equal(fs, a.settingsIn.FontFeatures) {
			a.settingsIn.FontFeatures = fs
		}
	}
	in.OnSubmit(keep)
	if !a.Ss.featuresEditing {
		keep()
	}
}

func (a *App) shellPage(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
	login := shell.Login()
	note := locales.SetShellGroupDetail()
	if len(a.Hosts) > 1 {
		note += " " + locales.SetShellThisComputer()
	}
	settingsGroup(c, pal, locales.SetShellGroup(), note, func() {
		settingRow(c, pal, locales.SetShell(), locales.SetShellDetail(proto.ShortDir(login)), func() {
			if !a.Ss.shellEditing {
				a.Ss.shellDraft = s.Shell
			}
			in := controls.TextField(c, "shell", &a.Ss.shellDraft, proto.ShortDir(login), 220, pal)
			a.Ss.shellEditing = in.Focused()
			// Kept as it is submitted, or as the field loses the focus.
			in.OnSubmit(func() { a.settingsIn.Shell = strings.TrimSpace(a.Ss.shellDraft) })
			if !a.Ss.shellEditing && strings.TrimSpace(a.Ss.shellDraft) != s.Shell {
				a.settingsIn.Shell = strings.TrimSpace(a.Ss.shellDraft)
			}
		})
		settingRow(c, pal, locales.SetScrollback(), locales.SetScrollbackDetail(), func() {
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
	settingsGroup(c, pal, locales.SetEnvironment(), locales.SetEnvironmentDetail(), func() {
		for i, kv := range s.Env {
			k, v, _ := strings.Cut(kv, "=")
			ui.Row(c.Key("env"+strconv.Itoa(i))).Padding(8, 0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, k).Font("monospace").FontSize(12.5).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, v).Font("monospace").FontSize(12.5).TextColor(pal.TextMuted).Grow(1).SingleLine().Ellipsis("…").MinWidth(0)
				idx := i
				paints.IconButton(c, "rm", glyph.Close, locales.CommonRemove(), pal, func() {
					a.settingsIn.Env = slices.Delete(slices.Clone(a.settingsIn.Env), idx, idx+1)
				})
			})
		}
		ui.Row(c).Padding(8, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			addEnv := func() {
				if kv := strings.TrimSpace(a.Ss.envDraft); strings.Contains(kv, "=") {
					a.settingsIn.Env = append(slices.Clone(a.settingsIn.Env), kv)
					a.Ss.envDraft = ""
				}
			}
			controls.TextField(c, "env-new", &a.Ss.envDraft, "NAME=value", 0, pal).Font("monospace").OnSubmit(addEnv)
			if style.FieldButton(c, "env-add", locales.CommonAdd(), false, pal) {
				addEnv()
			}
		})
	})
}

func (a *App) notificationsPage(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
	note := ""
	if !mygo.NotificationsSupported() {
		note = locales.SetNotifyUnpackaged()
	}
	settingsGroup(c, pal, locales.SetNotifyWhen(), note, func() {
		settingRow(c, pal, locales.SetNotifyWaiting(), locales.SetNotifyWaitingDetail(), func() {
			style.Toggle(c.Key("nw"), &s.NotifyWaiting, pal)
		})
		settingRow(c, pal, locales.SetNotifyDone(), locales.SetNotifyDoneDetail(), func() {
			style.Toggle(c.Key("nd"), &s.NotifyDone, pal)
		})
		settingRow(c, pal, locales.SetNotifyFailed(), locales.SetNotifyFailedDetail(), func() {
			style.Toggle(c.Key("nf"), &s.NotifyFailed, pal)
		})
	})
	settingsGroup(c, pal, locales.SetHow(), "", func() {
		settingRow(c, pal, locales.SetPlaySound(), "", func() { style.Toggle(c.Key("ns"), &s.NotifySound, pal) })
		settingRow(c, pal, locales.SetDockBadge(), locales.SetDockBadgeDetail(), func() { style.Toggle(c.Key("nb"), &s.DockBadge, pal) })
		settingRow(c, pal, locales.SetDockBounce(), locales.SetDockBounceDetail(), func() { style.Toggle(c.Key("nbo"), &s.BounceDock, pal) })
	})
	if style.Button(c, "test-notif", locales.SetTestNotification(), "", false, pal) {
		n := mygo.NewNotification(mygo.NotificationOptions{Title: "Hyla", Body: locales.SetTestNotificationBody(), Silent: !s.NotifySound})
		go func() {
			if err := n.Show(); err != nil {
				a.update(func() {
					a.Ss.toast = locales.SetNotifyError(err.Error())
					if a.SetWin != nil {
						a.SetWin.Invalidate()
					}
				})
			}
		}()
	}
}

func (a *App) agentsPage(c *ui.Context, pal *theme.Palette) {
	s := &a.settingsIn
	settingsGroup(c, pal, locales.SetRemoteControl(), "", func() {
		settingRow(c, pal, locales.SetRemoteControlRow(), locales.SetRemoteControlDetail(), func() {
			style.Toggle(c.Key("remote"), &s.RemoteControl, pal)
		})
	})
	codeBlock(c, pal, "hyla do pane.split.down\nhyla do session.attention\nhyla actions        # what hyla do runs")
	settingsGroup(c, pal, locales.SetCli(), locales.SetCliDetail(), func() {
		detail := locales.SetCliNotInstalled()
		if a.Ss.cliPath != "" {
			detail = a.Ss.cliPath
		}
		settingRow(c, pal, locales.SetCliRow(), detail, func() {
			label := locales.CommonInstall()
			if a.Ss.cliPath != "" {
				label = locales.CommonReinstall()
			}
			if style.Button(c, "cli", label, "", a.Ss.cliPath == "", pal) {
				a.installCLI()
				time.AfterFunc(1500*time.Millisecond, func() { a.update(a.refreshAgents) })
			}
		})
	})
	settingsGroup(c, pal, locales.SetSkills(), locales.SetSkillsDetail(), func() {
		for _, st := range cli.SkillStates() {
			name := map[string]string{"claude": "Claude Code", "agents": "Codex, OpenCode, Pi"}[st.Agent]
			detail := map[string]string{"current": locales.SetSkillInstalled(), "outdated": locales.SetSkillOutdated(), "missing": locales.SetSkillMissing()}[st.State] + " · " + proto.ShortDir(st.Dir)
			settingRow(c, pal, name, detail, func() {
				agent := st.Agent
				if st.State != "missing" && style.Button(c, "rm-"+agent, locales.CommonRemove(), "", false, pal) {
					if err := cli.RemoveSkill(agent); err != nil {
						a.toastIn(c, err.Error())
					}
				}
				if st.State != "current" {
					label := locales.CommonInstall()
					if st.State == "outdated" {
						label = locales.CommonUpdate()
					}
					if style.Button(c, "in-"+agent, label, "", true, pal) {
						if err := cli.InstallSkill(agent); err != nil {
							a.toastIn(c, err.Error())
						}
					}
				}
			})
		}
	})
	settingsGroup(c, pal, locales.SetProgramStatus(), locales.SetProgramStatusDetail(), nil)
	codeBlock(c, pal, "hyla report working \"Deploying\" --progress 40\nhyla report blocked \"Approve the migration?\" --kind permission\nhyla report done \"Deployed\"")
}

func codeBlock(c *ui.Context, pal *theme.Palette, code string) {
	ui.Text(c, code).Font("monospace").FontSize(12).LineHeight(1.55).TextColor(pal.Text).
		Padding(12, 16).Radius(10).Background(pal.Chip).Selectable()
}

func (a *App) advancedPage(c *ui.Context, pal *theme.Palette) {
	settingsGroup(c, pal, locales.SetConfiguration(), locales.SetConfigurationDetail(), func() {
		settingRow(c, pal, locales.SetConfigFile(), proto.ShortDir(settings.Path()), func() {
			if style.Button(c, "reveal-config", locales.CommonShowIn(platform.FileManager()), "", false, pal) {
				a.ensureConfig()
				platform.RevealPath(settings.Path())
			}
			if style.Button(c, "edit-config", locales.CommonOpen(), "", true, pal) {
				a.openConfig()
			}
		})
		for i, p := range a.Ss.problems {
			ui.Row(c.Key("problem"+strconv.Itoa(i))).Padding(0, 0, 8, 0).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(pal.Attention).Shrink(0)
				ui.Text(c, p).FontSize(12.5).TextColor(pal.Attention).Grow(1).MinWidth(0)
			})
		}
	})
	settingsGroup(c, pal, locales.SetBackgroundService(), locales.SetBackgroundServiceDetail(), func() {
		settingRow(c, pal, "hylad", a.Ss.service, func() {
			if style.Button(c, "svc-restart", locales.CommonRestart(), "", false, pal) {
				a.Ss.confirm = &confirmation{
					open: true, title: locales.ConfirmRestartServiceTitle(),
					detail: locales.RestartServiceDetail(),
					action: locales.CommonRestart(), destroy: true,
					run: func() { a.restartDaemon(); time.AfterFunc(2*time.Second, func() { a.update(a.refreshService) }) },
				}
			}
		})
		settingRow(c, pal, locales.SetLog(), proto.ShortDir(filepath.Join(proto.Dir(), "hylad.log")), func() {
			if style.Button(c, "log", locales.CommonShow(), "", false, pal) {
				platform.RevealPath(filepath.Join(proto.Dir(), "hylad.log"))
			}
		})
	})
	settingsGroup(c, pal, locales.SetData(), locales.SetDataDetail(), func() {
		settingRow(c, pal, locales.SetFolder(), proto.ShortDir(proto.Dir()), func() {
			if style.Button(c, "reveal", locales.CommonShowIn(platform.FileManager()), "", false, pal) {
				platform.OpenPath(proto.Dir())
			}
		})
		settingRow(c, pal, locales.SetReset(), locales.SetResetDetail(), func() {
			if style.Button(c, "reset", locales.SetResetButton(), "", false, pal) {
				a.Ss.confirm = &confirmation{
					open: true, title: locales.SetResetTitle(), detail: locales.SetResetConfirmDetail(), action: locales.SetResetAction(), destroy: true,
					run: func() { a.applySettings(settings.Default()) },
				}
			}
		})
	})
}

// ensureConfig writes the configuration file, with its header, when
// there is none yet, so that there is a file to open.
func (a *App) ensureConfig() {
	if _, err := os.Stat(settings.Path()); err != nil {
		settings.Save(a.settingsIn)
	}
}

// openConfig opens the configuration in the user's text editor.
func (a *App) openConfig() {
	a.ensureConfig()
	platform.OpenTextFile(settings.Path())
}

// toastIn shows a short message in the window c is of.
func (a *App) toastIn(c *ui.Context, msg string) { c.Toast(msg) }

// --- keyboard ---

func (a *App) keyboardPage(c *ui.Context, pal *theme.Palette) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		controls.TextField(c, "kb-search", &a.Ss.filter, locales.SetKeyboardSearch(), 0, pal)
		if len(a.settingsIn.Shortcuts) > 0 && style.Button(c, "kb-reset", locales.SetRestoreDefaults(), "", false, pal) {
			a.settingsIn.Shortcuts = nil
			a.Ss.notice = ""
		}
	})
	if a.Ss.notice != "" {
		ui.Text(c, a.Ss.notice).FontSize(12).TextColor(pal.Attention).Padding(0, 4)
	}
	q := strings.ToLower(strings.TrimSpace(a.Ss.filter))
	menus := []string{"Hyla", "Session", "Edit", "Pane", "View"}
	for _, menu := range menus {
		var acts []actions.Action
		for _, act := range actions.All {
			keys := actions.Glyphs(actions.Keys(act, a.settingsIn.Shortcuts))
			found := q == "" || strings.Contains(strings.ToLower(actionTitle(act.ID)), q) || strings.Contains(strings.ToLower(act.Title), q) || strings.Contains(strings.ToLower(keys), q) || strings.Contains(act.ID, q)
			if searchHits != nil {
				found = found && searchHits[strings.TrimSuffix(actionTitle(act.ID), "…")]
			}
			if act.Menu == menu && found {
				acts = append(acts, act)
			}
		}
		if len(acts) == 0 {
			continue
		}
		title := map[string]func() string{"Hyla": locales.MenuApp, "Session": locales.MenuSession, "Edit": locales.MenuEdit, "Pane": locales.MenuPane, "View": locales.MenuView}[menu]()
		settingsGroup(c, pal, title, "", func() {
			for _, act := range acts {
				a.shortcutRow(c, pal, act)
			}
		})
	}
	ui.Text(c, locales.SetKeyboardHelp()).FontSize(12).TextColor(pal.TextFaint).Padding(0, 4)
}

func (a *App) shortcutRow(c *ui.Context, pal *theme.Palette, act actions.Action) {
	keys := actions.Keys(act, a.settingsIn.Shortcuts)
	_, custom := a.settingsIn.Shortcuts[act.ID]
	ui.Row(c.Key(act.ID)).Padding(7, 0).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, strings.TrimSuffix(actionTitle(act.ID), "…")).FontSize(13.5).TextColor(pal.Text).SingleLine()
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
				delete(a.settingsIn.Shortcuts, act.ID)
				if len(a.settingsIn.Shortcuts) == 0 {
					a.settingsIn.Shortcuts = nil
				}
			}
		}
		recording := a.Ss.recording == act.ID
		chip := ui.ButtonBase(c.Key("keys")).Height(26).MinWidth(84).Padding(0, 10).Radius(7).Center().Label(locales.SetShortcutFor(strings.TrimSuffix(actionTitle(act.ID), "…")))
		bg := pal.Control
		switch {
		case recording:
			bg = pal.Accent.Alpha(0.18)
		case chip.Hovered():
			bg = pal.ControlHover
		}
		style.Face(chip, pal, bg)
		if chip.Clicked() && !recording {
			a.setRecording(act.ID)
			a.Ss.recordRef.Focus()
		}
		if recording {
			chip.Bind(&a.Ss.recordRef).HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputKeyDown {
					return ev.Kind == ui.InputText
				}
				return a.recordKey(act, ev.Mods, ev.Key)
			})
			if !a.Ss.recordRef.Focused(c) && !chip.Pressed() {
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
func (a *App) setRecording(id string) {
	if a.Ss.recording == id {
		return
	}
	was := a.Ss.recording != ""
	a.Ss.recording = id
	if was != (id != "") && a.Win != nil {
		mygo.App.SetMenu(a.menu())
		a.applyGlobalShortcuts()
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
func (a *App) recordKey(act actions.Action, mods ui.Modifiers, key ui.Key) bool {
	switch {
	case key == ui.KeyEscape && mods == 0:
		a.setRecording("")
		return true
	case (key == ui.KeyBackspace || key == ui.KeyDelete) && mods == 0:
		a.setShortcut(act.ID, "")
		a.setRecording("")
		return true
	}
	name, ok := keyNames[key]
	if !ok {
		return false // a modifier alone, or a key menus can't take
	}
	if mods&(ui.Super|ui.Ctrl|ui.Alt) == 0 && !strings.HasPrefix(name, "F") {
		a.Ss.notice = shortcutNeeds()
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
	a.setShortcut(act.ID, accel)
	a.setRecording("")
	return true
}

// setShortcut binds an action, taking the keys from any other that had
// them.
func (a *App) setShortcut(id, accel string) {
	sc := map[string]string{}
	for k, v := range a.settingsIn.Shortcuts {
		sc[k] = v
	}
	a.Ss.notice = ""
	if accel != "" {
		for _, other := range actions.All {
			if other.ID != id && strings.EqualFold(actions.Keys(other, sc), accel) {
				sc[other.ID] = ""
				a.Ss.notice = locales.SetShortcutTaken(actions.Glyphs(accel), strings.TrimSuffix(actionTitle(other.ID), "…"))
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
	a.settingsIn.Shortcuts = sc
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
