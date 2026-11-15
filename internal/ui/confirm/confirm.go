// Package confirm asks the user a question, with its answers, before the
// app does what can not be undone.
package confirm

import (
	"github.com/egoist/mygo/ui"

	"hyla/internal/ui/style"
	"hyla/internal/ui/theme"
	"hyla/locales"
)

// Ask is a question the app asks, and the answers it offers.
type Ask struct {
	Open   bool
	Title  string
	Detail string
	// Action and Run are the main answer, which Return takes.
	Action string
	Run    func()
	// Destroy marks the question as one before a loss, whose main answer
	// wears the warning.
	Destroy bool
	// Ends marks the question as one before a quit: Return may be what a
	// program reads, so it takes a click or Escape.
	Ends bool
	// Alt is a second answer besides Action and Cancel, which runs
	// AltRun; AltDestroy marks it as one that destroys.
	Alt        string
	AltRun     func()
	AltDestroy bool
}

// Dialog asks *pending's question in c's window, and clears it once
// answered, which it reports.
func Dialog(c *ui.Context, pal *theme.Palette, pending **Ask) bool {
	cf := *pending
	run, alt := false, false
	ui.DialogBase(c.Key("confirm"), &cf.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		width := float32(380)
		if cf.Alt != "" {
			width = 460 // room for a third button
		}
		style.Floating(panel.Width(width).Padding(22).Gap(8).Radius(14).Background(pal.Popover), pal).Label(cf.Title)
		ui.Text(c, cf.Title).FontSize(15).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, cf.Detail).FontSize(13).TextColor(pal.TextMuted).LineHeight(1.4)
		ui.Row(c).Gap(8).Justify(ui.End).Margin(14, 0, 0, 0).Children(func() {
			if cf.Ends {
				// Return may be what a program reads, so this question
				// takes a click, or Escape: the focus goes to the panel,
				// not to Cancel, which a Return would press.
				ui.Box(c).Focus()
				if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
					cf.Open = false
				}
				if style.Button(c, "ok", cf.Action, "", true, pal) {
					run = true
				}
				return
			}
			if style.Button(c, "cancel", locales.CommonCancel(), "esc", false, pal) {
				cf.Open = false
			}
			if cf.Alt != "" && style.Button(c, "alt", cf.Alt, "", false, pal) {
				alt = true
			}
			if style.DefaultButton(c, "ok", cf.Action, "↩", true, pal) || c.Shortcut(0, ui.KeyEnter) {
				run = true
			}
		})
		if !cf.Ends && c.Shortcut(0, ui.KeyEnter) {
			run = true
		}
	})
	if run {
		cf.Open = false
		cf.Run()
	} else if alt {
		cf.Open = false
		cf.AltRun()
	}
	// Only once the answer came: what it runs may clear *pending itself,
	// which removeHost does as the page, rebuilt, sees the host gone.
	// Another question asked in its place, as the page's own, stays.
	if cf.Open {
		return false
	}
	if *pending == cf {
		*pending = nil
	}
	return true
}
